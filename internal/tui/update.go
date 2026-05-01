package tui

import (
	"strings"
	"time"

	"github.com/atotto/clipboard"
	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/al4danim/tick-tui/internal/store"
)

// copyToClipboard is the clipboard write hook. Tests replace it with a stub.
var copyToClipboard = clipboard.WriteAll

// ----- messages -------------------------------------------------------------

type todayLoadedMsg struct{ resp *store.TodayResponse }
type projectsLoadedMsg struct{ names []string }
type featureSavedMsg struct{ feature *store.Feature }
type featureMarkedDoneMsg struct{ feature *store.Feature }
type featureUntickedMsg struct{ feature *store.Feature }
type featureDeletedMsg struct{ id string }
type graceExpiredMsg struct{ id string }
type footerExpireMsg struct{}
type errMsg struct{ err error }

// FileChangedMsg fires when the watcher sees an external modification to
// tasks.md. The cmd/tick wiring sends it via Program.Send. Exported so
// the file-watcher goroutine in main can build it.
type FileChangedMsg struct{}

// ----- Init / Cmd builders --------------------------------------------------

// Init returns the startup commands: load today + load projects.
func (m Model) Init() tea.Cmd {
	return tea.Batch(m.cmdLoadToday(), m.cmdLoadProjects())
}

func (m Model) cmdLoadToday() tea.Cmd {
	return func() tea.Msg {
		resp, err := m.apiClient.GetToday()
		if err != nil {
			return errMsg{err}
		}
		return todayLoadedMsg{resp}
	}
}

func (m Model) cmdLoadProjects() tea.Cmd {
	return func() tea.Msg {
		items, err := m.apiClient.GetProjects()
		if err != nil {
			return projectsLoadedMsg{nil}
		}
		names := make([]string, len(items))
		for i, p := range items {
			names[i] = p.Name
		}
		return projectsLoadedMsg{names}
	}
}

func (m Model) cmdMarkDone(id string) tea.Cmd {
	return func() tea.Msg {
		f, err := m.apiClient.MarkDone(id)
		if err != nil {
			return errMsg{err}
		}
		return featureMarkedDoneMsg{f}
	}
}

func (m Model) cmdUndone(id string) tea.Cmd {
	return func() tea.Msg {
		f, err := m.apiClient.Undone(id)
		if err != nil {
			return errMsg{err}
		}
		return featureUntickedMsg{f}
	}
}

func (m Model) cmdDelete(id string) tea.Cmd {
	return func() tea.Msg {
		if err := m.apiClient.Delete(id); err != nil {
			return errMsg{err}
		}
		return featureDeletedMsg{id}
	}
}

func (m Model) cmdSave() tea.Cmd {
	titleVal := m.titleInput.Value()
	projectVal := m.projectInput.Value()

	if m.editingID == "" {
		// New feature via POST.
		// Only send a date when the user explicitly changed it; otherwise send ""
		// so the server stores NULL rather than silently setting today.
		postDate := ""
		if m.dateModified {
			postDate = m.editDate.Format("2006-01-02")
		}
		text := buildPostText(titleVal, projectVal)
		return func() tea.Msg {
			f, err := m.apiClient.Create(text, postDate)
			if err != nil {
				return errMsg{err}
			}
			return featureSavedMsg{f}
		}
	}
	// Existing feature via PUT.
	// nil date = don't send the field (server leaves existing value untouched).
	// &dateStr = send the new value.
	id := m.editingID
	var datePut *string
	if m.dateModified {
		s := m.editDate.Format("2006-01-02")
		datePut = &s
	}
	return func() tea.Msg {
		f, err := m.apiClient.Update(id, titleVal, projectVal, datePut)
		if err != nil {
			return errMsg{err}
		}
		return featureSavedMsg{f}
	}
}

func cmdGraceTimer(id string) tea.Cmd {
	return tea.Tick(3*time.Second, func(t time.Time) tea.Msg {
		return graceExpiredMsg{id}
	})
}

func cmdFooterTimer() tea.Cmd {
	return tea.Tick(3*time.Second, func(t time.Time) tea.Msg {
		return footerExpireMsg{}
	})
}

// ----- Update ---------------------------------------------------------------

// Update is the central message dispatcher.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil

	case todayLoadedMsg:
		m.loading = false
		m.err = nil
		m.today = *msg.resp
		m.pendingReload = false
		m.buildRows()
		m.clampCursor()
		// Sticky-add: a save just landed; reopen the edit panel for the next entry.
		// (Re-running enterEditNew here, after buildRows, is the only place where
		// the new draft row survives the reload-driven row rebuild.)
		if m.addSticky && m.mode == modeList {
			return m.enterEditNew()
		}
		return m, nil

	case projectsLoadedMsg:
		m.projects = msg.names
		return m, nil

	case featureSavedMsg:
		// Reload to get server-canonical state
		return m, m.cmdLoadToday()

	case featureMarkedDoneMsg:
		return m, m.cmdLoadToday()

	case featureUntickedMsg:
		return m, m.cmdLoadToday()

	case featureDeletedMsg:
		return m, m.cmdLoadToday()

	case graceExpiredMsg:
		// Only clear grace if it matches the current grace ID
		if m.mode == modeGraceUndo && m.graceID == msg.id {
			m.mode = modeList
			m.graceID = ""
			m.footerMsg = ""
			return m, m.drainPendingReload()
		}
		return m, nil

	case footerExpireMsg:
		m.footerMsg = ""
		return m, nil

	case FileChangedMsg:
		// External edit (mobile sync, Obsidian, manual edit). Reload now if the
		// user is just browsing; otherwise queue it so we don't blow away an
		// in-flight edit, confirm prompt, or grace window.
		if m.mode == modeList {
			return m, m.cmdLoadToday()
		}
		m.pendingReload = true
		return m, nil

	case errMsg:
		m.err = msg.err
		m.footerMsg = "error: " + msg.err.Error()
		// On any error, drop sticky-add so we don't auto-reopen edit on the next reload.
		m.addSticky = false
		return m, cmdFooterTimer()

	case tea.KeyMsg:
		return m.handleKey(msg)
	}
	return m, nil
}

func (m Model) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch m.mode {
	case modeList:
		return m.handleListKey(msg)
	case modeGraceUndo:
		return m.handleGraceKey(msg)
	case modeConfirmUntick:
		return m.handleConfirmUntickKey(msg)
	case modeConfirmDelete:
		return m.handleConfirmDeleteKey(msg)
	case modeEdit:
		return m.handleEditKey(msg)
	}
	return m, nil
}

// digitFromKey returns (digit, true) if msg is a single 0-9 rune.
func digitFromKey(msg tea.KeyMsg) (int, bool) {
	if msg.Type != tea.KeyRunes || len(msg.Runes) != 1 {
		return 0, false
	}
	r := msg.Runes[0]
	if r >= '0' && r <= '9' {
		return int(r - '0'), true
	}
	return 0, false
}

func (m Model) handleListKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	// Vim-style numeric prefix: digits accumulate into m.count.
	// A leading lone "0" is ignored (no zero-step), but "0" after another digit
	// is appended (so "10j" works).
	if d, ok := digitFromKey(msg); ok {
		if d == 0 && m.count == 0 {
			return m, nil
		}
		m.count = m.count*10 + d
		if m.count > 9999 {
			m.count = 9999
		}
		return m, nil
	}

	// Any non-digit key consumes the prefix.
	step := m.count
	if step < 1 {
		step = 1
	}
	m.count = 0

	switch {
	case key.Matches(msg, keys.Quit):
		return m, tea.Quit

	case key.Matches(msg, keys.Up):
		for i := 0; i < step; i++ {
			m.moveCursor(-1)
		}

	case key.Matches(msg, keys.Down):
		for i := 0; i < step; i++ {
			m.moveCursor(1)
		}

	case key.Matches(msg, keys.NextGroup):
		m.jumpProject(1)

	case key.Matches(msg, keys.PrevGroup):
		m.jumpProject(-1)

	case key.Matches(msg, keys.First):
		m.jumpSectionEdge(false)

	case key.Matches(msg, keys.Last):
		m.jumpSectionEdge(true)

	case key.Matches(msg, keys.Help):
		m.helpExpanded = !m.helpExpanded

	case key.Matches(msg, keys.Add):
		// `a` always streams: save → open next draft. ESC or empty submit ends.
		m.addSticky = true
		m.footerMsg = "+ keep adding · Esc to stop"
		return m.enterEditNew()

	case key.Matches(msg, keys.Edit):
		return m.enterEditExisting()

	case key.Matches(msg, keys.MarkDone):
		return m.handleMarkDone()

	case key.Matches(msg, keys.Untick):
		return m.handleUntick()

	case key.Matches(msg, keys.Delete):
		return m.handleDeletePrompt()

	case key.Matches(msg, keys.Yank):
		return m.handleYank()

	case key.Matches(msg, keys.Filter):
		return m.handleFilterToggle()
	}
	return m, nil
}

func (m Model) handleFilterToggle() (Model, tea.Cmd) {
	if m.filterActive {
		m.filterActive = false
		m.activeProject = ""
		m.buildRows()
		m.clampCursor()
		m.footerMsg = ""
		return m, nil
	}
	// Pick the project from the row currently under the cursor.
	f := m.currentFeature()
	if f == nil {
		// Cursor on separator or empty list: nothing to filter on.
		return m, nil
	}
	proj := ""
	if f.ProjectName != nil {
		proj = *f.ProjectName
	}
	m.filterActive = true
	m.activeProject = proj
	m.buildRows()
	m.cursor = 0
	m.clampCursor()
	return m, nil
}

func (m Model) handleYank() (Model, tea.Cmd) {
	f := m.currentFeature()
	if f == nil {
		return m, nil
	}
	if err := copyToClipboard(f.Title); err != nil {
		m.footerMsg = "copy failed: " + err.Error()
		return m, cmdFooterTimer()
	}
	m.footerMsg = `copied "` + f.Title + `"`
	return m, cmdFooterTimer()
}

// drainPendingReload returns a reload cmd if a watcher event arrived while we
// were busy. Use at every transition back into modeList that doesn't already
// trigger a reload via cmdSave / cmdUndone / cmdDelete / etc.
func (m *Model) drainPendingReload() tea.Cmd {
	if !m.pendingReload {
		return nil
	}
	m.pendingReload = false
	return m.cmdLoadToday()
}

func (m Model) handleGraceKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if key.Matches(msg, keys.Undo) {
		// Undo: reverse the done mark
		id := m.graceID
		m.mode = modeList
		m.graceID = ""
		m.footerMsg = ""
		return m, m.cmdUndone(id)
	}
	// Any other key: leave grace and process key normally in list mode
	m.mode = modeList
	m.graceID = ""
	m.footerMsg = ""
	return m.handleListKey(msg)
}

func (m Model) handleConfirmUntickKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if key.Matches(msg, keys.Yes) {
		id := m.pendingID
		m.mode = modeList
		m.pendingID = ""
		m.footerMsg = ""
		return m, m.cmdUndone(id)
	}
	// Cancel
	m.mode = modeList
	m.pendingID = ""
	m.footerMsg = ""
	return m, m.drainPendingReload()
}

func (m Model) handleConfirmDeleteKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if key.Matches(msg, keys.Yes) {
		id := m.pendingID
		m.mode = modeList
		m.pendingID = ""
		m.footerMsg = ""
		return m, m.cmdDelete(id)
	}
	// Cancel
	m.mode = modeList
	m.pendingID = ""
	m.footerMsg = ""
	return m, m.drainPendingReload()
}

func (m Model) handleEditKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch {
	case key.Matches(msg, keys.Escape):
		m.addSticky = false
		return m.exitEdit(false)

	case key.Matches(msg, keys.Enter):
		// New entries with an empty title get cancelled (also ends sticky streak).
		// project alone (e.g. pre-filled from lastProject/activeProject) is not
		// enough to mean "save" — without a title there is nothing to save.
		if m.editingID == "" && strings.TrimSpace(m.titleInput.Value()) == "" {
			m.addSticky = false
			m.footerMsg = ""
			return m.exitEdit(false)
		}
		return m.exitEdit(true)

	case key.Matches(msg, keys.Tab):
		// In project field with ghost: accept ghost first, then advance field
		if m.field == fieldProject {
			ghost := computeProjectGhost(m.projectInput.Value(), m.projects)
			if ghost != "" {
				m.projectInput.SetValue(m.projectInput.Value() + ghost)
				m.projectInput.CursorEnd()
				return m, nil
			}
		}
		// Done edit has only the date field; Tab is a no-op.
		if m.editingDone {
			return m, nil
		}
		// Pending edit cycles title <-> project.
		if m.field == fieldTitle {
			m.field = fieldProject
		} else {
			m.field = fieldTitle
		}
		m.focusField()
		return m, nil

	case key.Matches(msg, keys.ShiftTab):
		if m.editingDone {
			return m, nil
		}
		if m.field == fieldTitle {
			m.field = fieldProject
		} else {
			m.field = fieldTitle
		}
		m.focusField()
		return m, nil

	default:
		// In date field, ↑/↓ change date by ±1 day
		if m.field == fieldDate {
			switch msg.String() {
			case "up", "k":
				m.editDate = m.editDate.AddDate(0, 0, 1)
				m.dateModified = true
				return m, nil
			case "down", "j":
				m.editDate = m.editDate.AddDate(0, 0, -1)
				m.dateModified = true
				return m, nil
			}
			// Other keys do nothing in date field
			return m, nil
		}
		// Delegate to textinput
		return m.updateActiveInput(msg)
	}
}

func (m Model) updateActiveInput(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	switch m.field {
	case fieldTitle:
		m.titleInput, cmd = m.titleInput.Update(msg)
	case fieldProject:
		m.projectInput, cmd = m.projectInput.Update(msg)
	}
	return m, cmd
}

// ----- action helpers -------------------------------------------------------

func (m Model) enterEditNew() (Model, tea.Cmd) {
	m.mode = modeEdit
	m.editingID = ""
	m.editingDone = false
	m.field = fieldTitle
	m.titleInput.SetValue("")
	// Pre-fill project: filter mode forces the active project; otherwise fall
	// back to the last project the user submitted.
	defaultProj := m.lastProject
	if m.filterActive {
		defaultProj = m.activeProject
	}
	m.projectInput.SetValue(defaultProj)
	m.projectInput.CursorEnd()
	m.editDate = time.Now()
	m.dateModified = false
	m.focusField()
	// Prepend a draft phantom row at the top so existing rows shift down.
	draft := row{kind: rowDraft}
	m.rows = append([]row{draft}, m.rows...)
	m.cursor = 0
	return m, textinput.Blink
}

func (m Model) enterEditExisting() (Model, tea.Cmd) {
	f := m.currentFeature()
	if f == nil {
		return m, nil
	}
	m.mode = modeEdit
	m.editingID = f.ID
	m.editingDone = f.IsDone == 1
	m.titleInput.SetValue(f.Title)
	m.titleInput.CursorEnd()
	proj := ""
	if f.ProjectName != nil {
		proj = *f.ProjectName
	}
	m.projectInput.SetValue(proj)
	// Date: use completed_at if set, else today
	if f.CompletedAt != nil && *f.CompletedAt != "" {
		if t, err := time.Parse("2006-01-02", *f.CompletedAt); err == nil {
			m.editDate = t
		} else {
			m.editDate = time.Now()
		}
	} else {
		m.editDate = time.Now()
	}
	m.dateModified = false
	// Done feature: edit date only. Pending feature: edit title/project.
	if m.editingDone {
		m.field = fieldDate
	} else {
		m.field = fieldTitle
	}
	m.focusField()
	return m, textinput.Blink
}

func (m Model) exitEdit(save bool) (Model, tea.Cmd) {
	m.titleInput.Blur()
	m.projectInput.Blur()
	wasNew := m.editingID == ""
	m.mode = modeList

	if !save {
		m.editingID = ""
		if wasNew {
			m.buildRows()
			m.clampCursor()
		}
		return m, m.drainPendingReload()
	}
	// Remember project for next `a`/`A`. Title typed via @-suffix in the title
	// field is also captured so the next add inherits it too.
	_, projFromTitle := extractProjectFromTitle(m.titleInput.Value())
	proj := strings.TrimSpace(m.projectInput.Value())
	if proj == "" {
		proj = projFromTitle
	}
	m.lastProject = proj
	cmd := m.cmdSave()
	m.editingID = ""
	if wasNew {
		// Drop draft row; reload from server will refresh with the new feature.
		m.buildRows()
		m.clampCursor()
	}
	return m, cmd
}

func (m Model) handleMarkDone() (Model, tea.Cmd) {
	f := m.currentFeature()
	if f == nil || f.IsDone == 1 {
		return m, nil
	}
	// Guard: this feature is already in its grace window — don't fire a second request.
	if m.graceID != "" && f.ID == m.graceID {
		return m, nil
	}
	id := f.ID
	m.mode = modeGraceUndo
	m.graceID = id
	m.footerMsg = "marked done · u to undo"
	return m, tea.Batch(m.cmdMarkDone(id), cmdGraceTimer(id))
}

func (m Model) handleUntick() (Model, tea.Cmd) {
	f := m.currentFeature()
	if f == nil || f.IsDone == 0 {
		return m, nil
	}
	m.mode = modeConfirmUntick
	m.pendingID = f.ID
	title := f.Title
	m.footerMsg = `un-tick "` + title + `"? y/n`
	return m, nil
}

func (m Model) handleDeletePrompt() (Model, tea.Cmd) {
	f := m.currentFeature()
	if f == nil {
		return m, nil
	}
	m.mode = modeConfirmDelete
	m.pendingID = f.ID
	title := f.Title
	m.footerMsg = `delete "` + title + `"? y/n`
	return m, nil
}

// focusField sets focus on the appropriate textinput.
func (m *Model) focusField() {
	m.titleInput.Blur()
	m.projectInput.Blur()
	switch m.field {
	case fieldTitle:
		m.titleInput.Focus()
	case fieldProject:
		m.projectInput.Focus()
	case fieldDate:
		// No textinput for date — handled by custom ↑/↓ keys
	}
}
