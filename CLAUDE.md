# CLAUDE.md

## 项目概述

`tick-tui` 是 Tick 任务管理系统的命令行 TUI 客户端，基于 Go + bubbletea。
设计理念是 lazygit 风格的窄窗口（约 40 字符宽），全程同一画面，没有任何弹框 / modal / popup —— 所有编辑就地内联。

数据直接读写本地 markdown 文件（路径由首次启动 wizard 决定，默认 fallback `~/.tick/tasks.md`），**不再依赖任何服务端**。
推荐放在 Obsidian vault 里，手机端在 Obsidian 里手敲新任务即可，tick-tui 启动时自动补全 metadata。

我（开发者）个人的实际路径是 `~/hoard/.tick/tasks.md`（hoard = 我的 Obsidian vault），仅作为本文档中举例参考；新用户的路径由 wizard 选定。

## 架构

```
cmd/tick/main.go           入口：加载配置、建 store、跑 bubbletea
internal/store/
  types.go                 Feature / TodayResponse / ProjectItem
  markdown.go              parser / serializer / 文件 IO / Store 接口实现
  markdown_test.go         单测
internal/config/
  config.go                读 / 写 ~/.config/tick/config（key=value，行内 ` #` 注释）
  config_test.go
internal/setup/
  detect.go                Obsidian vault 检测（读 obsidian.json）
  detect_test.go
  strings.go               i18n EN/ZH strings 表
  wizard.go                首次启动 wizard 的 bubbletea 子 model
  wizard_test.go
internal/tui/
  model.go                 Model + 状态机常量 + buildRows + 项目分组排序
  update.go                Update：消息分发、store tea.Cmd、按键 handler
  view.go                  View：列表渲染、padBetween、scrollWindow
  editor.go                ComputeGhostText / renderTitleWithGhost / renderProjectField
  styles.go                lipgloss 样式集中
  keys.go                  bubbles/key 绑定 + shortHelp / longHelp
  update_test.go           关键状态机单测
internal/watcher/
  watcher.go               fsnotify-based tasks.md 监听
```

依赖：`charmbracelet/bubbletea` v1.3 · `bubbles` v1.0 · `lipgloss` v1.1 · `atotto/clipboard`。

## 数据模型

```go
type Feature struct {
    ID          string  // 8-char hex (e.g. "a3k7m2x9"); empty = not yet assigned
    Title       string
    ProjectName *string
    IsDone      int     // 0/1
    CompletedAt *string // YYYY-MM-DD; nil 表示未完成
    CreatedAt   string  // YYYY-MM-DD
}
```

## 文件格式

紧凑 ASCII 单行，字段顺序自由（解析时位置无关）：

```
- [ ] buy milk @home +2026-05-01 [a3k7m2x9]
- [x] write report @work +2026-04-29 *2026-04-30 [b1d4e5f0]
- [ ] 买菜 @家庭 +2026-05-01 [c2f3a4b5]
```

| 部分 | 含义 | 必填 |
|---|---|---|
| `- [ ]` / `- [x]` | 状态 | 是 |
| 描述文本 | task title（含 CJK） | 是 |
| `@project` | 可选项目（`@` + 非空白） | 否 |
| `+YYYY-MM-DD` | 创建日 | 否（缺则 sweep 时补 today） |
| `*YYYY-MM-DD` | 完成日（仅 `[x]` 行） | 否（缺则 sweep 时补 today） |
| `[ID]` | 8 字符 hex 随机 ID，**强制行尾** | 否（缺则 sweep 时随机分配） |

ID 用 8 字符 hex（`crypto/rand` 4 字节）。**为什么不用自增数字**：手机插件 + Mac CLI 双向同步时，两边都按"max + 1"会撞 ID（实际遇到过两条任务 [63] 同 ID 导致 mark-done 走错行）。8 hex = 32 bit ≈ 40亿种，碰撞概率近 0。

正则 `\s\[([a-zA-Z0-9]{1,16})\]\s*$` — 接受 1-16 字符，兼容旧数字 ID 直到下次 sweep 自动重写为 hex。`[3 个]` 这类 CJK 描述不会误匹配（中文不是 alphanumeric）。

如果两条行碰巧同 ID（迁移残留 / 极端碰撞），sweep 会给第二条重新 roll 一个。

## 双文件 + 7 天滚动归档

```
~/hoard/.tick/
  tasks.md       ← undone + 过去 7 天的 done — 任意时刻 < 350 行 / 35 KB
  archive.md     ← 7 天前的 done；append-only；TUI 列表不读它
```

mark-done / undone 是**就地操作**（仅修改 tasks.md），不跨文件移动。

每次 `loadTasks()` 跑一次"被动 sweep"：
1. 缺 `[ID]` → 分配 `genID()`（8 hex chars）
2. 重复 `[ID]` → 第二条重新 roll
3. 缺 `+date` → 补 `+today`
4. 状态 `[x]` 但缺 `*date` → 补 `*today`（手机端手动勾选语义补丁）
5. 状态 `[x]` 且 `*date < today-7d` → 移到 archive.md

效果：手机端在 Obsidian 里手敲 `- [ ] 任务 @项目` 一行，sync 到 Mac，tick-tui 启动 → 自动补成完整一行写回。

## TUI today 语义

- `pending = tasks.md 中所有 [ ] 行`
- `done section = tasks.md 中 [x] 且 *date == today 的行`
- 历史完成（archive.md）TUI 列表不读

## bubbletea 状态机

```go
type mode int
const (
    modeList         // 默认浏览态
    modeEdit         // a 新建 / e 编辑
    modeConfirmUntick // U 后等 y/n
    modeConfirmDelete // D 后等 y/n
    modeGraceUndo    // t 后 3s grace
)

type editField int
const (
    fieldTitle
    fieldProject
    fieldDate
)
```

### Edit 模式分两种

| 进入方式             | editingDone | 起始字段     | Tab 行为                          | 字段集            |
|----------------------|-------------|--------------|-----------------------------------|-------------------|
| `a`（连续新建）       | false       | `fieldTitle` | title ↔ project 循环              | title + project   |
| `e` 在 pending 行     | false       | `fieldTitle` | title ↔ project 循环              | title + project   |
| `e` 在 done 行        | true        | `fieldDate`  | no-op（只有一个字段）             | date              |

cmdSave 始终带上 titleInput/projectInput 当前值；`dateModified` 只有用户在 fieldDate 按 ↑/↓ 时才会变 true。

### `a` 的连续添加（sticky）

按 `a` → 设 `m.addSticky=true` → enterEditNew → rows 顶部插 `rowDraft` phantom。
保存后 todayLoadedMsg 看到 sticky 仍为 true → 自动重新 enterEditNew。
ESC 或空 Enter 关闭 sticky 退出。

### Mark Done 流程

按 `t` → mode=modeGraceUndo，graceID=feature.id → `tea.Batch(cmdMarkDone, cmdGraceTimer(3s))`
3s 内按 `u` → 调 store.Undone，回 modeList
其他键 / 3s 过期 → 回 modeList，footer 清空

store.MarkDone **就地**改 `[x]` + 加 `*today`，不跨文件。

### Pending 区项目分组排序

`groupByProject` 把 pending 按项目分组，组间按 count desc，无项目组永远放最末。
done 区不分组。

`[` / `]` 跳上/下一个项目首行；`g` / `G` 跳当前 section（pending 或 done）的首/末行。

### 项目过滤（`p` 键）

按 `p` → 取当前光标行的项目作为 filter；列表只显示匹配项目（含 done）；title bar 显示 ` · @work`。
新建任务自动归该项目（enterEditNew 优先用 activeProject 预填 project 字段）。
再按 `p` 关闭 filter。

### Ghost Text

只在 `fieldProject` 工作（`editor.go: computeProjectGhost`）：前缀匹配 `m.projects` 列表第一个，dim 显示在光标后；Tab 接受。
`fieldTitle` 不做 @-completion。

`m.projects` 在 `Init()` 通过 `cmdLoadProjects()` 拉一次。

### CJK 安全

`renderTitleWithGhost` 用 `[]rune` 切片避免 byte 切 UTF-8 中间的 panic。
项目名 regex 用 `@(\S+)`（非空白即可），匹配 CJK 项目名。

## 完整键位

| 键            | 模式         | 作用                                          |
|---------------|--------------|-----------------------------------------------|
| `j` `k` ↑↓    | List         | 上下移动（跳过 separator）                    |
| `Nj` `Nk`     | List         | vim 风格：数字前缀重复 N 次（如 `5j` 下 5 行）|
| `[` `]`       | List         | 跳上/下一个项目首行                           |
| `g` `G`       | List         | 跳当前 section 首/末行                        |
| `t`           | List         | 标 done（pending 行）+ 3s grace               |
| `u`           | GraceUndo    | grace 内反标                                  |
| `U`           | List         | done 行反标（y/n 确认）                       |
| `a`           | List         | 连续新建：Enter 保存后立即开新 draft；Esc 或空 Enter 退出 |
| `p`           | List         | 切换项目过滤：按下=只显示当前光标行的项目；再按=回到全部 |
| `e`           | List         | 编辑当前行（pending: title/project; done: date）|
| `D`           | List         | 删除（y/n 确认）                              |
| `y`           | List         | 复制当前行的 title 到剪贴板                   |
| `?`           | List         | 切换详细帮助                                  |
| `q` / `Ctrl+C`| List         | 退出                                          |
| Tab           | Edit         | 切下一个字段（pending edit 内）；项目 ghost 时先接受 |
| Shift+Tab     | Edit         | 反向切                                        |
| ↑ ↓           | Edit/Date    | ±1 天                                         |
| Enter         | Edit         | 保存所有字段                                  |
| ESC           | Edit         | 丢弃                                          |
| `y`           | Confirm      | 执行 untick / delete                          |
| 任何其他键    | Confirm      | 取消                                          |

## 开发

```bash
go test ./...                  # 全部测试
make build                     # bin/tick
make install                   # cp 到 ~/.local/bin/tick
./bin/tick                     # 运行（首次启动 wizard 选路径）
```

`go env -w GOPROXY=https://goproxy.cn,direct` 走中国镜像。

## 配置

首次启动 wizard 自动写 `~/.config/tick/config`（mode 0600）：

```
TICK_TASKS_FILE=<wizard 选定的绝对路径>
```

行内注释 ` #`（空格 + 井号）会被截断。空值或字段缺失时 fallback 到默认 `~/.tick/tasks.md`。`archive.md` 自动放在同一目录。

Wizard 会扫 `~/Library/Application Support/obsidian/obsidian.json`（Mac）或 `~/.config/obsidian/obsidian.json`（Linux）列出已注册的 vault；用户选 vault 后路径自动拼成 `<vault>/.tick/tasks.md`。Tab 切英中。

## 后续待做（v2）

- 统计面板：30 天柱状图 + 年度热力图（`s` 切视图，stats 视图放开 40 字符宽度限制使用 terminal full width）
- archive 按年拆分（5 年后再考虑）
- fsnotify 自动 reload（多端同步时实时跟随外部改动）
- `/` 搜索 / 过滤

## 设计决策（不要回退）

1. **mark-done 就地不跨文件**：tasks.md 保留 7 天 done，sweep 时才挪到 archive。理由：高频路径（每次写）零跨文件 IO；done section 当天能展示；统计面板（archive.md）是 read-only 路径不影响热路径。
2. **手机端手敲容错**：解析非常宽容，缺 ID/date 自动补；`[x]` 缺 *date 当今日处理；`[x] *date < today-7d` 自动归档。让用户在 Obsidian 里随手敲一行就行。
3. **title 字段不做 @-completion**：用户明确说 title 不需要 @；项目改在 fieldProject 选。
4. **done 行 e 只能改 date**：避免误改已完成任务的 title/project。
5. **不做乐观更新**：所有写操作等 store 返回再 reload。本地 IO 极快。
6. **rowDraft phantom**：按 a 在 rows 顶部插一行 phantom，不动 m.today；exitEdit 通过 buildRows 自动清理。
7. **`a` 永远 sticky**：连续新建是默认；不再保留"加一条退出"的非 sticky 模式。

## 相关仓库

- `~/Sync/tick-obsidian/`     Obsidian 插件，配套客户端（直接读写同一个 tasks.md）
- `~/Sync/feature-check/`     旧服务端 (FastAPI + SQLite)，**已废弃**，仅作迁移源
- `~/Sync/zsh-tick/`          旧 zsh + fzf 客户端，**已废弃**
