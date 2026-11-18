# Project Map — Meteor CH Workbench

> AI 快速检索用。新会话先读这个文件，再按需深入具体文件。
> 更新时间：2026-11-18

---

## 技术栈

| 层 | 选型 |
|---|---|
| 桌面框架 | Wails v3（`@wailsio/runtime` ^3.1.0） |
| 后端 | Go 1.26，clickhouse-go/v2 |
| 前端 | Vue 3 `<script setup>` + TypeScript |
| UI | Tailwind CSS + 自定义 `.ch-*` 色板 |
| 编辑器 | Monaco Editor |
| 状态 | Pinia（单个 store：`stores/app.ts`） |
| 国际化 | vue-i18n@9，key 前缀分组：`common.*` / `editor.*` / `menu.*` |
| 构建 | Vite（前端），Wails CLI v3（打包） |

---

## 架构

```
Wails v3
  ┌─ Vue 3 SPA ──────────────────────┐
  │  App.vue（侧边栏 + 菜单 + Modal）│
  │          → QueryEditor.vue ★    │  ★ 核心视图，承载 90% 业务
  │  api.ts  ── Call.ByName ──┐     │
  └───────────────────────────┼─────┘
                              ▼
                    main.App (app.go)
                    ├─ config.Manager  → connections.json 读写
                    └─ clickhouse.Service → clickhouse-go/v2 连接池
```

**Wails 绑定规则**：`Call.ByName('main.App.<方法名>', ...args)`，全部封装在 `frontend/src/api.ts`。

---

## 后端文件（Go）

```
main.go                            入口，注册 App Service
app.go                            39 个方法（含 2 个生命周期回调）→ 前端可调用
internal/clickhouse/service.go    Query / Exec / GetDatabases / GetTables / GetViews / GetTableSchema / GetClusters / TestConnection
internal/clickhouse/export.go     结果集导出注册表（csv / json / markdown）+ QueryResult 行数上限 50000
internal/config/manager.go        连接与分组：LoadAll / Save / Delete / Reorder
internal/config/export.go         配置导出注册表（json / yaml / csv / dsn / dbeaver）
internal/config/import.go         配置导入解析器 + 合并策略
internal/config/history.go        查询历史（history.json，上限 200）
internal/config/tabs.go           标签页快照（tabs.json）
```

### App 方法速查表（app.go）

| 方法 | 用途 | 返回 |
|---|---|---|
| `GetAll` | 启动时拉全量配置 | `DataFile{folders, connections}` |
| `GetConnections` / `GetConnection(id)` | 连接 CRUD 读 | |
| `SaveConnection(conn)` | 增/改连接，自动 Invalidate 旧池 | |
| `DeleteConnection(id)` | 删连接 + 释放池 | |
| `TestConnection(conn)` | 测试连通性（Ping） | error 为失败原因 |
| `Disconnect(connectionID)` | 断开池，下次重连 | |
| `GetFolders` / `SaveFolder` / `DeleteFolder` | 分组 CRUD | |
| `ReorderFolders(ids)` / `ReorderConnections(ids)` | 拖拽后持久化顺序 | |
| `ExportConfig()` | 导出 JSON 文本（含密码） | `string` |
| `GetExportFormats()` | 导出格式元信息列表 | `ExportFormat[]` |
| `ExportConfigByFormat(format, includePassword)` | 按格式导出 | `(文本, 扩展名, error)` |
| `SaveExportAs(format, includePassword)` | 另存为对话框 | 写入路径 |
| `ImportConfig(raw)` | 导入并覆盖 | |
| `GetImportFormats()` | 导入格式元信息列表 | `ImportFormat[]` |
| `PickImportFile()` | 打开文件对话框 | `(内容, 路径, error)` |
| `ImportConfigWithOptions(raw, opts)` | 指定格式/模式导入 | `ImportResult` |
| `Query(id, sql, queryID)` | SELECT/SHOW；`queryID` 用于取消，传空串表示不可取消 | `QueryResult{columns, rows, row_count, elapsed_ms, truncated, row_limit, error?}` |
| `CancelQuery(queryID)` | 取消正在运行的查询 | `bool`（是否找到并取消） |
| `Execute(id, sql)` | DDL/DML | `ExecResult{rows_affected, error?}` |
| `GetResultExportFormats()` | 结果集导出格式列表 | `ResultExportFormat[]` |
| `SaveQueryResultAs(format, columns, rows)` | 结果集另存为 | 写入路径 |
| `GetQueryHistory` / `DeleteQueryHistory` / `ClearQueryHistory` | 查询历史 | `HistoryEntry[]` |
| `GetTabs` / `SaveTabs(snapshot)` | 标签页快照读写 | `TabsFile` |
| `GetDatabases(id)` | SHOW DATABASES | `string[]` |
| `GetTables(id, db)` | 库下表名+引擎+注释（排除视图） | `TableInfo[]{name, engine?, comment?}` |
| `GetViews(id, db)` | 库下视图（引擎以 View 结尾） | `TableInfo[]` |
| `GetTableSchema(id, db, table)` | 表结构（编辑表用） | `ColumnInfo[]{name, type, default?, comment?}` |
| `GetClusters(id)` | system.clusters | `ClusterInfo[]` |
| `OpenConfigFolder` | 打开配置目录 | |
| `GetAppVersion` | "0.1.0" | |

**关键约定**：
- `Query` / `Execute` 的 SQL 错误放返回体的 `.error` 字段，Go error 为 nil。前端从 `.error` 读错误，不是 catch 异常。
- `Query` 行数上限 `DefaultQueryRowLimit = 50000`，超限置 `truncated = true`。
- 导出/导入均用注册表模式：新增格式只需在对应 `export.go` / `import.go` 追加一项，前端通过 `Get*Formats()` 自动感知。
- 配置导出脱敏基于深拷贝（`sanitize`），不会污染内存中的真实配置。

---

## 前端文件（Vue 3 + TS）

```
src/
├── App.vue                      主框架：顶栏 MenuBar + 侧边栏 + 路由出口
├── main.ts                      入口（createApp + registerPinia + registerI18n + mount）
├── i18n.ts                      vue-i18n 初始化 + 语言检测
├── api.ts                       全部 Call.ByName 封装（与 App 方法一一对应，含导入导出 / 历史 / 标签 / 结果导出）
├── types.ts                     共享接口：Connection, Folder, DataFile, ColumnInfo, TableInfo, ClusterInfo, QueryResult, ExecResult, ExportFormat, ImportFormat, ImportOptions, ImportResult, HistoryEntry, TabsFile, ResultExportFormat
├── style.css                    Tailwind + 全局样式 + .btn-primary / .btn-ghost / .btn-sm
├── stores/app.ts                唯一 Pinia store：folders / connections / activeConnectionId / connectionStatus / groupedConnections / loadAll / checkConnection ...
├── utils/ddl.ts                 前端 DDL 构建：buildCreateTableSQL / buildAlterTableSQL / buildDropTableSQL + CLICKHOUSE_ENGINES(26) + CLICKHOUSE_TYPES(42)
├── utils/tabs.ts                QueryTab 类型与工厂：每个标签独立持有 sql / result / error / querying / queryId / canceled / viewState / resultScrollTop
├── utils/resultTable.ts         结果表格虚拟滚动纯函数：ROW_HEIGHT / VIRTUAL_THRESHOLD / measureColWidths / shouldVirtualize
├── utils/sqlLiteral.ts          SQL 字面量与复制纯函数：escapeSqlString / sqlLiteral / quoteIdent / rowToTSV / buildInsertStatement
├── router/index.ts              仅 1 条路由：/ → /query，懒加载 QueryEditor
├── locales/zh.ts, en.ts         i18n key：common / menu / editor（建表/编辑表相关 key 都在 editor.*）
├── components/
│   ├── MenuBar.vue              顶栏：连接选择器、语言切换、菜单
│   ├── ConnectionModal.vue      新建/编辑连接（Native/HTTP/TLS/压缩/测试）
│   ├── FolderModal.vue          新建/编辑分组
│   ├── CreateTableModal.vue     可视化建表（列类型=select，引擎=select，SQL Preview）
│   └── EditTableModal.vue       可视化编辑表（加载 schema → MOD/REN/DROP/ADD → ALTER 执行）
└── views/
    └── QueryEditor.vue ★       唯一视图。核心：Monaco SQL 编辑器（多标签，editor.api.js 精简入口动态 import）+ 数据库树（表/视图） + 集群面板 + 结果表（虚拟滚动 / 右键复制 / 导出 CSV·JSON·Markdown / 截断提示） + 历史面板 + 右键菜单 + 所有 Modal 宿主
```

### QueryEditor.vue — 核心视图承载清单

它管理所有 Modal 的可见性状态：
```ts
createModal = { visible, database }          → CreateTableModal
editModal   = { visible, database, table }  → EditTableModal
ddlModal    = { visible, sql }               → 查看 DDL 写入编辑器
dropConfirm = { visible, db, table }         → 删除表确认
```

所有右键菜单也在 QueryEditor 里渲染（数据库菜单 / 表菜单），触发函数：
- `openCreateTable(db)` — 悬停 + 按钮 / 右键数据库 → 建表
- `openEditTable(db, table)` — 右键表 → 编辑
- `openTableDDL(db, table)` — 右键表 → SHOW CREATE
- `confirmDropTable(db, table)` — 右键表 → 删除确认

### Tailwind 自定义色板（`frontend/tailwind.config.js`）

| 类 | 值 | 用途 |
|---|---|---|
| `.ch-bg` | `#1a1b1e` | 页面背景 |
| `.ch-panel` | `#25262b` | 面板/卡片 |
| `.ch-border` | `#3a3d45` | 边框 / 分隔线 / input border |
| `.ch-text` | `#d4d4d8` | 主文字 |
| `.ch-muted` | `#71717a` | 次要文字 / 标签 |
| `.ch-accent` | `#e74c3c` | 强调色（红）：按钮 / focus border / checkbox |
| `.ch-hover` | `#2e2f36` | 悬停背景 |
| `.ch-active` | `#3a3d45` | 选中背景 |

全局工具类（style.css）：`.btn-primary` / `.btn-ghost` / `.btn-sm`

### 交互 / 布局约定

- 全局 root 用 `<Teleport to="body">` 渲染 Modal（z-index `2000` 或 `3000`）
- 数据库树外层需要 `group` 类才能让内层 `group-hover:opacity-100` 的按钮显示
- 表名 / 视图名行点击 → 直接用 `SELECT * ... LIMIT 10` **预览数据**（右下角红底高亮标识选中项）；右键 → 菜单
- SQL 编辑器快捷键：`Ctrl/Cmd + Enter` 执行选中/全部
- 侧边栏宽度可拖拽，状态存 Pinia
- 所有 Modal 关闭动画靠 `v-if + @click.self + Esc`

---

## 关键数据结构（frontend/src/types.ts）

```ts
Connection     { id, folder_id?, name, host, port, protocol:'native'|'http', user, password, database, secure, use_compress }
Folder         { id, name }
DataFile       { folders, connections }
ColumnInfo     { name, type, default?, comment? }
TableInfo      { name, engine?, comment? }
ClusterInfo    { cluster, version, host_name, host_address, port, is_local, shard_num, replica_num, uuid }
QueryResult    { columns: ColumnInfo[], rows: any[][], row_count, elapsed_ms, truncated, row_limit, error? }
ExecResult     { rows_affected, error? }
```

---

## DDL 工具（frontend/src/utils/ddl.ts）

前端生成 SQL，后端只执行。好处：实时 Preview 让用户确认。

| 导出 | 类型/值 | 说明 |
|---|---|---|
| `CLICKHOUSE_ENGINES` | `string[26]` | MergeTree 家族 + Memory/Log/TinyLog + Kafka/S3/MySQL/HDFS + MaterializedView + Distributed |
| `CLICKHOUSE_TYPES` | `string[42]` | Int/UInt/Float/Decimal/Boolean/Date/DateTime/UUID/IPv4/IPv6/JSON/Object/Array/Map/LowCardinality/Nullable/FixedString/Enum |
| `buildCreateTableSQL(def)` | 函数 | 表名/注释/引擎/ORDER BY/PARTITION BY/PRIMARY KEY/SETTINGS/列定义 → CREATE TABLE |
| `buildAlterTableSQL(db, table, changes)` | 函数 | `EditTableChange[]` → `ALTER TABLE ...` 多条语句 |
| `buildDropTableSQL(db, table)` | 函数 | `DROP TABLE IF EXISTS db.table` |

`EditTableChange` 类型：
```ts
{ type:'add'|'modify'|'drop'|'rename', column, newName?, newType?, newDefault?, newComment?, position?:{first?, after?} }
```

---

## 配置文件

配置存 JSON，路径按 OS：

| 系统 | 目录 |
|---|---|
| Windows | `%AppData%\meteor-ch-workbench\` |
| macOS | `~/Library/Application Support/meteor-ch-workbench/` |
| Linux | `~/.config/meteor-ch-workbench/` |

该目录下有三个文件：

| 文件 | 结构 | 说明 |
|---|---|---|
| `connections.json` | `{ folders: Folder[], connections: Connection[] }` | 连接与分组，**密码明文存储** |
| `history.json` | `{ entries: HistoryEntry[] }` | 查询历史，上限 200 |
| `tabs.json` | `{ tabs: [...], activeId }` | 标签页快照（仅标题 + SQL） |

---

## 测试

后端单元测试覆盖 `internal/config` 与 `internal/clickhouse`：

| 文件 | 覆盖内容 |
|---|---|
| `internal/config/export_test.go` | 5 种导出格式的渲染、脱敏、转义、往返一致性 |
| `internal/config/import_test.go` | 4 种解析器边界（DBeaver / CSV 缺列 / DSN 非法行）+ 合并策略（同名复用、ID 冲突重分配、folder_id 重映射） |
| `internal/config/history_test.go` | 历史记录、上限裁剪、删除与清空 |
| `internal/config/tabs_test.go` | 标签快照读写与容错 |
| `internal/clickhouse/export_test.go` | 结果集导出渲染（CSV / JSON / Markdown，含竖线、换行、CRLF、反斜杠转义边界） |
| `internal/clickhouse/cancel_test.go` | 查询取消：空 ID / 未注册 / 正常取消 / 重复取消 / 注销清理 / 重复注册取消旧 / 超时不误判 |

---

## 开发命令

```bash
# 类型检查
cd frontend && npx vue-tsc --noEmit

# 前端构建（改完 src 必须执行，Wails 加载的是 dist）
cd frontend && npm run build

# 后端编译
go build ./...

# 后端测试
go test ./...

# 完整 Wails 开发 (v3, CLI 命令待定)
# wails dev / 或 go run 启动后端 + vite 启动前端
```

---

## 文档索引

| 文件 | 内容 |
|---|---|
| `README.md` | 项目首页 |
| `docs/API.md` | 完整 Go → TS 接口文档 |
| `docs/FEATURES.md` | 用户视角的功能说明 |
| `docs/CODE_INDEX.md` | 代码索引：逐文件职责、关键符号、依赖方向、改动路径速查 |
| `.trae/PROJECT.md`（本文件） | AI 快速检索地图 |
| `.trae/rules/roadmap.md` | 已完成能力 + P1–P6 后续计划（含方案与验收） |

---

## 快速上手：我想改 X，应该看哪？

| 想做的事 | 改哪些文件 |
|---|---|
| 加一个后端 API | `app.go` 加公开方法 → `internal/clickhouse/service.go` 或 `config/manager.go` 实现 → `frontend/src/api.ts` 加封装 |
| 加一个新的 Modal 弹窗 | `components/` 下建 .vue → 在目标视图（通常 `QueryEditor.vue`）加 visible state + 引入 |
| 加新 i18n key | `locales/zh.ts` 和 `en.ts` 同步加，key 前缀遵循现有分组（common/menu/editor） |
| 改 DDL 生成逻辑 | `utils/ddl.ts` |
| 加新的表类型/引擎选项 | `utils/ddl.ts` 里的 `CLICKHOUSE_TYPES` / `CLICKHOUSE_ENGINES` 数组 |
| 改连接表单字段 | `types.ts` Connection 接口 → `config/manager.go` JSON 序列化 → `components/ConnectionModal.vue` |
| 改 SQL 执行错误处理 | `internal/clickhouse/service.go`（`.error` 字段约定）→ 前端从 `QueryResult.error` 读 |
| 改 UI 颜色/样式 | `tailwind.config.js` 色板 / `style.css` 全局工具类 |
| 改侧边栏树结构 | `views/QueryEditor.vue` 中 `db` / `table` 的 v-for 区域 |
| 加一种配置导出格式 | `internal/config/export.go` 注册表追加一项（渲染函数） → `export_test.go` 补测试 |
| 加一种配置导入格式 | `internal/config/import.go` 注册表追加一项（解析函数 + 自动识别） → `import_test.go` 补测试 |
| 加一种结果集导出格式 | `internal/clickhouse/export.go` 注册表追加一项 → `export_test.go` 补测试 |
| 改查询行数上限 | `internal/clickhouse/export.go` 的 `DefaultQueryRowLimit` |
| 改历史保留条数 | `internal/config/history.go` 的 `HistoryMaxEntries` |
| 改标签页结构 | `frontend/src/utils/tabs.ts`（类型）→ `internal/config/tabs.go`（持久化） |
| 改合并导入策略 | `internal/config/import.go` 的 `mergeData` → `import_test.go` 对应测试 |
| 加单元测试 | 同目录 `*_test.go`，表驱动风格，覆盖正常 + 边界 + 往返一致性 |