package store

import (
	"sort"
	"time"
)

// ComputeStreak counts consecutive done-days starting at today and walking
// backwards, scanning both tasks.md and archive.md. Stops at the first day
// with zero completions, or when we walk past the earliest data (whichever
// comes first — the latter prevents an infinite loop when today itself is
// a done-day and there is no zero-day in the data set).
//
// Pure-read path: no sweep, no writes. Same locking discipline as
// GetCompletionsByDate (acquires mu briefly for tasks.md, then loadArchive
// takes the lock again internally).
func (s *Store) ComputeStreak(today time.Time) (int, error) {
	doneDays := make(map[string]bool)
	earliest := ""

	s.mu.Lock()
	tasks, err := s.loadTasksLockedSimple()
	s.mu.Unlock()
	if err != nil {
		return 0, err
	}
	for _, f := range tasks {
		if f.IsDone == 1 && f.CompletedAt != nil && *f.CompletedAt != "" {
			d := *f.CompletedAt
			doneDays[d] = true
			if earliest == "" || d < earliest {
				earliest = d
			}
		}
	}

	archived, err := s.loadArchive()
	if err != nil {
		return 0, err
	}
	for _, f := range archived {
		if f.IsDone == 1 && f.CompletedAt != nil && *f.CompletedAt != "" {
			d := *f.CompletedAt
			doneDays[d] = true
			if earliest == "" || d < earliest {
				earliest = d
			}
		}
	}

	if len(doneDays) == 0 {
		return 0, nil
	}

	// Walk backwards from today; stop at first non-done day, or when we'd
	// step before the earliest known done date.
	streak := 0
	cursor := today
	for {
		if !doneDays[cursor.Format("2006-01-02")] {
			return streak, nil
		}
		streak++
		// Bound: don't walk past earliest. earliest is non-empty here because
		// doneDays was non-empty above.
		if cursor.Format("2006-01-02") <= earliest {
			return streak, nil
		}
		cursor = cursor.AddDate(0, 0, -1)
	}
}

// OldestCompletionDate scans tasks.md + archive.md and returns the earliest
// CompletedAt date found among IsDone==1 features. Returns zero time + nil if
// there are no completed tasks anywhere. Pure-read path (no sweep).
//
// TUI uses this once on stats entry to bound left-arrow scrolling so the user
// gets a "no older data" hint instead of scrolling into pre-history.
func (s *Store) OldestCompletionDate() (time.Time, error) {
	earliest := ""

	s.mu.Lock()
	tasks, err := s.loadTasksLockedSimple()
	s.mu.Unlock()
	if err != nil {
		return time.Time{}, err
	}
	for _, f := range tasks {
		if f.IsDone == 1 && f.CompletedAt != nil && *f.CompletedAt != "" {
			if earliest == "" || *f.CompletedAt < earliest {
				earliest = *f.CompletedAt
			}
		}
	}

	archived, err := s.loadArchive()
	if err != nil {
		return time.Time{}, err
	}
	for _, f := range archived {
		if f.IsDone == 1 && f.CompletedAt != nil && *f.CompletedAt != "" {
			if earliest == "" || *f.CompletedAt < earliest {
				earliest = *f.CompletedAt
			}
		}
	}

	if earliest == "" {
		return time.Time{}, nil
	}
	t, err := time.Parse("2006-01-02", earliest)
	if err != nil {
		return time.Time{}, err
	}
	return t, nil
}

// GetTasksOnDate returns all done features (IsDone==1) whose CompletedAt date
// equals d (compared as YYYY-MM-DD). It scans both tasks.md and archive.md.
// Results are sorted by ID ascending for stable pagination.
// This is a pure-read path: no sweep, no writes.
func (s *Store) GetTasksOnDate(d time.Time) ([]Feature, error) {
	target := d.Format("2006-01-02")

	var out []Feature

	s.mu.Lock()
	tasks, err := s.loadTasksLockedSimple()
	s.mu.Unlock()
	if err != nil {
		return nil, err
	}
	for _, f := range tasks {
		if f.IsDone == 1 && f.CompletedAt != nil && *f.CompletedAt == target {
			out = append(out, f)
		}
	}

	// loadArchive acquires its own lock internally.
	archived, err := s.loadArchive()
	if err != nil {
		return nil, err
	}
	for _, f := range archived {
		if f.IsDone == 1 && f.CompletedAt != nil && *f.CompletedAt == target {
			out = append(out, f)
		}
	}

	// Stable sort by ID so the task panel doesn't reorder across reloads.
	sort.Slice(out, func(i, j int) bool {
		return out[i].ID < out[j].ID
	})

	return out, nil
}

// GetCompletionsByDate scans tasks.md + archive.md and returns a map of
// YYYY-MM-DD → count of done tasks completed on that date.
// start and end are inclusive. This is a pure-read path: it does NOT run
// the sweep (no writes, no archiving side-effects).
func (s *Store) GetCompletionsByDate(start, end time.Time) (map[string]int, error) {
	startStr := start.Format("2006-01-02")
	endStr := end.Format("2006-01-02")

	out := make(map[string]int)

	s.mu.Lock()
	tasks, err := s.loadTasksLockedSimple()
	s.mu.Unlock()
	if err != nil {
		return nil, err
	}
	for _, f := range tasks {
		if f.IsDone != 1 || f.CompletedAt == nil {
			continue
		}
		d := *f.CompletedAt
		if d >= startStr && d <= endStr {
			out[d]++
		}
	}

	// loadArchive acquires its own lock internally; call it after we released mu.
	archived, err := s.loadArchive()
	if err != nil {
		return nil, err
	}
	for _, f := range archived {
		if f.IsDone != 1 || f.CompletedAt == nil {
			continue
		}
		d := *f.CompletedAt
		if d >= startStr && d <= endStr {
			out[d]++
		}
	}

	return out, nil
}
