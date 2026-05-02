package tui

import (
	"strconv"
	"strings"
	"time"

	"github.com/atotto/clipboard"
	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/al4danim/tick-tui/internal/config"
	"github.com/al4danim/tick-tui/internal/i18n"
	"github.com/al4danim/tick-tui/internal/setup"
	"github.com/al4danim/tick-tui/internal/store"
)

// setLangPersist is the hook used by `l` to write the new language to the
// config file. Tests override it so they can assert calls without touching
// the real filesystem.
var setLangPersist = config.SetLang

func itoa(i int) string { return strconv.Itoa(i) }

// sameDay reports whether two times fall on the same calendar day (in their
// own location). Use this instead of comparing formatted strings or raw
// time.Equal — selectedDate is built via AddDate from a wall-clock now and
// retains hour/min/sec, while incoming messages may carry a different instant
// for the same day.
func sameDay(a, b time.Time) bool {
	ay, am, ad := a.Date()
	by, bm, bd := b.Date()
	return ay == by && am == bm && ad == bd
}

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
type footerExpireMsg struct{ token int }
type graceTickMsg struct{ id string }
type errMsg struct{ err error }
type statsLoadedMsg struct{ data map[string]int }
type tasksOnDateLoadedMsg struct {
	date  time.Time
	tasks []store.Feature
}
type oldestDataLoadedMsg struct{ d time.Time }
type streakLoadedMsg struct{ n int }

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

func (m Model) cmdLoadStats(start, end time.Time) tea.Cmd {
	return func() tea.Msg {
		data, err := m.apiClient.GetCompletionsByDate(start, end)
		if err != nil {
			return errMsg{err}
		}
		return statsLoadedMsg{data}
	}
}

func (m Model) cmdLoadTasksOnDate(d time.Time) tea.Cmd {
	return func() tea.Msg {
		tasks, err := m.apiClient.GetTasksOnDate(d)
		if err != nil {
			return errMsg{err}
		}
		return tasksOnDateLoadedMsg{date: d, tasks: tasks}
	}
}

// cmdLoadOldestData fetches the earliest completion date once per stats entry.
// Failures fall back silently to "unbounded" (zero time) — better UX than a
// transient error every time the user opens stats.
func (m Model) cmdLoadOldestData() tea.Cmd {
	return func() tea.Msg {
		d, err := m.apiClient.OldestCompletionDate()
		if err != nil {
			return oldestDataLoadedMsg{d: time.Time{}}
		}
		return oldestDataLoadedMsg{d: d}
	}
}

// cmdLoadStreak fetches the current done-day streak from the store. The store
// scans both tasks.md and archive.md so the streak can exceed 30 days. On
// failure we fall back to 0 (rather than surfacing an error every entry); the
// label simply renders "🔥 0d" until the next stats entry retries.
func (m Model) cmdLoadStreak(today time.Time) tea.Cmd {
	return func() tea.Msg {
		n, err := m.apiClient.ComputeStreak(today)
		if err != nil {
			return streakLoadedMsg{n: 0}
		}
		return streakLoadedMsg{n: n}
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

func cmdGraceTick(id string) tea.Cmd {
	return tea.Tick(time.Second, func(t time.Time) tea.Msg {
		return graceTickMsg{id}
	})
}

// setTransientFooter sets a non-error footer message and schedules its
// automatic removal in 3s. The token mechanism ensures that a later
// confirm/grace prompt (which uses a different path without a timer) will
// NOT be cleared by a stale expire message from an earlier transient.
func (m *Model) setTransientFooter(s string) tea.Cmd {
	m.footerToken++
	m.footerMsg = s
	m.footerErr = false
	tok := m.footerToken
	return tea.Tick(3*time.Second, func(t time.Time) tea.Msg {
		return footerExpireMsg{token: tok}
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
		// Only clear when the token matches; stale timers from superseded
		// transient messages are silently discarded (fixes Bug 2).
		if msg.token == m.footerToken {
			m.footerMsg = ""
			m.footerErr = false
			m.err = nil // fixes Bug 1: m.err was never cleared
		}
		return m, nil

	case graceTickMsg:
		if m.mode == modeGraceUndo && m.graceID == msg.id {
			remaining := time.Until(m.graceDeadline)
			if remaining > 0 {
				secs := int(remaining.Seconds()) + 1 // ceiling
				if secs > 3 {
					secs = 3
				}
				m.footerMsg = m.strings.MarkedDone(secs)
				return m, cmdGraceTick(msg.id)
			}
			// Remaining ≤ 0: clear footer immediately to eliminate a 1-2 frame
			// cosmetic delay before graceExpiredMsg arrives. We deliberately
			// leave mode/graceID alone — graceExpiredMsg owns that transition.
			// Brief window where footer hides but `u` still works is intentional.
			m.footerMsg = ""
		}
		return m, nil

	case statsLoadedMsg:
		m.statsData = msg.data
		m.statsLoading = false
		m.statsErr = nil
		return m, nil

	case streakLoadedMsg:
		m.streak = msg.n
		return m, nil

	case tasksOnDateLoadedMsg:
		// Only apply if this response matches the currently selected date
		// (discard stale responses from rapidly-pressed arrow keys).
		// Compare via Y/M/D fields to avoid time-of-day drift across the
		// AddDate calls that produced selectedDate (no string formatting).
		if sameDay(msg.date, m.selectedDate) {
			m.selectedTasks = msg.tasks
		}
		return m, nil

	case oldestDataLoadedMsg:
		m.oldestDataDate = msg.d
		return m, nil

	case FileChangedMsg:
		// External edit (mobile sync, Obsidian, manual edit). Reload now if the
		// user is just browsing; otherwise queue it so we don't blow away an
		// in-flight edit, confirm prompt, or grace window.
		if m.mode == modeList {
			return m, m.cmdLoadToday()
		}
		// In stats/settings modes, queue the reload until user returns to list.
		m.pendingReload = true
		return m, nil

	case errMsg:
		m.err = msg.err
		// On any error, drop sticky-add so we don't auto-reopen edit on the next reload.
		m.addSticky = false
		// setTransientFooter resets footerErr=false, so set it true afterward.
		cmd := m.setTransientFooter(m.strings.ErrorMsg(msg.err.Error()))
		m.footerErr = true
		return m, cmd

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
	case modeStats30, modeStatsYear:
		return m.handleStatsKey(msg)
	case modeSettings:
		return m.handleSettingsKey(msg)
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
		// The "adding" status is conveyed through the title bar chip and the
		// edit footer hint (UX 1), not a transient footer message.
		m.addSticky = true
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

	case key.Matches(msg, keys.Stats30):
		return m.enterStats30()

	case key.Matches(msg, keys.StatsYear):
		return m.enterStatsYear()

	case key.Matches(msg, keys.Settings):
		return m.enterSettings()

	case key.Matches(msg, keys.Lang):
		return m.toggleLang()
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
		// setTransientFooter resets footerErr=false, so set it true afterward.
		cmd := m.setTransientFooter(m.strings.CopyFailed(err.Error()))
		m.footerErr = true
		return m, cmd
	}
	cmd := m.setTransientFooter(m.strings.CopiedTitle(f.Title))
	return m, cmd
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
	// `l` (lang toggle) is intentionally inert in grace mode — switching UI
	// language while a destructive-undo timer is counting down would be
	// surprising. We exit grace silently and drop the keypress.
	if key.Matches(msg, keys.Lang) {
		m.mode = modeList
		m.graceID = ""
		m.footerMsg = ""
		return m, nil
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
		m.footerMsg = ""       // Bug 3: clear any stale sticky-add footer message
		m.footerToken++        // invalidate any pending transient timer
		return m.exitEdit(false)

	case key.Matches(msg, keys.Enter):
		// New entries with an empty title get cancelled (also ends sticky streak).
		// project alone (e.g. pre-filled from lastProject/activeProject) is not
		// enough to mean "save" — without a title there is nothing to save.
		if m.editingID == "" && strings.TrimSpace(m.titleInput.Value()) == "" {
			m.addSticky = false
			m.footerMsg = ""   // Bug 3: clear stale footer on empty-Enter exit
			m.footerToken++    // invalidate any pending transient timer
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
	m.err = nil // clear any stale error so prompt isn't tinted red
	m.footerErr = false
	m.graceDeadline = time.Now().Add(3 * time.Second)
	m.footerMsg = m.strings.MarkedDone(3)
	// Increment token so any pending transient timer won't clear this prompt.
	m.footerToken++
	return m, tea.Batch(m.cmdMarkDone(id), cmdGraceTimer(id), cmdGraceTick(id))
}

func (m Model) handleUntick() (Model, tea.Cmd) {
	f := m.currentFeature()
	if f == nil || f.IsDone == 0 {
		return m, nil
	}
	m.mode = modeConfirmUntick
	m.pendingID = f.ID
	m.err = nil // clear stale error so prompt is not red
	m.footerErr = false
	// Increment token so any pending transient timer won't clear this prompt.
	m.footerToken++
	title := f.Title
	m.footerMsg = m.strings.UntickConfirm(title)
	return m, nil
}

func (m Model) handleDeletePrompt() (Model, tea.Cmd) {
	f := m.currentFeature()
	if f == nil {
		return m, nil
	}
	m.mode = modeConfirmDelete
	m.pendingID = f.ID
	m.err = nil // clear stale error so prompt is not red
	m.footerErr = false
	// Increment token so any pending transient timer won't clear this prompt.
	m.footerToken++
	title := f.Title
	m.footerMsg = m.strings.DeleteConfirm(title)
	return m, nil
}

// ----- stats mode -----------------------------------------------------------

func (m Model) enterStats30() (Model, tea.Cmd) {
	now := timeNow()
	end := now
	start := now.AddDate(0, 0, -29)
	m.mode = modeStats30
	m.statsLoading = true
	m.statsData = nil
	m.statsErr = nil
	m.statsEnd = end // pin "today" so the rendered axis matches the loaded window
	// Reset drill-down state on every entry (s key while in stats also resets).
	m.selectedDate = time.Time{}
	m.selectedTasks = nil
	m.selectedScroll = 0
	m.statsWindowEnd = end
	m.streak = 0
	cmds := []tea.Cmd{m.cmdLoadStats(start, end), m.cmdLoadStreak(end)}
	if m.oldestDataDate.IsZero() {
		cmds = append(cmds, m.cmdLoadOldestData())
	}
	return m, tea.Batch(cmds...)
}

func (m Model) enterStatsYear() (Model, tea.Cmd) {
	now := timeNow()
	end := now
	start := now.AddDate(0, 0, -364)
	m.mode = modeStatsYear
	m.statsLoading = true
	m.statsData = nil
	m.statsErr = nil
	m.statsEnd = end
	cmds := []tea.Cmd{m.cmdLoadStats(start, end), m.cmdLoadStreak(end)}
	if m.oldestDataDate.IsZero() {
		cmds = append(cmds, m.cmdLoadOldestData())
	}
	return m, tea.Batch(cmds...)
}

func (m Model) handleStatsKey(msg tea.KeyMsg) (Model, tea.Cmd) {
	switch {
	case key.Matches(msg, keys.Escape):
		if m.mode == modeStats30 && !m.selectedDate.IsZero() {
			// First esc: exit drill-down, return to idle stats view.
			m.selectedDate = time.Time{}
			m.selectedTasks = nil
			m.selectedScroll = 0
			return m, nil
		}
		// Second esc (or esc from year / idle stats): return to list.
		m.mode = modeList
		return m, m.drainPendingReload()

	case key.Matches(msg, keys.Quit):
		return m, tea.Quit

	case key.Matches(msg, keys.Stats30):
		// s while in either stats mode → go to 30-day (resets selection).
		return m.enterStats30()

	case key.Matches(msg, keys.StatsYear):
		// S while in either stats mode → go to year.
		return m.enterStatsYear()

	case key.Matches(msg, keys.Lang):
		return m.toggleLang()

	case msg.String() == "left":
		return m.statsScrollLeft()

	case msg.String() == "right":
		return m.statsScrollRight()

	case msg.String() == "up" || msg.String() == "k":
		if m.mode == modeStats30 && !m.selectedDate.IsZero() {
			if m.selectedScroll > 0 {
				m.selectedScroll--
			}
		}
		return m, nil

	case msg.String() == "down" || msg.String() == "j":
		if m.mode == modeStats30 && !m.selectedDate.IsZero() {
			max := len(m.selectedTasks) - 10
			if max < 0 {
				max = 0
			}
			if m.selectedScroll < max {
				m.selectedScroll++
			}
		}
		return m, nil
	}
	return m, nil
}

// statsScrollLeft moves selectedDate one day earlier (or initializes to today on first press).
// If the selected date moves outside the bars window, the window shifts left too.
func (m Model) statsScrollLeft() (Model, tea.Cmd) {
	if m.mode != modeStats30 {
		return m, nil
	}
	if m.statsWindowEnd.IsZero() {
		m.statsWindowEnd = m.statsEnd
	}

	// Compute the proposed new selected date.
	var proposed time.Time
	if m.selectedDate.IsZero() {
		// First ← enters drill-down mode at today.
		proposed = m.statsEnd
	} else {
		proposed = m.selectedDate.AddDate(0, 0, -1)
	}

	// Hard left boundary: don't let the user scroll before the earliest
	// completion in the user's data. oldestDataDate.IsZero() means we either
	// haven't loaded it yet (initial frames) or there's no data at all — in
	// both cases fall through and let the user explore freely.
	if !m.oldestDataDate.IsZero() && proposed.Before(m.oldestDataDate) && !sameDay(proposed, m.oldestDataDate) {
		cmd := m.setTransientFooter(m.strings.NoOlderData())
		return m, cmd
	}

	m.selectedDate = proposed
	// Shift window left if selectedDate is before the window's left edge.
	windowStart := m.statsWindowEnd.AddDate(0, 0, -29)
	if m.selectedDate.Before(windowStart) {
		m.statsWindowEnd = m.statsWindowEnd.AddDate(0, 0, -1)
	}
	m.selectedScroll = 0
	return m, m.cmdLoadTasksOnDate(m.selectedDate)
}

// statsScrollRight moves selectedDate one day later, stopping at today.
func (m Model) statsScrollRight() (Model, tea.Cmd) {
	if m.mode != modeStats30 || m.selectedDate.IsZero() {
		return m, nil
	}
	if m.statsWindowEnd.IsZero() {
		m.statsWindowEnd = m.statsEnd
	}

	// Don't allow selecting future dates. Compare by calendar day so identical
	// dates with different times of day (selectedDate carries clock from now)
	// don't trip the bound.
	if sameDay(m.selectedDate, m.statsEnd) {
		return m, nil
	}
	next := m.selectedDate.AddDate(0, 0, 1)
	m.selectedDate = next
	// Shift window right if selectedDate is past the window's right edge.
	if m.selectedDate.After(m.statsWindowEnd) && !sameDay(m.selectedDate, m.statsWindowEnd) {
		m.statsWindowEnd = m.selectedDate
	}
	m.selectedScroll = 0
	return m, m.cmdLoadTasksOnDate(m.selectedDate)
}


// toggleLang flips EN ↔ ZH, refreshes the strings table, and persists the
// new language to the config file. The data set is unchanged so we don't
// reload tasks. If config persistence fails we still flip in-memory and
// surface the error via a transient footer (next launch may revert, but the
// session keeps working).
func (m Model) toggleLang() (Model, tea.Cmd) {
	m.lang = m.lang.Toggle()
	m.strings = i18n.For(m.lang)
	if m.configPath == "" {
		return m, nil
	}
	if err := setLangPersist(m.configPath, m.lang.String()); err != nil {
		cmd := m.setTransientFooter(m.strings.ConfigWriteFailed(err.Error()))
		m.footerErr = true
		return m, cmd
	}
	return m, nil
}

// ----- settings mode --------------------------------------------------------

// i18nLangToSetupLang bridges the two independent Lang types. Both packages
// keep their own enum on purpose (see CLAUDE.md design decision 16); this is
// the single explicit boundary mapping.
func i18nLangToSetupLang(l i18n.Lang) setup.Lang {
	if l == i18n.LangZH {
		return setup.LangZH
	}
	return setup.LangEN
}

func (m Model) enterSettings() (Model, tea.Cmd) {
	vaults := setup.DetectObsidianVaults()
	m.mode = modeSettings
	// Inherit current TUI language so the wizard opens in the user's chosen
	// language; user can still Tab inside the wizard for that session only.
	m.settingsModel = setup.NewModel(i18nLangToSetupLang(m.lang), vaults)
	m.settingsChosen = ""
	m.configUpdated = false
	return m, nil
}

// handleSettingsKey dispatches to the wizard sub-model and translates its
// outbound signals into parent-mode transitions.
//
// Contract: the wizard's subCmd is *never* propagated. tea.Quit from the
// sub-model means "I'm done" (either chose a path or cancelled), not "kill
// the whole app". We inspect Chosen()/QuitRequested() to decide which.
//
// The parent does NOT inspect any specific key (ESC, etc.) — that would
// double-handle keys the sub-model already consumed. Sub-model state is the
// single source of truth.
func (m Model) handleSettingsKey(msg tea.KeyMsg) (Model, tea.Cmd) {
	newSubModel, subCmd := m.settingsModel.Update(msg)
	m.settingsModel = newSubModel.(setup.Model)

	// Path 1: user picked a tasks file → persist + return to list. Discard
	// subCmd (it's tea.Quit, intended only for the standalone wizard host).
	if chosen := m.settingsModel.Chosen(); chosen != "" && !m.configUpdated {
		if err := config.WriteFull(config.DefaultPath(), chosen, m.lang.String()); err != nil {
			m.mode = modeList
			cmd := m.setTransientFooter(m.strings.ConfigWriteFailed(err.Error()))
			m.footerErr = true
			return m, cmd
		}
		m.configUpdated = true
		m.settingsChosen = chosen
		m.mode = modeList
		m.footerMsg = m.strings.ConfigUpdated()
		m.footerToken++
		return m, m.drainPendingReload()
	}

	// Path 2: user cancelled (ctrl+c, or ESC from modePick). Discard subCmd
	// (also tea.Quit) so the parent app keeps running.
	if m.settingsModel.QuitRequested() {
		m.mode = modeList
		return m, m.drainPendingReload()
	}

	// Path 3: still inside the wizard (typing in custom path, navigating
	// items, ESC from modeCustom → modePick). subCmd is safe to forward
	// here — it's never tea.Quit on this path (all tea.Quit paths set
	// chosen or quitRequested above), but it may carry textinput.Blink
	// that the cursor animation needs.
	return m, subCmd
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
