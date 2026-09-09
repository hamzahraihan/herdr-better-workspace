// Package install manages this plugin's herdr keybinding: it writes and
// removes one delimited [[keys.command]] block in herdr's config.toml,
// so setup needs no manual editing.
package install

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

const (
	startMarker   = "# herdr-better-workspace: managed by `herdr-better-workspace install` — safe to delete, re-created on install."
	endMarker     = "# herdr-better-workspace: end."
	DefaultKey    = "prefix+space"
	DefaultWidth  = "60%"
	DefaultHeight = "70%"
)

// Options configures the managed keybinding.
type Options struct {
	Key      string // herdr key chord, e.g. prefix+space
	Width    string // popup width, e.g. 60%
	Height   string // popup height, e.g. 70%
	Exe      string // command herdr runs; "" resolves to the current binary
	HerdrBin string // herdr binary used for config reload
	DryRun   bool   // report without writing
}

// ConfigPath mirrors herdr's own resolution: $HERDR_CONFIG_PATH wins,
// otherwise the OS user-config dir + herdr/config.toml.
func ConfigPath() (string, error) {
	if v := strings.TrimSpace(os.Getenv("HERDR_CONFIG_PATH")); v != "" {
		return v, nil
	}
	dir, err := os.UserConfigDir()
	if err != nil || dir == "" {
		return "", errors.New("cannot locate config dir: set HERDR_CONFIG_PATH explicitly")
	}
	return filepath.Join(dir, "herdr", "config.toml"), nil
}

func checkField(name, v string) error {
	if strings.TrimSpace(v) == "" {
		return fmt.Errorf("%s must not be empty", name)
	}
	if strings.ContainsAny(v, "\"\n\r") {
		return fmt.Errorf("%s must not contain quotes or newlines", name)
	}
	return nil
}

// resolveExe returns the forward-slashed absolute path herdr will execute.
// Forward slashes keep the TOML double-quoted string escape-free and run
// fine through cmd.exe on Windows.
func resolveExe(exe string) (string, error) {
	if strings.TrimSpace(exe) == "" {
		self, err := os.Executable()
		if err != nil {
			return "", fmt.Errorf("resolve plugin binary: %w", err)
		}
		exe = self
	}
	abs, err := filepath.Abs(exe)
	if err != nil {
		return "", fmt.Errorf("resolve plugin binary: %w", err)
	}
	if err := checkField("exe", abs); err != nil {
		return "", err
	}
	return filepath.ToSlash(abs), nil
}

// block renders the managed TOML stanza (trailing newline included).
func block(o Options, exe string) string {
	return strings.Join([]string{
		startMarker,
		"[[keys.command]]",
		fmt.Sprintf("key = %q", o.Key),
		`type = "popup"`,
		fmt.Sprintf("command = %q", exe),
		fmt.Sprintf("width = %q", o.Width),
		fmt.Sprintf("height = %q", o.Height),
		`description = "New workspace (interactive form)"`,
		endMarker,
		"",
	}, "\n")
}

// span locates the managed block lines in content. ok=false when absent.
// An opening marker without a closing marker is a corrupt state callers
// must not silently rewrite.
func span(content string) (start, end int, ok bool, corrupt bool) {
	lines := strings.Split(content, "\n")
	s, e := -1, -1
	for i, l := range lines {
		if strings.TrimSpace(l) == startMarker {
			s = i
		} else if strings.TrimSpace(l) == endMarker && s >= 0 {
			e = i
			break
		}
	}
	if s < 0 {
		return 0, 0, false, false
	}
	if e < 0 {
		return 0, 0, false, true
	}
	return s, e, true, false
}

// splice swaps lines[s..e] for replacement, normalizing the surrounding
// blank lines so repeated installs converge byte-for-byte. An empty
// replacement deletes the span.
func splice(lines []string, s, e int, replacement string) string {
	head := strings.TrimRight(strings.Join(lines[:s], "\n"), "\n")
	rest := strings.Trim(strings.Join(lines[e+1:], "\n"), "\n")
	rep := strings.TrimRight(replacement, "\n")
	var b strings.Builder
	if head != "" {
		b.WriteString(head + "\n")
	}
	if rep != "" {
		if head != "" {
			b.WriteString("\n")
		}
		b.WriteString(rep + "\n")
	}
	if rest != "" {
		b.WriteString(rest + "\n")
	}
	return b.String()
}

// Install writes (or refreshes) the managed keybinding. It reports whether
// the file changed. With DryRun set, nothing is written.
func Install(o Options) (changed bool, path string, err error) {
	if err := checkField("key", o.Key); err != nil {
		return false, "", err
	}
	if err := checkField("width", o.Width); err != nil {
		return false, "", err
	}
	if err := checkField("height", o.Height); err != nil {
		return false, "", err
	}
	exe, err := resolveExe(o.Exe)
	if err != nil {
		return false, "", err
	}
	path, err = ConfigPath()
	if err != nil {
		return false, "", err
	}
	raw, readErr := os.ReadFile(path)
	if readErr != nil && !errors.Is(readErr, os.ErrNotExist) {
		return false, path, fmt.Errorf("read %s: %w", path, readErr)
	}
	content := string(raw)
	if content != "" {
		if _, _, _, corrupt := span(content); corrupt {
			return false, path, fmt.Errorf("%s has a start marker without an end marker — remove those lines by hand, then re-run install", path)
		}
		if !strings.HasSuffix(content, "\n") {
			content += "\n"
		}
	}
	stitched := block(o, exe)
	lines := strings.Split(content, "\n")
	var next string
	if s, e, ok, _ := span(content); ok {
		next = splice(lines, s, e, stitched)
	} else if strings.TrimSpace(content) == "" {
		next = stitched
	} else {
		next = content + "\n" + stitched
	}
	if next == content {
		return false, path, nil
	}
	if o.DryRun {
		return true, path, nil
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return false, path, fmt.Errorf("create config dir: %w", err)
	}
	if len(raw) > 0 {
		if err := os.WriteFile(path+".bak", raw, 0o644); err != nil {
			return false, path, fmt.Errorf("backup %s: %w", path, err)
		}
	}
	if err := os.WriteFile(path, []byte(next), 0o644); err != nil {
		return false, path, fmt.Errorf("write %s: %w", path, err)
	}
	return true, path, nil
}

// Uninstall removes the managed block. removed=false means there was
// nothing to do.
func Uninstall() (removed bool, path string, err error) {
	path, err = ConfigPath()
	if err != nil {
		return false, "", err
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return false, path, nil
		}
		return false, path, fmt.Errorf("read %s: %w", path, err)
	}
	content := string(raw)
	s, e, ok, corrupt := span(content)
	if corrupt {
		return false, path, fmt.Errorf("%s has a start marker without an end marker — remove those lines by hand", path)
	}
	if !ok {
		return false, path, nil
	}
	lines := strings.Split(content, "\n")
	next := splice(lines, s, e, "")
	if err := os.WriteFile(path+".bak", raw, 0o644); err != nil {
		return false, path, fmt.Errorf("backup %s: %w", path, err)
	}
	if err := os.WriteFile(path, []byte(next), 0o644); err != nil {
		return false, path, fmt.Errorf("write %s: %w", path, err)
	}
	return true, path, nil
}

// Reload asks a running herdr server to pick up config changes.
func Reload(ctx context.Context, herdrBin string) (string, error) {
	bin := strings.TrimSpace(herdrBin)
	if bin == "" {
		if v := strings.TrimSpace(os.Getenv("HERDR_BIN")); v != "" {
			bin = v
		} else {
			bin = "herdr"
		}
	}
	if ctx == nil {
		ctx = context.Background()
	}
	ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	if _, err := exec.LookPath(bin); err != nil {
		return "", fmt.Errorf("%s not found in PATH: %w", bin, err)
	}
	cmd := exec.CommandContext(ctx, bin, "server", "reload-config")
	out, err := cmd.CombinedOutput()
	return strings.TrimSpace(string(out)), err
}
