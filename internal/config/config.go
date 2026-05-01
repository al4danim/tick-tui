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

// Load reads the config file at the given path.
// If the file does not exist it creates a template; missing TICK_TASKS_FILE
// falls back to ~/hoard/.tick/tasks.md.
func Load(path string) (*Config, error) {
	if _, err := os.Stat(path); os.IsNotExist(err) {
		if createErr := createTemplate(path); createErr != nil {
			return nil, fmt.Errorf("config not found and could not create template: %w", createErr)
		}
		// Don't error on first run — just use the default. The template is there
		// for the user to override later.
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
	// Obsidian's default file tree, so the user can't accidentally edit a
	// row in a way that confuses the parser.
	return filepath.Join(home, "hoard", ".tick", "tasks.md")
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
