package store

import (
	"os"
	"path/filepath"
	"sort"
	"testing"
	"time"
)

// tempStoreWithArchive creates a Store with both tasks.md and archive.md contents.
func tempStoreWithArchive(t *testing.T, tasksContent, archiveContent string) *Store {
	t.Helper()
	dir := t.TempDir()
	tasksPath := filepath.Join(dir, "tasks.md")
	archivePath := filepath.Join(dir, "archive.md")
	if err := os.WriteFile(tasksPath, []byte(tasksContent), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(archivePath, []byte(archiveContent), 0644); err != nil {
		t.Fatal(err)
	}
	s, err := New(tasksPath)
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func TestGetCompletionsByDate_SpansArchive(t *testing.T) {
	// tasks.md has 5 done in range; archive.md has 3 done in range.
	d1 := "2026-04-25"
	d2 := "2026-04-20"

	tasks := "" +
		"- [x] task1 @work +2026-04-25 *" + d1 + " [a0000001]\n" +
		"- [x] task2 @work +2026-04-25 *" + d1 + " [a0000002]\n" +
		"- [x] task3 @home +2026-04-25 *" + d1 + " [a0000003]\n" +
		"- [x] task4 @home +2026-04-25 *" + d1 + " [a0000004]\n" +
		"- [x] task5 @ops  +2026-04-25 *" + d1 + " [a0000005]\n" +
		"- [ ] pending1 +2026-04-26 [a0000006]\n"

	archive := "" +
		"- [x] arc1 @work +2026-04-20 *" + d2 + " [b0000001]\n" +
		"- [x] arc2 @home +2026-04-20 *" + d2 + " [b0000002]\n" +
		"- [x] arc3 @ops  +2026-04-20 *" + d2 + " [b0000003]\n" +
		"- [x] out-of-range @work +2026-01-01 *2026-01-01 [b0000004]\n"

	s := tempStoreWithArchive(t, tasks, archive)

	start := time.Date(2026, 4, 19, 0, 0, 0, 0, time.UTC)
	end := time.Date(2026, 4, 26, 0, 0, 0, 0, time.UTC)

	result, err := s.GetCompletionsByDate(start, end)
	if err != nil {
		t.Fatal(err)
	}

	if result[d1] != 5 {
		t.Errorf("count for %s: got %d want 5", d1, result[d1])
	}
	if result[d2] != 3 {
		t.Errorf("count for %s: got %d want 3", d2, result[d2])
	}
	if result["2026-01-01"] != 0 {
		t.Errorf("out-of-range date should not be counted, got %d", result["2026-01-01"])
	}
}

func TestGetCompletionsByDate_OnlyDone(t *testing.T) {
	// Pending tasks must NOT be counted even if they have a created date in range.
	today := time.Now().Format("2006-01-02")
	tasks := "" +
		"- [ ] pending @work +" + today + " [c0000001]\n" +
		"- [x] done @work +" + today + " *" + today + " [c0000002]\n"

	s := tempStoreWithArchive(t, tasks, "")

	start, _ := time.Parse("2006-01-02", today)
	end := start

	result, err := s.GetCompletionsByDate(start, end)
	if err != nil {
		t.Fatal(err)
	}

	if result[today] != 1 {
		t.Errorf("count for today: got %d want 1 (only done, not pending)", result[today])
	}
}

func TestGetCompletionsByDate_DateRangeBoundary(t *testing.T) {
	// start and end are inclusive (closed interval).
	d := "2026-03-15"
	before := "2026-03-14"
	after := "2026-03-16"

	tasks := "" +
		"- [x] on-start @work +2026-03-15 *" + d + " [d0000001]\n" +
		"- [x] before-start @work +2026-03-14 *" + before + " [d0000002]\n"
	archive := "- [x] after-end @home +2026-03-16 *" + after + " [d0000003]\n"

	s := tempStoreWithArchive(t, tasks, archive)

	start, _ := time.Parse("2006-01-02", d)
	end, _ := time.Parse("2006-01-02", d)

	result, err := s.GetCompletionsByDate(start, end)
	if err != nil {
		t.Fatal(err)
	}

	if result[d] != 1 {
		t.Errorf("boundary date count: got %d want 1", result[d])
	}
	if result[before] != 0 {
		t.Errorf("before-range date should be 0, got %d", result[before])
	}
	if result[after] != 0 {
		t.Errorf("after-range date should be 0, got %d", result[after])
	}
}

func TestGetCompletionsByDate_CJKTitleNoEffect(t *testing.T) {
	// CJK titles parse correctly and don't break count.
	d := "2026-04-10"
	tasks := "- [x] 买菜做饭 @家庭 +" + d + " *" + d + " [e0000001]\n"

	s := tempStoreWithArchive(t, tasks, "")

	start, _ := time.Parse("2006-01-02", d)
	end := start

	result, err := s.GetCompletionsByDate(start, end)
	if err != nil {
		t.Fatal(err)
	}

	if result[d] != 1 {
		t.Errorf("CJK title done task count: got %d want 1", result[d])
	}
}

func TestGetCompletionsByDate_EmptyFiles(t *testing.T) {
	s := tempStoreWithArchive(t, "", "")
	start := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	end := time.Date(2026, 12, 31, 0, 0, 0, 0, time.UTC)

	result, err := s.GetCompletionsByDate(start, end)
	if err != nil {
		t.Fatal(err)
	}
	if len(result) != 0 {
		t.Errorf("empty files should yield empty map, got %v", result)
	}
}

// ----- GetTasksOnDate tests --------------------------------------------------

func TestGetTasksOnDate_SpansBothFiles(t *testing.T) {
	d := "2026-04-25"
	tasks := "" +
		"- [x] task1 @work +2026-04-25 *" + d + " [a0000001]\n" +
		"- [x] task2 @home +2026-04-25 *" + d + " [a0000002]\n" +
		"- [ ] pending1 +2026-04-25 [a0000003]\n"
	archive := "- [x] arc1 @ops +2026-04-25 *" + d + " [b0000001]\n"

	s := tempStoreWithArchive(t, tasks, archive)
	date, _ := time.Parse("2006-01-02", d)
	result, err := s.GetTasksOnDate(date)
	if err != nil {
		t.Fatal(err)
	}
	if len(result) != 3 {
		t.Errorf("expected 3 tasks (2 tasks + 1 archive); got %d", len(result))
	}
}

func TestGetTasksOnDate_Empty(t *testing.T) {
	s := tempStoreWithArchive(t, "", "")
	date := time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC)
	result, err := s.GetTasksOnDate(date)
	if err != nil {
		t.Fatal(err)
	}
	if len(result) != 0 {
		t.Errorf("empty files should return empty slice; got %d", len(result))
	}
}

func TestGetTasksOnDate_PendingNotIncluded(t *testing.T) {
	d := "2026-04-25"
	tasks := "" +
		"- [ ] pending @work +2026-04-25 [a0000001]\n" +
		"- [x] done @work +2026-04-25 *" + d + " [a0000002]\n"

	s := tempStoreWithArchive(t, tasks, "")
	date, _ := time.Parse("2006-01-02", d)
	result, err := s.GetTasksOnDate(date)
	if err != nil {
		t.Fatal(err)
	}
	if len(result) != 1 {
		t.Errorf("pending should not appear; got %d tasks", len(result))
	}
}

func TestGetTasksOnDate_WrongDateNotIncluded(t *testing.T) {
	d := "2026-04-25"
	other := "2026-04-24"
	tasks := "" +
		"- [x] correct @work +2026-04-25 *" + d + " [a0000001]\n" +
		"- [x] wrong @work +2026-04-24 *" + other + " [a0000002]\n"

	s := tempStoreWithArchive(t, tasks, "")
	date, _ := time.Parse("2006-01-02", d)
	result, err := s.GetTasksOnDate(date)
	if err != nil {
		t.Fatal(err)
	}
	if len(result) != 1 {
		t.Errorf("only 1 task on target date; got %d", len(result))
	}
	if result[0].ID != "a0000001" {
		t.Errorf("wrong task returned: %s", result[0].ID)
	}
}

func TestGetTasksOnDate_CJKTitle(t *testing.T) {
	d := "2026-04-25"
	tasks := "- [x] 买菜做饭 @家庭 +" + d + " *" + d + " [e0000001]\n"

	s := tempStoreWithArchive(t, tasks, "")
	date, _ := time.Parse("2006-01-02", d)
	result, err := s.GetTasksOnDate(date)
	if err != nil {
		t.Fatal(err)
	}
	if len(result) != 1 {
		t.Fatalf("expected 1 CJK task; got %d", len(result))
	}
	if result[0].Title != "买菜做饭" {
		t.Errorf("CJK title: got %q want %q", result[0].Title, "买菜做饭")
	}
}

func TestGetTasksOnDate_StableSort(t *testing.T) {
	d := "2026-04-25"
	tasks := "" +
		"- [x] c @work +" + d + " *" + d + " [c0000003]\n" +
		"- [x] a @work +" + d + " *" + d + " [a0000001]\n" +
		"- [x] b @work +" + d + " *" + d + " [b0000002]\n"

	s := tempStoreWithArchive(t, tasks, "")
	date, _ := time.Parse("2006-01-02", d)
	result, err := s.GetTasksOnDate(date)
	if err != nil {
		t.Fatal(err)
	}
	if len(result) != 3 {
		t.Fatalf("expected 3 tasks; got %d", len(result))
	}

	ids := []string{result[0].ID, result[1].ID, result[2].ID}
	sorted := []string{"a0000001", "b0000002", "c0000003"}
	// ids should already be in sorted order
	if !sort.StringsAreSorted(ids) {
		t.Errorf("results not sorted by ID; got %v want %v", ids, sorted)
	}
}

// ----- OldestCompletionDate tests -------------------------------------------

func TestOldestCompletionDate_AcrossFiles(t *testing.T) {
	tasks := "" +
		"- [x] recent @work +2026-04-25 *2026-04-25 [a0000001]\n" +
		"- [x] mid @work +2026-04-20 *2026-04-20 [a0000002]\n"
	archive := "" +
		"- [x] oldest @work +2025-01-15 *2025-01-15 [b0000001]\n" +
		"- [x] middle @work +2025-06-01 *2025-06-01 [b0000002]\n"

	s := tempStoreWithArchive(t, tasks, archive)
	got, err := s.OldestCompletionDate()
	if err != nil {
		t.Fatal(err)
	}
	want, _ := time.Parse("2006-01-02", "2025-01-15")
	if !got.Equal(want) {
		t.Errorf("oldest: got %v want %v", got, want)
	}
}

func TestOldestCompletionDate_OnlyTasks(t *testing.T) {
	tasks := "- [x] one @work +2026-03-10 *2026-03-10 [a0000001]\n"
	s := tempStoreWithArchive(t, tasks, "")
	got, err := s.OldestCompletionDate()
	if err != nil {
		t.Fatal(err)
	}
	want, _ := time.Parse("2006-01-02", "2026-03-10")
	if !got.Equal(want) {
		t.Errorf("oldest: got %v want %v", got, want)
	}
}

func TestOldestCompletionDate_Empty(t *testing.T) {
	s := tempStoreWithArchive(t, "", "")
	got, err := s.OldestCompletionDate()
	if err != nil {
		t.Fatal(err)
	}
	if !got.IsZero() {
		t.Errorf("expected zero time when no completions; got %v", got)
	}
}

func TestOldestCompletionDate_PendingIgnored(t *testing.T) {
	// Pending tasks (no *date) must NOT skew the oldest date.
	tasks := "" +
		"- [ ] pending +2024-01-01 [a0000001]\n" +
		"- [x] done @work +2026-03-10 *2026-03-10 [a0000002]\n"

	s := tempStoreWithArchive(t, tasks, "")
	got, err := s.OldestCompletionDate()
	if err != nil {
		t.Fatal(err)
	}
	want, _ := time.Parse("2006-01-02", "2026-03-10")
	if !got.Equal(want) {
		t.Errorf("pending should be ignored; got %v want %v", got, want)
	}
}

// TestLoadArchiveMissingFile verifies that pure-read stats methods still work
// if archive.md is deleted after store.New(). Mobile sync conflicts and
// manual cleanup can produce this transient state.
func TestLoadArchiveMissingFile(t *testing.T) {
	tasks := "- [x] one @work +2026-03-10 *2026-03-10 [a0000001]\n"
	s := tempStoreWithArchive(t, tasks, "")

	// Remove archive.md outright.
	if err := os.Remove(s.archivePath); err != nil {
		t.Fatal(err)
	}

	// All three pure-read methods must succeed.
	if _, err := s.GetCompletionsByDate(
		time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
		time.Date(2026, 12, 31, 0, 0, 0, 0, time.UTC),
	); err != nil {
		t.Errorf("GetCompletionsByDate with missing archive: %v", err)
	}
	if _, err := s.GetTasksOnDate(time.Date(2026, 3, 10, 0, 0, 0, 0, time.UTC)); err != nil {
		t.Errorf("GetTasksOnDate with missing archive: %v", err)
	}
	if _, err := s.OldestCompletionDate(); err != nil {
		t.Errorf("OldestCompletionDate with missing archive: %v", err)
	}
}
