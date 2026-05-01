package store

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// --- parser tests ----------------------------------------------------------

func TestParse_AllFields(t *testing.T) {
	f, ok := parseLine(`- [x] write report @work +2026-04-29 *2026-04-30 [42]`)
	if !ok {
		t.Fatal("parse failed")
	}
	if f.IsDone != 1 {
		t.Errorf("IsDone: got %d want 1", f.IsDone)
	}
	if f.Title != "write report" {
		t.Errorf("Title: %q", f.Title)
	}
	if f.ProjectName == nil || *f.ProjectName != "work" {
		t.Errorf("Project: %v", f.ProjectName)
	}
	if f.CreatedAt != "2026-04-29" {
		t.Errorf("Created: %q", f.CreatedAt)
	}
	if f.CompletedAt == nil || *f.CompletedAt != "2026-04-30" {
		t.Errorf("Done: %v", f.CompletedAt)
	}
	if f.ID != "42" {
		t.Errorf("ID: %s", f.ID)
	}
}

func TestParse_OrderInsensitive(t *testing.T) {
	// fields shuffled — should still produce equivalent Feature
	f, ok := parseLine(`- [ ] buy milk +2026-05-01 @home [7]`)
	if !ok {
		t.Fatal("parse failed")
	}
	if f.Title != "buy milk" {
		t.Errorf("Title: %q", f.Title)
	}
	if f.ProjectName == nil || *f.ProjectName != "home" {
		t.Errorf("Project: %v", f.ProjectName)
	}
	if f.CreatedAt != "2026-05-01" {
		t.Errorf("Created: %q", f.CreatedAt)
	}
	if f.ID != "7" {
		t.Errorf("ID: %s", f.ID)
	}
}

func TestParse_OnlyChecked(t *testing.T) {
	// Bare line, no metadata — emulates a hand-typed mobile entry
	f, ok := parseLine(`- [ ] 买菜 @home`)
	if !ok {
		t.Fatal("parse failed")
	}
	if f.Title != "买菜" {
		t.Errorf("Title: %q", f.Title)
	}
	if f.ProjectName == nil || *f.ProjectName != "home" {
		t.Errorf("Project: %v", f.ProjectName)
	}
	if f.ID != "" {
		t.Errorf("ID should be 0 for unassigned, got %s", f.ID)
	}
	if f.CreatedAt != "" {
		t.Errorf("CreatedAt should be empty: %q", f.CreatedAt)
	}
}

func TestParse_CJKProject(t *testing.T) {
	f, ok := parseLine(`- [ ] 完成报告 @工作 +2026-05-01 [3]`)
	if !ok {
		t.Fatal("parse failed")
	}
	if f.Title != "完成报告" {
		t.Errorf("Title: %q", f.Title)
	}
	if f.ProjectName == nil || *f.ProjectName != "工作" {
		t.Errorf("Project: %v", f.ProjectName)
	}
}

func TestParse_NonTaskLine(t *testing.T) {
	if _, ok := parseLine("# A heading"); ok {
		t.Error("heading should not parse as task")
	}
	if _, ok := parseLine("just a comment"); ok {
		t.Error("plain text should not parse as task")
	}
	if _, ok := parseLine(""); ok {
		t.Error("empty line should not parse")
	}
}

func TestParse_IDOnlyAtEnd(t *testing.T) {
	// `[3 个]` is part of description, not an ID — Chinese chars don't match `\d+`
	f, ok := parseLine(`- [ ] 买苹果 [3 个] @home +2026-05-01 [12]`)
	if !ok {
		t.Fatal("parse failed")
	}
	if f.ID != "12" {
		t.Errorf("ID: got %s want 12", f.ID)
	}
	if !strings.Contains(f.Title, "[3 个]") {
		t.Errorf("Title should preserve [3 个]: %q", f.Title)
	}
}

// --- marshal tests ---------------------------------------------------------

func TestMarshal_Roundtrip(t *testing.T) {
	proj := "work"
	created := "2026-04-29"
	done := "2026-04-30"
	original := Feature{
		ID:          "42",
		Title:       "write report",
		ProjectName: &proj,
		IsDone:      1,
		CompletedAt: &done,
		CreatedAt:   created,
	}
	line := marshalLine(original)
	got, ok := parseLine(line)
	if !ok {
		t.Fatalf("parse of marshaled line failed: %q", line)
	}
	if got.ID != original.ID || got.Title != original.Title ||
		got.IsDone != original.IsDone || got.CreatedAt != original.CreatedAt {
		t.Errorf("roundtrip mismatch: in=%+v out=%+v line=%q", original, got, line)
	}
	if got.ProjectName == nil || *got.ProjectName != *original.ProjectName {
		t.Errorf("project mismatch: %v vs %v", got.ProjectName, original.ProjectName)
	}
	if got.CompletedAt == nil || *got.CompletedAt != *original.CompletedAt {
		t.Errorf("done mismatch: %v vs %v", got.CompletedAt, original.CompletedAt)
	}
}

func TestMarshal_NoProjectNoDone(t *testing.T) {
	f := Feature{ID: "1", Title: "buy milk", CreatedAt: "2026-05-01"}
	line := marshalLine(f)
	want := "- [ ] buy milk +2026-05-01 [1]"
	if line != want {
		t.Errorf("got %q want %q", line, want)
	}
}

// --- store IO tests --------------------------------------------------------

func tempStore(t *testing.T, initialContent string) *Store {
	t.Helper()
	dir := t.TempDir()
	tasksPath := filepath.Join(dir, "tasks.md")
	if err := os.WriteFile(tasksPath, []byte(initialContent), 0644); err != nil {
		t.Fatal(err)
	}
	s, err := New(tasksPath)
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func readFile(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func TestStore_LoadAssignsMissingIDs(t *testing.T) {
	s := tempStore(t, "- [ ] task one @home\n- [ ] task two\n")

	features, err := s.loadTasks()
	if err != nil {
		t.Fatal(err)
	}
	if len(features) != 2 {
		t.Fatalf("got %d features", len(features))
	}
	if features[0].ID == "" || features[1].ID == "" {
		t.Error("IDs should be assigned")
	}
	if features[0].ID == features[1].ID {
		t.Error("IDs should be distinct")
	}
	// File should be rewritten with IDs
	content := readFile(t, s.tasksPath)
	if !strings.Contains(content, "[") {
		t.Errorf("file should contain assigned IDs after sweep:\n%s", content)
	}
}

func TestStore_LoadFillsMissingCreatedDate(t *testing.T) {
	s := tempStore(t, "- [ ] no date task [99]\n")

	features, _ := s.loadTasks()
	if len(features) != 1 {
		t.Fatalf("got %d", len(features))
	}
	today := time.Now().Format("2006-01-02")
	if features[0].CreatedAt != today {
		t.Errorf("CreatedAt: got %q want %q", features[0].CreatedAt, today)
	}
}

func TestStore_LoadAutoCompletesXWithoutDate(t *testing.T) {
	// Mobile user toggled [ ] → [x] manually but didn't add *date
	s := tempStore(t, "- [x] checked off task @home +2026-05-01 [10]\n")

	features, _ := s.loadTasks()
	today := time.Now().Format("2006-01-02")
	if len(features) != 1 {
		t.Fatalf("got %d", len(features))
	}
	if features[0].CompletedAt == nil || *features[0].CompletedAt != today {
		t.Errorf("CompletedAt should be backfilled to today, got %v", features[0].CompletedAt)
	}
}

func TestStore_SweepArchivesOldDones(t *testing.T) {
	old := time.Now().AddDate(0, 0, -10).Format("2006-01-02")
	recent := time.Now().AddDate(0, 0, -3).Format("2006-01-02")
	content := "- [x] old done @home +2026-01-01 *" + old + " [1]\n" +
		"- [x] recent done @home +2026-04-15 *" + recent + " [2]\n" +
		"- [ ] still pending @work +2026-05-01 [3]\n"

	s := tempStore(t, content)

	features, _ := s.loadTasks()
	// Pending + recent done remain; old done archived
	if len(features) != 2 {
		t.Errorf("expected 2 surviving features (pending + recent done), got %d: %+v", len(features), features)
	}
	tasksContent := readFile(t, s.tasksPath)
	if strings.Contains(tasksContent, "old done") {
		t.Error("old done should be removed from tasks.md")
	}
	archiveContent := readFile(t, s.archivePath)
	if !strings.Contains(archiveContent, "old done") {
		t.Errorf("old done should be in archive.md:\n%s", archiveContent)
	}
}

// --- end-to-end Store interface tests --------------------------------------

func TestStore_GetToday_FiltersTodaysDone(t *testing.T) {
	yesterday := time.Now().AddDate(0, 0, -1).Format("2006-01-02")
	today := time.Now().Format("2006-01-02")
	content := "- [ ] pending one @home +" + today + " [1]\n" +
		"- [x] done today @work +" + today + " *" + today + " [2]\n" +
		"- [x] done yesterday @work +" + yesterday + " *" + yesterday + " [3]\n"

	s := tempStore(t, content)

	resp, err := s.GetToday()
	if err != nil {
		t.Fatal(err)
	}
	if len(resp.Pending) != 1 || resp.Pending[0].ID != "1" {
		t.Errorf("pending: %+v", resp.Pending)
	}
	if len(resp.Done) != 1 || resp.Done[0].ID != "2" {
		t.Errorf("done: %+v", resp.Done)
	}
	if resp.DoneToday != 1 {
		t.Errorf("DoneToday: %d", resp.DoneToday)
	}
}

func TestStore_Create(t *testing.T) {
	s := tempStore(t, "")

	f, err := s.Create("buy milk @home", "")
	if err != nil {
		t.Fatal(err)
	}
	if f.Title != "buy milk" {
		t.Errorf("Title: %q", f.Title)
	}
	if f.ProjectName == nil || *f.ProjectName != "home" {
		t.Errorf("Project: %v", f.ProjectName)
	}
	if f.ID == "" {
		t.Error("ID should be assigned")
	}
	content := readFile(t, s.tasksPath)
	if !strings.Contains(content, "buy milk") {
		t.Errorf("file missing task:\n%s", content)
	}
}

func TestStore_MarkDone_InPlace(t *testing.T) {
	today := time.Now().Format("2006-01-02")
	s := tempStore(t, "- [ ] buy milk @home +"+today+" [1]\n")

	_, err := s.MarkDone("1")
	if err != nil {
		t.Fatal(err)
	}

	content := readFile(t, s.tasksPath)
	if !strings.Contains(content, "[x]") {
		t.Errorf("expected [x] in tasks.md:\n%s", content)
	}
	if !strings.Contains(content, "*"+today) {
		t.Errorf("expected *today in tasks.md:\n%s", content)
	}
	// Should NOT yet be in archive (sweep keeps it for 7 days)
	archive := readFile(t, s.archivePath)
	if strings.Contains(archive, "buy milk") {
		t.Errorf("should not move to archive immediately:\n%s", archive)
	}
}

func TestStore_Undone(t *testing.T) {
	today := time.Now().Format("2006-01-02")
	s := tempStore(t, "- [x] done task @home +"+today+" *"+today+" [1]\n")

	_, err := s.Undone("1")
	if err != nil {
		t.Fatal(err)
	}
	features, _ := s.loadTasks()
	if len(features) != 1 {
		t.Fatalf("got %d", len(features))
	}
	if features[0].IsDone != 0 {
		t.Error("should be IsDone=0 after Undone")
	}
	if features[0].CompletedAt != nil {
		t.Errorf("CompletedAt should be nil, got %v", features[0].CompletedAt)
	}
}

func TestStore_Update(t *testing.T) {
	today := time.Now().Format("2006-01-02")
	s := tempStore(t, "- [ ] old title @home +"+today+" [1]\n")

	_, err := s.Update("1", "new title", "work", nil)
	if err != nil {
		t.Fatal(err)
	}
	features, _ := s.loadTasks()
	if features[0].Title != "new title" {
		t.Errorf("Title: %q", features[0].Title)
	}
	if features[0].ProjectName == nil || *features[0].ProjectName != "work" {
		t.Errorf("Project: %v", features[0].ProjectName)
	}
}

func TestStore_Delete(t *testing.T) {
	today := time.Now().Format("2006-01-02")
	s := tempStore(t, "- [ ] keep me +"+today+" [1]\n- [ ] delete me +"+today+" [2]\n")

	if err := s.Delete("2"); err != nil {
		t.Fatal(err)
	}
	features, _ := s.loadTasks()
	if len(features) != 1 || features[0].ID != "1" {
		t.Errorf("expected only id=1 remaining: %+v", features)
	}
}

func TestGenID_Hex8(t *testing.T) {
	id := genID()
	if len(id) != idLen {
		t.Errorf("genID length: got %d want %d (%q)", len(id), idLen, id)
	}
	for _, r := range id {
		if !((r >= '0' && r <= '9') || (r >= 'a' && r <= 'f')) {
			t.Errorf("genID has non-hex char %q in %q", r, id)
			break
		}
	}
	if genID() == id {
		t.Error("two consecutive genID() returned the same value (extremely unlikely)")
	}
}

// Sweep should resolve duplicate IDs by re-rolling — otherwise MarkDone-by-ID
// silently targets the wrong row (the bug that motivated this whole switch).
func TestStore_LoadTasks_ResolvesDuplicateIDs(t *testing.T) {
	today := time.Now().Format("2006-01-02")
	dup := "abcd1234"
	s := tempStore(t,
		"- [ ] first +"+today+" ["+dup+"]\n"+
			"- [ ] second +"+today+" ["+dup+"]\n")

	features, err := s.loadTasks()
	if err != nil {
		t.Fatal(err)
	}
	if len(features) != 2 {
		t.Fatalf("got %d features", len(features))
	}
	if features[0].ID == features[1].ID {
		t.Errorf("duplicate ID survived sweep: %q == %q", features[0].ID, features[1].ID)
	}
}

func TestStore_GetToday_ReturnsYesterdayDone(t *testing.T) {
	today := time.Now().Format("2006-01-02")
	yesterday := time.Now().AddDate(0, 0, -1).Format("2006-01-02")
	twoDaysAgo := time.Now().AddDate(0, 0, -2).Format("2006-01-02")

	content := "- [ ] pending task +" + today + " [1]\n" +
		"- [x] done today @work +" + today + " *" + today + " [2]\n" +
		"- [x] done yesterday @work +" + yesterday + " *" + yesterday + " [3]\n" +
		"- [x] done two days ago @home +" + twoDaysAgo + " *" + twoDaysAgo + " [4]\n"

	s := tempStore(t, content)

	resp, err := s.GetToday()
	if err != nil {
		t.Fatal(err)
	}

	if len(resp.Pending) != 1 || resp.Pending[0].ID != "1" {
		t.Errorf("Pending: %+v", resp.Pending)
	}
	if len(resp.Done) != 1 || resp.Done[0].ID != "2" {
		t.Errorf("Done (today): %+v", resp.Done)
	}
	if len(resp.DoneYesterday) != 1 || resp.DoneYesterday[0].ID != "3" {
		t.Errorf("DoneYesterday: %+v", resp.DoneYesterday)
	}
	// Two-days-ago task: not in Done or DoneYesterday (but stays in tasks.md until 7d cutoff)
	for _, f := range resp.Done {
		if f.ID == "4" {
			t.Error("two-days-ago done should not appear in Done")
		}
	}
	for _, f := range resp.DoneYesterday {
		if f.ID == "4" {
			t.Error("two-days-ago done should not appear in DoneYesterday")
		}
	}
}

func TestStore_GetProjects(t *testing.T) {
	today := time.Now().Format("2006-01-02")
	s := tempStore(t, "- [ ] a @work +"+today+" [1]\n"+
		"- [ ] b @home +"+today+" [2]\n"+
		"- [ ] c @work +"+today+" [3]\n")

	projects, err := s.GetProjects()
	if err != nil {
		t.Fatal(err)
	}
	if len(projects) != 2 {
		t.Errorf("expected 2 distinct projects, got %d: %+v", len(projects), projects)
	}
}
