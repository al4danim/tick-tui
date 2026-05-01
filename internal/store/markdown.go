package store

import (
	crand "crypto/rand"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"
)

// archiveCutoffDays — done tasks older than this get moved from tasks.md to archive.md.
const archiveCutoffDays = 7

// idLen — number of hex chars per task ID. 8 hex chars = 32 bits ≈ 4 billion
// values; collision probability stays under 0.1% well past 30k tasks (birthday
// paradox), which is more than enough headroom for a personal task list and
// avoids the long, ugly UUID-style IDs.
const idLen = 8

// Store reads/writes the markdown task files. Single-process safe via mu.
type Store struct {
	tasksPath   string
	archivePath string
	mu          sync.Mutex
}

// New opens (or creates) the task files. archivePath is derived from tasksPath.
func New(tasksPath string) (*Store, error) {
	dir := filepath.Dir(tasksPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("create dir: %w", err)
	}
	archivePath := filepath.Join(dir, "archive.md")
	if err := ensureFile(tasksPath); err != nil {
		return nil, err
	}
	if err := ensureFile(archivePath); err != nil {
		return nil, err
	}
	return &Store{tasksPath: tasksPath, archivePath: archivePath}, nil
}

func ensureFile(path string) error {
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return os.WriteFile(path, nil, 0644)
	} else if err != nil {
		return err
	}
	return nil
}

// genID returns a fresh random hex ID. With 32 random bits, two devices
// generating IDs concurrently will essentially never collide for any realistic
// number of tasks. (For the duplicates we *do* see in practice, raise idLen.)
func genID() string {
	b := make([]byte, idLen/2)
	if _, err := crand.Read(b); err != nil {
		// crypto/rand failure is exceptional — fall back to time-based to keep
		// the app usable. Time-based collisions are still rare enough for
		// single-user multi-device.
		ns := time.Now().UnixNano()
		for i := range b {
			b[i] = byte(ns >> (i * 8))
		}
	}
	return hex.EncodeToString(b)
}

// ----- parse / marshal ------------------------------------------------------

var (
	checkboxRe = regexp.MustCompile(`^- \[([ xX])\]\s+(.*)$`)
	// idRe matches the trailing [ID] token. Accepts 1-16 alphanumerics:
	// new IDs are 8 hex chars; legacy IDs are 1+ decimal digits and stay
	// readable until the next sweep rewrites them. CJK descriptions like
	// "[3 个]" don't match because the second character isn't ASCII.
	idRe       = regexp.MustCompile(`\s\[([a-zA-Z0-9]{1,16})\]\s*$`)
	projectRe  = regexp.MustCompile(`(^|\s)@(\S+)`)
	createdRe  = regexp.MustCompile(`(^|\s)\+(\d{4}-\d{2}-\d{2})(\s|$)`)
	doneRe     = regexp.MustCompile(`(^|\s)\*(\d{4}-\d{2}-\d{2})(\s|$)`)
	wsRe       = regexp.MustCompile(`\s+`)
)

// parseLine attempts to read a single markdown line as a Feature.
// Returns (zero, false) if the line doesn't start with `- [ ]` / `- [x]`.
func parseLine(line string) (Feature, bool) {
	line = strings.TrimRight(line, " \t\r\n")
	cb := checkboxRe.FindStringSubmatch(line)
	if cb == nil {
		return Feature{}, false
	}
	isDone := 0
	if cb[1] == "x" || cb[1] == "X" {
		isDone = 1
	}
	body := cb[2]
	f := Feature{IsDone: isDone}

	// ID at end
	if m := idRe.FindStringSubmatchIndex(body); m != nil {
		f.ID = body[m[2]:m[3]]
		body = body[:m[0]]
	}

	// Created date (first match)
	if m := createdRe.FindStringSubmatchIndex(body); m != nil {
		f.CreatedAt = body[m[4]:m[5]]
		body = stripRange(body, m[0], m[1])
	}

	// Done date (first match)
	if m := doneRe.FindStringSubmatchIndex(body); m != nil {
		d := body[m[4]:m[5]]
		f.CompletedAt = &d
		body = stripRange(body, m[0], m[1])
	}

	// Project (first @word; we model only one project per task)
	if m := projectRe.FindStringSubmatchIndex(body); m != nil {
		p := body[m[4]:m[5]]
		f.ProjectName = &p
		body = stripRange(body, m[0], m[1])
	}

	// Description = remaining text, whitespace-collapsed.
	body = wsRe.ReplaceAllString(body, " ")
	f.Title = strings.TrimSpace(body)
	return f, true
}

// stripRange removes body[start:end] but keeps a single space between adjacent
// tokens. The regex anchors (^|\s)…(\s|$) consume surrounding whitespace, so
// after deletion neighbours are otherwise stuck together.
func stripRange(body string, start, end int) string {
	left := body[:start]
	right := body[end:]
	leftEndsSpace := left == "" || strings.HasSuffix(left, " ")
	rightStartsSpace := right == "" || strings.HasPrefix(right, " ")
	if !leftEndsSpace && !rightStartsSpace {
		return left + " " + right
	}
	return left + right
}

// marshalLine produces the canonical written form. Field order is fixed:
// status · description · @project · +created · *done · [id].
func marshalLine(f Feature) string {
	var b strings.Builder
	if f.IsDone == 1 {
		b.WriteString("- [x] ")
	} else {
		b.WriteString("- [ ] ")
	}
	b.WriteString(f.Title)
	if f.ProjectName != nil && *f.ProjectName != "" {
		b.WriteString(" @")
		b.WriteString(*f.ProjectName)
	}
	if f.CreatedAt != "" {
		b.WriteString(" +")
		b.WriteString(f.CreatedAt)
	}
	if f.CompletedAt != nil && *f.CompletedAt != "" {
		b.WriteString(" *")
		b.WriteString(*f.CompletedAt)
	}
	if f.ID != "" {
		b.WriteString(" [")
		b.WriteString(f.ID)
		b.WriteString("]")
	}
	return b.String()
}

// ----- file IO --------------------------------------------------------------

// loadTasks reads tasks.md, runs the sweep (assign IDs, fill dates, archive
// done > 7 days), and returns the surviving features. It rewrites tasks.md and
// appends to archive.md only if the sweep changed anything.
func (s *Store) loadTasks() ([]Feature, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.loadTasksLocked()
}

func (s *Store) loadTasksLocked() ([]Feature, error) {
	raw, err := os.ReadFile(s.tasksPath)
	if err != nil {
		return nil, err
	}
	today := time.Now()
	todayStr := today.Format("2006-01-02")
	cutoff := today.AddDate(0, 0, -archiveCutoffDays).Format("2006-01-02")

	var features []Feature
	var keepLines []string
	var archived []Feature
	changed := false

	// Track IDs we've already seen this pass; if a duplicate slips in (e.g.
	// two devices both generated the same hex ID, or migration left dupes),
	// reassign the second occurrence so MarkDone-by-ID stays unambiguous.
	seenIDs := map[string]bool{}

	for _, line := range strings.Split(string(raw), "\n") {
		if strings.TrimSpace(line) == "" {
			keepLines = append(keepLines, "")
			continue
		}
		f, ok := parseLine(line)
		if !ok {
			// Preserve non-task lines (headers, separators, manual notes).
			keepLines = append(keepLines, line)
			continue
		}
		// Sweep: fill missing fields
		if f.ID == "" {
			f.ID = genID()
			changed = true
		}
		// Resolve duplicate IDs by re-rolling
		for seenIDs[f.ID] {
			f.ID = genID()
			changed = true
		}
		seenIDs[f.ID] = true
		if f.CreatedAt == "" {
			f.CreatedAt = todayStr
			changed = true
		}
		if f.IsDone == 1 && (f.CompletedAt == nil || *f.CompletedAt == "") {
			d := todayStr
			f.CompletedAt = &d
			changed = true
		}
		// Archive done tasks older than the cutoff
		if f.IsDone == 1 && f.CompletedAt != nil && *f.CompletedAt < cutoff {
			archived = append(archived, f)
			changed = true
			continue
		}
		features = append(features, f)
		// Re-marshal so any whitespace / order quirks normalize.
		keepLines = append(keepLines, marshalLine(f))
	}

	if changed {
		if err := atomicWrite(s.tasksPath, joinLines(keepLines)); err != nil {
			return nil, err
		}
		if len(archived) > 0 {
			if err := s.appendArchiveLocked(archived); err != nil {
				return nil, err
			}
		}
	}
	return features, nil
}

// loadArchive reads archive.md without any sweep.
func (s *Store) loadArchive() ([]Feature, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	raw, err := os.ReadFile(s.archivePath)
	if err != nil {
		return nil, err
	}
	var out []Feature
	for _, line := range strings.Split(string(raw), "\n") {
		if f, ok := parseLine(line); ok {
			out = append(out, f)
		}
	}
	return out, nil
}

// saveTasks writes the current feature set back to tasks.md (used by mutating
// operations after they've already taken the lock).
func (s *Store) saveTasksLocked(features []Feature) error {
	lines := make([]string, 0, len(features))
	for _, f := range features {
		lines = append(lines, marshalLine(f))
	}
	return atomicWrite(s.tasksPath, joinLines(lines))
}

func (s *Store) appendArchiveLocked(features []Feature) error {
	if len(features) == 0 {
		return nil
	}
	f, err := os.OpenFile(s.archivePath, os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	defer f.Close()
	for _, ff := range features {
		if _, err := f.WriteString(marshalLine(ff) + "\n"); err != nil {
			return err
		}
	}
	return f.Sync()
}

func atomicWrite(path string, data []byte) error {
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

func joinLines(lines []string) []byte {
	s := strings.Join(lines, "\n")
	if !strings.HasSuffix(s, "\n") {
		s += "\n"
	}
	return []byte(s)
}

// ----- Store interface methods ---------------------------------------------

// GetToday returns pending features and today's completions.
func (s *Store) GetToday() (*TodayResponse, error) {
	pending := []Feature{}
	doneToday := []Feature{}

	all, err := s.loadTasks()
	if err != nil {
		return nil, err
	}
	today := time.Now().Format("2006-01-02")
	for _, f := range all {
		if f.IsDone == 1 {
			if f.CompletedAt != nil && *f.CompletedAt == today {
				doneToday = append(doneToday, f)
			}
			// Older done (1-6 days back) live in tasks.md too but TUI doesn't show them.
			continue
		}
		pending = append(pending, f)
	}
	return &TodayResponse{
		Pending:    pending,
		Done:       doneToday,
		DoneToday:  len(doneToday),
		TotalToday: len(pending) + len(doneToday),
	}, nil
}

// GetProjects returns the distinct project names from current tasks.md.
func (s *Store) GetProjects() ([]ProjectItem, error) {
	all, err := s.loadTasks()
	if err != nil {
		return nil, err
	}
	seen := map[string]bool{}
	var out []ProjectItem
	var nextID int64 = 1
	for _, f := range all {
		if f.ProjectName == nil || *f.ProjectName == "" {
			continue
		}
		name := *f.ProjectName
		if seen[name] {
			continue
		}
		seen[name] = true
		out = append(out, ProjectItem{ID: nextID, Name: name})
		nextID++
	}
	return out, nil
}

// Create adds a new pending task. text may carry a trailing "@project" suffix
// which is split out (preserves original parse_input semantics).
func (s *Store) Create(text, date string) (*Feature, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	title, project := splitProjectSuffix(text)
	if date == "" {
		date = time.Now().Format("2006-01-02")
	}
	f := Feature{
		ID:        genID(),
		Title:     title,
		CreatedAt: date,
	}
	if project != "" {
		f.ProjectName = &project
	}
	features, err := s.loadTasksLockedSimple()
	if err != nil {
		return nil, err
	}
	features = append(features, f)
	if err := s.saveTasksLocked(features); err != nil {
		return nil, err
	}
	return &f, nil
}

// Update edits an existing task's title and project (and optionally date).
func (s *Store) Update(id string, title, project string, date *string) (*Feature, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	t, projFromTitle := splitProjectSuffix(title)
	finalProject := project
	if finalProject == "" && projFromTitle != "" {
		finalProject = projFromTitle
	}

	features, err := s.loadTasksLockedSimple()
	if err != nil {
		return nil, err
	}
	var updated *Feature
	for i, f := range features {
		if f.ID != id {
			continue
		}
		features[i].Title = t
		if finalProject != "" {
			features[i].ProjectName = &finalProject
		} else {
			features[i].ProjectName = nil
		}
		if date != nil {
			d := *date
			features[i].CompletedAt = &d
		}
		updated = &features[i]
		break
	}
	if updated == nil {
		return nil, fmt.Errorf("task %s not found", id)
	}
	if err := s.saveTasksLocked(features); err != nil {
		return nil, err
	}
	return updated, nil
}

// MarkDone flips a task's status to done (in-place; tasks.md only).
func (s *Store) MarkDone(id string) (*Feature, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	features, err := s.loadTasksLockedSimple()
	if err != nil {
		return nil, err
	}
	today := time.Now().Format("2006-01-02")
	var updated *Feature
	for i := range features {
		if features[i].ID != id {
			continue
		}
		features[i].IsDone = 1
		features[i].CompletedAt = &today
		updated = &features[i]
		break
	}
	if updated == nil {
		return nil, fmt.Errorf("task %s not found", id)
	}
	if err := s.saveTasksLocked(features); err != nil {
		return nil, err
	}
	return updated, nil
}

// Undone reverts a done task. CompletedAt cleared.
func (s *Store) Undone(id string) (*Feature, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	features, err := s.loadTasksLockedSimple()
	if err != nil {
		return nil, err
	}
	var updated *Feature
	for i := range features {
		if features[i].ID != id {
			continue
		}
		features[i].IsDone = 0
		features[i].CompletedAt = nil
		updated = &features[i]
		break
	}
	if updated == nil {
		return nil, fmt.Errorf("task %s not found", id)
	}
	if err := s.saveTasksLocked(features); err != nil {
		return nil, err
	}
	return updated, nil
}

// Delete removes a task entirely from tasks.md.
func (s *Store) Delete(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	features, err := s.loadTasksLockedSimple()
	if err != nil {
		return err
	}
	out := features[:0]
	found := false
	for _, f := range features {
		if f.ID == id {
			found = true
			continue
		}
		out = append(out, f)
	}
	if !found {
		return fmt.Errorf("task %s not found", id)
	}
	return s.saveTasksLocked(out)
}

// loadTasksLockedSimple is loadTasks WITHOUT the sweep — used by mutating ops
// that have already done the sweep via the original GetToday call. We don't
// want every Create/Update to trigger a fresh archive walk.
func (s *Store) loadTasksLockedSimple() ([]Feature, error) {
	raw, err := os.ReadFile(s.tasksPath)
	if err != nil {
		return nil, err
	}
	var out []Feature
	for _, line := range strings.Split(string(raw), "\n") {
		if f, ok := parseLine(line); ok {
			out = append(out, f)
		}
	}
	return out, nil
}

// splitProjectSuffix pulls a trailing "@project" off a title.
// Same semantics as internal/tui/editor.go:extractProjectFromTitle.
func splitProjectSuffix(s string) (string, string) {
	atRe := regexp.MustCompile(`@([^@\s]+)$`)
	m := atRe.FindStringSubmatchIndex(s)
	if m == nil {
		return strings.TrimSpace(s), ""
	}
	project := s[m[2]:m[3]]
	clean := strings.TrimSpace(s[:m[0]])
	return clean, project
}
