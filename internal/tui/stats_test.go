package tui

import (
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/al4danim/tick-tui/internal/i18n"
	"github.com/al4danim/tick-tui/internal/store"
)

// stripANSI strips ANSI escape sequences for plain-text assertions.
func stripANSI(s string) string {
	var out strings.Builder
	inEsc := false
	for _, r := range s {
		if r == '\x1b' {
			inEsc = true
			continue
		}
		if inEsc {
			if r == 'm' {
				inEsc = false
			}
			continue
		}
		out.WriteRune(r)
	}
	return out.String()
}

func enStrings() i18n.TUIStrings { return i18n.For(i18n.LangEN) }
func zhStrings() i18n.TUIStrings { return i18n.For(i18n.LangZH) }

func TestRenderBars30_KeyMarkers(t *testing.T) {
	end := time.Date(2026, 5, 2, 0, 0, 0, 0, time.UTC) // today

	// 10 done on Apr 3 (leftmost day = end - 29)
	leftDay := end.AddDate(0, 0, -29).Format("2006-01-02") // 2026-04-03
	data := map[string]int{
		leftDay:                  10,
		end.Format("2006-01-02"): 5,
	}

	out := renderBars30(data, end, 40, 0, time.Time{}, nil, 0, "", enStrings())
	plain := stripANSI(out)

	// Title should mention total (15 done)
	if !strings.Contains(plain, "15 done") {
		t.Errorf("expected '15 done' in output; got:\n%s", plain)
	}

	// Start date label (leftmost day)
	if !strings.Contains(plain, "Apr 3") {
		t.Errorf("expected start date 'Apr 3' in output; got:\n%s", plain)
	}

	// End date label (today)
	if !strings.Contains(plain, "May 2") {
		t.Errorf("expected end date 'May 2' in output; got:\n%s", plain)
	}

	// avg and max markers
	if !strings.Contains(plain, "avg") {
		t.Errorf("expected 'avg' in output; got:\n%s", plain)
	}
	if !strings.Contains(plain, "max") {
		t.Errorf("expected 'max' in output; got:\n%s", plain)
	}

	// footer keys
	if !strings.Contains(plain, "esc back") {
		t.Errorf("expected 'esc back' in footer; got:\n%s", plain)
	}
	if !strings.Contains(plain, "S year") {
		t.Errorf("expected 'S year' in footer; got:\n%s", plain)
	}
}

func TestRenderBars30_KeyMarkers_ZH(t *testing.T) {
	end := time.Date(2026, 5, 2, 0, 0, 0, 0, time.UTC)
	leftDay := end.AddDate(0, 0, -29).Format("2006-01-02")
	data := map[string]int{
		leftDay:                  10,
		end.Format("2006-01-02"): 5,
	}

	out := renderBars30(data, end, 40, 0, time.Time{}, nil, 0, "", zhStrings())
	plain := stripANSI(out)

	// ZH: title = "Tick · 最近 30 天 · 完成 15 件"
	if !strings.Contains(plain, "最近 30 天") {
		t.Errorf("ZH: title prefix missing; got:\n%s", plain)
	}
	if !strings.Contains(plain, "完成 15") {
		t.Errorf("ZH: total missing; got:\n%s", plain)
	}
	// ZH date labels should be CJK
	if !strings.Contains(plain, "5月2日") {
		t.Errorf("ZH: end date label missing; got:\n%s", plain)
	}
	if !strings.Contains(plain, "4月3日") {
		t.Errorf("ZH: start date label missing; got:\n%s", plain)
	}
	if !strings.Contains(plain, "日均") {
		t.Errorf("ZH: stats avg label missing; got:\n%s", plain)
	}
	if !strings.Contains(plain, "峰值") {
		t.Errorf("ZH: stats max label missing; got:\n%s", plain)
	}
	if !strings.Contains(plain, "esc 返回") {
		t.Errorf("ZH: footer missing; got:\n%s", plain)
	}
	if !strings.Contains(plain, "S 年度") {
		t.Errorf("ZH: footer S key missing; got:\n%s", plain)
	}
}

func TestRenderBars30_EmptyData(t *testing.T) {
	end := time.Date(2026, 5, 2, 0, 0, 0, 0, time.UTC)
	out := renderBars30(map[string]int{}, end, 40, 0, time.Time{}, nil, 0, "", enStrings())
	plain := stripANSI(out)

	if !strings.Contains(plain, "0 done") {
		t.Errorf("empty data should show '0 done'; got:\n%s", plain)
	}
	if !strings.Contains(plain, "no completions") {
		t.Errorf("empty data should mention 'no completions'; got:\n%s", plain)
	}
}

func TestRenderBars30_EmptyData_ZH(t *testing.T) {
	end := time.Date(2026, 5, 2, 0, 0, 0, 0, time.UTC)
	out := renderBars30(map[string]int{}, end, 40, 0, time.Time{}, nil, 0, "", zhStrings())
	plain := stripANSI(out)
	if !strings.Contains(plain, "本时间段无完成") {
		t.Errorf("ZH empty: missing localized hint; got:\n%s", plain)
	}
}

func TestRenderBars30_BarCharsPresent(t *testing.T) {
	end := time.Date(2026, 5, 2, 0, 0, 0, 0, time.UTC)
	data := map[string]int{
		end.Format("2006-01-02"): 8,
	}
	out := renderBars30(data, end, 40, 0, time.Time{}, nil, 0, "", enStrings())
	hasBlock := false
	for _, r := range out {
		if r >= '▁' && r <= '█' {
			hasBlock = true
			break
		}
	}
	if !hasBlock {
		t.Error("expected at least one block character in bar chart")
	}
}

func TestRenderHeatYear_TodayMarker(t *testing.T) {
	end := time.Date(2026, 5, 2, 0, 0, 0, 0, time.UTC)
	data := map[string]int{
		end.Format("2006-01-02"): 3,
	}

	out := renderHeatYear(data, end, 80, enStrings())
	plain := stripANSI(out)

	if !strings.Contains(plain, "today=") {
		t.Errorf("expected 'today=' legend; got:\n%s", plain)
	}
	if !strings.Contains(plain, "Sat May 2") {
		t.Errorf("expected today's date in legend; got:\n%s", plain)
	}
}

func TestRenderHeatYear_TodayMarker_ZH(t *testing.T) {
	end := time.Date(2026, 5, 2, 0, 0, 0, 0, time.UTC) // Saturday
	data := map[string]int{
		end.Format("2006-01-02"): 3,
	}

	out := renderHeatYear(data, end, 80, zhStrings())
	plain := stripANSI(out)

	if !strings.Contains(plain, "今天=") {
		t.Errorf("ZH: 今天 marker missing; got:\n%s", plain)
	}
	if !strings.Contains(plain, "周六") {
		t.Errorf("ZH: weekday missing; got:\n%s", plain)
	}
	if !strings.Contains(plain, "5月2日") {
		t.Errorf("ZH: monthday missing; got:\n%s", plain)
	}
}

func TestRenderHeatYear_NarrowWindow(t *testing.T) {
	end := time.Date(2026, 5, 2, 0, 0, 0, 0, time.UTC)
	want := enStrings().StatsTooNarrow
	out := renderHeatYear(map[string]int{}, end, 59, enStrings())
	if out != want {
		t.Errorf("narrow window EN: got %q want %q", out, want)
	}

	wantZH := zhStrings().StatsTooNarrow
	outZH := renderHeatYear(map[string]int{}, end, 59, zhStrings())
	if outZH != wantZH {
		t.Errorf("narrow window ZH: got %q want %q", outZH, wantZH)
	}
}

func TestRenderHeatYear_ExactMinWidth(t *testing.T) {
	end := time.Date(2026, 5, 2, 0, 0, 0, 0, time.UTC)
	out := renderHeatYear(map[string]int{}, end, 60, enStrings())
	if out == enStrings().StatsTooNarrow {
		t.Error("exactly 60 cols should show the heatmap, not the narrow message")
	}
}

func TestRenderHeatYear_TotalInTitle(t *testing.T) {
	end := time.Date(2026, 5, 2, 0, 0, 0, 0, time.UTC)
	d := end.AddDate(0, 0, -100).Format("2006-01-02")
	data := map[string]int{d: 7}

	out := renderHeatYear(data, end, 80, enStrings())
	plain := stripANSI(out)

	if !strings.Contains(plain, "7 done") {
		t.Errorf("expected total '7 done' in title; got:\n%s", plain)
	}
}

func TestRenderHeatYear_MaxInTitle(t *testing.T) {
	end := time.Date(2026, 5, 2, 0, 0, 0, 0, time.UTC)
	d := end.AddDate(0, 0, -50).Format("2006-01-02")
	data := map[string]int{d: 12}

	out := renderHeatYear(data, end, 80, enStrings())
	plain := stripANSI(out)

	if !strings.Contains(plain, "max 12") {
		t.Errorf("expected 'max 12' in title; got:\n%s", plain)
	}
}

func TestRenderHeatYear_HeatCellPresent(t *testing.T) {
	end := time.Date(2026, 5, 2, 0, 0, 0, 0, time.UTC)
	data := map[string]int{end.Format("2006-01-02"): 5}

	out := renderHeatYear(data, end, 80, enStrings())
	if !strings.Contains(out, heatCell) {
		t.Errorf("expected '%s' in heatmap output", heatCell)
	}
}

func TestRenderHeatYear_LegendKeys(t *testing.T) {
	end := time.Date(2026, 5, 2, 0, 0, 0, 0, time.UTC)
	out := renderHeatYear(map[string]int{}, end, 80, enStrings())
	plain := stripANSI(out)

	if !strings.Contains(plain, "less") {
		t.Errorf("expected 'less' in legend; got:\n%s", plain)
	}
	if !strings.Contains(plain, "more") {
		t.Errorf("expected 'more' in legend; got:\n%s", plain)
	}
	if !strings.Contains(plain, "esc back") {
		t.Errorf("expected 'esc back' in footer; got:\n%s", plain)
	}
	if !strings.Contains(plain, "s 30-day") {
		t.Errorf("expected 's 30-day' in footer; got:\n%s", plain)
	}
}

func TestRenderHeatYear_LegendKeys_ZH(t *testing.T) {
	end := time.Date(2026, 5, 2, 0, 0, 0, 0, time.UTC)
	out := renderHeatYear(map[string]int{}, end, 80, zhStrings())
	plain := stripANSI(out)

	if !strings.Contains(plain, "少") {
		t.Errorf("ZH: '少' legend missing; got:\n%s", plain)
	}
	if !strings.Contains(plain, "多") {
		t.Errorf("ZH: '多' legend missing; got:\n%s", plain)
	}
	if !strings.Contains(plain, "esc 返回") {
		t.Errorf("ZH: footer missing; got:\n%s", plain)
	}
	if !strings.Contains(plain, "s 30 天") {
		t.Errorf("ZH: footer s key missing; got:\n%s", plain)
	}
}

// TestRenderHeatYear_DayRowsAlign verifies fix #7: every day-row in the
// heatmap grid renders with the same number of runes (proxy for grid
// alignment). CJK weekday/month labels must not desync row widths.
func TestRenderHeatYear_DayRowsAlign(t *testing.T) {
	for _, lang := range []string{"EN", "ZH"} {
		s := enStrings()
		var prefix string
		if lang == "ZH" {
			s = zhStrings()
			prefix = "   周" // ZH weekday labels start with 周
		} else {
			prefix = "   "
		}
		end := time.Date(2026, 5, 2, 0, 0, 0, 0, time.UTC)
		out := renderHeatYear(map[string]int{}, end, 80, s)
		plain := stripANSI(out)
		lines := strings.Split(plain, "\n")

		var dayLines []string
		for _, ln := range lines {
			if strings.HasPrefix(ln, prefix) && lang == "ZH" {
				dayLines = append(dayLines, ln)
			}
			// EN: row labels are "Mon/Tue/Wed/Thu/Fri/Sat/Sun" — same 3-letter prefix.
			if lang == "EN" && len(ln) > 6 && (strings.HasPrefix(ln, "   Mon") ||
				strings.HasPrefix(ln, "   Tue") || strings.HasPrefix(ln, "   Wed") ||
				strings.HasPrefix(ln, "   Thu") || strings.HasPrefix(ln, "   Fri") ||
				strings.HasPrefix(ln, "   Sat") || strings.HasPrefix(ln, "   Sun")) {
				dayLines = append(dayLines, ln)
			}
		}

		if len(dayLines) != 7 {
			t.Errorf("%s: expected 7 day rows, got %d; output:\n%s", lang, len(dayLines), plain)
			continue
		}

		// All rows must have identical rune count → grid aligns.
		want := 0
		for range dayLines[0] {
			want++
		}
		for i, ln := range dayLines {
			got := 0
			for range ln {
				got++
			}
			if got != want {
				t.Errorf("%s: day row %d rune count %d != row 0 rune count %d (rows misaligned)",
					lang, i, got, want)
			}
		}
	}
}

// TestHeatColors_StrictlyMonotonic verifies fix #4: all 5 heat-color levels
// are distinct ANSI codes (no duplicates that flatten the visible gradient).
func TestHeatColors_StrictlyMonotonic(t *testing.T) {
	seen := map[string]int{}
	for i, c := range heatColors {
		s := string(c)
		if prev, dup := seen[s]; dup {
			t.Errorf("heatColors[%d]=%q duplicates level %d (gradient flat)", i, s, prev)
		}
		seen[s] = i
	}
	if len(seen) != len(heatColors) {
		t.Errorf("expected %d distinct levels, got %d", len(heatColors), len(seen))
	}
}

// ----- drill-down renderBars30 tests ----------------------------------------

func TestRenderBars30_NoSelection_NoSidePanel(t *testing.T) {
	end := time.Date(2026, 5, 2, 0, 0, 0, 0, time.UTC)
	data := map[string]int{end.Format("2006-01-02"): 3}

	out := renderBars30(data, end, 80, 0, time.Time{}, nil, 0, "", enStrings())
	plain := stripANSI(out)

	if strings.Contains(plain, "Selected") || strings.Contains(plain, "选中") {
		t.Errorf("no-selection mode must not show panel header; got:\n%s", plain)
	}
}

func TestRenderBars30_WithSelection_WidePanel(t *testing.T) {
	end := time.Date(2026, 5, 2, 0, 0, 0, 0, time.UTC)
	sel := time.Date(2026, 4, 23, 0, 0, 0, 0, time.UTC)
	// Use the canonical Go reference time "2006-01-02"; "2026-01-02" yields
	// "0026-..." and silently disables the data lookup (regression catch).
	data := map[string]int{end.Format("2006-01-02"): 3}

	proj := "work"
	tasks := []store.Feature{
		{ID: "aaa", Title: "finish review", ProjectName: &proj, IsDone: 1},
	}

	out := renderBars30(data, end, 80, 5, sel, tasks, 0, "", enStrings())
	plain := stripANSI(out)

	if !strings.Contains(plain, "Selected") {
		t.Errorf("wide layout: expected 'Selected' in output; got:\n%s", plain)
	}
	if !strings.Contains(plain, "finish review") {
		t.Errorf("wide layout: expected task title in output; got:\n%s", plain)
	}
	// Total = 3; ensures the data map key actually matches what render reads.
	if !strings.Contains(plain, "3 done") {
		t.Errorf("wide layout: expected '3 done' total in title; got:\n%s", plain)
	}
}

func TestRenderBars30_NarrowSelection_PanelBelow(t *testing.T) {
	end := time.Date(2026, 5, 2, 0, 0, 0, 0, time.UTC)
	sel := time.Date(2026, 4, 23, 0, 0, 0, 0, time.UTC)
	data := map[string]int{}

	tasks := []store.Feature{
		{ID: "bbb", Title: "narrow task", IsDone: 1},
	}

	out := renderBars30(data, end, 50, 0, sel, tasks, 0, "", enStrings())
	plain := stripANSI(out)

	// Both selected header and task title should appear somewhere in the output.
	if !strings.Contains(plain, "Selected") {
		t.Errorf("narrow layout: expected 'Selected' header; got:\n%s", plain)
	}
	if !strings.Contains(plain, "narrow task") {
		t.Errorf("narrow layout: expected task title; got:\n%s", plain)
	}
}

func TestRenderBars30_StreakChip(t *testing.T) {
	end := time.Date(2026, 5, 2, 0, 0, 0, 0, time.UTC)
	data := map[string]int{}

	out := renderBars30(data, end, 80, 12, time.Time{}, nil, 0, "", enStrings())
	if !strings.Contains(out, "🔥") {
		t.Errorf("expected 🔥 streak chip in output; got:\n%s", out)
	}
}

func TestRenderBars30_TruncatesLongTitle(t *testing.T) {
	end := time.Date(2026, 5, 2, 0, 0, 0, 0, time.UTC)
	sel := time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC)
	data := map[string]int{}

	longTitle := "这是一个非常非常非常非常非常非常非常非常非常非常长的任务标题用来测试截断功能"
	tasks := []store.Feature{
		{ID: "ccc", Title: longTitle, IsDone: 1},
	}

	out := renderBars30(data, end, 50, 0, sel, tasks, 0, "", zhStrings())
	plain := stripANSI(out)

	if !strings.Contains(plain, "…") {
		t.Errorf("long CJK title should be truncated with '…'; got:\n%s", plain)
	}
	// Verify no line exceeds the terminal width in visual columns.
	for _, line := range strings.Split(out, "\n") {
		w := lipgloss.Width(line)
		if w > 55 { // allow small slack for ANSI overhead in lipgloss.Width
			t.Errorf("line too wide (%d > 55): %q", w, line)
		}
	}
}

func TestRenderTaskPanel_OverflowBelow(t *testing.T) {
	date := time.Date(2026, 4, 23, 0, 0, 0, 0, time.UTC)
	tasks := make([]store.Feature, 12)
	for i := range tasks {
		tasks[i] = store.Feature{ID: "f" + string(rune('a'+i)), Title: "task", IsDone: 1}
	}

	lines := renderTaskPanel(date, tasks, 0, 40, enStrings())
	// scroll=0, 12 tasks: should see 10 + "↓ 2 more" line
	plain := strings.Join(lines, "\n")
	if !strings.Contains(plain, "more") && !strings.Contains(plain, "↓") {
		t.Errorf("expected overflow-below indicator; lines:\n%s", plain)
	}
}

func TestRenderTaskPanel_OverflowAbove(t *testing.T) {
	date := time.Date(2026, 4, 23, 0, 0, 0, 0, time.UTC)
	tasks := make([]store.Feature, 12)
	for i := range tasks {
		tasks[i] = store.Feature{ID: "f" + string(rune('a'+i)), Title: "task", IsDone: 1}
	}

	lines := renderTaskPanel(date, tasks, 2, 40, enStrings())
	plain := strings.Join(lines, "\n")
	if !strings.Contains(plain, "above") && !strings.Contains(plain, "↑") {
		t.Errorf("expected overflow-above indicator; lines:\n%s", plain)
	}
}

// ----- streak algorithm tests -----------------------------------------------

func TestStreakAlgorithm_ThreeDays(t *testing.T) {
	today := time.Date(2026, 5, 2, 0, 0, 0, 0, time.UTC)
	data := map[string]int{
		"2026-05-02": 3,
		"2026-05-01": 2,
		"2026-04-30": 1,
		// 2026-04-29 = 0 → streak stops at 3
	}
	got := computeStreak(data, today)
	if got != 3 {
		t.Errorf("streak: got %d want 3", got)
	}
}

func TestStreakZero(t *testing.T) {
	today := time.Date(2026, 5, 2, 0, 0, 0, 0, time.UTC)
	data := map[string]int{} // today has 0 completions
	got := computeStreak(data, today)
	if got != 0 {
		t.Errorf("streak zero: got %d want 0", got)
	}
}

func TestStreakMax30(t *testing.T) {
	today := time.Date(2026, 5, 2, 0, 0, 0, 0, time.UTC)
	data := make(map[string]int)
	for i := 0; i < 35; i++ {
		d := today.AddDate(0, 0, -i)
		data[d.Format("2006-01-02")] = 5
	}
	got := computeStreak(data, today)
	if got != 30 {
		t.Errorf("streak capped at 30: got %d want 30", got)
	}
}

// TestPadOrTruncToWidth covers the helper used by month-header alignment.
func TestPadOrTruncToWidth(t *testing.T) {
	cases := []struct {
		in    string
		width int
		want  string
	}{
		{"Jan", 3, "Jan"},      // exact
		{"May", 3, "May"},      // exact
		{"5月", 3, "5月"},      // ASCII '5' (width 1) + CJK '月' (width 2) = exactly 3
		{"X", 3, "X  "},        // pad
		{"Jan", 5, "Jan  "},    // pad
		{"longer", 3, "lon"},   // truncate
		{"工作", 3, "工 "},     // CJK width 2; one CJK rune = width 2, then pad 1
	}
	for _, c := range cases {
		got := padOrTruncToWidth(c.in, c.width)
		if got != c.want {
			t.Errorf("padOrTruncToWidth(%q, %d) = %q want %q", c.in, c.width, got, c.want)
		}
	}
}
