# herdr-better-workspace

An interactive **"Open Workspace" picker plugin** for [`herdr`](https://herdr.dev),
the terminal workspace manager for AI coding agents.

Instead of memorizing CLI flags (`herdr workspace create --label … --cwd …`),
press a key, browse to a folder, and open it as a workspace — with mouse,
keyboard, or a typed path. Built with Go and the Charm stack
(`bubbletea`, `bubbles`, `lipgloss`); it drives `herdr` through `os/exec`,
so it works with any installed herdr server.

```text
┌──────────────────────────────────────────────────────────────┐
│ Open Workspace                                               │
│ C:\javascript-projects\inkstream                             │
│                                                              │
│ ▸ ✓ Open this folder                                         │
│   ⎇ Open with new worktree                                   │
│   ⌂ Home                                                     │
│   ↑ ..                                                       │
│   ▪ api\                                                     │
│   ▪ apps\                                                    │
│   · AGENTS.md                                                │
│                                                              │
│ / path · ↑↓ move · ↵ select · ← up · n new folder · . hidden │
│ · mouse click×2 open · esc                                   │
└──────────────────────────────────────────────────────────────┘
```

## Features

- 📂 Folder browser with pinned actions, directories, and files
- 🖱️ Full mouse support: hover highlight, click-select, click-again to open, wheel scroll
- ⌨️ Keyboard-first: arrows, `↵` select, `←` up, `/` path bar, `n` new folder, `.` hidden files
- 🔍 Path bar — type or paste any directory (`~` and env vars expand) and jump straight to it
- 🌿 Open a folder as a workspace, or create a Git worktree from it (branch prompt included)
- 🛡️ Never touches existing contents: no scaffolding or `git init` in picker mode
- 🤖 Scriptable non-interactive mode (`--name` + `--cwd`) with project templates
- 📐 Fluid layout: the box fills whatever popup size you configure

## Requirements

| Requirement | Notes |
|---|---|
| `herdr` on `PATH` | Any recent version with `workspace create` over the socket API |
| A running herdr server | The plugin talks to your current session (`herdr status` to check) |
| `git` on `PATH` | Only for the **Open with new worktree** row |
| Go 1.24+ | Only to build from source |
| A mouse-capable terminal | Only for mouse support; keyboard works everywhere |

## Installation

### 1. Build the binary

```powershell
git clone <this-repo>
cd herdr-better-workspace
go mod tidy
go build -o herdr-better-workspace.exe ./cmd/herdr-better-workspace
```

Cross-compile examples:

```powershell
$env:GOOS="linux";   $env:GOARCH="amd64"; go build -o herdr-better-workspace-linux ./cmd/herdr-better-workspace
$env:GOOS="darwin";  $env:GOARCH="arm64"; go build -o herdr-better-workspace-mac   ./cmd/herdr-better-workspace
```

### 2. Put it on `PATH`

Either copy it next to `herdr.exe` (`%LOCALAPPDATA%\Programs\Herdr\bin`),
or keep it in your project folder (it resolves by absolute path in the
keybinding below).

### 3. Register the keybinding (automatic)

Run the plugin's installer — it appends a managed `[[keys.command]]` block
to your herdr `config.toml` (backing it up as `config.toml.bak`), using the
binary's own path so it works regardless of `PATH`. Re-running it only
updates the block, never duplicates it:

```powershell
.\herdr-better-workspace.exe install
```

This binds `prefix+space` to a 60% × 70% popup and hot-reloads the running
herdr server. Customize with flags:

```powershell
.\herdr-better-workspace.exe install --key prefix+alt+n --width 70% --height 80%
.\herdr-better-workspace.exe install --dry-run   # preview without writing
.\herdr-better-workspace.exe uninstall           # remove the block again
```

Manual alternative: herdr has no plugin manifest — a "plugin" is any
executable, optionally bound to a key. Copy the block from
`.\herdr-better-workspace.exe --help` into `%APPDATA%\herdr\config.toml`
(or wherever `$HERDR_CONFIG_PATH` points), then run
`herdr server reload-config`.

### 4. Verify

```powershell
.\herdr-better-workspace.exe --version
```

Press `<prefix>` then `Space` inside herdr: the picker opens as a modal.

## How to use

### Opening a workspace

1. Launch the picker (`prefix+space`, or run the binary in a terminal).
2. Browse to the folder you want (it starts in the current directory, or pass `--cwd <dir>` to start elsewhere).
3. Highlight **✓ Open this folder** and press `↵` (or double-click it).
4. Watch the spinner while `herdr workspace create --label <folder> --cwd <dir> --focus` runs.
5. The success screen shows the path, the herdr workspace id, and the raw output. `↵` dismisses.

The folder label defaults to the directory name. Nothing inside the folder is
created, modified, or git-initialized.

### Creating a worktree

Highlight **⎇ Open with new worktree**, press `↵`, type a branch name, confirm.
Runs `herdr worktree create --cwd <dir> --branch <name> --focus`. The folder
must be a Git checkout — otherwise herdr reports an error on the failure screen
(`r` retries, `b` goes back).

### Keyboard reference

| Keys | Action |
|---|---|
| `↑` / `↓` | Move selection |
| `↵` | Select: open folder / descend / jump home / go up / confirm |
| `←` / `Backspace` | Up one directory |
| `/` | Path bar: type or paste a directory, `↵` jumps, `esc` cancels |
| `n` | New folder: create it and descend into it |
| `.` | Toggle hidden (dot) files |
| `Esc` / `Ctrl+C` | Quit (`b` back, `r` retry on the failure screen) |

### Mouse reference

| Gesture | Action |
|---|---|
| Hover | Highlights the row under the cursor |
| Click a row | Selects it |
| Click the selected row again | Opens it (same as `↵`) |
| Wheel | Scrolls 3 rows |
| Click the path line | Focuses the path bar |
| Click while typing a path | Cancels the path bar |
| Click the success screen | Dismisses it |

Prompts, the spinner screen, and the failure screen stay keyboard-driven.

### Scriptable mode (no TUI)

Passing `--name` **and** `--cwd` skips the picker and creates directly.
Unlike picker mode, this path scaffolds a starter template and can `git init`:

```powershell
herdr-better-workspace --name demo --cwd ~/Projects/demo --template go --focus
herdr-better-workspace --name demo --cwd ./demo --template python --no-git --no-focus
herdr-better-workspace --name demo --cwd ./demo --template rust --dry-run
```

Templates: `blank`, `go`, `node-ts`, `python`, `rust`.
Template files expand `{{WORKSPACE_NAME}}` and never overwrite existing files.

## Configuration

All flags (also visible via `--help`):

| Flag | Default | Meaning |
|---|---|---|
| `--cwd <dir>` | current dir | Non-interactive target, or picker start dir when `--name` is omitted |
| `--name <label>` | — | With `--cwd`: non-interactive creation |
| `--template <id>` | `blank` | Non-interactive scaffold template |
| `--git` / `--no-git` | on | Non-interactive `git init` |
| `--focus` / `--no-focus` | on | Focus the workspace after creation |
| `--herdr-bin <path>` | `herdr` / `$HERDR_BIN` | herdr binary override |
| `--dry-run` | off | Scaffold locally, print the command, skip herdr |
| `--version`, `--help` | — | — |

## Troubleshooting

| Symptom | Fix |
|---|---|
| `herdr binary not found in PATH` | Install herdr, or pass `--herdr-bin` / set `$HERDR_BIN` |
| `herdr workspace create failed` | Is the server running? Check `herdr status` |
| Worktree row fails on a plain folder | That folder isn't a Git checkout — use **Open this folder**, or `git init` there first |
| Clicks land on the wrong row | The UI assumes it renders at the popup's top-left; report it with your terminal + popup size |
| Mouse does nothing | Your terminal may not report mouse events; keyboard works everywhere |
| `error: --name requires --cwd` | Non-interactive mode needs both flags together |

After editing `config.toml` by hand, always run `herdr server reload-config`
and check its `diagnostics` for syntax errors.

## Development

```powershell
go vet ./...
go test -count=1 ./...
```

- `cmd/herdr-better-workspace/main.go` — CLI entry point
- `internal/config/config.go` — starter templates, home helpers
- `internal/herdr/client.go` — validation, scaffolding, `git init`, herdr/worktree exec
- `internal/herdr/client_test.go` — validation, argv shape, no-touch-open guard
- `internal/ui/model.go` — Bubble Tea picker (`Init`/`Update`/`View`)
- `internal/ui/*_test.go` — ordering, labels, fluid width, path bar, mouse gestures
- `internal/ui/styles.go` — lipgloss theme
