package setup

import (
	"fmt"
	"os"
	"path/filepath"
	gostr "strings"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type itemKind int

const (
	kindVault itemKind = iota
	kindDefault
	kindCustom
)

type item struct {
	kind  itemKind
	label string // shown in listing (vault name or fixed label)
	path  string // resolved absolute path; empty for kindCustom (input first)
}

type mode int

const (
	modePick mode = iota
	modeCustom
)

// INVARIANT: every code path that returns tea.Quit MUST also set
// either m.chosen != "" (success) or m.quitRequested = true (cancellation)
// before returning. The parent TUI's handleSettingsKey relies on these
// signals to distinguish "wizard finished, do something" from
// "wizard wants to kill the host app". Violating this invariant will
// cause settings-mode ctrl+c / esc to terminate the entire tick app.
//
// See TestWizard_QuitInvariant which enforces this invariant via black-box
// testing of all known input paths.
//
// Language toggle: in modePick l flips EN↔ZH (consistent with the main TUI's
// l binding). Tab no longer toggles language. In modeCustom l passes through
// to the textinput so users can type paths like ~/local/foo; to change language
// press Esc back to modePick first.
type Model struct {
	lang   Lang
	mode   mode
	cursor int
	items  []item

	custom    textinput.Model
	customErr string

	chosen        string // populated on successful pick; "" if quit without choosing
	quitRequested bool   // user pressed ctrl+c (parent should NOT propagate tea.Quit)
	width         int
	height        int
}

// NewModel builds the wizard. Pass the detected vaults; pass an empty slice
// to hide the vault options (e.g., Obsidian not installed).
func NewModel(lang Lang, vaults []Vault) Model {
	home, _ := os.UserHomeDir()
	defaultPath := filepath.Join(home, "tick", "tasks.md")

	items := make([]item, 0, len(vaults)+2)
	for _, v := range vaults {
		items = append(items, item{
			kind:  kindVault,
			label: v.Name,
			path:  filepath.Join(v.Path, "tick", "tasks.md"),
		})
	}
	items = append(items,
		item{kind: kindDefault, path: defaultPath},
		item{kind: kindCustom},
	)

	ti := textinput.New()
	ti.Placeholder = ""
	ti.CharLimit = 512
	ti.Width = 50

	return Model{
		lang:   lang,
		items:  items,
		custom: ti,
	}
}

func (m Model) Init() tea.Cmd { return nil }

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil
	case tea.KeyMsg:
		if m.mode == modeCustom {
			return m.updateCustom(msg)
		}
		return m.updatePick(msg)
	}
	return m, nil
}

var (
	keyUp    = key.NewBinding(key.WithKeys("up", "k"))
	keyDown  = key.NewBinding(key.WithKeys("down", "j"))
	keyEnter = key.NewBinding(key.WithKeys("enter"))
	keyLang  = key.NewBinding(key.WithKeys("l"))
	keyEsc   = key.NewBinding(key.WithKeys("esc"))
	keyQuit  = key.NewBinding(key.WithKeys("ctrl+c"))
)

func (m Model) toggleLang() Model {
	if m.lang == LangEN {
		m.lang = LangZH
	} else {
		m.lang = LangEN
	}
	return m
}

func (m Model) updatePick(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch {
	case key.Matches(msg, keyQuit):
		m.quitRequested = true
		return m, tea.Quit
	case key.Matches(msg, keyEsc):
		// In modePick, ESC means "cancel". Signal cancellation to the parent
		// (when embedded as sub-model) by setting quitRequested. The standalone
		// first-run wizard treats this the same as ctrl+c (no chosen path).
		m.quitRequested = true
		return m, tea.Quit
	case key.Matches(msg, keyLang):
		m = m.toggleLang()
	case key.Matches(msg, keyUp):
		if m.cursor > 0 {
			m.cursor--
		}
	case key.Matches(msg, keyDown):
		if m.cursor < len(m.items)-1 {
			m.cursor++
		}
	case key.Matches(msg, keyEnter):
		it := m.items[m.cursor]
		if it.kind == kindCustom {
			m.mode = modeCustom
			m.custom.Focus()
			return m, textinput.Blink
		}
		m.chosen = it.path
		return m, tea.Quit
	}
	return m, nil
}

func (m Model) updateCustom(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch {
	case key.Matches(msg, keyQuit):
		m.quitRequested = true
		return m, tea.Quit
	case key.Matches(msg, keyEsc):
		m.mode = modePick
		m.custom.Blur()
		m.customErr = ""
		return m, nil
	case key.Matches(msg, keyEnter):
		path, err := validateCustomPath(m.custom.Value(), stringsFor(m.lang))
		if err != nil {
			m.customErr = err.Error()
			return m, nil
		}
		m.chosen = path
		return m, tea.Quit
	}
	var cmd tea.Cmd
	m.custom, cmd = m.custom.Update(msg)
	m.customErr = ""
	return m, cmd
}

// Chosen returns the path the user picked, or "" if they quit without choosing.
func (m Model) Chosen() string { return m.chosen }

// QuitRequested reports whether the user explicitly asked to leave the
// wizard (ctrl+c at any time, or ESC from modePick). Parent sub-model hosts
// use this as the "cancel" signal instead of propagating tea.Quit, which
// would kill the entire app.
func (m Model) QuitRequested() bool { return m.quitRequested }

// validateCustomPath expands ~ and checks the result is absolute. The parent
// dir doesn't have to exist yet — Store creates it on first save.
func validateCustomPath(in string, s strings) (string, error) {
	in = gostr.TrimSpace(in)
	if in == "" {
		return "", fmt.Errorf("%s", s.CustomErrEmpty)
	}
	if gostr.HasPrefix(in, "~") {
		home, err := os.UserHomeDir()
		if err == nil {
			in = filepath.Join(home, gostr.TrimPrefix(in, "~"))
		}
	}
	if !filepath.IsAbs(in) {
		return "", fmt.Errorf("%s", s.CustomErrNotAbs)
	}
	return filepath.Clean(in), nil
}

// ----- View ----------------------------------------------------------------

var (
	titleStyle    = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("12"))
	cursorStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("12")).Bold(true)
	dimStyle      = lipgloss.NewStyle().Foreground(lipgloss.Color("240"))
	tipStyle      = lipgloss.NewStyle().Foreground(lipgloss.Color("242")).Italic(true)
	hotkeysStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("240"))
	errStyle      = lipgloss.NewStyle().Foreground(lipgloss.Color("9"))
	langTagStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("12"))
)

func (m Model) View() string {
	s := stringsFor(m.lang)
	var b gostr.Builder

	tag := "[EN]"
	if m.lang == LangZH {
		tag = "[中]"
	}
	b.WriteString(titleStyle.Render(s.Title))
	b.WriteString("    ")
	b.WriteString(langTagStyle.Render(tag))
	b.WriteString("\n\n")

	b.WriteString(s.Question)
	b.WriteString("\n\n")

	for i, it := range m.items {
		cursor := "  "
		if i == m.cursor && m.mode == modePick {
			cursor = cursorStyle.Render("> ")
		}
		label, desc := m.itemLabel(it, s)
		b.WriteString(cursor)
		if i == m.cursor && m.mode == modePick {
			b.WriteString(cursorStyle.Render(label))
		} else {
			b.WriteString(label)
		}
		b.WriteString("\n  ")
		b.WriteString(dimStyle.Render("┕ " + desc))
		b.WriteString("\n\n")
	}

	if m.mode == modeCustom {
		b.WriteString(s.CustomPrompt)
		b.WriteString("\n  ")
		b.WriteString(m.custom.View())
		b.WriteString("\n  ")
		b.WriteString(dimStyle.Render(s.CustomHelp))
		b.WriteString("\n  ")
		b.WriteString(dimStyle.Render(s.BackHint))
		if m.customErr != "" {
			b.WriteString("\n  ")
			b.WriteString(errStyle.Render(m.customErr))
		}
		b.WriteString("\n\n")
	} else {
		b.WriteString(tipStyle.Render(s.Tip))
		b.WriteString("\n\n")
	}

	b.WriteString(hotkeysStyle.Render(s.Hotkeys))
	return b.String()
}

func (m Model) itemLabel(it item, s strings) (label, desc string) {
	switch it.kind {
	case kindVault:
		return fmt.Sprintf(s.VaultLabel, it.label), it.path
	case kindDefault:
		return s.DefaultLabel, s.DefaultDesc
	case kindCustom:
		return s.CustomLabel, s.CustomDesc
	}
	return "", ""
}

// DefaultLang inspects $LANG; returns LangZH for zh-prefixed locales, LangEN otherwise.
// Currently unused by main (we always start in EN per user choice), but kept
// here so future toggles don't have to reinvent the lookup.
func DefaultLang() Lang {
	v := os.Getenv("LANG")
	if gostr.HasPrefix(gostr.ToLower(v), "zh") {
		return LangZH
	}
	return LangEN
}
