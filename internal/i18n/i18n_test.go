package i18n

import (
	"strings"
	"testing"
	"time"
)

func TestParseLang(t *testing.T) {
	cases := []struct {
		in   string
		want Lang
	}{
		{"", LangEN},
		{"en", LangEN},
		{"zh", LangZH},
		{"ZH", LangZH},
		{"unknown", LangEN},
	}
	for _, c := range cases {
		if got := ParseLang(c.in); got != c.want {
			t.Errorf("ParseLang(%q): got %v want %v", c.in, got, c.want)
		}
	}
}

func TestLang_StringRoundtrip(t *testing.T) {
	if LangEN.String() != "en" {
		t.Errorf("LangEN.String(): got %q", LangEN.String())
	}
	if LangZH.String() != "zh" {
		t.Errorf("LangZH.String(): got %q", LangZH.String())
	}
	if ParseLang(LangZH.String()) != LangZH {
		t.Error("zh roundtrip failed")
	}
}

func TestLang_Toggle(t *testing.T) {
	if LangEN.Toggle() != LangZH {
		t.Errorf("EN.Toggle != ZH")
	}
	if LangZH.Toggle() != LangEN {
		t.Errorf("ZH.Toggle != EN")
	}
}

func TestFor_returnsCorrectTable(t *testing.T) {
	en := For(LangEN)
	zh := For(LangZH)
	if en.Loading != "loading…" {
		t.Errorf("EN Loading: got %q", en.Loading)
	}
	if zh.Loading != "加载中…" {
		t.Errorf("ZH Loading: got %q", zh.Loading)
	}
}

func TestMarkedDone_BothLanguages(t *testing.T) {
	en := For(LangEN).MarkedDone(2)
	if !strings.Contains(en, "marked done") || !strings.Contains(en, "2s") {
		t.Errorf("EN MarkedDone: got %q", en)
	}
	zh := For(LangZH).MarkedDone(2)
	if !strings.Contains(zh, "已完成") || !strings.Contains(zh, "撤销") {
		t.Errorf("ZH MarkedDone: got %q", zh)
	}
}

func TestUntickConfirm_ZH(t *testing.T) {
	got := For(LangZH).UntickConfirm("买菜")
	if !strings.Contains(got, "撤销完成") {
		t.Errorf("ZH UntickConfirm: got %q", got)
	}
	if !strings.Contains(got, "买菜") {
		t.Errorf("ZH UntickConfirm should embed title; got %q", got)
	}
	if !strings.Contains(got, "y/n") {
		t.Errorf("ZH UntickConfirm should keep y/n marker; got %q", got)
	}
}

func TestDeleteConfirm_ZH(t *testing.T) {
	got := For(LangZH).DeleteConfirm("写报告")
	if !strings.Contains(got, "删除") || !strings.Contains(got, "写报告") {
		t.Errorf("ZH DeleteConfirm: got %q", got)
	}
}

func TestCopiedTitle_ZH(t *testing.T) {
	got := For(LangZH).CopiedTitle("买菜")
	if !strings.Contains(got, "已复制") {
		t.Errorf("ZH CopiedTitle: got %q", got)
	}
}

func TestErrorMsg_BothLanguages(t *testing.T) {
	if !strings.Contains(For(LangEN).ErrorMsg("disk full"), "error:") {
		t.Error("EN ErrorMsg missing 'error:'")
	}
	if !strings.Contains(For(LangZH).ErrorMsg("disk full"), "错误：") {
		t.Error("ZH ErrorMsg missing '错误：'")
	}
}

func TestConfigUpdated_BothLanguages(t *testing.T) {
	en := For(LangEN).ConfigUpdated()
	zh := For(LangZH).ConfigUpdated()
	if !strings.Contains(en, "config updated") {
		t.Errorf("EN: %q", en)
	}
	if !strings.Contains(zh, "配置已更新") {
		t.Errorf("ZH: %q", zh)
	}
}

func TestBars30Title_ZH(t *testing.T) {
	got := For(LangZH).Bars30Title(87)
	if !strings.Contains(got, "最近 30 天") {
		t.Errorf("ZH Bars30Title prefix: got %q", got)
	}
	if !strings.Contains(got, "完成 87") {
		t.Errorf("ZH Bars30Title count: got %q", got)
	}
}

func TestHeatYearTitle_WithAndWithoutMax(t *testing.T) {
	zh := For(LangZH)
	titleWithMax := zh.HeatYearTitle(1247, 12, "3月14日")
	if !strings.Contains(titleWithMax, "完成 1247 件") {
		t.Errorf("ZH HeatYearTitle total: got %q", titleWithMax)
	}
	if !strings.Contains(titleWithMax, "峰值 12") {
		t.Errorf("ZH HeatYearTitle max: got %q", titleWithMax)
	}

	titleNoMax := zh.HeatYearTitle(0, 0, "")
	if strings.Contains(titleNoMax, "峰值") {
		t.Errorf("ZH HeatYearTitle should hide max when 0; got %q", titleNoMax)
	}
}

func TestBars30Stats_ZH(t *testing.T) {
	got := For(LangZH).Bars30Stats(2.9, 8, "4月23日")
	if !strings.Contains(got, "日均 2.9") {
		t.Errorf("ZH Bars30Stats avg: got %q", got)
	}
	if !strings.Contains(got, "峰值 8") {
		t.Errorf("ZH Bars30Stats max: got %q", got)
	}
}

func TestHeatTodayLabel_ZH(t *testing.T) {
	t0 := time.Date(2026, 5, 2, 0, 0, 0, 0, time.UTC) // Saturday
	got := For(LangZH).HeatTodayLabel(t0)
	if !strings.Contains(got, "今天=") {
		t.Errorf("ZH HeatTodayLabel marker: got %q", got)
	}
	if !strings.Contains(got, "周六") {
		t.Errorf("ZH HeatTodayLabel weekday: got %q", got)
	}
	if !strings.Contains(got, "5月2日") {
		t.Errorf("ZH HeatTodayLabel monthday: got %q", got)
	}
}

func TestWeekdayShort(t *testing.T) {
	if For(LangEN).WeekdayShort(time.Saturday) != "Sat" {
		t.Error("EN Saturday")
	}
	if For(LangZH).WeekdayShort(time.Saturday) != "周六" {
		t.Error("ZH Saturday")
	}
	if For(LangEN).WeekdayShort(time.Sunday) != "Sun" {
		t.Error("EN Sunday")
	}
	if For(LangZH).WeekdayShort(time.Sunday) != "周日" {
		t.Error("ZH Sunday")
	}
}

func TestMonthShort(t *testing.T) {
	if For(LangEN).MonthShort(time.May) != "May" {
		t.Error("EN May")
	}
	if For(LangZH).MonthShort(time.May) != "5月" {
		t.Error("ZH May")
	}
	if For(LangEN).MonthShort(time.December) != "Dec" {
		t.Error("EN December")
	}
	if For(LangZH).MonthShort(time.December) != "12月" {
		t.Error("ZH December")
	}
}

func TestMonthDay(t *testing.T) {
	t0 := time.Date(2026, 4, 23, 0, 0, 0, 0, time.UTC)
	if got := For(LangEN).MonthDay(t0); got != "Apr 23" {
		t.Errorf("EN MonthDay: got %q", got)
	}
	if got := For(LangZH).MonthDay(t0); got != "4月23日" {
		t.Errorf("ZH MonthDay: got %q", got)
	}
}

func TestLongHelp_ContainsLineBreaks(t *testing.T) {
	en := For(LangEN).LongHelp()
	if strings.Count(en, "\n") < 5 {
		t.Errorf("EN LongHelp should be multi-line; got:\n%s", en)
	}
	zh := For(LangZH).LongHelp()
	if !strings.Contains(zh, "导航") {
		t.Errorf("ZH LongHelp should contain '导航'; got:\n%s", zh)
	}
	if !strings.Contains(zh, "统计") {
		t.Errorf("ZH LongHelp should contain '统计'; got:\n%s", zh)
	}
	if !strings.Contains(zh, "l 切换 EN/中") {
		t.Errorf("ZH LongHelp should mention l toggle; got:\n%s", zh)
	}
}

func TestShortHelp_BothLanguages(t *testing.T) {
	en := For(LangEN).ShortHelp
	zh := For(LangZH).ShortHelp
	if !strings.Contains(en, "a add") {
		t.Errorf("EN ShortHelp: got %q", en)
	}
	if !strings.Contains(zh, "a 新建") {
		t.Errorf("ZH ShortHelp: got %q", zh)
	}
}

func TestStatsTooNarrow_BothLanguages(t *testing.T) {
	if !strings.Contains(For(LangEN).StatsTooNarrow, "too narrow") {
		t.Error("EN narrow message missing")
	}
	if !strings.Contains(For(LangZH).StatsTooNarrow, "终端太窄") {
		t.Error("ZH narrow message missing")
	}
}

func TestChipLabels_BothLanguages(t *testing.T) {
	en := For(LangEN)
	zh := For(LangZH)
	if en.ChipTitle != "title" || en.ChipProject != "proj" || en.ChipDate != "date" {
		t.Errorf("EN chips wrong: %q %q %q", en.ChipTitle, en.ChipProject, en.ChipDate)
	}
	if zh.ChipTitle != "标题" || zh.ChipProject != "项目" || zh.ChipDate != "日期" {
		t.Errorf("ZH chips wrong: %q %q %q", zh.ChipTitle, zh.ChipProject, zh.ChipDate)
	}
}

func TestChipTitleArrow_BothLanguages(t *testing.T) {
	if got := For(LangEN).ChipTitleArrow("work"); got != "title → @work" {
		t.Errorf("EN ChipTitleArrow: got %q", got)
	}
	if got := For(LangZH).ChipTitleArrow("work"); got != "标题 → @work" {
		t.Errorf("ZH ChipTitleArrow: got %q", got)
	}
	// CJK project name should round-trip unchanged.
	if got := For(LangZH).ChipTitleArrow("家庭"); got != "标题 → @家庭" {
		t.Errorf("ZH ChipTitleArrow with CJK: got %q", got)
	}
}

func TestChipTitlePlaceholder_BothLanguages(t *testing.T) {
	if For(LangEN).ChipTitlePlaceholder != "(title)" {
		t.Errorf("EN placeholder: got %q", For(LangEN).ChipTitlePlaceholder)
	}
	if For(LangZH).ChipTitlePlaceholder != "（标题）" {
		t.Errorf("ZH placeholder: got %q", For(LangZH).ChipTitlePlaceholder)
	}
}

func TestNoOlderData_BothLanguages(t *testing.T) {
	if got := For(LangEN).NoOlderData(); got != "no older data" {
		t.Errorf("EN NoOlderData: got %q", got)
	}
	if got := For(LangZH).NoOlderData(); got != "没有更早的数据" {
		t.Errorf("ZH NoOlderData: got %q", got)
	}
}

func TestStreakLabel(t *testing.T) {
	en := For(LangEN)
	zh := For(LangZH)

	cases := []struct {
		days   int
		wantEN string
		wantZH string
	}{
		{0, "🔥 0d", "🔥 0 天"},
		{12, "🔥 12d", "🔥 12 天"},
		{29, "🔥 29d", "🔥 29 天"},
		{30, "🔥 30+d", "🔥 30+ 天"},
		{45, "🔥 30+d", "🔥 30+ 天"},
	}
	for _, c := range cases {
		if got := en.StreakLabel(c.days); got != c.wantEN {
			t.Errorf("EN StreakLabel(%d): got %q want %q", c.days, got, c.wantEN)
		}
		if got := zh.StreakLabel(c.days); got != c.wantZH {
			t.Errorf("ZH StreakLabel(%d): got %q want %q", c.days, got, c.wantZH)
		}
	}
}
