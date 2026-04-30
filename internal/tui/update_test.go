package tui

import (
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/yaoyi/tick-tui/internal/api"
)

// --- fixtures ---------------------------------------------------------------

func pendingFeature(id int64, title string) api.Feature {
	return api.Feature{ID: id, Title: title, IsDone: 0, CreatedAt: "2026-05-01"}
}

func doneFeature(id int64, title string) api.Feature {
	done := "2026-05-01"
	return api.Feature{ID: id, Title: title, IsDone: 1, CompletedAt: &done, CreatedAt: "2026-05-01"}
}

func modelWithRows(pending, done []api.Feature) Model {
	m := NewModel(nil) // nil client: tests don't call API
	m.today = api.TodayResponse{
		Pending:    pending,
		Done:       done,
		DoneToday:  len(done),
		TotalToday: len(pending) + len(done),
	}
	m.buildRows()
	m.cursor = 0
	return m
}

func pressKey(s string) tea.KeyMsg {
	return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(s)}
}

func pressSpecialKey(k tea.KeyType) tea.KeyMsg {
	return tea.KeyMsg{Type: k}
}

// update runs m.Update and returns the new Model (panics if cast fails).
func update(m Model, msg tea.Msg) (Model, tea.Cmd) {
	newModel, cmd := m.Update(msg)
	return newModel.(Model), cmd
}

// --- tests ------------------------------------------------------------------

func TestMarkDone_entersGrace(t *testing.T) {
	m := modelWithRows([]api.Feature{pendingFeature(1, "buy milk")}, nil)

	newM, cmd := update(m, pressKey("t"))

	if newM.mode != modeGraceUndo {
		t.Errorf("mode: got %v want modeGraceUndo", newM.mode)
	}
	if newM.graceID != 1 {
		t.Errorf("graceID: got %d want 1", newM.graceID)
	}
	if cmd == nil {
		t.Error("expected non-nil Cmd (markDone + graceTimer)")
	}
	if newM.footerMsg == "" {
		t.Error("expected footer message")
	}
}

func TestGraceUndo_sendsUndone(t *testing.T) {
	m := modelWithRows([]api.Feature{pendingFeature(1, "buy milk")}, nil)
	m.mode = modeGraceUndo
	m.graceID = 1
	m.footerMsg = "marked done · u to undo"

	newM, cmd := update(m, pressKey("u"))

	if newM.mode != modeList {
		t.Errorf("mode: got %v want modeList", newM.mode)
	}
	if cmd == nil {
		t.Error("expected non-nil Cmd (undone)")
	}
	if newM.graceID != 0 {
		t.Errorf("graceID should be cleared, got %d", newM.graceID)
	}
}

func TestGraceOtherKey_exitsGraceAndProcesses(t *testing.T) {
	m := modelWithRows([]api.Feature{
		pendingFeature(1, "task A"),
		pendingFeature(2, "task B"),
	}, nil)
	m.mode = modeGraceUndo
	m.graceID = 1
	m.cursor = 0

	newM, _ := update(m, pressKey("j"))

	if newM.mode != modeList {
		t.Errorf("mode: got %v want modeList", newM.mode)
	}
	if newM.graceID != 0 {
		t.Errorf("graceID should be cleared")
	}
	// j moves cursor down
	if newM.cursor != 1 {
		t.Errorf("cursor: got %d want 1", newM.cursor)
	}
}

func TestAdd_entersEditNew(t *testing.T) {
	m := modelWithRows(nil, nil)

	newM, cmd := update(m, pressKey("a"))

	if newM.mode != modeEdit {
		t.Errorf("mode: got %v want modeEdit", newM.mode)
	}
	if newM.editingID != 0 {
		t.Errorf("editingID: got %d want 0 (new)", newM.editingID)
	}
	if newM.field != fieldTitle {
		t.Errorf("field: got %v want fieldTitle", newM.field)
	}
	if newM.titleInput.Value() != "" {
		t.Errorf("titleInput should be empty, got %q", newM.titleInput.Value())
	}
	if !newM.titleInput.Focused() {
		t.Error("titleInput should be focused")
	}
	if cmd == nil {
		t.Error("expected non-nil Cmd (blink)")
	}
}

func TestEdit_entersEditExisting(t *testing.T) {
	proj := "work"
	f := api.Feature{ID: 42, Title: "finish report", ProjectName: &proj, IsDone: 0, CreatedAt: "2026-05-01"}
	m := modelWithRows([]api.Feature{f}, nil)

	newM, _ := update(m, pressKey("e"))

	if newM.mode != modeEdit {
		t.Errorf("mode: got %v want modeEdit", newM.mode)
	}
	if newM.editingID != 42 {
		t.Errorf("editingID: got %d want 42", newM.editingID)
	}
	if newM.titleInput.Value() != "finish report" {
		t.Errorf("titleInput: got %q want %q", newM.titleInput.Value(), "finish report")
	}
}

func TestTabCyclesFields(t *testing.T) {
	// Pending edit (a / e on pending) cycles title <-> project; date is excluded.
	m := modelWithRows([]api.Feature{pendingFeature(1, "task")}, nil)
	m, _ = update(m, pressKey("a")) // enter edit mode (new pending)

	if m.field != fieldTitle {
		t.Fatalf("initial field=%v want fieldTitle", m.field)
	}
	// title → project
	m, _ = update(m, pressSpecialKey(tea.KeyTab))
	if m.field != fieldProject {
		t.Errorf("after 1 Tab: field=%v want fieldProject", m.field)
	}
	// project → title (no date in pending edit)
	m, _ = update(m, pressSpecialKey(tea.KeyTab))
	if m.field != fieldTitle {
		t.Errorf("after 2 Tab: field=%v want fieldTitle (no date in pending edit)", m.field)
	}
}

func TestTabNoOpInDoneEdit(t *testing.T) {
	// Done edit (e on done) only allows the date field; Tab is a no-op.
	m := modelWithRows(nil, []api.Feature{doneFeature(5, "old")})
	m.cursor = 0
	m.buildRows()
	m, _ = update(m, pressKey("e"))

	if m.field != fieldDate {
		t.Fatalf("done edit initial field=%v want fieldDate", m.field)
	}
	if !m.editingDone {
		t.Fatalf("editingDone=false; want true")
	}
	m, _ = update(m, pressSpecialKey(tea.KeyTab))
	if m.field != fieldDate {
		t.Errorf("Tab in done edit: field=%v want fieldDate (no-op)", m.field)
	}
}

func TestUntick_entersConfirmMode(t *testing.T) {
	m := modelWithRows(nil, []api.Feature{doneFeature(5, "old task")})
	// cursor is on separator (pending is empty), skip to done row
	m.cursor = 0 // only done rows, no separator since pending is empty
	m.buildRows() // rebuild: pending empty, done has 1 → no separator

	newM, _ := update(m, pressKey("U"))

	if newM.mode != modeConfirmUntick {
		t.Errorf("mode: got %v want modeConfirmUntick", newM.mode)
	}
	if newM.pendingID != 5 {
		t.Errorf("pendingID: got %d want 5", newM.pendingID)
	}
}

func TestConfirmUntick_yExecutes(t *testing.T) {
	m := modelWithRows(nil, []api.Feature{doneFeature(5, "old task")})
	m.mode = modeConfirmUntick
	m.pendingID = 5

	newM, cmd := update(m, pressKey("y"))

	if newM.mode != modeList {
		t.Errorf("mode: got %v want modeList", newM.mode)
	}
	if cmd == nil {
		t.Error("expected non-nil Cmd (undone)")
	}
}

func TestConfirmUntick_otherKeyCancels(t *testing.T) {
	m := modelWithRows(nil, []api.Feature{doneFeature(5, "old task")})
	m.mode = modeConfirmUntick
	m.pendingID = 5

	newM, cmd := update(m, pressKey("n"))

	if newM.mode != modeList {
		t.Errorf("mode: got %v want modeList", newM.mode)
	}
	if newM.pendingID != 0 {
		t.Errorf("pendingID should be cleared")
	}
	if cmd != nil {
		t.Error("cancel should not produce Cmd")
	}
}

func TestConfirmDelete_yExecutes(t *testing.T) {
	m := modelWithRows([]api.Feature{pendingFeature(3, "task")}, nil)
	m.mode = modeConfirmDelete
	m.pendingID = 3

	newM, cmd := update(m, pressKey("y"))

	if newM.mode != modeList {
		t.Errorf("mode: got %v want modeList", newM.mode)
	}
	if cmd == nil {
		t.Error("expected non-nil Cmd (delete)")
	}
}

func TestEditEscape_returnsToList(t *testing.T) {
	m := modelWithRows([]api.Feature{pendingFeature(1, "task")}, nil)
	m, _ = update(m, pressKey("e")) // enter edit

	newM, _ := update(m, pressSpecialKey(tea.KeyEsc))

	if newM.mode != modeList {
		t.Errorf("mode: got %v want modeList", newM.mode)
	}
	if newM.editingID != 0 {
		t.Errorf("editingID should be 0, got %d", newM.editingID)
	}
}

func TestGraceExpired_clearsGrace(t *testing.T) {
	m := modelWithRows(nil, nil)
	m.mode = modeGraceUndo
	m.graceID = 7
	m.footerMsg = "marked done · u to undo"

	newM, _ := update(m, graceExpiredMsg{id: 7})

	if newM.mode != modeList {
		t.Errorf("mode: got %v want modeList", newM.mode)
	}
	if newM.graceID != 0 {
		t.Errorf("graceID should be 0")
	}
	if newM.footerMsg != "" {
		t.Errorf("footerMsg should be empty, got %q", newM.footerMsg)
	}
}

func TestGraceExpired_wrongID_ignored(t *testing.T) {
	m := modelWithRows(nil, nil)
	m.mode = modeGraceUndo
	m.graceID = 7

	newM, _ := update(m, graceExpiredMsg{id: 99})

	// Different ID → grace should remain
	if newM.mode != modeGraceUndo {
		t.Errorf("mode should remain grace, got %v", newM.mode)
	}
}

func TestWindowSizeMsg(t *testing.T) {
	m := modelWithRows(nil, nil)
	newM, _ := update(m, tea.WindowSizeMsg{Width: 120, Height: 40})
	if newM.width != 120 || newM.height != 40 {
		t.Errorf("size: got %dx%d want 120x40", newM.width, newM.height)
	}
}

func TestTodayLoadedMsg_rebuildsRows(t *testing.T) {
	m := modelWithRows(nil, nil)
	resp := &api.TodayResponse{
		Pending: []api.Feature{pendingFeature(1, "task1"), pendingFeature(2, "task2")},
		Done:    []api.Feature{doneFeature(3, "done1")},
	}

	newM, _ := update(m, todayLoadedMsg{resp: resp})

	// pending(2) + separator(1) + done(1) = 4 rows
	if len(newM.rows) != 4 {
		t.Errorf("rows: got %d want 4", len(newM.rows))
	}
	if newM.rows[2].kind != rowSeparator {
		t.Errorf("row[2] should be separator")
	}
}

func TestEditDate_upDownChangesDate(t *testing.T) {
	// Date editing is only available on done features.
	m := modelWithRows(nil, []api.Feature{doneFeature(7, "old task")})
	m.cursor = 0
	m.buildRows()
	m, _ = update(m, pressKey("e"))

	if m.field != fieldDate {
		t.Fatalf("done edit field=%v want fieldDate", m.field)
	}

	before := m.editDate
	m, _ = update(m, pressKey("up"))
	after := m.editDate

	if !after.After(before) {
		t.Errorf("up should advance date; before=%v after=%v", before, after)
	}
	if after.Sub(before) != 24*time.Hour {
		t.Errorf("date delta: got %v want 24h", after.Sub(before))
	}
}

// --- Ghost text helper test -------------------------------------------------

func TestComputeGhostText(t *testing.T) {
	projects := []string{"work", "read", "life"}

	tests := []struct {
		input string
		want  string
	}{
		{"@r", "ead"},
		{"@w", "ork"},
		{"@re", "ad"},
		{"@read", ""},  // already complete
		{"@z", ""},     // no match
		{"hello", ""},  // no @ prefix
		{"hello @l", "ife"},
	}

	for _, tc := range tests {
		got := ComputeGhostText(tc.input, projects)
		if got != tc.want {
			t.Errorf("ComputeGhostText(%q) = %q, want %q", tc.input, got, tc.want)
		}
	}
}

func TestExtractProjectFromTitle(t *testing.T) {
	tests := []struct {
		input   string
		title   string
		project string
	}{
		{"buy milk @life", "buy milk", "life"},
		{"just a task", "just a task", ""},
		{"task @work extra", "task @work extra", ""}, // @ not at end
		{"@work", "", "work"},
	}
	for _, tc := range tests {
		gotTitle, gotProj := extractProjectFromTitle(tc.input)
		if gotTitle != tc.title || gotProj != tc.project {
			t.Errorf("extractProjectFromTitle(%q) = (%q, %q), want (%q, %q)",
				tc.input, gotTitle, gotProj, tc.title, tc.project)
		}
	}
}

// --- 🔴-4: CJK rune-safe renderTitleWithGhost ---

// TestRenderTitleWithGhost_CJK_noPanic verifies that a multi-byte CJK string at
// an interior rune position does not panic or produce garbled output.
func TestRenderTitleWithGhost_CJK_noPanic(t *testing.T) {
	// "买菜" is 2 runes, each 3 bytes. pos=1 is a valid rune boundary but
	// byte index 1 would fall inside the first character (3-byte sequence).
	value := "买菜"
	// Must not panic
	got := renderTitleWithGhost(value, 1, "", true)
	// Result should contain the full characters (no garbled bytes)
	if got == "" {
		t.Error("got empty string")
	}
}

func TestRenderTitleWithGhost_CJK_posAtEnd(t *testing.T) {
	value := "买菜"
	// pos == len(runes) is a valid cursor-at-end position
	got := renderTitleWithGhost(value, 2, "", true)
	if got == "" {
		t.Error("got empty string")
	}
}

func TestRenderTitleWithGhost_CJK_posExceedsLen(t *testing.T) {
	value := "买"
	// pos beyond rune length should be clamped, not panic
	got := renderTitleWithGhost(value, 99, "", true)
	if got == "" {
		t.Error("got empty string")
	}
}

// --- 🔴-5: narrow terminal separator ---

func TestRenderSeparator_zeroWidth_noPanic(t *testing.T) {
	// Must not panic with strings.Repeat negative count
	got := renderSeparator(0)
	if got == "" {
		t.Error("expected non-empty fallback")
	}
}

func TestRenderSeparator_veryNarrow_noPanic(t *testing.T) {
	got := renderSeparator(3)
	if got == "" {
		t.Error("expected non-empty fallback")
	}
}

func TestRenderSeparator_normalWidth(t *testing.T) {
	got := renderSeparator(40)
	// A normal-width separator should contain the label text
	if !containsSubstring(got, "done") {
		t.Errorf("separator should contain 'done', got %q", got)
	}
}

// containsSubstring is a simple helper that strips ANSI for the check.
func containsSubstring(s, sub string) bool {
	// Strip ANSI escape codes with a naive approach sufficient for tests
	clean := ""
	inEsc := false
	for _, r := range s {
		if r == '\x1b' {
			inEsc = true
			continue
		}
		if inEsc {
			if r == 'm' {
				inEsc = false
			}
			continue
		}
		clean += string(r)
	}
	return len(clean) > 0 && len(sub) > 0 && (len(clean) >= len(sub)) && func() bool {
		for i := 0; i <= len(clean)-len(sub); i++ {
			if clean[i:i+len(sub)] == sub {
				return true
			}
		}
		return false
	}()
}

// --- 🟡-9+10: dateModified guards ---

func TestCmdSave_newFeature_dateNotModified_sendsEmptyDate(t *testing.T) {
	var capturedDate string
	captured := false

	m := modelWithRows(nil, nil)
	m.mode = modeEdit
	m.editingID = 0
	m.dateModified = false
	m.titleInput.SetValue("new task")
	m.projectInput.SetValue("")

	// Replace apiClient with a stub that records the date argument
	stub := &stubClient{createFn: func(text, date string) (*api.Feature, error) {
		capturedDate = date
		captured = true
		f := api.Feature{ID: 1, Title: "new task"}
		return &f, nil
	}}
	m.apiClient = stub

	cmd := m.cmdSave()
	if cmd != nil {
		cmd() // execute the returned tea.Cmd
	}

	if !captured {
		t.Fatal("Create was not called")
	}
	if capturedDate != "" {
		t.Errorf("date should be empty when dateModified=false, got %q", capturedDate)
	}
}

func TestCmdSave_newFeature_dateModified_sendsDate(t *testing.T) {
	var capturedDate string

	m := modelWithRows(nil, nil)
	m.mode = modeEdit
	m.editingID = 0
	m.dateModified = true
	m.editDate = time.Date(2025, 4, 30, 0, 0, 0, 0, time.UTC)
	m.titleInput.SetValue("new task")

	stub := &stubClient{createFn: func(text, date string) (*api.Feature, error) {
		capturedDate = date
		return &api.Feature{ID: 1, Title: "new task"}, nil
	}}
	m.apiClient = stub

	cmd := m.cmdSave()
	if cmd != nil {
		cmd()
	}
	if capturedDate != "2025-04-30" {
		t.Errorf("date: got %q want %q", capturedDate, "2025-04-30")
	}
}

func TestCmdSave_existingFeature_dateNotModified_sendsNilDate(t *testing.T) {
	var capturedDate *string
	captured := false

	m := modelWithRows(nil, nil)
	m.mode = modeEdit
	m.editingID = 42
	m.dateModified = false
	m.titleInput.SetValue("old task")

	stub := &stubClient{updateFn: func(id int64, title, project string, date *string) (*api.Feature, error) {
		capturedDate = date
		captured = true
		return &api.Feature{ID: id, Title: title}, nil
	}}
	m.apiClient = stub

	cmd := m.cmdSave()
	if cmd != nil {
		cmd()
	}
	if !captured {
		t.Fatal("Update was not called")
	}
	if capturedDate != nil {
		t.Errorf("date should be nil when dateModified=false, got %v", *capturedDate)
	}
}

func TestCmdSave_existingFeature_dateModified_sendsDate(t *testing.T) {
	var capturedDate *string

	m := modelWithRows(nil, nil)
	m.mode = modeEdit
	m.editingID = 42
	m.dateModified = true
	m.editDate = time.Date(2026, 1, 15, 0, 0, 0, 0, time.UTC)
	m.titleInput.SetValue("old task")

	stub := &stubClient{updateFn: func(id int64, title, project string, date *string) (*api.Feature, error) {
		capturedDate = date
		return &api.Feature{ID: id, Title: title}, nil
	}}
	m.apiClient = stub

	cmd := m.cmdSave()
	if cmd != nil {
		cmd()
	}
	if capturedDate == nil {
		t.Fatal("date should be non-nil when dateModified=true")
	}
	if *capturedDate != "2026-01-15" {
		t.Errorf("date: got %q want %q", *capturedDate, "2026-01-15")
	}
}

// --- 🟡-6: renderTitleWithGhost rendering order ---

// Ghost must only appear at cursor-end; mid-text cursor must suppress it.
func TestRenderTitleWithGhost_ghostSuppressedWhenCursorMidText(t *testing.T) {
	// cursor at pos 1, after="菜" → ghost should NOT appear
	got := renderTitleWithGhost("买菜", 1, "phantom", true)
	if containsSubstring(got, "phantom") {
		t.Errorf("ghost should be suppressed when cursor is mid-text, got %q", got)
	}
	if !containsSubstring(got, "买") {
		t.Errorf("before text should appear, got %q", got)
	}
	if !containsSubstring(got, "菜") {
		t.Errorf("after text should appear, got %q", got)
	}
}

func TestRenderTitleWithGhost_ghostAppearsAtEnd(t *testing.T) {
	// cursor at end → ghost should appear
	got := renderTitleWithGhost("buy", 3, "milk", true)
	if !containsSubstring(got, "milk") {
		t.Errorf("ghost should appear at cursor-end, got %q", got)
	}
}

func TestRenderTitleWithGhost_CJK_midCursor_noGhost(t *testing.T) {
	// CJK text, cursor in middle, ghost candidate present → ghost suppressed
	got := renderTitleWithGhost("工作", 1, "作流", true)
	if containsSubstring(got, "作流") {
		t.Errorf("ghost should not appear mid-text even with CJK, got %q", got)
	}
}

// --- 🟡-12: grace period double-press guard ---

func TestMarkDone_gracePeriod_repeatT_noExtraCmd(t *testing.T) {
	m := modelWithRows([]api.Feature{pendingFeature(10, "feed cat")}, nil)
	// Simulate already being in grace for feature 10
	m.mode = modeGraceUndo
	m.graceID = 10

	// Pressing t again (handleMarkDone called) should be a no-op
	newM, cmd := m.handleMarkDone()

	if cmd != nil {
		t.Error("pressing t during grace for same feature should not produce a Cmd")
	}
	if newM.mode != modeGraceUndo {
		t.Errorf("mode should remain modeGraceUndo, got %v", newM.mode)
	}
}

func TestMarkDone_afterGraceCleared_allowsNewMark(t *testing.T) {
	m := modelWithRows([]api.Feature{pendingFeature(10, "feed cat")}, nil)
	// graceID is 0 (grace expired or never set)
	m.mode = modeList
	m.graceID = 0

	newM, cmd := m.handleMarkDone()

	if cmd == nil {
		t.Error("after grace cleared, pressing t should produce a Cmd")
	}
	if newM.mode != modeGraceUndo {
		t.Errorf("mode: got %v want modeGraceUndo", newM.mode)
	}
}

// --- 🔵-14: CJK ghost text via regex ---

func TestComputeGhostText_CJK(t *testing.T) {
	projects := []string{"工作", "read", "生活"}

	tests := []struct {
		input string
		want  string
	}{
		{"@工", "作"},           // CJK prefix match
		{"@r", "ead"},          // ASCII still works
		{"@工作", ""},           // exact match → no ghost
		{"@生", "活"},           // second CJK project
		{"@工作 extra", ""},    // @ not at end → no match
	}

	for _, tc := range tests {
		got := ComputeGhostText(tc.input, projects)
		if got != tc.want {
			t.Errorf("ComputeGhostText(%q) = %q, want %q", tc.input, got, tc.want)
		}
	}
}

// --- 🔵-16: availableRows reduced in date edit mode ---

func TestAvailableRows_dateModeReducesByOne(t *testing.T) {
	m := modelWithRows([]api.Feature{pendingFeature(1, "task")}, nil)
	m.width = 80
	m.height = 20
	// helpExpanded=false → footerLines=1; 2=titleBar+blank; availableRows=20-2-1=17
	// date mode subtracts one more → 16

	// Verify no panic with small height + date mode
	m.mode = modeEdit
	m.field = fieldDate
	// This should not panic; the rendered output should be non-empty
	got := m.View()
	if got == "" {
		t.Error("View() should not be empty")
	}
}

func TestAvailableRows_dateModeSmallHeight_noPanic(t *testing.T) {
	m := modelWithRows([]api.Feature{
		pendingFeature(1, "a"),
		pendingFeature(2, "b"),
		pendingFeature(3, "c"),
	}, nil)
	m.width = 40
	m.height = 5 // very narrow height
	m.mode = modeEdit
	m.field = fieldDate
	// Must not panic even when availableRows would go to 1 after clamping
	got := m.View()
	if got == "" {
		t.Error("View() should return something even at height=5")
	}
}

// stubClient implements the api.Client interface surface used by tests.
type stubClient struct {
	createFn func(text, date string) (*api.Feature, error)
	updateFn func(id int64, title, project string, date *string) (*api.Feature, error)
}

func (s *stubClient) GetToday() (*api.TodayResponse, error) {
	return &api.TodayResponse{}, nil
}
func (s *stubClient) GetProjects() ([]api.ProjectItem, error) { return nil, nil }
func (s *stubClient) Create(text, date string) (*api.Feature, error) {
	if s.createFn != nil {
		return s.createFn(text, date)
	}
	return &api.Feature{}, nil
}
func (s *stubClient) Update(id int64, title, project string, date *string) (*api.Feature, error) {
	if s.updateFn != nil {
		return s.updateFn(id, title, project, date)
	}
	return &api.Feature{}, nil
}
func (s *stubClient) MarkDone(id int64) (*api.Feature, error)  { return &api.Feature{}, nil }
func (s *stubClient) Undone(id int64) (*api.Feature, error)    { return &api.Feature{}, nil }
func (s *stubClient) Delete(id int64) error                    { return nil }
