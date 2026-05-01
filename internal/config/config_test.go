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
	if !strings.HasSuffix(cfg.TasksFile, "/hoard/.tick/tasks.md") {
		t.Errorf("TasksFile should fall back to default ~/hoard/.tick/tasks.md, got %q", cfg.TasksFile)
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
