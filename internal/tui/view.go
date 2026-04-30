package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/yaoyi/tick-tui/internal/api"
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
	// The date editor row is inserted below the selected item; reserve space for it.
	if m.mode == modeEdit && m.field == fieldDate {
		availableRows--
	}
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
	lines := make([]string, 0, len(m.rows)+1)
	insertedDateEditor := false

	for i, r := range m.rows {
		if r.kind == rowSeparator {
			sep := renderSeparator(m.width)
			lines = append(lines, sep)
			continue
		}

		selected := i == m.cursor
		inEdit := m.mode == modeEdit && selected

		line := m.renderFeatureLine(r.feature, selected, inEdit)
		lines = append(lines, line)

		// Insert date editor row right after the selected line in edit+date mode
		if inEdit && m.field == fieldDate && !insertedDateEditor {
			lines = append(lines, renderDateEditor(m.editDate))
			insertedDateEditor = true
		}
	}
	return lines
}

func (m Model) renderFeatureLine(f api.Feature, selected bool, inEdit bool) string {
	isDone := f.IsDone == 1

	// Checkbox
	var checkbox string
	if isDone {
		checkbox = styleGreen.Render("[x]")
	} else {
		checkbox = "[ ]"
	}

	// Prefix
	prefix := "  "
	if selected {
		prefix = styleSelected.Render("› ")
	}

	// Project suffix
	var projectPart string
	if inEdit && m.field == fieldProject {
		projectPart = renderProjectField(m.projectInput.Value(), m.projects, true)
	} else if inEdit && m.field == fieldTitle {
		// Show project as normal, editable via project field
		if f.ProjectName != nil && *f.ProjectName != "" {
			projectPart = styleCyan.Render("@" + *f.ProjectName)
		}
	} else {
		if f.ProjectName != nil && *f.ProjectName != "" {
			pStyle := styleCyan
			if isDone {
				pStyle = styleDim
			}
			projectPart = pStyle.Render("@" + *f.ProjectName)
		}
	}

	// Title part
	var titlePart string
	if inEdit && m.field == fieldTitle {
		titlePart = renderTitleWithGhost(m.titleInput.Value(), m.titleInput.Position(), "", true)
	} else if inEdit && m.field == fieldProject {
		titlePart = m.titleInput.Value()
	} else if inEdit && m.field == fieldDate {
		// Done edit: keep original title look; pending edit on date never reaches here.
		if isDone {
			titlePart = styleDim.Render(f.Title)
		} else {
			titlePart = m.titleInput.Value()
		}
	} else if isDone {
		titlePart = styleDim.Render(f.Title)
	} else {
		titlePart = f.Title
	}

	if selected && !isDone {
		titlePart = styleSelected.Render(titlePart)
	}

	// Build full line with padding
	content := prefix + checkbox + " " + titlePart
	if projectPart != "" {
		content = padBetween(content, projectPart, m.width)
	}

	if isDone {
		// Always dim done feature rows, including in edit mode (date-only edit).
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
		// Confirm / grace messages take priority
		if m.err != nil {
			return styleError.Render(m.footerMsg)
		}
		return m.footerMsg
	}
	if m.helpExpanded {
		return longHelp
	}
	return styleDim.Render(shortHelp)
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
