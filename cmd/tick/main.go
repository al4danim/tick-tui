package main

import (
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/al4danim/tick-tui/internal/config"
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
	cfg, err := config.Load(cfgPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "tick: %v\n", err)
		os.Exit(1)
	}

	s, err := store.New(cfg.TasksFile)
	if err != nil {
		fmt.Fprintf(os.Stderr, "tick: open store: %v\n", err)
		os.Exit(1)
	}

	model := tui.NewModel(s)

	p := tea.NewProgram(
		model,
		tea.WithAltScreen(),
		tea.WithMouseCellMotion(),
	)

	stop, werr := watcher.Watch(cfg.TasksFile, func() {
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
