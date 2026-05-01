package tui

import "github.com/charmbracelet/lipgloss"

var (
	styleBold     = lipgloss.NewStyle().Bold(true)
	styleDim      = lipgloss.NewStyle().Faint(true)
	styleGray     = lipgloss.NewStyle().Faint(true)
	styleSelected = lipgloss.NewStyle().Bold(true)
	styleTitleBar = lipgloss.NewStyle().Bold(true)
	styleError    = lipgloss.NewStyle().Foreground(lipgloss.Color("1"))
	styleChip     = lipgloss.NewStyle().Faint(true)
	styleCursor   = lipgloss.NewStyle().Reverse(true)
)
