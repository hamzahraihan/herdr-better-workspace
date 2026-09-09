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
	"herdr-better-workspace/internal/install"
	"herdr-better-workspace/internal/ui"
)

const version = "0.1.0"

func usage() {
	fmt.Fprintf(os.Stderr, `herdr-better-workspace v%s — interactive herdr workspace creator

Usage:
  herdr-better-workspace install [--key <chord>] [--dry-run] [--auto]
                                          register the herdr keybinding (default: prefix+space)
  herdr-better-workspace uninstall       remove the herdr keybinding
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
  type = "plugin_action"
  command = "herdr-better-workspace.open-workspace-picker"
  description = "Open Workspace"
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
	if args := flag.Args(); len(args) > 0 {
		switch args[0] {
		case "install", "uninstall":
			os.Exit(runSetupCmd(args[0], args[1:], *herdrBin))
		case "open-picker":
			os.Exit(runOpenPicker(args[1:], *herdrBin))
		default:
			fmt.Fprintf(os.Stderr, "unexpected argument: %s\n\n", strings.Join(args, " "))
			usage()
			os.Exit(2)
		}
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

// runSetupCmd implements `install` (register the herdr keybinding from the
// running binary's own path) and `uninstall` (remove it). Reload failures
// only warn: the binding still applies on next herdr launch.
func runSetupCmd(cmd string, args []string, herdrBin string) int {
	fs := flag.NewFlagSet(cmd, flag.ContinueOnError)
	key := fs.String("key", install.DefaultKey, "herdr key chord for the popup")
	dry := fs.Bool("dry-run", false, "report without writing")
	auto := fs.Bool("auto", false, "quiet setup for plugin build/startup hooks (warn instead of failing)")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if fs.NArg() > 0 {
		fmt.Fprintf(os.Stderr, "unexpected argument: %s\n\n", strings.Join(fs.Args(), " "))
		usage()
		return 2
	}
	if cmd == "uninstall" {
		removed, path, err := install.Uninstall()
		if err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			return 1
		}
		if !removed {
			fmt.Printf("no managed keybinding in %s (nothing to do)\n", path)
			return 0
		}
		fmt.Printf("removed keybinding from %s\n", path)
		if out, err := install.Reload(context.Background(), herdrBin); err != nil {
			fmt.Fprintf(os.Stderr, "warning: config reloaded on next launch (%v)\n", err)
		} else if out != "" {
			fmt.Printf("herdr: %s\n", out)
		}
		return 0
	}
	changed, path, err := install.Install(install.Options{
		Key: *key, HerdrBin: herdrBin, DryRun: *dry,
	})
	if err != nil {
		if *auto {
			fmt.Fprintf(os.Stderr, "warning: herdr-better-workspace install: %v\n", err)
			return 0
		}
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		return 1
	}
	if *auto {
		if _, err := install.Reload(context.Background(), herdrBin); err != nil {
			fmt.Fprintf(os.Stderr, "warning: herdr-better-workspace install: %v\n", err)
		}
		return 0
	}
	if *dry {
		if changed {
			fmt.Printf("would register [%s] in %s\n", *key, path)
		} else {
			fmt.Printf("already registered in %s\n", path)
		}
		return 0
	}
	if changed {
		fmt.Printf("registered [%s] keybinding in %s (backup: %s.bak)\n", *key, path, path)
	} else {
		fmt.Printf("already registered in %s\n", path)
	}
	if _, err := install.Reload(context.Background(), herdrBin); err != nil {
		fmt.Fprintf(os.Stderr, "warning: keybinding applies on next herdr launch (%v)\n", err)
	} else {
		fmt.Println("herdr config reloaded — press your key to open the picker")
	}
	return 0
}

// runOpenPicker is the headless entry point herdr invokes as the
// open-workspace-picker action: it opens the picker overlay on the active
// pane and exits. No TUI runs here — actions execute without a TTY.
func runOpenPicker(args []string, herdrBin string) int {
	fs := flag.NewFlagSet("open-picker", flag.ContinueOnError)
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if fs.NArg() > 0 {
		fmt.Fprintf(os.Stderr, "unexpected argument: %s\n\n", strings.Join(fs.Args(), " "))
		return 2
	}
	out, err := herdr.OpenPickerPane(context.Background(), herdrBin)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		return 1
	}
	if out != "" {
		fmt.Println(out)
	}
	return 0
}
