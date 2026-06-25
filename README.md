# proggy

Markdown checklist progress reports in your terminal.

```
$ proggy PROGRESS.md

  Section                                    Done  Total      %
  ──────────────────────────────────────── ────── ────── ──────
  Backend: VM Interpreter                    30     32    93%  [██████████████████░░]
  Backend: C                                 21     27    77%  [███████████████░░░░░]
  TOTAL                                      51     59    86%  [█████████████████░░░]
```

**proggy** reads any markdown file with `## Heading` sections and `- [x]` / `- [ ]` checkboxes, then prints a clean completion table with progress bars. Sub-headings (`###`–`######`) render as an indented tree, parent rows roll up all descendant tasks, and fully-complete sections are dimmed so you can focus on what's left.

## Install

### Homebrew (macOS / Linux)

```sh
brew install nickglenn/tap/proggy
```

### Scoop (Windows)

```powershell
scoop bucket add nickglenn https://github.com/nickglenn/scoop-bucket
scoop install proggy
```

### Debian / Ubuntu

Download the `.deb` from the [latest release](https://github.com/nickglenn/proggy/releases):

```sh
sudo dpkg -i proggy_*.deb
```

### Go

```sh
go install github.com/nickglenn/proggy@latest
```

## Usage

```sh
# Default: reads PROGRESS.md in the current directory
proggy

# Specify a file
proggy path/to/TODO.md

# Live-watch mode: re-renders on file changes
proggy --watch

# Control how deep sub-sections render (0–4, default 1)
proggy --depth=2 path/to/TODO.md

# Remove completed tasks and fully-complete sections in place
proggy --prune path/to/TODO.md

# Move the removed tasks/sections into another file instead of discarding
proggy --prune --prune-to=DONE.md path/to/TODO.md

# Print version
proggy --version
```

> Flags must precede the path, e.g. `proggy --depth=2 TODO.md`.

## Markdown format

proggy expects this structure:

```md
## Section Name

- [x] Completed task
- [ ] Incomplete task

### Subsection (grouped under parent section)

- [x] Another done item
- [ ] Still working on this
```

- `## Heading` starts a new top-level section row in the table
- `### Subheading` (through `######`) nest as indented child rows, drawn with a box-drawing tree (`├─`, `└─`, `│`)
- Parent rows roll up the counts of all their descendants
- `--depth=N` (0–4, default 1) sets how deep sub-sections render; deeper levels stay hidden but still count toward their parents
- `- [x]` counts as done, `- [ ]` as remaining, and `- [/]` / `- [~]` / `- [-]` as in-progress (shown as a yellow bar segment)

## Pruning

`--prune` rewrites the file in place, removing every completed task and any
(sub)section whose tasks — and all of its descendants' tasks — are complete.
Sections with open or in-progress work keep those items (and surrounding prose)
but shed their completed tasks. **This is destructive and makes no backup.**

`--prune-to=<path>` moves the removed tasks and sections into another markdown
file instead of discarding them, merging into matching headings (by title at
each level) so completed work accumulates without duplicating heading lines.

## Features

- Hierarchical section tree with rolled-up parent counts and `--depth` control
- Progress bars with filled/empty blocks
- Dimmed rows for 100% sections (respects `NO_COLOR` and `TERM=dumb`)
- `--watch` mode polls for file changes and live-updates
- `--prune` / `--prune-to` to clear or archive completed work
- Zero dependencies, single binary

## License

MIT
