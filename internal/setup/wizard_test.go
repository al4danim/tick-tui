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
	case "tab":
		return tea.KeyMsg{Type: tea.KeyTab}
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

func TestUpdate_tabTogglesLang(t *testing.T) {
	m := NewModel(LangEN, nil)
	if m.lang != LangEN {
		t.Fatal("seed should be EN")
	}
	out, _ := m.Update(keyMsg("tab"))
	if out.(Model).lang != LangZH {
		t.Errorf("after tab want ZH, got %v", out.(Model).lang)
	}
	out, _ = out.(Model).Update(keyMsg("tab"))
	if out.(Model).lang != LangEN {
		t.Errorf("after second tab want EN, got %v", out.(Model).lang)
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
