package config

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const (
	defaultHost = "http://127.0.0.1:5050"
	configTemplate = `# tick TUI configuration
# Edit this file to point to your server.
TICK_HOST=http://127.0.0.1:5050
TICK_TOKEN=
`
)

// Config holds runtime configuration read from ~/.config/tick/config.
type Config struct {
	Host  string
	Token string
}

// Load reads the config file at the given path.
// If the file does not exist it creates a template and returns an error
// prompting the user to edit it.
func Load(path string) (*Config, error) {
	if _, err := os.Stat(path); os.IsNotExist(err) {
		if createErr := createTemplate(path); createErr != nil {
			return nil, fmt.Errorf("config not found and could not create template: %w", createErr)
		}
		return nil, fmt.Errorf("config created at %s — please edit it and re-run tick", path)
	}

	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open config %s: %w", path, err)
	}
	defer f.Close()

	cfg := &Config{Host: defaultHost}
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
		// Strip trailing inline comments (" # ...") so that lines like
		// TICK_TOKEN=mysecret # comment don't pollute the value.
		if idx := strings.Index(v, " #"); idx >= 0 {
			v = v[:idx]
		}
		v = strings.TrimSpace(v)
		switch strings.TrimSpace(k) {
		case "TICK_HOST":
			if v != "" {
				cfg.Host = v
			}
		case "TICK_TOKEN":
			cfg.Token = v
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

func createTemplate(path string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, []byte(configTemplate), 0o600)
}
