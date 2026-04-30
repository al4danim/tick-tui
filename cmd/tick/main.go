package main

import (
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/yaoyi/tick-tui/internal/api"
	"github.com/yaoyi/tick-tui/internal/config"
	"github.com/yaoyi/tick-tui/internal/tui"
)

func main() {
	cfgPath := config.DefaultPath()
	cfg, err := config.Load(cfgPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "tick: %v\n", err)
		os.Exit(1)
	}

	client := api.New(cfg.Host, cfg.Token)
	model := tui.NewModel(client)

	p := tea.NewProgram(
		model,
		tea.WithAltScreen(),
		tea.WithMouseCellMotion(),
	)
	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "tick: %v\n", err)
		os.Exit(1)
	}
}
