package tui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/al4danim/tick-tui/internal/store"
)

// footerHeight returns the number of lines renderFooter will occupy.
// This must stay in sync with renderFooter's branch logic (fixes Bug 5).
func (m Model) footerHeight() int {
	if m.mode == modeEdit {
		return 1
	}
	if m.footerMsg != "" {
		return 1
	}
	if m.helpExpanded {
		return strings.Count(m.strings.LongHelp(), "\n") + 1
	}
	return 1
}

// View renders the full TUI screen.
func (m Model) View() string {
	if m.width == 0 {
		return ""
	}

	switch m.mode {
	case modeStats30:
		data := m.statsData
		if data == nil {
			data = map[string]int{}
		}
		// Use m.statsEnd (captured at entry) rather than timeNow() so the
		// X-axis "today" label stays in sync with the loaded data window even
		// if the session crosses midnight. Defensive fallback to timeNow()
		// if statsEnd was never set (test fixtures, etc.).
		end := m.statsEnd
		if end.IsZero() {
			end = timeNow()
		}
		windowEnd := m.statsWindowEnd
		if windowEnd.IsZero() {
			windowEnd = end
		}
		return renderBars30(data, windowEnd, m.width, m.streak, m.selectedDate, m.selectedTasks, m.selectedScroll, m.footerMsg, m.strings)
	case modeStatsYear:
		data := m.statsData
		if data == nil {
			data = map[string]int{}
		}
		end := m.statsEnd
		if end.IsZero() {
			end = timeNow()
		}
		return renderHeatYear(data, end, m.width, m.strings)
	case modeSettings:
		return m.settingsModel.View()
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
	right := styleDim.Render(m.strings.DoneCount(m.today.DoneToday, m.today.TotalToday))
	bar := left + right
	if m.filterActive {
		label := "@" + m.activeProject
		if m.activeProject == "" {
			label = m.strings.NoProjectLabel
		}
		bar += styleBold.Render(" · " + label)
	}
	// UX 1: show sticky-add state in the title bar so the footer can show the
	// edit hint instead of the "keep adding" message.
	if m.addSticky && m.mode == modeEdit {
		bar += styleDim.Render(m.strings.AddingChip)
	}
	return lipgloss.NewStyle().MaxWidth(m.width).Render(bar)
}

func (m Model) renderList() string {
	// 2 = title bar + blank line separating title from list
	// footerHeight() is the single source of truth for how many lines the footer uses.
	availableRows := m.height - 2 - m.footerHeight()
	if availableRows < 1 {
		availableRows = 1
	}

	if m.loading {
		return styleDim.Render(m.strings.Loading)
	}

	if len(m.rows) == 0 {
		return styleGray.Render(m.strings.NoTasksHint)
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
			lines = append(lines, renderSeparator(m.width, m.strings.DoneSeparator))
		case rowDraft:
			// Draft is always selected + always in edit mode.
			lines = append(lines, m.renderFeatureLine(store.Feature{}, true, true, true, 0))
		default:
			selected := i == m.cursor
			inEdit := m.mode == modeEdit && selected
			lines = append(lines, m.renderFeatureLine(r.feature, selected, inEdit, false, r.daysAgo))
		}
	}
	return lines
}

func chip(label string) string {
	return styleChip.Render("[" + label + "]")
}

func (m Model) renderFeatureLine(f store.Feature, selected, inEdit, isDraft bool, daysAgo int) string {
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
				rightPart = chip(m.strings.ChipTitleArrow(proj))
			} else {
				rightPart = chip(m.strings.ChipTitle)
			}
		case fieldProject:
			titlePart = m.titleInput.Value()
			if titlePart == "" {
				titlePart = styleDim.Render(m.strings.ChipTitlePlaceholder)
			}
			rightPart = renderProjectField(m.projectInput.Value(), m.projects, true) + " " + chip(m.strings.ChipProject)
		case fieldDate:
			titlePart = f.Title
			rightPart = m.editDate.Format("2006-01-02") + " " + chip(m.strings.ChipDate)
		}
	} else {
		titlePart = f.Title
		if f.ProjectName != nil && *f.ProjectName != "" {
			rightPart = "@" + *f.ProjectName
		}
		// Append dim "-1d" marker for yesterday's done rows.
		// We build a plain-text annotation here; the whole row is dimmed below.
		if daysAgo > 0 {
			if rightPart != "" {
				rightPart = "-1d " + rightPart
			} else {
				rightPart = "-1d"
			}
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

func renderSeparator(width int, label string) string {
	if label == "" {
		label = " done "
	}
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
	var out string
	switch {
	// UX 1: edit hint always takes priority so users can always see field controls.
	case m.mode == modeEdit:
		out = styleDim.Render(editFooterHint(m))
	case m.footerMsg != "":
		// Confirm, grace, or transient (copy/error) messages.
		if m.footerErr {
			out = styleError.Render(m.footerMsg)
		} else {
			out = m.footerMsg
		}
	case m.helpExpanded:
		// longHelp is multi-line; skip MaxWidth so lines aren't squashed together.
		return m.strings.LongHelp()
	default:
		out = styleDim.Render(footerShortHelp(m))
	}
	// Bug 4: clamp to terminal width to prevent wrapping that shifts list rows.
	return lipgloss.NewStyle().MaxWidth(m.width).Render(out)
}

// editFooterHint returns the context help shown while editing.
// In sticky-add mode Esc both cancels the current edit and exits sticky, so
// the hint replaces the per-field "Esc cancel" with a single "Esc stops adding".
func editFooterHint(m Model) string {
	if m.addSticky {
		switch m.field {
		case fieldTitle:
			return m.strings.EditStickyTabProject
		case fieldProject:
			return m.strings.EditStickyTabTitle
		case fieldDate:
			return m.strings.EditStickyDateField
		default:
			return m.strings.EditStickyFallback
		}
	}
	switch m.field {
	case fieldTitle:
		return m.strings.EditTabToProject
	case fieldProject:
		return m.strings.EditTabToTitle
	case fieldDate:
		return m.strings.EditDateField
	default:
		return m.strings.EditFallback
	}
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
