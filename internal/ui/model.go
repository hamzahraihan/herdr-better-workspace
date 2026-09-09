// Package ui implements the "Open Workspace" folder-picker TUI: browse
// directories, pick one, and register it with herdr via os/exec.
package ui

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"

	"herdr-better-workspace/internal/herdr"
)

// Entry kinds in the picker list.
const (
	ekOpen = iota
	ekWorktree
	ekHome
	ekUp
	ekDir
	ekFile
)

type entry struct {
	kind int
	name string // display name (dirs keep a trailing separator)
	path string // absolute path
}

// Screen states.
const (
	stBrowse = iota
	stPrompt
	stCreating
	stSuccess
	stError
)

// Prompt kinds.
const (
	pkMkdir = iota
	pkBranch
)

type createFinishedMsg struct {
	result *herdr.CreateResult
	err    error
}

type worktreeFinishedMsg struct {
	cmdLine string
	output  string
	branch  string
	path    string
	err     error
}

// Model is the Bubble Tea model for the picker.
type Model struct {
	cwd        string
	entries    []entry
	cursor     int
	offset     int
	lastClick  int
	showHidden bool

	state       int
	promptKind  int
	promptTitle string
	prompt      textinput.Model
	promptErr   string
	addr        textinput.Model
	addrFocused bool
	hint        string

	spin          spinner.Model
	creatingLabel string
	creatingPath  string
	pendingCmd    string

	worktreeMode   bool
	worktreeBranch string
	result         *herdr.CreateResult
	worktreeOut    string
	createErr      error

	retryWorktree bool
	retryPath     string
	retryLabel    string
	retryBranch   string

	width, height int
	dryRun        bool
	herdrBin      string
}

// NewModel starts the picker in the process working directory.
func NewModel(dryRun bool, herdrBin string) Model {
	return NewModelWithDir(dryRun, herdrBin, "")
}

// NewModelWithDir starts the picker in startDir (falls back to cwd, then home).
func NewModelWithDir(dryRun bool, herdrBin, startDir string) Model {
	cwd := strings.TrimSpace(startDir)
	if cwd == "" {
		var err error
		cwd, err = os.Getwd()
		if err != nil || cwd == "" {
			cwd, _ = os.UserHomeDir()
		}
	}
	if abs, err := herdr.ResolvePath(cwd); err == nil {
		cwd = abs
	}
	pi := textinput.New()
	pi.Prompt = "> "
	pi.CharLimit = 1024
	pi.Width = 48
	ai := textinput.New()
	ai.Prompt = "› "
	ai.Placeholder = "Type a path, Enter to go"
	ai.CharLimit = 1024
	ai.Width = 52
	m := Model{
		cwd:       cwd,
		lastClick: -1,
		spin:      spinner.New(spinner.WithSpinner(spinner.Dot)),
		prompt:    pi,
		addr:      ai,
		dryRun:    dryRun,
		herdrBin:  herdrBin,
	}
	m.reload()
	return m
}

// Init implements tea.Model.
func (m Model) Init() tea.Cmd {
	return tea.Batch(textinput.Blink, m.spin.Tick)
}

func (m Model) bin() string {
	if m.herdrBin != "" {
		return m.herdrBin
	}
	return herdr.DefaultHerdrBin()
}

// specials returns the four pinned rows for dir.
func specials(dir string) []entry {
	home, _ := os.UserHomeDir()
	return []entry{
		{kind: ekOpen, name: "Open this folder", path: dir},
		{kind: ekWorktree, name: "Open with new worktree", path: dir},
		{kind: ekHome, name: "Home", path: home},
		{kind: ekUp, name: "..", path: filepath.Dir(dir)},
	}
}

// buildEntries lists dir as picker rows: pinned actions, then dirs, then
// files (each alpha-sorted). Pure for testability.
func buildEntries(dir string, showHidden bool) ([]entry, error) {
	out := specials(dir)
	infos, err := os.ReadDir(dir)
	if err != nil {
		return out, err
	}
	var dirs, files []entry
	for _, fi := range infos {
		n := fi.Name()
		if !showHidden && strings.HasPrefix(n, ".") {
			continue
		}
		full := filepath.Join(dir, n)
		if fi.IsDir() {
			dirs = append(dirs, entry{kind: ekDir, name: n + string(os.PathSeparator), path: full})
		} else {
			files = append(files, entry{kind: ekFile, name: n, path: full})
		}
	}
	less := func(a, b entry) bool { return strings.ToLower(a.name) < strings.ToLower(b.name) }
	sort.Slice(dirs, func(i, j int) bool { return less(dirs[i], dirs[j]) })
	sort.Slice(files, func(i, j int) bool { return less(files[i], files[j]) })
	return append(append(out, dirs...), files...), nil
}

// reload refreshes the entry list, preserving the cursor when possible.
func (m *Model) reload() {
	ents, err := buildEntries(m.cwd, m.showHidden)
	m.lastClick = -1
	if err != nil {
		m.hint = "cannot read folder: " + err.Error()
	} else if m.hint != "" && strings.HasPrefix(m.hint, "cannot read folder:") {
		m.hint = ""
	}
	m.entries = ents
	if m.cursor >= len(m.entries) {
		m.cursor = len(m.entries) - 1
	}
	if m.cursor < 0 {
		m.cursor = 0
	}
	m.clampOffset()
}

func (m *Model) chdir(dir string) {
	abs, err := herdr.ResolvePath(dir)
	if err != nil {
		m.hint = err.Error()
		return
	}
	if abs == m.cwd {
		m.hint = "already at filesystem root"
		return
	}
	st, err := os.Stat(abs)
	if err != nil || !st.IsDir() {
		m.hint = "not a directory: " + dir
		return
	}
	m.cwd = abs
	m.cursor = 0
	m.offset = 0
	m.hint = ""
	m.reload()
}

// workspaceLabel derives a herdr label from a directory path.
func workspaceLabel(dir string) string {
	clean := strings.TrimRight(strings.TrimSuffix(strings.TrimSpace(dir), ":"), `/\`)
	base := filepath.Base(clean)
	if base == "" || base == "." || base == string(filepath.Separator) {
		if vol := filepath.VolumeName(dir); vol != "" {
			return strings.TrimSuffix(vol, ":")
		}
		return "workspace"
	}
	return base
}

func (m Model) visibleCount() int {
	if m.height <= 0 {
		return 12
	}
	n := m.height - 12
	if n < 5 {
		n = 5
	}
	if n > 30 {
		n = 30
	}
	return n
}

func (m Model) innerWidth() int {
	// Follow the popup size: lipgloss adds the 2 border columns on top of
	// Width, so width-8 keeps a 1-column margin on each side.
	w := 56
	if m.width > 0 {
		w = m.width - 8
	}
	if w < 20 {
		w = 20
	}
	return w
}

// syncWidths fits the text inputs to the current box width.
func (m *Model) syncWidths() {
	w := m.innerWidth() - 4
	if w < 10 {
		w = 10
	}
	m.prompt.Width = w
	m.addr.Width = w
}

func (m *Model) clampOffset() {
	n := 12
	if m.height > 0 {
		n = m.height - 12
		if n < 5 {
			n = 5
		}
		if n > 30 {
			n = 30
		}
	}
	if m.cursor < m.offset {
		m.offset = m.cursor
	}
	if m.cursor >= m.offset+n {
		m.offset = m.cursor - n + 1
	}
	if m.offset < 0 {
		m.offset = 0
	}
}

func (m *Model) openPrompt(kind int, title, value, placeholder string) {
	m.promptKind = kind
	m.promptTitle = title
	m.promptErr = ""
	m.prompt.SetValue(value)
	m.prompt.Placeholder = placeholder
	m.prompt.CursorEnd()
	_ = m.prompt.Focus()
	m.state = stPrompt
}

// quote shell-quotes one argv word for the preview line.
func quote(s string) string {
	if strings.ContainsAny(s, " \t\"'") {
		return `"` + strings.ReplaceAll(s, `"`, `\"`) + `"`
	}
	return s
}

func (m *Model) startCreate(dir string) tea.Cmd {
	label := workspaceLabel(dir)
	if err := herdr.ValidateName(label); err != nil {
		m.hint = err.Error()
		return nil
	}
	opts := herdr.CreateOptions{
		Name: label, Path: dir,
		TemplateID: "blank", SkipScaffold: true, InitGit: false,
		Focus: true, HerdrBin: m.bin(), DryRun: m.dryRun,
	}
	m.state = stCreating
	m.worktreeMode = false
	m.creatingLabel = label
	m.creatingPath = dir
	m.pendingCmd = opts.CommandString(dir)
	m.retryWorktree = false
	m.retryPath = dir
	m.retryLabel = label
	return tea.Batch(m.spin.Tick, func() tea.Msg {
		res, err := herdr.Create(context.Background(), opts)
		return createFinishedMsg{result: res, err: err}
	})
}

func (m *Model) startWorktree(dir, branch string) tea.Cmd {
	branch = strings.TrimSpace(branch)
	if branch == "" {
		m.promptErr = "branch name is required"
		return nil
	}
	m.state = stCreating
	m.worktreeMode = true
	m.worktreeBranch = branch
	m.creatingLabel = branch
	m.creatingPath = dir
	m.pendingCmd = fmt.Sprintf("%s worktree create --cwd %s --branch %s --focus",
		m.bin(), quote(dir), quote(branch))
	m.retryWorktree = true
	m.retryPath = dir
	m.retryBranch = branch
	bin := m.bin()
	dry := m.dryRun
	return tea.Batch(m.spin.Tick, func() tea.Msg {
		cmdLine, out, err := herdr.CreateWorktree(context.Background(), bin, dir, branch, true, dry)
		return worktreeFinishedMsg{cmdLine: cmdLine, output: out, branch: branch, path: dir, err: err}
	})
}

func (m *Model) retry() tea.Cmd {
	if m.retryWorktree {
		return m.startWorktree(m.retryPath, m.retryBranch)
	}
	return m.startCreate(m.retryPath)
}

func (m *Model) activate(e entry) tea.Cmd {
	switch e.kind {
	case ekOpen:
		return m.startCreate(m.cwd)
	case ekWorktree:
		m.openPrompt(pkBranch, "New branch (worktree)", "", "feature/my-branch")
		return nil
	case ekHome:
		if e.path == "" {
			m.hint = "home directory unknown"
			return nil
		}
		m.chdir(e.path)
		return nil
	case ekUp:
		m.chdir(e.path)
		return nil
	case ekDir:
		m.chdir(e.path)
		return nil
	default: // ekFile
		m.hint = e.name + " is a file — pick a folder"
		return nil
	}
}

func (m *Model) confirmPrompt() tea.Cmd {
	v := strings.TrimSpace(m.prompt.Value())
	switch m.promptKind {
	case pkMkdir:
		if v == "" {
			m.promptErr = "folder name is required"
			return nil
		}
		if strings.ContainsAny(v, `/\`) || v == ".." || strings.Contains(v, ":") {
			m.promptErr = "invalid folder name (no slashes, .., or drive letters)"
			return nil
		}
		full := filepath.Join(m.cwd, v)
		if err := os.MkdirAll(full, 0o755); err != nil {
			m.promptErr = "cannot create folder: " + err.Error()
			return nil
		}
		m.state = stBrowse
		m.chdir(full)
		return nil
	default: // pkBranch
		if v == "" {
			m.promptErr = "branch name is required"
			return nil
		}
		return m.startWorktree(m.cwd, v)
	}
}

// Update implements tea.Model.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.syncWidths()
		m.clampOffset()
		return m, nil

	case createFinishedMsg:
		if msg.err != nil {
			m.state = stError
			m.createErr = msg.err
		} else {
			m.state = stSuccess
			m.result = msg.result
		}
		return m, nil

	case worktreeFinishedMsg:
		if msg.err != nil {
			m.state = stError
			m.createErr = msg.err
		} else {
			m.state = stSuccess
			m.worktreeMode = true
			m.worktreeBranch = msg.branch
			m.creatingPath = msg.path
			m.pendingCmd = msg.cmdLine
			m.worktreeOut = msg.output
			m.result = nil
		}
		return m, nil

	case tea.MouseMsg:
		return m, m.handleMouse(msg)

	case tea.KeyMsg:
		key := msg.String()
		switch m.state {
		case stCreating:
			if key == "ctrl+c" || key == "esc" {
				return m, tea.Quit
			}
			var cmd tea.Cmd
			m.spin, cmd = m.spin.Update(msg)
			return m, cmd

		case stSuccess:
			if key == "ctrl+c" || key == "esc" || key == "enter" || key == "q" {
				return m, tea.Quit
			}
			return m, nil

		case stError:
			switch key {
			case "ctrl+c", "esc", "q":
				return m, tea.Quit
			case "b", "backspace":
				m.state = stBrowse
				m.createErr = nil
				return m, nil
			case "r":
				m.createErr = nil
				return m, m.retry()
			}
			return m, nil

		case stPrompt:
			switch key {
			case "ctrl+c":
				return m, tea.Quit
			case "esc":
				m.state = stBrowse
				m.promptErr = ""
				return m, nil
			case "enter":
				return m, m.confirmPrompt()
			}
			var cmd tea.Cmd
			m.prompt, cmd = m.prompt.Update(msg)
			return m, cmd

		default: // stBrowse
			m.lastClick = -1
			if m.addrFocused {
				switch key {
				case "ctrl+c":
					return m, tea.Quit
				case "esc":
					m.addrFocused = false
					m.addr.Blur()
					m.addr.SetValue(m.cwd)
					m.hint = ""
					return m, nil
				case "enter":
					dest := strings.TrimSpace(m.addr.Value())
					abs, err := herdr.ResolvePath(dest)
					if err != nil {
						m.hint = err.Error()
						return m, nil
					}
					if st, err := os.Stat(abs); err != nil || !st.IsDir() {
						m.hint = "not a directory: " + dest
						return m, nil
					}
					m.addrFocused = false
					m.addr.Blur()
					m.chdir(abs)
					return m, nil
				}
				var cmd tea.Cmd
				m.addr, cmd = m.addr.Update(msg)
				return m, cmd
			}
			switch key {
			case "ctrl+c", "esc":
				return m, tea.Quit
			case "up":
				if m.cursor > 0 {
					m.cursor--
					m.clampOffset()
				}
				return m, nil
			case "down":
				if m.cursor < len(m.entries)-1 {
					m.cursor++
					m.clampOffset()
				}
				return m, nil
			case "enter":
				if len(m.entries) > 0 {
					return m, m.activate(m.entries[m.cursor])
				}
				return m, nil
			case "left", "backspace":
				m.chdir(filepath.Dir(m.cwd))
				return m, nil
			case "/":
				m.focusAddr()
				return m, nil
			case "n":
				m.openPrompt(pkMkdir, "New folder name", "", "my-folder")
				return m, nil
			case ".":
				m.showHidden = !m.showHidden
				m.reload()
				if m.showHidden {
					m.hint = "showing hidden files"
				} else {
					m.hint = ""
				}
				return m, nil
			}
			return m, nil
		}
	}

	if m.state == stCreating {
		var cmd tea.Cmd
		m.spin, cmd = m.spin.Update(msg)
		return m, cmd
	}
	if m.state == stPrompt {
		var cmd tea.Cmd
		m.prompt, cmd = m.prompt.Update(msg)
		return m, cmd
	}
	if m.state == stBrowse && m.addrFocused {
		var cmd tea.Cmd
		m.addr, cmd = m.addr.Update(msg)
		return m, cmd
	}
	return m, nil
}

// --- View helpers ---

func trunc(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	if n <= 1 {
		return string(r[:n])
	}
	return string(r[:n-1]) + "…"
}

func truncLeft(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	if n <= 1 {
		return string(r[len(r)-n:])
	}
	return "…" + string(r[len(r)-n+1:])
}

func rowLabel(e entry) string {
	switch e.kind {
	case ekOpen:
		return "✓ Open this folder"
	case ekWorktree:
		return "⎇ Open with new worktree"
	case ekHome:
		return "⌂ Home"
	case ekUp:
		return "↑ .."
	case ekDir:
		return "▪ " + e.name
	default:
		return "· " + e.name
	}
}

func (m Model) browseView() string {
	w := m.innerWidth()
	var b strings.Builder
	b.WriteString(PickerTitleStyle.Render("Open Workspace") + "\n")
	if m.addrFocused {
		b.WriteString(m.addr.View() + "\n\n")
	} else {
		b.WriteString(PickerPathStyle.Render(truncLeft(m.cwd, w)) + "\n\n")
	}

	n := m.visibleCount()
	end := m.offset + n
	if end > len(m.entries) {
		end = len(m.entries)
	}
	for i := m.offset; i < end; i++ {
		label := trunc(rowLabel(m.entries[i]), w-2)
		if i == m.cursor {
			b.WriteString(SelectedRowStyle.Width(w).Render("▸ "+label) + "\n")
		} else if m.entries[i].kind == ekFile {
			b.WriteString(FileRowStyle.Width(w).Render("  "+label) + "\n")
		} else {
			b.WriteString(DirRowStyle.Width(w).Render("  "+label) + "\n")
		}
	}
	b.WriteString("\n")
	if m.hint != "" {
		b.WriteString(WarnStyle.Render(trunc(m.hint, w)) + "\n")
	}
	b.WriteString(pickerFooter())
	return PickerBox.Width(w + 4).Render(b.String())
}

func pickerFooter() string {
	sep := HintStyle.Render(" · ")
	parts := []string{
		FooterHiStyle.Render("/") + " " + HintStyle.Render("path"),
		KeyStyle.Render("↑↓") + " " + HintStyle.Render("move"),
		KeyStyle.Render("↵") + " " + HintStyle.Render("select"),
		KeyStyle.Render("←") + " " + HintStyle.Render("up"),
		FooterHiStyle.Render("n") + " " + HintStyle.Render("new folder"),
		KeyStyle.Render(".") + " " + HintStyle.Render("hidden"),
		KeyStyle.Render("mouse") + " " + HintStyle.Render("click×2 open"),
		FooterHiStyle.Render("esc"),
	}
	return strings.Join(parts, sep)
}

func (m Model) promptView() string {
	w := m.innerWidth()
	var b strings.Builder
	b.WriteString(PickerTitleStyle.Render("Open Workspace") + "\n")
	b.WriteString(PickerPathStyle.Render(truncLeft(m.cwd, w)) + "\n\n")
	b.WriteString(LabelFocused.Render(m.promptTitle) + "\n")
	b.WriteString(FocusedBox.Render(m.prompt.View()) + "\n")
	if m.promptErr != "" {
		b.WriteString(FieldErrorStyle.Render("! "+trunc(m.promptErr, w)) + "\n")
	}
	b.WriteString("\n" + Menu("↵", "Confirm", "esc", "Back"))
	return PickerBox.Width(w + 4).Render(b.String())
}

func (m Model) creatingView() string {
	var b strings.Builder
	b.WriteString(PickerTitleStyle.Render("Open Workspace") + "\n\n")
	if m.worktreeMode {
		b.WriteString(m.spin.View() + " Creating worktree " +
			CodeStyle.Render(m.worktreeBranch) + " …\n\n")
	} else {
		b.WriteString(m.spin.View() + " Opening workspace " +
			CodeStyle.Render(m.creatingLabel) + " …\n\n")
	}
	b.WriteString(HintStyle.Render("› "+truncLeft(m.pendingCmd, 80)) + "\n\n")
	b.WriteString(Menu("Esc", "Abort"))
	return b.String()
}

func (m Model) successWorkspaceView() string {
	r := m.result
	var b strings.Builder
	b.WriteString(PickerTitleStyle.Render("Open Workspace") + "\n\n")
	b.WriteString(SuccessStyle.Render("✓ Workspace opened") + "\n\n")
	lines := []string{"Path: " + r.Path, "Label: " + r.Name}
	if r.WorkspaceID != "" {
		lines = append(lines, "herdr id: "+r.WorkspaceID)
	}
	if r.DryRun {
		lines = append(lines, "Mode: dry-run (herdr not called)")
	}
	lines = append(lines, "", "$ "+r.CommandLine)
	if out := strings.TrimSpace(r.HerdrOutput); out != "" {
		if len(out) > 800 {
			out = out[:800] + "…"
		}
		lines = append(lines, "", "--- herdr output ---", out)
	}
	b.WriteString(SuccessBox.Render(strings.Join(lines, "\n")) + "\n")
	b.WriteString("\n" + Menu("Enter", "Done", "Esc", "Quit"))
	return b.String()
}

func (m Model) successWorktreeView() string {
	var b strings.Builder
	b.WriteString(PickerTitleStyle.Render("Open Workspace") + "\n\n")
	b.WriteString(SuccessStyle.Render("✓ Worktree created") + "\n\n")
	lines := []string{"Branch: " + m.worktreeBranch, "Path: " + m.creatingPath}
	lines = append(lines, "", "$ "+m.pendingCmd)
	if out := strings.TrimSpace(m.worktreeOut); out != "" {
		if len(out) > 800 {
			out = out[:800] + "…"
		}
		lines = append(lines, "", "--- herdr output ---", out)
	}
	b.WriteString(SuccessBox.Render(strings.Join(lines, "\n")) + "\n")
	b.WriteString("\n" + Menu("Enter", "Done", "Esc", "Quit"))
	return b.String()
}

func (m Model) errorView() string {
	var b strings.Builder
	b.WriteString(PickerTitleStyle.Render("Open Workspace") + "\n\n")
	b.WriteString(ErrorStyle.Render("✗ Failed") + "\n\n")
	msg := ""
	if m.createErr != nil {
		msg = m.createErr.Error()
	}
	if len(msg) > 1200 {
		msg = msg[:1200] + "…"
	}
	b.WriteString(FailureBox.Render(msg) + "\n")
	b.WriteString("\n" + Menu("r", "Retry", "b", "Back", "Esc", "Quit"))
	return b.String()
}

// View implements tea.Model.
func (m Model) View() string {
	switch m.state {
	case stPrompt:
		return m.promptView()
	case stCreating:
		return m.creatingView()
	case stSuccess:
		if m.worktreeMode {
			return m.successWorktreeView()
		}
		return m.successWorkspaceView()
	case stError:
		return m.errorView()
	default:
		return m.browseView()
	}
}

// rowAtY maps a screen Y coordinate to an entry index. In the browse view
// rows always start at line 4: border, title, path, blank, then entries.
// It returns -1 for coordinates outside the list.
func (m Model) rowAtY(y int) int {
	idx := y - 4 + m.offset
	if idx < 0 || idx >= len(m.entries) {
		return -1
	}
	return idx
}

// focusAddr focuses the path bar prefilled with the current directory.
func (m *Model) focusAddr() {
	m.addrFocused = true
	m.addr.SetValue(m.cwd)
	m.promptErr = ""
	m.hint = ""
	m.addr.CursorEnd()
	_ = m.addr.Focus()
}

// handleMouse implements hover-highlight, click-select / second-click-open,
// wheel scroll, and path-bar focus. Coordinates assume the view renders at
// the screen origin (true inside herdr popups and alt-screen terminals).
func (m *Model) handleMouse(msg tea.MouseMsg) tea.Cmd {
	if m.state == stSuccess {
		if msg.Action == tea.MouseActionPress && msg.Button == tea.MouseButtonLeft {
			return tea.Quit
		}
		return nil
	}
	if m.state != stBrowse {
		return nil
	}
	if msg.Button == tea.MouseButtonWheelUp || msg.Button == tea.MouseButtonWheelDown {
		switch msg.Button {
		case tea.MouseButtonWheelUp:
			m.cursor -= 3
		case tea.MouseButtonWheelDown:
			m.cursor += 3
		default:
			return nil
		}
		if m.cursor < 0 {
			m.cursor = 0
		}
		if m.cursor > len(m.entries)-1 {
			m.cursor = len(m.entries) - 1
		}
		m.clampOffset()
		return nil
	}
	if msg.Action == tea.MouseActionMotion {
		if m.addrFocused || msg.Button != tea.MouseButtonNone {
			return nil
		}
		if idx := m.rowAtY(msg.Y); idx >= 0 && idx != m.cursor {
			m.cursor = idx
			m.clampOffset()
		}
		return nil
	}
	if msg.Action != tea.MouseActionPress || msg.Button != tea.MouseButtonLeft {
		return nil
	}
	if m.addrFocused {
		m.addrFocused = false
		m.addr.Blur()
		m.addr.SetValue(m.cwd)
		m.hint = ""
		return nil
	}
	if msg.Y == 2 {
		m.focusAddr()
		return nil
	}
	idx := m.rowAtY(msg.Y)
	if idx < 0 {
		return nil
	}
	if idx == m.cursor && idx == m.lastClick {
		m.lastClick = -1
		return m.activate(m.entries[idx])
	}
	m.cursor = idx
	m.lastClick = idx
	m.clampOffset()
	return nil
}
