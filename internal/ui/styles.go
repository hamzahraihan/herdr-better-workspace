package ui

import (
	"github.com/charmbracelet/lipgloss"
)

// Palette + reusable styles. Boring, high-contrast, readable on dark/light.
var (
	accent     = lipgloss.Color("#7C3AED")
	accentDim  = lipgloss.Color("#6D28D9")
	muted      = lipgloss.Color("#6B7280")
	good       = lipgloss.Color("#22C55E")
	bad        = lipgloss.Color("#EF4444")
	warn       = lipgloss.Color("#F59E0B")
	borderIdle = lipgloss.Color("#374151")
)

var (
	TitleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#FFFFFF")).
			Background(accent).
			Padding(0, 2)

	SubtitleStyle = lipgloss.NewStyle().
			Foreground(muted)

	FocusedBox = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(accent).
			Padding(0, 1)

	BlurredBox = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(borderIdle).
			Padding(0, 1)

	ErrorBox = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(bad).
			Padding(0, 1)

	LabelFocused = lipgloss.NewStyle().Bold(true).Foreground(accent)
	LabelBlurred = lipgloss.NewStyle().Bold(true).Foreground(muted)
	LabelError   = lipgloss.NewStyle().Bold(true).Foreground(bad)

	FieldErrorStyle = lipgloss.NewStyle().Foreground(bad)
	HintStyle       = lipgloss.NewStyle().Foreground(muted)
	DescStyle       = lipgloss.NewStyle().Foreground(muted).Italic(true)

	SuccessStyle = lipgloss.NewStyle().Bold(true).Foreground(good)
	ErrorStyle   = lipgloss.NewStyle().Bold(true).Foreground(bad)
	WarnStyle    = lipgloss.NewStyle().Bold(true).Foreground(warn)

	ButtonFocused = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#FFFFFF")).
			Background(accentDim).
			Padding(0, 3)
	ButtonBlurred = lipgloss.NewStyle().
			Foreground(muted).
			Border(lipgloss.RoundedBorder()).
			BorderForeground(borderIdle).
			Padding(0, 3)

	KeyStyle  = lipgloss.NewStyle().Bold(true).Foreground(accent)
	DescKey   = lipgloss.NewStyle().Foreground(muted)
	MenuBar   = lipgloss.NewStyle().Foreground(muted).Padding(1, 0, 0, 0)
	CodeStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#E5E7EB"))

	SuccessBox = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(good).
			Padding(1, 2)

	FailureBox = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(bad).
			Padding(1, 2)
)

// Menu renders the bottom keybinding bar: pairs of [key] description.
func Menu(pairs ...string) string {
	// pairs: key, desc, key, desc, ...
	out := ""
	for i := 0; i+1 < len(pairs); i += 2 {
		if i > 0 {
			out += "   "
		}
		out += KeyStyle.Render("["+pairs[i]+"]") + " " + DescKey.Render(pairs[i+1])
	}
	return MenuBar.Render(out)
}

var (
	// PickerBox frames the Open Workspace popup (square border like herdr).
	PickerBox = lipgloss.NewStyle().
			Border(lipgloss.NormalBorder()).
			BorderForeground(borderIdle).
			Padding(0, 1)

	PickerTitleStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#FFFFFF"))
	PickerPathStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("#C4B5FD"))

	// SelectedRowStyle is the full-width purple cursor row.
	SelectedRowStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(lipgloss.Color("#FFFFFF")).
				Background(lipgloss.Color("#6D28D9"))

	DirRowStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("#E5E7EB"))
	FileRowStyle = lipgloss.NewStyle().Foreground(muted)

	// FooterHiStyle marks the standout footer keys (g/n/esc).
	FooterHiStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#F87171"))
)
