package tui

import (
	"fmt"
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
		return strings.Count(longHelp, "\n") + 1
	}
	return 1
}

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
	// UX 1: show sticky-add state in the title bar so the footer can show the
	// edit hint instead of the "keep adding" message.
	if m.addSticky && m.mode == modeEdit {
		bar += styleDim.Render(" · adding")
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
		return longHelp
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
			return "Tab → project · Enter save · Esc stops adding"
		case fieldProject:
			return "Tab → title · Enter save · Esc stops adding"
		case fieldDate:
			return "↑/↓ ±1 day · Enter save · Esc stops adding"
		default:
			return "Enter save · Esc stops adding"
		}
	}
	switch m.field {
	case fieldTitle:
		return "Tab → project · Enter save · Esc cancel"
	case fieldProject:
		return "Tab → title · Enter save · Esc cancel"
	case fieldDate:
		return "↑/↓ ±1 day · Enter save · Esc cancel"
	default:
		return "Enter save · Esc cancel"
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
