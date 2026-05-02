package setup

import (
	"os"
	gostr "strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func keyMsg(s string) tea.KeyMsg {
	switch s {
	case "up":
		return tea.KeyMsg{Type: tea.KeyUp}
	case "down":
		return tea.KeyMsg{Type: tea.KeyDown}
	case "enter":
		return tea.KeyMsg{Type: tea.KeyEnter}
	case "esc":
		return tea.KeyMsg{Type: tea.KeyEsc}
	}
	return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(s)}
}

func TestNewModel_buildsItemsForVaults(t *testing.T) {
	m := NewModel(LangEN, []Vault{{Name: "hoard", Path: "/Users/x/hoard"}})
	if len(m.items) != 3 {
		t.Fatalf("want 3 items (1 vault + default + custom), got %d", len(m.items))
	}
	if m.items[0].kind != kindVault || m.items[0].label != "hoard" {
		t.Errorf("first item: %+v", m.items[0])
	}
	if !gostr.Contains(m.items[0].path, "/Users/x/hoard/.tick/tasks.md") {
		t.Errorf("vault path should resolve under .tick/: got %q", m.items[0].path)
	}
	if m.items[1].kind != kindDefault {
		t.Errorf("second item should be default")
	}
	if m.items[2].kind != kindCustom {
		t.Errorf("third item should be custom")
	}
}

func TestNewModel_noVaultsStillHasDefaultAndCustom(t *testing.T) {
	m := NewModel(LangEN, nil)
	if len(m.items) != 2 {
		t.Fatalf("want 2 items, got %d", len(m.items))
	}
	if m.items[0].kind != kindDefault || m.items[1].kind != kindCustom {
		t.Errorf("items: %+v", m.items)
	}
}

func TestUpdate_downUpMovesCursor(t *testing.T) {
	m := NewModel(LangEN, []Vault{{Name: "v1", Path: "/v1"}})
	out, _ := m.Update(keyMsg("down"))
	if out.(Model).cursor != 1 {
		t.Errorf("down: cursor want 1, got %d", out.(Model).cursor)
	}
	out, _ = out.(Model).Update(keyMsg("down"))
	out, _ = out.(Model).Update(keyMsg("down")) // would go past last
	if out.(Model).cursor != 2 {
		t.Errorf("cursor should clamp at len-1=2, got %d", out.(Model).cursor)
	}
	out, _ = out.(Model).Update(keyMsg("up"))
	if out.(Model).cursor != 1 {
		t.Errorf("up from 2 want 1, got %d", out.(Model).cursor)
	}
}

func TestUpdate_enterOnVaultPicks(t *testing.T) {
	m := NewModel(LangEN, []Vault{{Name: "v1", Path: "/abs/v1"}})
	out, cmd := m.Update(keyMsg("enter"))
	if out.(Model).Chosen() == "" {
		t.Error("Chosen() should be set after Enter on vault")
	}
	if !gostr.HasSuffix(out.(Model).Chosen(), "/abs/v1/.tick/tasks.md") {
		t.Errorf("Chosen path: %q", out.(Model).Chosen())
	}
	if cmd == nil {
		t.Error("Enter on vault should issue tea.Quit")
	}
}

func TestUpdate_enterOnCustomEntersInputMode(t *testing.T) {
	m := NewModel(LangEN, nil)
	// Items: [default, custom]. cursor=0 default; move to custom.
	out, _ := m.Update(keyMsg("down"))
	out, _ = out.(Model).Update(keyMsg("enter"))
	if out.(Model).mode != modeCustom {
		t.Errorf("Enter on custom should enter modeCustom, got %v", out.(Model).mode)
	}
	if out.(Model).Chosen() != "" {
		t.Error("Chosen() should remain empty until custom path confirmed")
	}
}

func TestUpdate_customModeEscReturnsToPick(t *testing.T) {
	m := NewModel(LangEN, nil)
	out, _ := m.Update(keyMsg("down"))
	out, _ = out.(Model).Update(keyMsg("enter")) // enter custom mode
	out, _ = out.(Model).Update(keyMsg("esc"))
	if out.(Model).mode != modePick {
		t.Errorf("Esc should return to modePick, got %v", out.(Model).mode)
	}
}

func TestValidateCustomPath_table(t *testing.T) {
	home, _ := os.UserHomeDir()
	cases := []struct {
		in      string
		want    string // empty means error
		wantErr bool
	}{
		{"", "", true},
		{"   ", "", true},
		{"relative/path.md", "", true},
		{"/abs/foo/tasks.md", "/abs/foo/tasks.md", false},
		{"~/notes/tasks.md", home + "/notes/tasks.md", false},
		{"  /abs/with/spaces.md  ", "/abs/with/spaces.md", false},
	}
	for _, c := range cases {
		got, err := validateCustomPath(c.in, enStrings)
		if c.wantErr && err == nil {
			t.Errorf("%q: expected error, got %q", c.in, got)
			continue
		}
		if !c.wantErr && err != nil {
			t.Errorf("%q: unexpected error: %v", c.in, err)
			continue
		}
		if !c.wantErr && got != c.want {
			t.Errorf("%q: got %q want %q", c.in, got, c.want)
		}
	}
}

func TestUpdate_customSubmitEmptyShowsError(t *testing.T) {
	m := NewModel(LangEN, nil)
	out, _ := m.Update(keyMsg("down"))                // to custom
	out, _ = out.(Model).Update(keyMsg("enter"))      // into custom mode
	out, _ = out.(Model).Update(keyMsg("enter"))      // submit empty
	mm := out.(Model)
	if mm.customErr == "" {
		t.Error("expected customErr on empty submit")
	}
	if mm.Chosen() != "" {
		t.Error("Chosen should still be empty")
	}
}

func TestUpdate_pickModeEscSetsQuitRequested(t *testing.T) {
	m := NewModel(LangEN, nil)
	out, cmd := m.Update(keyMsg("esc"))
	mm := out.(Model)
	if !mm.QuitRequested() {
		t.Error("ESC in modePick should set quitRequested=true (cancel signal for parents)")
	}
	if mm.Chosen() != "" {
		t.Error("ESC must not set chosen path")
	}
	if cmd == nil {
		t.Error("ESC in modePick should still issue tea.Quit (standalone wizard quits)")
	}
}

func TestUpdate_ctrlCSetsQuitRequested(t *testing.T) {
	m := NewModel(LangEN, nil)
	out, cmd := m.Update(tea.KeyMsg{Type: tea.KeyCtrlC})
	mm := out.(Model)
	if !mm.QuitRequested() {
		t.Error("ctrl+c should set quitRequested=true")
	}
	if cmd == nil {
		t.Error("ctrl+c should issue tea.Quit")
	}
}

func TestUpdate_ctrlCFromCustomModeSetsQuitRequested(t *testing.T) {
	m := NewModel(LangEN, nil)
	out, _ := m.Update(keyMsg("down"))             // to custom item
	out, _ = out.(Model).Update(keyMsg("enter"))   // into modeCustom
	out, _ = out.(Model).Update(tea.KeyMsg{Type: tea.KeyCtrlC})
	if !out.(Model).QuitRequested() {
		t.Error("ctrl+c in modeCustom should also set quitRequested=true")
	}
}

func TestUpdate_chosenDoesNotSetQuitRequested(t *testing.T) {
	m := NewModel(LangEN, []Vault{{Name: "v1", Path: "/abs/v1"}})
	out, _ := m.Update(keyMsg("enter")) // pick vault
	mm := out.(Model)
	if mm.Chosen() == "" {
		t.Fatal("setup: vault should be chosen")
	}
	if mm.QuitRequested() {
		t.Error("Choosing a path must NOT set quitRequested (parents distinguish chose vs cancelled)")
	}
}

// TestWizard_QuitInvariant locks down the invariant documented on Model:
// any tea.Quit cmd returned by Update must be accompanied by either
// Chosen() != "" (success path) or QuitRequested() == true (cancel path).
// This test is intentionally exhaustive over known quit-producing paths so
// that adding a new tea.Quit branch without the matching signal causes a
// clear failure here.
func TestWizard_QuitInvariant(t *testing.T) {
	type signal int
	const (
		sigChosen        signal = iota // m.Chosen() != ""
		sigQuitRequested               // m.QuitRequested() == true
	)

	type tc struct {
		name           string
		setup          func() Model
		keys           []tea.Msg
		expectedSignal signal
	}

	oneVault := []Vault{{Name: "hoard", Path: "/abs/hoard"}}

	cases := []tc{
		{
			name:           "ctrl+c in modePick",
			setup:          func() Model { return NewModel(LangEN, nil) },
			keys:           []tea.Msg{tea.KeyMsg{Type: tea.KeyCtrlC}},
			expectedSignal: sigQuitRequested,
		},
		{
			name:  "esc in modePick",
			setup: func() Model { return NewModel(LangEN, nil) },
			keys:  []tea.Msg{keyMsg("esc")},
			expectedSignal: sigQuitRequested,
		},
		{
			name:  "ctrl+c in modeCustom",
			setup: func() Model { return NewModel(LangEN, nil) },
			// navigate to custom item (index 1 with no vaults), enter custom mode
			keys: []tea.Msg{
				keyMsg("down"),
				keyMsg("enter"),
				tea.KeyMsg{Type: tea.KeyCtrlC},
			},
			expectedSignal: sigQuitRequested,
		},
		{
			name:           "enter on default item in modePick",
			setup:          func() Model { return NewModel(LangEN, nil) },
			keys:           []tea.Msg{keyMsg("enter")},
			expectedSignal: sigChosen,
		},
		{
			name:           "enter on vault item in modePick",
			setup:          func() Model { return NewModel(LangEN, oneVault) },
			keys:           []tea.Msg{keyMsg("enter")},
			expectedSignal: sigChosen,
		},
		{
			name:  "enter on valid custom path in modeCustom",
			setup: func() Model { return NewModel(LangEN, nil) },
			// no vaults: items=[default, custom]; cursor starts at 0 (default)
			// move down to custom, enter custom mode, type a valid path, submit
			keys: []tea.Msg{
				keyMsg("down"),
				keyMsg("enter"),
				// type characters that form a valid absolute path
				tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("/tmp/tick/tasks.md")},
				keyMsg("enter"),
			},
			expectedSignal: sigChosen,
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			var (
				mdl tea.Model = c.setup()
				cmd tea.Cmd
			)

			for i, msg := range c.keys {
				mdl, cmd = mdl.Update(msg)
				// Only the last key is expected to produce the quit; interim
				// cmds (e.g. textinput.Blink) are intentionally ignored.
				_ = i
			}

			// Execute the cmd returned by the final key press.
			if cmd == nil {
				t.Fatalf("expected a non-nil tea.Cmd after final key, got nil")
			}
			msg := cmd()
			if _, isQuit := msg.(tea.QuitMsg); !isQuit {
				t.Fatalf("expected cmd to produce tea.QuitMsg, got %T", msg)
			}

			wizard := mdl.(Model)

			// Core invariant: at least one signal must be set.
			if wizard.Chosen() == "" && !wizard.QuitRequested() {
				t.Fatalf("INVARIANT VIOLATED: tea.Quit returned but neither Chosen nor QuitRequested is set")
			}

			// Semantic check: the right signal for the right path.
			switch c.expectedSignal {
			case sigChosen:
				if wizard.Chosen() == "" {
					t.Errorf("expected Chosen() != \"\", got empty")
				}
				if wizard.QuitRequested() {
					t.Errorf("success path must NOT set QuitRequested (parent uses it as cancel signal)")
				}
			case sigQuitRequested:
				if !wizard.QuitRequested() {
					t.Errorf("expected QuitRequested() == true, got false")
				}
				if wizard.Chosen() != "" {
					t.Errorf("cancel path must NOT set Chosen, got %q", wizard.Chosen())
				}
			}
		})
	}
}

func TestWizard_LangToggle_ModePick_L(t *testing.T) {
	m := NewModel(LangEN, nil)
	out, _ := m.Update(keyMsg("l"))
	if out.(Model).lang != LangZH {
		t.Errorf("modePick + l: want ZH, got %v", out.(Model).lang)
	}
	out, _ = out.(Model).Update(keyMsg("l"))
	if out.(Model).lang != LangEN {
		t.Errorf("modePick + l + l: want EN, got %v", out.(Model).lang)
	}
}

func TestWizard_LangToggle_ModeCustom_L_PassesThrough(t *testing.T) {
	m := NewModel(LangEN, nil)
	// navigate to custom item and enter modeCustom
	out, _ := m.Update(keyMsg("down"))   // cursor to custom
	out, _ = out.(Model).Update(keyMsg("enter")) // enter modeCustom
	mm := out.(Model)
	if mm.mode != modeCustom {
		t.Fatalf("setup failed: expected modeCustom, got %v", mm.mode)
	}
	langBefore := mm.lang
	out, _ = mm.Update(keyMsg("l"))
	result := out.(Model)
	// lang must NOT change
	if result.lang != langBefore {
		t.Errorf("modeCustom + l: lang should not change (got %v)", result.lang)
	}
	// l must have been fed to the textinput
	if !gostr.Contains(result.custom.Value(), "l") {
		t.Errorf("modeCustom + l: textinput value should contain 'l', got %q", result.custom.Value())
	}
}

func TestDefaultLang(t *testing.T) {
	t.Setenv("LANG", "zh_CN.UTF-8")
	if DefaultLang() != LangZH {
		t.Error("zh_CN should default to ZH")
	}
	t.Setenv("LANG", "en_US.UTF-8")
	if DefaultLang() != LangEN {
		t.Error("en_US should default to EN")
	}
	t.Setenv("LANG", "")
	if DefaultLang() != LangEN {
		t.Error("empty LANG should default to EN")
	}
}
