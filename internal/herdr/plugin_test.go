package herdr

import (
	"strings"
	"testing"
)

func TestParsePaneCurrentCwd(t *testing.T) {
	out := `{"id":"cli:pane:current","result":{"pane":{"cwd":"C:\\herdr-plugins-project\\herdr-better-workspace\\","pane_id":"w1V:p1"},"type":"pane_current"}}`
	got, err := parsePaneCurrentCwd(out)
	if err != nil {
		t.Fatalf("parsePaneCurrentCwd: %v", err)
	}
	if got != `C:\herdr-plugins-project\herdr-better-workspace\` {
		t.Fatalf("cwd = %q", got)
	}
}

func TestParsePaneCurrentCwdMissing(t *testing.T) {
	for _, out := range []string{
		`{}`,
		`{"result":{"pane":{}}}`,
		`not json`,
	} {
		if _, err := parsePaneCurrentCwd(out); err == nil {
			t.Fatalf("expected error for %q", out)
		}
	}
}

func TestPickerPaneArgsSeedsCwd(t *testing.T) {
	dir := t.TempDir()
	args := pickerPaneArgs(dir)
	joined := strings.Join(args, "\x00")
	if !strings.Contains(joined, "--cwd\x00"+dir) {
		t.Fatalf("args missing --cwd %q: %q", dir, args)
	}
	// Base open args preserved.
	for _, want := range []string{"plugin", "pane", "open", "--placement", "overlay", "--focus"} {
		if !strings.Contains(joined, want) {
			t.Fatalf("args missing %q: %q", want, args)
		}
	}
}

func TestPickerPaneArgsSkipsBadCwd(t *testing.T) {
	base := len(pickerPaneArgs(""))
	for _, bad := range []string{"", "   ", t.TempDir() + "/does-not-exist"} {
		if got := len(pickerPaneArgs(bad)); got != base {
			t.Fatalf("cwd %q: args = %d, want base %d", bad, got, base)
		}
	}
}
