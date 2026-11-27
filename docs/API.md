# API Reference

后端 `internal/app.App` 暴露的公开方法。**纯 Go API**，无前端绑定层。
Fyne GUI 层通过 `fyneapp.Backend()` 全局单例直接调用：

```go
app := Backend()
result, err := app.Query(connID, sql, queryID)
```

---

## 1. 连接管理

### GetAll()
一次性读取全部配置数据（文件夹 + 连接列表）。

| | |
|---|---|
| Go | `func (a *App) GetAll() (*config.DataFile, error)` |
| TS | `api.GetAll(): Promise<DataFile>` |
| 返回 | `{ folders: Folder[], connections: Connection[] }` |

---

### GetConnections()
获取所有已保存的连接配置。

| | |
|---|---|
| Go | `func (a *App) GetConnections() ([]config.Connection, error)` |
| TS | `api.GetConnections(): Promise<Connection[]>` |

---

### GetConnection(id)
按 ID 读取单条连接配置。

| | |
|---|---|
| Go | `func (a *App) GetConnection(id string) (*config.Connection, error)` |
| TS | `api.GetConnection(id: string): Promise<Connection>` |
| 参数 | `id` — 连接 ID |

---

### SaveConnection(conn)
保存（新增或更新）一条连接。保存后旧连接池自动失效，下次使用新参数建连。

| | |
|---|---|
| Go | `func (a *App) SaveConnection(conn config.Connection) error` |
| TS | `api.SaveConnection(conn: Connection): Promise<void>` |

Connection 结构：
```ts
interface Connection {
  id: string
  folder_id?: string
  name: string
  host: string
  port: number
  protocol: 'native' | 'http'
  user: string
  password: string
  database: string
  secure: boolean
  use_compress: boolean
}
```

---

### DeleteConnection(id)
删除连接配置，同步释放其连接池。

| | |
|---|---|
| Go | `func (a *App) DeleteConnection(id string) error` |
| TS | `api.DeleteConnection(id: string): Promise<void>` |

---

### TestConnection(conn)
用传入参数尝试建连并 Ping，用于「测试连接」按钮。

| | |
|---|---|
| Go | `func (a *App) TestConnection(conn config.Connection) error` |
| TS | `api.TestConnection(conn: Connection): Promise<void>` |
| 失败 | 返回 error，消息直接展示给用户 |

---

### Disconnect(connectionID)
断开指定连接：先取消该连接下全部在途查询（`cancelByConn`），再关闭并移除连接池，下次查询自动重新建连。

| | |
|---|---|
| Go | `func (a *App) Disconnect(connectionID string)` |
| TS | `api.Disconnect(connectionID: string): Promise<void>` |

> 断开后仍在途的查询会立即以 `QueryResult.error = "查询已取消"` 返回，而非驱动层原始报错。前端若收到 `connection closed` / `EOF` 类错误，应提示「连接已断开」并提供重连入口。

---

## 2. 分组管理

### GetFolders()
获取所有连接分组。

| | |
|---|---|
| Go | `func (a *App) GetFolders() ([]config.Folder, error)` |
| TS | `api.GetFolders(): Promise<Folder[]>` |

Folder 结构：
```ts
interface Folder {
  id: string
  name: string
}
```

---

### SaveFolder(folder)
新增或更新分组。

| | |
|---|---|
| Go | `func (a *App) SaveFolder(f config.Folder) error` |
| TS | `api.SaveFolder(folder: Folder): Promise<void>` |

---

### DeleteFolder(id)
删除分组，其下连接自动移到根级。

| | |
|---|---|
| Go | `func (a *App) DeleteFolder(id string) error` |
| TS | `api.DeleteFolder(id: string): Promise<void>` |

---

### ReorderFolders(ids)
按给定顺序持久化分组顺序（拖拽排序后调用）。

| | |
|---|---|
| Go | `func (a *App) ReorderFolders(ids []string) error` |
| TS | `api.ReorderFolders(ids: string[]): Promise<void>` |
| 参数 | `ids` — 新的分组 ID 顺序数组 |

---

### ReorderConnections(ids)
按给定顺序持久化连接顺序（组内 / 跨组拖拽后调用）。

| | |
|---|---|
| Go | `func (a *App) ReorderConnections(ids []string) error` |
| TS | `api.ReorderConnections(ids: string[]): Promise<void>` |
| 参数 | `ids` — 新的连接 ID 顺序数组 |

---

## 2.1 配置导入导出

### ExportConfig()
导出完整配置为 JSON 文本（含密码）。

| | |
|---|---|
| Go | `func (a *App) ExportConfig() (string, error)` |
| TS | `api.ExportConfig(): Promise<string>` |

---

### GetExportFormats()
返回支持的导出格式列表，前端据此动态渲染格式选择。

| | |
|---|---|
| Go | `func (a *App) GetExportFormats() []config.ExportFormat` |
| TS | `api.GetExportFormats(): Promise<ExportFormat[]>` |
| 返回 | `ExportFormat{ id, name, extension, mime, description }[]` |

当前支持：`json` / `yaml` / `csv` / `dsn` / `dbeaver`。

---

### ExportConfigByFormat(format, includePassword)
按指定格式导出配置文本，第二个返回值为建议扩展名。

| | |
|---|---|
| Go | `func (a *App) ExportConfigByFormat(format string, includePassword bool) (string, string, error)` |
| TS | `api.ExportConfigByFormat(format: string, includePassword: bool): Promise<[string, string]>` |
| 参数 | `includePassword` — false 时清空所有密码（脱敏） |

> **注意**：脱敏只影响导出内容，不会修改内存中的真实配置（`sanitize` 做了深拷贝）。

---

### SaveExportAs(format, includePassword)
弹出系统「另存为」对话框，把配置写入用户选择的路径。

| | |
|---|---|
| Go | `func (a *App) SaveExportAs(format string, includePassword bool) (string, error)` |
| TS | `api.SaveExportAs(format: string, includePassword: bool): Promise<string>` |
| 返回 | 实际写入的文件路径（用户取消时为空） |

---

### ImportConfig(raw)
导入 JSON 配置并覆盖当前全部连接与分组。

| | |
|---|---|
| Go | `func (a *App) ImportConfig(raw string) error` |
| TS | `api.ImportConfig(raw: string): Promise<void>` |

---

### GetImportFormats()
返回支持的导入格式列表。

| | |
|---|---|
| Go | `func (a *App) GetImportFormats() []config.ImportFormat` |
| TS | `api.GetImportFormats(): Promise<ImportFormat[]>` |

当前支持：`json`（含 DBeaver 结构）/ `yaml` / `csv` / `dsn`。

---

### PickImportFile()
弹出系统「打开文件」对话框，返回用户选中文件的**内容**与文件名。

| | |
|---|---|
| Go | `func (a *App) PickImportFile() (string, string, error)` |
| TS | `api.PickImportFile(): Promise<[string, string]>` |
| 返回 | `[文件内容, 文件路径]`（用户取消时均为空） |

---

### ImportConfigWithOptions(raw, opts)
按指定格式与模式导入。`format` 为空时自动识别；`mode` 为 `replace`（覆盖，默认）或 `merge`（合并）。

| | |
|---|---|
| Go | `func (a *App) ImportConfigWithOptions(raw string, opts config.ImportOptions) (config.ImportResult, error)` |
| TS | `api.ImportConfigWithOptions(raw: string, opts: ImportOptions): Promise<ImportResult>` |

ImportResult 结构：
```ts
interface ImportResult {
  mode: 'replace' | 'merge'
  format: string            // 实际识别/使用的格式
  importedFolders: number   // 本次导入的分组数
  importedConns: number     // 本次导入的连接数
  totalFolders: number      // 导入后的分组总数
  totalConns: number        // 导入后的连接总数
}
```

> **合并模式规则**：同名分组复用已有分组（不重复创建）；分组 ID 冲突时重新生成并把引用它的连接 `folder_id` 一并重映射；连接 ID 冲突时重新生成。指向不存在分组的连接在导入后归到根级。

---

## 3. SQL 执行

### Query(connectionID, sql, queryID)
执行 SELECT / SHOW 等查询语句，返回结果表格。

| | |
|---|---|
| Go | `func (a *App) Query(connectionID, sql, queryID string) (*clickhouse.QueryResult, error)` |
| TS | `api.Query(connectionID: string, sql: string, queryID: string): Promise<QueryResult>` |

`queryID` 由前端生成（`crypto.randomUUID()`），用于支持主动取消。传空字符串表示不可取消。

QueryResult 结构：
```ts
interface QueryResult {
  columns: ColumnInfo[] // 列信息列表，每项含 name / type / comment / is_primary / has_default
  rows: any[][] | null  // 数据行；0 行时后端返回 []（旧版本可能为 null，前端按可空处理）
  row_count: number     // 总行数
  elapsed_ms: number    // 执行耗时
  truncated: boolean    // 是否因超过行数上限被截断
  row_limit: number     // 本次生效的行数上限
  error?: string        // SQL 错误（如语法错），放在这里而不是 Go error
}
```

> **设计说明**：SQL 执行失败时 Go 返回的 `error` 为 nil，错误信息放在 `QueryResult.error` 里。这样前端能正常渲染结果面板并在面板内展示错误文本，而不是当作网络级异常处理。

> **大结果保护**：`Query` 的行数上限取自 `settings.json` 的 `query_row_limit`（见 `GetSettings` / `SaveSettings`），未配置时退回 `DefaultQueryRowLimit = 50000`；`query_row_limit = 0` 表示不限制。超出上限的部分被丢弃并置 `truncated = true`，前端在结果面板提示用户。这样避免一次性把巨量数据传给前端导致界面卡死。

> **实现要点**：服务层提供 `QueryWithLimit` 接收显式上限，`Query` 委托给它并传 `DefaultQueryRowLimit` 保持向后兼容；`rowLimit <= 0` 一律退回默认值（「不限制」仍由 `scanRows` 的 `limit = 0` 语义表达，不会误判为默认）。

> **取消行为**：查询执行期间调用 `CancelQuery(queryID)`，本次查询立即返回，`QueryResult.error = "查询已取消"`，已读取的部分行被丢弃（`rows` 为空）。前端据此用灰色提示条而非红色错误条展示。

---

### CancelQuery(queryID)
取消指定 queryID 正在执行的查询。

| | |
|---|---|
| Go | `func (a *App) CancelQuery(queryID string) bool` |
| TS | `api.CancelQuery(queryID: string): Promise<boolean>` |

返回值：`true` 表示该查询确实正在运行并已被取消；`false` 表示查询已结束或 ID 不存在，前端应静默忽略。

> **实现要点**：后端 `Service` 维护 `running map[string]context.CancelFunc`。`Query` 开始时注册可取消 context，结束时（含 defer）注销。取消时调用 `cancel()`，驱动层 `QueryContext` 与逐行扫描都会立即返回。前端关闭标签页时若查询仍在运行，会自动调用本方法，避免 context 泄漏。

---

### Execute

| | |
|---|---|
| Go | `func (a *App) Execute(connectionID, sql string) (*clickhouse.ExecResult, error)` |
| TS | `api.Execute(connectionID: string, sql: string): Promise<ExecResult>` |

执行 DDL / DML（CREATE / ALTER / DROP / INSERT 等）。

**mutation 同步**：若语句是 ClickHouse mutation（`ALTER TABLE ... UPDATE` / `ALTER TABLE ... DELETE` / `DELETE FROM`），后端自动追加 `SETTINGS mutations_sync = 2`，等 mutation 在全部副本完成后才返回，保证紧接着的查询能读到新数据；语句本身已显式包含 `mutations_sync` 时不覆盖。其余语句原样执行。判定逻辑见 `ensureMutationSync`。

ExecResult 结构：
```ts
interface ExecResult {
  rows_affected: number
  error?: string
}
```

> 同样，SQL 错误放在 `ExecResult.error` 而非 Go error。可视化建表 / 编辑表 / 删除表、结果单元格「应用修改」均使用此接口。

---

## 3.1 结果集导出

### GetResultExportFormats()
返回结果集支持的导出格式。

| | |
|---|---|
| Go | `func (a *App) GetResultExportFormats() []clickhouse.ResultExportFormat` |
| TS | `api.GetResultExportFormats(): Promise<ResultExportFormat[]>` |

当前支持 8 种格式（注册表模式，新增格式只需在 `internal/clickhouse/export.go` 追加一项，前端自动感知）：

| id | 名称 | 扩展名 | 说明 | 用到 `target` |
|---|---|---|---|---|
| `csv` | CSV | `.csv` | 通用表格格式，首行列名 | 否 |
| `json` | JSON | `.json` | 整表数组，便于程序读取 | 否 |
| `markdown` | Markdown | `.md` | 直接贴进文档的表格 | 否 |
| `insert` | SQL INSERT | `.sql` | 单条 `INSERT INTO ... VALUES (...), (...);` | 是 |
| `jsonl` | JSON Lines | `.jsonl` | 每行一个 JSON 对象，流式、可被 jq / 数据管道逐行消费 | 否 |
| `tsv` | TSV | `.tsv` | 制表符分隔，ClickHouse 原生格式，粘贴到表格工具不易串列 | 否 |
| `xlsx` | Excel | `.xlsx` | 真正的 Excel 工作簿：表头加粗冻结、自动筛选、列宽自适应 | 否 |
| `sql_ddl` | SQL DDL + INSERT | `.sql` | 先 `CREATE TABLE` 再插入，可在新库一键建表回灌 | 是 |

`insert` 与 `sql_ddl` 需要导出时提供目标表名（见下一节的 `target` 参数）。`insert` 渲染为单条 `INSERT INTO ... VALUES (...), (...);`；`sql_ddl` 在其前面追加 `CREATE TABLE IF NOT EXISTS`，列类型按该列首个非空值推断（整数 → `Int64`、浮点 → `Float64`、布尔 → `UInt8`、其余 → `String`，整列为空回落 `String`）。

字符串字面量转义规则见 `clickhouse/export.go` 的 `formatLiteral` 函数（反斜杠双写、单引号转义为 `'\'`、布尔转 `1/0`、非有限浮点转 `NULL`）。

其余格式均忽略 `target`：

- **`jsonl`**：每行一个对象，缺列补 `null`，单元格内的换行与引号由 `encoding/json` 转义，保证「一行一记录」。
- **`tsv`**：单元格内的 `\`、`\t`、`\r\n` 按 ClickHouse 原生 TSV 约定转义为 `\\`、`\t`、`\n`（反斜杠最先双写），不会破坏列结构。
- **`xlsx`**：单工作表 `Result`；数值列写入数字类型（Excel 可直接求和），布尔写 `TRUE/FALSE`，非有限浮点（NaN / Inf）转文本；超过 32767 字符的单元格按字符截断并追加 `...`；列宽按内容宽度估算（CJK 按双宽计），夹在 8–60 之间。由于 xlsx 是 zip 容器，内部先构建工作簿再整体写出，不适用「逐行写」的流式优化，超大结果集导出时内存占用高于文本格式。

---

### SaveQueryResultAs(format, columns, rows, target)
把当前结果集按指定格式写入用户选择的文件。

| | |
|---|---|
| Go | `func (a *App) SaveQueryResultAs(format string, columns []string, rows [][]interface{}, target string) (string, error)` |
| TS | `api.SaveQueryResultAs(format: string, columns: string[], rows: any[][], target?: string): Promise<string>` |
| 返回 | 实际写入的文件路径（用户取消时为空） |

`target` 仅 `insert` / `sql_ddl` 格式使用（目标表名，可为 `db.table` 或带反引号的完整名，必填，为空时报错）；`sql_ddl` 未指定表名时前端会回退到占位符 `your_table`。其余格式传空字符串即可。

> 由前端把已渲染的结果数据传回后端渲染落盘，不重新查库，因此导出的就是用户眼前这份数据（包括被截断的部分）。
>
> **流式落盘**：后端先弹文件对话框，再用 `os.Create` 打开文件，然后通过 `clickhouse.ExportResultTo(io.Writer, ...)` 逐行写出，不在内存中构造完整字符串，因此大结果导出不再产生「行数据 + 完整文本」双份内存占用。旧的 `ExportResultByFormat` 保留为基于 `bytes.Buffer` 的适配层，与 `ExportResultTo` 输出逐字节一致（`export_test.go` 有对应一致性与写入失败用例）。

---

## 3.2 查询历史

### GetQueryHistory()
读取查询历史（新的在前）。

| | |
|---|---|
| Go | `func (a *App) GetQueryHistory() ([]config.HistoryEntry, error)` |
| TS | `api.GetQueryHistory(): Promise<HistoryEntry[]>` |

HistoryEntry 结构：
```ts
interface HistoryEntry {
  id: string
  sql: string
  connectionId: string
  timestamp: string    // RFC3339
  duration: number     // 毫秒
  success: boolean
}
```

> **记录时机**：`Query` 执行结束后自动记录（成功或 SQL 错误都会记，`success` 区分）。最多保留 `HistoryMaxEntries = 200` 条，超出丢弃最旧的，存于 `history.json`。

---

### DeleteQueryHistory(id)
删除单条历史。

| | |
|---|---|
| Go | `func (a *App) DeleteQueryHistory(id string) error` |
| TS | `api.DeleteQueryHistory(id: string): Promise<void>` |

---

### ClearQueryHistory()
清空全部历史。

| | |
|---|---|
| Go | `func (a *App) ClearQueryHistory() error` |
| TS | `api.ClearQueryHistory(): Promise<void>` |

> 只清空查询历史（`history.json`），**不影响**已收藏的 SQL（`saved_sql.json`）。

---

### GetSavedSQL()
返回全部手动收藏的 SQL，最近更新的排在最前。

| | |
|---|---|
| Go | `func (a *App) GetSavedSQL() ([]config.SavedSQL, error)` |
| TS | `api.GetSavedSQL(): Promise<SavedSQL[]>` |

```ts
interface SavedSQL {
  id: string
  name: string          // 标题；留空时后端取 SQL 首个非空行
  sql: string
  connection_id: string
  created_at: string
  updated_at: string
}
```

> 存于 `saved_sql.json`。与查询历史语义分离：收藏**无条数上限**，且不随「清空历史」删除。

---

### SaveSQL(name, sql, connectionId)
收藏一条 SQL，返回新建记录。

| | |
|---|---|
| Go | `func (a *App) SaveSQL(name string, sql string, connectionID string) (*config.SavedSQL, error)` |
| TS | `api.SaveSQL(name: string, sql: string, connectionID: string): Promise<SavedSQL>` |

`sql` 为空或与已有收藏（忽略首尾空白）重复时返回 error，文案分别含 `saved sql is empty` / `saved sql already exists`，前端据此提示。

---

### UpdateSavedSQL(id, name, sql)
修改收藏的标题与 SQL 文本，`created_at` 保持不变，`updated_at` 刷新。

| | |
|---|---|
| Go | `func (a *App) UpdateSavedSQL(id string, name string, sql string) (*config.SavedSQL, error)` |
| TS | `api.UpdateSavedSQL(id: string, name: string, sql: string): Promise<SavedSQL>` |

ID 不存在返回 `saved sql not found`；`sql` 为空同样报错。

---

### DeleteSavedSQL(id)
删除指定 ID 的收藏记录；ID 不存在返回 `saved sql not found`。

| | |
|---|---|
| Go | `func (a *App) DeleteSavedSQL(id string) error` |
| TS | `api.DeleteSavedSQL(id: string): Promise<void>` |

---

## 3.3 标签页持久化

### GetTabs()
读取上次退出时的标签页快照（标题 / SQL / 活动标签）。

| | |
|---|---|
| Go | `func (a *App) GetTabs() (config.TabsFile, error)` |
| TS | `api.GetTabs(): Promise<TabsFile>` |

TabsFile 结构：
```ts
interface TabsFile {
  tabs: { id: string, title: string, sql: string }[]
  activeId: string
}
```

---

### SaveTabs(snapshot)
保存标签页快照，前端在标签增删、重命名、切换、SQL 变更（防抖）时调用。

| | |
|---|---|
| Go | `func (a *App) SaveTabs(snapshot config.TabsFile) error` |
| TS | `api.SaveTabs(snapshot: TabsFile): Promise<void>` |

> 存于 `tabs.json`。只持久化标题和 SQL，**不持久化查询结果**——重启后标签内容在，结果为空，需要重新执行。

---

### GetSettings()
返回当前用户偏好设置，供设置面板回显。

| | |
|---|---|
| Go | `func (a *App) GetSettings() (config.SettingsFile, error)` |
| TS | `api.GetSettings(): Promise<SettingsFile>` |

### SaveSettings(settings)
校验并保存用户偏好设置，返回保存后实际生效的值。

| | |
|---|---|
| Go | `func (a *App) SaveSettings(settings config.SettingsFile) (config.SettingsFile, error)` |
| TS | `api.SaveSettings(settings: SettingsFile): Promise<SettingsFile>` |

```ts
interface SettingsFile {
  query_row_limit: number   // 0 表示不限制
}
```

> 存于 `settings.json`，采用「内存缓存 + `sync.RWMutex` + 全量写回」。写入前经 `NormalizeRowLimit` 收敛：负数或小于 `RowLimitMin = 100` 退回默认 50000，超过 `RowLimitMax = 1000000` 截断到上限，`0` 原样保留表示不限制。文件损坏或缺失时 `RowLimit()` 静默退回默认值，保证查询不会因设置文件异常而失败。

---

## 4. 元数据浏览

### GetDatabases(connectionID)
获取数据库列表（`SHOW DATABASES`）。

| | |
|---|---|
| Go | `func (a *App) GetDatabases(connectionID string) ([]string, error)` |
| TS | `api.GetDatabases(connectionID: string): Promise<string[]>` |

---

### GetTables(connectionID, database)
获取指定数据库下的表列表（不含视图）。

| | |
|---|---|
| Go | `func (a *App) GetTables(connectionID, database string) ([]clickhouse.TableInfo, error)` |
| TS | `api.GetTables(connectionID: string, database: string): Promise<TableInfo[]>` |

TableInfo 结构：
```ts
interface TableInfo {
  name: string
  engine?: string
  comment?: string
}
```

> 实现按 `system.tables` 中引擎 `engine NOT LIKE '%View'` 过滤，只返回真实表，视图由 `GetViews` 单独返回，避免重复。

---

### GetViews(connectionID, database)
获取指定数据库下的所有视图信息（普通 `View`、`MaterializedView`、`LiveView` 等，按引擎以 `View` 结尾匹配）。

| | |
|---|---|
| Go | `func (a *App) GetViews(connectionID, database string) ([]clickhouse.TableInfo, error)` |
| TS | `api.GetViews(connectionID: string, database: string): Promise<TableInfo[]>` |

返回结构与 `TableInfo` 相同（`name / engine / comment`），前端据此在左侧导航中单独渲染「视图」分区。

---

### GetTableSchema(connectionID, database, table)
获取指定表的列结构（用于「编辑表」Modal）。

| | |
|---|---|
| Go | `func (a *App) GetTableSchema(connectionID, database, table string) ([]clickhouse.ColumnInfo, error)` |
| TS | `api.GetTableSchema(connectionID: string, database: string, table: string): Promise<ColumnInfo[]>` |

ColumnInfo 结构：
```ts
interface ColumnInfo {
  name: string
  type: string
  default?: string
  comment?: string
  is_primary?: boolean   // 是否属于主键（DESCRIBE 的 primary_key = 1）
  has_default?: boolean  // 是否有默认值表达式（DESCRIBE 的 default_type 非空）
}
```

> `is_primary` 用于结果单元格编辑时自动选取 WHERE 依据列（前端 `detectKeyColumns` 优先使用主键）。

---

### GetClusters(connectionID)
查询 `system.clusters` 获取集群拓扑。

| | |
|---|---|
| Go | `func (a *App) GetClusters(connectionID string) ([]clickhouse.ClusterInfo, error)` |
| TS | `api.GetClusters(connectionID: string): Promise<ClusterInfo[]>` |

ClusterInfo 结构：
```ts
interface ClusterInfo {
  cluster: string
  version: string
  host_name: string
  host_address: string
  port: number
  is_local: boolean
  shard_num: number
  replica_num: number
  uuid: string
}
```

---

## 5. 其他

### OpenConfigFolder()
用系统文件管理器打开配置文件目录。

| | |
|---|---|
| Go | `func (a *App) OpenConfigFolder() error` |
| TS | `api.OpenConfigFolder(): Promise<void>` |

---

### GetAppVersion()
返回应用版本号。

| | |
|---|---|
| Go | `func (a *App) GetAppVersion() string` |
| TS | `api.GetAppVersion(): Promise<string>` |

---

## DDL SQL 构建工具

DDL 语句（CREATE / ALTER / DROP TABLE）由 Fyne GUI 层直接构造 SQL 字符串，后端 `Execute` 只负责执行。这种设计保持后端 API 极简，同时让 UI 层可以实时预览 SQL 让用户确认后再执行。