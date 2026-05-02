package main

import (
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/al4danim/tick-tui/internal/config"
	"github.com/al4danim/tick-tui/internal/i18n"
	"github.com/al4danim/tick-tui/internal/setup"
	"github.com/al4danim/tick-tui/internal/store"
	"github.com/al4danim/tick-tui/internal/tui"
	"github.com/al4danim/tick-tui/internal/watcher"
)

// Set by goreleaser via -ldflags at build time.
var (
	version = "dev"
	commit  = "none"
	date    = "unknown"
)

func main() {
	for _, a := range os.Args[1:] {
		if a == "--version" || a == "-v" {
			fmt.Printf("tick %s (%s, %s)\n", version, commit, date)
			return
		}
	}

	cfgPath := config.DefaultPath()

	if !config.Exists(cfgPath) {
		if err := runSetup(cfgPath); err != nil {
			fmt.Fprintf(os.Stderr, "tick: %v\n", err)
			os.Exit(1)
		}
	}

	cfg, err := config.Load(cfgPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "tick: %v\n", err)
		os.Exit(1)
	}

	tasksFile := cfg.TasksFile
	if env := os.Getenv("TICK_TASKS_FILE"); env != "" {
		tasksFile = env
	}

	s, err := store.New(tasksFile)
	if err != nil {
		fmt.Fprintf(os.Stderr, "tick: open store: %v\n", err)
		os.Exit(1)
	}

	lang := i18n.ParseLang(cfg.Lang)
	model := tui.NewModel(s, lang, cfgPath)

	p := tea.NewProgram(
		model,
		tea.WithAltScreen(),
		tea.WithMouseCellMotion(),
	)

	stop, werr := watcher.Watch(tasksFile, func() {
		p.Send(tui.FileChangedMsg{})
	})
	if werr == nil {
		defer stop()
	}
	// If the watcher fails to start (rare — e.g. file system w/o inotify),
	// the TUI still works; external file changes just won't auto-reflect
	// until the next reload triggered by user action.

	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "tick: %v\n", err)
		os.Exit(1)
	}
}

// runSetup launches the first-run wizard, persists the user's choice to
// cfgPath, and returns. If the user quits without choosing (Ctrl+C) we
// abort startup — there's nothing useful to do without a tasks file.
func runSetup(cfgPath string) error {
	vaults := setup.DetectObsidianVaults()
	m := setup.NewModel(setup.LangEN, vaults)

	final, err := tea.NewProgram(m, tea.WithAltScreen()).Run()
	if err != nil {
		return fmt.Errorf("setup: %w", err)
	}
	chosen := final.(setup.Model).Chosen()
	if chosen == "" {
		return fmt.Errorf("setup cancelled")
	}
	if err := config.Write(cfgPath, chosen); err != nil {
		return fmt.Errorf("write config: %w", err)
	}
	return nil
}
