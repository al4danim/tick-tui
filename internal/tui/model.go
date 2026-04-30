package tui

import (
	"sort"
	"time"

	"github.com/charmbracelet/bubbles/textinput"
	"github.com/yaoyi/tick-tui/internal/api"
)

// APIClient is the subset of *api.Client methods used by the TUI.
// Defined as an interface so tests can inject a stub without a real server.
type APIClient interface {
	GetToday() (*api.TodayResponse, error)
	GetProjects() ([]api.ProjectItem, error)
	Create(text, date string) (*api.Feature, error)
	Update(id int64, title, project string, date *string) (*api.Feature, error)
	MarkDone(id int64) (*api.Feature, error)
	Undone(id int64) (*api.Feature, error)
	Delete(id int64) error
}

// mode represents which state the TUI is in.
type mode int

const (
	modeList          mode = iota
	modeEdit               // a / e (editingID==0 means new)
	modeConfirmUntick      // U pressed, waiting y/n
	modeConfirmDelete      // D pressed, waiting y/n
	modeGraceUndo          // t pressed, 3s grace period for u
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
	feature api.Feature // valid only when kind==rowFeature
}

// Model is the bubbletea application model.
type Model struct {
	mode         mode
	today        api.TodayResponse
	cursor       int
	rows         []row
	field        editField
	titleInput   textinput.Model
	projectInput textinput.Model
	editDate     time.Time
	dateModified bool   // true only when user has explicitly changed the date field
	editingDone  bool   // true when editing a done feature (date-only edit mode)
	editingID    int64  // 0 = new feature
	pendingID    int64  // target for U / D / grace operations
	graceID      int64  // feature being held in grace period
	apiClient    APIClient
	err          error
	width        int
	height       int
	footerMsg    string
	footerExpire time.Time
	helpExpanded bool
	projects     []string // project names for ghost-text autocomplete
	loading      bool
}

// NewModel builds an initial Model ready for Init.
func NewModel(client APIClient) Model {
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
	}
}

// currentFeature returns the feature at the current cursor position, or nil
// if cursor is on a separator or out of bounds.
func (m *Model) currentFeature() *api.Feature {
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
// Pending features are grouped by project; groups are ordered by feature count
// descending. Within each group, original server order is preserved.
// Then a separator (only if both sides non-empty), then done features.
func (m *Model) buildRows() {
	rows := make([]row, 0, len(m.today.Pending)+len(m.today.Done)+1)
	for _, f := range groupByProject(m.today.Pending) {
		rows = append(rows, row{kind: rowFeature, feature: f})
	}
	if len(m.today.Pending) > 0 && len(m.today.Done) > 0 {
		rows = append(rows, row{kind: rowSeparator})
	}
	for _, f := range m.today.Done {
		rows = append(rows, row{kind: rowFeature, feature: f})
	}
	m.rows = rows
}

// groupByProject returns features re-ordered so that features sharing a project
// are contiguous, and groups are ordered by group size descending.
// Ties are broken by first-appearance order in the input.
func groupByProject(features []api.Feature) []api.Feature {
	type group struct {
		name  string
		items []api.Feature
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
			groups[name] = &group{name: name, items: []api.Feature{f}, seen: i}
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
	out := make([]api.Feature, 0, len(features))
	for _, name := range order {
		out = append(out, groups[name].items...)
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
