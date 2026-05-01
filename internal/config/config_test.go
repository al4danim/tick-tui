package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoad_explicitTasksFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config")
	content := "TICK_TASKS_FILE=/tmp/somewhere/tasks.md\n"
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("unexpected: %v", err)
	}
	if cfg.TasksFile != "/tmp/somewhere/tasks.md" {
		t.Errorf("TasksFile: got %q", cfg.TasksFile)
	}
}

func TestLoad_emptyValueFallsBackToDefault(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config")
	if err := os.WriteFile(path, []byte("TICK_TASKS_FILE=\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("unexpected: %v", err)
	}
	if !strings.HasSuffix(cfg.TasksFile, "/.tick/tasks.md") {
		t.Errorf("TasksFile should fall back to default ~/.tick/tasks.md, got %q", cfg.TasksFile)
	}
}

func TestLoad_missingFileCreatesTemplate(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "subdir", "config")

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("unexpected: %v", err)
	}
	if cfg.TasksFile == "" {
		t.Error("default TasksFile should be set")
	}
	if _, statErr := os.Stat(path); os.IsNotExist(statErr) {
		t.Error("template file was not created")
	}
	data, _ := os.ReadFile(path)
	if !strings.Contains(string(data), "TICK_TASKS_FILE") {
		t.Error("template missing TICK_TASKS_FILE")
	}
}

func TestLoad_inlineComment(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config")
	content := "TICK_TASKS_FILE=/tmp/x/tasks.md  # explanatory comment\n"
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("unexpected: %v", err)
	}
	if cfg.TasksFile != "/tmp/x/tasks.md" {
		t.Errorf("TasksFile: got %q want %q", cfg.TasksFile, "/tmp/x/tasks.md")
	}
}

func TestLoad_tildeExpands(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config")
	content := "TICK_TASKS_FILE=~/some/path/tasks.md\n"
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("unexpected: %v", err)
	}
	home, _ := os.UserHomeDir()
	want := home + "/some/path/tasks.md"
	if cfg.TasksFile != want {
		t.Errorf("TasksFile: got %q want %q", cfg.TasksFile, want)
	}
}

func TestCreateTemplate_fileMode(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "subdir", "config")
	if _, err := Load(path); err != nil {
		t.Fatal(err)
	}
	info, statErr := os.Stat(path)
	if statErr != nil {
		t.Fatalf("template not created: %v", statErr)
	}
	if perm := info.Mode().Perm(); perm != 0o600 {
		t.Errorf("file mode: got %04o want 0600", perm)
	}
}

func TestExists(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config")
	if Exists(path) {
		t.Error("expected !Exists for missing file")
	}
	if err := os.WriteFile(path, []byte(""), 0o600); err != nil {
		t.Fatal(err)
	}
	if !Exists(path) {
		t.Error("expected Exists for present file")
	}
}

func TestWrite_createsFileWithCorrectMode(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "subdir", "config")
	if err := Write(path, "/abs/path/tasks.md"); err != nil {
		t.Fatalf("Write: %v", err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if perm := info.Mode().Perm(); perm != 0o600 {
		t.Errorf("file mode: got %04o want 0600", perm)
	}
	data, _ := os.ReadFile(path)
	if !strings.Contains(string(data), "TICK_TASKS_FILE=/abs/path/tasks.md") {
		t.Errorf("body should contain TICK_TASKS_FILE=...; got: %q", data)
	}

	// And Load should read what we wrote.
	cfg, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.TasksFile != "/abs/path/tasks.md" {
		t.Errorf("loaded TasksFile: got %q", cfg.TasksFile)
	}
}
