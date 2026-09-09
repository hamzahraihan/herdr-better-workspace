// Command herdr-better-workspace is an interactive "Open Workspace"
// folder-picker plugin: browse directories and register the pick with
// herdr via `herdr workspace create` (or a git worktree via
// `herdr worktree create`).
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"herdr-better-workspace/internal/herdr"
	"herdr-better-workspace/internal/ui"
)

const version = "0.1.0"

func usage() {
	fmt.Fprintf(os.Stderr, `herdr-better-workspace v%s — interactive herdr workspace creator

Usage:
  herdr-better-workspace [--cwd <start-dir>] [--dry-run]
                                          browse-and-pick TUI (default)
  herdr-better-workspace --name <label> --cwd <path> [--template <id>]
      [--git/--no-git] [--focus/--no-focus] [--dry-run]

Flags:
  --name string       workspace label (with --cwd: non-interactive create)
  --cwd string        non-interactive target dir, or TUI start dir when
                      --name is omitted
  --template string   blank (default), go, node-ts, python, rust
  --git / --no-git    git init on/off (default on)
  --focus / --no-focus
                      herdr focus on/off (default on)
  --herdr-bin string  herdr binary override (default "herdr" or $HERDR_BIN)
  --dry-run           scaffold locally, skip `+"`herdr workspace create`"+`
  --version           print version and exit
  --help, -h          show this help

Examples:
  herdr-better-workspace
  herdr-better-workspace --dry-run
  herdr-better-workspace --name demo --cwd ~/Projects/demo --template go --git --focus
  herdr-better-workspace --name demo --cwd ./demo --template python --dry-run

  [[keys.command]]
  key = "prefix+space"
  type = "popup"
  command = "herdr-better-workspace"
  width = "60%%"
  height = "70%%"
`, version)
}

func main() {
	var (
		name     = flag.String("name", "", "workspace label (with --cwd: non-interactive mode)")
		cwd      = flag.String("cwd", "", "workspace directory")
		template = flag.String("template", "blank", "template id")
		gitOn    = flag.Bool("git", true, "initialize git repository")
		gitOff   = flag.Bool("no-git", false, "skip git init")
		focusOn  = flag.Bool("focus", true, "focus workspace after creation")
		focusOff = flag.Bool("no-focus", false, "do not focus workspace")
		herdrBin = flag.String("herdr-bin", "", "herdr binary override")
		dryRun   = flag.Bool("dry-run", false, "scaffold locally, skip herdr exec")
		showVer  = flag.Bool("version", false, "print version and exit")
		showHelp = flag.Bool("help", false, "show help")
		shortH   = flag.Bool("h", false, "show help")
	)
	flag.Usage = usage
	flag.Parse()

	if *showVer {
		fmt.Printf("herdr-better-workspace v%s\n", version)
		return
	}
	if *showHelp || *shortH {
		usage()
		return
	}
	if extra := flag.Args(); len(extra) > 0 {
		fmt.Fprintf(os.Stderr, "unexpected argument: %s\n\n", strings.Join(extra, " "))
		usage()
		os.Exit(2)
	}

	initGit := *gitOn && !*gitOff
	focus := *focusOn && !*focusOff

	// Non-interactive path: --name + --cwd skips the TUI entirely.
	if strings.TrimSpace(*name) != "" && strings.TrimSpace(*cwd) != "" {
		opts := herdr.CreateOptions{
			Name:       *name,
			Path:       *cwd,
			TemplateID: *template,
			InitGit:    initGit,
			Focus:      focus,
			HerdrBin:   *herdrBin,
			DryRun:     *dryRun,
		}
		res, err := herdr.Create(context.Background(), opts)
		if err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("workspace %q created at %s\n", res.Name, res.Path)
		if res.WorkspaceID != "" {
			fmt.Printf("herdr id: %s\n", res.WorkspaceID)
		}
		if out := strings.TrimSpace(res.HerdrOutput); out != "" {
			fmt.Println(out)
		}
		return
	}
	if strings.TrimSpace(*name) != "" && strings.TrimSpace(*cwd) == "" {
		fmt.Fprintln(os.Stderr, "error: --name requires --cwd for non-interactive mode")
		usage()
		os.Exit(2)
	}

	m := ui.NewModelWithDir(*dryRun, *herdrBin, *cwd)
	p := tea.NewProgram(m, tea.WithAltScreen(), tea.WithMouseAllMotion())
	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}
