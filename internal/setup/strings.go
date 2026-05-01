package setup

// Lang selects which strings table the wizard renders.
type Lang int

const (
	LangEN Lang = iota
	LangZH
)

// strings groups every user-visible label so View() never inlines a literal.
type strings struct {
	Title            string
	Question         string
	VaultLabel       string // %s = vault name
	DefaultLabel     string
	DefaultDesc      string
	CustomLabel      string
	CustomDesc       string
	Tip              string
	Hotkeys          string
	CustomPrompt     string
	CustomHelp       string
	CustomErrEmpty   string
	CustomErrNotAbs  string
	CustomErrParent  string
	BackHint         string
}

var enStrings = strings{
	Title:           "Welcome to tick",
	Question:        "Where should tick store tasks.md?",
	VaultLabel:      "Obsidian vault: %s",
	DefaultLabel:    "~/.tick/tasks.md",
	DefaultDesc:     "default, no sync",
	CustomLabel:     "Custom path...",
	CustomDesc:      "type your own",
	Tip:             "Tip: storing inside an Obsidian vault syncs across devices via Obsidian Sync.",
	Hotkeys:         "Enter confirm · ↑/↓ move · Tab toggle EN/中 · Ctrl+C quit",
	CustomPrompt:    "Absolute path to tasks.md:",
	CustomHelp:      "e.g. /Users/you/notes/.tick/tasks.md",
	CustomErrEmpty:  "path cannot be empty",
	CustomErrNotAbs: "path must be absolute (start with / or ~)",
	CustomErrParent: "parent directory does not exist and could not be created",
	BackHint:        "Esc go back",
}

var zhStrings = strings{
	Title:           "欢迎使用 tick",
	Question:        "tasks.md 放在哪里？",
	VaultLabel:      "Obsidian vault：%s",
	DefaultLabel:    "~/.tick/tasks.md",
	DefaultDesc:     "默认，不同步",
	CustomLabel:     "自定义路径...",
	CustomDesc:      "手动输入",
	Tip:             "提示：放进 Obsidian vault 可以通过 Obsidian Sync 跨设备同步。",
	Hotkeys:         "Enter 确认 · ↑/↓ 移动 · Tab 切换 EN/中 · Ctrl+C 退出",
	CustomPrompt:    "tasks.md 的绝对路径：",
	CustomHelp:      "例如 /Users/you/notes/.tick/tasks.md",
	CustomErrEmpty:  "路径不能为空",
	CustomErrNotAbs: "路径必须是绝对路径（以 / 或 ~ 开头）",
	CustomErrParent: "父目录不存在且无法创建",
	BackHint:        "Esc 返回",
}

func stringsFor(l Lang) strings {
	if l == LangZH {
		return zhStrings
	}
	return enStrings
}
