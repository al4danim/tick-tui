# tick

A lazygit-style narrow TUI for a markdown todo file. No server, no database — just `tasks.md` in your filesystem (or your Obsidian vault).

Companion: [tick-obsidian](https://github.com/al4danim/tick-obsidian) — same file, native Obsidian UI, works on mobile.

```
┌──────────────────────────────────────┐
│ Today                          3/8   │
│                                      │
│ Pending                              │
│   buy milk @home                     │
│   write report @work                 │
│   call mom                           │
│                                      │
│ Done today                           │
│   [x] gym                            │
│   [x] groceries @home                │
│                                      │
│ a add · t done · e edit · ? help     │
└──────────────────────────────────────┘
```

## Install

### Homebrew (Mac / Linux)

```sh
brew tap al4danim/tick
brew install tick
```

### Pre-built binary

Grab the right archive from [Releases](https://github.com/al4danim/tick-tui/releases) and drop the `tick` binary into your `$PATH`.

### From source

```sh
go install github.com/al4danim/tick-tui/cmd/tick@latest
```

## Usage

```sh
tick                         # first run shows a setup wizard
tick --version
```

On first launch a wizard offers to put `tasks.md` inside an Obsidian vault (auto-detected) or at the default `~/.tick/tasks.md`. The choice is saved to `~/.config/tick/config`:

```
TICK_TASKS_FILE=/path/you/picked/.tick/tasks.md
```

`archive.md` is auto-created in the same directory. Edit the config file directly to relocate later.

## File format

One task per line, position-insensitive when parsed:

```
- [ ] buy milk @home +2026-05-01 [a3k7m2x9]
- [x] write report @work +2026-04-29 *2026-04-30 [b1d4e5f0]
```

| token | meaning |
|---|---|
| `- [ ]` / `- [x]` | status |
| description | task title (CJK ok) |
| `@project` | optional project |
| `+YYYY-MM-DD` | created date |
| `*YYYY-MM-DD` | done date (only when `[x]`) |
| `[hex]` | 8-char hex ID, **must be at end of line** |

`[ID]` is filled in automatically — handwriting `- [ ] buy milk @home` and saving works; `tick` adds the date and ID on next launch.

## Why two files

```
<your-tasks-dir>/
  tasks.md    ← undone + last 7 days of done — always small (< 50 KB)
  archive.md  ← older done rows; append-only
```

Mark-done is in-place (just toggles `[x]`); rows older than 7 days roll into `archive.md` automatically.

## Keys

| key | action |
|---|---|
| `j` `k` ↑↓ | move (`5j` = down 5) |
| `[` `]` | jump prev / next project |
| `g` `G` | first / last in section |
| `t` | mark done (3s undo with `u`) |
| `U` | un-tick a done row |
| `a` | add (streams: Enter saves & opens next; Esc to stop) |
| `e` | edit current row |
| `D` | delete |
| `y` | copy title to clipboard |
| `p` | toggle project filter |
| `?` | help |
| `q` | quit |

External edits to `tasks.md` (mobile sync, Obsidian, manual edit) are picked up automatically — no manual refresh needed.

## License

MIT
