// Plugin protocol surface: identifiers plus the headless launcher that
// opens the picker overlay. herdr executes action commands without a TTY,
// so the action only drives herdr CLI calls; the interactive TUI lives in
// the overlay pane the launcher opens.
package herdr

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"
)

const (
	// PluginID matches herdr-plugin.toml and the install keybinding.
	PluginID = "herdr-better-workspace"
	// ActionOpenPicker is the [[actions]] id in the manifest.
	ActionOpenPicker = "open-workspace-picker"
	// PanePicker is the [[panes]] overlay entrypoint in the manifest.
	PanePicker = "workspace-picker"
)

// ActionRef is the `plugin_action` command reference used in keybindings:
// "<plugin-id>.<action-id>". Location-independent, so reinstalls (which
// land in fresh hash-suffixed directories) never stale it.
func ActionRef() string {
	return PluginID + "." + ActionOpenPicker
}

// ResolveHerdrBin picks the herdr binary: explicit override, then
// HERDR_BIN_PATH (what herdr directs plugins at), HERDR_BIN, then PATH.
func ResolveHerdrBin(override string) string {
	if v := strings.TrimSpace(override); v != "" {
		return v
	}
	for _, env := range []string{"HERDR_BIN_PATH", "HERDR_BIN"} {
		if v := strings.TrimSpace(os.Getenv(env)); v != "" {
			return v
		}
	}
	return "herdr"
}

// runHerdr executes herdr with a timeout, returning trimmed combined output.
func runHerdr(ctx context.Context, herdrBin string, timeout time.Duration, args ...string) (string, error) {
	bin := ResolveHerdrBin(herdrBin)
	if ctx == nil {
		ctx = context.Background()
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	if _, err := exec.LookPath(bin); err != nil {
		return "", fmt.Errorf("%s not found in PATH: %w", bin, err)
	}
	cmd := exec.CommandContext(ctx, bin, args...)
	var buf bytes.Buffer
	cmd.Stdout = &buf
	cmd.Stderr = &buf
	if err := cmd.Run(); err != nil {
		out := strings.TrimSpace(buf.String())
		if out == "" {
			out = err.Error()
		}
		return "", fmt.Errorf("herdr %s failed: %s\n--- output ---\n%s", strings.Join(args, " "), err, out)
	}
	return strings.TrimSpace(buf.String()), nil
}

// parsePaneCurrentCwd extracts .result.pane.cwd from `herdr pane current`
// output. Split out for testability; the CLI wraps the pane object in a
// result envelope ({"result":{"pane":{"cwd":"..."},"type":"pane_current"}}).
func parsePaneCurrentCwd(out string) (string, error) {
	var doc struct {
		Result struct {
			Pane struct {
				Cwd string `json:"cwd"`
			} `json:"pane"`
		} `json:"result"`
	}
	if err := json.Unmarshal([]byte(out), &doc); err != nil {
		return "", fmt.Errorf("parse pane current output: %w", err)
	}
	if cwd := strings.TrimSpace(doc.Result.Pane.Cwd); cwd != "" {
		return cwd, nil
	}
	return "", fmt.Errorf("pane current output missing .result.pane.cwd")
}

// ActivePaneCwd reports the focused pane's working directory — i.e. the
// current workspace location the picker should start in. It must run before
// the picker overlay takes focus; afterwards `pane current` would report the
// picker pane itself.
func ActivePaneCwd(ctx context.Context, herdrBin string) (string, error) {
	out, err := runHerdr(ctx, herdrBin, 10*time.Second, "pane", "current")
	if err != nil {
		return "", err
	}
	return parsePaneCurrentCwd(out)
}

// pickerPaneArgs builds the `herdr plugin pane open` argv. originCwd seeds
// the overlay process working directory so the picker's default path starts
// at the current workspace location; empty skips --cwd (prior behavior).
func pickerPaneArgs(originCwd string) []string {
	args := []string{
		"plugin", "pane", "open",
		"--plugin", PluginID,
		"--entrypoint", PanePicker,
		"--placement", "overlay",
		"--focus",
	}
	if cwd := strings.TrimSpace(originCwd); cwd != "" {
		if info, err := os.Stat(cwd); err == nil && info.IsDir() {
			args = append(args, "--cwd", cwd)
		}
	}
	return args
}

// OpenPickerPane opens the picker overlay on the active pane and focuses it.
// Overlay placement targets the active pane by herdr semantics, so no
// workspace argument is passed. The active pane's cwd is captured first and
// passed as the overlay's working directory, so the picker starts at the
// current workspace location instead of the plugin host directory. Cwd
// detection never fails the open: on any error it falls back to the
// previous behavior (no --cwd, picker falls back to its own Getwd).
func OpenPickerPane(ctx context.Context, herdrBin string) (string, error) {
	var originCwd string
	if cwd, err := ActivePaneCwd(ctx, herdrBin); err == nil {
		originCwd = cwd
	}
	args := pickerPaneArgs(originCwd)
	withCwd := len(args) > len(pickerPaneArgs(""))
	out, err := runHerdr(ctx, herdrBin, 30*time.Second, args...)
	if err != nil && withCwd {
		msg := strings.ToLower(err.Error())
		if strings.Contains(msg, "unknown") || strings.Contains(msg, "unexpected") ||
			strings.Contains(msg, "flag") || strings.Contains(msg, "--cwd") {
			return runHerdr(ctx, herdrBin, 30*time.Second, pickerPaneArgs("")...)
		}
	}
	return out, err
}

// Exec runs an arbitrary herdr command (install tooling uses it for
// link/list).
func Exec(ctx context.Context, herdrBin string, args ...string) (string, error) {
	return runHerdr(ctx, herdrBin, 30*time.Second, args...)
}

// Registered reports whether PluginID appears in `herdr plugin list --json`.
func Registered(ctx context.Context, herdrBin string) (bool, error) {
	out, err := runHerdr(ctx, herdrBin, 15*time.Second, "plugin", "list", "--json")
	if err != nil {
		return false, err
	}
	var v any
	if err := json.Unmarshal([]byte(out), &v); err != nil {
		return strings.Contains(out, `"`+PluginID+`"`), nil
	}
	return walkForPluginID(v), nil
}

func walkForPluginID(v any) bool {
	switch t := v.(type) {
	case map[string]any:
		for k, val := range t {
			if k == "plugin_id" || k == "id" {
				if s, ok := val.(string); ok && s == PluginID {
					return true
				}
			}
			if walkForPluginID(val) {
				return true
			}
		}
	case []any:
		for _, e := range t {
			if walkForPluginID(e) {
				return true
			}
		}
	}
	return false
}
