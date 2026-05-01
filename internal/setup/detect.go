// Package setup provides a first-run wizard that picks a tasks.md location.
//
// The wizard is shown only when ~/.config/tick/config does not yet exist.
// It scans for installed Obsidian vaults so the user can drop tasks.md into
// one with a single keypress (so todos sync to mobile via Obsidian Sync).
package setup

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"sort"
)

// Vault is a single Obsidian vault as registered in obsidian.json.
type Vault struct {
	Name string // basename of Path; used as the menu label
	Path string // absolute filesystem path to the vault root
}

// DetectObsidianVaults reads Obsidian's registry and returns vaults sorted
// by basename. Returns an empty slice (not an error) when Obsidian isn't
// installed or the registry is unreadable — the wizard just hides the
// vault options in that case.
func DetectObsidianVaults() []Vault {
	path, ok := obsidianRegistryPath()
	if !ok {
		return nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	return parseObsidianRegistry(data)
}

func obsidianRegistryPath() (string, bool) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", false
	}
	switch runtime.GOOS {
	case "darwin":
		return filepath.Join(home, "Library", "Application Support", "obsidian", "obsidian.json"), true
	case "linux":
		return filepath.Join(home, ".config", "obsidian", "obsidian.json"), true
	}
	return "", false
}

// parseObsidianRegistry decodes the JSON registry. Exposed for tests.
//
// Format (as of Obsidian 1.4):
//
//	{"vaults":{"<id>":{"path":"/abs/path","ts":...,"open":true},...}}
func parseObsidianRegistry(data []byte) []Vault {
	var doc struct {
		Vaults map[string]struct {
			Path string `json:"path"`
		} `json:"vaults"`
	}
	if err := json.Unmarshal(data, &doc); err != nil {
		return nil
	}
	out := make([]Vault, 0, len(doc.Vaults))
	for _, v := range doc.Vaults {
		if v.Path == "" {
			continue
		}
		out = append(out, Vault{
			Name: filepath.Base(v.Path),
			Path: v.Path,
		})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}
