# API Reference

后端 Go 方法通过 Wails v3 自动绑定为前端可调用函数。

调用格式：
```ts
import { Call } from '@wailsio/runtime'
await Call.ByName('main.App.<方法名>', ...args)
```

全部方法已在 `frontend/src/api.ts` 中统一封装，组件层直接：
```ts
import { api } from '../api'
await api.GetAll()
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
断开指定连接的连接池，下次查询自动重新建连。

| | |
|---|---|
| Go | `func (a *App) Disconnect(connectionID string)` |
| TS | `api.Disconnect(connectionID: string): Promise<void>` |

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

## 3. SQL 执行

### Query(connectionID, sql)
执行 SELECT / SHOW 等查询语句，返回结果表格。

| | |
|---|---|
| Go | `func (a *App) Query(connectionID, sql string) (*clickhouse.QueryResult, error)` |
| TS | `api.Query(connectionID: string, sql: string): Promise<QueryResult>` |

QueryResult 结构：
```ts
interface QueryResult {
  columns: string[]     // 列名列表
  rows: any[][]         // 数据行
  row_count: number     // 总行数
  elapsed_ms: number    // 执行耗时
  error?: string        // SQL 错误（如语法错），放在这里而不是 Go error
}
```

> **设计说明**：SQL 执行失败时 Go 返回的 `error` 为 nil，错误信息放在 `QueryResult.error` 里。这样前端能正常渲染结果面板并在面板内展示错误文本，而不是当作网络级异常处理。

---

### Execute(connectionID, sql)
执行 DDL / DML（CREATE / ALTER / DROP / INSERT 等）。

| | |
|---|---|
| Go | `func (a *App) Execute(connectionID, sql string) (*clickhouse.ExecResult, error)` |
| TS | `api.Execute(connectionID: string, sql: string): Promise<ExecResult>` |

ExecResult 结构：
```ts
interface ExecResult {
  rows_affected: number
  error?: string
}
```

> 同样，SQL 错误放在 `ExecResult.error` 而非 Go error。可视化建表 / 编辑表 / 删除表均使用此接口。

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
}
```

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

前端 `frontend/src/utils/ddl.ts` 提供纯 TypeScript 的 SQL 构建函数，所有 DDL 语句由前端生成、后端只负责执行：

| 函数 | 用途 |
|---|---|
| `buildCreateTableSQL(def)` | 生成 `CREATE TABLE ... ENGINE=... ORDER BY ...` |
| `buildAlterTableSQL(db, table, changes[])` | 根据变更列表生成一条或多条 `ALTER TABLE` |
| `buildDropTableSQL(db, table)` | 生成 `DROP TABLE IF EXISTS` |
| `CLICKHOUSE_ENGINES` | 26 种常用引擎常量数组 |
| `CLICKHOUSE_TYPES` | 42 种常用列类型常量数组（供 select 下拉） |

设计理由：DDL 构造放前端可以实时生成「SQL 预览」让用户确认，同时保持后端 API 极简——只需一个 `Execute` 即可覆盖所有 DDL 场景。