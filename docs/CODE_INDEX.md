# 代码索引

> 按文件列出职责、关键入口与依赖关系，便于人和 AI 快速定位。
> 与 `.trae/PROJECT.md`（AI 检索地图）互补：本文更偏「文件清单」，PROJECT.md 更偏「怎么改」。
> 更新时间：2026-11-23

---

## 仓库总览

```
meteor-ch-workbench/
├── main.go              Wails 应用入口（兼容基线）
├── cmd/meteor-ch-gio/   Gio 原生 GUI 独立入口
│   └── main.go          Gio 窗口启动与事件循环入口
├── app.go               所有前端可调用方法的门面（43 个，含 2 个生命周期回调）
├── wails.json           Wails 打包配置
├── go.mod / go.sum      Go 依赖
├── internal/
│   ├── gui/gioapp/      Gio SQL 工作台主窗口原型
│   ├── app_platform_test.go 应用入口平台文件交互行为测试（仓库根目录）
│   ├── app/             GUI 无关的应用服务编排（连接 / 查询 / 配置 / 历史 / 收藏 / 标签 / 设置 / 结果导出）
│   ├── platform/        文件对话框与本地文件系统能力端口（Wails 入口使用）
│   │   ├── wailsadapter/ Wails v3 对话框与系统操作实现
│   │   └── osfs/       Go 标准库文件系统实现
│   ├── clickhouse/      数据库交互层（service.go / export.go）
│   └── config/          持久化层（manager / export / import / history / tabs / saved_sql / settings）
├── frontend/
│   └── src/             Vue 3 前端
│       ├── main.ts      应用装配（Pinia / Router / i18n / 自定义指令）
│       ├── App.vue      应用外壳：侧边栏 + 菜单栏 + 全局 Modal/右键菜单宿主
│       ├── api.ts       Wails 绑定唯一封装点
│       ├── types.ts     与后端 struct 一一对应的 TS 类型
│       ├── i18n.ts      中英切换 + localStorage 记忆
│       ├── router/      仅一条路由 → QueryEditor
│       ├── stores/      Pinia（app.ts 唯一）
│       ├── utils/       纯函数（ddl.ts / tabs.ts / resultTable.ts / sqlProjection.ts / dragPanel.ts / sqlLiteral.ts / explain.ts / sqlError.ts / pagination.ts / completionContext.ts / completionItems.ts / resultView.ts / resultFilter.ts / autoRefresh.ts / cellEdit.ts，均配 *.test.ts）
│       ├── views/       QueryEditor（唯一视图）
│       ├── components/  业务组件（DatabaseTree / ResultGrid / ResultFilterPanel / HistoryPanel / SavedSQLPanel）+ 5 个 Modal + MenuBar
│       ├── composables/ useMonaco.ts（编辑器初始化 / 懒加载 / 主题 / 事件桥接）、useSqlCompletion.ts（SQL 补全 provider + 缓存）
│       └── locales/     zh.ts / en.ts
├── docs/                API.md / FEATURES.md / CODE_INDEX.md
└── .trae/               PROJECT.md / rules/
```

依赖方向是单向的：

```
cmd/meteor-ch-gio（Gio 原生 GUI）
  └─► internal/gui/gioapp（窗口布局、连接树与交互状态）
        └─► internal/app.Service

app.go（Wails 兼容适配层）
  ├─► internal/platform/wailsadapter（系统对话框 / 打开目录）
  ├─► internal/platform/osfs（本地文件读写）
  └─► internal/app（GUI 无关的应用用例）
        ├─► internal/config
        └─► internal/clickhouse ──► internal/config（仅引用 Connection 类型）
frontend/src/api.ts ──► app.go（经 Wails Call.ByName）
```

`clickhouse` 依赖 `config` 只为一个用途：读取 `config.Connection` 建连。不存在反向依赖，无需担心循环引用。

前端装配链：

```
main.ts ──► App.vue ──► router ──► QueryEditor.vue
   │           │
   └─► stores/app.ts（loadAll 启动时拉全量）
   └─► i18n.ts（初始语言从 localStorage / navigator.language 推断）
```

---

## 后端

### [main.go](file:///C:/works/go_works/meteor-ch-workbench/main.go)

职责：创建 Wails 应用、注册 `App` 服务、绑定事件。

| 关键点 | 说明 |
|---|---|
| `App` 注册 | 所有 `*App` 的公开方法自动暴露给前端 |
| 启动参数 | 窗口尺寸、标题等在 `main.go` 中定义 |

### [app.go](file:///C:/works/go_works/meteor-ch-workbench/app.go)

职责：**旧版前端调用的兼容入口**，保留历史方法签名并委托给 `internal/app.Service`；文件对话框和打开目录通过 `internal/platform.Platform` 调用；配置文件与查询结果的读写通过 `internal/platform.FileSystem` 调用。

| 区块 | 方法 | 说明 |
|---|---|---|
| 生命周期 | `OnStartup` / `OnShutdown` | 启动时加载配置，退出时释放连接池 |
| 连接 CRUD | `GetAll` / `GetConnections` / `GetConnection` / `SaveConnection` / `DeleteConnection` | `SaveConnection` 保存后会使旧连接池失效 |
| 连通性 | `TestConnection` / `Disconnect` | |
| 分组 | `GetFolders` / `SaveFolder` / `DeleteFolder` | |
| 排序 | `ReorderFolders` / `ReorderConnections` | 拖拽后持久化顺序 |
| 配置导出 | `ExportConfig` / `GetExportFormats` / `ExportConfigByFormat` / `SaveExportAs` | |
| 配置导入 | `ImportConfig` / `GetImportFormats` / `PickImportFile` / `ImportConfigWithOptions` | |
| 查询 | `Query` / `CancelQuery` / `Execute` | SQL 错误放返回体 `.error`，非 Go error；`Query` 必须传 queryID 才能取消 |
| 结果导出 | `GetResultExportFormats` / `SaveQueryResultAs` | |
| 历史 | `GetQueryHistory` / `DeleteQueryHistory` / `ClearQueryHistory` | 查询历史由 `internal/app.Service.Query` 自动记录 |
| 收藏 SQL | `GetSavedSQL` / `SaveSQL` / `UpdateSavedSQL` / `DeleteSavedSQL` | 无上限、无清空；重复 SQL 拒绝 |
| 标签 | `GetTabs` / `SaveTabs` | |
| 元数据 | `GetDatabases` / `GetTables` / `GetViews` / `GetTableSchema` / `GetTableDetails` / `GetClusters` | |
| 其他 | `OpenConfigFolder` / `GetAppVersion` | 平台操作经 `internal/platform` 注入 |

**阅读顺序建议**：先看 `GetAll` 了解数据结构，再看 `Query` 了解错误约定，其余方法模式高度一致。

### [internal/clickhouse/service.go](file:///C:/works/go_works/meteor-ch-workbench/internal/clickhouse/service.go)

职责：连接池管理 + SQL 执行 + 元数据查询。

| 符号 | 类型 | 说明 |
|---|---|---|
| `Service` | struct | `mu` + `dbs map[string]*sql.DB` + `running map[string]context.CancelFunc` |
| `Service.getDB(conn)` | 函数 | 懒建连，已存在则复用；`SetMaxOpenConns(5)` |
| `Service.Invalidate(id)` | 函数 | 关闭并移除指定连接池 |
| `Service.register(queryID)` | 函数 | 注册可取消 context；空 ID 退化为 `context.Background()`；返回注销函数 |
| `Service.CancelQuery(queryID)` | 函数 | 取消运行中查询，返回是否真的找到并取消 |
| `Service.Query(conn, sql, queryID)` | 函数 | 支持取消；取消时返回 `Error = "查询已取消"` |
| `Service.Execute(conn, sql)` | 函数 | DDL/DML，返回 `ExecResult` |
| `ensureMutationSync(sql)` | 函数 | mutation 语句（`ALTER ... UPDATE/DELETE`、`DELETE FROM`）自动追加 `SETTINGS mutations_sync = 2`，等服务端 mutation 完成才返回；已显式指定则不覆盖 |
| `isMutationStatement(sql)` / `stripLeadingComments(sql)` / `hasSetting(sql, key)` | 函数 | 语句类型判定（跳过前置注释）、SETTINGS 键存在性检查 |
| `buildDSN(conn)` | 函数 | 由 protocol + secure 决定 scheme |
| `scanRows(ctx, rows, limit)` | 函数 | 逐行检查 ctx，使取消能在一行内生效 |
| `errQueryCanceled` | 常量 | `"查询已取消"`，前端按此文案识别取消 |
| `isCanceled(ctx)` | 函数 | 只认 `context.Canceled`，超时不算用户取消 |
| `QueryResult` | struct | `columns` 现为 `[]ColumnInfo`（含 name/type/comment/is_primary/has_default） |
| `ColumnInfo` / `TableInfo` / `TableDetails` / `ClusterInfo` | struct | 元数据返回结构；`TableDetails` 包含引擎、注释、完整 DDL、分区键、排序键、主键和 TTL 摘要 |
| `Service.GetTableDetails(conn, database, table)` | 函数 | 读取 `system.tables` 元数据；TTL 从 `create_table_query` 提取，完整 DDL 原样保留 |
| `DefaultQueryRowLimit` | 常量 | 见 export.go，50000 |

**重要设计说明**：文件头注释解释了为什么用 DSN 而非 `clickhouse.Open` + `Options.Protocol` —— 后者存在协议路由缺陷，指定 HTTP 仍发 Native 握手包。**这是踩过坑的记录，改动建连方式前务必读完那段注释。**

### [internal/clickhouse/export.go](file:///C:/works/go_works/meteor-ch-workbench/internal/clickhouse/export.go)

职责：结果集导出注册表 + 行数上限常量。

| 符号 | 说明 |
|---|---|
| `DefaultQueryRowLimit = 50000` | 未配置 `settings.json` 时 `Query` 的行数上限；实际生效值由 `SettingsManager.RowLimit()` 提供，`0` 表示不限制（见 [settings.go](file:///C:/works/go_works/meteor-ch-workbench/internal/config/settings.go)） |
| `resultExportRegistry` | 私有注册表：`{Meta, RenderTo}`，`RenderTo` 统一签名 `func(io.Writer, columns, rows, target) error` |
| `ResultExportFormats()` | 返回元信息列表，供前端下拉 |
| `ExportResultTo(w, format, columns, rows, target)` | **主契约**：按 ID 分发，逐行写入 `io.Writer`，返回 `(扩展名, error)`；不构造完整字符串 |
| `ExportResultByFormat(format, columns, rows, target)` | 兼容层：基于 `bytes.Buffer` 调 `ExportResultTo`，返回 `(文本, 扩展名, error)`，与流式输出逐字节一致 |
| `ResultExportExtension(format)` | 查询格式对应的扩展名，未知格式返回空串 |
| `renderResultCSV` / `renderResultJSON` / `renderResultMarkdown` / `renderResultInsert` | 具体渲染实现（`*To` 版本）；前三者忽略 `target` |
| `renderResultJSONLTo` | JSON Lines：每行一个对象，缺列补 `null`，按 4 KB 刷盘 |
| `renderResultTSVTo` / `escapeTSVCell` | TSV：表头 + 数据，单元格 `\`（先双写）、`\t`、`\r\n`、`\n`、`\r` 转义 |
| `renderResultDDLTo` / `inferColumnType` | 建表 + INSERT：按列首个非空值推断 `Int64` / `Float64` / `UInt8` / `String`，`target` 必填 |
| `flushBuilder` | 通用流式刷盘：缓冲达到阈值时写 `io.Writer` 并清空 |
| `escapeMarkdownCell` | Markdown 单元格转义：`\` 优先双写 → `|` 转 `\|` → 换行统一转 `<br>` |
| `escapeInsertIdentifier` | INSERT 列名反引号包裹，内部反引号双写 |
| `insertLiteral` / `escapeInsertString` | INSERT 值字面量，规则与 `frontend/src/utils/sqlLiteral.ts` 对齐（`\` 双写、`'` 转 `\'`、布尔 1/0、非有限浮点转 NULL） |

**Excel 单独成文件**：[export_xlsx.go](file:///C:/works/go_works/meteor-ch-workbench/internal/clickhouse/export_xlsx.go) 负责 `.xlsx` 渲染（依赖 `github.com/xuri/excelize/v2`），因为 xlsx 是 zip 容器，无法逐行写文本，需先在内存中构建工作簿再整体写出。要点：

| 符号 | 说明 |
|---|---|
| `renderResultXLSXTo` | 工作簿字节流写入 `io.Writer`，签名与其他格式一致 |
| `buildXLSX` | 单工作表 `Result`（删除默认 `Sheet1`）、表头加粗、冻结首行、自动筛选、列宽自适应 |
| `setXLSXCell` | 按类型写单元格：数值 → 数字、布尔 → `TRUE/FALSE`、非有限浮点转文本、其余 → 字符串 |
| `columnAxis` / `xlsxColumnWidth` / `displayWidth` / `isWideRune` | 列名换算（上限 16384 列）与列宽估算（CJK 双宽，夹在 8–60） |
| `truncateRunes` / `xlsxMaxCellRunes` | 按字符截断到 32767，避免 Excel 报文件损坏 |

**扩展方式**：向 `resultExportRegistry` 追加一项即可，前端自动感知。补测试到 [export_test.go](file:///C:/works/go_works/meteor-ch-workbench/internal/clickhouse/export_test.go) 或按格式拆到 [export_formats_test.go](file:///C:/works/go_works/meteor-ch-workbench/internal/clickhouse/export_formats_test.go)。

> `insert` / `sql_ddl` 依赖 `target`（目标表名），由 `App.SaveQueryResultAs` 的第四参数透传；前端在 [ResultGrid.vue](file:///C:/works/go_works/meteor-ch-workbench/frontend/src/components/ResultGrid.vue) 弹出目标表对话框，默认值取侧边栏选中表，也允许手动输入（`sql_ddl` 无目标表时回退占位符 `your_table`）。两处转义规则必须同步，`export_test.go` 有对应一致性用例。

### [internal/config/manager.go](file:///C:/works/go_works/meteor-ch-workbench/internal/config/manager.go)

职责：`connections.json` 的读写与内存缓存。**这是持久化模式的原型**，history.go / tabs.go 都沿用它。

| 符号 | 说明 |
|---|---|
| `Connection` / `Folder` / `DataFile` | 核心数据结构 |
| `Manager` | 持有 `mu` + 内存数据 + 文件路径 |
| `LoadAll` / `Save` / `Delete` / `Reorder` | CRUD |

**模式要点**：内存缓存 + `sync.RWMutex` + 变更后全量写回文件。新加持久化文件时照此实现。

### [internal/config/export.go](file:///C:/works/go_works/meteor-ch-workbench/internal/config/export.go)

职责：配置导出注册表（5 种格式）。

| 符号 | 说明 |
|---|---|
| `ExportFormat` | 元信息结构 |
| `exportRegistry` | 私有注册表 |
| `sanitize` | **深拷贝后清空密码**，确保脱敏不污染内存数据 |
| `ExportByFormat` / `ExportFormats` | 对外入口 |

### [internal/config/import.go](file:///C:/works/go_works/meteor-ch-workbench/internal/config/import.go)

职责：配置导入解析器 + 合并策略。**本包最容易出错的文件**，P4 在这里修了 3 个缺陷。

| 符号 | 说明 |
|---|---|
| `ImportFormat` / `ImportOptions` / `ImportResult` | 入参出参结构 |
| `importRegistry` | 私有注册表：`{Meta, Detect, Parse}` |
| `looseString` | 自定义 JSON 类型，兼容 DBeaver port 的字符串/数字两种写法 |
| `parseJSONOrDBeaver` | JSON 与 DBeaver 共用入口，**先剥 `jdbc:` 前缀再 `url.Parse`** |
| `parseYAML` / `parseCSV` / `parseDSN` | 其余三个解析器 |
| `mergeData` | 合并策略：同名分组复用 → ID 冲突重分配 → **旧 ID 存入 remap 后再重生成** |
| `normalizeData` | 清洗悬空 `folder_id` 等非法引用 |

**改动前必读**：`mergeData` 中 `oldID := f.ID` 必须写在 `f.GenID()` **之前**，否则 remap 键会错误。这是 P4 修复的 bug，别改回去。

### [internal/config/history.go](file:///C:/works/go_works/meteor-ch-workbench/internal/config/history.go)

职责：查询历史持久化。

| 符号 | 说明 |
|---|---|
| `HistoryMaxEntries = 200` | 保留上限 |
| `HistoryEntry` | id / sql / connectionId / timestamp / duration / success |
| `HistoryManager` | 沿用 Manager 模式 |
| `Add` | 新增后裁剪到上限 |

> 与 [saved_sql.go](file:///C:/works/go_works/meteor-ch-workbench/internal/config/saved_sql.go) 语义必须保持分离：历史是自动流水、有上限、可清空；收藏是用户资产、无上限、不受清空历史影响。

### [internal/config/saved_sql.go](file:///C:/works/go_works/meteor-ch-workbench/internal/config/saved_sql.go)

职责：手动收藏 SQL 的持久化（`saved_sql.json`），模式同构于 history.go。

| 符号 | 说明 |
|---|---|
| `SavedSQL` | id / name / sql / connectionId / createdAt / updatedAt |
| `SavedSQLManager` | 沿用 Manager 模式：内存缓存 + `sync.RWMutex` + 变更后全量写回 |
| `Add` | 追加到列表头部；空 SQL 拒绝、忽略首尾空白的重复 SQL 拒绝；`name` 为空时取 SQL 首个非空行（超 80 字符截断） |
| `Update` | 按 ID 改标题与 SQL，保留 `CreatedAt`、刷新 `UpdatedAt` |
| `Delete` | 按 ID 删除，不存在返回 `ErrSavedSQLNotFound` |
| `ErrSavedSQLDuplicate` / `ErrSavedSQLNotFound` | 前端据此映射 `editor.savedDuplicate` 等提示 |

> **无条数上限**，也不提供 `Clear` —— 收藏只能逐条删除，避免误操作清空用户资产。

### [internal/config/tabs.go](file:///C:/works/go_works/meteor-ch-workbench/internal/config/tabs.go)

职责：标签页快照持久化（仅标题 + SQL + activeId，不存结果）。

| 符号 | 说明 |
|---|---|
| `TabsFile` | `{tabs, activeId}` |
| `TabsManager` | 沿用 Manager 模式 |

### [internal/config/settings.go](file:///C:/works/go_works/meteor-ch-workbench/internal/config/settings.go)

职责：用户偏好设置持久化（`settings.json`），目前只有 `query_row_limit` 一项。

| 符号 | 说明 |
|---|---|
| `SettingsFile` | `{query_row_limit}`，`0` 表示不限制 |
| `SettingsManager` | 沿用 Manager 模式；`RowLimit()` 供查询路径直接读取，读盘失败静默退回 `DefaultRowLimit` |
| `NormalizeRowLimit` | 纯函数，收敛到 `[RowLimitMin=100, RowLimitMax=1000000]`，`0` 原样保留，负数 / 过小回落默认 50000 |

---

## 后端测试

| 文件 | 覆盖重点 |
|---|---|
| [export_test.go](file:///C:/works/go_works/meteor-ch-workbench/internal/config/export_test.go) | 5 格式渲染、脱敏**不改源数据**、转义、往返一致性 |
| [import_test.go](file:///C:/works/go_works/meteor-ch-workbench/internal/config/import_test.go) | DBeaver 多版本兼容、CSV 缺列/短行、DSN 非法行、合并三场景、Manager 集成 |
| [history_test.go](file:///C:/works/go_works/meteor-ch-workbench/internal/config/history_test.go) | 记录、上限裁剪、删除、清空 |
| [saved_sql_test.go](file:///C:/works/go_works/meteor-ch-workbench/internal/config/saved_sql_test.go) | 新增 / 无上限 / 空 SQL 拒绝 / 重复拒绝 / 更新 / 删除 / **清空历史不影响收藏** / 重载 / 空文件 / 损坏文件 |
| [tabs_test.go](file:///C:/works/go_works/meteor-ch-workbench/internal/config/tabs_test.go) | 快照读写、容错 |
| [export_test.go（clickhouse）](file:///C:/works/go_works/meteor-ch-workbench/internal/clickhouse/export_test.go) | 结果集 CSV / JSON / Markdown 渲染（含 Markdown 竖线、换行、CRLF、反斜杠转义边界） |
| [export_formats_test.go](file:///C:/works/go_works/meteor-ch-workbench/internal/clickhouse/export_formats_test.go) | JSONL 逐行合法性、TSV 转义与列数、DDL 建表结构与类型推断、Excel 内容 / 截断 / 列名换算 / 写入失败 |
| [cancel_test.go](file:///C:/works/go_works/meteor-ch-workbench/internal/clickhouse/cancel_test.go) | 查询取消：空 ID、未注册、重复取消、注销清理、重复注册、超时不误判 |
| [mutation_test.go](file:///C:/works/go_works/meteor-ch-workbench/internal/clickhouse/mutation_test.go) | `ensureMutationSync`：ALTER UPDATE/DELETE、DELETE FROM、大小写、前置注释、显式 mutations_sync 不覆盖、SELECT/INSERT/CREATE/ADD COLUMN 不误判、空语句与纯注释 |

**测试风格约定**：表驱动；用例名用中文描述场景；重点覆盖「边界 + 错误 + 往返一致性」，不只测正常路径。

---

## 前端

### [main.ts](file:///C:/works/go_works/meteor-ch-workbench/frontend/src/main.ts)

应用装配入口。顺序敏感：

```ts
createPinia → router → i18n → 注册 v-focus 指令 → store.loadAll() → mount('#app')
```

| 关键点 | 说明 |
|---|---|
| `v-focus` 自定义指令 | 全局可用，Modal 打开时自动聚焦 + `input` 自动全选 |
| `store.loadAll()` | 在 mount 前调用，保证首屏已有数据 |
| `app.config.errorHandler` | 运行时错误以红色横幅显示在页面顶部（配合 `index.html` 的 `window.onerror` / `unhandledrejection`）。**Vue 组件内错误默认不冒泡到 `window.onerror`**，没有这层捕获就只能看到白屏，无法定位 |

新增全局插件 / 指令在此登记。

### [App.vue](file:///C:/works/go_works/meteor-ch-workbench/frontend/src/App.vue) ★

**应用外壳，容易误认为只是容器**。实际承担以下职责：

| 职责 | 说明 |
|---|---|
| 侧边栏 | 宽度 160–480（默认 256），可拖拽调宽，宽度存 `localStorage['meteor-ch.sidebarWidth']` |
| 侧边栏折叠 | 折叠状态持久化 |
| 菜单栏 | 内嵌 `MenuBar.vue`（连接选择 / 语言切换） |
| 语言切换 | `showLangMenu` + `setLocale`，与 `i18n.ts` 联动 |
| 全局 Modal 宿主 | `ConnectionModal` / `FolderModal` 在此挂载，**不在 ConnectionManager 中** |
| 全局右键菜单 | `ctxMenu` 状态（folder / connection 两类），统一渲染 |
| 调试 debug | `console.log` 有 debug 开关 |
| 启动行为 | `onMounted` 只 `loadAll()`，**不调用 `checkAllConnections()`**，保证默认不建任何连接 |

**注意**：`ConnectionManager.vue` 里的 Modal 引用是历史遗留，真实宿主在 `App.vue`。

### [router/index.ts](file:///C:/works/go_works/meteor-ch-workbench/frontend/src/router/index.ts)

| 关键点 | 说明 |
|---|---|
| `createWebHashHistory` | **必须是 hash 模式**，Wails 的 `base: './'` 下 history 模式会 404 |
| 路由表 | `/` → 重定向 `/query`；`/query` 懒加载 `QueryEditor.vue` |

全应用只有这一条实际路由。新增页面在此登记。

### [i18n.ts](file:///C:/works/go_works/meteor-ch-workbench/frontend/src/i18n.ts)

| 符号 | 说明 |
|---|---|
| `Locale` | `'en' \| 'zh'` |
| `getInitialLocale` | 优先 `localStorage['meteor-locale']`，否则按 `navigator.language` 推断（zh 开头 → zh，其余 → en） |
| `i18n` | `legacy: false`（Composition API 模式，用 `useI18n()`） |
| `setLocale` | 写 locale + `localStorage` + `document.documentElement.lang` |
| `getCurrentLocale` | 读取当前语言 |

`fallbackLocale: 'en'`——**新增 key 若只补中文，英文环境会回退到 en 而显示缺失**，所以中英必须同步。

### [style.css](file:///C:/works/go_works/meteor-ch-workbench/frontend/src/style.css)

Tailwind 指令入口（`@tailwind base/components/utilities`）+ 全局基础样式 + 滚动条定制。

### [shims-vue.d.ts](file:///C:/works/go_works/meteor-ch-workbench/frontend/src/shims-vue.d.ts)

仅声明 `*.vue` 模块，供 `vue-tsc` 识别单文件组件。**不要删除**，删了类型检查会全线报错。

### [api.ts](file:///C:/works/go_works/meteor-ch-workbench/frontend/src/api.ts)

职责：**所有 Wails 绑定的唯一封装点**。组件层不直接写 `Call.ByName`。

```ts
// 统一形态
export const api = {
  GetAll: () => Call.ByName('main.App.GetAll'),
  Query: (connectionID, sql) => Call.ByName('main.App.Query', connectionID, sql),
  // ...
}
```

**规则**：后端加公开方法 → 这里必须加一条；不加则组件无法调用。

### [types.ts](file:///C:/works/go_works/meteor-ch-workbench/frontend/src/types.ts)

共享 TS 接口，与后端 struct 一一对应。**后端改字段必须同步这里**。

含：`Connection` / `Folder` / `DataFile` / `ColumnInfo` / `TableInfo` / `ClusterInfo` / `QueryResult` / `ExecResult` / `ExportFormat` / `ImportFormat` / `ImportOptions` / `ImportResult` / `HistoryEntry` / `TabsFile` / `ResultExportFormat`。

### [stores/app.ts](file:///C:/works/go_works/meteor-ch-workbench/frontend/src/stores/app.ts)

唯一 Pinia store。

| 状态 / 动作 | 说明 |
|---|---|
| `folders` / `connections` / `activeConnectionId` | 核心数据；`activeConnectionId` 初始为 `null` |
| `connectionStatus` | 连通性缓存 |
| `groupedConnections` | computed，按分组聚合 |
| `loadAll` | 启动时拉全量配置（**只读本地 JSON，不建任何网络连接**） |
| `checkConnection` / `checkAllConnections` | 连通性探测。**启动时不得自动调用**——它会真实 `TestConnection` 打后端，节点不可达就刷一屏 `dial tcp ... timeout`。仅在用户点连接、右键检查、导入配置后调用 |

### [utils/ddl.ts](file:///C:/works/go_works/meteor-ch-workbench/frontend/src/utils/ddl.ts)

**纯函数，无副作用**，前端生成 SQL、后端只执行。

| 符号 | 说明 |
|---|---|
| `CLICKHOUSE_ENGINES` | 26 种引擎 |
| `CLICKHOUSE_TYPES` | 42 种类型 |
| `buildCreateTableSQL` / `buildAlterTableSQL` / `buildDropTableSQL` | 三种 SQL 构建 |
| `EditTableChange` | 变更结构 |

### [utils/tabs.ts](file:///C:/works/go_works/meteor-ch-workbench/frontend/src/utils/tabs.ts)

`QueryTab` 类型与工厂。每个标签独立持有 `sql` / `result` / `error` / `querying` / `queryId` / `canceled` / `resultScrollTop` / `viewState` / `page`（`page` 为 `PageState | null`，`null` 表示非分页的普通查询）/ `lastExecutedSql`（**最终发给后端**的语句，含分页 `LIMIT` 与筛选 `WHERE`，供「查看 SQL」面板展示）。

| 符号 | 说明 |
|---|---|
| `createTab(index, sql?)` | 构造空结果标签 |
| `restoreTabs(snapshot)` | 把 `tabs.json` 快照还原为标签列表，返回 `{ tabs, activeId }`；**保证至少一个标签**（空 / `null` 快照回退默认标签），`null` 条目或缺字段按占位值补齐，`active_tab_id` 不存在时回退第一个。视图层不得直接读 `activeTab.xxx`，必须先过此函数 |

### [utils/resultTable.ts](file:///C:/works/go_works/meteor-ch-workbench/frontend/src/utils/resultTable.ts)

**纯函数，无副作用**，结果表格虚拟滚动与列宽计算。

| 符号 | 说明 |
|---|---|
| `ROW_HEIGHT` | 固定行高 28px，虚拟滚动估算依据，**不可改成动态高度** |
| `VIRTUAL_THRESHOLD` | 200，超过才启用虚拟滚动 |
| `measureColWidths(cols, rows)` | 取前 200 行测列宽，单字符按 7px，限制 80–420px |
| `buildGridStyle(widths, count)` | 生成 `grid-template-columns`，宽度缺失时回退均分 |
| `shouldVirtualize(rowCount)` | 阈值判断 |
| `visibleColumnIndexes(total, hidden)` | 可见列原始索引，跳过隐藏列且保持顺序；越界隐藏索引自动忽略 |
| `hiddenColumnIndexes(total, hidden)` | 规范化隐藏列：去重、过滤越界 / 小数、升序 |
| `pickCellIndexes(cells, indexes)` | 按可见列裁剪行数据（供复制整行使用） |
| `gridWidthsFor(indexes, widths)` | 按可见列索引取列宽，与 `buildGridStyle` 配合保证列数一致 |

> 隐藏列的渲染契约：`ResultTableBody` 的 `columns` 始终是**完整列**，另收 `visibleColumnIndexes`，表头与行循环都用它取 `ci`，行数据用 `cellAt(row, pos)` 读到原始单元格。这样隐藏列不会导致后面列错位，事件回传的仍是原始列索引（与筛选 / 编辑 / 菜单全部对齐）。

### [utils/sqlProjection.ts](file:///C:/works/go_works/meteor-ch-workbench/frontend/src/utils/sqlProjection.ts)

**纯函数，无副作用**，「改写 SQL 隐藏列」的核心：定位单条 `SELECT` / `WITH` 的投影区间并按列删除。

| 符号 | 说明 |
|---|---|
| `findSelectList(sql)` | 返回顶层 `SELECT` 与 `FROM` 之间的字符区间；括号 / 引号内关键字不参与，含 `UNION` / `INTERSECT` / `EXCEPT` 或多语句时返回 `null` |
| `splitTopLevel(expr)` | 按顶层逗号切分投影项，不切分函数参数与字符串内的逗号 |
| `parseProjection(expr)` | 解析投影项的输出列名：`AS` 别名 → 去 `AS` 别名 → 限定名取最后一段 → 裸标识符；表达式（含 `(` 或运算符）返回 `null` |
| `projectionOutputNames(exprs)` | 批量取输出列名 |
| `rewriteProjection(sql, removeIndexes)` | 按投影项索引删除，保留 `WHERE` / `GROUP BY` / `ORDER BY` / `LIMIT` 等尾部；空索引列表原样返回 |
| `rewriteProjectionToColumns(sql, removedNames, allNames?)` | 按**结果列名**删除（界面用的入口）。若投影全部是 `*` / `t.*`，用 `allNames`（来自 `result.columns`）展开成显式列名再删；`*` 与表达式混排时不展开（安全失败）；返回 `{ ok:false, reason:'unsupported' \| 'not-found' }` |
| `stripPageTail(sql)` | 去掉分页追加的 ` LIMIT n OFFSET n`，避免改写后页状态错乱 |
| `quoteIdentifier(name)` | 非普通标识符（含空格 / 中文 / 反引号）自动加反引号并转义 |

**设计取舍**：不做完整 SQL 解析，只做「顶层关键字定位 + 括号/引号深度追踪」。`extractAlias` 对省略 `AS` 的别名只在「前段是单一列引用 / 限定列 / 函数调用」时才接受，避免把 `count(*), a` 里的 `a` 误当别名。无法可靠改写时统一返回 `ok: false` + `reason`，界面按原因分别提示。

### [utils/dragPanel.ts](file:///C:/works/go_works/meteor-ch-workbench/frontend/src/utils/dragPanel.ts)

**纯函数，无副作用**，浮层面板拖拽定位。

| 符号 | 说明 |
|---|---|
| `PANEL_MARGIN` | 8px，面板距视口边缘最小距离 |
| `clampPanelPosition(point, size, viewport, margin?)` | 把面板位置收进视口；视口小于面板时退化为 `margin` |
| `dragPanelPosition(origin, startCursor, currentCursor, size, viewport, margin?)` | 按指针位移平移原点后再做视口收敛，指针拖出窗口也安全 |

配套 [usePanelDrag.ts](file:///C:/works/go_works/meteor-ch-workbench/frontend/src/composables/usePanelDrag.ts)：`startDrag` / `moveDrag` / `endDrag` 走 `setPointerCapture`，`offset` 为空时用 `props.x/y` 作为基准，关闭时 `resetDrag()` 清空。（「按此值过滤」与「查看 SQL」两个浮层已接入）

### [utils/sqlLiteral.ts](file:///C:/works/go_works/meteor-ch-workbench/frontend/src/utils/sqlLiteral.ts)

**纯函数，无副作用**，结果复制相关的文本与 SQL 字面量渲染。

| 符号 | 说明 |
|---|---|
| `escapeSqlString(s)` | 单引号包裹，转义反斜杠与单引号 |
| `sqlLiteral(v)` | `null` → `NULL`，布尔 → `1/0`，数值原样，其余走 `escapeSqlString` |
| `quoteIdent(name)` | 反引号包裹标识符，内部反引号双写 |
| `rowToTSV(row)` | 整行转为制表符分隔，NULL 输出空串 |
| `columnsToCSV(columns)` | 列名以 `, ` 连接 |
| `buildInsertStatement(target, cols, row)` | 单行 INSERT |
| `buildInsertBatch(target, cols, rows)` | 多行 INSERT，末行分号、行间逗号 |
| `insertTarget(db, table)` | 生成 `` `db`.`table` ``；无表时回退占位符 `your_table` |

> 被 `QueryEditor.vue` 的结果单元格右键菜单使用。若后续做 SQL INSERT 导出，复用本文件而非另写转义。

### [utils/explain.ts](file:///C:/works/go_works/meteor-ch-workbench/frontend/src/utils/explain.ts)

**纯函数，无副作用**，EXPLAIN 拼接与可解释性判定（P7-2）。

| 符号 | 说明 |
|---|---|
| `EXPLAIN_MODES` | 四种模式：`PLAN` / `AST` / `PLAN_DETAIL` / `PIPELINE`，含显示名 i18n key |
| `splitStatements(sql)` | 剥离 `--` / `#` / `/* */` 注释后按 `;` 切分，丢弃空段 |
| `canExplain(sql)` | 仅单条且非 DDL/DML 等以 `BLOCKED_PREFIXES` 开头的语句返回 true |
| `getExplainKeyword(mode)` | 模式 → `EXPLAIN` / `EXPLAIN AST` / `EXPLAIN PLAN` / `EXPLAIN PIPELINE` |
| `buildExplainSQL(sql, mode)` | 取首条语句拼前缀；空内容返回空串 |

> 前端零后端改动：拼好的 SQL 直接走现有 `api.Query`，结果复用结果表格。`QueryEditor.vue` 结果工具栏的「执行计划」按钮 + 模式下拉即由此驱动，按钮在 `querying` 或 `!canExplain` 时禁用。

### [utils/sqlError.ts](file:///C:/works/go_works/meteor-ch-workbench/frontend/src/utils/sqlError.ts)

**纯函数，无副作用**，ClickHouse 错误文本友好化（P7-5）。

| 符号 | 说明 |
|---|---|
| `parseSqlError(raw)` | 返回 `{ code, summary, raw }`：提炼错误码、一句话原因（`DB::Exception` 描述 + 高频码中文映射），原文完整保留 |
| `extractErrorCode(text)` | 从 `Code: 60.` 提取数字，未命中返回 `null` |
| `describeErrorCode(code)` | 高频错误码中文映射（60 表不存在 / 47 标识符 / 62 语法 / 159 超时 / 241 内存 等），未收录返回 `null` |
| `firstLine(text)` | 无错误码时的回退首行 |
| `isCanceledMessage(text, canceledText)` | 取消提示判定 |

> 结果面板把 `summary` 作为短提示、`raw` 折叠在「展开原文」按钮后，避免友好化掩盖真实信息。

### [utils/pagination.ts](file:///C:/works/go_works/meteor-ch-workbench/frontend/src/utils/pagination.ts)

**纯函数，无副作用**，结果集分页（P7-3，方案 A：前端拼 SQL）。

| 符号 | 说明 |
|---|---|
| `DEFAULT_PAGE_SIZE` | 50000，与后端 `DefaultQueryRowLimit` 对齐 |
| `splitStatements(sql)` | 剥注释后按 `;` 切分，注释中的分号不参与切分 |
| `headKeyword(stmt)` | 取首关键字大写，兼容 `count(1)` 这种带括号写法 |
| `isSingleSelect(sql)` | 可翻页判定：单条语句且首关键字为 `SELECT` / `WITH`，DDL/DML/SHOW/EXPLAIN 等一律拒绝 |
| `buildPageSQL(sql, offset, pageSize)` | 追加 `LIMIT n OFFSET m`；剥离尾部空分号，负偏移归零、零页大小归 1、小数取整 |
| `nextPage` / `prevPage` / `resetPage` | 分页状态推进，`prevPage` 不越过 0 |
| `currentPageNumber` / `hasPrevPage` / `hasNextPage` | 页码与按钮可用性；`hasNextPage` 依据「结果行数 ≥ 页大小」判断最后一页 |

> `ResultGrid` 的截断提示条右侧出现「上一页 / 第 N 页 / 下一页」，`QueryEditor` 的 `executeSelectedSQL(rawSQL, page)` 在 `page` 非空时用 `buildPageSQL` 拼装实际执行 SQL。后端零改动，`docs/API.md` 不变。

### [utils/completionContext.ts](file:///C:/works/go_works/meteor-ch-workbench/frontend/src/utils/completionContext.ts)

**纯函数，无副作用**，SQL 补全的上下文判定（P8-3）。

| 符号 | 说明 |
|---|---|
| `getCompletionContext(text, cursor)` | 返回 `{ kind, prefix, qualifier, preferTable, preferColumn }`：判定光标处于表名位置 / 列位置 / 其他；`kind` 为 `table` / `column` / `keyword` |
| `extractTableRefs(sql)` | 从 `FROM` / `JOIN` / `INTO` 等子句提取库表引用与别名，单段表名（无库）忽略 |
| `resolveQualifier(sql, qualifier)` | 按别名 → 表名 → 库表全名依次解析限定符，未命中返回 `null` |
| `isCursorInComment(text, cursor)` | 光标是否落在 `/* */` 块注释或 `--` / `#` 行注释内，命中则不触发补全 |
| `unquoteIdentifier(raw)` | 剥离反引号 / 双引号并还原转义，返回 `db.table` |
| `filterByPrefix(items, prefix)` | 忽略大小写的前缀过滤 |

> 判定规则要点：`FROM` / `JOIN` 之后到出现 `WHERE` / `ON` / `,` 等关键字之前视为表名位置；`db.` / 别名后的点触发列补全；其余位置按「SELECT 列表 → 列」，找不到上下文时退回关键字。

### [utils/completionItems.ts](file:///C:/works/go_works/meteor-ch-workbench/frontend/src/utils/completionItems.ts)

**纯函数，无副作用**，把上下文 + 数据源组装为补全候选列表（P8-3）。

| 符号 | 说明 |
|---|---|
| `SQL_KEYWORDS` | 静态关键字表（`SELECT` / `FROM` / `GROUP BY` / `JOIN` / `WITH` / `FORMAT` 等） |
| `SQL_FUNCTIONS` | ClickHouse 常用函数与返回类型说明（`count` / `uniq` / `sum` / `toDateTime` 等） |
| `buildCompletionItems(options)` | 按 `context.kind` 组装候选：表名位置给库名 + 全限定表名，列位置给列名（带 `db.table` 限定符时仅给该表列，SELECT 列表额外带表名），其他位置给关键字 + 函数 |
| `CompletionItem` | `{ label, kind, detail, insertText }`；函数插入带左括号，表名插入反引号包裹的全名 |

> 与 `useSqlCompletion.ts` 的分工：本文件不依赖 Monaco 与 API，纯数据组装，可完全单测；provider 只负责取数据、转 `CompletionItemKind` 与坐标换算。

### [utils/resultView.ts](file:///C:/works/go_works/meteor-ch-workbench/frontend/src/utils/resultView.ts)

**纯函数，无副作用**，结果表格的复制字段与本地搜索能力。

| 符号 | 说明 |
|---|---|
| `isEmptyCell(v)` | `null` / `undefined` 视为空 |
| `collectColumnValues` / `distinctColumnValues` / `columnValuesText` | 按索引顺序取某列非空值 / 去重 / 换行拼接 |
| `columnListText(columns, index)` | 取列标题，越界返回空串；表头菜单「复制列名」只取当前列 |
| `cellText(v)` | 单元格转可搜索文本，空值转空串 |
| `rowMatches(row, keyword, column)` | 单行匹配：关键字去首尾空白后忽略大小写子串匹配；`column >= 0` 只匹配该列，否则全列；空关键字恒真 |
| `filterRowIndices(rows, keyword, column)` | 返回命中的原始行索引数组；空关键字返回全部索引，供 `rowIndices` 直接驱动虚拟滚动与复制字段 |

> `ResultGrid` 的 `rowIndices` 由 `filterRowIndices(result.rows, searchText, searchColumn)` 计算：搜索为空时等价于原来的全量索引，搜索非空时虚滚动、复制字段、行数统计自动跟随过滤结果。

### 空结果与空值约定（`QueryResult.rows`）

后端 `scanRows` 以 `make([][]interface{}, 0)` 初始化，**0 行时返回 `[]` 而非 `nil`**（见 [service.go](file:///C:/works/go_works/meteor-ch-workbench/internal/clickhouse/service.go)）。但历史数据/兜底路径仍可能给 `null`，故 `frontend/src/types.ts` 将 `rows` 声明为 `any[][] | null`。

> 正因为类型标了可空，`vue-tsc` 能全量揪出所有裸访问（本轮共 6 处：`DatabaseTree.vue` 的 DDL 取值、`ResultGrid.vue` 的导出 / 右键取行 / 取单元格值等）。约定：**访问 `result.rows` / `result.columns` 一律先判空或 `?.` + `?? []`**，不要写 `result.rows.length`。新增相关代码若出现 `TS18047`，说明漏了判空。

### [utils/cellEdit.ts](file:///C:/works/go_works/meteor-ch-workbench/frontend/src/utils/cellEdit.ts)

**纯函数，无副作用**，结果单元格双击编辑 → `ALTER TABLE ... UPDATE` 语句生成（P10）。

| 符号 | 说明 |
|---|---|
| `SourceRef` / `EditTarget` / `KeyColumn` | 源表引用（`database` 可空 + `table`）/ 编辑目标（列名 + 旧值 + 新值）/ 定位列（列名 + 值） |
| `NO_SOURCE_HINT` | 无法定位源表时的提示文案（`JOIN` / `UNION` / 子查询场景） |
| `stripComments(sql)` | 剥离 `/* */` / `--` / `#` 注释，避免注释中的 `FROM` 误判 |
| `splitQualified(name)` | 拆 `db.table` / `table`，剥离反引号与引号，超过两段返回 `null` |
| `parseQuerySource(sql)` | 从单表 `FROM` / `UPDATE` 提取源表；命中 `JOIN` / `UNION` / 带括号子查询 / `numbers` / `system` / `remote` / `cluster` 时返回 `null` |
| `canEditCell(source, keyColumns)` | 允许编辑判定：源表可解析 + 至少一个定位列 + 定位列值全部非空 |
| `isNumericType(type)` | 按 `U?Int*` / `Float*` / `Decimal` / `Bool` 前缀判定数值类型 |
| `numericLiteral(raw)` | 合法数字字面量原样返回，否则回退 `sqlLiteral` |
| `buildKeyCondition(keyColumns, columnType?)` | 生成 `\`col\` = 值 AND ...`，数值列不加引号 |
| `buildAlterUpdateSQL(source, target, keyColumns, columnType?)` | 生成 `ALTER TABLE \`db\`.\`table\`\nUPDATE \`col\` = 值\nWHERE 条件;`；源表或定位列为空返回空串 |
| `valueForEdit(raw)` | 单元格值 → 可编辑字符串，空值转空串 |
| `AutoKeyInput` / `AutoKeyResult` | 自动选键输入（`columns` 含 `is_primary`、`rows`）/ 输出（`indexes` + `reason: 'primary' \| 'unique' \| 'all'`） |
| `detectKeyColumns(input)` | **三级自动选键（对齐 DBeaver）**：① 有 `is_primary` 列 → 用全部主键列；② 无主键 → 取第一个「全非空且全唯一」列；③ 都没有 → 退化为**全部列**（因此绝大多数结果集无需手动指定即可编辑） |
| `buildRowKeys(indexes, columns, row)` | 按列索引从当前行取出 `KeyColumn[]`，越界索引自动过滤 |

> `ResultGrid` 双击单元格时用它算出 SQL，先**暂存到 `pendingEdits`**（单元格变琥珀色显示新值），底部操作条「应用修改」再一次性 `emit('apply-edits', statements)`；`QueryEditor.onResultApplyEdits` 逐条调 `api.Execute` 执行，全部成功后按当前查询刷新结果（`keepResult`），失败即停并写入 `tab.error`。另有「撤销修改」（清空待提交）与「填入编辑器」（`emit('edit-sql')` 批量填入 Monaco，不执行）。结果刷新 / 翻页后 `watch(result)` 清空待提交列表。
>
> **定位列（WHERE 依据）默认全自动**：`watch(result, { immediate: true })` 调 `detectKeyColumns` 算出 `autoKey`，表头以 `KEY` 徽标标出所选列。用户可在表头右键「用作编辑定位列」覆盖（`userKeyIndex >= 0` 时优先于自动结果，再点一次取消回到自动）。表头菜单顶部展示 `editKeyHint` 说明当前策略（主键 / 唯一列 / 全部列 / 手动）。
>
> 自动化依赖后端 `ColumnInfo.is_primary`：`GetTableSchema` 的 `DESCRIBE TABLE` 本就返回 `primary_key` 列，现已解析透出（见 `internal/clickhouse/service.go`），前端据此判定主键。

### [utils/resultFilter.ts](file:///C:/works/go_works/meteor-ch-workbench/frontend/src/utils/resultFilter.ts)

**纯函数，无副作用**，结果区 DBeaver 风格排序 / 按值过滤，以及 Navicat 风格的多条件筛选：把表格操作翻译成 SQL 子句。

| 符号 | 说明 |
|---|---|
| `FILTER_EQ` / `FILTER_NE` / `FILTER_GT` / `FILTER_GE` / `FILTER_LT` / `FILTER_LE` / `FILTER_LIKE` / `FILTER_NOT_LIKE` | 八种过滤操作符常量 |
| `FILTER_OP_ORDER` | 菜单展示顺序 |
| `isNullValue(v)` / `valuePreview(v, max)` | 空值判定；预览按「头…尾」截断（默认 18 字符） |
| `opSymbol(op)` | 操作符 → SQL 符号（`NOT LIKE` 单独处理） |
| `quoteColumn(column)` / `escapeSqlString(s)` | 列名反引号包裹；字符串单引号包裹并按 ClickHouse 规则转义 |
| `filterExpression(col, op, value, percent)` | 生成条件表达式；空值 `=` → `IS NULL`、`<>` → `IS NOT NULL`；`LIKE` / `NOT LIKE` 统一走 `likePattern` 自动补 `%值%`（含 `%` / `_` 时不补） |
| `conditionPreview(col, op, value)` | 菜单项文案（列 + 符号 + 截断值） |
| `isConditionable(sql)` | 仅单条 `SELECT` / `WITH` 可用于追加条件 |
| `appendSort(sql, conditions)` | 追加/替换 `ORDER BY`（列名 `quoteColumn`），保留前置注释与 `FORMAT` / `SETTINGS` 等尾部子句 |
| `appendWhere(sql, exprs)` | 在 `ORDER BY` 之前追加 `WHERE`，多条件用 `AND` 连接 |
| `FILTER_JOIN_AND` / `FILTER_JOIN_OR` / `FILTER_JOIN_ORDER` | 条件组之间的连接词常量与展示顺序 |
| `FilterRule` / `FilterCondition` | 规则（操作符 + 值 + `numeric?` + `time?`）/ 条件组（列 + 连接词 + 多条规则） |
| `isNumericType(type)` | 按 `Nullable(U?Int* / Float* / Decimal*)` 前缀判定 ClickHouse 数值列 |
| `timeKindOf(type)` / `isTimeType(type)` | 判定时间列并分类为 `datetime64` / `datetime` / `date`（剥离 `Nullable()` 后再匹配，`DateTime64` 先于 `DateTime` 判定） |
| `ruleMode(rule)` | 规则模式：`time` 优先 → `numeric` → `text`（面板据此切换占位符与模式按钮） |
| `stripTimeZone(text)` | 去掉 `Z` / `±HH:MM` 后缀并把 `T` 换成空格；`datetime` / `date` 列必须用这种无时区文本，否则 `DateTime('...+08:00')` 在 `DateTime` 列上仍会报 TYPE_MISMATCH |
| `isTimeLiteral(text)` | 判断输入是否形如时间字面量（`YYYY-MM-DD[ T]HH:mm[:ss[.fff]][Z / ±HH:MM]`） |
| `timeLiteral(col, op, value, kind)` | 按列类型生成时间条件：`datetime64` → `DateTime64('原值')`（保留精度与时区）；`datetime` → `DateTime('YYYY-MM-DD HH:mm:ss')`；`date` → `Date('YYYY-MM-DD')`；非时间值时退回普通字符串字面量 |
| `rawFilterExpression(col, op, value, numeric, time?)` | 单条规则 → 条件表达式；`LIKE` 永远按文本走；`time` 模式且值合法时交给 `timeLiteral`，否则回退 `escapeSqlString` |
| `conditionExpression(condition)` | 条件组 → 表达式；单规则直接输出，多规则括号内 `OR` 拼接，空规则忽略 |
| `buildFilterWhere(conditions)` | 多条件组按 `AND` / `OR` 拼成完整 `WHERE` 子句，空规则自动忽略，全部为空返回空串（筛选面板的只读预览也用它） |
| `countActiveRules(conditions)` | 有效规则数（工具栏徽标用） |

> **时间列坑位**：ClickHouse 不允许把带时区的字符串直接与 `DateTime` 列比较（`Cannot convert string 2026-01-12T18:00:00+08:00 to type DateTime`）。因此时间列筛选一律经 `DateTime()/DateTime64()/Date()` 包裹，`DateTime` 列还会先剥时区，避免旧版服务端（如 22.5）解析失败。

> `ResultGrid` 只管生成 SQL 并 `emit('rerun', sql)`，由 `QueryEditor.onResultRerun` 走既有 `executeSelectedSQL` 重跑，后端零改动。

### [utils/autoRefresh.ts](file:///C:/works/go_works/meteor-ch-workbench/frontend/src/utils/autoRefresh.ts)

**纯函数，无副作用**，结果区定时刷新的秒数归一化与倒计时（P9-1）。

| 符号 | 说明 |
|---|---|
| `AUTO_REFRESH_DISABLED` / `MIN_REFRESH_SECONDS` / `MAX_REFRESH_SECONDS` | `0` 关闭，允许 1–86400 秒 |
| `AUTO_REFRESH_PRESETS` | 预设间隔 `[5, 10, 30, 60, 300]` |
| `AUTO_REFRESH_STORAGE_KEY` | `meteor-ch.autoRefreshSeconds`，存 localStorage |
| `normalizeSeconds(v)` | 非数字 / 小于 1 秒 → 关闭；小数向下取整；超过上限截断 |
| `isRefreshEnabled(seconds)` / `intervalMs(seconds)` | 启用判定与毫秒换算 |
| `isCustomPreset(seconds)` | 是否为非预设的自定义间隔（用于下拉补位） |
| `formatSeconds(seconds)` | 倒计时/选项显示用字符串 |
| `remainingSeconds(input)` | `{ enabled, intervalSeconds, lastRunAt, now }` → 剩余秒数，时钟回拨不出负数 |
| `shouldFire(input)` | 是否已到触发时刻 |
| `parseStoredSeconds(raw)` | 解析 localStorage 字符串 |

> `ResultGrid` 用 1 秒 `setInterval` 驱动倒计时，到点 emit `refresh`；父组件 `QueryEditor.onResultRefresh` 以 `keepResult = true` 调 `executeSelectedSQL`，刷新过程中旧结果不闪空。查询进行中 `tickRefresh` 不叠加触发。

### 前端单元测试

`utils/` 下每个模块均有同名 `*.test.ts`，由 **Vitest 2.x**（与 vite 5 兼容，勿升到 5.x）驱动：

```bash
cd frontend && npm run test          # 单次
cd frontend && npm run test:watch    # 监听
```

遵循项目后端同款约定：表驱动 + 中文用例名 + 覆盖正常 / 边界。当前覆盖 `ddl` / `tabs` / `resultTable` / `sqlLiteral` / `explain` / `sqlError` / `pagination` / `completionContext` / `completionItems` / `resultView` / `resultFilter` / `autoRefresh` / `cellEdit` / `menuPosition` 十四个模块（**453** 用例）。新增纯函数进 `utils/` 时同步补测试。

### composables/

#### [composables/useSqlCompletion.ts](file:///C:/works/go_works/meteor-ch-workbench/frontend/src/composables/useSqlCompletion.ts)

SQL 自动补全的 Monaco provider 与数据缓存（P8-3，纯前端，后端零改动）。

| 符号 | 说明 |
|---|---|
| `createSqlCompletion(monaco, options)` | 工厂：返回 `{ register, invalidate, dispose }`；`options` 提供 `getConnectionId()` 与 `getText()` |
| `register()` | 注册 `languages.registerCompletionItemProvider('sql', ...)`，触发字符为 `.` / 反引号 / 双引号 |
| `invalidate(connectionId?)` | 清空库表列缓存；切换连接时由 `QueryEditor` 调用 |
| 缓存 | 库 + 表清单按连接缓存（TTL 120s），列清单按 `connectionId|db|table` 缓存（TTL 120s）；同 key 并发请求复用同一 in-flight Promise，避免每次按键打后端 |

> 候选上限 200 条，`CompletionItemKind` 映射：keyword → 14、function → 1、table → 6（Class）、column → 3（Field）。补全逻辑与 Monaco 核心 chunk 分离，`monaco-editor` chunk 体积不变（2,753 kB）。

### views/

只有 [QueryEditor.vue](file:///C:/works/go_works/meteor-ch-workbench/frontend/src/views/QueryEditor.vue) 一个视图。

> 原 `ConnectionManager.vue` 为早期骨架（API 调用全部被注释），已于 2026-11-18 删除。其功能已由 `App.vue`（侧边栏 + 连接/分组 Modal + 右键菜单）完整承接。

#### [views/QueryEditor.vue](file:///C:/works/go_works/meteor-ch-workbench/frontend/src/views/QueryEditor.vue) ★

**核心视图，负责布局与跨模块编排**。P7-1 起结果区已拆入 `components/ResultGrid.vue`、数据库树已拆入 `components/DatabaseTree.vue`、历史面板已拆入 `components/HistoryPanel.vue`、编辑器初始化已抽入 `composables/useMonaco.ts`，文件按区块阅读：

| 区域 | 内容 |
|---|---|
| 顶部工具栏 | 连接选择、运行/取消按钮、历史开关、树刷新（调用 `databaseTreeRef.reload()`） |
| 标签栏 | 多标签、双击重命名、关闭 |
| 左侧树 | 已拆出，见 [DatabaseTree.vue](file:///C:/works/go_works/meteor-ch-workbench/frontend/src/components/DatabaseTree.vue) |
| 编辑器区 | 已抽入 [useMonaco.ts](file:///C:/works/go_works/meteor-ch-workbench/frontend/src/composables/useMonaco.ts)（懒加载 / 主题 / `Ctrl+Enter` / 事件桥接），本文件只保留 `restoreTab` / `saveActiveTabState` 等标签级 viewState 编排，`onMounted` 只调用 `initEditor()` |
| 结果区 | 已拆出，见 [ResultGrid.vue](file:///C:/works/go_works/meteor-ch-workbench/frontend/src/components/ResultGrid.vue) |
| 历史面板 | 已拆出，见 [HistoryPanel.vue](file:///C:/works/go_works/meteor-ch-workbench/frontend/src/components/HistoryPanel.vue) |
| 收藏面板 | 已拆出，见 [SavedSQLPanel.vue](file:///C:/works/go_works/meteor-ch-workbench/frontend/src/components/SavedSQLPanel.vue)；「收藏当前 SQL」对话框（`saveDialogVisible`，`Teleport to="body"`）留在本文件 |

**关键函数**：

| 函数 | 说明 |
|---|---|
| `executeSelectedSQL(rawSQL)` | 普通执行与 EXPLAIN 共用：`api.Query` → 写 `tab.result` / `tab.error` |
| `runQuery` | `currentEditorSelection()` 取选区或全文 → `executeSelectedSQL` |
| `cancelQuery` | 顶部运行按钮在 `querying` 时切换为取消，调用 `api.CancelQuery` |
| `closeAllMenus` | 依次调用 `databaseTreeRef.closeMenus()` 与 `resultGridRef.closeMenus()` |
| `onResultError` / `onResultScrollTop` | 接收 ResultGrid 的导出错误与滚动位置，写回活动标签 |
| `onResultEditSql(sql)` | 接收 ResultGrid 双击编辑生成的 `ALTER TABLE ... UPDATE` 语句，`setValue` 进编辑器并聚焦，**不自动执行** |
| `onTreePreview` / `onTreeSelectDB` / `onTreeInsertDDL` / `onTreeError` | 承接 DatabaseTree 的预览、选库、DDL 回填与错误上报 |
| `restoreTab` | 切换标签时恢复 Monaco viewState 与结果滚动位置（`resultGridRef.restoreScroll`） |
| `toggleHistory` / `applyHistory(sql)` | 翻转历史面板开关；接收 HistoryPanel 回填的 SQL 写入编辑器并同步到当前标签 |
| `toggleSavedSQL` / `saveCurrentSQL` / `confirmSaveSQL` / `applySavedSQL` | 收藏面板开关（与历史面板互斥）、打开收藏对话框（预填 SQL 首行作标题）、提交收藏（捕获 name / sql / connectionId）、回填选中收藏到编辑器 |

### components/

| 文件 | 职责 |
|---|---|
| [ResultGrid.vue](file:///C:/works/go_works/meteor-ch-workbench/frontend/src/components/ResultGrid.vue) | 结果区编排组件（P11-4 拆分后 **946 行**，原 1,576 行）：只保留状态与业务编排，UI 块下沉到下方子组件，编辑 / 自动刷新 / 筛选逻辑下沉到 `composables/useCellEditing.ts`、`useAutoRefresh.ts`、`useResultFilterPanel.ts`。props 收 `result / error / canceled / querying / availableSql / executedSql / insertDb / insertTable / scrollTop / page / applyResult`，emit `explain / error / scroll-top / page / refresh / rerun / edit-sql / apply-edits`，**`defineExpose({ closeMenus, restoreScroll })`** —— 被 `QueryEditor` 通过 `resultGridRef` 调用，删掉会报 `xxx is not a function`。表格下方固定分页栏由 `utils/pagination.ts` 驱动。主内容区判定链固定为 `canceled → error → showLoading → isEmptyResult → searchNoMatch → hasTable → 初始提示`。菜单定位统一走 `utils/menuPosition.ts` 的 `clampMenuPosition` |
| [ResultTableBody.vue](file:///C:/works/go_works/meteor-ch-workbench/frontend/src/components/ResultTableBody.vue) | 表格主体：表头、`@tanstack/vue-virtual` 虚拟滚动、单元格渲染与行内编辑输入框。行高固定 28px；暴露 `scrollEl` / `headerScrollEl` 供父组件桥接横向滚动同步 |
| [ResultToolbarInfo.vue](file:///C:/works/go_works/meteor-ch-workbench/frontend/src/components/ResultToolbarInfo.vue) | 工具栏左侧信息区：行数、SQL 预览按钮、搜索框、刷新倒计时；`defineExpose({ sqlButtonWrap })` 供父组件定位 SQL 面板 |
| [ResultToolbarActions.vue](file:///C:/works/go_works/meteor-ch-workbench/frontend/src/components/ResultToolbarActions.vue) | 工具栏右侧按钮组：刷新 / 自动刷新 / Explain 模式 / 导出 / 筛选 |
| [ResultCellMenu.vue](file:///C:/works/go_works/meteor-ch-workbench/frontend/src/components/ResultCellMenu.vue) | 单元格右键菜单（复制值 / 字面量 / 行 / INSERT / 按值筛选） |
| [ResultHeaderMenu.vue](file:///C:/works/go_works/meteor-ch-workbench/frontend/src/components/ResultHeaderMenu.vue) | 表头右键菜单（复制列名 / 列值、仅本列搜索、排序、按值筛选、切换编辑定位列、隐藏此列、隐藏此列（改写 SQL）、恢复全部隐藏列）。`is-key` 基于 `keyIndexes.includes(colIndex)`，多列自动定位时同样显示勾选态（P11-3）；`hiddenCount` 为 0 时「恢复全部」置灰；`hide-column-sql` 事件交给 `ResultGrid` 用 `rewriteProjectionToColumns` 改写后走 `rerun` |
| [ResultCellFilterPanel.vue](file:///C:/works/go_works/meteor-ch-workbench/frontend/src/components/ResultCellFilterPanel.vue) | 按值筛选弹层（DBeaver 式候选值 + 自定义操作符）；标题栏可拖拽（`usePanelDrag`），拖拽中不触发外部点击关闭 |
| [ResultSqlPanel.vue](file:///C:/works/go_works/meteor-ch-workbench/frontend/src/components/ResultSqlPanel.vue) | 已执行 SQL 预览面板（原始 / 已修改标识、复制、关闭）；标题栏可拖拽 |
| [ResultAutoRefreshPicker.vue](file:///C:/works/go_works/meteor-ch-workbench/frontend/src/components/ResultAutoRefreshPicker.vue) | 自动刷新下拉 + 自定义秒数弹窗；定时器与触发仍留在 `useAutoRefresh` |
| [ResultCellEditBar.vue](file:///C:/works/go_works/meteor-ch-workbench/frontend/src/components/ResultCellEditBar.vue) | 底部待编辑条：计数、填入编辑器 / 丢弃 / 应用修改 |
| [ResultApplyConfirmModal.vue](file:///C:/works/go_works/meteor-ch-workbench/frontend/src/components/ResultApplyConfirmModal.vue) | 应用修改确认弹窗（P11-1）：列出全部待执行 `ALTER ... UPDATE` 语句、目标表与条数，提供复制 SQL / 仅填入编辑器 / 确认执行；执行中主按钮禁用 |
| [ResultExportBar.vue](file:///C:/works/go_works/meteor-ch-workbench/frontend/src/components/ResultExportBar.vue) | 导出格式选择行（格式列表来自 `GetResultExportFormats()`，前端自动感知）；导出中显示 `editor.exportingRows` 进度文案（带命中行数），完成后显示导出路径 |
| [ResultFilterPanel.vue](file:///C:/works/go_works/meteor-ch-workbench/frontend/src/components/ResultFilterPanel.vue) | 多条件筛选面板（Navicat 风格）：条件组增删、组间 AND / OR、规则增删与操作符切换、数值 / 文本模式切换、仅读 `WHERE` 预览、重置 / 应用。纯 UI，状态由 `ResultGrid` 持有；props 收 `visible / columns / conditions / error / applyDisabled`，emit `close / update:conditions / update:error / apply / clear`（`v-model:conditions` 与 `v-model:error`）。所有变更均以不可变方式回传新数组，不修改父级 props；列类型经 `isNumericType` 判定数值列并自动置 `numeric` |
| [DatabaseTree.vue](file:///C:/works/go_works/meteor-ch-workbench/frontend/src/components/DatabaseTree.vue) | 左侧数据库树：库 / 表 / 视图 / 集群分区、表与库右键菜单、DDL 查看 / 建表 / 编辑 / DROP 全流程。props 收 `active / selectedDb / selectedTable`，emit `preview / select-db / append-uuid / insert-ddl / error`，暴露 `closeMenus` / `reload`；内部 `watch(activeConnectionId, immediate)` 自动加载库与集群 |
| [HistoryPanel.vue](file:///C:/works/go_works/meteor-ch-workbench/frontend/src/components/HistoryPanel.vue) | 查询历史面板：列表、单条删除、清空、时间格式化、加载 / 空态。props 收 `visible`，emit `select`（回填 SQL）与 `close`，暴露 `reload`；内部 `watch(visible, immediate)` 自动拉取历史 |
| [SavedSQLPanel.vue](file:///C:/works/go_works/meteor-ch-workbench/frontend/src/components/SavedSQLPanel.vue) | 已收藏 SQL 面板：列表、关键字过滤（标题 + SQL）、内联重命名、单条删除、时间格式化、加载 / 空态 / 无匹配态。props 收 `visible / connectionId`，emit `select / error / close`，暴露 `reload`；内部 `watch(visible, immediate)` 自动拉取。与 HistoryPanel 同构，无「清空」按钮 |
| [MenuBar.vue](file:///C:/works/go_works/meteor-ch-workbench/frontend/src/components/MenuBar.vue) | 顶栏：连接选择器、语言切换、菜单；Help 菜单下含快捷键 / 查询设置 / 关于弹窗，查询设置弹窗读写 `GetSettings` / `SaveSettings`（行数上限 + 不限制勾选） |
| [ConnectionModal.vue](file:///C:/works/go_works/meteor-ch-workbench/frontend/src/components/ConnectionModal.vue) | 新建/编辑连接表单 |
| [FolderModal.vue](file:///C:/works/go_works/meteor-ch-workbench/frontend/src/components/FolderModal.vue) | 分组表单 |
| [CreateTableModal.vue](file:///C:/works/go_works/meteor-ch-workbench/frontend/src/components/CreateTableModal.vue) | 可视化建表 + SQL 预览 |
| [EditTableModal.vue](file:///C:/works/go_works/meteor-ch-workbench/frontend/src/components/EditTableModal.vue) | 加载 schema → MOD/REN/DROP/ADD → ALTER |

### composables/

| 文件 | 职责 |
|---|---|
| [useMonaco.ts](file:///C:/works/go_works/meteor-ch-workbench/frontend/src/composables/useMonaco.ts) | Monaco 编辑器生命周期：空 Blob Worker 的 `MonacoEnvironment` 安装、`editor.api.js` + SQL 语言包懒加载、`ch-dark` 主题、`Ctrl+Enter` / 内容变化 / 选区变化三个回调桥接（`onRun` / `onContentChange` / `onSelectionChange`）、`onBeforeUnmount` 自动 `dispose`。对外暴露 `editorRef` / `editor` / `initEditor(initialValue)` / `getEditor()` / `disposeEditor()` |
| [useCellEditing.ts](file:///C:/works/go_works/meteor-ch-workbench/frontend/src/composables/useCellEditing.ts) | 单元格编辑状态机（P11-4 自 `ResultGrid` 抽出）：编辑中单元格 / 待编辑列表、定位列（`keyIndexes` + `keyStrategy` + `editKeyHint`）、`commitEditCell` 生成 `ALTER ... UPDATE`、应用确认（`applyPendingEdits` / `confirmApplyEdits`），以及 P11-2 的失败回执——`applyResult` 报错且 `ok` 已知时，仅把**未执行**部分回填到 `pendingEdits` |
| [useResultFilterPanel.ts](file:///C:/works/go_works/meteor-ch-workbench/frontend/src/composables/useResultFilterPanel.ts) | 多条件筛选 + 按值筛选（P11-4 抽出）：条件组状态、`applyFilters` / `applyFilterPanel` / `applyCandidate` 拼接 `WHERE` 后 `onRerun`，以及候选值 `filterCandidates`（去重 + 关键字过滤 + 上限 12 条） |
| [useAutoRefresh.ts](file:///C:/works/go_works/meteor-ch-workbench/frontend/src/composables/useAutoRefresh.ts) | 结果区自动刷新（P11-4 抽出）：秒数持久化（`autoRefresh` 纯函数 + localStorage）、`setInterval` 倒计时、秒数到点触发 `onRefresh`、查询结束重启定时器、下拉菜单开关与自定义秒数弹窗状态 |

### locales/

[zh.ts](file:///C:/works/go_works/meteor-ch-workbench/frontend/src/locales/zh.ts) 与 [en.ts](file:///C:/works/go_works/meteor-ch-workbench/frontend/src/locales/en.ts) **必须同步修改**，key 前缀分组：`common.*` / `menu.*` / `editor.*`。

---

## 常见改动路径速查

| 需求 | 改动点 |
|---|---|
| 新增后端 API | `app.go` → 实现层 → `api.ts` → `types.ts` → `docs/API.md` |
| 新增导出格式（配置） | `config/export.go` 注册表 + `export_test.go` |
| 新增导入格式（配置） | `config/import.go` 注册表 + `import_test.go` |
| 新增导出格式（结果集） | `clickhouse/export.go` 注册表 + `export_test.go` |
| 调整行数上限 | `clickhouse/export.go` 的 `DefaultQueryRowLimit` |
| 调整历史上限 | `config/history.go` 的 `HistoryMaxEntries` |
| 新增持久化文件（无上限类） | 照 `config/saved_sql.go`：内存缓存 + RWMutex + 全量写回 |
| 新增前端文案 | `locales/zh.ts` + `locales/en.ts` |
| 新增 Modal | `components/` 新建 → 在目标 view 加 visible state |
| 修改 DDL 生成 | `utils/ddl.ts`（纯函数，可直接单测） |

---

## 已知遗留问题

| 位置 | 问题 | 对应计划 |
|---|---|---|
| `vite.config.ts` | Monaco 已切 `editor.api.js` 精简入口并懒加载，单包 2,753 kB（gzip 716 kB），属编辑器核心固有体积；`chunkSizeWarningLimit` 已调至 3000，构建无警告 | P6-4（已完成） |
| `connections.json` | 密码明文存储 | P5（已主动跳过） |
| `README.md` / `docs/API.md` | 修改公开方法后需手动同步，无自动化校验 | 无 |

## 原生 GUI 迁移新增目录

### `internal/app/`

| 文件 | 职责 |
|---|---|
| `service.go` | 与 GUI 框架无关的应用服务入口；编排连接管理、SQL 查询/取消/历史、DDL/DML 执行和数据库元数据读取，并由 Gio 与 Wails 共享。 |

### `docs/architecture/`

| 文件 | 职责 |
|---|---|
| `NATIVE_GUI_MIGRATION.md` | Gio 原生 GUI 迁移目标分层、当前迁移边界、阶段顺序与验收原则。 |


### [internal/gui/gioapp/window.go](../internal/gui/gioapp/window.go)

职责：Gio 原生工作台窗口与交互状态。

- 异步连接测试、数据库列表和表 / 视图加载；使用请求序号过滤过期响应。
- SQL 编辑器内容通过 `internal/app.Service.Query` 执行，生成独立 queryID；停止操作调用 `CancelQuery`。
- 结果区显示列名、结果行、行数、耗时、行数上限、截断状态和 SQL 错误；结果行使用纵向 Gio 列表滚动，表头和各结果行使用共享滚动位置的横向列表，保持列对齐。
- 结果表格首列提供「复制行」操作；点击单元格复制当前值，点击行首操作复制整行，使用带引号转义的制表符分隔文本。
- 表头各数据列末尾提供拖动手柄，列宽在 80–480dp 范围内调整；调整后的宽度同步应用于表头和所有结果行，拖动距离按设备像素换算为 dp。
- 查询结果摘要行提供「导出 CSV」和「导出 TSV」按钮；使用 `gioui.org/x/explorer` 系统保存对话框，后台打开文件并通过 `internal/app.Service.ExportQueryResult` 将结果流式写入文件，结束后关闭文件并回到 Gio 事件循环更新状态。取消保存不报错，写入 / 关闭失败会显示错误。
- 导出复用当前已加载的 `QueryResult.Rows`，不会重新执行 SQL；结果被行数上限截断时只导出已加载行，界面明确提示。序列化过程按 Writer 流式写出，但尚未实现数据库游标级全量流式查询。
- `resultColumnWidth` 将列宽权重转换为固定 dp 宽度；列宽权重仍由列名和前 30 行样本估算。
- 表结构和 ClickHouse 存储元数据显示在可隐藏的右侧详情面板；字段与 DDL 分别使用独立 Gio 列表纵向滚动，DDL 复制通过 `gioui.org/io/clipboard.WriteCmd` 执行。
- `splitDDLLines` 统一 LF / CRLF 换行并保留空行；`schema_test.go` 覆盖空 DDL、LF、CRLF 和空行场景。
- `widget.Editor` 目前仍是基础编辑器，语法高亮、补全和大 SQL 性能尚未验收。

### [internal/gui/gioapp/query_test.go](../internal/gui/gioapp/query_test.go)

职责：验证查询结果单元格显示格式、复制完整值、整行制表符分隔与引号转义、自动列宽边界和拖动列宽上下限。

### [internal/app/service.go](../internal/app/service.go)

职责：GUI 无关的应用服务编排，供旧 Wails 适配层与后续 Gio 界面共同调用。

| 能力 | 说明 |
|---|---|
| 连接与查询 | 连接保存/删除/测试/断开、查询/取消、DDL/DML、批量执行 |
| 元数据 | 数据库、表、视图、表结构、集群拓扑 |
| 配置用例 | 连接与文件夹读取、排序、配置导入导出 |
| 工作区 | 查询历史、收藏 SQL、标签页快照 |
| 用户设置 | 设置读取/保存，查询行数上限仍通过设置管理器生效 |

**边界**：本包不得依赖 Wails、Gio、WebView 或前端运行时。文件选择、保存对话框和系统打开目录已通过 `internal/platform` 端口抽离；Gio 专属剪贴板交互由 `internal/gui/gioapp` 直接使用 Gio clipboard command，不进入 GUI 无关的应用服务。


### [internal/platform](../internal/platform/platform.go)

职责：定义 GUI 适配层与本地文件系统需要的能力端口，不依赖具体 GUI 框架。

- `Platform.SaveFile` / `Platform.OpenFile`：保存与打开文件对话框。
- `Platform.OpenDirectory`：使用系统文件管理器打开目录。
- `FileSystem.ReadFile` / `WriteFile` / `Create`：读取配置、写出配置与流式创建结果文件。
- `internal/platform/wailsadapter`：当前 Wails v3 对话框与系统操作实现。
- `internal/platform/osfs`：基于 Go 标准库的文件系统实现，可在 Gio 入口复用。


### [internal/app/service.go](../internal/app/service.go)

职责：与 GUI 无关的应用服务编排，供 Wails 兼容层和后续 Gio 界面共同调用。

- 连接管理、SQL 查询/取消、DDL/DML 执行、元数据查询。
- 配置导入导出、连接文件夹排序、查询历史、收藏 SQL、标签页快照和用户设置。
- 查询结果通过 `io.Writer` 导出，不负责选择文件路径。
- `Close` 统一释放 ClickHouse 连接池。

### [internal/platform/platform.go](../internal/platform/platform.go)

职责：定义桌面平台能力端口，不绑定具体 GUI 框架。

- `Platform.SaveFile`：保存文件对话框。
- `Platform.OpenFile`：打开文件对话框。
- `Platform.OpenDirectory`：使用系统文件管理器打开目录。

### [internal/platform/wailsadapter/adapter.go](../internal/platform/wailsadapter/adapter.go)

职责：将系统对话框与目录打开能力适配到 Wails v3。未来 Gio 入口可实现相同接口，应用服务无需改动。


### [internal/platform/wailsadapter/adapter_test.go](../internal/platform/wailsadapter/adapter_test.go)

职责：验证 Wails 文件对话框取消错误的归一化判断，覆盖空错误、英式与美式取消拼写、大小写差异和非取消错误。该测试不启动 Wails 应用；系统对话框交互仍需在真实桌面环境中补充集成验证。
