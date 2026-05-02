package config

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const configTemplate = `# tick TUI configuration
# Path to your tasks markdown file. archive.md is created in the same directory.
TICK_TASKS_FILE=
# UI language: en or zh. Toggle in-app with l.
TICK_LANG=en
`

// Config holds runtime configuration read from ~/.config/tick/config.
type Config struct {
	TasksFile string
	Lang      string // "en" or "zh"; empty == default ("en")
}

// Exists reports whether the config file is present. Used by main to decide
// between "first launch — run wizard" and "load existing config".
func Exists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

// Write creates ~/.config/tick/config (mode 0600, parent dir 0700) with the
// given tasks file path. Lang defaults to "en". This signature is preserved
// for the existing call site (setup wizard); use WriteFull to also set Lang.
func Write(path, tasksFile string) error {
	return WriteFull(path, tasksFile, "")
}

// WriteFull writes both TICK_TASKS_FILE and TICK_LANG. Empty lang is treated
// as "en" so the config file always has an explicit value.
func WriteFull(path, tasksFile, lang string) error {
	if lang == "" {
		lang = "en"
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return fmt.Errorf("create config dir: %w", err)
	}
	body := fmt.Sprintf(
		"# tick TUI configuration\n"+
			"# Path to your tasks markdown file. archive.md is created in the same directory.\n"+
			"TICK_TASKS_FILE=%s\n"+
			"# UI language: en or zh. Toggle in-app with l.\n"+
			"TICK_LANG=%s\n",
		tasksFile, lang,
	)
	return os.WriteFile(path, []byte(body), 0o600)
}

// SetLang reads the existing config (preserving TasksFile), updates only
// TICK_LANG, and rewrites the file. Used by the in-app `l` toggle.
func SetLang(path, lang string) error {
	cfg, err := Load(path)
	if err != nil {
		return err
	}
	return WriteFull(path, cfg.TasksFile, lang)
}

// Load reads the config file at the given path. If TICK_TASKS_FILE is missing
// or empty, falls back to ~/.tick/tasks.md. TICK_LANG defaults to "en".
//
// Caller is expected to check Exists() first; Load on a missing file still
// works (returns the default) but the wizard should run before reaching here
// on a true first launch.
func Load(path string) (*Config, error) {
	if _, err := os.Stat(path); os.IsNotExist(err) {
		if createErr := createTemplate(path); createErr != nil {
			return nil, fmt.Errorf("config not found and could not create template: %w", createErr)
		}
		// Don't error — just use the default. The template is there for the
		// user to override later.
	}

	cfg := &Config{TasksFile: defaultTasksFile(), Lang: "en"}

	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open config %s: %w", path, err)
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		k, v, found := strings.Cut(line, "=")
		if !found {
			continue
		}
		// Strip trailing inline comments (" # ...").
		if idx := strings.Index(v, " #"); idx >= 0 {
			v = v[:idx]
		}
		v = strings.TrimSpace(v)
		switch strings.TrimSpace(k) {
		case "TICK_TASKS_FILE":
			if v != "" {
				cfg.TasksFile = expandUser(v)
			}
		case "TICK_LANG":
			if v != "" {
				cfg.Lang = v
			}
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("scan config: %w", err)
	}
	return cfg, nil
}

// DefaultPath returns ~/.config/tick/config.
func DefaultPath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".config", "tick", "config")
}

func defaultTasksFile() string {
	home, _ := os.UserHomeDir()
	// .tick/ rather than tick/ — the dot keeps the data directory out of
	// Obsidian's default file tree (when the user opts to put it inside a
	// vault), so they can't accidentally edit a row in a way that confuses
	// the parser. Default is in $HOME (no vault) — the wizard offers vault
	// paths separately.
	return filepath.Join(home, ".tick", "tasks.md")
}

// expandUser turns a leading ~ into $HOME.
func expandUser(p string) string {
	if strings.HasPrefix(p, "~") {
		home, _ := os.UserHomeDir()
		return filepath.Join(home, strings.TrimPrefix(p, "~"))
	}
	return p
}

func createTemplate(path string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, []byte(configTemplate), 0o600)
}
