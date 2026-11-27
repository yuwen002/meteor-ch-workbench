# 代码索引

> 按文件列出职责、关键入口与依赖关系，便于人和 AI 快速定位。
> 更新时间：2026-11-27

---

## 仓库总览

```
meteor-ch-workbench/
├── main.go                 Fyne 应用入口：NewMainWindow + Lifecycle
├── go.mod / go.sum         Go 依赖（fyne.io/fyne/v2 + clickhouse-go/v2）
├── internal/
│   ├── app/
│   │   └── app.go          业务门面：Query / CancelQuery / TestConnection / 元数据 / 持久化
│   ├── clickhouse/         数据库交互层
│   │   ├── service.go      连接池 + Query + CancelQuery + 表/视图/集群元数据
│   │   ├── metadata.go     GetDatabases / GetTables / GetViews / GetClusters
│   │   ├── query.go        QueryResult / ColumnInfo / QueryParams
│   │   ├── exec.go         DDL / EXECUTE / RowsAffected
│   │   ├── export.go       结果集导出注册表（CSV/JSON/MD/INSERT/YAML/TSV/XML）+ ExportResultTo(writer, ...)
│   │   ├── export_xlsx.go  XLSX 导出（excelize）
│   │   └── import.go       结果集导入注册表（CSV/JSON/YAML/TSV）
│   ├── config/             持久化层（JSON 文件 + sync.RWMutex）
│   │   ├── manager.go      Connection + Folder CRUD + connections.json
│   │   ├── history.go      HistoryEntry + 200 条上限 + history.json
│   │   ├── saved_sql.go   SavedSQL + 收藏 CRUD + saved_sqls.json
│   │   ├── tabs.go         TabState + Tabs 持久化 + tabs.json
│   │   ├── settings.go     AppSettings（RowLimit + 是否限制）
│   │   ├── export.go       配置导出注册表（JSON/YAML/CSV/DSN/DBeaver）
│   │   └── import.go       配置导入注册表（JSON/YAML/CSV/DSN）
│   └── gui/
│       └── fyneapp/        Fyne v2 GUI 层
│           ├── main_window.go       主窗口构建 + 快捷键 + Tree 懒加载 + 右键菜单 + AppTabs 标签页 + 分页 + EXPLAIN + 定时刷新 + 单元格编辑 + 搜索框
│           ├── result_table.go      Table 包装器：BaseWidget + TableWithHeaders + filteredRows 搜索过滤 + SecondaryTappable 右键菜单
│           ├── result_export.go     ShowFileSave 对话框 + ExportResultTo 直写
│           ├── connection_dialog.go 连接新建/编辑/删除/测试对话框 + 文件夹新建/编辑对话框
│           ├── tree_wrapper.go      Tree 右键包装器：嵌入 widget.BaseWidget + widget.Tree，实现 SecondaryTappable 捕获右键/长按
│           ├── side_panels.go       侧边栏四 Tab：连接 / 历史 / 收藏 / 配置
│           ├── tabs.go              QueryTab 结构体 + 持久化序列化/反序列化 + looksLikeSelect + appendLimitOffset 辅助
│           └── app_state.go         Backend() 全局 App 单例 + Shutdown()
└── docs/
    ├── API.md              后端公开方法契约
    ├── FEATURES.md         用户视角功能说明
    ├── CODE_INDEX.md       本文档
    └── migration/
        └── FYNE_PLAN.md    Fyne 迁移进度表
```

**依赖方向（单向，无循环）：**

```
main.go
  └─> internal/gui/fyneapp (UI 层)
        └─> internal/app (业务门面)
              └─> internal/clickhouse (数据库交互)
              └─> internal/config (持久化)
                    └─> encoding/json
```

---

## Fyne GUI 层 (`internal/gui/fyneapp/`)

### main_window.go (18 KB)
主窗口 1 号文件。职责：
- **窗口布局**：HSplit（侧边栏 AppTabs vs 中心区）+ VSplit（编辑器 vs 结果表）
- **自定义快捷键**：`runShortcut`（Ctrl/Cmd+Enter → onRunQuery）+ `stopShortcut`（Esc → onStopQuery），实现 `fyne.Shortcut` 接口，注册到 `mw.window.Canvas().AddShortcut`
- **数据库树**：`widget.NewTree` 四参数模板 + `parseNodeID` 解析节点 ID（`conn:xxx` / `db:xxx:yyy` / `tbl:xxx:yyy:zzz` / `folder:xxx` / `cluster:xxx`）+ `onBranchOpened` 懒加载（本地 `dbCache`）
- **Tree 右键菜单**：`treeWrapper` 包装器 + `onTreeRightClick` 按节点类型分发（conn → 测试/编辑/删除、folder → 编辑/删除、db → USE/SHOW TABLES、tbl → SELECT/DESCRIBE/SHOW CREATE/TRUNCATE/DROP、view → SELECT/DESCRIBE/SHOW CREATE）
- **标签页系统**：`AppTabs` + `tabItems` 数组 + `onTabSelected` 回调参数是 `*TabItem`（需遍历映射回索引）+ `rebuildAppTabs` / `refreshAppTabs` / `restoreActiveTab`
- **结果区 Header**：搜索 Entry + 清除按钮 + 分页（pageSize 50/100/200/500 + 上一页/下一页）+ EXPLAIN 按钮 + 定时刷新 PopUpMenu（5s/10s/30s/停止）+ 导出按钮 + status label
- **onRunQuery / runQueryInTab**：拼接 LIMIT/OFFSET（`looksLikeSelect` 判定）+ goroutine 执行 + `fyne.Do` 回 UI
- **onExplain**：检测并去重 SQL 中已有 EXPLAIN 前缀
- **onToggleAutoRefresh**：`time.Ticker` + `refreshStop channel` 优雅停止 + `fyne.Do` 回 UI
- **onCellEdit / guessTargetTable**：点击单元格 → Dialog 生成 ALTER UPDATE（三种值类型）+ 从 SQL 的 FROM 自动猜表名
- **表操作**：点击 Tree 的 tbl/view 节点 → ConfirmDialog 选择 SELECT 或 DESCRIBE

关键结构体：`MainWindow`（持有 sidebar/sidebarWrapper/lastSelectedNode/editor/resultTable/appTabs/tabs/connections/folders/dbCache/pageSize/pageOffset/autoRefresh/refreshStop）

### result_table.go (~8 KB)
Table 包装器，嵌入 `widget.BaseWidget` + `*widget.Table`，同时实现 `fyne.SecondaryTappable`：
- 包装器结构：`CreateRenderer` / `MinSize` 透传给内部 Table，`Widget()` 返回 `*ResultTable` 自身（`fyne.CanvasObject`）
- `filteredRows []int` 维护「可见行 → 原始行号」映射，所有 Table 回调自动适配
- `SetSearch(text)` → `recomputeFilter()`：全列大小写不敏感子串匹配，懒过滤
- `OnSelected` 缓存 `lastCol` / `lastRow`（绕过 Fyne Table 无公开选中 API）
- `TappedSecondary` 用缓存值触发 `onRightClick` 回调 → 弹右键菜单
- ShowHeaderRow + ShowHeaderColumn + HideSeparators
- `setColumnWidths`：采样 200 行自动计算列宽（80~420 范围）
- 辅助函数：`cellToString` / `sqlLiteral` / `buildInsertSQL` / `rowToTSV`

### tree_wrapper.go (0.9 KB)
Tree 右键包装器，绕过 Fyne Tree 无公开选中 API 的限制：
- 嵌入 `widget.BaseWidget` + `*widget.Tree`，实现 `fyne.SecondaryTappable` 接口
- `TappedSecondary(*fyne.PointEvent)` 捕获右键/长按 → 调 `onRightClick` 回调（传入缓存的选中节点 ID + 绝对坐标）
- `getNodeID func() TreeNodeID` 闭包从 MainWindow 取 `lastSelectedNode`
- `CreateRenderer` / `MinSize` 透传给内部 Tree

### result_export.go (2.2 KB)
结果集导出对话框：
- Backend().GetResultExportFormats() 自动感知可用格式
- formatSelect（Select 控件）+ ShowFileSave URIWriteCloser
- clickhouse.ExportResultTo(writer, format.ID, cols, rows, "") 直写 io.Writer

### connection_dialog.go (4.9 KB)
连接/文件夹 CRUD：
- `NewConnectionDialog(window, onSaved)` 构造
- `showNew` / `showEdit(connID)` / `showDelete(connID)` / `showNewFolder` / `showEditFolder(folderID)`
- TestConnection 后端调用 + dialog 显示结果
- buildEditDialog 抽取的内部实现（showNew/showEdit 共用）
- 表单字段：Name / Host / Port / Database / Username / Password / Folder 归属

### side_panels.go (6.6 KB)
侧边栏 AppTabs（四个 TabItem）：
- **连接 Tab**：`buildConnectionPanel` — 嵌入 main_window.sidebar + "+ 连接" / "+ 文件夹" 按钮
- **历史 Tab**：`buildHistoryPanel` — widget.List + OnSelected 回填编辑器 + Backend().GetQueryHistory / ClearQueryHistory
- **收藏 Tab**：`buildSavedPanel` — widget.List + OnSelected 回填 + Modal 收藏对话框（SaveSQL）
- **配置 Tab**：`buildConfigPanel` — GetExportFormats + passwordCheck + ExportConfigByFormat + ImportConfig
- `truncateSQL(s, max)` — 取第一条非空行截断显示

### tabs.go (2.8 KB)
QueryTab + 持久化工具：
- `QueryTab` 结构体：ID / Title / SQL / QueryID / Executing / Result（clickhouse.QueryResult）
- `splitTabsFromBackend` / `restoreTabsFromBackend`：Backend().LoadTabs / SaveTabs 序列化/反序列化
- `NewQueryTab(sql)` 默认标题 "新查询"
- 辅助函数：`splitLines` / `trimSpace` / `looksLikeSelect`（SELECT/WITH/SHOW/DESCRIBE/EXPLAIN）/ `appendLimitOffset`（已有 LIMIT 时跳过，OFFSET>0 时拼 LIMIT n OFFSET m）

### app_state.go (0.2 KB)
Backend() 全局单例 + Shutdown() 入口。

---

## 后端门面 (`internal/app/app.go`)

所有 UI 层调后端的统一入口。**不依赖任何 GUI 代码**。

```go
type App struct {
    svc  *clickhouse.Service
    cfg  *config.Manager
    hist *config.HistoryManager
    saved *config.SavedSQLManager
    tabs *config.TabManager
    settings *config.SettingsManager
}
```

关键公开方法：
| 方法 | 职责 |
|------|------|
| `Query(connID, sql, queryID string) (*clickhouse.QueryResult, error)` | 执行查询，queryID 为空则不可取消 |
| `CancelQuery(queryID string) bool` | 停止正在运行的查询 |
| `TestConnection(connID string) error` | 验证连通性 |
| `GetAllConnections() []config.Connection` | 列表 |
| `UpsertConnection(c config.Connection) error` | 新增/修改（全量写回） |
| `DeleteConnection(id string) error` | 删除 |
| `GetFolders() []config.Folder` | 列表 |
| `UpsertFolder(f config.Folder) error` | 新增/修改 |
| `GetDatabases(connID string) ([]clickhouse.DatabaseInfo, error)` | 元数据 |
| `GetTables(connID, db string) ([]clickhouse.TableInfo, error)` | 元数据 |
| `GetViews(connID, db string) ([]clickhouse.ViewInfo, error)` | 元数据 |
| `GetClusters(connID string) ([]clickhouse.ClusterInfo, error)` | 元数据 |
| `GetResultExportFormats() []clickhouse.ResultExportFormat` | 注册表自动感知 |
| `GetExportFormats() []config.ExportFormat` | 配置导出格式 |
| `ExportConfigByFormat(fmtID string, sanitize bool) (string, []string, error)` | 导出 |
| `ImportConfig(content string) error` | 自动识别格式导入 |
| `GetQueryHistory() ([]config.HistoryEntry, error)` | 历史 |
| `ClearQueryHistory() error` | 清空历史 |
| `SaveSQL(name, sql, connID string) (config.SavedSQL, error)` | 收藏 |
| `GetSavedSQL() ([]config.SavedSQL, error)` | 收藏列表 |
| `LoadTabs() ([]config.TabState, int, error)` | 标签页恢复 |
| `SaveTabs(tabs []config.TabState, activeIdx int) error` | 标签页持久化 |

---

## clickhouse 层 (`internal/clickhouse/`)

### service.go (14 KB)
核心服务：`Service{connections map[string]*connWrapper}`。每个 connWrapper 持有 `driver.Conn` + `running map[string]context.CancelFunc`（query cancellation）。
- `Query(ctx, connID, sql, queryID string)` — 注册 context 到 running → 执行查询 → 注销 → 返回
- `CancelQuery(queryID string) bool` — 从 running 取 CancelFunc 并执行
- 连接懒加载：首次 Query 时按 Connection 配置建连接

### metadata.go (7 KB)
`GetDatabases(connID)` / `GetTables(connID, db)` / `GetViews(connID, db)` / `GetClusters(connID)` — 直接跑 SHOW / system.tables / system.clusters 查询

### query.go (3.4 KB)
`QueryResult` / `ColumnInfo` / `QueryParams` 类型定义

### export.go (19 KB)
结果集导出**注册表**：
```go
var ResultExporters = map[string]ResultExporter{
    "csv": {...}, "json": {...}, "markdown": {...},
    "insert": {...}, "yaml": {...}, "tsv": {...}, "xml": {...},
}
```
- `ExportResultTo(w io.Writer, formatID string, columns []string, rows [][]interface{}, tableName string) (int64, error)` — **直接写 io.Writer**，Fyne GUI 层 ShowFileSave 的 URIWriteCloser 直接传入即可
- `GetResultExportFormats() []ResultExportFormat` — UI 层用来渲染格式下拉

### export_xlsx.go (7 KB)
XLSX 格式的独立实现（excelize 库），不进注册表（体积大 + 需额外依赖）

### import.go (17 KB)
结果集导入注册表（CSV/JSON/YAML/TSV），配合 IMPORT 生成

### exec.go (5.3 KB)
DDL / EXECUTE / RowsAffected

---

## config 层 (`internal/config/`)

### manager.go (12 KB)
Connection + Folder 管理。JSON 全量写回 + `sync.RWMutex`。
- `Connections` / `Folders` 字段
- `UpsertConnection` / `DeleteConnection` / `UpsertFolder` / `DeleteFolder`
- 自动去重（按 Host+Port 检查）

### history.go (3.9 KB)
查询历史：HistoryEntry{SQL, Success, DurationMs, ConnectionID, Timestamp}。200 条上限，按时间倒序。

### saved_sql.go (5.6 KB)
收藏 SQL：SavedSQL{Name, SQL, ConnectionID, CreatedAt}。

### tabs.go (3.5 KB)
标签页持久化：TabState{Title, SQL} + activeIdx。重启恢复。

### settings.go (3.5 KB)
AppSettings{RowLimit int, NoLimit bool}。

### export.go (9.3 KB)
配置导出注册表（JSON/YAML/CSV/DSN/DBeaver）。sanitize 字段深拷贝后清空密码。

### import.go (17 KB)
配置导入注册表。JSON/YAML/CSV/DSN 自动识别（按首字节/扩展名/键名特征匹配）。

---

## 附录：测试文件

```
internal/clickhouse/
  ├── cancel_test.go            Query 取消测试
  ├── exec_batch_test.go        批量执行测试
  ├── export_test.go            导出往返一致性
  ├── export_formats_test.go    各格式覆盖
  ├── mutation_test.go          ALTER 测试
  └── service_test.go           连接基础测试

internal/config/
  ├── export_test.go            配置导出往返
  ├── history_test.go           历史持久化
  ├── import_test.go            配置导入自动识别
  ├── saved_sql_test.go         收藏持久化
  ├── settings_test.go          设置持久化
  └── tabs_test.go              标签页持久化
```

全部采用表驱动 + 中文用例名 + 覆盖正常/边界/往返。

---

## 已废弃的旧前端

项目原使用 **Wails v3 + Vue 3 + TypeScript + Tailwind CSS + Monaco Editor** 作为 GUI 层，源码位于 `frontend/` 目录（已于 2025-11-27 整体删除）。旧 UI 包含：
- Monaco Editor SQL 编辑器（懒加载 + SQL 语言包）
- 数据库树 + 结果网格（@tanstack/vue-virtual 虚拟滚动）
- 右键菜单、多条件筛选面板、单元格编辑栏
- vue-i18n 中英双语
- Vite 构建产物 → Wails runtime embed

迁移原因：Vue 前端构建链路复杂（npm/Vite/Tailwind/Monaco）、Wails CLI 版本不兼容、桌面打包体积大（含完整浏览器 runtime）。迁移至 Go + Fyne 后，**零前端依赖、33 MB 单文件、跨平台原生 GUI**。

迁移进度见 `docs/migration/FYNE_PLAN.md`。