package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoad_happy(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config")
	content := "TICK_HOST=http://192.168.1.1:9000\nTICK_TOKEN=mytoken\n"
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Host != "http://192.168.1.1:9000" {
		t.Errorf("host: got %q", cfg.Host)
	}
	if cfg.Token != "mytoken" {
		t.Errorf("token: got %q", cfg.Token)
	}
}

func TestLoad_commentsSkipped(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config")
	content := "# this is a comment\nTICK_HOST=http://example.com:8080\n# another comment\nTICK_TOKEN=\n"
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Host != "http://example.com:8080" {
		t.Errorf("host: got %q", cfg.Host)
	}
	if cfg.Token != "" {
		t.Errorf("token should be empty, got %q", cfg.Token)
	}
}

func TestLoad_defaultHostWhenEmpty(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config")
	// TICK_HOST is present but blank
	if err := os.WriteFile(path, []byte("TICK_HOST=\nTICK_TOKEN=\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Host != defaultHost {
		t.Errorf("host: got %q, want default %q", cfg.Host, defaultHost)
	}
}

func TestLoad_fileNotExists_createsTemplate(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "subdir", "config")

	_, err := Load(path)
	if err == nil {
		t.Fatal("expected error when file does not exist")
	}
	if !strings.Contains(err.Error(), "please edit it") {
		t.Errorf("error should mention editing config, got: %v", err)
	}

	// Template should have been created
	if _, statErr := os.Stat(path); os.IsNotExist(statErr) {
		t.Error("template file was not created")
	}

	// Template should contain TICK_HOST
	data, readErr := os.ReadFile(path)
	if readErr != nil {
		t.Fatal(readErr)
	}
	if !strings.Contains(string(data), "TICK_HOST") {
		t.Error("template does not contain TICK_HOST")
	}
}

func TestLoad_emptyLines(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config")
	content := "\n\nTICK_HOST=http://localhost:5050\n\nTICK_TOKEN=tok\n\n"
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Token != "tok" {
		t.Errorf("token: got %q", cfg.Token)
	}
}

// --- 🟡-7: inline comment stripping ---

func TestLoad_inlineComment_host(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config")
	content := "TICK_HOST=http://x:5050  # comment\nTICK_TOKEN=\n"
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Host != "http://x:5050" {
		t.Errorf("host: got %q want %q", cfg.Host, "http://x:5050")
	}
}

func TestLoad_inlineComment_token(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config")
	content := "TICK_HOST=http://localhost:5050\nTICK_TOKEN=mysecret # not part of token\n"
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Token != "mysecret" {
		t.Errorf("token: got %q want %q", cfg.Token, "mysecret")
	}
}

// --- 🟡-8: template file mode ---

func TestCreateTemplate_fileMode(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "subdir2", "config")

	_, err := Load(path)
	// Load returns an error (prompts user to edit), but the file must be created.
	if err == nil {
		t.Fatal("expected error when template is freshly created")
	}

	info, statErr := os.Stat(path)
	if statErr != nil {
		t.Fatalf("template not created: %v", statErr)
	}
	if perm := info.Mode().Perm(); perm != 0o600 {
		t.Errorf("file mode: got %04o want 0600", perm)
	}
}
