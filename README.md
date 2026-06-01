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

**proggy** reads any markdown file with `## Heading` sections and `- [x]` / `- [ ]` checkboxes, then prints a clean completion table with progress bars. Fully-complete sections are dimmed so you can focus on what's left.

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

# Print version
proggy --version
```

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

- `## Heading` starts a new section row in the table
- `### Subheading` groups items under the current section
- `- [x]` counts as done, `- [ ]` counts as remaining

## Features

- Progress bars with filled/empty blocks
- Dimmed rows for 100% sections (respects `NO_COLOR` and `TERM=dumb`)
- `--watch` mode polls for file changes and live-updates
- Zero dependencies, single binary

## License

MIT
