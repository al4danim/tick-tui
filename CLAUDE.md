# CLAUDE.md

## 项目概述

`tick-tui` 是 Tick 任务管理系统的命令行 TUI 客户端，基于 Go + bubbletea。
设计理念是 lazygit 风格的窄窗口（约 40 字符宽），全程同一画面，没有任何弹框 / modal / popup —— 所有编辑就地内联。

服务端是 `~/Sync/feature-check/` 的 FastAPI（也是同一台机器或 Tailscale 内的另一台），
通过 HTTP API 通讯，配置文件 `~/.config/tick/config` 给出 `TICK_HOST` 和 `TICK_TOKEN`。

桌面 Tick（pywebview）和 Obsidian 插件也连同一个服务端，三个客户端各有侧重。

## 架构

```
cmd/tick/main.go           入口：加载配置、建客户端、跑 bubbletea
internal/api/
  client.go                HTTP 客户端（GetToday/GetProjects/Create/Update/MarkDone/Undone/Delete）
  types.go                 Feature / TodayResponse / ProjectItem
  client_test.go           httptest 覆盖每个方法的 happy + error
internal/config/
  config.go                读 ~/.config/tick/config（key=value，行内 ` #` 注释）
  config_test.go
internal/tui/
  model.go                 Model + 状态机常量 + buildRows + 项目分组排序
  update.go                Update：消息分发、API tea.Cmd、按键 handler
  view.go                  View：列表渲染、padBetween、scrollWindow
  editor.go                ComputeGhostText / renderTitleWithGhost / renderProjectField
  styles.go                lipgloss 样式集中
  keys.go                  bubbles/key 绑定 + shortHelp / longHelp
  update_test.go           关键状态机单测
```

依赖：`charmbracelet/bubbletea` v1.3 · `bubbles` v1.0 · `lipgloss` v1.1。

## 数据模型与服务端契约

服务端是历史包袱：响应字段 `completed_at`，写入字段 `completion_date`（POST 的 form key 和 PUT 的 JSON key）。
客户端 `api.Feature` 用 `json:"completed_at"`，`Update()` body 用 `completion_date`。

```go
type Feature struct {
    ID          int64   `json:"id"`
    Title       string  `json:"title"`
    ProjectName *string `json:"project_name"`
    IsDone      int     `json:"is_done"`     // 0/1
    CompletedAt *string `json:"completed_at"` // 可空 YYYY-MM-DD
    CreatedAt   string  `json:"created_at"`
}
```

### 端点

```
GET  /api/today        → {pending:[Feature], done:[Feature], done_today, total_today, ...}
GET  /api/projects     → [{id,name,...}]
POST /features         form: text, completion_date
PUT  /features/{id}    JSON: {title, completion_date?}
PATCH /features/{id}/done    无条件 SET completed_at = today
PATCH /features/{id}/undone  SET is_done=0, completed_at 不动
DELETE /features/{id}  204 或 200+body 都接受
```

`do()` 接受所有 2xx；`Update()` 把 project 拼到 title `"标题 @项目"` 提交，让服务端 `parse_input()` 拆。
PUT body **不发** `project_name` 字段（服务端忽略它）。

### `/api/today` 语义（v3）

- `pending` = 所有 `is_done=0`（不再按日期过滤）
- `done`    = `is_done=1 AND date(completed_at)=today`

副作用：用户编辑 done 任务把日期改到非今日 → 该任务从 today 视图消失（既不在 done 也不在 pending）。这是设计。

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
| `a`（新建 pending）   | false       | `fieldTitle` | title ↔ project 循环              | title + project   |
| `e` 在 pending 行     | false       | `fieldTitle` | title ↔ project 循环              | title + project   |
| `e` 在 done 行        | true        | `fieldDate`  | no-op（只有一个字段）             | date              |

cmdSave 始终带上 titleInput/projectInput 当前值；`dateModified` 只有用户在 fieldDate 按 ↑/↓ 时才会变 true，
未改过则 PUT 时 `completion_date` 字段不发（服务端保留原值）。

### `a` 的 Phantom Row

按 `a`：rows 顶部 prepend 一个 `kind: rowDraft` 的空行，cursor=0；下方所有行下移。
取消（ESC）或保存：`buildRows()` 重建从 today 数据，draft row 自动消失。
保存后 cmdLoadToday 拉新数据，新 feature 出现在合适的项目分组中。

### Mark Done 流程

按 `t` → mode=modeGraceUndo，graceID=feature.id → `tea.Batch(cmdMarkDone, cmdGraceTimer(3s))`
3s 内按 `u` → 发 `/undone`，回 modeList
其他键 / 3s 过期 → 回 modeList，footer 清空

服务端 `/done` **无条件**写 today（v3 决策），保证 tick 永远反映"现在完成"。

### Pending 区项目分组排序

`groupByProject` 把 pending 按项目分组，组间按 count desc，无项目组永远放最末。
done 区不分组，按服务端原顺序（创建时间倒序）。

`[` / `]` 跳上/下一个项目首行；`g` / `G` 跳当前 section（pending 或 done）的首/末行 —— 用 separator 作为 section 边界。

### Ghost Text

只在 `fieldProject` 工作（`editor.go: computeProjectGhost`）：前缀匹配 `m.projects` 列表第一个，dim 灰色 inline 显示在光标后；Tab 接受。
`fieldTitle` 不做 @-completion（由用户明确决定）。

`m.projects` 在 `Init()` 通过 `cmdLoadProjects()` 拉一次。

### CJK 安全

`renderTitleWithGhost` 用 `[]rune` 切片避免 byte 切 UTF-8 中间的 panic。
项目名 regex 用 `[^@\s]*` 而非 `\w*`，匹配 CJK 项目名。

## 完整键位

| 键            | 模式         | 作用                                          |
|---------------|--------------|-----------------------------------------------|
| `j` `k` ↑↓    | List         | 上下移动（跳过 separator）                    |
| `[` `]`       | List         | 跳上/下一个项目首行                           |
| `g` `G`       | List         | 跳当前 section 首/末行                        |
| `t`           | List         | 标 done（pending 行）+ 3s grace               |
| `u`           | GraceUndo    | grace 内反标                                  |
| `U`           | List         | done 行反标（y/n 确认）                       |
| `a`           | List         | 新建（在 pending 顶部插 draft 行）            |
| `e`           | List         | 编辑当前行（pending: title/project; done: date）|
| `D`           | List         | 删除（y/n 确认）                              |
| `r`           | List         | 手动刷新                                      |
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
./bin/tick                     # 运行（需服务端在线）
```

`go env -w GOPROXY=https://goproxy.cn,direct` 走中国镜像。

## 配置

`~/.config/tick/config`（mode 0600）：

```
TICK_HOST=http://127.0.0.1:5050
TICK_TOKEN=
```

行内注释 ` #`（空格 + 井号）会被截断。`TICK_HOST` 留空时默认 `http://127.0.0.1:5050`。

## 服务端启动

```bash
cd ~/Sync/feature-check && .venv/bin/python -m uvicorn app:app --host 127.0.0.1 --port 5050
```

`__main__` 块默认起 pywebview（桌面 Tick），跑 TUI 不需要。Mac mini 上常驻服务端时一般用 launchd 或 systemd 之类（见 feature-check/CLAUDE.md）。

## 后续待做（v2 阶段）

- 统计面板（柱状图 / 热力图 / 计数）— v1 砍掉了，复用服务端 `/api/stats` `/api/chart` `/api/heatmap`
- "全部最近 200 条" feed 视图（v1 只看今日）
- 跨日 feature 自动归档（已有服务端语义，TUI 没专门 UI）
- `/` 搜索 / 过滤
- token 鉴权下的 `/api/projects` 端点保护（服务端 🟡）
- `TodayResponse` 加 `pending_today` 字段（客户端 🟡）

## 设计决策（不要回退）

1. **tick 无条件写 today**：服务端 `/done` 端点不再保留旧 `completed_at`。补勾昨天用 `t` + `e` 改日期两步。原因：避免编辑过的 pending 在 tick 后"消失"。
2. **title 字段不做 @-completion**：用户明确说 title 不需要 @；项目改在 fieldProject 选。
3. **done 行 e 只能改 date**：避免误改已完成任务的 title/project。
4. **PUT 把 project 拼到 title**：服务端 PUT 的 `parse_input(title)` 提 @project；不发 `project_name` 字段（服务端会忽略它）。这是历史包袱，不要试图"修正"它。
5. **不做乐观更新**：所有写操作等服务端返回再 reload。Tailscale 延迟可接受。
6. **rowDraft phantom**：按 a 在 rows 顶部插一行 phantom，不动 m.today；exitEdit 通过 buildRows 自动清理。

## 相关仓库

- `~/Sync/feature-check/`     服务端 (FastAPI + SQLite)
- `~/Sync/obsidian-tick/`     Obsidian 插件（手机端使用）
- `~/Sync/zsh-tick/`          旧 zsh + fzf 客户端，已被本仓替代，待删
- `~/.claude/plans/zsh-to-do-effervescent-moler.md`  本轮设计 plan（含完整 UI 草图和键位讨论）
