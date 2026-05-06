package tui

import (
	"sort"
	"time"

	"github.com/charmbracelet/bubbles/textinput"
	"github.com/al4danim/tick-tui/internal/i18n"
	"github.com/al4danim/tick-tui/internal/setup"
	"github.com/al4danim/tick-tui/internal/store"
)

// APIClient is the subset of *store.Store methods used by the TUI.
// (Name kept for diff stability; "Store" would be more accurate now.)
type APIClient interface {
	GetToday() (*store.TodayResponse, error)
	GetProjects() ([]store.ProjectItem, error)
	Create(text, date string) (*store.Feature, error)
	Update(id string, title, project string, date *string) (*store.Feature, error)
	MarkDone(id string) (*store.Feature, error)
	Undone(id string) (*store.Feature, error)
	Delete(id string) error
	GetCompletionsByDate(start, end time.Time) (map[string]int, error)
	GetTasksOnDate(d time.Time) ([]store.Feature, error)
	OldestCompletionDate() (time.Time, error)
	ComputeStreak(today time.Time) (int, error)
}

// mode represents which state the TUI is in.
type mode int

const (
	modeList          mode = iota
	modeEdit               // a / e (editingID==0 means new)
	modeConfirmUntick      // U pressed, waiting y/n
	modeConfirmDelete      // D pressed, waiting y/n
	modeGraceUndo          // t pressed, 3s grace period for u
	modeStats30            // s: 30-day bar chart
	modeStatsYear          // S: year heatmap
	modeSettings           // O: change tasks folder via wizard sub-model
)

// editField tracks which field is active in edit mode.
type editField int

const (
	fieldTitle   editField = iota
	fieldProject           // @project
	fieldDate              // completion date
)

// rowKind distinguishes real feature rows from the separator.
type rowKind int

const (
	rowFeature   rowKind = iota
	rowSeparator         // "─ done ─" divider
	rowDraft             // phantom row for `a` (new feature being composed)
)

// row is one entry in the rendered list.
type row struct {
	kind    rowKind
	feature store.Feature // valid only when kind==rowFeature
	daysAgo int           // 0 = today/pending, 1 = yesterday done
}

// Model is the bubbletea application model.
type Model struct {
	mode         mode
	today        store.TodayResponse
	cursor       int
	rows         []row
	field        editField
	titleInput   textinput.Model
	projectInput textinput.Model
	editDate     time.Time
	dateModified bool   // true only when user has explicitly changed the date field
	editingDone  bool   // true when editing a done feature (date-only edit mode)
	editingID    string // empty = new feature
	pendingID    string // target for U / D / grace operations
	graceID      string // feature being held in grace period
	apiClient    APIClient
	err          error
	width        int
	height       int
	footerMsg    string
	footerToken   int        // incremented by setTransientFooter; expire msgs with stale tokens are discarded
	footerErr     bool       // true when footerMsg is an error (controls red rendering)
	graceDeadline time.Time  // when the grace window expires; used for countdown display
	helpExpanded bool
	projects     []string // project names for ghost-text autocomplete
	loading      bool
	count        int    // vim-style numeric prefix: e.g. typing 5 then j moves down 5 rows
	addSticky    bool   // "a": stay in add mode after each save until ESC / empty submit
	lastProject  string // last project value submitted; pre-fills the project field on next `a`
	filterActive bool   // "p" toggle: when true, only rows matching activeProject are shown
	activeProject string // project name to filter by (empty string == "no project" group)
	pendingReload bool  // a watcher event arrived while the user was mid-flow; reload when we land back in modeList
	// Stats fields
	statsData    map[string]int // completion counts loaded by cmdLoadStats
	statsLoading bool           // true while cmdLoadStats is in-flight
	statsErr     error          // non-nil if the last stats load failed
	statsEnd     time.Time      // "today" snapshot captured when stats view was entered;
	//                              View() renders against this so labels don't drift past midnight
	//                              while the underlying statsData is still the older window.

	// Stats drill-down state — active only when in modeStats30 with selectedDate non-zero.
	selectedDate   time.Time      // zero = idle (no drill-down); non-zero = drill-down active
	selectedTasks  []store.Feature // tasks completed on selectedDate
	selectedScroll int            // scroll offset into selectedTasks (0-based)
	// statsWindowEnd is the right edge of the visible bars window.
	// Starts equal to statsEnd; shifts left when user scrolls past 30 days.
	statsWindowEnd time.Time
	streak         int       // consecutive done-day streak from today backwards (max 30+)
	oldestDataDate time.Time // earliest CompletedAt across tasks/archive; zero = unknown / no data
	// Settings fields
	settingsModel   setup.Model  // wizard sub-model for O (change folder)
	settingsChosen  string       // path selected by settingsModel; "" until confirmed
	configUpdated   bool         // true after a successful config.Write in settings
	// i18n
	lang     i18n.Lang
	strings  i18n.TUIStrings
	configPath string // for persisting language toggle; empty == skip persistence (tests)
}

// timeNow is swappable in tests to pin the clock.
var timeNow = time.Now

// NewModel builds an initial Model ready for Init. lang controls the TUI
// language; pass i18n.LangEN if not yet known. configPath is the absolute
// path used by the `l` toggle to persist language preference; pass "" to
// skip persistence (tests).
func NewModel(client APIClient, lang i18n.Lang, configPath string) Model {
	ti := textinput.New()
	ti.Placeholder = "task title  @project"
	ti.CharLimit = 200

	pi := textinput.New()
	pi.Placeholder = "project"
	pi.CharLimit = 80

	return Model{
		mode:         modeList,
		apiClient:    client,
		titleInput:   ti,
		projectInput: pi,
		width:        80,
		height:       24,
		loading:      true,
		lang:         lang,
		strings:      i18n.For(lang),
		configPath:   configPath,
	}
}

// currentFeature returns the feature at the current cursor position, or nil
// if cursor is on a separator or out of bounds.
func (m *Model) currentFeature() *store.Feature {
	if m.cursor < 0 || m.cursor >= len(m.rows) {
		return nil
	}
	r := m.rows[m.cursor]
	if r.kind == rowSeparator {
		return nil
	}
	f := r.feature
	return &f
}

// buildRows reconstructs m.rows from m.today.
// When filterActive is true, only features matching activeProject are kept
// (empty string matches the "no project" group).
// Pending features are grouped by project; groups are ordered by feature count
// descending. Within each group, original server order is preserved.
// Then a separator (only when pending > 0 AND (today done > 0 OR yesterday done > 0)),
// then today done features followed by yesterday done features (both share the same separator).
func (m *Model) buildRows() {
	pending := m.today.Pending
	done := m.today.Done
	doneYesterday := m.today.DoneYesterday
	if m.filterActive {
		pending = filterByProject(pending, m.activeProject)
		done = filterByProject(done, m.activeProject)
		doneYesterday = filterByProject(doneYesterday, m.activeProject)
	}

	totalDone := len(done) + len(doneYesterday)
	rows := make([]row, 0, len(pending)+totalDone+1)
	for _, f := range groupByProject(pending) {
		rows = append(rows, row{kind: rowFeature, feature: f})
	}
	if len(pending) > 0 && totalDone > 0 {
		rows = append(rows, row{kind: rowSeparator})
	}
	for _, f := range done {
		rows = append(rows, row{kind: rowFeature, feature: f, daysAgo: 0})
	}
	for _, f := range doneYesterday {
		rows = append(rows, row{kind: rowFeature, feature: f, daysAgo: 1})
	}
	m.rows = rows
}

// filterByProject returns features whose project matches the target.
// An empty target matches features with no project assigned.
func filterByProject(features []store.Feature, target string) []store.Feature {
	out := make([]store.Feature, 0, len(features))
	for _, f := range features {
		name := ""
		if f.ProjectName != nil {
			name = *f.ProjectName
		}
		if name == target {
			out = append(out, f)
		}
	}
	return out
}

// groupByProject returns features re-ordered so that features sharing a project
// are contiguous, and groups are ordered by group size descending.
// Ties are broken by first-appearance order in the input.
func groupByProject(features []store.Feature) []store.Feature {
	type group struct {
		name  string
		items []store.Feature
		seen  int
	}
	groups := map[string]*group{}
	var order []string
	for i, f := range features {
		name := ""
		if f.ProjectName != nil {
			name = *f.ProjectName
		}
		if g, ok := groups[name]; ok {
			g.items = append(g.items, f)
		} else {
			groups[name] = &group{name: name, items: []store.Feature{f}, seen: i}
			order = append(order, name)
		}
	}
	sort.SliceStable(order, func(i, j int) bool {
		ni, nj := order[i], order[j]
		// "no project" group always last
		if ni == "" && nj != "" {
			return false
		}
		if nj == "" && ni != "" {
			return true
		}
		gi, gj := groups[ni], groups[nj]
		if len(gi.items) != len(gj.items) {
			return len(gi.items) > len(gj.items)
		}
		return gi.seen < gj.seen
	})
	out := make([]store.Feature, 0, len(features))
	for _, name := range order {
		items := groups[name].items
		sort.SliceStable(items, func(i, j int) bool {
			return items[i].Title < items[j].Title
		})
		out = append(out, items...)
	}
	return out
}

// projectAt returns the project name for the row at index i (empty for no
// project, separator, or out of range).
func (m *Model) projectAt(i int) string {
	if i < 0 || i >= len(m.rows) {
		return ""
	}
	r := m.rows[i]
	if r.kind != rowFeature {
		return ""
	}
	if r.feature.ProjectName == nil {
		return ""
	}
	return *r.feature.ProjectName
}

// jumpSectionEdge moves cursor to the first or last row in the current section.
// Sections are bounded by rowSeparator: pending side / done side.
func (m *Model) jumpSectionEdge(toEnd bool) {
	if len(m.rows) == 0 {
		return
	}
	a := m.cursor
	for a > 0 && m.rows[a-1].kind != rowSeparator {
		a--
	}
	b := m.cursor
	for b+1 < len(m.rows) && m.rows[b+1].kind != rowSeparator {
		b++
	}
	if toEnd {
		m.cursor = b
	} else {
		m.cursor = a
	}
}

// jumpProject moves the cursor to the first row of the next/prev project group.
// direction: +1 = down, -1 = up.
func (m *Model) jumpProject(direction int) {
	if len(m.rows) == 0 {
		return
	}
	current := m.projectAt(m.cursor)
	if direction > 0 {
		for i := m.cursor + 1; i < len(m.rows); i++ {
			if m.rows[i].kind == rowSeparator {
				continue
			}
			if m.rows[i].kind == rowFeature && m.projectAt(i) != current {
				m.cursor = i
				return
			}
		}
		return
	}
	// direction < 0: find first feature row above cursor with a different project,
	// then walk back to the first row of that project group.
	prev := -1
	for i := m.cursor - 1; i >= 0; i-- {
		if m.rows[i].kind != rowFeature {
			continue
		}
		if m.projectAt(i) != current {
			prev = i
			break
		}
	}
	if prev < 0 {
		return
	}
	prevName := m.projectAt(prev)
	first := prev
	for i := prev - 1; i >= 0; i-- {
		if m.rows[i].kind != rowFeature {
			break
		}
		if m.projectAt(i) != prevName {
			break
		}
		first = i
	}
	m.cursor = first
}

// clampCursor ensures cursor stays in bounds and never lands on a separator.
func (m *Model) clampCursor() {
	if len(m.rows) == 0 {
		m.cursor = 0
		return
	}
	if m.cursor < 0 {
		m.cursor = 0
	}
	if m.cursor >= len(m.rows) {
		m.cursor = len(m.rows) - 1
	}
	// If we're on a separator, nudge down (wrap up if at bottom).
	if len(m.rows) > 0 && m.rows[m.cursor].kind == rowSeparator {
		if m.cursor+1 < len(m.rows) {
			m.cursor++
		} else if m.cursor > 0 {
			m.cursor--
		}
	}
}

// moveCursor moves cursor by delta, skipping separator rows.
func (m *Model) moveCursor(delta int) {
	if len(m.rows) == 0 {
		return
	}
	next := m.cursor + delta
	// Skip separator
	if next >= 0 && next < len(m.rows) && m.rows[next].kind == rowSeparator {
		next += delta
	}
	if next < 0 {
		next = 0
	}
	if next >= len(m.rows) {
		next = len(m.rows) - 1
	}
	// If still on separator (edge case: only a separator exists)
	if next >= 0 && next < len(m.rows) && m.rows[next].kind == rowSeparator {
		return
	}
	m.cursor = next
}
