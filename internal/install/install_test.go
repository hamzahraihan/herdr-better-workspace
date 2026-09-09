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
	opts := Options{Key: DefaultKey, Exe: "/bin/hbw", SkipLinkCheck: true}
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
	for _, want := range []string{
		`key = "prefix+space"`,
		`type = "plugin_action"`,
		`command = "herdr-better-workspace.open-workspace-picker"`,
	} {
		if !strings.Contains(content, want) {
			t.Fatalf("block missing %q:\n%s", want, content)
		}
	}
	if strings.Contains(content, "width =") {
		t.Fatalf("plugin_action block must not carry popup sizes:\n%s", content)
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
	opts := Options{Key: DefaultKey, Exe: "/bin/hbw", SkipLinkCheck: true}
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
		{Key: "", Exe: "/bin/hbw", SkipLinkCheck: true},
		{Key: "a\"b", Exe: "/bin/hbw", SkipLinkCheck: true},
	} {
		if _, _, err := Install(opts); err == nil {
			t.Fatalf("expected error for %+v", opts)
		}
	}
}

func TestUninstallRoundTrip(t *testing.T) {
	path := withConfig(t, "[ui]\n")
	opts := Options{Key: DefaultKey, Exe: "/bin/hbw", SkipLinkCheck: true}
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

func TestInstallAdoptsLegacyBlock(t *testing.T) {
	legacy := "[ui]\n\n# herdr-better-workspace: interactive workspace creator.\n[[keys.command]]\nkey = \"prefix+space\"\ntype = \"popup\"\ncommand = \"C:/old/path.exe\"\nwidth = \"90%\"\nheight = \"90%\"\ndescription = \"New workspace (interactive form)\"\n"
	path := withConfig(t, legacy)
	opts := Options{Key: DefaultKey, Exe: "/bin/hbw", SkipLinkCheck: true}
	changed, _, err := Install(opts)
	if err != nil {
		t.Fatalf("Install: %v", err)
	}
	if !changed {
		t.Fatal("legacy block should be adopted")
	}
	raw, _ := os.ReadFile(path)
	content := string(raw)
	if strings.Contains(content, "C:/old/path.exe") {
		t.Fatalf("legacy command survived:\n%s", content)
	}
	if !strings.Contains(content, `type = "plugin_action"`) {
		t.Fatalf("adopted block must use plugin_action form:\n%s", content)
	}
	if n := countManaged(t, path); n != 1 {
		t.Fatalf("managed blocks = %d, want 1", n)
	}
	if changed, _, err := Install(opts); err != nil || changed {
		t.Fatalf("changed=%v err=%v, want no-op", changed, err)
	}
}

func TestInstallRefusesForeignKey(t *testing.T) {
	foreign := "[[keys.command]]\nkey = \"prefix+space\"\ntype = \"plugin_action\"\ncommand = \"someone-else.action\"\n"
	withConfig(t, foreign)
	opts := Options{Key: DefaultKey, Exe: "/bin/hbw", SkipLinkCheck: true}
	if _, _, err := Install(opts); err == nil {
		t.Fatal("expected key-conflict error")
	} else if !strings.Contains(err.Error(), "--key") {
		t.Fatalf("error should suggest --key: %v", err)
	}
}
