package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/al4danim/tick-tui/internal/store"
)

// View renders the full TUI screen.
func (m Model) View() string {
	if m.width == 0 {
		return ""
	}

	var sb strings.Builder
	sb.WriteString(m.renderTitleBar())
	sb.WriteByte('\n')
	sb.WriteString(m.renderList())
	sb.WriteByte('\n')
	sb.WriteString(m.renderFooter())
	return sb.String()
}

func (m Model) renderTitleBar() string {
	left := styleTitleBar.Render("Tick")
	right := styleDim.Render(fmt.Sprintf(" · %d/%d done", m.today.DoneToday, m.today.TotalToday))
	bar := left + right
	if m.filterActive {
		label := "@" + m.activeProject
		if m.activeProject == "" {
			label = "(no project)"
		}
		bar += styleBold.Render(" · " + label)
	}
	return lipgloss.NewStyle().MaxWidth(m.width).Render(bar)
}

func (m Model) renderList() string {
	// Calculate available height for list rows
	footerLines := 1
	if m.helpExpanded {
		footerLines = strings.Count(longHelp, "\n") + 2
	}
	// 2 = title bar + blank line
	availableRows := m.height - 2 - footerLines
	if availableRows < 1 {
		availableRows = 1
	}

	if m.loading {
		return styleDim.Render("loading…")
	}

	if len(m.rows) == 0 {
		return styleGray.Render("no tasks for today · press a to add")
	}

	lines := m.buildListLines()

	// Scroll window: keep cursor visible
	start, end := scrollWindow(m.cursor, len(lines), availableRows)

	var sb strings.Builder
	for i := start; i < end; i++ {
		sb.WriteString(lines[i])
		if i < end-1 {
			sb.WriteByte('\n')
		}
	}
	return sb.String()
}

// buildListLines converts m.rows into rendered strings, handling edit mode.
func (m Model) buildListLines() []string {
	lines := make([]string, 0, len(m.rows))
	for i, r := range m.rows {
		switch r.kind {
		case rowSeparator:
			lines = append(lines, renderSeparator(m.width))
		case rowDraft:
			// Draft is always selected + always in edit mode.
			lines = append(lines, m.renderFeatureLine(store.Feature{}, true, true, true))
		default:
			selected := i == m.cursor
			inEdit := m.mode == modeEdit && selected
			lines = append(lines, m.renderFeatureLine(r.feature, selected, inEdit, false))
		}
	}
	return lines
}

func chip(label string) string {
	return styleChip.Render("[" + label + "]")
}

func (m Model) renderFeatureLine(f store.Feature, selected, inEdit, isDraft bool) string {
	isDone := f.IsDone == 1

	checkbox := "[ ]"
	if isDone {
		checkbox = "[x]"
	}

	prefix := "  "
	switch {
	case isDraft:
		prefix = styleBold.Render("+ ")
	case selected:
		prefix = styleSelected.Render("› ")
	}

	var titlePart, rightPart string

	if inEdit {
		switch m.field {
		case fieldTitle:
			titlePart = renderTitleWithGhost(m.titleInput.Value(), m.titleInput.Position(), "", true)
			// Show the current project alongside the field label so the user can
			// see what will be submitted without Tab-ing into the project field.
			if proj := strings.TrimSpace(m.projectInput.Value()); proj != "" {
				rightPart = chip("title → @" + proj)
			} else {
				rightPart = chip("title")
			}
		case fieldProject:
			titlePart = m.titleInput.Value()
			if titlePart == "" {
				titlePart = styleDim.Render("(title)")
			}
			rightPart = renderProjectField(m.projectInput.Value(), m.projects, true) + " " + chip("proj")
		case fieldDate:
			titlePart = f.Title
			rightPart = m.editDate.Format("2006-01-02") + " " + chip("date")
		}
	} else {
		titlePart = f.Title
		if f.ProjectName != nil && *f.ProjectName != "" {
			rightPart = "@" + *f.ProjectName
		}
		if selected && !isDone {
			titlePart = styleSelected.Render(titlePart)
		}
	}

	content := prefix + checkbox + " " + titlePart
	if rightPart != "" {
		content = padBetween(content, rightPart, m.width)
	}

	// Done rows are dimmed as a whole — the only "color" semantic we keep.
	if isDone {
		content = styleDim.Render(content)
	}

	return lipgloss.NewStyle().MaxWidth(m.width).Render(content)
}

// padBetween tries to right-align the right part within width.
// Falls back to simple space separation if not enough room.
func padBetween(left, right string, width int) string {
	// Strip ANSI for length calculation
	visLeft := lipgloss.Width(left)
	visRight := lipgloss.Width(right)
	gap := width - visLeft - visRight
	if gap < 1 {
		gap = 1
	}
	return left + strings.Repeat(" ", gap) + right
}

func renderSeparator(width int) string {
	label := " done "
	labelWidth := lipgloss.Width(label)
	// When the terminal is too narrow to fit label + at least one dash on each
	// side, just render the label alone to avoid strings.Repeat with negative n.
	if width < labelWidth+2 {
		return styleGray.Render(label)
	}
	lineWidth := width - labelWidth
	half := lineWidth / 2
	left := strings.Repeat("─", half)
	right := strings.Repeat("─", lineWidth-half)
	sep := left + label + right
	return styleGray.Render(sep)
}

func (m Model) renderFooter() string {
	if m.footerMsg != "" {
		// Confirm / grace / sticky-add messages take priority over context help.
		if m.err != nil {
			return styleError.Render(m.footerMsg)
		}
		return m.footerMsg
	}
	if m.mode == modeEdit {
		return styleDim.Render(editFooterHint(m))
	}
	if m.helpExpanded {
		return longHelp
	}
	return styleDim.Render(shortHelp)
}

// editFooterHint returns the context help shown while editing.
func editFooterHint(m Model) string {
	switch m.field {
	case fieldTitle:
		return "Tab → project · Enter save · Esc cancel"
	case fieldProject:
		return "Tab → title · Enter save · Esc cancel"
	case fieldDate:
		return "↑/↓ ±1 day · Enter save · Esc cancel"
	}
	return "Enter save · Esc cancel"
}

// scrollWindow returns [start, end) indices such that cursor stays visible.
func scrollWindow(cursor, total, visible int) (int, int) {
	if total <= visible {
		return 0, total
	}
	start := cursor - visible/2
	if start < 0 {
		start = 0
	}
	end := start + visible
	if end > total {
		end = total
		start = end - visible
	}
	if start < 0 {
		start = 0
	}
	return start, end
}
