package tui

import "github.com/charmbracelet/bubbles/key"

type keyMap struct {
	Up         key.Binding
	Down       key.Binding
	NextGroup  key.Binding
	PrevGroup  key.Binding
	First      key.Binding
	Last       key.Binding
	MarkDone   key.Binding
	Untick     key.Binding
	Add        key.Binding
	Edit       key.Binding
	Delete     key.Binding
	Yank       key.Binding
	Filter     key.Binding
	Help       key.Binding
	Quit       key.Binding
	Undo       key.Binding
	Yes        key.Binding
	Tab        key.Binding
	ShiftTab   key.Binding
	Enter      key.Binding
	Escape     key.Binding
	Stats30    key.Binding // s: 30-day bar chart
	StatsYear  key.Binding // S: year heatmap
	Settings   key.Binding // O: change tasks folder
	Lang       key.Binding // l: toggle EN/ZH
}

var keys = keyMap{
	Up:         key.NewBinding(key.WithKeys("k", "up"), key.WithHelp("k/↑", "up")),
	Down:       key.NewBinding(key.WithKeys("j", "down"), key.WithHelp("j/↓", "down")),
	NextGroup:  key.NewBinding(key.WithKeys("]"), key.WithHelp("]", "next project")),
	PrevGroup:  key.NewBinding(key.WithKeys("["), key.WithHelp("[", "prev project")),
	First:      key.NewBinding(key.WithKeys("g"), key.WithHelp("g", "first in section")),
	Last:       key.NewBinding(key.WithKeys("G"), key.WithHelp("G", "last in section")),
	MarkDone:   key.NewBinding(key.WithKeys("t"), key.WithHelp("t", "done")),
	Untick:     key.NewBinding(key.WithKeys("U"), key.WithHelp("U", "un-tick")),
	Add:        key.NewBinding(key.WithKeys("a"), key.WithHelp("a", "add (streams)")),
	Edit:       key.NewBinding(key.WithKeys("e"), key.WithHelp("e", "edit")),
	Delete:     key.NewBinding(key.WithKeys("D"), key.WithHelp("D", "delete")),
	Yank:       key.NewBinding(key.WithKeys("y"), key.WithHelp("y", "copy title")),
	Filter:     key.NewBinding(key.WithKeys("p"), key.WithHelp("p", "filter project")),
	Help:       key.NewBinding(key.WithKeys("?"), key.WithHelp("?", "help")),
	Quit:       key.NewBinding(key.WithKeys("q", "ctrl+c"), key.WithHelp("q", "quit")),
	Undo:       key.NewBinding(key.WithKeys("u"), key.WithHelp("u", "undo")),
	Yes:        key.NewBinding(key.WithKeys("y"), key.WithHelp("y", "yes")),
	Tab:        key.NewBinding(key.WithKeys("tab")),
	ShiftTab:   key.NewBinding(key.WithKeys("shift+tab")),
	Enter:      key.NewBinding(key.WithKeys("enter")),
	Escape:     key.NewBinding(key.WithKeys("esc")),
	Stats30:    key.NewBinding(key.WithKeys("s"), key.WithHelp("s", "30-day chart")),
	StatsYear:  key.NewBinding(key.WithKeys("S"), key.WithHelp("S", "year heatmap")),
	Settings:   key.NewBinding(key.WithKeys("O"), key.WithHelp("O", "change folder")),
	Lang:       key.NewBinding(key.WithKeys("l"), key.WithHelp("l", "lang/语言")),
}

// footerShortHelp returns the one-line help string shown at the bottom of the
// list in normal mode. When a project filter is active it surfaces the "p clear
// filter" action prominently so the user knows how to get back to all tasks.
func footerShortHelp(m Model) string {
	if m.filterActive {
		return m.strings.ShortHelpFilter
	}
	return m.strings.ShortHelp
}
