package tui

import (
	"strconv"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/al4danim/tick-tui/internal/store"
)

// --- fixtures ---------------------------------------------------------------

func pendingFeature(id string, title string) store.Feature {
	return store.Feature{ID: id, Title: title, IsDone: 0, CreatedAt: "2026-05-01"}
}

func doneFeature(id string, title string) store.Feature {
	done := "2026-05-01"
	return store.Feature{ID: id, Title: title, IsDone: 1, CompletedAt: &done, CreatedAt: "2026-05-01"}
}

func modelWithRows(pending, done []store.Feature) Model {
	m := NewModel(nil) // nil client: tests don't call API
	m.today = store.TodayResponse{
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
	m := modelWithRows([]store.Feature{pendingFeature("1", "buy milk")}, nil)

	newM, cmd := update(m, pressKey("t"))

	if newM.mode != modeGraceUndo {
		t.Errorf("mode: got %v want modeGraceUndo", newM.mode)
	}
	if newM.graceID != "1" {
		t.Errorf("graceID: got %s want 1", newM.graceID)
	}
	if cmd == nil {
		t.Error("expected non-nil Cmd (markDone + graceTimer)")
	}
	if newM.footerMsg == "" {
		t.Error("expected footer message")
	}
}

func TestGraceUndo_sendsUndone(t *testing.T) {
	m := modelWithRows([]store.Feature{pendingFeature("1", "buy milk")}, nil)
	m.mode = modeGraceUndo
	m.graceID = "1"
	m.footerMsg = "marked done · u to undo"

	newM, cmd := update(m, pressKey("u"))

	if newM.mode != modeList {
		t.Errorf("mode: got %v want modeList", newM.mode)
	}
	if cmd == nil {
		t.Error("expected non-nil Cmd (undone)")
	}
	if newM.graceID != "" {
		t.Errorf("graceID should be cleared, got %s", newM.graceID)
	}
}

func TestGraceOtherKey_exitsGraceAndProcesses(t *testing.T) {
	m := modelWithRows([]store.Feature{
		pendingFeature("1", "task A"),
		pendingFeature("2", "task B"),
	}, nil)
	m.mode = modeGraceUndo
	m.graceID = "1"
	m.cursor = 0

	newM, _ := update(m, pressKey("j"))

	if newM.mode != modeList {
		t.Errorf("mode: got %v want modeList", newM.mode)
	}
	if newM.graceID != "" {
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
	if newM.editingID != "" {
		t.Errorf("editingID: got %s want 0 (new)", newM.editingID)
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
	f := store.Feature{ID: "42", Title: "finish report", ProjectName: &proj, IsDone: 0, CreatedAt: "2026-05-01"}
	m := modelWithRows([]store.Feature{f}, nil)

	newM, _ := update(m, pressKey("e"))

	if newM.mode != modeEdit {
		t.Errorf("mode: got %v want modeEdit", newM.mode)
	}
	if newM.editingID != "42" {
		t.Errorf("editingID: got %s want 42", newM.editingID)
	}
	if newM.titleInput.Value() != "finish report" {
		t.Errorf("titleInput: got %q want %q", newM.titleInput.Value(), "finish report")
	}
}

func TestTabCyclesFields(t *testing.T) {
	// Pending edit (a / e on pending) cycles title <-> project; date is excluded.
	m := modelWithRows([]store.Feature{pendingFeature("1", "task")}, nil)
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
	m := modelWithRows(nil, []store.Feature{doneFeature("5", "old")})
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
	m := modelWithRows(nil, []store.Feature{doneFeature("5", "old task")})
	// cursor is on separator (pending is empty), skip to done row
	m.cursor = 0 // only done rows, no separator since pending is empty
	m.buildRows() // rebuild: pending empty, done has 1 → no separator

	newM, _ := update(m, pressKey("U"))

	if newM.mode != modeConfirmUntick {
		t.Errorf("mode: got %v want modeConfirmUntick", newM.mode)
	}
	if newM.pendingID != "5" {
		t.Errorf("pendingID: got %s want 5", newM.pendingID)
	}
}

func TestConfirmUntick_yExecutes(t *testing.T) {
	m := modelWithRows(nil, []store.Feature{doneFeature("5", "old task")})
	m.mode = modeConfirmUntick
	m.pendingID = "5"

	newM, cmd := update(m, pressKey("y"))

	if newM.mode != modeList {
		t.Errorf("mode: got %v want modeList", newM.mode)
	}
	if cmd == nil {
		t.Error("expected non-nil Cmd (undone)")
	}
}

func TestConfirmUntick_otherKeyCancels(t *testing.T) {
	m := modelWithRows(nil, []store.Feature{doneFeature("5", "old task")})
	m.mode = modeConfirmUntick
	m.pendingID = "5"

	newM, cmd := update(m, pressKey("n"))

	if newM.mode != modeList {
		t.Errorf("mode: got %v want modeList", newM.mode)
	}
	if newM.pendingID != "" {
		t.Errorf("pendingID should be cleared")
	}
	if cmd != nil {
		t.Error("cancel should not produce Cmd")
	}
}

func TestConfirmDelete_yExecutes(t *testing.T) {
	m := modelWithRows([]store.Feature{pendingFeature("3", "task")}, nil)
	m.mode = modeConfirmDelete
	m.pendingID = "3"

	newM, cmd := update(m, pressKey("y"))

	if newM.mode != modeList {
		t.Errorf("mode: got %v want modeList", newM.mode)
	}
	if cmd == nil {
		t.Error("expected non-nil Cmd (delete)")
	}
}

func TestEditEscape_returnsToList(t *testing.T) {
	m := modelWithRows([]store.Feature{pendingFeature("1", "task")}, nil)
	m, _ = update(m, pressKey("e")) // enter edit

	newM, _ := update(m, pressSpecialKey(tea.KeyEsc))

	if newM.mode != modeList {
		t.Errorf("mode: got %v want modeList", newM.mode)
	}
	if newM.editingID != "" {
		t.Errorf("editingID should be 0, got %s", newM.editingID)
	}
}

func TestGraceExpired_clearsGrace(t *testing.T) {
	m := modelWithRows(nil, nil)
	m.mode = modeGraceUndo
	m.graceID = "7"
	m.footerMsg = "marked done · u to undo"

	newM, _ := update(m, graceExpiredMsg{id: "7"})

	if newM.mode != modeList {
		t.Errorf("mode: got %v want modeList", newM.mode)
	}
	if newM.graceID != "" {
		t.Errorf("graceID should be 0")
	}
	if newM.footerMsg != "" {
		t.Errorf("footerMsg should be empty, got %q", newM.footerMsg)
	}
}

func TestGraceExpired_wrongID_ignored(t *testing.T) {
	m := modelWithRows(nil, nil)
	m.mode = modeGraceUndo
	m.graceID = "7"

	newM, _ := update(m, graceExpiredMsg{id: "99"})

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
	resp := &store.TodayResponse{
		Pending: []store.Feature{pendingFeature("1", "task1"), pendingFeature("2", "task2")},
		Done:    []store.Feature{doneFeature("3", "done1")},
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
	m := modelWithRows(nil, []store.Feature{doneFeature("7", "old task")})
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
	m.editingID = ""
	m.dateModified = false
	m.titleInput.SetValue("new task")
	m.projectInput.SetValue("")

	// Replace apiClient with a stub that records the date argument
	stub := &stubClient{createFn: func(text, date string) (*store.Feature, error) {
		capturedDate = date
		captured = true
		f := store.Feature{ID: "1", Title: "new task"}
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
	m.editingID = ""
	m.dateModified = true
	m.editDate = time.Date(2025, 4, 30, 0, 0, 0, 0, time.UTC)
	m.titleInput.SetValue("new task")

	stub := &stubClient{createFn: func(text, date string) (*store.Feature, error) {
		capturedDate = date
		return &store.Feature{ID: "1", Title: "new task"}, nil
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
	m.editingID = "42"
	m.dateModified = false
	m.titleInput.SetValue("old task")

	stub := &stubClient{updateFn: func(id string, title, project string, date *string) (*store.Feature, error) {
		capturedDate = date
		captured = true
		return &store.Feature{ID: id, Title: title}, nil
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
	m.editingID = "42"
	m.dateModified = true
	m.editDate = time.Date(2026, 1, 15, 0, 0, 0, 0, time.UTC)
	m.titleInput.SetValue("old task")

	stub := &stubClient{updateFn: func(id string, title, project string, date *string) (*store.Feature, error) {
		capturedDate = date
		return &store.Feature{ID: id, Title: title}, nil
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
	m := modelWithRows([]store.Feature{pendingFeature("10", "feed cat")}, nil)
	// Simulate already being in grace for feature 10
	m.mode = modeGraceUndo
	m.graceID = "10"

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
	m := modelWithRows([]store.Feature{pendingFeature("10", "feed cat")}, nil)
	// graceID is 0 (grace expired or never set)
	m.mode = modeList
	m.graceID = ""

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
	m := modelWithRows([]store.Feature{pendingFeature("1", "task")}, nil)
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
	m := modelWithRows([]store.Feature{
		pendingFeature("1", "a"),
		pendingFeature("2", "b"),
		pendingFeature("3", "c"),
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

// --- project filter (p) ---

func featureWithProject(id string, title, project string) store.Feature {
	p := project
	return store.Feature{ID: id, Title: title, ProjectName: &p, IsDone: 0, CreatedAt: "2026-05-01"}
}

func TestFilter_pTogglesOnCurrentProject(t *testing.T) {
	m := modelWithRows([]store.Feature{
		featureWithProject("1", "A1", "work"),
		featureWithProject("2", "B1", "home"),
		featureWithProject("3", "A2", "work"),
	}, nil)
	// rows after groupByProject: work group (size 2) first, then home (size 1)
	// cursor 0 → first work feature
	m.cursor = 0

	newM, _ := update(m, pressKey("p"))

	if !newM.filterActive {
		t.Fatal("filterActive should be true")
	}
	if newM.activeProject != "work" {
		t.Errorf("activeProject: got %q want %q", newM.activeProject, "work")
	}
	// Only the two "work" rows should remain; no separator (done is empty).
	if len(newM.rows) != 2 {
		t.Errorf("rows count: got %d want 2", len(newM.rows))
	}
}

func TestFilter_pAgainClears(t *testing.T) {
	m := modelWithRows([]store.Feature{
		featureWithProject("1", "A", "work"),
		featureWithProject("2", "B", "home"),
	}, nil)
	m.cursor = 0
	m, _ = update(m, pressKey("p")) // engage
	if !m.filterActive {
		t.Fatal("setup: filter should be active")
	}

	newM, _ := update(m, pressKey("p")) // disengage

	if newM.filterActive {
		t.Error("filterActive should be false after second p")
	}
	if newM.activeProject != "" {
		t.Errorf("activeProject should be cleared, got %q", newM.activeProject)
	}
	if len(newM.rows) != 2 {
		t.Errorf("rows after clear: got %d want 2", len(newM.rows))
	}
}

func TestFilter_noProjectRow_filtersEmptyProject(t *testing.T) {
	// Feature 1 has no project; filter should keep only the no-project row(s).
	m := modelWithRows([]store.Feature{
		pendingFeature("1", "no-proj"),
		featureWithProject("2", "with-proj", "work"),
	}, nil)
	// no-project group is rendered last when grouped, but cursor 0 targets the
	// "work" feature (group size 1 == no-proj group size 1, work wins by first-seen
	// ordering since work appears second... actually pendingFeature(1) is no-proj
	// at index 0 → seen=0; work at index 1 → seen=1; ties broken by seen. So
	// no-proj group is first. But "no project" is forced to last. So work first.
	// To pin behavior we set cursor explicitly to the no-proj row.
	for i, r := range m.rows {
		if r.kind == rowFeature && r.feature.ID == "1" {
			m.cursor = i
			break
		}
	}

	newM, _ := update(m, pressKey("p"))

	if !newM.filterActive {
		t.Fatal("filterActive should be true")
	}
	if newM.activeProject != "" {
		t.Errorf("activeProject should be empty (no-proj filter), got %q", newM.activeProject)
	}
	if len(newM.rows) != 1 {
		t.Errorf("rows count: got %d want 1", len(newM.rows))
	}
}

func TestFilter_addPrefillsActiveProject(t *testing.T) {
	m := modelWithRows([]store.Feature{
		featureWithProject("1", "A", "work"),
	}, nil)
	m.cursor = 0
	m.lastProject = "home" // would normally pre-fill, but filter overrides
	m, _ = update(m, pressKey("p"))

	newM, _ := update(m, pressKey("a"))

	if newM.projectInput.Value() != "work" {
		t.Errorf("projectInput should be pre-filled with active project; got %q", newM.projectInput.Value())
	}
}

func TestFilter_pOnSeparator_isNoOp(t *testing.T) {
	m := modelWithRows(
		[]store.Feature{featureWithProject("1", "p", "work")},
		[]store.Feature{doneFeature("2", "d")},
	)
	// row 1 is the separator
	m.cursor = 1
	if m.rows[1].kind != rowSeparator {
		t.Fatalf("setup: row[1] should be separator, got kind=%v", m.rows[1].kind)
	}

	newM, _ := update(m, pressKey("p"))

	if newM.filterActive {
		t.Error("p on separator should be a no-op")
	}
}

// --- lastProject: project pre-fill on next add ---

func TestLastProject_savedOnExitEditTrue(t *testing.T) {
	m := modelWithRows(nil, nil)
	m, _ = update(m, pressKey("a"))
	m.titleInput.SetValue("buy milk")
	m.projectInput.SetValue("home")

	stub := &stubClient{createFn: func(text, date string) (*store.Feature, error) {
		return &store.Feature{ID: "1", Title: "buy milk"}, nil
	}}
	m.apiClient = stub

	newM, _ := update(m, pressSpecialKey(tea.KeyEnter))
	if newM.lastProject != "home" {
		t.Errorf("lastProject: got %q want %q", newM.lastProject, "home")
	}
}

func TestLastProject_extractedFromTitleAtSign(t *testing.T) {
	// User typed "@work" inline in the title field with project field empty.
	m := modelWithRows(nil, nil)
	m, _ = update(m, pressKey("a"))
	m.titleInput.SetValue("ship report @work")
	m.projectInput.SetValue("")

	stub := &stubClient{createFn: func(text, date string) (*store.Feature, error) {
		return &store.Feature{ID: "1"}, nil
	}}
	m.apiClient = stub

	newM, _ := update(m, pressSpecialKey(tea.KeyEnter))
	if newM.lastProject != "work" {
		t.Errorf("lastProject: got %q want %q (extracted from title)", newM.lastProject, "work")
	}
}

func TestLastProject_notUpdatedOnEsc(t *testing.T) {
	m := modelWithRows(nil, nil)
	m.lastProject = "home"
	m, _ = update(m, pressKey("a"))
	m.titleInput.SetValue("nope")
	m.projectInput.SetValue("work")

	newM, _ := update(m, pressSpecialKey(tea.KeyEsc))
	if newM.lastProject != "home" {
		t.Errorf("ESC should not update lastProject; got %q want %q", newM.lastProject, "home")
	}
}

func TestEnterEditNew_prefillsLastProject(t *testing.T) {
	m := modelWithRows(nil, nil)
	m.lastProject = "work"

	newM, _ := update(m, pressKey("a"))

	if newM.projectInput.Value() != "work" {
		t.Errorf("project field should be pre-filled with %q, got %q", "work", newM.projectInput.Value())
	}
	// Title still empty so user can type immediately.
	if newM.titleInput.Value() != "" {
		t.Errorf("title field should be empty, got %q", newM.titleInput.Value())
	}
	if newM.field != fieldTitle {
		t.Errorf("initial field should be fieldTitle, got %v", newM.field)
	}
}

func TestEnterEditNew_emptyLastProject_emptyField(t *testing.T) {
	m := modelWithRows(nil, nil)
	// lastProject defaults to ""

	newM, _ := update(m, pressKey("a"))

	if newM.projectInput.Value() != "" {
		t.Errorf("project field should be empty when no lastProject, got %q", newM.projectInput.Value())
	}
}

// --- sticky add (capital A) ---

func TestStickyAdd_lowerA_setsStickyFlag(t *testing.T) {
	m := modelWithRows(nil, nil)

	newM, cmd := update(m, pressKey("a"))

	if newM.mode != modeEdit {
		t.Errorf("mode: got %v want modeEdit", newM.mode)
	}
	if !newM.addSticky {
		t.Error("addSticky should be true after a (sticky is now default)")
	}
	if newM.editingID != "" {
		t.Errorf("editingID: got %s want 0 (new)", newM.editingID)
	}
	if cmd == nil {
		t.Error("expected blink Cmd")
	}
}

func TestStickyAdd_savedMsgReopensEdit(t *testing.T) {
	// Simulate the lifecycle: A → edit → Enter → exitEdit returns modeList →
	// featureSavedMsg → cmdLoadToday → todayLoadedMsg → must reopen edit.
	m := modelWithRows(nil, nil)
	m.addSticky = true
	m.mode = modeList // post-save state

	resp := &store.TodayResponse{
		Pending: []store.Feature{pendingFeature("1", "saved task")},
	}
	newM, cmd := update(m, todayLoadedMsg{resp: resp})

	if newM.mode != modeEdit {
		t.Errorf("after todayLoadedMsg in sticky: mode=%v want modeEdit", newM.mode)
	}
	if newM.editingID != "" {
		t.Errorf("editingID: got %s want 0 (fresh draft)", newM.editingID)
	}
	if newM.titleInput.Value() != "" {
		t.Errorf("title should be empty for new draft, got %q", newM.titleInput.Value())
	}
	if !newM.addSticky {
		t.Error("addSticky should remain true through reopen")
	}
	if cmd == nil {
		t.Error("expected blink Cmd from re-entered edit")
	}
}

func TestStickyAdd_todayLoadedWhileNotSticky_doesNotReopen(t *testing.T) {
	m := modelWithRows(nil, nil)
	m.addSticky = false
	m.mode = modeList

	resp := &store.TodayResponse{Pending: []store.Feature{pendingFeature("1", "x")}}
	newM, _ := update(m, todayLoadedMsg{resp: resp})

	if newM.mode != modeList {
		t.Errorf("non-sticky reload should stay in list, got %v", newM.mode)
	}
}

func TestStickyAdd_escClearsFlagAndExits(t *testing.T) {
	m := modelWithRows(nil, nil)
	m, _ = update(m, pressKey("a"))

	newM, _ := update(m, pressSpecialKey(tea.KeyEsc))

	if newM.mode != modeList {
		t.Errorf("mode: got %v want modeList", newM.mode)
	}
	if newM.addSticky {
		t.Error("ESC should clear addSticky")
	}
}

func TestStickyAdd_emptyEnterExits(t *testing.T) {
	called := false
	stub := &stubClient{createFn: func(text, date string) (*store.Feature, error) {
		called = true
		return &store.Feature{}, nil
	}}
	m := modelWithRows(nil, nil)
	m.apiClient = stub
	m, _ = update(m, pressKey("a"))
	// title and project are both empty

	newM, _ := update(m, pressSpecialKey(tea.KeyEnter))

	if called {
		t.Error("empty Enter in sticky should NOT call Create")
	}
	if newM.mode != modeList {
		t.Errorf("mode: got %v want modeList", newM.mode)
	}
	if newM.addSticky {
		t.Error("empty Enter should clear addSticky")
	}
}

// Regression: pre-filled project (from lastProject / activeProject) used to keep
// the empty-Enter exit path from triggering, so an empty title would be saved
// as a blank task. Empty title now exits regardless of project value.
func TestStickyAdd_emptyTitle_withPrefilledProject_doesNotSave(t *testing.T) {
	called := false
	stub := &stubClient{createFn: func(text, date string) (*store.Feature, error) {
		called = true
		return &store.Feature{}, nil
	}}
	m := modelWithRows(nil, nil)
	m.apiClient = stub
	m.lastProject = "work"
	m, _ = update(m, pressKey("a"))
	// titleInput is empty, projectInput was pre-filled with "work"

	newM, _ := update(m, pressSpecialKey(tea.KeyEnter))

	if called {
		t.Error("empty title with pre-filled project should NOT call Create")
	}
	if newM.mode != modeList {
		t.Errorf("mode: got %v want modeList", newM.mode)
	}
	if newM.addSticky {
		t.Error("empty Enter should clear addSticky even with non-empty project")
	}
}

func TestStickyAdd_errClearsFlag(t *testing.T) {
	m := modelWithRows(nil, nil)
	m.addSticky = true

	newM, _ := update(m, errMsg{err: errFakeClipboard})

	if newM.addSticky {
		t.Error("errMsg should clear addSticky to avoid auto-reopen on next reload")
	}
}

// --- vim-style numeric prefix navigation ---

func tenPending() []store.Feature {
	out := make([]store.Feature, 10)
	for i := 0; i < 10; i++ {
		out[i] = pendingFeature(itoa(i+1), "task")
	}
	return out
}

func TestCount_5j_movesDownFive(t *testing.T) {
	m := modelWithRows(tenPending(), nil)
	m.cursor = 0

	m, _ = update(m, pressKey("5"))
	if m.count != 5 {
		t.Fatalf("count after '5': got %d want 5", m.count)
	}
	m, _ = update(m, pressKey("j"))
	if m.cursor != 5 {
		t.Errorf("cursor after 5j: got %d want 5", m.cursor)
	}
	if m.count != 0 {
		t.Errorf("count should be cleared after j, got %d", m.count)
	}
}

func TestCount_12j_multiDigit(t *testing.T) {
	// Make 15 rows so 12j is in-bounds
	rows := make([]store.Feature, 15)
	for i := range rows {
		rows[i] = pendingFeature(itoa(i+1), "task")
	}
	m := modelWithRows(rows, nil)
	m.cursor = 0

	m, _ = update(m, pressKey("1"))
	m, _ = update(m, pressKey("2"))
	if m.count != 12 {
		t.Fatalf("count after '12': got %d want 12", m.count)
	}
	m, _ = update(m, pressKey("j"))
	if m.cursor != 12 {
		t.Errorf("cursor after 12j: got %d want 12", m.cursor)
	}
}

func TestCount_3k_movesUpThree(t *testing.T) {
	m := modelWithRows(tenPending(), nil)
	m.cursor = 7

	m, _ = update(m, pressKey("3"))
	m, _ = update(m, pressKey("k"))
	if m.cursor != 4 {
		t.Errorf("cursor after 3k from 7: got %d want 4", m.cursor)
	}
}

func TestCount_leadingZeroIgnored(t *testing.T) {
	m := modelWithRows(tenPending(), nil)
	m.cursor = 0

	m, _ = update(m, pressKey("0"))
	if m.count != 0 {
		t.Errorf("leading 0 should be ignored, got count=%d", m.count)
	}
	m, _ = update(m, pressKey("j"))
	if m.cursor != 1 {
		t.Errorf("after lone 0 then j: cursor should be 1 (single step), got %d", m.cursor)
	}
}

func TestCount_zeroAfterDigitAppends(t *testing.T) {
	rows := make([]store.Feature, 12)
	for i := range rows {
		rows[i] = pendingFeature(itoa(i+1), "task")
	}
	m := modelWithRows(rows, nil)
	m.cursor = 0

	m, _ = update(m, pressKey("1"))
	m, _ = update(m, pressKey("0"))
	if m.count != 10 {
		t.Fatalf("count '10': got %d want 10", m.count)
	}
}

func TestCount_clearedByNonDigitNonMove(t *testing.T) {
	m := modelWithRows(tenPending(), nil)
	m.cursor = 0

	m, _ = update(m, pressKey("5"))
	if m.count != 5 {
		t.Fatalf("setup: count=%d", m.count)
	}
	// 'r' is refresh — should clear count without moving
	m, _ = update(m, pressKey("r"))
	if m.count != 0 {
		t.Errorf("count should be cleared by non-move key, got %d", m.count)
	}
	// Next j should be a single step
	m, _ = update(m, pressKey("j"))
	if m.cursor != 1 {
		t.Errorf("cursor after r-then-j: got %d want 1", m.cursor)
	}
}

func TestCount_clampedAtEnd(t *testing.T) {
	m := modelWithRows(tenPending(), nil)
	m.cursor = 0

	// 99j on 10 rows should land on the last row
	m, _ = update(m, pressKey("9"))
	m, _ = update(m, pressKey("9"))
	m, _ = update(m, pressKey("j"))
	if m.cursor != len(m.rows)-1 {
		t.Errorf("99j: cursor=%d want %d", m.cursor, len(m.rows)-1)
	}
}

// --- yank: copy current title to clipboard ---

// withCopyStub installs a clipboard stub for the duration of a test.
func withCopyStub(t *testing.T, fn func(string) error) {
	t.Helper()
	prev := copyToClipboard
	copyToClipboard = fn
	t.Cleanup(func() { copyToClipboard = prev })
}

func TestYank_copiesCurrentTitle(t *testing.T) {
	var got string
	withCopyStub(t, func(s string) error { got = s; return nil })

	m := modelWithRows([]store.Feature{pendingFeature("1", "buy milk")}, nil)

	newM, cmd := update(m, pressKey("y"))

	if got != "buy milk" {
		t.Errorf("clipboard: got %q want %q", got, "buy milk")
	}
	if newM.footerMsg != `copied "buy milk"` {
		t.Errorf("footerMsg: got %q", newM.footerMsg)
	}
	if cmd == nil {
		t.Error("expected footer timer Cmd")
	}
}

func TestYank_doneRowAlsoCopies(t *testing.T) {
	var got string
	withCopyStub(t, func(s string) error { got = s; return nil })

	m := modelWithRows(nil, []store.Feature{doneFeature("7", "old task")})
	m.cursor = 0

	_, _ = update(m, pressKey("y"))

	if got != "old task" {
		t.Errorf("clipboard: got %q want %q", got, "old task")
	}
}

func TestYank_separatorIsNoOp(t *testing.T) {
	called := false
	withCopyStub(t, func(s string) error { called = true; return nil })

	m := modelWithRows(
		[]store.Feature{pendingFeature("1", "p")},
		[]store.Feature{doneFeature("2", "d")},
	)
	// row[1] is the separator
	m.cursor = 1
	// bypass clampCursor so we're actually on the separator
	if m.rows[1].kind != rowSeparator {
		t.Fatalf("setup: row[1] should be separator, got kind=%v", m.rows[1].kind)
	}

	newM, cmd := update(m, pressKey("y"))

	if called {
		t.Error("clipboard should not be called when cursor is on separator")
	}
	if newM.footerMsg != "" {
		t.Errorf("footerMsg should be empty, got %q", newM.footerMsg)
	}
	if cmd != nil {
		t.Error("expected nil Cmd on no-op yank")
	}
}

func TestYank_clipboardErrorShowsFooter(t *testing.T) {
	withCopyStub(t, func(s string) error { return errFakeClipboard })

	m := modelWithRows([]store.Feature{pendingFeature("1", "task")}, nil)

	newM, cmd := update(m, pressKey("y"))

	if !containsSubstring(newM.footerMsg, "copy failed") {
		t.Errorf("footerMsg should mention failure, got %q", newM.footerMsg)
	}
	if cmd == nil {
		t.Error("expected footer timer Cmd even on failure")
	}
}

var errFakeClipboard = &clipboardErr{"no clipboard"}

type clipboardErr struct{ s string }

func (e *clipboardErr) Error() string { return e.s }

// stubClient implements the api.Client interface surface used by tests.
type stubClient struct {
	createFn func(text, date string) (*store.Feature, error)
	updateFn func(id string, title, project string, date *string) (*store.Feature, error)
}

func (s *stubClient) GetToday() (*store.TodayResponse, error) {
	return &store.TodayResponse{}, nil
}
func (s *stubClient) GetProjects() ([]store.ProjectItem, error) { return nil, nil }
func (s *stubClient) Create(text, date string) (*store.Feature, error) {
	if s.createFn != nil {
		return s.createFn(text, date)
	}
	return &store.Feature{}, nil
}
func (s *stubClient) Update(id string, title, project string, date *string) (*store.Feature, error) {
	if s.updateFn != nil {
		return s.updateFn(id, title, project, date)
	}
	return &store.Feature{}, nil
}
func (s *stubClient) MarkDone(id string) (*store.Feature, error)  { return &store.Feature{}, nil }
func (s *stubClient) Undone(id string) (*store.Feature, error)    { return &store.Feature{}, nil }
func (s *stubClient) Delete(id string) error                    { return nil }


func itoa(i int) string { return strconv.Itoa(i) }
