package ui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func keyMsg(s string) tea.KeyMsg {
	switch s {
	case "enter":
		return tea.KeyMsg{Type: tea.KeyEnter}
	case "esc":
		return tea.KeyMsg{Type: tea.KeyEscape}
	}
	return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(s)}
}
func TestFluidWidthFillsPopup(t *testing.T) {
	root := t.TempDir()
	m := NewModelWithDir(false, "herdr", root)
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 100, Height: 30})
	m = updated.(Model)
	if got := m.innerWidth(); got != 92 {
		t.Fatalf("innerWidth = %d, want 92 (terminal - 8)", got)
	}
	first := strings.SplitN(m.View(), "\n", 2)[0]
	if got := len([]rune(first)); got != 98 {
		t.Fatalf("box width = %d, want 98 (terminal - 2)", got)
	}
}

func TestAddressBarJump(t *testing.T) {
	root := t.TempDir()
	sub := filepath.Join(root, "proj")
	if err := os.MkdirAll(sub, 0o755); err != nil {
		t.Fatal(err)
	}
	m := NewModelWithDir(false, "herdr", root)

	// "/" focuses the path bar prefilled with cwd.
	updated, _ := m.Update(keyMsg("/"))
	m = updated.(Model)
	if !m.addrFocused {
		t.Fatal("address bar not focused after /")
	}
	if m.addr.Value() != m.cwd {
		t.Fatalf("bar = %q, want cwd %q", m.addr.Value(), m.cwd)
	}

	// Typing appends; enter on a bad path keeps focus with a hint.
	updated, _ = m.Update(keyMsg("x"))
	m = updated.(Model)
	updated, _ = m.Update(keyMsg("enter"))
	m = updated.(Model)
	if !m.addrFocused {
		t.Fatal("bar should stay focused on invalid path")
	}
	if m.hint == "" {
		t.Fatal("expected hint for invalid path")
	}

	// Valid path jumps there and unfocuses.
	m.addr.SetValue(sub)
	updated, _ = m.Update(keyMsg("enter"))
	m = updated.(Model)
	if m.addrFocused {
		t.Fatal("bar should unfocus after successful jump")
	}
	if m.cwd != sub {
		t.Fatalf("cwd = %q, want %q", m.cwd, sub)
	}
}

func mouseClick(x, y int) tea.MouseMsg {
	return tea.MouseMsg{X: x, Y: y, Action: tea.MouseActionPress, Button: tea.MouseButtonLeft}
}

func mouseMove(x, y int) tea.MouseMsg {
	return tea.MouseMsg{X: x, Y: y, Action: tea.MouseActionMotion, Button: tea.MouseButtonNone}
}

func mouseWheel(y int, down bool) tea.MouseMsg {
	btn := tea.MouseButtonWheelUp
	if down {
		btn = tea.MouseButtonWheelDown
	}
	return tea.MouseMsg{X: 5, Y: y, Action: tea.MouseActionPress, Button: btn}
}

func mouseFixture(t *testing.T) (Model, string) {
	t.Helper()
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "proj"), 0o755); err != nil {
		t.Fatal(err)
	}
	return NewModelWithDir(false, "herdr", root), root
}

func TestMouseHoverHighlights(t *testing.T) {
	m, _ := mouseFixture(t)
	// entries[4] is "proj/", rendered at screen row 8 (rows start at line 4).
	updated, _ := m.Update(mouseMove(5, 8))
	m = updated.(Model)
	if m.cursor != 4 {
		t.Fatalf("cursor = %d, want 4 after hover", m.cursor)
	}
}

func TestMouseClickSelectsThenOpens(t *testing.T) {
	m, root := mouseFixture(t)
	updated, _ := m.Update(mouseClick(5, 8))
	m = updated.(Model)
	if m.cursor != 4 {
		t.Fatalf("cursor = %d, want 4 after first click", m.cursor)
	}
	if m.cwd != root {
		t.Fatalf("first click must only select, cwd = %q", m.cwd)
	}
	updated, _ = m.Update(mouseClick(5, 8))
	m = updated.(Model)
	if m.cwd != filepath.Join(root, "proj") {
		t.Fatalf("second click must open, cwd = %q", m.cwd)
	}
}

func TestMouseWheelScrolls(t *testing.T) {
	m, _ := mouseFixture(t)
	updated, _ := m.Update(mouseWheel(6, true))
	m = updated.(Model)
	if m.cursor != 3 {
		t.Fatalf("cursor = %d, want 3 after wheel down", m.cursor)
	}
	updated, _ = m.Update(mouseWheel(6, false))
	m = updated.(Model)
	if m.cursor != 0 {
		t.Fatalf("cursor = %d, want 0 after wheel up", m.cursor)
	}
}

func TestMouseClickPathBarFocuses(t *testing.T) {
	m, _ := mouseFixture(t)
	updated, _ := m.Update(mouseClick(10, 2))
	m = updated.(Model)
	if !m.addrFocused {
		t.Fatal("clicking the path line should focus the path bar")
	}
}
