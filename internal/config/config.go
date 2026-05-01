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
`

// Config holds runtime configuration read from ~/.config/tick/config.
type Config struct {
	TasksFile string
}

// Exists reports whether the config file is present. Used by main to decide
// between "first launch — run wizard" and "load existing config".
func Exists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

// Write creates ~/.config/tick/config (mode 0600, parent dir 0700) with the
// given tasks file path. Used by the setup wizard on first launch.
func Write(path, tasksFile string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return fmt.Errorf("create config dir: %w", err)
	}
	body := fmt.Sprintf("# tick TUI configuration\n# Path to your tasks markdown file. archive.md is created in the same directory.\nTICK_TASKS_FILE=%s\n", tasksFile)
	return os.WriteFile(path, []byte(body), 0o600)
}

// Load reads the config file at the given path. If TICK_TASKS_FILE is missing
// or empty, falls back to ~/.tick/tasks.md.
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

	cfg := &Config{TasksFile: defaultTasksFile()}

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
