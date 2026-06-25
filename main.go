// proggy parses markdown checklist files and prints a per-section
// completion table to the terminal.
//
// Usage: proggy [flags] [path]
//
// Sections (## .. ###### headings) form a tree. Each row counts the
// `- [x]` / `- [ ]` / `- [/]` checkboxes beneath it, rolling up all
// descendant tasks. The --depth flag controls how deep sub-sections
// render; deeper levels stay hidden but still contribute to counts.
//
// Rows that are fully complete (100%) are dimmed via ANSI escape
// sequences so the eye can skip them and focus on sections that
// still have open work. Dimming is suppressed when stdout is not a
// TTY or when NO_COLOR / TERM=dumb is set.
//
// With --watch, the tool clears the terminal and reprints the table
// whenever the file changes on disk.
//
// With --prune (optionally --prune-to=<path>), completed tasks and
// fully-complete (sub)sections are removed from the file in place;
// --prune-to moves the removed content into another markdown file,
// merging into matching headings.
package main

import (
	"bufio"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"
	"time"
	"unicode/utf8"
)

// version is set at build time via ldflags.
var version = "dev"

// ANSI escape sequences.
const (
	ansiDim    = "\x1b[2m"
	ansiReset  = "\x1b[0m"
	ansiClear  = "\x1b[2J\x1b[H"
	ansiYellow = "\x1b[33m"
)

// nameWidth is the display width of the section-name column.
const nameWidth = 40

// useColor returns true when stdout looks like a color-capable TTY.
// Honors the NO_COLOR convention (https://no-color.org/) and the
// classic TERM=dumb opt-out.
func useColor() bool {
	if os.Getenv("NO_COLOR") != "" {
		return false
	}
	if os.Getenv("TERM") == "dumb" {
		return false
	}
	fi, err := os.Stdout.Stat()
	if err != nil {
		return false
	}
	return (fi.Mode() & os.ModeCharDevice) != 0
}

// status classifies a checklist line.
type status int

const (
	statusOpen status = iota
	statusPartial
	statusDone
)

// item is a single checklist line, preserving its original text.
type item struct {
	raw    string
	status status
}

// element is one body line under a heading: either a task or raw
// passthrough text (prose, blank line, code fence, plain bullet).
type element struct {
	task *item  // nil => raw passthrough
	raw  string // used when task == nil
}

// node is a heading-delimited section. The synthetic root has level 0
// and an empty heading.
type node struct {
	level    int // 2..6 for ## .. ######; 0 for the synthetic root
	title    string
	heading  string // raw heading line ("" for root)
	body     []element
	children []*node
	parent   *node
}

func main() {
	watch := flag.Bool("watch", false, "watch the file for changes and re-render")
	showVersion := flag.Bool("version", false, "print version and exit")
	depth := flag.Int("depth", 1, "sub-section depth to render (0-4)")
	prune := flag.Bool("prune", false, "remove completed tasks and fully-complete sections from the file in place")
	pruneTo := flag.String("prune-to", "", "move pruned tasks/sections into this markdown file")
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "proggy — markdown checklist progress reports\n\n")
		fmt.Fprintf(os.Stderr, "Usage: proggy [flags] [path]\n\n")
		fmt.Fprintf(os.Stderr, "Arguments:\n")
		fmt.Fprintf(os.Stderr, "  path    markdown file to parse (default: PROGRESS.md)\n\n")
		fmt.Fprintf(os.Stderr, "Flags (must precede the path):\n")
		flag.PrintDefaults()
	}
	flag.Parse()

	if *showVersion {
		fmt.Printf("proggy %s\n", version)
		return
	}

	path := "PROGRESS.md"
	watchMode := *watch
	for _, arg := range flag.Args() {
		if arg == "--watch" || arg == "-watch" {
			watchMode = true
		} else {
			path = arg
		}
	}

	if *depth < 0 || *depth > 4 {
		fmt.Fprintf(os.Stderr, "error: --depth must be between 0 and 4\n")
		os.Exit(1)
	}

	if *prune || *pruneTo != "" {
		if err := runPrune(path, *pruneTo); err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			os.Exit(1)
		}
		return
	}

	if watchMode {
		runWatch(path, *depth)
	} else {
		if err := render(path, *depth); err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			os.Exit(1)
		}
	}
}

// parseHeading reports whether line is a level-2..6 ATX heading and,
// if so, returns its level and trimmed title.
func parseHeading(line string) (level int, title string, ok bool) {
	i := 0
	for i < len(line) && line[i] == '#' {
		i++
	}
	if i < 2 || i > 6 {
		return 0, "", false
	}
	if i >= len(line) || line[i] != ' ' {
		return 0, "", false
	}
	return i, strings.TrimSpace(line[i+1:]), true
}

// taskStatus classifies a checklist line, reporting ok=false for
// non-task lines.
func taskStatus(line string) (status, bool) {
	trimmed := strings.TrimSpace(line)
	switch {
	case strings.HasPrefix(trimmed, "- [x]"):
		return statusDone, true
	case strings.HasPrefix(trimmed, "- [/]"),
		strings.HasPrefix(trimmed, "- [~]"),
		strings.HasPrefix(trimmed, "- [-]"):
		return statusPartial, true
	case strings.HasPrefix(trimmed, "- [ ]"):
		return statusOpen, true
	}
	return 0, false
}

// parse reads markdown into a document tree, preserving every line
// verbatim so the tree can reconstruct the file.
func parse(r io.Reader) *node {
	root := &node{level: 0}
	stack := []*node{root}

	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for scanner.Scan() {
		line := scanner.Text()

		if level, title, ok := parseHeading(line); ok {
			for len(stack) > 1 && stack[len(stack)-1].level >= level {
				stack = stack[:len(stack)-1]
			}
			parent := stack[len(stack)-1]
			n := &node{level: level, title: title, heading: line, parent: parent}
			parent.children = append(parent.children, n)
			stack = append(stack, n)
			continue
		}

		cur := stack[len(stack)-1]
		if st, ok := taskStatus(line); ok {
			cur.body = append(cur.body, element{task: &item{raw: line, status: st}})
		} else {
			cur.body = append(cur.body, element{raw: line})
		}
	}
	return root
}

// counts rolls up all descendant task tallies.
func (n *node) counts() (checked, partial, total int) {
	for _, e := range n.body {
		if e.task == nil {
			continue
		}
		total++
		switch e.task.status {
		case statusDone:
			checked++
		case statusPartial:
			partial++
		}
	}
	for _, c := range n.children {
		cc, cp, ct := c.counts()
		checked += cc
		partial += cp
		total += ct
	}
	return
}

// render parses the markdown file and prints the completion table.
func render(path string, depth int) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()

	root := parse(f)
	fmt.Print(renderDoc(root, depth, useColor()))
	return nil
}

// visibleChildren returns the children that will render: within the
// depth limit and holding at least one task in their rollup.
func visibleChildren(n *node, maxLevel int) []*node {
	var out []*node
	for _, c := range n.children {
		if c.level > maxLevel {
			continue
		}
		if _, _, total := c.counts(); total == 0 {
			continue
		}
		out = append(out, c)
	}
	return out
}

// renderDoc renders the whole document tree as a table string.
func renderDoc(root *node, depth int, color bool) string {
	maxLevel := 2 + depth

	var b strings.Builder
	sep := fmt.Sprintf("  %s %s %s %s\n",
		strings.Repeat("─", nameWidth),
		strings.Repeat("─", 6),
		strings.Repeat("─", 6),
		strings.Repeat("─", 6))

	b.WriteString("\n")
	fmt.Fprintf(&b, "  %-*s %6s %6s %6s\n", nameWidth, "Section", "Done", "Total", "%")
	b.WriteString(sep)

	visible := visibleChildren(root, maxLevel)
	for i, c := range visible {
		renderNode(&b, c, maxLevel, color, "", i == len(visible)-1, true)
	}

	b.WriteString(sep)

	totalChecked, totalPartial, totalAll := 0, 0, 0
	for _, c := range root.children {
		cc, cp, ct := c.counts()
		totalChecked += cc
		totalPartial += cp
		totalAll += ct
	}
	totalPct := 0
	if totalAll > 0 {
		totalPct = totalChecked * 100 / totalAll
	}
	bar := progressBar(totalChecked, totalPartial, totalAll, 20, color)
	fmt.Fprintf(&b, "  %-*s %4d %6d %5d%%  %s\n", nameWidth, "TOTAL", totalChecked, totalAll, totalPct, bar)
	b.WriteString("\n")

	return b.String()
}

// renderNode prints one row for n and recurses into its visible
// children, drawing box-drawing tree connectors. top marks root's
// direct children, which render with no connector.
func renderNode(b *strings.Builder, n *node, maxLevel int, color bool, childPrefix string, isLast, top bool) {
	checked, partial, total := n.counts()

	linePrefix := childPrefix
	if !top {
		if isLast {
			linePrefix += "└─ "
		} else {
			linePrefix += "├─ "
		}
	}

	pct := checked * 100 / total

	avail := max(nameWidth-utf8.RuneCountInString(linePrefix), 1)
	title := n.title
	if utf8.RuneCountInString(title) > avail {
		r := []rune(title)
		title = string(r[:avail-1]) + "…"
	}
	displayName := linePrefix + title
	if pad := nameWidth - utf8.RuneCountInString(displayName); pad > 0 {
		displayName += strings.Repeat(" ", pad)
	}

	bar := progressBar(checked, partial, total, 20, color)
	pre, suf := "", ""
	if color && pct == 100 {
		pre, suf = ansiDim, ansiReset
	}
	fmt.Fprintf(b, "  %s%s %4d %6d %5d%%  %s%s\n", pre, displayName, checked, total, pct, bar, suf)

	var nextPrefix string
	switch {
	case top:
		nextPrefix = ""
	case isLast:
		nextPrefix = childPrefix + "   "
	default:
		nextPrefix = childPrefix + "│  "
	}

	visible := visibleChildren(n, maxLevel)
	for i, c := range visible {
		renderNode(b, c, maxLevel, color, nextPrefix, i == len(visible)-1, false)
	}
}

// runWatch polls the file's modification time and re-renders the
// table whenever it changes.
func runWatch(path string, depth int) {
	lastMod := time.Time{}

	for {
		info, err := os.Stat(path)
		if err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			os.Exit(1)
		}

		mod := info.ModTime()
		if mod != lastMod {
			lastMod = mod
			fmt.Print(ansiClear)
			if err := render(path, depth); err != nil {
				fmt.Fprintf(os.Stderr, "error: %v\n", err)
				os.Exit(1)
			}
			fmt.Printf("  %sWatching %s for changes… (Ctrl-C to stop)%s\n", ansiDim, path, ansiReset)
		}

		time.Sleep(500 * time.Millisecond)
	}
}

func progressBar(checked, partial, total, width int, color bool) string {
	if total == 0 {
		return "[" + strings.Repeat("░", width) + "]"
	}
	filled := checked * width / total
	yellow := partial * width / total
	if filled+yellow > width {
		yellow = width - filled
	}
	empty := width - filled - yellow
	bar := strings.Repeat("█", filled)
	if yellow > 0 && color {
		bar += ansiYellow + strings.Repeat("█", yellow) + ansiReset
	} else {
		bar += strings.Repeat("█", yellow)
	}
	bar += strings.Repeat("░", empty)
	return "[" + bar + "]"
}

// runPrune parses path, removes completed tasks and fully-complete
// sections, optionally merges the removed content into pruneTo, then
// rewrites path in place. It is one-shot and ignores --watch.
func runPrune(path, pruneTo string) error {
	info, err := os.Stat(path)
	if err != nil {
		return err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}

	root := parse(strings.NewReader(string(data)))
	removed, tasks, sections := pruneNode(root)

	if pruneTo != "" {
		if err := mergePruneTo(pruneTo, removed); err != nil {
			return err
		}
	}

	if err := os.WriteFile(path, []byte(root.serialize()), info.Mode().Perm()); err != nil {
		return err
	}

	fmt.Printf("removed %d tasks, %d sections\n", tasks, sections)
	return nil
}

// pruneNode mutates n in place, returning a mirror tree of removed
// content plus tallies of removed tasks and section headings. Fully-
// complete child sections are detached whole; surviving sections keep
// their open/partial tasks and prose but shed completed tasks.
func pruneNode(n *node) (removed *node, tasks, sections int) {
	removed = &node{level: n.level, title: n.title, heading: n.heading}

	var kept []*node
	for _, c := range n.children {
		checked, partial, total := c.counts()
		if total > 0 && partial == 0 && checked == total {
			c.parent = removed
			removed.children = append(removed.children, c)
			tasks += total
			sections += countNodes(c)
			continue
		}
		childRemoved, ct, cs := pruneNode(c)
		kept = append(kept, c)
		tasks += ct
		sections += cs
		if len(childRemoved.body) > 0 || len(childRemoved.children) > 0 {
			childRemoved.parent = removed
			removed.children = append(removed.children, childRemoved)
		}
	}
	n.children = kept

	var keptBody []element
	for _, e := range n.body {
		if e.task != nil && e.task.status == statusDone {
			removed.body = append(removed.body, e)
			tasks++
		} else {
			keptBody = append(keptBody, e)
		}
	}
	n.body = keptBody

	return removed, tasks, sections
}

// countNodes counts n and all its descendants.
func countNodes(n *node) int {
	c := 1
	for _, ch := range n.children {
		c += countNodes(ch)
	}
	return c
}

// serialize reconstructs the markdown text for the tree rooted at n.
func (n *node) serialize() string {
	var b strings.Builder
	n.writeTo(&b)
	return b.String()
}

func (n *node) writeTo(b *strings.Builder) {
	if n.heading != "" {
		b.WriteString(n.heading)
		b.WriteString("\n")
	}
	for _, e := range n.body {
		if e.task != nil {
			b.WriteString(e.task.raw)
		} else {
			b.WriteString(e.raw)
		}
		b.WriteString("\n")
	}
	for _, c := range n.children {
		c.writeTo(b)
	}
}

// mergePruneTo merges the removed tree into the target markdown file,
// matching headings by trimmed title at each level and creating any
// that are missing. The target is created if it does not exist.
func mergePruneTo(target string, removed *node) error {
	var root *node
	mode := os.FileMode(0644)
	if info, err := os.Stat(target); err == nil {
		data, rerr := os.ReadFile(target)
		if rerr != nil {
			return rerr
		}
		root = parse(strings.NewReader(string(data)))
		mode = info.Mode().Perm()
	} else if os.IsNotExist(err) {
		root = &node{level: 0}
	} else {
		return err
	}

	mergeInto(root, removed)
	return os.WriteFile(target, []byte(root.serialize()), mode)
}

// mergeInto appends src's body into dst and merges src's children into
// dst by matching heading title at each level, creating headings as
// needed so removed content lands under the same structure.
func mergeInto(dst, src *node) {
	dst.body = append(dst.body, src.body...)
	for _, sc := range src.children {
		dc := findChild(dst, sc.level, sc.title)
		if dc == nil {
			dc = &node{level: sc.level, title: sc.title, heading: headingLine(sc), parent: dst}
			dst.children = append(dst.children, dc)
		}
		mergeInto(dc, sc)
	}
}

// findChild returns dst's child matching level and trimmed title.
func findChild(dst *node, level int, title string) *node {
	want := strings.TrimSpace(title)
	for _, c := range dst.children {
		if c.level == level && strings.TrimSpace(c.title) == want {
			return c
		}
	}
	return nil
}

// headingLine returns n's raw heading, synthesizing one if absent.
func headingLine(n *node) string {
	if n.heading != "" {
		return n.heading
	}
	return strings.Repeat("#", n.level) + " " + n.title
}
