// proggy parses markdown checklist files and prints a per-section
// completion table to the terminal.
//
// Usage: proggy [--watch] [path]
//
// Rows that are fully complete (100%) are dimmed via ANSI escape
// sequences so the eye can skip them and focus on sections that
// still have open work. Dimming is suppressed when stdout is not a
// TTY or when NO_COLOR / TERM=dumb is set.
//
// With --watch, the tool clears the terminal and reprints the table
// whenever the file changes on disk.
package main

import (
	"bufio"
	"flag"
	"fmt"
	"os"
	"strings"
	"time"
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

type section struct {
	name    string
	checked int
	partial int
	total   int
}

func main() {
	watch := flag.Bool("watch", false, "watch the file for changes and re-render")
	showVersion := flag.Bool("version", false, "print version and exit")
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "proggy — markdown checklist progress reports\n\n")
		fmt.Fprintf(os.Stderr, "Usage: proggy [flags] [path]\n\n")
		fmt.Fprintf(os.Stderr, "Arguments:\n")
		fmt.Fprintf(os.Stderr, "  path    markdown file to parse (default: PROGRESS.md)\n\n")
		fmt.Fprintf(os.Stderr, "Flags:\n")
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

	if watchMode {
		runWatch(path)
	} else {
		if err := render(path); err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			os.Exit(1)
		}
	}
}

// render parses the markdown file and prints the completion table.
func render(path string) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()

	var sections []section
	var current *section

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Text()

		if strings.HasPrefix(line, "## ") && !strings.HasPrefix(line, "### ") {
			name := strings.TrimPrefix(line, "## ")
			sections = append(sections, section{name: name})
			current = &sections[len(sections)-1]
			continue
		}

		if strings.HasPrefix(line, "### ") {
			continue
		}

		if current == nil {
			continue
		}

		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "- [x]") {
			current.checked++
			current.total++
		} else if strings.HasPrefix(trimmed, "- [/]") ||
			strings.HasPrefix(trimmed, "- [~]") ||
			strings.HasPrefix(trimmed, "- [-]") {
			current.partial++
			current.total++
		} else if strings.HasPrefix(trimmed, "- [ ]") {
			current.total++
		}
	}

	totalChecked := 0
	totalPartial := 0
	totalAll := 0
	color := useColor()

	fmt.Println()
	fmt.Printf("  %-40s %6s %6s %6s\n", "Section", "Done", "Total", "%")
	fmt.Printf("  %s %s %s %s\n",
		strings.Repeat("─", 40),
		strings.Repeat("─", 6),
		strings.Repeat("─", 6),
		strings.Repeat("─", 6))

	for _, s := range sections {
		if s.total == 0 {
			continue
		}
		pct := 0
		if s.total > 0 {
			pct = s.checked * 100 / s.total
		}
		bar := progressBar(s.checked, s.partial, s.total, 20, color)
		prefix, suffix := "", ""
		if color && pct == 100 {
			prefix, suffix = ansiDim, ansiReset
		}
		fmt.Printf("  %s%-40s %4d %6d %5d%%  %s%s\n", prefix, s.name, s.checked, s.total, pct, bar, suffix)
		totalChecked += s.checked
		totalPartial += s.partial
		totalAll += s.total
	}

	fmt.Printf("  %s %s %s %s\n",
		strings.Repeat("─", 40),
		strings.Repeat("─", 6),
		strings.Repeat("─", 6),
		strings.Repeat("─", 6))

	totalPct := 0
	if totalAll > 0 {
		totalPct = totalChecked * 100 / totalAll
	}
	bar := progressBar(totalChecked, totalPartial, totalAll, 20, color)
	fmt.Printf("  %-40s %4d %6d %5d%%  %s\n", "TOTAL", totalChecked, totalAll, totalPct, bar)
	fmt.Println()

	return nil
}

// runWatch polls the file's modification time and re-renders the
// table whenever it changes.
func runWatch(path string) {
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
			if err := render(path); err != nil {
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
