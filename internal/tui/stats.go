package tui

import (
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/al4danim/tick-tui/internal/i18n"
	"github.com/al4danim/tick-tui/internal/store"
)

// barChars maps a height level (0-5) to unicode block characters.
// Level 0 = nothing (space within the bar area handled by caller),
// 1-8 map to eighths-of-block characters.
var barChars = []rune{' ', '▁', '▂', '▃', '▄', '▅', '▆', '▇', '█'}

// heatColors maps 5 intensity levels (0-4) to ANSI 256 foreground colors.
// Level 0: dim grey (no activity); 1-4: monotonically brighter greens.
// Picked from the xterm 256-colour cube so the gradient is visible on most
// terminals: 22 (deep dark green) → 28 → 34 → 70 (mid yellowish-green) → 40
// (bright green). Earlier version had level 1 == level 3 (both 28), which
// flattened the visual gradient.
var heatColors = []lipgloss.Color{"240", "22", "28", "34", "40"}

const heatCell = "■"

// minHeatWidth is the minimum terminal width for the year heatmap.
const minHeatWidth = 60

// barRows is how many display rows the bar chart occupies.
const barRows = 5

// wideStatsWidth is the minimum terminal width for the side-by-side bars+panel layout.
const wideStatsWidth = 70

// barsAreaWidth is the fixed width reserved for the bar chart area in wide layout.
const barsAreaWidth = 36

// renderBars30 produces the 30-day bar chart as a multi-line string.
// windowEnd is the rightmost column of the visible 30-day window; windowEnd-29 is the leftmost.
// width is the available terminal width. streak is the current consecutive-day streak.
// selectedDate (zero = idle), selectedTasks and selectedScroll control the drill-down panel.
// footerOverride: if non-empty, replaces the contextual footer for one frame
// (used for transient hints like "no older data").
// s carries localized labels.
func renderBars30(
	data map[string]int,
	windowEnd time.Time,
	width int,
	streak int,
	selectedDate time.Time,
	selectedTasks []store.Feature,
	selectedScroll int,
	footerOverride string,
	s i18n.TUIStrings,
) string {
	// Build the 30-day window (oldest first = left).
	days := make([]time.Time, 30)
	for i := 0; i < 30; i++ {
		days[i] = windowEnd.AddDate(0, 0, -(29 - i))
	}

	counts := make([]int, 30)
	total := 0
	maxCount := 0
	maxDate := ""
	for i, d := range days {
		key := d.Format("2006-01-02")
		c := data[key]
		counts[i] = c
		total += c
		if c > maxCount {
			maxCount = c
			maxDate = s.MonthDay(d)
		}
	}

	// Compute bar heights.
	barHeights := make([]int, 30)
	for i, c := range counts {
		if maxCount == 0 {
			barHeights[i] = 0
		} else {
			totalTiers := barRows * 8
			tier := (c * totalTiers + maxCount - 1) / maxCount // ceiling division
			if tier > totalTiers {
				tier = totalTiers
			}
			barHeights[i] = tier
		}
	}

	// Find which column index (0-29) corresponds to selectedDate, if any.
	selectedCol := -1
	if !selectedDate.IsZero() {
		selStr := selectedDate.Format("2006-01-02")
		for i, d := range days {
			if d.Format("2006-01-02") == selStr {
				selectedCol = i
				break
			}
		}
	}

	wide := width >= wideStatsWidth

	// Build task panel lines (used in wide layout inline, narrow layout appended after).
	var panelLines []string
	if !selectedDate.IsZero() {
		panelWidth := width - barsAreaWidth
		if !wide {
			panelWidth = width
		}
		// Guard: terminal-too-narrow / test fixtures can yield ≤ 0; force ≥ 1
		// so renderTaskPanel doesn't pass negative widths to truncateToWidth.
		if panelWidth < 1 {
			panelWidth = 1
		}
		panelLines = renderTaskPanel(selectedDate, selectedTasks, selectedScroll, panelWidth, s)
	}

	var sb strings.Builder

	// --- Title row ---
	titleLeft := styleBold.Render(s.Bars30Title(total))
	streakChip := styleDim.Render(s.StreakLabel(streak))
	titleRow := padBetween(titleLeft, streakChip, width)
	sb.WriteString(lipgloss.NewStyle().MaxWidth(width).Render(titleRow))
	sb.WriteByte('\n')

	// --- Divider ---
	divWidth := width
	if divWidth < 2 {
		divWidth = 2
	}
	sb.WriteString(styleDim.Render(strings.Repeat("─", divWidth)))
	sb.WriteByte('\n')

	// --- Marker row (only when a column is selected and within window) ---
	// This is a single line containing only ▼ at the selected column. It
	// appears above the bars so the user sees what's selected without
	// obscuring the bar's actual height. When there's no selection we skip
	// the row entirely — first-screen height stays the same as before.
	markerStr := buildMarkerRow(selectedCol)
	hasMarker := markerStr != ""

	// panelRowOffset shifts which panel line lines up with each bars-area
	// row so the panel header (row 0) is vertically next to the marker (when
	// shown) instead of next to the topmost bar row.
	panelRowOffset := 0
	if hasMarker {
		paddedMarker := padOrTruncToWidth(markerStr, barsAreaWidth)
		if wide && len(panelLines) > 0 {
			sb.WriteString(paddedMarker)
			sb.WriteString(panelLines[0])
		} else {
			sb.WriteString(paddedMarker)
		}
		sb.WriteByte('\n')
		panelRowOffset = 1
	}

	// --- Bar rows (5 rows) ---
	// In wide layout, each bar row is barsAreaWidth chars; the remaining space
	// is filled by the panel row (or blank if panel has fewer rows than bars).
	//
	// Row index into panelLines: top bar row → panel row panelRowOffset.
	// The bars area itself is 1 (prefix space) + 30 (bars) = 31 chars; we pad
	// to barsAreaWidth with spaces to give a gap before the panel.
	for rowIdx := barRows - 1; rowIdx >= 0; rowIdx-- {
		barRowStr := buildBarRow(rowIdx, barHeights)
		// Pad to barsAreaWidth.
		barRowStr = padOrTruncToWidth(barRowStr, barsAreaWidth)

		panelRowIdx := (barRows - 1) - rowIdx + panelRowOffset
		if wide && len(panelLines) > panelRowIdx {
			sb.WriteString(barRowStr)
			sb.WriteString(panelLines[panelRowIdx])
		} else {
			sb.WriteString(barRowStr)
		}
		sb.WriteByte('\n')
	}

	// --- X-axis ---
	axisStr := buildAxisRow(days, selectedCol)
	axisStr = padOrTruncToWidth(axisStr, barsAreaWidth)
	axisPanelIdx := barRows + panelRowOffset
	if wide && len(panelLines) > axisPanelIdx {
		sb.WriteString(axisStr)
		sb.WriteString(panelLines[axisPanelIdx])
	} else {
		sb.WriteString(axisStr)
	}
	sb.WriteByte('\n')

	// --- Date labels row ---
	leftLabel := s.MonthDay(days[0])
	rightLabel := s.MonthDay(windowEnd)
	axisWidth := 31
	labelGap := axisWidth - lipgloss.Width(leftLabel) - lipgloss.Width(rightLabel)
	if labelGap < 1 {
		labelGap = 1
	}
	dateRowRaw := " " + leftLabel + strings.Repeat(" ", labelGap) + rightLabel
	dateRowPadded := padOrTruncToWidth(dateRowRaw, barsAreaWidth)
	dateRowPanelIdx := barRows + 1 + panelRowOffset
	if wide && len(panelLines) > dateRowPanelIdx {
		sb.WriteString(styleDim.Render(dateRowPadded))
		sb.WriteString(panelLines[dateRowPanelIdx])
	} else {
		sb.WriteString(styleDim.Render(dateRowPadded))
	}
	sb.WriteByte('\n')

	// --- Remaining panel lines (wide layout) ---
	remainingStart := barRows + 2 + panelRowOffset
	if wide && len(panelLines) > remainingStart {
		// Output remaining panel lines with empty bars-area prefix.
		emptyBarsPrefix := strings.Repeat(" ", barsAreaWidth)
		for i := remainingStart; i < len(panelLines); i++ {
			sb.WriteString(emptyBarsPrefix)
			sb.WriteString(panelLines[i])
			sb.WriteByte('\n')
		}
	}

	// --- Stats row ---
	var statsLine string
	if total > 0 {
		avg := float64(total) / 30.0
		statsLine = s.Bars30Stats(avg, maxCount, maxDate)
	} else {
		statsLine = s.StatsBars30NoData
	}
	sb.WriteByte('\n')
	sb.WriteString(styleDim.Render(statsLine))
	sb.WriteByte('\n')

	// --- Narrow panel (below bars) ---
	if !wide && len(panelLines) > 0 {
		sb.WriteByte('\n')
		for _, pl := range panelLines {
			sb.WriteString(pl)
			sb.WriteByte('\n')
		}
	}

	// --- Footer ---
	sb.WriteByte('\n')
	switch {
	case footerOverride != "":
		sb.WriteString(styleDim.Render(" " + footerOverride))
	case !selectedDate.IsZero():
		sb.WriteString(styleDim.Render(s.StatsBars30FooterDrill))
	default:
		sb.WriteString(styleDim.Render(s.StatsBars30Footer))
	}

	return sb.String()
}

// buildBarRow renders one horizontal bar row as a string.
// rowIdx 0 = bottom tier (renders last), barRows-1 = top tier (renders first).
// Selection highlight is NOT drawn here — a previous version put a ▕ on
// every bar row of the selected column, which visually obscured the bar's
// real height (a height-1 bar would look full-tall). The selection is now
// indicated by a single ▼ above the bars (markerRow) and ^ on the axis.
func buildBarRow(rowIdx int, barHeights []int) string {
	rowBase := rowIdx * 8
	var b strings.Builder
	b.WriteByte(' ')
	for col := 0; col < 30; col++ {
		h := barHeights[col]
		var ch rune
		switch {
		case h <= rowBase:
			ch = ' '
		case h >= rowBase+8:
			ch = barChars[8]
		default:
			ch = barChars[h-rowBase]
		}
		b.WriteRune(ch)
	}
	return b.String()
}

// buildMarkerRow renders the row above the bars: a single ▼ at the selected
// column, spaces elsewhere. Returns "" when there is no selection (caller
// should skip the row entirely so first-screen height stays unchanged).
func buildMarkerRow(selectedCol int) string {
	if selectedCol < 0 || selectedCol >= 30 {
		return ""
	}
	row := make([]rune, 31)
	row[0] = ' '
	for i := 0; i < 30; i++ {
		row[i+1] = ' '
	}
	row[selectedCol+1] = '▼'
	return string(row)
}

// buildAxisRow renders the x-axis line with ^ marker under selected column.
func buildAxisRow(days []time.Time, selectedCol int) string {
	axis := make([]rune, 31)
	axis[0] = ' '
	for i := 0; i < 30; i++ {
		axis[i+1] = '─'
	}
	// Place ^ under Monday columns (weekly markers) and selected column.
	for i, d := range days {
		if d.Weekday() == time.Monday {
			axis[i+1] = '┬'
		}
	}
	if selectedCol >= 0 && selectedCol < 30 {
		axis[selectedCol+1] = '^'
	}
	return styleDim.Render(string(axis))
}

// renderTaskPanel builds the lines for the drill-down task panel.
// panelWidth is the usable column width for the panel.
func renderTaskPanel(date time.Time, tasks []store.Feature, scroll int, panelWidth int, s i18n.TUIStrings) []string {
	lines := []string{}

	// Row 0: selected date header.
	lines = append(lines, styleBold.Render(s.SelectedHeader(date)))
	// Row 1: done count.
	lines = append(lines, styleDim.Render(s.SelectedDoneCount(len(tasks))))
	// Row 2: blank separator.
	lines = append(lines, "")

	// Clamp scroll.
	maxScroll := len(tasks) - 10
	if maxScroll < 0 {
		maxScroll = 0
	}
	if scroll > maxScroll {
		scroll = maxScroll
	}
	if scroll < 0 {
		scroll = 0
	}

	// Determine the visible window.
	// Show at most 10 tasks, with possible ↑ above and ↓ below indicators.
	visStart := scroll
	visEnd := scroll + 10
	if visEnd > len(tasks) {
		visEnd = len(tasks)
	}

	// ↑ indicator if scrolled down.
	if scroll > 0 {
		lines = append(lines, styleDim.Render(s.MoreTasksAbove(scroll)))
	}

	// Task lines.
	prefixWidth := lipgloss.Width("· ")
	for _, f := range tasks[visStart:visEnd] {
		proj := ""
		if f.ProjectName != nil && *f.ProjectName != "" {
			proj = "@" + *f.ProjectName
		}
		var line string
		if proj != "" {
			// Long project name in a tight panel can drive titleW negative;
			// floor at 1 so truncateToWidth still yields at least "…".
			titleW := panelWidth - prefixWidth - lipgloss.Width(proj) - 1
			if titleW < 1 {
				titleW = 1
			}
			line = "· " + proj + " " + truncateToWidth(f.Title, titleW)
		} else {
			titleW := panelWidth - prefixWidth
			if titleW < 1 {
				titleW = 1
			}
			line = "· " + truncateToWidth(f.Title, titleW)
		}
		lines = append(lines, line)
	}

	// ↓ indicator if more below.
	remaining := len(tasks) - visEnd
	if remaining > 0 {
		lines = append(lines, styleDim.Render(s.MoreTasksBelow(remaining)))
	}

	return lines
}

// truncateToWidth truncates s so its visible width (lipgloss.Width) does not exceed maxW.
// If truncated, appends "…". CJK-safe.
func truncateToWidth(s string, maxW int) string {
	if maxW <= 0 {
		return ""
	}
	w := lipgloss.Width(s)
	if w <= maxW {
		return s
	}
	// Truncate rune by rune, leaving room for "…" (width 1).
	var b strings.Builder
	used := 0
	for _, r := range s {
		rw := lipgloss.Width(string(r))
		if used+rw+1 > maxW { // +1 for "…"
			break
		}
		b.WriteRune(r)
		used += rw
	}
	return b.String() + "…"
}

// renderHeatYear produces the year heatmap as a multi-line string.
// end is the most recent day (today). width is the terminal width.
// Returns a single-line narrow-window message when width < 60.
func renderHeatYear(data map[string]int, end time.Time, width int, s i18n.TUIStrings) string {
	if width < minHeatWidth {
		return s.StatsTooNarrow
	}

	// Build 365-day window ending on end (inclusive).
	start := end.AddDate(0, 0, -364)

	// Compute max for threshold calculation.
	maxCount := 0
	total := 0
	maxDate := ""
	for i := 0; i < 365; i++ {
		d := start.AddDate(0, 0, i)
		c := data[d.Format("2006-01-02")]
		total += c
		if c > maxCount {
			maxCount = c
			maxDate = s.MonthDay(d)
		}
	}

	// heatLevel returns 0-4 for a given count.
	heatLevel := func(count int) int {
		if count == 0 || maxCount == 0 {
			return 0
		}
		q := maxCount / 4 // quarter
		if q == 0 {
			q = 1
		}
		switch {
		case count <= q:
			return 1
		case count <= 2*q:
			return 2
		case count <= 3*q:
			return 3
		default:
			return 4
		}
	}

	// The grid is 53 columns (weeks) × 7 rows (Mon-Sun).
	goWeekday := func(t time.Time) int {
		// 0=Mon, 1=Tue, ..., 6=Sun
		wd := int(t.Weekday()) // 0=Sun, 1=Mon..6=Sat
		return (wd + 6) % 7    // shift so Mon=0
	}

	type cell struct {
		count  int
		active bool // within the 365-day window
	}
	cells := [53][7]cell{}

	// The grid starts on the Monday of the week containing `start`.
	gridStart := start
	wd := goWeekday(start)
	gridStart = gridStart.AddDate(0, 0, -wd) // roll back to Monday

	startS := start.Format("2006-01-02")
	endS := end.Format("2006-01-02")

	for col := 0; col < 53; col++ {
		for row := 0; row < 7; row++ {
			d := gridStart.AddDate(0, 0, col*7+row)
			ds := d.Format("2006-01-02")
			if ds >= startS && ds <= endS {
				cells[col][row] = cell{count: data[ds], active: true}
			}
		}
	}

	var sb strings.Builder

	// Title
	sb.WriteString(styleBold.Render(s.HeatYearTitle(total, maxCount, maxDate)))
	sb.WriteByte('\n')
	sb.WriteString(styleDim.Render(strings.Repeat("─", width)))
	sb.WriteByte('\n')
	sb.WriteByte('\n')

	// Month header. For each week column, decide whether to start a new month
	// label there (label tied to first-week-of-month).
	monthRow := make([]string, 53)
	for col := 0; col < 53; col++ {
		for row := 0; row < 7; row++ {
			d := gridStart.AddDate(0, 0, col*7+row)
			ds := d.Format("2006-01-02")
			if ds >= startS && ds <= endS {
				if d.Day() <= 7 {
					monthRow[col] = s.MonthShort(d.Month())
				}
				break
			}
		}
	}

	// Render month header. The grid below uses 1 cell == 1 char, so a label
	// occupying N visible columns must skip N week-columns ahead.
	//
	// We allocate exactly monthLabelWidth = 3 week-columns per label (covers
	// ~21 days, fits both "Jan" and "5月 " which display as 3 cols after pad).
	// Labels wider than 3 are truncated at 3 visible cells; narrower are
	// right-padded to 3 with spaces. This guarantees alignment regardless
	// of locale.
	const monthLabelWidth = 3
	sb.WriteString("         ") // left padding == row-label width below
	col := 0
	for col < 53 {
		if monthRow[col] != "" {
			label := padOrTruncToWidth(monthRow[col], monthLabelWidth)
			sb.WriteString(label)
			col += monthLabelWidth
		} else {
			sb.WriteString(" ")
			col++
		}
	}
	sb.WriteByte('\n')

	// Day rows: Mon-Sun (localized).
	dayLabels := []string{
		s.WeekdayShort(time.Monday),
		s.WeekdayShort(time.Tuesday),
		s.WeekdayShort(time.Wednesday),
		s.WeekdayShort(time.Thursday),
		s.WeekdayShort(time.Friday),
		s.WeekdayShort(time.Saturday),
		s.WeekdayShort(time.Sunday),
	}
	todayStr := end.Format("2006-01-02")
	todayLevel := heatLevel(data[todayStr])

	// Pad weekday labels so the heatmap grid aligns regardless of label width.
	maxLabelWidth := 0
	for _, lbl := range dayLabels {
		if w := lipgloss.Width(lbl); w > maxLabelWidth {
			maxLabelWidth = w
		}
	}

	for row := 0; row < 7; row++ {
		lbl := dayLabels[row]
		pad := maxLabelWidth - lipgloss.Width(lbl)
		if pad < 0 {
			pad = 0
		}
		sb.WriteString(styleDim.Render("   " + lbl + strings.Repeat(" ", pad)))
		sb.WriteString("  ")
		for col := 0; col < 53; col++ {
			c := cells[col][row]
			if !c.active {
				sb.WriteString(" ")
				continue
			}
			lvl := heatLevel(c.count)
			style := lipgloss.NewStyle().Foreground(heatColors[lvl])
			sb.WriteString(style.Render(heatCell))
		}
		sb.WriteByte('\n')
	}

	// Legend row.
	sb.WriteByte('\n')
	legendParts := []string{s.StatsHeatLegendLess}
	for lvl := 0; lvl <= 4; lvl++ {
		style := lipgloss.NewStyle().Foreground(heatColors[lvl])
		legendParts = append(legendParts, style.Render(heatCell))
	}
	legendParts = append(legendParts, s.StatsHeatLegendMore)
	legend := " " + strings.Join(legendParts, " ")

	// Append today's cell info: " · today=Sat May 2 (■)"
	todayLevelStyle := lipgloss.NewStyle().Foreground(heatColors[todayLevel])
	legend += styleDim.Render(s.HeatTodayLabel(end)) + todayLevelStyle.Render(heatCell) + styleDim.Render(")")

	sb.WriteString(legend)
	sb.WriteByte('\n')

	// Footer.
	sb.WriteByte('\n')
	sb.WriteString(styleDim.Render(s.StatsHeatYearFooter))

	return sb.String()
}

// padOrTruncToWidth returns label adjusted to exactly width visible columns
// (lipgloss.Width). Wider labels are truncated rune-by-rune; narrower labels
// are right-padded with spaces. CJK runes consume 2 visible columns each.
func padOrTruncToWidth(label string, width int) string {
	w := lipgloss.Width(label)
	if w == width {
		return label
	}
	if w < width {
		return label + strings.Repeat(" ", width-w)
	}
	// Truncate by accumulating runes until we'd exceed width.
	var b strings.Builder
	used := 0
	for _, r := range label {
		rw := lipgloss.Width(string(r))
		if used+rw > width {
			break
		}
		b.WriteRune(r)
		used += rw
	}
	// Right-pad if we couldn't fill exactly (e.g. last rune was 2 cols when
	// only 1 col remained → skipped, leaving used < width).
	if used < width {
		b.WriteString(strings.Repeat(" ", width-used))
	}
	return b.String()
}

// computeStreak counts consecutive done-days from today backwards, stopping at
// the first day with zero completions. Capped at 30 (callers display "30+").
func computeStreak(data map[string]int, today time.Time) int {
	streak := 0
	for i := 0; i < 30; i++ {
		key := today.AddDate(0, 0, -i).Format("2006-01-02")
		if data[key] == 0 {
			return streak
		}
		streak++
	}
	return streak
}
