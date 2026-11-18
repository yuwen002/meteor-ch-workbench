# 代码索引

> 按文件列出职责、关键入口与依赖关系，便于人和 AI 快速定位。
> 与 `.trae/PROJECT.md`（AI 检索地图）互补：本文更偏「文件清单」，PROJECT.md 更偏「怎么改」。
> 更新时间：2026-11-18

---

## 仓库总览

```
meteor-ch-workbench/
├── main.go              Wails 应用入口
├── app.go               所有前端可调用方法的门面（39 个）
├── wails.json           Wails 打包配置
├── go.mod / go.sum      Go 依赖
├── internal/
│   ├── clickhouse/      数据库交互层（service.go / export.go）
│   └── config/          持久化层（manager / export / import / history / tabs）
├── frontend/
│   └── src/             Vue 3 前端
│       ├── main.ts      应用装配（Pinia / Router / i18n / 自定义指令）
│       ├── App.vue      应用外壳：侧边栏 + 菜单栏 + 全局 Modal/右键菜单宿主
│       ├── api.ts       Wails 绑定唯一封装点
│       ├── types.ts     与后端 struct 一一对应的 TS 类型
│       ├── i18n.ts      中英切换 + localStorage 记忆
│       ├── router/      仅一条路由 → QueryEditor
│       ├── stores/      Pinia（app.ts 唯一）
│       ├── utils/       纯函数（ddl.ts / tabs.ts / resultTable.ts / sqlLiteral.ts）
│       ├── views/       QueryEditor（唯一视图）
│       ├── components/  5 个 Modal 与 MenuBar
│       └── locales/     zh.ts / en.ts
├── docs/                API.md / FEATURES.md / CODE_INDEX.md
└── .trae/               PROJECT.md / rules/
```

依赖方向是单向的：

```
app.go
  ├─► internal/clickhouse   ──► internal/config（仅引用 Connection 类型）
  └─► internal/config
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

职责：**前端调用的唯一入口**，负责编排 config 与 clickhouse 两个包。

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
| 历史 | `GetQueryHistory` / `DeleteQueryHistory` / `ClearQueryHistory` / `recordHistory`（私有） | `recordHistory` 在 `Query` 后自动调用 |
| 标签 | `GetTabs` / `SaveTabs` | |
| 元数据 | `GetDatabases` / `GetTables` / `GetViews` / `GetTableSchema` / `GetClusters` | |
| 其他 | `OpenConfigFolder` / `GetAppVersion` | |

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
| `buildDSN(conn)` | 函数 | 由 protocol + secure 决定 scheme |
| `scanRows(ctx, rows, limit)` | 函数 | 逐行检查 ctx，使取消能在一行内生效 |
| `errQueryCanceled` | 常量 | `"查询已取消"`，前端按此文案识别取消 |
| `isCanceled(ctx)` | 函数 | 只认 `context.Canceled`，超时不算用户取消 |
| `QueryResult` | struct | `columns` 现为 `[]ColumnInfo`（含 name/type/comment） |
| `ColumnInfo` / `TableInfo` / `ClusterInfo` | struct | 元数据返回结构 |
| `DefaultQueryRowLimit` | 常量 | 见 export.go，50000 |

**重要设计说明**：文件头注释解释了为什么用 DSN 而非 `clickhouse.Open` + `Options.Protocol` —— 后者存在协议路由缺陷，指定 HTTP 仍发 Native 握手包。**这是踩过坑的记录，改动建连方式前务必读完那段注释。**

### [internal/clickhouse/export.go](file:///C:/works/go_works/meteor-ch-workbench/internal/clickhouse/export.go)

职责：结果集导出注册表 + 行数上限常量。

| 符号 | 说明 |
|---|---|
| `DefaultQueryRowLimit = 50000` | `Query` 的行数上限 |
| `resultExportRegistry` | 私有注册表：`{Meta, Render}` |
| `ResultExportFormats()` | 返回元信息列表，供前端下拉 |
| `ExportResultByFormat(format, columns, rows)` | 按 ID 分发到 Render，返回 `(文本, 扩展名, error)` |
| `renderResultCSV` / `renderResultJSON` / `renderResultMarkdown` | 具体渲染实现 |
| `escapeMarkdownCell` | Markdown 单元格转义：`\` 优先双写 → `|` 转 `\|` → 换行统一转 `<br>` |

**扩展方式**：向 `resultExportRegistry` 追加一项即可，前端自动感知。补测试到 [export_test.go](file:///C:/works/go_works/meteor-ch-workbench/internal/clickhouse/export_test.go)。

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

### [internal/config/tabs.go](file:///C:/works/go_works/meteor-ch-workbench/internal/config/tabs.go)

职责：标签页快照持久化（仅标题 + SQL + activeId，不存结果）。

| 符号 | 说明 |
|---|---|
| `TabsFile` | `{tabs, activeId}` |
| `TabsManager` | 沿用 Manager 模式 |

---

## 后端测试

| 文件 | 覆盖重点 |
|---|---|
| [export_test.go](file:///C:/works/go_works/meteor-ch-workbench/internal/config/export_test.go) | 5 格式渲染、脱敏**不改源数据**、转义、往返一致性 |
| [import_test.go](file:///C:/works/go_works/meteor-ch-workbench/internal/config/import_test.go) | DBeaver 多版本兼容、CSV 缺列/短行、DSN 非法行、合并三场景、Manager 集成 |
| [history_test.go](file:///C:/works/go_works/meteor-ch-workbench/internal/config/history_test.go) | 记录、上限裁剪、删除、清空 |
| [tabs_test.go](file:///C:/works/go_works/meteor-ch-workbench/internal/config/tabs_test.go) | 快照读写、容错 |
| [export_test.go（clickhouse）](file:///C:/works/go_works/meteor-ch-workbench/internal/clickhouse/export_test.go) | 结果集 CSV / JSON / Markdown 渲染（含 Markdown 竖线、换行、CRLF、反斜杠转义边界） |
| [cancel_test.go](file:///C:/works/go_works/meteor-ch-workbench/internal/clickhouse/cancel_test.go) | 查询取消：空 ID、未注册、重复取消、注销清理、重复注册、超时不误判 |

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
| `folders` / `connections` / `activeConnectionId` | 核心数据 |
| `connectionStatus` | 连通性缓存 |
| `groupedConnections` | computed，按分组聚合 |
| `loadAll` | 启动时拉全量 |
| `checkConnection` | 状态检测 |

### [utils/ddl.ts](file:///C:/works/go_works/meteor-ch-workbench/frontend/src/utils/ddl.ts)

**纯函数，无副作用**，前端生成 SQL、后端只执行。

| 符号 | 说明 |
|---|---|
| `CLICKHOUSE_ENGINES` | 26 种引擎 |
| `CLICKHOUSE_TYPES` | 42 种类型 |
| `buildCreateTableSQL` / `buildAlterTableSQL` / `buildDropTableSQL` | 三种 SQL 构建 |
| `EditTableChange` | 变更结构 |

### [utils/tabs.ts](file:///C:/works/go_works/meteor-ch-workbench/frontend/src/utils/tabs.ts)

`QueryTab` 类型与工厂。每个标签独立持有 `sql` / `result` / `error` / `querying` / `queryId` / `canceled` / `resultScrollTop` / `viewState`。

### [utils/resultTable.ts](file:///C:/works/go_works/meteor-ch-workbench/frontend/src/utils/resultTable.ts)

**纯函数，无副作用**，结果表格虚拟滚动与列宽计算。

| 符号 | 说明 |
|---|---|
| `ROW_HEIGHT` | 固定行高 28px，虚拟滚动估算依据，**不可改成动态高度** |
| `VIRTUAL_THRESHOLD` | 200，超过才启用虚拟滚动 |
| `measureColWidths(cols, rows)` | 取前 200 行测列宽，单字符按 7px，限制 80–420px |
| `buildGridStyle(widths, count)` | 生成 `grid-template-columns`，宽度缺失时回退均分 |
| `shouldVirtualize(rowCount)` | 阈值判断 |

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

### views/

只有 [QueryEditor.vue](file:///C:/works/go_works/meteor-ch-workbench/frontend/src/views/QueryEditor.vue) 一个视图。

> 原 `ConnectionManager.vue` 为早期骨架（API 调用全部被注释），已于 2026-11-18 删除。其功能已由 `App.vue`（侧边栏 + 连接/分组 Modal + 右键菜单）完整承接。

#### [views/QueryEditor.vue](file:///C:/works/go_works/meteor-ch-workbench/frontend/src/views/QueryEditor.vue) ★

**核心视图，承载 ~90% 业务逻辑**。文件较长，按区块阅读：

| 区域 | 内容 |
|---|---|
| 顶部工具栏 | 连接选择、运行/取消按钮、历史开关、导出按钮 |
| 标签栏 | 多标签、双击重命名、关闭 |
| 左侧树 | 数据库 → 表 / 视图分区，单击预览、右键菜单 |
| 编辑器区 | Monaco（`editor.api.js` 精简入口 + SQL 语言包，均在 `onMounted` 内动态 import），`Ctrl+Enter` 执行，viewState 保存恢复 |
| 结果区 | 表格（> 200 行走虚拟滚动）、截断提示条、导出下拉（CSV / JSON / Markdown）、单元格右键菜单 |
| 历史面板 | 列表 + 单条删除 + 清空 |
| Modal 宿主 | CreateTable / EditTable / DropConfirm / DDL |
| 右键菜单 | 数据库菜单 / 表菜单，统一在此渲染 |

**关键函数**：

| 函数 | 行 | 说明 |
|---|---|---|
| `runQuery` | ~255 | 取选区或全文 → `api.Query` → 写 `tab.result` / `tab.error` |
| `exportResult` | ~290 | `api.SaveQueryResultAs`，用户取消不报错 |
| `loadHistory` | ~310 | 拉历史列表 |

### components/

| 文件 | 职责 |
|---|---|
| [MenuBar.vue](file:///C:/works/go_works/meteor-ch-workbench/frontend/src/components/MenuBar.vue) | 顶栏：连接选择器、语言切换、菜单 |
| [ConnectionModal.vue](file:///C:/works/go_works/meteor-ch-workbench/frontend/src/components/ConnectionModal.vue) | 新建/编辑连接表单 |
| [FolderModal.vue](file:///C:/works/go_works/meteor-ch-workbench/frontend/src/components/FolderModal.vue) | 分组表单 |
| [CreateTableModal.vue](file:///C:/works/go_works/meteor-ch-workbench/frontend/src/components/CreateTableModal.vue) | 可视化建表 + SQL 预览 |
| [EditTableModal.vue](file:///C:/works/go_works/meteor-ch-workbench/frontend/src/components/EditTableModal.vue) | 加载 schema → MOD/REN/DROP/ADD → ALTER |

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