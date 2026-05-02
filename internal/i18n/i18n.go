// Package i18n holds the bilingual string table for the TUI surface.
// It deliberately stays independent of internal/setup (which has its own
// wizard-specific i18n) so neither domain leaks into the other.
package i18n

import (
	"fmt"
	"time"
)

// Lang selects which strings table the TUI renders.
type Lang int

const (
	LangEN Lang = iota
	LangZH
)

// ParseLang turns a config string ("en"/"zh") into a Lang. Unknown values
// fall back to LangEN.
func ParseLang(s string) Lang {
	switch s {
	case "zh", "ZH", "Zh":
		return LangZH
	default:
		return LangEN
	}
}

// String returns the canonical config token for a Lang ("en" / "zh").
func (l Lang) String() string {
	if l == LangZH {
		return "zh"
	}
	return "en"
}

// Toggle flips EN ↔ ZH.
func (l Lang) Toggle() Lang {
	if l == LangEN {
		return LangZH
	}
	return LangEN
}

// TUIStrings groups every user-visible literal in the TUI. View / update /
// stats render code looks up by field name; nothing inlines a literal.
type TUIStrings struct {
	lang Lang

	// --- list view ---
	Loading       string
	NoTasksHint   string // "no tasks for today · press a to add"
	AddingChip    string // " · adding"
	DoneSeparator string // " done " (label between separator dashes)

	// --- short / long help ---
	ShortHelp        string
	ShortHelpFilter  string // shown when project filter is active
	LongHelpHeading  longHelpStrings

	// --- edit footer hints ---
	EditTabToProject     string
	EditTabToTitle       string
	EditDateField        string
	EditFallback         string
	EditStickyTabProject string
	EditStickyTabTitle   string
	EditStickyDateField  string
	EditStickyFallback   string

	// --- title bar suffixes ---
	NoProjectLabel string // "(no project)"

	// --- edit-mode chip labels (right-aligned hint badges in edit mode) ---
	ChipTitle            string // "title" / "标题"
	ChipProject          string // "proj" / "项目"
	ChipDate             string // "date" / "日期"
	ChipTitlePlaceholder string // "(title)" / "（标题）" — dim placeholder when title field empty

	// --- stats footers ---
	StatsBars30Footer       string // idle footer: "←/→ pick day · esc back · S year · q quit"
	StatsBars30FooterDrill  string // drill-down footer: "←/→ pick day · ↑/↓ scroll · esc back · S year"
	StatsHeatYearFooter     string // "esc back · s 30-day · q quit"

	// --- stats labels ---
	StatsBars30TitlePrefix string // "Tick · last 30 days"
	StatsBars30Done        string // "done" / "件" suffix piece
	StatsBars30NoData      string // "no completions in this period"
	StatsHeatTitlePrefix   string // "Tick · last 365 days"
	StatsHeatMaxLabel      string // "max"
	StatsBarsAvgLabel      string // "avg"
	StatsBarsMaxLabel      string // "max"
	StatsHeatLegendLess    string // "less"
	StatsHeatLegendMore    string // "more"
	StatsHeatTodayLabel    string // "today"
	StatsTooNarrow         string // narrow-window message
}

type longHelpStrings struct {
	NavLine1     string
	NavLine2     string
	NavLine3     string
	ActionsLine1 string
	ActionsLine2 string
	ActionsLine3 string
	GraceLine    string
	EditLine     string
	DateLine     string
	StatsLine    string
	OtherLine    string
}

var enLong = longHelpStrings{
	NavLine1:     "Navigation:  j/k or ↑/↓ move (Nj/Nk repeats N times, e.g. 5j)",
	NavLine2:     "             ] / [ jump next/prev project",
	NavLine3:     "             g first · G last (within current section: pending or done)",
	ActionsLine1: "Actions:     a add (streams: Enter saves & opens next; Esc/empty stops)",
	ActionsLine2: "             e edit · t mark done · U un-tick · y copy title · D delete",
	ActionsLine3: "             p toggle project filter (uses current row's project; press again to clear)",
	GraceLine:    "Grace:       after t, press u within 3s to undo",
	EditLine:     "Edit fields: Tab next field · Shift+Tab prev · Enter save · ESC cancel",
	DateLine:     "Date field:  ↑/↓ ±1 day",
	StatsLine:    "Stats:       s 30-day chart · S year heatmap · O change folder",
	OtherLine:    "Other:       l toggle EN/中 · ? toggle help · q quit",
}

var zhLong = longHelpStrings{
	NavLine1:     "导航：       j/k 或 ↑/↓ 移动（Nj/Nk 重复 N 次，如 5j）",
	NavLine2:     "             ] / [ 跳到上/下一个项目",
	NavLine3:     "             g 当前段首 · G 当前段末（pending 或 done）",
	ActionsLine1: "操作：       a 新建（连续：Enter 保存并打开下一条；Esc/空 退出）",
	ActionsLine2: "             e 编辑 · t 完成 · U 撤销完成 · y 复制标题 · D 删除",
	ActionsLine3: "             p 切换项目过滤（用当前行的项目；再按清除）",
	GraceLine:    "宽限期：     按 t 后 3 秒内按 u 可撤销",
	EditLine:     "编辑字段：   Tab 下一字段 · Shift+Tab 上一字段 · Enter 保存 · ESC 取消",
	DateLine:     "日期字段：   ↑/↓ ±1 天",
	StatsLine:    "统计：       s 30 天柱图 · S 年度热力图 · O 修改文件夹",
	OtherLine:    "其他：       l 切换 EN/中 · ? 帮助 · q 退出",
}

var stringsEN = TUIStrings{
	lang: LangEN,

	Loading:       "loading…",
	NoTasksHint:   "no tasks for today · press a to add",
	AddingChip:    " · adding",
	DoneSeparator: " done ",

	ShortHelp:       "a add · t done · e edit · p filter · y copy · D del · l 中/EN · ? help · q quit",
	ShortHelpFilter: "p clear filter · a add · t done · e edit · y copy · D del · l 中/EN · ? help",
	LongHelpHeading: enLong,

	EditTabToProject:     "Tab → project · Enter save · Esc cancel",
	EditTabToTitle:       "Tab → title · Enter save · Esc cancel",
	EditDateField:        "↑/↓ ±1 day · Enter save · Esc cancel",
	EditFallback:         "Enter save · Esc cancel",
	EditStickyTabProject: "Tab → project · Enter save · Esc stops adding",
	EditStickyTabTitle:   "Tab → title · Enter save · Esc stops adding",
	EditStickyDateField:  "↑/↓ ±1 day · Enter save · Esc stops adding",
	EditStickyFallback:   "Enter save · Esc stops adding",

	NoProjectLabel: "(no project)",

	ChipTitle:            "title",
	ChipProject:          "proj",
	ChipDate:             "date",
	ChipTitlePlaceholder: "(title)",

	StatsBars30Footer:       " ←/→ pick day · esc back · S year · q quit",
	StatsBars30FooterDrill:  " ←/→ pick day · ↑/↓ scroll · esc back · S year",
	StatsHeatYearFooter:     " esc back · s 30-day · q quit",

	StatsBars30TitlePrefix: "Tick · last 30 days",
	StatsBars30Done:        "done",
	StatsBars30NoData:      " no completions in this period",
	StatsHeatTitlePrefix:   "Tick · last 365 days",
	StatsHeatMaxLabel:      "max",
	StatsBarsAvgLabel:      "avg",
	StatsBarsMaxLabel:      "max",
	StatsHeatLegendLess:    "less",
	StatsHeatLegendMore:    "more",
	StatsHeatTodayLabel:    "today",
	StatsTooNarrow:         "terminal too narrow · resize to ≥ 60 cols · esc back",
}

var stringsZH = TUIStrings{
	lang: LangZH,

	Loading:       "加载中…",
	NoTasksHint:   "今日无任务 · 按 a 新建",
	AddingChip:    " · 新建中",
	DoneSeparator: " 已完成 ",

	ShortHelp:       "a 新建 · t 完成 · e 编辑 · p 过滤 · y 复制 · D 删除 · l 中/EN · ? 帮助 · q 退出",
	ShortHelpFilter: "p 清除过滤 · a 新建 · t 完成 · e 编辑 · y 复制 · D 删除 · l 中/EN · ? 帮助",
	LongHelpHeading: zhLong,

	EditTabToProject:     "Tab → 项目 · Enter 保存 · Esc 取消",
	EditTabToTitle:       "Tab → 标题 · Enter 保存 · Esc 取消",
	EditDateField:        "↑/↓ ±1 天 · Enter 保存 · Esc 取消",
	EditFallback:         "Enter 保存 · Esc 取消",
	EditStickyTabProject: "Tab → 项目 · Enter 保存 · Esc 停止新建",
	EditStickyTabTitle:   "Tab → 标题 · Enter 保存 · Esc 停止新建",
	EditStickyDateField:  "↑/↓ ±1 天 · Enter 保存 · Esc 停止新建",
	EditStickyFallback:   "Enter 保存 · Esc 停止新建",

	NoProjectLabel: "（无项目）",

	ChipTitle:            "标题",
	ChipProject:          "项目",
	ChipDate:             "日期",
	ChipTitlePlaceholder: "（标题）",

	StatsBars30Footer:       " ←/→ 选某天 · esc 返回 · S 年度 · q 退出",
	StatsBars30FooterDrill:  " ←/→ 选某天 · ↑/↓ 滚动 · esc 返回 · S 年度",
	StatsHeatYearFooter:     " esc 返回 · s 30 天 · q 退出",

	StatsBars30TitlePrefix: "Tick · 最近 30 天",
	StatsBars30Done:        "完成",
	StatsBars30NoData:      " 本时间段无完成",
	StatsHeatTitlePrefix:   "Tick · 最近 365 天",
	StatsHeatMaxLabel:      "峰值",
	StatsBarsAvgLabel:      "日均",
	StatsBarsMaxLabel:      "峰值",
	StatsHeatLegendLess:    "少",
	StatsHeatLegendMore:    "多",
	StatsHeatTodayLabel:    "今天",
	StatsTooNarrow:         "终端太窄 · 请放大到 ≥60 列 · esc 返回",
}

// For returns the TUIStrings for the requested language.
func For(l Lang) TUIStrings {
	if l == LangZH {
		return stringsZH
	}
	return stringsEN
}

// Lang returns the language this strings table belongs to.
func (s TUIStrings) Lang() Lang { return s.lang }

// ----- title-bar / status -------------------------------------------------

// DoneCount renders the "X/Y done" piece in the title bar.
func (s TUIStrings) DoneCount(done, total int) string {
	if s.lang == LangZH {
		return fmt.Sprintf(" · %d/%d 完成", done, total)
	}
	return fmt.Sprintf(" · %d/%d done", done, total)
}

// MarkedDone renders the grace footer line shown after pressing `t`.
func (s TUIStrings) MarkedDone(secs int) string {
	if s.lang == LangZH {
		return fmt.Sprintf("已完成 · u 撤销（%ds）", secs)
	}
	return fmt.Sprintf("marked done · u to undo (%ds)", secs)
}

// UntickConfirm renders the un-tick confirmation prompt.
func (s TUIStrings) UntickConfirm(title string) string {
	if s.lang == LangZH {
		return fmt.Sprintf(`撤销完成 "%s"？y/n`, title)
	}
	return fmt.Sprintf(`un-tick "%s"? y/n`, title)
}

// DeleteConfirm renders the delete confirmation prompt.
func (s TUIStrings) DeleteConfirm(title string) string {
	if s.lang == LangZH {
		return fmt.Sprintf(`删除 "%s"？y/n`, title)
	}
	return fmt.Sprintf(`delete "%s"? y/n`, title)
}

// CopiedTitle renders the yank success message.
func (s TUIStrings) CopiedTitle(title string) string {
	if s.lang == LangZH {
		return fmt.Sprintf(`已复制 "%s"`, title)
	}
	return fmt.Sprintf(`copied "%s"`, title)
}

// CopyFailed renders the yank failure message.
func (s TUIStrings) CopyFailed(reason string) string {
	if s.lang == LangZH {
		return "复制失败：" + reason
	}
	return "copy failed: " + reason
}

// ErrorMsg renders a generic transient error.
func (s TUIStrings) ErrorMsg(reason string) string {
	if s.lang == LangZH {
		return "错误：" + reason
	}
	return "error: " + reason
}

// ConfigUpdated is shown after writing a new tasks-file path.
func (s TUIStrings) ConfigUpdated() string {
	if s.lang == LangZH {
		return "配置已更新 · 按 q 重启"
	}
	return "config updated · q to restart"
}

// ConfigWriteFailed renders a failed config-write transient.
func (s TUIStrings) ConfigWriteFailed(reason string) string {
	if s.lang == LangZH {
		return "配置写入失败：" + reason
	}
	return "config write failed: " + reason
}

// NoOlderData is the transient footer shown when ← would scroll past the
// earliest completion date in the user's tasks/archive.
func (s TUIStrings) NoOlderData() string {
	if s.lang == LangZH {
		return "没有更早的数据"
	}
	return "no older data"
}

// ChipTitleArrow renders the right-side chip shown while the title field is
// active and a project is set, e.g. "[title → @work]" / "[标题 → @work]".
// The "@project" piece is intentionally untranslated; "@" is the on-disk
// project sigil and we want it to round-trip with the markdown file.
func (s TUIStrings) ChipTitleArrow(project string) string {
	if s.lang == LangZH {
		return "标题 → @" + project
	}
	return "title → @" + project
}

// ----- long help ----------------------------------------------------------

// LongHelp returns the multi-line help block.
func (s TUIStrings) LongHelp() string {
	h := s.LongHelpHeading
	return h.NavLine1 + "\n" +
		h.NavLine2 + "\n" +
		h.NavLine3 + "\n" +
		h.ActionsLine1 + "\n" +
		h.ActionsLine2 + "\n" +
		h.ActionsLine3 + "\n" +
		h.GraceLine + "\n" +
		h.EditLine + "\n" +
		h.DateLine + "\n" +
		h.StatsLine + "\n" +
		h.OtherLine
}

// ----- stats labels --------------------------------------------------------

// Bars30Title renders the title line of the 30-day chart.
func (s TUIStrings) Bars30Title(total int) string {
	if s.lang == LangZH {
		return fmt.Sprintf("%s · 完成 %d 件", s.StatsBars30TitlePrefix, total)
	}
	return fmt.Sprintf("%s · %d done", s.StatsBars30TitlePrefix, total)
}

// HeatYearTitle renders the title line of the year heatmap.
// max==0 hides the max suffix.
func (s TUIStrings) HeatYearTitle(total, max int, maxDate string) string {
	if s.lang == LangZH {
		title := fmt.Sprintf("%s · 完成 %d 件", s.StatsHeatTitlePrefix, total)
		if max > 0 && maxDate != "" {
			title += fmt.Sprintf(" · 峰值 %d（%s）", max, maxDate)
		}
		return title
	}
	title := fmt.Sprintf("%s · %d done", s.StatsHeatTitlePrefix, total)
	if max > 0 && maxDate != "" {
		title += fmt.Sprintf(" · max %d (%s)", max, maxDate)
	}
	return title
}

// Bars30Stats renders the "avg X.Y/day · max N (date)" footer of the 30-day chart.
func (s TUIStrings) Bars30Stats(avg float64, max int, maxDate string) string {
	if s.lang == LangZH {
		if maxDate != "" {
			return fmt.Sprintf(" 日均 %.1f · 峰值 %d（%s）", avg, max, maxDate)
		}
		return fmt.Sprintf(" 日均 %.1f", avg)
	}
	if maxDate != "" {
		return fmt.Sprintf(" avg %.1f/day · max %d (%s)", avg, max, maxDate)
	}
	return fmt.Sprintf(" avg %.1f/day", avg)
}

// HeatTodayLabel renders the "today=Sat May 2" piece of the heatmap legend.
// The level cell character is appended by the caller (with a color style).
func (s TUIStrings) HeatTodayLabel(t time.Time) string {
	if s.lang == LangZH {
		return fmt.Sprintf(" · %s=%s %s (",
			s.StatsHeatTodayLabel,
			s.WeekdayShort(t.Weekday()),
			s.MonthDay(t),
		)
	}
	return fmt.Sprintf(" · %s=%s %s (",
		s.StatsHeatTodayLabel,
		s.WeekdayShort(t.Weekday()),
		s.MonthDay(t),
	)
}

// StreakLabel renders the streak chip "🔥 12 天" / "🔥 12d".
func (s TUIStrings) StreakLabel(days int) string {
	if s.lang == LangZH {
		if days >= 30 {
			return "🔥 30+ 天"
		}
		return fmt.Sprintf("🔥 %d 天", days)
	}
	if days >= 30 {
		return "🔥 30+d"
	}
	return fmt.Sprintf("🔥 %dd", days)
}

// SelectedHeader renders "选中 4月23日 周三" / "Selected Apr 23 Wed".
func (s TUIStrings) SelectedHeader(t time.Time) string {
	if s.lang == LangZH {
		return fmt.Sprintf("选中 %s %s", s.MonthDay(t), s.WeekdayShort(t.Weekday()))
	}
	return fmt.Sprintf("Selected %s %s", s.MonthDay(t), s.WeekdayShort(t.Weekday()))
}

// SelectedDoneCount renders "完成 12 件" / "12 done".
func (s TUIStrings) SelectedDoneCount(n int) string {
	if s.lang == LangZH {
		return fmt.Sprintf("完成 %d 件", n)
	}
	return fmt.Sprintf("%d done", n)
}

// MoreTasksAbove renders "↑ 上方 X 条" / "↑ X above".
func (s TUIStrings) MoreTasksAbove(n int) string {
	if s.lang == LangZH {
		return fmt.Sprintf("↑ 上方 %d 条", n)
	}
	return fmt.Sprintf("↑ %d above", n)
}

// MoreTasksBelow renders "↓ 还有 X 条" / "↓ X more".
func (s TUIStrings) MoreTasksBelow(n int) string {
	if s.lang == LangZH {
		return fmt.Sprintf("↓ 还有 %d 条", n)
	}
	return fmt.Sprintf("↓ %d more", n)
}

// ----- date / weekday localization -----------------------------------------

// WeekdayShort returns a localized 3-rune weekday abbreviation.
func (s TUIStrings) WeekdayShort(w time.Weekday) string {
	if s.lang == LangZH {
		return zhWeekdays[w]
	}
	return enWeekdays[w]
}

// MonthShort returns a localized 3-rune month abbreviation.
func (s TUIStrings) MonthShort(m time.Month) string {
	if s.lang == LangZH {
		return zhMonths[m-1]
	}
	return enMonths[m-1]
}

// MonthDay renders "May 2" or "5月2日".
func (s TUIStrings) MonthDay(t time.Time) string {
	if s.lang == LangZH {
		return fmt.Sprintf("%d月%d日", int(t.Month()), t.Day())
	}
	return fmt.Sprintf("%s %d", s.MonthShort(t.Month()), t.Day())
}

var enWeekdays = [...]string{"Sun", "Mon", "Tue", "Wed", "Thu", "Fri", "Sat"}
var zhWeekdays = [...]string{"周日", "周一", "周二", "周三", "周四", "周五", "周六"}
var enMonths = [...]string{"Jan", "Feb", "Mar", "Apr", "May", "Jun", "Jul", "Aug", "Sep", "Oct", "Nov", "Dec"}
var zhMonths = [...]string{"1月", "2月", "3月", "4月", "5月", "6月", "7月", "8月", "9月", "10月", "11月", "12月"}
