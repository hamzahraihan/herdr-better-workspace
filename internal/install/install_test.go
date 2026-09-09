package install

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func withConfig(t *testing.T, initial string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "config.toml")
	t.Setenv("HERDR_CONFIG_PATH", path)
	if initial != "" {
		if err := os.WriteFile(path, []byte(initial), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return path
}

func countManaged(t *testing.T, path string) int {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	n := 0
	for _, l := range strings.Split(string(raw), "\n") {
		if strings.TrimSpace(l) == startMarker {
			n++
		}
	}
	return n
}

func TestInstallFreshAndIdempotent(t *testing.T) {
	path := withConfig(t, "[ui]\n")
	opts := Options{Key: DefaultKey, Width: DefaultWidth, Height: DefaultHeight, Exe: "/bin/hbw"}
	changed, got, err := Install(opts)
	if err != nil {
		t.Fatalf("Install: %v", err)
	}
	if !changed || got != path {
		t.Fatalf("changed=%v path=%q", changed, got)
	}
	raw, _ := os.ReadFile(path)
	content := string(raw)
	if !strings.Contains(content, "[ui]") {
		t.Fatal("existing content lost")
	}
	for _, want := range []string{`key = "prefix+space"`, `width = "60%"`} {
		if !strings.Contains(content, want) {
			t.Fatalf("block missing %q:\n%s", want, content)
		}
	}
	// Exe is absolutized for herdr; only the tail is stable across machines.
	if !strings.Contains(content, `bin/hbw"`) {
		t.Fatalf("block missing exe path:\n%s", content)
	}
	// Second run must converge: no change, single block.
	changed, _, err = Install(opts)
	if err != nil {
		t.Fatalf("re-Install: %v", err)
	}
	if changed {
		t.Fatal("second install should be a no-op")
	}
	if n := countManaged(t, path); n != 1 {
		t.Fatalf("managed blocks = %d, want 1", n)
	}
}

func TestInstallRefreshesKey(t *testing.T) {
	path := withConfig(t, "")
	opts := Options{Key: DefaultKey, Width: DefaultWidth, Height: DefaultHeight, Exe: "/bin/hbw"}
	if _, _, err := Install(opts); err != nil {
		t.Fatal(err)
	}
	opts.Key = "prefix+alt+n"
	changed, _, err := Install(opts)
	if err != nil {
		t.Fatal(err)
	}
	if !changed {
		t.Fatal("key change should rewrite the block")
	}
	raw, _ := os.ReadFile(path)
	if !strings.Contains(string(raw), `key = "prefix+alt+n"`) {
		t.Fatalf("key not updated:\n%s", raw)
	}
	if n := countManaged(t, path); n != 1 {
		t.Fatalf("managed blocks = %d, want 1", n)
	}
}

func TestInstallRejectsBadFields(t *testing.T) {
	withConfig(t, "")
	for _, opts := range []Options{
		{Key: "", Width: "60%", Height: "70%", Exe: "/bin/hbw"},
		{Key: "a\"b", Width: "60%", Height: "70%", Exe: "/bin/hbw"},
		{Key: "prefix+x", Width: "", Height: "70%", Exe: "/bin/hbw"},
	} {
		if _, _, err := Install(opts); err == nil {
			t.Fatalf("expected error for %+v", opts)
		}
	}
}

func TestUninstallRoundTrip(t *testing.T) {
	path := withConfig(t, "[ui]\n")
	opts := Options{Key: DefaultKey, Width: DefaultWidth, Height: DefaultHeight, Exe: "/bin/hbw"}
	if _, _, err := Install(opts); err != nil {
		t.Fatal(err)
	}
	removed, _, err := Uninstall()
	if err != nil {
		t.Fatal(err)
	}
	if !removed {
		t.Fatal("expected removal")
	}
	raw, _ := os.ReadFile(path)
	if !strings.Contains(string(raw), "[ui]") || strings.Contains(string(raw), "keys.command") {
		t.Fatalf("bad state after uninstall:\n%s", raw)
	}
	removed, _, err = Uninstall()
	if err != nil || removed {
		t.Fatalf("second uninstall: removed=%v err=%v", removed, err)
	}
}

func TestUninstallMissingFile(t *testing.T) {
	withConfig(t, "")
	removed, _, err := Uninstall()
	if err != nil || removed {
		t.Fatalf("removed=%v err=%v", removed, err)
	}
}
