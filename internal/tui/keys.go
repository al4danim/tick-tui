package tui

import "github.com/charmbracelet/bubbles/key"

type keyMap struct {
	Up        key.Binding
	Down      key.Binding
	NextGroup key.Binding
	PrevGroup key.Binding
	First     key.Binding
	Last      key.Binding
	MarkDone  key.Binding
	Untick    key.Binding
	Add       key.Binding
	Edit      key.Binding
	Delete    key.Binding
	Yank      key.Binding
	Filter    key.Binding
	Help      key.Binding
	Quit      key.Binding
	Undo      key.Binding
	Yes       key.Binding
	Tab       key.Binding
	ShiftTab  key.Binding
	Enter     key.Binding
	Escape    key.Binding
}

var keys = keyMap{
	Up:        key.NewBinding(key.WithKeys("k", "up"), key.WithHelp("k/↑", "up")),
	Down:      key.NewBinding(key.WithKeys("j", "down"), key.WithHelp("j/↓", "down")),
	NextGroup: key.NewBinding(key.WithKeys("]"), key.WithHelp("]", "next project")),
	PrevGroup: key.NewBinding(key.WithKeys("["), key.WithHelp("[", "prev project")),
	First:     key.NewBinding(key.WithKeys("g"), key.WithHelp("g", "first in section")),
	Last:      key.NewBinding(key.WithKeys("G"), key.WithHelp("G", "last in section")),
	MarkDone:  key.NewBinding(key.WithKeys("t"), key.WithHelp("t", "done")),
	Untick:    key.NewBinding(key.WithKeys("U"), key.WithHelp("U", "un-tick")),
	Add:       key.NewBinding(key.WithKeys("a"), key.WithHelp("a", "add (streams)")),
	Edit:      key.NewBinding(key.WithKeys("e"), key.WithHelp("e", "edit")),
	Delete:    key.NewBinding(key.WithKeys("D"), key.WithHelp("D", "delete")),
	Yank:      key.NewBinding(key.WithKeys("y"), key.WithHelp("y", "copy title")),
	Filter:    key.NewBinding(key.WithKeys("p"), key.WithHelp("p", "filter project")),
	Help:      key.NewBinding(key.WithKeys("?"), key.WithHelp("?", "help")),
	Quit:      key.NewBinding(key.WithKeys("q", "ctrl+c"), key.WithHelp("q", "quit")),
	Undo:      key.NewBinding(key.WithKeys("u"), key.WithHelp("u", "undo")),
	Yes:       key.NewBinding(key.WithKeys("y"), key.WithHelp("y", "yes")),
	Tab:       key.NewBinding(key.WithKeys("tab")),
	ShiftTab:  key.NewBinding(key.WithKeys("shift+tab")),
	Enter:     key.NewBinding(key.WithKeys("enter")),
	Escape:    key.NewBinding(key.WithKeys("esc")),
}

const shortHelp = "a add · t done · e edit · p filter · y copy · D del · ? help · q quit"

// footerShortHelp returns the one-line help string shown at the bottom of the
// list in normal mode. When a project filter is active it surfaces the "p clear
// filter" action prominently so the user knows how to get back to all tasks.
func footerShortHelp(m Model) string {
	if m.filterActive {
		return "p clear filter · a add · t done · e edit · y copy · D del · ? help"
	}
	return shortHelp
}

const longHelp = `Navigation:  j/k or ↑/↓ move (Nj/Nk repeats N times, e.g. 5j)
             ] / [ jump next/prev project
             g first · G last (within current section: pending or done)
Actions:     a add (streams: Enter saves & opens next; Esc/empty stops)
             e edit · t mark done · U un-tick · y copy title · D delete
             p toggle project filter (uses current row's project; press again to clear)
Grace:       after t, press u within 3s to undo
Edit fields: Tab next field · Shift+Tab prev · Enter save · ESC cancel
Date field:  ↑/↓ ±1 day
Other:       ? toggle help · q quit`
