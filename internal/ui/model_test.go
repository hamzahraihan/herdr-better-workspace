package ui

import (
	"os"
	"path/filepath"
	"testing"
)

func mkTree(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	for _, d := range []string{"api", "apps", ".hidden-dir"} {
		if err := os.MkdirAll(filepath.Join(root, d), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	for _, f := range []string{"AGENTS.md", "zeta.txt", ".env"} {
		if err := os.WriteFile(filepath.Join(root, f), []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return root
}

func TestBuildEntriesOrderAndSections(t *testing.T) {
	root := mkTree(t)
	ents, err := buildEntries(root, false)
	if err != nil {
		t.Fatalf("buildEntries: %v", err)
	}
	// 4 pinned rows first.
	for i, want := range []int{ekOpen, ekWorktree, ekHome, ekUp} {
		if ents[i].kind != want {
			t.Fatalf("row %d kind = %d, want %d", i, ents[i].kind, want)
		}
	}
	rest := ents[4:]
	// Dirs first (alpha), then files (alpha); hidden excluded.
	var got []string
	for _, e := range rest {
		got = append(got, e.name)
	}
	want := []string{"api" + string(os.PathSeparator), "apps" + string(os.PathSeparator), "AGENTS.md", "zeta.txt"}
	if len(got) != len(want) {
		t.Fatalf("entries = %q, want %q", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("entries = %q, want %q", got, want)
		}
	}
}

func TestBuildEntriesHidden(t *testing.T) {
	root := mkTree(t)
	ents, err := buildEntries(root, true)
	if err != nil {
		t.Fatal(err)
	}
	seen := map[string]bool{}
	for _, e := range ents[4:] {
		seen[e.name] = true
	}
	if !seen[".env"] || !seen[".hidden-dir"+string(os.PathSeparator)] {
		t.Fatalf("hidden entries missing with showHidden=true: %v", seen)
	}
}

func TestBuildEntriesMissingDir(t *testing.T) {
	ents, err := buildEntries(filepath.Join(t.TempDir(), "nope"), false)
	if err == nil {
		t.Fatal("expected error for missing dir")
	}
	if len(ents) != 4 {
		t.Fatalf("expected 4 pinned rows on error, got %d", len(ents))
	}
}

func TestWorkspaceLabel(t *testing.T) {
	if got := workspaceLabel("/x/projects/inkstream"); got != "inkstream" {
		t.Fatalf("got %q", got)
	}
	if got := workspaceLabel(`C:\javascript-projects\inkstream`); got != "inkstream" {
		t.Fatalf("got %q", got)
	}
	if got := workspaceLabel(""); got != "workspace" {
		t.Fatalf("empty dir should fall back, got %q", got)
	}
}
