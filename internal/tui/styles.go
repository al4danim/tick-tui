package tui

import "github.com/charmbracelet/lipgloss"

var (
	styleBold      = lipgloss.NewStyle().Bold(true)
	styleDim       = lipgloss.NewStyle().Faint(true)
	styleGreen     = lipgloss.NewStyle().Foreground(lipgloss.Color("2"))
	styleCyan      = lipgloss.NewStyle().Foreground(lipgloss.Color("6"))
	styleBlue      = lipgloss.NewStyle().Foreground(lipgloss.Color("4"))
	styleGray      = lipgloss.NewStyle().Faint(true)
	styleSelected  = lipgloss.NewStyle().Bold(true)
	styleTitleBar  = lipgloss.NewStyle().Bold(true)
	styleDimCyan   = lipgloss.NewStyle().Foreground(lipgloss.Color("6")).Faint(true)
	styleUnderline = lipgloss.NewStyle().Underline(true).Foreground(lipgloss.Color("4"))
	styleError     = lipgloss.NewStyle().Foreground(lipgloss.Color("1"))
)
