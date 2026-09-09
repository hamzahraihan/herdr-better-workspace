// Package herdr executes workspace creation via the herdr CLI (os/exec),
// with local directory scaffolding and optional git initialization.
package herdr

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"herdr-better-workspace/internal/config"
)

const defaultTimeout = 60 * time.Second

// CreateOptions captures the workspace settings collected by the TUI.
type CreateOptions struct {
	Name         string   // herdr --label value
	Path         string   // herdr --cwd value (workspace directory)
	TemplateID   string   // config.Template ID used for scaffolding
	InitGit      bool     // run `git init` in Path
	SkipScaffold bool     // open existing folder as-is (no template files)
	Focus        bool     // pass --focus (true) or --no-focus (false)
	Env          []string // optional KEY=VALUE entries forwarded via --env
	HerdrBin     string   // herdr binary override (default: "herdr")
	DryRun       bool     // scaffold locally but skip the herdr exec call
}

// CreateResult is the observable outcome rendered on the success screen.
type CreateResult struct {
	Name           string
	Path           string
	TemplateID     string
	GitInitialized bool
	Focus          bool
	DryRun         bool
	HerdrOutput    string
	WorkspaceID    string
	CommandLine    string
}

// DefaultHerdrBin returns the herdr binary to invoke.
func DefaultHerdrBin() string {
	if v := os.Getenv("HERDR_BIN"); v != "" {
		return v
	}
	return "herdr"
}

// EffectiveBin resolves the binary for these options.
func (o CreateOptions) EffectiveBin() string {
	if o.HerdrBin != "" {
		return o.HerdrBin
	}
	return DefaultHerdrBin()
}

// ResolvePath expands ~ and env vars, cleans, and absolutizes p.
func ResolvePath(p string) (string, error) {
	p = strings.TrimSpace(p)
	if p == "" {
		return "", errors.New("path is empty")
	}
	if strings.ContainsRune(p, '\x00') {
		return "", errors.New("path contains NUL byte")
	}
	if strings.HasPrefix(p, "~") {
		home, err := os.UserHomeDir()
		if err != nil || home == "" {
			return "", errors.New("cannot expand ~: home directory unknown")
		}
		if p == "~" {
			p = home
		} else if strings.HasPrefix(p, "~/") || strings.HasPrefix(p, `~\`) {
			p = filepath.Join(home, p[2:])
		}
	}
	p = os.ExpandEnv(p)
	abs, err := filepath.Abs(p)
	if err != nil {
		return "", fmt.Errorf("resolve path: %w", err)
	}
	return filepath.Clean(abs), nil
}

// ValidateName checks the workspace label.
func ValidateName(name string) error {
	name = strings.TrimSpace(name)
	if name == "" {
		return errors.New("workspace name is required")
	}
	if len(name) > 64 {
		return errors.New("workspace name must be 64 characters or fewer")
	}
	if strings.ContainsRune(name, '\x00') {
		return errors.New("workspace name contains invalid character")
	}
	for _, r := range name {
		if r == '\n' || r == '\r' || r == '\t' {
			return errors.New("workspace name must be a single line")
		}
	}
	return nil
}

// ValidatePath checks the directory input (existence is NOT required;
// Create will MkdirAll the resolved path).
func ValidatePath(p string) error {
	trimmed := strings.TrimSpace(p)
	if trimmed == "" {
		return errors.New("directory path is required")
	}
	if strings.ContainsRune(trimmed, '\x00') {
		return errors.New("directory path contains invalid character")
	}
	// Reject obviously illegal Windows characters in the final element only
	// when running on Windows is unnecessary — keep validation portable and
	// let the OS be the authority at MkdirAll time.
	if len(trimmed) > 32767 {
		return errors.New("directory path is too long")
	}
	return nil
}

// ValidateTemplateID ensures the template exists.
func ValidateTemplateID(id string) error {
	if id == "" {
		return errors.New("template is required")
	}
	for _, t := range config.AvailableTemplates() {
		if t.ID == id {
			return nil
		}
	}
	return fmt.Errorf("unknown template %q", id)
}

// Validate checks all options at once.
func Validate(o CreateOptions) error {
	if err := ValidateName(o.Name); err != nil {
		return err
	}
	if err := ValidatePath(o.Path); err != nil {
		return err
	}
	if err := ValidateTemplateID(o.TemplateID); err != nil {
		return err
	}
	for _, kv := range o.Env {
		if !strings.Contains(kv, "=") {
			return fmt.Errorf("env entry %q must be KEY=VALUE", kv)
		}
	}
	return nil
}

// Args builds the `herdr workspace create` argv (without the binary).
func (o CreateOptions) Args(resolvedPath string) []string {
	args := []string{"workspace", "create", "--label", strings.TrimSpace(o.Name), "--cwd", resolvedPath}
	for _, kv := range o.Env {
		args = append(args, "--env", kv)
	}
	if o.Focus {
		args = append(args, "--focus")
	} else {
		args = append(args, "--no-focus")
	}
	return args
}

// CommandString renders the full command line for display / dry-run.
func (o CreateOptions) CommandString(resolvedPath string) string {
	parts := append([]string{o.EffectiveBin()}, o.Args(resolvedPath)...)
	quoted := make([]string, 0, len(parts))
	for _, p := range parts {
		if strings.ContainsAny(p, " \t\"'") {
			quoted = append(quoted, `"`+strings.ReplaceAll(p, `"`, `\"`)+`"`)
		} else {
			quoted = append(quoted, p)
		}
	}
	return strings.Join(quoted, " ")
}

// safeModuleName derives a filesystem/module-safe token for template expansion.
func safeModuleName(name string) string {
	s := strings.ToLower(strings.TrimSpace(name))
	var b strings.Builder
	prevDash := false
	for _, r := range s {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			b.WriteRune(r)
			prevDash = false
		case r == '-' || r == '_' || r == ' ' || r == '.' || r == '/':
			if !prevDash && b.Len() > 0 {
				b.WriteByte('-')
				prevDash = true
			}
		}
	}
	out := strings.Trim(b.String(), "-")
	if out == "" {
		out = "workspace"
	}
	return out
}

// scaffold writes template files ({{WORKSPACE_NAME}} expanded) without overwriting.
func scaffold(dir, templateID, workspaceName string) error {
	var tmpl *config.Template
	for _, t := range config.AvailableTemplates() {
		if t.ID == templateID {
			t := t
			tmpl = &t
			break
		}
	}
	if tmpl == nil {
		return fmt.Errorf("unknown template %q", templateID)
	}
	safe := safeModuleName(workspaceName)
	for rel, body := range tmpl.Files {
		// Guard against zip-slip style entries in template definitions.
		clean := filepath.Clean(filepath.FromSlash(rel))
		if clean == "." || strings.HasPrefix(clean, "..") || filepath.IsAbs(clean) {
			return fmt.Errorf("template %q has unsafe path %q", templateID, rel)
		}
		full := filepath.Join(dir, clean)
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			return fmt.Errorf("create directory %s: %w", filepath.Dir(full), err)
		}
		if _, err := os.Stat(full); err == nil {
			continue // never overwrite user content
		}
		content := strings.ReplaceAll(body, "{{WORKSPACE_NAME}}", safe)
		rawName := strings.ReplaceAll(workspaceName, "{{WORKSPACE_NAME}}", workspaceName)
		_ = rawName
		if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
			return fmt.Errorf("write %s: %w", full, err)
		}
	}
	return nil
}

// initGit runs `git init` inside dir. A pre-existing repo is a no-op success.
func initGit(ctx context.Context, dir string) error {
	if _, err := os.Stat(filepath.Join(dir, ".git")); err == nil {
		return nil
	}
	gitBin, err := exec.LookPath("git")
	if err != nil {
		return errors.New("git binary not found in PATH (uncheck git init or install git)")
	}
	cmd := exec.CommandContext(ctx, gitBin, "init")
	cmd.Dir = dir
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		msg := strings.TrimSpace(stderr.String())
		if msg == "" {
			msg = err.Error()
		}
		return fmt.Errorf("git init: %s", msg)
	}
	return nil
}

// tryExtractWorkspaceID best-effort parses the workspace id from herdr JSON output.
func tryExtractWorkspaceID(output string) string {
	dec := json.NewDecoder(strings.NewReader(output))
	var root map[string]any
	if err := dec.Decode(&root); err != nil {
		return ""
	}
	res, _ := root["result"].(map[string]any)
	if res == nil {
		return ""
	}
	// Documented shape: .result.workspace (+ .result.tab, .result.root_pane).
	if ws, _ := res["workspace"].(map[string]any); ws != nil {
		for _, k := range []string{"workspace_id", "id"} {
			if v, _ := ws[k].(string); v != "" {
				return v
			}
		}
	}
	for _, k := range []string{"workspace_id", "id"} {
		if v, _ := res[k].(string); v != "" {
			return v
		}
	}
	return ""
}

// Create validates, scaffolds the directory, optionally git-inits, then calls herdr.
func Create(ctx context.Context, o CreateOptions) (*CreateResult, error) {
	if err := Validate(o); err != nil {
		return nil, err
	}
	if ctx == nil {
		ctx = context.Background()
	}
	ctx, cancel := context.WithTimeout(ctx, defaultTimeout)
	defer cancel()

	name := strings.TrimSpace(o.Name)
	resolved, err := ResolvePath(o.Path)
	if err != nil {
		return nil, fmt.Errorf("invalid path: %w", err)
	}
	if err := os.MkdirAll(resolved, 0o755); err != nil {
		return nil, fmt.Errorf("create directory %s: %w", resolved, err)
	}
	if !o.SkipScaffold {
		if err := scaffold(resolved, o.TemplateID, name); err != nil {
			return nil, err
		}
	}
	gitDone := false
	if o.InitGit {
		if err := initGit(ctx, resolved); err != nil {
			return nil, err
		}
		gitDone = true
	}

	cmdLine := o.CommandString(resolved)
	if o.DryRun {
		return &CreateResult{
			Name: name, Path: resolved, TemplateID: o.TemplateID,
			GitInitialized: gitDone, Focus: o.Focus, DryRun: true,
			CommandLine: cmdLine,
			HerdrOutput: "(dry-run: herdr command not executed)",
		}, nil
	}

	bin := o.EffectiveBin()
	if _, err := exec.LookPath(bin); err != nil {
		return nil, fmt.Errorf("%s binary not found in PATH: %w\nresolved command: %s", bin, err, cmdLine)
	}
	cmd := exec.CommandContext(ctx, bin, o.Args(resolved)...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		combined := strings.TrimSpace(stdout.String() + "\n" + stderr.String())
		if combined == "" {
			combined = err.Error()
		}
		return nil, fmt.Errorf("herdr workspace create failed: %s\n--- output ---\n%s", err, combined)
	}
	out := strings.TrimSpace(stdout.String())
	if out == "" {
		out = strings.TrimSpace(stderr.String())
	}
	return &CreateResult{
		Name: name, Path: resolved, TemplateID: o.TemplateID,
		GitInitialized: gitDone, Focus: o.Focus,
		HerdrOutput: out, WorkspaceID: tryExtractWorkspaceID(out),
		CommandLine: cmdLine,
	}, nil
}

// CreateWorktree runs `herdr worktree create --cwd <dir> --branch <branch>`
// for the "Open with new worktree" picker row. It returns the raw herdr
// output (JSON) for the success screen. An empty branch is rejected before
// any exec call; a non-repo cwd surfaces herdr's own error verbatim.
func CreateWorktree(ctx context.Context, herdrBin, dir, branch string, focus, dryRun bool) (cmdLine, output string, err error) {
	branch = strings.TrimSpace(branch)
	if branch == "" {
		return "", "", errors.New("branch name is required")
	}
	resolved, err := ResolvePath(dir)
	if err != nil {
		return "", "", fmt.Errorf("invalid path: %w", err)
	}
	bin := herdrBin
	if bin == "" {
		bin = DefaultHerdrBin()
	}
	args := []string{"worktree", "create", "--cwd", resolved, "--branch", branch}
	if focus {
		args = append(args, "--focus")
	} else {
		args = append(args, "--no-focus")
	}
	cmdLine = bin + " " + strings.Join(quoteArgs(args), " ")
	if dryRun {
		return cmdLine, "(dry-run: herdr command not executed)", nil
	}
	if ctx == nil {
		ctx = context.Background()
	}
	ctx, cancel := context.WithTimeout(ctx, defaultTimeout)
	defer cancel()
	if _, err := exec.LookPath(bin); err != nil {
		return cmdLine, "", fmt.Errorf("%s binary not found in PATH: %w\nresolved command: %s", bin, err, cmdLine)
	}
	cmd := exec.CommandContext(ctx, bin, args...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		combined := strings.TrimSpace(stdout.String() + "\n" + stderr.String())
		if combined == "" {
			combined = err.Error()
		}
		return cmdLine, "", fmt.Errorf("herdr worktree create failed: %s\n--- output ---\n%s", err, combined)
	}
	out := strings.TrimSpace(stdout.String())
	if out == "" {
		out = strings.TrimSpace(stderr.String())
	}
	return cmdLine, out, nil
}

// quoteArgs quotes argv entries containing whitespace for display.
func quoteArgs(args []string) []string {
	quoted := make([]string, 0, len(args))
	for _, p := range args {
		if strings.ContainsAny(p, " \t\"'") {
			quoted = append(quoted, `"`+strings.ReplaceAll(p, `"`, `\"`)+`"`)
		} else {
			quoted = append(quoted, p)
		}
	}
	return quoted
}
