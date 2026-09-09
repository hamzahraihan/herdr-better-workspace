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

// OpenPickerPane opens the picker overlay on the active pane and focuses it.
// Overlay placement targets the active pane by herdr semantics, so no
// workspace argument is passed.
func OpenPickerPane(ctx context.Context, herdrBin string) (string, error) {
	return runHerdr(ctx, herdrBin, 30*time.Second,
		"plugin", "pane", "open",
		"--plugin", PluginID,
		"--entrypoint", PanePicker,
		"--placement", "overlay",
		"--focus",
	)
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
