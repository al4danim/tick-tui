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
	Refresh   key.Binding
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
	Add:       key.NewBinding(key.WithKeys("a"), key.WithHelp("a", "add")),
	Edit:      key.NewBinding(key.WithKeys("e"), key.WithHelp("e", "edit")),
	Delete:    key.NewBinding(key.WithKeys("D"), key.WithHelp("D", "delete")),
	Refresh:   key.NewBinding(key.WithKeys("r"), key.WithHelp("r", "refresh")),
	Help:      key.NewBinding(key.WithKeys("?"), key.WithHelp("?", "help")),
	Quit:      key.NewBinding(key.WithKeys("q", "ctrl+c"), key.WithHelp("q", "quit")),
	Undo:      key.NewBinding(key.WithKeys("u"), key.WithHelp("u", "undo")),
	Yes:       key.NewBinding(key.WithKeys("y"), key.WithHelp("y", "yes")),
	Tab:       key.NewBinding(key.WithKeys("tab")),
	ShiftTab:  key.NewBinding(key.WithKeys("shift+tab")),
	Enter:     key.NewBinding(key.WithKeys("enter")),
	Escape:    key.NewBinding(key.WithKeys("esc")),
}

const shortHelp = "a add · t done · e edit · D del · r refresh · ? help · q quit"

const longHelp = `Navigation:  j/k or ↑/↓ move · ] / [ jump next/prev project
             g first · G last (within current section: pending or done) · r refresh
Actions:     a add · e edit · t mark done · U un-tick · D delete
Grace:       after t, press u within 3s to undo
Edit fields: Tab next field · Shift+Tab prev · Enter save · ESC cancel
Date field:  ↑/↓ ±1 day
Other:       ? toggle help · q quit`
