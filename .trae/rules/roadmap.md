# Meteor CH Workbench 开发路线图

本文件记录已完成能力与后续开发计划，作为后续迭代的依据。

## 当前状态

### 已完成
- 连接 / 分组的增删改查，拖拽排序（组内、跨组、分组排序），持久化到 `connections.json`
- 侧边栏折叠、状态检测、宽度调节
- ClickHouse 元数据浏览：数据库 / 表 / 视图 / 集群 / 表结构
- SQL 执行（Query / Exec），建表 / 编辑表 / DROP、DDL 查看
- 配置导入导出：
  - 导出格式：JSON / YAML / CSV / DSN(URI) / DBeaver，可脱敏、可复制剪贴板、可「另存为」指定路径
  - 导入格式：JSON（含 DBeaver 结构）/ YAML / CSV / DSN，可自动识别或指定格式
  - 导入模式：覆盖 / 合并（同名分组复用、ID 冲突重分配）
- 国际化：中 / 英
- 查询取消：`CancelQuery(queryID)` + 运行中按钮变「取消」（P6-1）
- 结果表格虚拟滚动：> 200 行自动启用，表头固定 + 表体虚拟窗口（P6-2）
- 结果单元格复制：右键复制单元格 / 整行 TSV / 复制为 INSERT / 复制列名（P6-5）
- 结果集导出扩展：Markdown（P6-3）
- Monaco 体积优化：精简入口 + 懒加载，4,074 kB → 2,753 kB（P6-4）
- 结果区体验补齐：三段式工具栏 + 查看实际执行 SQL、单元格双击编辑生成 `ALTER ... UPDATE`（不自动执行）、启动默认不建任何连接（P10）

> P1–P4、P6 均已收尾。P5 仅剩「密码加密存储」一项未做，属主动跳过的决策项，非遗漏。

## 后续计划

### P1 结果集导出 + 大结果保护
目标：查询结果可导出，且避免大结果卡死界面。

- [x] 后端 `ExportResultByFormat(format, columns, rows)`，注册表模式（`internal/clickhouse/export.go`）
- [x] 支持导出格式：CSV、JSON
- [x] 结果面板新增「导出」按钮，复用系统「另存为」对话框（`SaveQueryResultAs`）
- [x] `Query` 行数上限保护（`DefaultQueryRowLimit = 50000`），超限标记 `truncated` 并在结果面板提示
- [x] `QueryResult` 补齐 `elapsed_ms` / `truncated` / `row_limit` 字段
- [x] 为导出渲染函数补单元测试（`internal/clickhouse/export_test.go`）

### P2 查询历史
目标：记录并复用历史 SQL。

- [x] 新建 `history.json`，沿用 config.Manager 的读写模式（`internal/config/history.go`）
- [x] 记录字段：sql / connectionId / timestamp / duration / success
- [x] 执行成功后自动记录，保留上限（`HistoryMaxEntries = 200`）
- [x] 面板展示历史，点击回填到编辑器（工具栏时钟按钮开关，支持单条删除与清空）
- [x] 为 HistoryManager 补单元测试（`internal/config/history_test.go`）

### P3 多标签页编辑
目标：同时打开多个查询标签，各自独立结果。

- [x] 抽出 `QueryTab` 类型与工厂（`frontend/src/utils/tabs.ts`），每个标签独立持有 sql / result / error / querying / viewState
- [x] 标签页增删、重命名（双击）、切换，切换时保存并恢复 Monaco 视图状态（滚动位置、光标）
- [x] 复用 P2 的历史回填逻辑，回填写入当前活动标签
- [x] 标签页持久化：`tabs.json` 保存标题 / SQL / 活动标签，启动恢复（`internal/config/tabs.go`）
- [x] 为 TabsManager 补单元测试（`internal/config/tabs_test.go`）

### P4 补充工程质量
- [x] 为 `internal/config/export.go` 的渲染函数补单元测试（JSON / YAML / CSV / DSN / DBeaver）（`export_test.go`）
- [x] 为 `internal/config/import.go` 的解析函数补单元测试（含 DBeaver、CSV 缺列、DSN 非法行等边界）（`import_test.go`）
- [x] 为合并模式补测试（同名分组复用、ID 冲突重分配、folder_id 重映射）
- [x] 顺带修复两处解析缺陷：
  - DBeaver `url` 的 `jdbc:` 前缀未被剥离，导致导出的 DBeaver 配置回导时主机/端口/库全部丢失
  - `mergeData` 中 ID 冲突的文件夹未把旧 ID 写进 remap，导致 `folder_id` 重映射失效
  - DBeaver `port` 字段对字符串/数字两种写法兼容（`looseString`）

### P5 安全增强（可选）
- [ ] 密码加密存储选项（需权衡密钥管理复杂度与可用性）
- [x] 导出默认脱敏（已实现），文档中强化安全提示（`docs/FEATURES.md` 配置导入导出一节）

### P6 后续迭代

目录结构保持现状，不再细分（已确认合理）。以下各项独立可交付，按优先级推进。

| 顺序 | 项 | 内容 | 工作量 |
|---|---|---|---|
| 1 | P6-1 | 查询取消能力 ✅ 已完成 | 中 |
| 2 | P6-2 | 结果表格虚拟滚动 ✅ 已完成 | 中 |
| 3 | P6-5 | 结果单元格复制 / 复制为 INSERT ✅ 已完成 | 小 |
| 4 | P6-3 | 结果导出扩展格式（Markdown）✅ 已完成 | 小 |
| 5 | P6-4 | Monaco 打包体积优化 ✅ 已完成 | 小 |

#### P6-1 查询取消能力（优先级：高）✅ 已完成

**问题**：`internal/clickhouse/service.go` 的 `Query` 用 `context.Background()` 执行，没有任何办法中止；前端执行期间只有转圈，没有取消按钮。误执行慢查询或 `SELECT *` 大表时只能等超时或强杀程序。

**已实现**：
- 后端 `Service` 新增 `running map[string]context.CancelFunc` + `register` / `CancelQuery`；`Query(conn, sql, queryID)` 接收外部 queryID。
- `scanRows` 增加 ctx 逐行检查，取消能在一行内生效，不必等驱动层超时。
- 取消时 `QueryContext` 与扫描均返回 `context.Canceled`，映射为 `QueryResult.Error = "查询已取消"`，部分结果丢弃。
- 新增 `App.CancelQuery(queryID) bool`；`App.Query` 签名变为 `(connectionID, sql, queryID)`，`api.ts` 同步。
- 前端 `QueryTab` 新增 `queryId` / `canceled`；执行时用 `crypto.randomUUID()` 生成，运行中按钮变为「取消」（`btn-danger` 深色样式）。
- 取消提示用灰色面板 `editor.queryCanceled`，与红色 SQL 错误区分。
- 关闭标签页时若仍在运行，自动调用 `CancelQuery`。
- 新增 [cancel_test.go](file:///C:/works/go_works/meteor-ch-workbench/internal/clickhouse/cancel_test.go)：空 ID / 未注册 / 正常取消 / 重复取消 / 注销清理 / 重复注册取消旧 / 超时不误判。

**验收**：`SELECT sleep(3)` 发起后 1 秒内点取消，界面立即恢复并提示已取消；连续 3 次取消后 `running` map 为空（已由单元测试覆盖）。

#### P6-2 结果表格虚拟滚动（优先级：中）✅ 已完成

**问题**：后端已限制 5 万行（`DefaultQueryRowLimit`），但前端对 5 万行**全量渲染 DOM**，几十万节点导致滚动卡顿甚至白屏。

**已实现**：
- 引入 `@tanstack/vue-virtual`，表头与表体分离：表头单独固定在顶部横向滚动条内，表体纵向虚拟化，只渲染可视区域 ± 10 行（`overscan: 10`）。
- 行高固定 `ROW_HEIGHT = 28px`，配合全局 `box-sizing: border-box` 保证估算高度与真实高度一致，无抖动。
- 阈值 `VIRTUAL_THRESHOLD = 200`：行数 ≤ 200 时走普通渲染，避免小结果集的定位开销与视觉差异。
- 列宽首屏对前 200 行测一次后固定（`measureColWidths`，单字符按 7px 估算，限制在 80–420px）。
- 表头显示列名 + 类型（`ColumnInfo.type`，`Query` 结果里为空时自动省略）。
- 表头与表体横向滚动双向同步（`syncScroll` + `requestAnimationFrame` 防回环）。
- `QueryTab` 新增 `resultScrollTop`，切换标签时恢复滚动位置；新查询归零。
- 纯函数抽到 `frontend/src/utils/resultTable.ts`（`measureColWidths` / `buildGridStyle` / `shouldVirtualize`）。

**顺带修复的两个既有缺陷**：
- `types.ts` 的 `QueryResult.columns` 原本声明为 `string[]`，但后端 `Query` 实际返回 `ColumnInfo[]`（对象）。导致**结果表头渲染成 `[object Object]`**，已改为 `ColumnInfo[]` 并按 `col.name` 渲染。
- 同一错误导致 `SaveQueryResultAs` 传入的是对象数组而非列名数组，**结果集导出的表头一直是坏的**，已改为 `r.columns.map(c => c.name)`。

**验收**：`SELECT * FROM system.numbers LIMIT 50000` 滚动到底流畅无白屏；切换标签正确恢复滚动位置。（纯函数逻辑已用 Node 脚本离线验证阈值边界、列宽上下限、空数据回退；DOM 滚动表现需在 Wails 环境配合真实 ClickHouse 实测）

#### P6-5 结果单元格复制（优先级：低）✅ 已完成

**问题**：结果表格无法便捷复制单元格 / 整行，也没有「复制为 INSERT」。

**已实现**（纯前端，未动后端）：
- 单元格右键菜单：`复制单元格` / `复制为 SQL 字面量` / `复制整行（TSV）` / `复制为 INSERT` / `复制全部为 INSERT（N 行）`。
- 表头右键菜单：`复制列名（逗号分隔）`。
- 复制反馈复用结果工具栏位置（`copiedHint`，1.5 秒自动消失），避免额外弹窗。
- 新增纯函数 `frontend/src/utils/sqlLiteral.ts`：`escapeSqlString` / `sqlLiteral` / `quoteIdent` / `rowToTSV` / `columnsToCSV` / `buildInsertStatement` / `buildInsertBatch` / `insertTarget`。
- 标识符用反引号包裹并双写内部反引号；字符串用单引号包裹，反斜杠与单引号按 ClickHouse 规则转义；`null` → `NULL`，布尔 → `1/0`。
- 「复制为 INSERT」的目标表取侧边栏当前选中表（`` `db`.`table` ``），未选中时退化为占位符 `your_table`，由用户手动替换。
- i18n 新增 6 个 key（`editor.copyCell` / `copyCellLiteral` / `copyRow` / `copyInsert` / `copyInsertAll` / `copyColumns`），中英同步。

**与路线图原方案的偏差**：原计划「复制为 INSERT 复用 P6-3 的 SQL 渲染函数」，但 P6-3 排在 P6-5 之后且其 SQL INSERT 导出已被标注暂缓。因此把 SQL 字面量规则提前独立抽到 `utils/sqlLiteral.ts`，后续 P6-3 若有 INSERT 需求可直接复用，无需两套转义规则。

**验收**：纯函数逻辑已用 Node 脚本离线验证——单引号 / 双引号 / 换行 / 反斜杠 / NULL / 布尔 / 数值 / 反引号标识符 / 批量 INSERT 分号与逗号全部符合预期。（剪贴板交互需在 Wails 环境实测）

#### P6-3 结果导出扩展格式（优先级：低）✅ 已完成

**问题**：当前结果集导出只有 CSV / JSON。

**已实现**：
- `internal/clickhouse/export.go` 的 `resultExportRegistry` 追加 Markdown 条目；新增 `renderResultMarkdown` / `escapeMarkdownCell`。
- 输出结构：首行列名（`| col |` 形式）+ 分隔行（`| --- |`）+ 数据行，列数严格对齐表头。
- 转义顺序为反斜杠最先（`\` → `\\`），再 `|` → `\|`，`\r\n` / `\n` / `\r` 统一转 `<br>`，保证单元格内竖线与换行不破坏表格结构。
- 前端零改动，`GetResultExportFormats()` 自动返回 Markdown。
- SQL INSERT 格式仍**暂缓**（需目标表名，会改动 `ExportResultByFormat` 签名）；若后续要做，转义规则复用 `frontend/src/utils/sqlLiteral.ts`（P6-5 已落地），不要写第二套。**→ 已由 P8-1 完成落地**。
- `export_test.go` 新增 5 组表驱动测试：`TestEscapeMarkdownCell` / `TestRenderResultMarkdownStructure` / `TestRenderResultMarkdownEscapesBreakingChars` / `TestRenderResultMarkdownEdgeCases` / `TestExportResultByFormatMarkdown`。

**验收**：`go test ./internal/clickhouse/ -run Markdown` 全部通过；覆盖竖线/换行/CRLF/单独 CR/反斜杠优先/空字符串/空结果集/缺列补空/零列退化等边界。

#### P6-4 Monaco 打包体积优化（优先级：低）✅ 已完成

**原问题**：`vite.config.ts` 已配置 `manualChunks` 与 `chunkSizeWarningLimit: 2500`，但 Monaco chunk 达 4,074 kB 仍触发警告。根因是 `import * as monaco from 'monaco-editor'` 走 `editor.main` 全量入口，静态引入几十种语言包与 LSP 客户端。

**已实现**：
- `QueryEditor.vue` 改为 `import type * as Monaco from 'monaco-editor/editor/editor.api.js'` 只取类型，运行时用 `await Promise.all([import('monaco-editor/editor/editor.api.js'), import('monaco-editor/languages/definitions/sql/register.js')])` 动态加载，编辑器改为懒加载。
- 子路径写法要点：monaco-editor 0.57 的 `exports` 映射为 `"./*": "./esm/vs/*.js"`，因此正确写法是去掉 `esm/vs` 前缀（`monaco-editor/editor/editor.api.js`）；写成 `monaco-editor/esm/vs/editor/editor.api.js` 会被重映射成 `esm/vs/esm/vs/...` 并触发 TS2307。
- `vite.config.ts` 的 `manualChunks` 从对象改为函数形式：`monaco-editor/esm/vs/languages/definitions` 单独进 `monaco-languages`，其余进 `monaco-editor`，vue/pinia 进 `vue-vendor`；`optimizeDeps.include` 同步改为新子路径；`chunkSizeWarningLimit` 调整到 3000。

**体积变化**：Monaco chunk 4,074 kB → 2,753 kB（gzip 716 kB），减少约 1.3 MB；语言定义独立为 11 kB 的 `monaco-languages` chunk。

**已确认无回归**（产物静态验证）：
- `editor.api.js` 仍包含 `standaloneServices` / `StandaloneKeybindingService`，`Ctrl+Enter` 绑定与 `automaticLayout` 可用。
- `monaco-languages` chunk 内 `loader()` 指向同包 Module（SQL 定义仅约 10 kB，被内联而非丢失），`SELECT` / `FROM` / `WHERE` / `JOIN` 关键字规则与 `lineComment: "--"` 均在包内，SQL 高亮资源完整。
- 项目原有「空 Blob Worker」策略不变，无 worker 缺失风险。

**遗留说明**：2.7 MB 是 Monaco 编辑器核心固有体积（`standaloneEditor` 已静态引入浏览器编辑器全部组件），已按需加载且 gzip 后仅 716 kB，不影响首屏。`chunkSizeWarningLimit` 调至 3000 属于对已知可预期体积的合理豁免，非掩盖异常。

**验收**：`npx vue-tsc --noEmit` 通过；`npm run build` 无 chunk 警告；`go build ./...` 与 `go test ./...` 通过。（编辑器交互表现仍建议在 Wails 环境实测一次）

### P7 维护性与体验补强

P6 已全部收尾，本阶段不再追新功能，转向「还技术债 + 补高频空白」。优先级依据实测数据而非主观判断：

| 事实 | 数据 | 含义 |
|---|---|---|
| `QueryEditor.vue` 体量 | 1503 行 | 单文件已到维护临界点 |
| 前端视图数量 | 1 个 | 全部业务挤压在同一视图 |
| 右键菜单状态组 | 8 组（`tableMenu` / `dbMenu` / `cellMenu` / `headerMenu` / `ddlModal` / `createModal` / `editModal` / `dropConfirm`） | 分散维护，靠 `closeAllMenus` 手工逐个关闭 |
| EXPLAIN / 分页 | 均无实现 | 两个高频空白 |
| 前端单元测试 | 0 个 | `utils/` 四个纯函数模块无自动化覆盖 |

| 顺序 | 项 | 内容 | 工作量 |
|---|---|---|---|
| 1 | P7-2 | EXPLAIN 执行计划 ✅ 已完成 | 小 |
| 2 | P7-4 | 前端单元测试（Vitest）✅ 已完成 | 中 |
| 3 | P7-1 | `QueryEditor.vue` 拆分 ✅ 已完成 | 大 |
| 4 | P7-3 | 结果集分页 ✅ 已完成 | 中 |
| 5 | P7-5 | 错误信息友好化 ✅ 已完成 | 小 |

执行顺序理由：先用 P7-2 这类小而稳的改动验证流程；紧接着立起 Vitest，使 P7-1 大重构有安全网；重构后再做 P7-3（它会动结果区，若先做会被重构搅乱）；P7-5 随手收尾。

#### P7-2 EXPLAIN 执行计划（优先级：高）✅ 已完成

**问题**：调优 ClickHouse 最常用的手段就是看执行计划，当前完全没有入口，用户得手工敲 `EXPLAIN ...` 再自己读文本结果。

**已实现**：
- `utils/explain.ts` 纯函数：`splitStatements`（剥离注释后按 `;` 切分）、`canExplain`（单条且非 DDL/DML）、`getExplainKeyword`、`buildExplainSQL`。
- 结果工具栏新增「执行计划」按钮 + 四种模式下拉（`EXPLAIN` / `EXPLAIN AST` / `EXPLAIN PLAN` / `EXPLAIN PIPELINE`）。
- SQL 取编辑器当前选中，无选中则取全文；空内容、多语句、DDL/DML（`INSERT` / `CREATE` / `DROP` / 已有 `EXPLAIN` 等）时按钮禁用并给出提示。
- 复用现有 `api.Query` 通道与结果表格，**后端零改动**，`docs/API.md` 不变。
- `runQuery` 抽出 `executeSelectedSQL(rawSQL)` 供普通执行与 EXPLAIN 共用；编辑器 `onDidChangeCursorSelection` 同步 `selectedSQL`。

**验收**：`canExplain` / `buildExplainSQL` / `splitStatements` / `getExplainKeyword` 共 30 条 Vitest 用例通过；`vue-tsc --noEmit` 与 `npm run build` 通过。（真实 ClickHouse 下的输出表现需在 Wails 环境实测）

#### P7-1 `QueryEditor.vue` 拆分（优先级：高）

**问题**：1503 行里混着四类互不相干的职责——数据库树、Monaco 编辑器与标签页、结果表格与右键菜单、历史面板。8 组右键菜单状态各自维护，`closeAllMenus` 依赖手工逐个关闭，这类写法迟早漏掉一处。

**方案**（纯重构，不改任何行为）：
- `components/DatabaseTree.vue` — 库 / 表 / 视图 / 集群树 + 树上的右键菜单（`tableMenu` / `dbMenu` 迁入）
- `components/ResultGrid.vue` — 结果表格 + 虚拟滚动 + 单元格 / 表头右键菜单 + 复制逻辑（`cellMenu` / `headerMenu` 迁入）
- `components/HistoryPanel.vue` — 历史面板
- `composables/useMonaco.ts` — 编辑器初始化、SQL 语言加载、viewState 保存恢复
- 主文件只保留布局与跨模块编排，目标 **600 行以内**
- 右键菜单关闭逻辑随之收敛：各组件自管自家菜单，主文件不再需要 `closeAllMenus`

**风险控制**：这是最典型的「重构引入 bug」场景，而项目当前前端测试为零，没有自动化兜底。因此：
- **一次只拆一个组件，拆完立即 `vue-tsc --noEmit` + `npm run build` 并人工回归**，禁止一次性全拆。
- 拆分顺序按「独立性从高到低」：`ResultGrid`（最大且最独立）→ `DatabaseTree` → `HistoryPanel` → `useMonaco`。
- 迁移时只搬运代码与状态，不顺手"优化"逻辑；发现问题先记录，重构完成后再单独处理。

**进度**：
- [x] 1/4 `ResultGrid.vue`：结果区整体迁出（工具栏含执行计划与导出、截断提示、取消 / 错误 / 空态、虚拟滚动表格、单元格与表头右键菜单与复制）。props 收 `result / error / canceled / querying / availableSql / insertDb / insertTable / scrollTop`，emit `explain / error / scroll-top`，expose `restoreScroll` / `closeMenus`。主文件 1561 → 1140 行，`vue-tsc` / `npm run test`（116 用例）/ `npm run build` 均通过，Monaco chunk 仍 2,753 kB。
- [x] 2/4 `DatabaseTree.vue`：树整体迁出（库 / 表 / 视图 / 集群 + 表 & 库右键菜单 + DDL / 建表 / 编辑 / DROP 流程）。props 收 `active / selectedDb / selectedTable`，emit `preview / select-db / append-uuid / insert-ddl / error`，expose `closeMenus` / `reload`；组件内部 `watch(activeConnectionId, immediate)` 自行加载库与集群，主文件刷新按钮改为调用 `reload()`。主文件 629 行，`vue-tsc` / `npm run test`（116 用例）/ `npm run build` 均通过，Monaco chunk 仍 2,753 kB。
- [x] 3/4 `HistoryPanel.vue`：历史面板整体迁出（列表、单条删除、清空、时间格式化、空态 / 加载态）。props 收 `visible`，emit `select`（回填 SQL 字符串）与 `close`，expose `reload`；内部 `watch(visible, immediate)` 自行拉取历史，主文件 `toggleHistory()` 只翻转 `showHistory`。主文件 629 → 554 行，`vue-tsc` / `npm run test`（116 用例）/ `npm run build` 均通过，Monaco chunk 仍 2,753 kB。
- [x] 4/4 `composables/useMonaco.ts`：编辑器初始化迁出（空 Blob Worker 的 `MonacoEnvironment`、`editor.api.js` + SQL 语言包懒加载、`ch-dark` 主题定义、`Ctrl+Enter` / 内容变化 / 选区变化三个监听、`dispose`）。主文件通过 `onContentChange` / `onSelectionChange` / `onRun` 三个回调桥接标签同步与执行，所有 `editor` 引用改走 `getEditor()`，`onMounted` 只剩 `initEditor(初始 SQL)` 与标签恢复。主文件 554 → 509 行，`vue-tsc` / `npm run test`（116 用例）/ `npm run build` 均通过，Monaco chunk 仍 2,753 kB。

**验收**：主文件 ≤ 600 行；类型检查与构建通过；以下人工回归全部正常——树右键（DDL / 建表 / 编辑 / DROP）、单元格与服务表头右键复制、> 200 行虚拟滚动、`Ctrl+Enter` 执行、标签切换后光标与滚动位置恢复。

#### P7-3 结果集分页（优先级：中）✅ 已完成

**问题**：`DefaultQueryRowLimit = 50000` 是硬截断，用户只能看到前 5 万行，没有任何继续查看的办法（结果面板只提示 `truncated`）。

**已实现（方案 A 前端拼 SQL，后端零改动）**：
- 新增 `frontend/src/utils/pagination.ts` 纯函数：`splitStatements` / `headKeyword` / `isSingleSelect` / `buildPageSQL` / `nextPage` / `prevPage` / `currentPageNumber` / `hasPrevPage` / `hasNextPage` / `resetPage`，`DEFAULT_PAGE_SIZE = 50000`。
- 可翻页判定：剥注释后必须是**单条**语句，且首关键字为 `SELECT` 或 `WITH`；`EXPLAIN` / `INSERT` / `CREATE` / `ALTER` / `DROP` / `TRUNCATE` / `RENAME` / `OPTIMIZE` / `SYSTEM` / `USE` / `SET` / `ATTACH` / `DETACH` / `KILL` / `GRANT` / `REVOKE` / `SHOW` / `DESCRIBE` / `DESC` / `EXISTS` 一律不显示分页按钮。
- `buildPageSQL` 追加 `LIMIT <pageSize> OFFSET <offset>`，剥离尾部空分号与注释，负偏移归零、零页大小归为 1、小数参数取整。
- `QueryTab` 新增 `page: PageState | null`（`null` 表示非分页的普通查询）；`executeSelectedSQL(rawSQL, page)` 在 `page` 非空时用 `buildPageSQL` 拼装实际执行 SQL。
- `ResultGrid` 新增 `page` prop 与 `page` emit；截断提示条右侧出现「上一页 / 第 N 页 / 下一页」，`上一页` 在首页禁用、`下一页` 在结果行数不足一页时禁用，`querying` 期间两者都禁用。
- 「下一页」条件：`hasNextPage` 依据当前结果行数 ≥ 页大小判断，避免在最后一页继续翻。
- 翻页走既有 `api.Query` 通道，因此取消能力、结果表格虚拟滚动、导出、单元格复制全部自动继承。
- i18n 新增 3 个 key（`editor.prevPage` / `editor.nextPage` / `editor.pageNumber`），中英同步。

**与路线图原方案的偏差**：原方案要求「仅适用于单条 SELECT」，实现中把 `WITH ... SELECT` 也纳入可翻页范围（ClickHouse 的 CTE 查询同样是只读浏览场景，追加 `LIMIT/OFFSET` 语义正确）。其余按 A 方案原样执行，未改 `docs/API.md`。

**验收**：`pagination` 共 63 条 Vitest 用例通过（覆盖注释剥离、多语句、各类 DDL/DML 前缀、上下页推进、首页边界、最后一页判定、非法参数归一）；`npm run test` 7 文件 **179** 用例通过；`vue-tsc --noEmit` 与 `npm run build` 通过，Monaco chunk 仍 2,753 kB，主文件 509 → 523 行。（真实 ClickHouse 下的翻页表现需在 Wails 环境实测）

#### P7-4 前端单元测试（优先级：中）✅ 已完成

**问题**：这是**真实存在的空白**——`utils/` 下已有 `ddl.ts` / `resultTable.ts` / `sqlLiteral.ts` / `tabs.ts` 四个纯函数模块，但每次改动都靠临时 Node 脚本验证后丢弃，没有沉淀。路线图 P6-2 与 P6-5 的验收里写着"用 Node 脚本离线验证"，正是缺测试框架的直接证据。

**已实现**：
- 引入 **Vitest 2.x**（`vitest@^2.1.9`；vitest 5.x 要求 vite ≥ 6，与本项目 vite 5 冲突，**勿升级**）。`package.json` 新增 `npm run test` / `npm run test:watch`。
- **只测 `utils/` 纯函数**，未引入组件测试（组件依赖 Wails 运行时与 Monaco，模拟成本远高于收益）。
- 首批覆盖：`sqlLiteral`（31）、`resultTable`（12）、`tabs`（5）、`ddl`（17）。
- 新增 `explain.ts` / `sqlError.ts` 即刻配套测试（30 + 21），落实「纯函数进 `utils/` 就配测试」。
- 统一风格：表驱动、中文用例名、覆盖正常 + 边界，与后端测试约定一致。

**验收**：`npm run test` 通过，6 个模块共 **116** 用例；`vue-tsc --noEmit` 与 `npm run build` 通过。

#### P7-5 错误信息友好化（优先级：低）✅ 已完成

**问题**：ClickHouse 报错很长，常带 `Code: xx. DB::Exception: ...` 与调用堆栈，当前原样塞进红色面板，用户要自己从长串文本里找重点。

**已实现**：
- 新增 `utils/sqlError.ts`：`parseSqlError(raw)` 返回 `{ code, summary, raw }`——提炼错误码、一句话原因（`DB::Exception` 描述 + 高频码中文映射），原文完整保留。
- 高频错误码映射表覆盖 47 / 60 / 62 / 159 / 241 等，未命中回退原始文本首行。
- 结果面板改为：错误码徽标 + 短摘要 + 「展开原文」折叠按钮，原文不丢。
- 21 条 Vitest 用例覆盖错误码提取、映射、无码回退、空输入、原文保真。

**验收**：`parseSqlError` 相关 21 用例通过；`vue-tsc --noEmit` 与 `npm run build` 通过。

### P8 高频能力补强

P7 已全部收尾。P8 三项均由用户明确提出，且互相独立、可单独交付。

| 顺序 | 项 | 内容 | 工作量 | 影响面 |
|---|---|---|---|---|
| 1 | P8-1 | 结果集 INSERT 导出（方案 A，改后端签名） | 中 | 后端 + 前端 + API 契约 |
| 2 | P8-2 | 手动收藏常用 SQL（`saved_sql.json`） | 中 | 后端新增持久化 + 前端面板 |
| 3 | P8-3 | SQL 编写提示（Monaco 自动补全） | 大 | 纯前端 + 需真实库调优 |

执行顺序理由：P8-1 影响面最明确且有后端测试兜底，先做可一次性把「改签名」的连锁改动收口；P8-2 与 `history.go` 同构，模式成熟、风险最低，放在中段；P8-3 交互细节最多且依赖真实 ClickHouse 调优，放最后避免被前两项的改动反复干扰。

#### P8-1 结果集 INSERT 导出（方案 A：改后端签名）✅ 已完成

**问题**：当前结果集导出只有 CSV / JSON / Markdown 三种，均为「纯数据渲染」。用户想把 A 表查出来的数据回灌到 B 表时，只能手工拼 `INSERT` 语句。P6-3 阶段因 `ExportResultByFormat` 不接收目标表名而暂缓，现决定按**方案 A**落地。

**方案**：
- 注册表 `Render` 签名统一改为 `func(columns []string, rows [][]interface{}, target string) (string, error)`。CSV / JSON / Markdown 忽略 `target`（保证「同一输入必然同一输出」的既有性质），仅新增的 `renderResultInsert` 使用它。
- 这样保持「注册表模式 + 不改核心分发」的既有约定，而不是在 `ExportResultByFormat` 里对 `insert` 单开分支。
- 新增 `renderResultInsert`：输出 `INSERT INTO <target> (cols) VALUES (...), (...);`，单条语句批量形式，末行分号。转义规则必须与 `frontend/src/utils/sqlLiteral.ts` 完全一致（反斜杠、单引号、`null` → `NULL`、布尔 → `1/0`、标识符反引号双写），并在 `export_test.go` 中补「Go 渲染结果与前端规则一致」的表驱动用例，防止两套转义漂移。
- `SaveQueryResultAs(format, columns, rows, target)` 增加第四个参数；`app.go` 同步；`frontend/src/api.ts` 同步；`docs/API.md` 更新契约。

**目标表的两种来源（用户明确要求两者都要）**：
- **从侧边栏选中表带出**：打开导出对话框时，若侧边栏有选中表，预填 `` `db`.`table` ``（复用 `insertTarget` 语义），用户可一键采用。
- **手动输入**：对话框内提供可编辑输入框，允许用户改成任意目标表；未选中表时输入框为空并提示必填（不在导出文件中写占位符，避免生成不可执行的 SQL）。
- 仅当选择 INSERT 格式时才弹出该输入步骤；其余格式保持原有「直接弹系统另存为」流程不变。

**落地记录**：
- `internal/clickhouse/export.go`：`resultExportRegistry` 追加 `insert` 条目；新增 `renderResultInsert` / `escapeInsertIdentifier` / `insertLiteral` / `escapeInsertString`。字符串转义改为反斜杠风格（`\` → `\\`、`'` → `\'`），并新增 `NaN` / `Inf` → `NULL`，与前端 `sqlLiteral` 完全对齐。
- `app.go`：`SaveQueryResultAs` 增加 `target` 参数并透传。
- `frontend/src/api.ts`：`SaveQueryResultAs` 增加可选 `target`（默认空串）。
- `frontend/src/components/ResultGrid.vue`：选中 `insert` 时弹出「目标表」对话框（`Teleport to="body"`、z-index 2000、`@click.self` + Esc 关闭），默认值取侧边栏选中表，确认后调用 `runExport`；其余格式走原流程。
- `locales/zh.ts` / `en.ts`：新增 `editor.insertTarget*` 六个 key。
- `export_test.go`：新增 `TestInsertLiteral` / `TestEscapeInsertIdentifier` / `TestRenderResultInsertStructure` / `TestRenderResultInsertEdgeCases` / `TestExportResultByFormatInsert` / `TestExportResultByFormatIgnoresTargetForOtherFormats`。

**验收**：INSERT 导出产物可直接回灌 ClickHouse 成功；`export_test.go` 覆盖 Go 与前端转义一致性、空目标表报错、单行/多行/含中文与特殊字符、`null` 与布尔、其余格式忽略 `target`；`go test ./...`、`npm run test`（179 用例通过）、`vue-tsc --noEmit`、`npm run build`、`go build ./...` 全通过。

#### P8-2 手动收藏常用 SQL ✅ 已完成

**问题**：现有查询历史（`internal/config/history.go`）是**自动记录**、上限 200 条（`HistoryMaxEntries`）、且会被「清空历史」一并删除。用户想长期留存的常用 SQL 与此语义冲突——既可能被新记录挤出上限，也可能被误清空。

**方案**：
- 新建 `internal/config/saved_sql.go`，持久化到 `saved_sql.json`，严格照 `config/manager.go` 与 `history.go` 的既有模式：内存缓存 + `sync.RWMutex` + 变更后全量写回。
- 提供 `List` / `Add` / `Update` / `Delete`；**无条数上限、不参与「清空历史」**，且**刻意不提供 `Clear`**，收藏只能逐条删除，避免误操作清空用户资产。
- 记录字段：`id` / `name` / `sql` / `connectionId` / `createdAt` / `updatedAt`。同一 SQL 文本（忽略首尾空白）重复收藏时返回 `ErrSavedSQLDuplicate`，不静默产生重复条目；`name` 留空时取 SQL 首个非空行（超 80 字符截断）。
- 前端新建 `components/SavedSQLPanel.vue`（与 `HistoryPanel.vue` 同构）：搜索、内联重命名、单条删除、点击回填；`QueryEditor.vue` 工具栏加书签图标与「+」按钮，两个面板互斥显示；「+」打开 `Teleport` 收藏对话框，预填 SQL 首行作标题。
- 补 `saved_sql_test.go`：表驱动 + 中文用例名，覆盖正常往返、无上限、空 SQL 拒绝、重复收藏、删除不存在 ID、重启加载一致性、空文件与损坏文件，以及**「清空历史不影响收藏」**的跨界用例。

**落地记录**：
- 后端：`internal/config/saved_sql.go` + `saved_sql_test.go`；`App` 新增 `saved` 字段与 `GetSavedSQL` / `SaveSQL` / `UpdateSavedSQL` / `DeleteSavedSQL` 四个方法。
- 前端：`types.ts` 新增 `SavedSQL`；`api.ts` 新增四个方法；`SavedSQLPanel.vue` 新建；`QueryEditor.vue` 接入面板、开关互斥、收藏对话框；`locales/zh.ts` / `en.ts` 各补 12 个 `editor.*` key。

**验收**：收藏的 SQL 重启后仍在；清空历史不影响已保存项；超过 200 条历史后已保存项不受影响；`go test ./...`、`npm run test`（179 用例通过）、`vue-tsc --noEmit`、`npm run build`、`go build ./...` 全通过。

#### P8-3 SQL 编写提示（Monaco 自动补全）✅ 已完成

**问题**：当前 Monaco 只加载了 SQL **语法高亮**（`monaco-editor/languages/definitions/sql/register.js`），没有任何 `registerCompletionItemProvider`，写 SQL 时无任何提示。

**方案**（纯前端，后端零改动）：
- 新建 `composables/useSqlCompletion.ts` 承载 `registerCompletionItemProvider('sql', ...)`，由 `useMonaco.ts` 在懒加载完成后调用，保持主文件只做初始化编排。
- 三层补全来源：
  1. **静态关键字 + 函数**（`utils/completionItems.ts` 的 `SQL_KEYWORDS` / `SQL_FUNCTIONS`）：`SELECT` / `FROM` / `JOIN` / `WITH` / `FORMAT` 等及 ClickHouse 常用函数，函数插入自动带左括号。
  2. **库名 / 表名**：`api.GetDatabases` / `api.GetTables`，按光标位置推断是否处于 `FROM` / `JOIN` / `INTO` 之后；表名插入为 `` `db`.`table` `` 全限定形式。
  3. **列名**：`api.GetTableSchema`，支持 `db.` 表名限定与 `FROM db.t AS x` 别名限定，仅给出对应表的列。
- **缓存策略**：库表清单按 `connectionId`、列清单按 `connectionId|db|table` 缓存，TTL 120s；同 key 并发请求复用 in-flight Promise；切连接时 `QueryEditor` 调 `invalidateCompletion()` 全部失效。
- 补全项带 `kind`（keyword / function / table / column）与 `detail`（返回类型或所属库），单次候选上限 200 条。
- 上下文判定抽到 `utils/completionContext.ts` 纯函数（`getCompletionContext` / `extractTableRefs` / `resolveQualifier` / `isCursorInComment` / `unquoteIdentifier` / `filterByPrefix`），候选组装抽到 `utils/completionItems.ts`（`buildCompletionItems`），两者均配 Vitest。

**落地结果**：
- 新增 `frontend/src/utils/completionContext.ts` + `.test.ts`（64 用例）、`frontend/src/utils/completionItems.ts` + `.test.ts`（13 用例）、`frontend/src/composables/useSqlCompletion.ts`。
- 改动 `composables/useMonaco.ts`（新增 `getConnectionId` 选项与 `invalidateCompletion` 返回值）、`views/QueryEditor.vue`（传入活动连接 ID、切连接时失效缓存）。
- `docs/API.md` 不变（未新增后端方法）；`docs/CODE_INDEX.md` / `docs/FEATURES.md` / `.trae/PROJECT.md` 已同步。

**验收**：`utils/completionContext.test.ts` 覆盖 FROM / JOIN / `FROM a, ` 逗号多表 / SELECT 列表 / 注释内 / 空语句 / 库表限定 / 别名等上下文；`utils/completionItems.test.ts` 覆盖表名位置、库限定、别名限定、前缀过滤、函数插入、空数据源；`npm run test` 256 用例全通过；`vue-tsc --noEmit`、`npm run build`、`go build ./...`、`go test ./...` 通过；Monaco chunk 未增长（仍 2,753 kB，补全逻辑在 QueryEditor chunk）。

### P9 结果区体验增强

P8 已全部收尾后由用户提出，全部为**纯前端 + 后端零改动**，统一在 `ResultGrid.vue` 的本地视图层实现。

| # | 编号 | 内容 | 规模 | 涉及层 |
|---|---|---|---|---|
| 1 | P9-1 | 结果表格：复制字段 / 排序 / 搜索 / 刷新 / 定时刷新 / 自定义定时刷新 | 中 | 纯前端 |
| 2 | P9-2 | DBeaver 风格表头排序 + 按值过滤 | 小 | 纯前端 |
| 3 | P9-3 | 结果工具栏收拢 + Navicat 式多条件筛选 | 中 | 纯前端 |
| 4 | P9-4 | P9-2 / P9-3 实现复盘：缺陷修复 + 结构收敛 | 中 | 纯前端 |
| 5 | P9-5 | 空结果 / 加载中 / 刷新态的可视化区分 | 小 | 纯前端 |

#### P9-1 结果区复制字段 · 排序 · 搜索 · 刷新 · 定时刷新 ✅ 已完成

**方案**：
- **复制字段**：表头右键菜单新增「复制该字段全部值」「复制该字段去重值」，复制范围跟随当前搜索过滤结果；原有「复制列名（逗号分隔）」保留。
- **排序**：表头单击三态切换（升序 → 降序 → 取消），数值按大小排序而非字典序，空值恒排最后；只排已加载的结果，不发新查询。
- **搜索**：工具栏搜索框全列忽略大小写子串匹配，`Esc` 清空；行数区显示「筛选 N / 共 M 行」，无命中时表格上方提示。搜索与排序叠加（先过滤再排序）。
- **刷新**：工具栏「刷新」重新执行当前 SQL，`keepResult = true` 避免画面闪空。
- **定时刷新 + 自定义**：预设 5 / 10 / 30 / 60 / 300 秒，齿轮按钮打开自定义对话框支持 1–86400 秒任意整数；倒计时显示在工具栏，间隔存 localStorage，查询进行中不叠加触发。

**落地结果**：
- 新增 `frontend/src/utils/resultView.ts` + `.test.ts`（39 用例）、`frontend/src/utils/autoRefresh.ts` + `.test.ts`（23 用例）。
- 改动 `components/ResultGrid.vue`（`rowIndices` 驱动的虚拟滚动、搜索框、排序指示、表头菜单、刷新 ticker、自定义间隔 Modal）、`views/QueryEditor.vue`（`onResultRefresh` + `executeSelectedSQL` 的 `keepResult` 参数）、`locales/zh.ts` / `en.ts` 各补 15 个 `editor.*` key。

**验收**：`SortState` 三态切换、数值/字符串/空值/重复值排序、搜索过滤与叠加、字段复制去重均有用例覆盖；`npm run test` 318 用例全通过；`vue-tsc --noEmit`、`npm run build`、`go build ./...`、`go test ./...` 通过；Monaco chunk 未增长。

**P9-4 复盘补充（已落地）**：
- **搜索 UI 补齐**：P9-1 当初只落了 `resultView` 辅助函数与 i18n（`searchPlaceholder` / `clearSearch` / `searchNoMatch` / `filteredCount`），工具栏搜索框本身未接入，`rowIndices` 仍是全量索引。现新增 `cellText` / `rowMatches` / `filterRowIndices` 三个纯函数，`rowIndices` 改为由 `filterRowIndices(rows, searchText, searchColumn)` 驱动，工具栏内嵌搜索框（`Esc` / × 清空、`filteredCount` 计数、零命中提示 + 清空按钮），表头右键新增「仅在 xxx 中搜索」收窄到单列；`searchColumn` 随列数变化自动回落全列。
- **表头改为仅右键**：移除表头 `@click`，只保留 `@contextmenu`（`cursor-context-menu`），左键不再弹菜单；因本地排序从未落地，左键本无其他职责，移除无副作用。
- **复制列名修正**：表头菜单原「复制列名（逗号分隔）」复制全部列名，不符合右键单列的直觉；改为「复制列名」只复制当前列，全列复制保留为独立菜单项「复制列名（逗号分隔）」。

#### P9-2 DBeaver 风格表头排序 + 按值过滤 ✅ 已完成

**问题**：P9-1 的排序与搜索是**本地视图层**过滤，只作用于已加载的结果集；用户想按某列真正重排或过滤全表时没有入口。

**已实现（纯前端拼 SQL，后端零改动）**：
- 表头菜单新增「升序排序 / 降序排序 / 按此值过滤」，单元格右键菜单新增同类入口。
- 新增纯函数 `frontend/src/utils/resultFilter.ts`：`appendSort`（替换或追加 `ORDER BY`）、`appendWhere`（追加或合并 `WHERE`）、`filterExpression`（按操作符生成条件）、`escapeSqlString` 等；保留前置注释与尾部 `FORMAT` / `SETTINGS` 子句，重复操作时替换而非叠加。
- 仅对单条 `SELECT` / `WITH` 生效，其余语句相应入口禁用并给出提示。
- 应用后通过既有 `@rerun` 事件回主视图重新执行，取消、分页、虚拟滚动、导出全部自动继承。

**验收**：`resultFilter` 相关用例覆盖排序替换/追加、已有 WHERE 合并、`FORMAT` / `SETTINGS` 尾部保留、注释保留、非法语句拒绝；`vue-tsc --noEmit`、`npm run test`、`npm run build` 通过。

**P9-2 补充（表头按值过滤，DBeaver 式）**：
- 表头右键菜单新增「按此值过滤」入口，`filterPanel` 以 `rowIndex = -1` 打开，面板渲染为第二种形态：顶部「输入字符过滤列值」搜索框 → 去重后的列值清单（`headerCandidateValues`，上限 `HEADER_VALUE_LIMIT = 12`，取当前 `rowIndices` 命中行，点选即 `列 = 值` 重跑）→ 5 个常用操作符（`=` / `<>` / `>` / `<` / `LIKE`，套用搜索框内容）→ 搜索框非空时给出「仅在 xxx 中搜索」转本地搜索。
- 单元格右键形态保持不变（8 种操作符 + 自定义区）；两种形态由模板内 `filterPanel.rowIndex >= 0` 分支切换，共用 `filterColumnName` / `confirmCustomFilter` / `appendWhere`。
- 新增 i18n key：`editor.filterCandidatePlaceholder` / `editor.filterCandidateEmpty`（中英同步）。
- `docs/FEATURES.md` / `docs/CODE_INDEX.md` 已同步；`npm run test` 398 用例全通过，`vue-tsc --noEmit`、`npm run build` 通过。

#### P9-3 结果工具栏收拢 + Navicat 式多条件筛选 ✅ 已完成

**问题**：结果工具栏原先把 CSV / JSON / Markdown / INSERT 四个导出按钮横向排成一行，占满右侧并挤压其他按钮；同时缺少按列组合多条件的筛选入口。

**已实现（纯前端，后端零改动）**：
- **导出收拢**：四个导出按钮合并为单个「导出」下拉菜单（`Teleport` 之外的本地绝对定位面板，点外部 / Esc / 选择后自动关闭），菜单内列出全部格式并带图标与描述，工具栏横向压力显著降低。
- **工具栏分区**：刷新与自动刷新下拉合并为一个带边框的按钮组，执行计划、导出、筛选各为独立分区，视觉层次清晰。
- **多条件筛选（Navicat 风格）**：新增「筛选」按钮，打开筛选面板，结构为「列 + 条件组」：
  - 每个条件组选定一列，组内可加多条规则，规则之间自动以 **OR** 组合；
  - 条件组之间通过组头的 **AND / OR** 下拉连接，首组固定无连接词；
  - 支持 8 种操作符（`=` / `!=` / `>` / `>=` / `<` / `<=` / `LIKE` / `NOT LIKE`），空值规则自动忽略；
  - 按钮上显示当前有效规则数量徽标，支持「重置」清空全部条件；
  - 底部「应用」将结果拼成 `WHERE` 子句并经 `@rerun` 重新执行。
- `utils/resultFilter.ts` 新增 `FILTER_GE` / `FILTER_LE` 两个操作符、`FilterRule` / `FilterCondition` / `FILTER_JOIN_ORDER` 等类型，以及 `buildFilterWhere` / `countActiveRules` / `rawFilterExpression` / `conditionExpression` / `ruleSummary` 纯函数。
- 修复 `percent` 操作符生成 `col LIKE %abc%` 缺引号的缺陷，统一走 `escapeSqlString('%' + value + '%')`。
- `locales/zh.ts` / `en.ts` 各补 `editor.*` 导出与筛选相关文案。

**验收**：`resultFilter.test.ts` 共 71 用例全通过（含「多条件筛选」13 条新用例，覆盖各操作符、单/多规则、空值忽略、引号转义、AND/OR 连接、有效规则计数）；`npm run test` 364 用例全通过；`vue-tsc --noEmit`、`npm run build` 通过，Monaco chunk 未增长。

#### P9-4 P9-2 / P9-3 复盘：缺陷修复 + 结构收敛 ✅ 已完成

对 P9-2 / P9-3 的实现做一次自审，查出以下问题，按严重度排序。

**一、功能缺陷（会产出错误 SQL 或错误结果）**

| # | 缺陷 | 现状 | 影响 | 修法 |
|---|---|---|---|---|
| 1 | 分页 + `ORDER BY` 互相覆盖 | `QueryEditor.onResultRerun` 把 `tab.page` 透传给 `executeSelectedSQL`，后者用 `buildPageSQL` 直接拼 `LIMIT n OFFSET m`，与 `appendSort` / `appendWhere` 已经写入的 `LIMIT` 重复冲突 | 在分页状态下排序或筛选，SQL 变成两段 `LIMIT`，查询直接报语法错误 | 排序/筛选属于**改变结果集语义**的操作，必须重置分页（`page = null`、`offset = 0`）。`onResultRerun` 改为不继承 `tab.page`，新起 `executeSelectedSQL(sql, null, false)` |
| 2 | 筛选面板不感知 `#` 注释外的方言差异 | `baseStatement` 已剥离 `/* */`、`--`、`#`，但 `isConditionable` 只看首关键字；`INSERT INTO ... SELECT` 这类以 DML 开头但含子查询的语句直接被判为不可用 | 影响面小，但提示文案容易误导 | 保持现状，但把禁用原因细分（非单语句 / 非只读），tooltip 明确告知用户 | 
| 3 | 数字型列被强制加引号 | `conditionExpression` 一律走 `escapeSqlString`，`age > 18` 变成 `age > '18'`（现有测试已把这个行为固化为期望） | ClickHouse 会把 `'18'` 隐式转成 `String` 再比较，与 `UInt8` 列比较时可能触发类型不匹配或走不到索引 | `FilterRule` 增加 `numeric?: boolean`，面板按列类型（`result.columns[i].type`）自动判定；用户可手动切换「按数值 / 按文本」。必须同步改测试期望 |
| 4 | `LIKE` 与 `=` 的操作数缺少模糊匹配提示 | 筛选面板里选 `LIKE` 时，用户必须自己输 `%`，面板不提示 | 用户输 `abc` 得到 `LIKE 'abc'`（精确匹配），与预期「包含 abc」不符 | 面板在选中 `LIKE` / `NOT LIKE` 时显示占位符 `%abc%` 提示；或提供「自动补 %」开关，默认开启 |

**二、实现一致性问题**

| # | 问题 | 详情 |
|---|---|---|
| 5 | 两套过滤路径语义不一致 | `filterExpression`（按值过滤）在空值时返回 `IS NOT NULL`，且 `LIKE` 会自动补 `%`；`rawFilterExpression`（多条件筛选）空值直接忽略、`LIKE` 不补 `%`。同一个概念两套规则，后续必然踩坑 | 
| 6 | 冗余导出未使用 | `ruleSummary` / `conditionPreview` 在 `ResultGrid` 中并未渲染（面板直接用输入框），属死代码；`quoteColumn` 只是 `quoteIdent` 的别名，多余一层 |
| 7 | `appendSort` / `appendWhere` 的 `pageSize` 参数从未被调用方使用 | 两个函数都带可选 `pageSize` 重写 `LIMIT` 的分支，但实际调用一律不传，是 P7-3 分页留下的残留。分支逻辑复杂（涉及 `FORMAT` / `SETTINGS` 保序）却无测试覆盖，属于易腐化代码 |
| 8 | 列名转义策略不统一 | `appendSort` 用 `quoteColumn`（反引号），`rawFilterExpression` 直接拼原始列名（无引号）。列名含空格或特殊字符时，筛选生成的 SQL 非法 |

**三、交互与工程质量**

| # | 问题 | 详情 |
|---|---|---|
| 9 | 筛选面板无「已应用条件」回显 | 应用后关闭弹窗，用户只看到按钮上的数字徽标，想改条件必须重新点开从头填。`advancedFilterConditions` 状态其实保留了，但面板打开时若已有条件不会重置——这点是对的，但缺少「当前生效条件」的可读摘要 |
| 10 | 筛选条件与搜索框、本地排序的关系没有说明 | 搜索框是本地过滤、排序是本地排序、筛选是服务端重跑。三者叠加时用户不知道哪个先生效 | 工具栏给筛选按钮加 tooltip 说明「服务端筛选，会重新执行查询」 |
| 11 | 导出下拉缺少键盘可达性 | 当前只在 `Esc` 关闭，没有 `↑` / `↓` 导航与 `Enter` 选择。桌面工具这类菜单基本都支持 |
| 12 | `ResultGrid.vue` 体积继续膨胀 | P9 三轮改动后已超过 1400 行，菜单状态、导出、筛选、刷新四套逻辑混在一个文件 | 按 P7-1 的既有节奏，把筛选面板拆成 `components/ResultFilterPanel.vue`（纯 UI + props/emit），状态仍留在 `ResultGrid` |

**建议执行顺序**（先修真缺陷，再动结构）：
1. **缺陷 1（分页 + 排序冲突）** — 一行改动，风险最低、收益最大，先做。
2. **缺陷 8（列名转义不统一）** + **缺陷 5（两套语义统一）** — 都在 `resultFilter.ts`，一次改完并补测试。
3. **缺陷 3（数值列引号）** — 涉及面板 UI 与类型感知，改完同步测试期望。
4. **缺陷 4（LIKE 提示）** + **缺陷 9、10、11** — 交互补强，可合并为一次提交。
5. **问题 6、7（死代码清理）** — 纯删除，放最后避免干扰前面的验证。
6. **问题 12（拆 `ResultFilterPanel.vue`）** — 与前五项分开做，单独一轮回归。

**验收**：分页状态下点击排序/筛选能得到合法 SQL 且分页自动归位；数值列筛选不再带引号（并为文本列保留引号）；`resultFilter.test.ts` 补充「数值列 / 文本列 / 列名需转义 / LIKE 自动补通配」用例；`vue-tsc --noEmit`、`npm run test`、`npm run build` 通过。

**落地结果**：

| # | 项 | 落地 |
|---|---|---|
| 1 | 分页 + `ORDER BY` 冲突 | `QueryEditor.onResultRerun` 重置 `tab.page = null`，排序/筛选后分页归位，不再出现两段 `LIMIT` |
| 3 | 数值列被加引号 | `FilterRule` 增 `numeric?`；`rawFilterExpression` 数值模式下走裸字面量（非数字输入自动回退加引号）；`isNumericType` 按 `Nullable(Int* / UInt* / Float* / Decimal*)` 前缀判定，面板默认按列类型勾选并可手动切「数值 / 文本」 |
| 4 | `LIKE` 无提示 | 选中 `LIKE` / `NOT LIKE` 时占位符改为 `filterLikePlaceholder`（提示支持通配、留空自动补 `%`）；`likePattern` 统一：值不含 `%` / `_` 时自动补 `%v%` |
| 5 | 两套过滤语义不一致 | `filterExpression` 与 `rawFilterExpression` 均走 `quoteColumn` + 同一 `likePattern`；空值 `=` → `IS NULL`、`<>` → `IS NOT NULL`，两路径一致 |
| 6 | 死代码 | 移除未渲染的 `ruleSummary`、`quoteColumn` 别名层；`conditionPreview` 保留（按值过滤菜单在用） |
| 7 | `pageSize` 残留分支 | `appendSort` / `appendWhere` 删除 `pageSize` 参数与重写 `LIMIT` 的复杂分支，分页统一由 `pagination.ts` 负责 |
| 8 | 列名转义不统一 | 排序与筛选全部走 `quoteColumn`（反引号 + 反引号双写），列名含空格等特殊字符也能生成合法 SQL |
| 9 | 无已应用条件回显 | `ResultFilterPanel` 顶部新增只读 `WHERE` 预览（`buildFilterWhere` 实时计算） |
| 10 | 三套过滤关系不明 | 筛选按钮 tooltip 改为「服务端筛选：拼装 WHERE 并重新执行查询；与本地搜索 / 本地排序叠加生效」 |
| 11 | 导出下拉键盘可达性 | 菜单补 `role="menu"` / `aria-haspopup` / `aria-expanded`；打开即聚焦首项，`↑` `↓` `Home` `End` 导航，`Esc` 关闭，`focus:bg-ch-hover` 焦点可见 |
| 12 | `ResultGrid.vue` 膨胀 | 筛选面板拆入 `components/ResultFilterPanel.vue`（纯 UI，`v-model:conditions` / `v-model:error` + `close / apply / clear`），`ResultGrid` 仅保留状态持有与 SQL 拼装 |

**验收（实测）**：`resultFilter.test.ts` 87 用例、前端合计 **380** 用例全通过；`vue-tsc --noEmit`、`npm run build`（12.47s）、`go build ./...`、`go test ./...` 全部通过。

#### P9-5 空结果 / 加载中 / 刷新态的可视化区分（优先级：高）✅ 已完成

**用户现场反馈**：查一张空表（或筛选后无命中）时，界面**一直停在「刷新」那一块**，既没有「加载中」，也看不出「这张表是空的」——用户无法判断是还在查、查挂了、还是根本没数据。

**根因（三个独立缺陷叠加）**：

| # | 缺陷 | 代码位置 | 说明 |
|---|---|---|---|
| 1 | **加载中完全没有视觉反馈** | `ResultGrid.vue` 模板条件链 | 渲染链只有 `canceled` / `error` / `result && columns.length > 0` / `!querying && !result && !error`（提示语）/ `result && row_count === 0`。`querying === true` 且 `result === null` 时**所有分支都不命中**，结果区一片空白，只有工具栏刷新图标在转——正是用户看到的「停在刷新」 |
| 2 | **空表与「查询已执行但无命中」混淆** | `t('editor.noResults')` = 「查询完成，无结果」 | 只有一行灰字，没有表名、没有行数/耗时、不分「表本来就空」还是「筛选条件过滤掉了全部数据」。用户无从判断 |
| 3 | **刷新时 `keepResult` 隐藏了进行态** | `QueryEditor.onResultRefresh` 传 `keepResult = true` | 刷新期间旧结果保留是刻意的（避免闪空），但没有任何「正在刷新」标识，用户以为卡死。定时刷新更是完全静默 |

**修法**：

1. **补加载态分支**：在条件链最前面插入 `v-if="querying && !result"` 的骨架/加载占位——居中显示旋转图标 + `t('editor.queryRunning')`（「正在查询…」）。`keepResult` 场景（`querying && result`）则在结果区顶部叠加一条细进度条或「刷新中…」角标，不动已有表格。
2. **空结果升级为语义化空态**：把 `row_count === 0` 的灰字换成带图标 + 主副标题的空态卡片：
   - 主标题 `t('editor.emptyResultTitle')`：「查询完成，0 行」
   - 副标题按来源区分：普通查询 → 「该表暂无数据」；带筛选/搜索条件 → 「当前筛选条件下无匹配，可尝试放宽条件」并提供「清除筛选」快捷按钮
   - 底部补一行元信息：耗时 `x ms`、列数 N，让用户确认查询确实执行完了
3. **刷新态可见**：刷新/定时刷新进行中，工具栏行数位置显示 `刷新中…`（或旋转小图标），完成后恢复为行数；静默的定时刷新也至少给出 0.5s 的轻量提示。
4. **空表快捷引导**：空态里给一个「插入一行」或「查表结构」的次要按钮（可选，视空间决定），避免用户对着空白无动作可做。

**i18n**：新增 `editor.queryRunning` / `editor.emptyResultTitle` / `editor.emptyTableHint` / `editor.emptyFilteredHint` / `editor.clearFilterAction` / `editor.refreshing`，中英同步。

**验收**：查空表时立即出现「正在查询…」，返回后显示「查询完成，0 行 / 该表暂无数据」并带耗时；筛选后无命中显示「当前筛选条件下无匹配」且「清除筛选」可用；刷新期间顶部可见「刷新中…」；查询进行中工具栏按钮禁用但视觉上明确是「在查」而不是「卡住」；`vue-tsc --noEmit`、`npm run test`、`npm run build` 通过。

**落地结果**：
- 主内容区判定链收敛为 `canceled → error → showLoading → isEmptyResult → searchNoMatch → hasTable → 初始提示`，新增派生状态 `showLoading`（`querying && !result`）、`showRefreshing`（`querying && result`）、`isEmptyResult`（`row_count === 0`）、`hasTable`（有列）、`searchNoMatch`（有数据但本地搜索零命中）。
- 空态卡片：图标 + `emptyResultTitle`（「查询完成，0 行」）+ 副标题（`emptyTableHint` / `emptyFilteredHint`）+ 元信息行 `emptyResultMeta`（「{columns} 列 · 耗时 {elapsed} ms」）+ 底部列名清单；服务端筛选导致的空结果显示「清除筛选」按钮一键重跑原 SQL。
- `locales/zh.ts` / `en.ts` 新增 `queryRunning` / `emptyResultTitle` / `emptyTableHint` / `emptyResultMeta` / `emptyFilteredHint` / `clearFilterAction` / `refreshing`，中英同步。

**P9-5 复盘（空表白屏，已修复）**：

用户反馈「查空表时结果区什么都没显示 / 显示一下就不见了」。真实原因是**三个独立缺陷叠加**，前两个只在运行时才暴露，`vue-tsc` 与单测都查不出来：

| # | 缺陷 | 说明 | 修法 |
|---|---|---|---|
| 1 | `<script setup>` 的 TDZ 崩溃 | 新增的 `searchNoMatch`、`filteredByServer` 两个 computed 引用了**下方**才声明的 `isEmptyResult` / `hasTable` / `advancedFilterConditions`。`const` 不提升，computed 求值即抛 `ReferenceError`，整块渲染失败 | 把两个 computed 下移到依赖声明之后。**新增 computed 时必须确认它引用的每个 `const` 都在上方** |
| 2 | i18n key 缺失 | 模板用了 `t('editor.emptyResultMeta', ...)`，但该 key 中英文件都没加。vue-i18n 对未知 key 做参数插值时抛 `TypeError`，表现为「先渲染一半再消失」 | 补齐 `emptyResultMeta`；并约定新增模板 key 必须同步中英文 |
| 3 | `QueryResult.rows` 为 `null` | 后端 `scanRows` 用 `var result [][]interface{}` 初始化，0 行时保持 `nil`，JSON 序列化为 `null`；前端多处 `result.rows.length` 直接崩（`Cannot read properties of null`） | 后端改为 `make([][]interface{}, 0)`；`types.ts` 把 `rows` 标为 `any[][] \| null`，**借 `vue-tsc` 全量揪出 6 处裸访问**（含 `DatabaseTree.vue` 的 DDL 取值，人工排查会漏）；模板与 computed 一律 `?.` + `?? []` |

**诊断基建（保留）**：`main.ts` 加 `app.config.errorHandler`、`index.html` 加 `window.onerror` / `unhandledrejection`，把运行时错误以红色横幅显示在页面顶部——Vue 组件内错误默认不冒泡到 `window.onerror`，没有这层捕获就只能看到白屏。这次正是靠它一步拿到堆栈定位问题。

**教训**：类型检查对 TDZ 与 i18n key 完全无效。可空类型声明 + `vue-tsc` 是最可靠的「全量人工排查替代品」，覆盖面比 grep 完整。

#### P10 结果区体验补齐：单元格编辑 / 工具栏整理 / 启动不自动连接 ✅ 已完成

**一、结果区工具栏整理 + 实际执行 SQL 可见**

问题：筛选 / 排序 / 分页会改写 SQL 后重跑，用户看不到最终发给后端的是什么；工具栏按钮样式不一、左侧信息被压缩成竖排。

- 工具栏改为三段式：左侧「结果统计 + 查看 SQL」不可压缩单行（`shrink-0 whitespace-nowrap`，整行可横向滚动兜底），中间弹性留白，右侧刷新 / 执行计划 / 导出 / 筛选 `shrink-0` 保护。
- 「查看 SQL」按钮放在左侧统计旁，面板 Teleport 定位：`QueryTab.lastExecutedSql` 记录**最终发给后端**的语句（含分页 `LIMIT`、`WHERE`、`ORDER BY`），与编辑器原文不一致时按钮上打一个小圆点提示 `executedSqlModified`。
- 导出 / 筛选按钮补全边框，筛选有活跃条件时高亮。

**二、单元格双击编辑（生成 UPDATE，不自动执行）**

- 新增纯函数 [utils/cellEdit.ts](file:///C:/works/go_works/meteor-ch-workbench/frontend/src/utils/cellEdit.ts)：`parseQuerySource` 从单表 `FROM` / `UPDATE` 提取源表，`buildAlterUpdateSQL` 生成 `ALTER TABLE ... UPDATE ... WHERE ...;`。
- `ResultGrid` 双击单元格进入编辑，`Enter` / 失焦提交、`Esc` 取消；提交时 `emit('edit-sql', sql)`，`QueryEditor.onResultEditSql` 写入 Monaco 并聚焦，**绝不自动执行**（ClickHouse mutation 不可回滚）。
- 定位行靠「定位列」：默认自动识别结果中的主键列，表头右键「用作编辑定位列」可手工指定 / 取消。
- 允许编辑判定（`canEditCell`）：源表可解析（无 `JOIN` / `UNION` / 子查询 / `numbers` / `system` / `remote` / `cluster`）+ 至少一个定位列 + 定位列值非空。
- 数值列（`U?Int*` / `Float*` / `Decimal` / `Bool`）输出裸字面量，其余走 `sqlLiteral` 转义。
- `cellEdit.test.ts` 31 条用例，覆盖源表解析、各类拒绝场景、定位条件拼接、数值 / 文本字面量、SQL 形状。

**三、启动默认不连接**

问题：启动时 `App.vue` 的 `onMounted` 调用 `store.checkAllConnections()`，对**每条**已保存连接真实 `TestConnection`，远端节点不可达时刷一屏 `dial tcp ... timeout`，用户什么都没点就被建连。

- 移除启动时的自动探测。`activeConnectionId` 初始为 `null`，`DatabaseTree` 的 `watch` 因此不触发加载。
- 保留的 `checkConnection(s)` 调用全部是用户主动触发：点击连接、右键「检查连接」、连接行刷新按钮、导入配置后刷新状态。
- 打包同时明确：本项目 Wails v3 + 本机 CLI v2 不兼容，用 `go build -ldflags "-H windowsgui -s -w"`，避免 PE 子系统为 CONSOLE 弹出 DOS 窗口（详见 `project_rules.md`「Windows 打包」）。

**四、记录被误清空文件的恢复**

`ResultGrid.vue` 曾被一条写错的 PowerShell 批量替换命令清空（变量未赋值导致写入空内容）。因无 git 历史与编辑器备份，按 `dist/` 16:07 构建产物反推语义重建完整 SFC。教训：**批量改动前先确认变量已赋值，重要文件改动前先留副本**。

**五、单元格编辑定位列全自动化（P10 补充）**

**用户现场反馈**：双击单元格提示「请先在表头右键设置「编辑定位列」，用于生成 WHERE 条件」。别的工具（DBeaver / Navicat）双击就能改，不需要先手动点一下表头。

**根因（两个叠加）**：

| # | 缺陷 | 说明 |
|---|---|---|
| 1 | 自动选键条件过严 | 旧逻辑要求「结果集中**恰好只有 1 列**全非空且全唯一」才自动设键；查一张主键列 + 多个唯一列的表时（如 `id` 与 `trace_id` 都唯一）永远判不出，直接落到「请手动设置」分支 |
| 2 | 后端主键信息被丢弃 | `GetTableSchema` 用 `DESCRIBE TABLE` 本就返回 `primary_key` 列，但解析时只取了 `name/type/comment`，主键标记被丢弃，前端无从得知表主键 |

**修法（对齐 DBeaver 三级策略，无需任何人工设置）**：

1. 后端 `ColumnInfo` 新增 `is_primary` / `has_default`。`primary_key` 在 ClickHouse 中是 `UInt8`，驱动可能返回 `uint8/int64/bool/[]byte`，新增 `isTruthy` 统一判定，避免类型断言失败丢主键。
2. 前端新增纯函数 `detectKeyColumns`：
   - ① 有 `is_primary` 列 → 用**全部主键列**（复合主键多列 `AND`）；
   - ② 无主键 → 取**第一个**全非空且全唯一的列（不再要求「恰好一列」）；
   - ③ 都没有 → 退化为**全部列**拼 `WHERE`（DBeaver 的默认行为）。
3. 新增 `buildRowKeys` 按索引取当前行的列名与值，替代原先写死的单列取值。
4. `keyIndexes` 改为 computed：`userKeyIndex >= 0` 时用手动指定列，否则用 `autoKey.indexes`；表头徽标由 `PK` 改为 `KEY`，多列可同时标注。
5. 表头右键菜单顶部新增 `editKeyHint`，说明当前策略（手动 / 主键 N 列 / 唯一列 X / 全部 N 列），并把「用作编辑定位列」做成可取消（再次点击回落到自动）。
6. i18n 新增 `useAsEditKeyCancel` / `editAutoNoKey` / `editKeyManual` / `editKeyPrimary` / `editKeyUnique` / `editKeyAll`，并改写 `editNoKey`（原「请先设置」改为「结果集里没有可用于生成 WHERE 条件的列」——只有零列结果才会出现）。

**验收**：`npx vue-tsc --noEmit` 0 错误；`npm run test` **440** 用例（13 文件）全通过，其中 `cellEdit.test.ts` 由 31 条增至 **42** 条（新增 `detectKeyColumns` 8 条 + `buildRowKeys` 3 条）；`npm run build` 成功（Monaco chunk 2,753.05 kB / 715.82 kB gzip，未增长）；`go build ./...`、`go test ./...` 通过；EXE 重新打包，`Subsystem=2`，22 MB。

**六、单元格编辑改为 Navicat 式「改完直接应用」（P10 补充）**

**用户现场反馈**：只生成 SQL 填进编辑器还是不方便，Navicat 是双击改完点保存就生效，希望也能这样。

**修法**：`ResultGrid` 增加待提交队列 `pendingEdits`（`{rowIndex, colIndex, value, sql}`）：

1. `commitEditCell` 从「直接 `emit('edit-sql')`」改为**暂存**；同一单元格再次编辑则覆盖（改回原值则移除该项）。
2. 单元格渲染优先读 `pendingValue`，已修改内容以琥珀色显示；`isPendingEdited` 驱动该分支。
3. 表格下方新增操作条（`v-if="pendingCount > 0"`）：`pendingEdits`（已修改 N 处）/ `applyEdits`（应用修改）/ `discardEdits`（撤销修改）/ `fillToEditor`（填入编辑器）。
4. `applyPendingEdits` 发出新事件 `apply-edits(statements)`，由 `QueryEditor.onResultApplyEdits` 逐条 `api.Execute` 执行（SQL 错误仍走返回体 `.error`，不抛 Go error），全部成功后按当前查询 `keepResult` 刷新，失败即停并写 `tab.error`（`applyEditsFailed`）。
5. `watch(result)` 清空 `pendingEdits`，防止翻页 / 刷新后把旧行修改套到新结果上。

**注意（ClickHouse 特性）**：`ALTER ... UPDATE` 是异步 mutation，不可回滚；中断时仅停止后续语句，已提交的部分不撤销，因此操作条保持「先暂存、再确认」两段式，避免逐格回车即写库。

**验收**：`npx vue-tsc --noEmit` 0 错误；`npm run test` **440** 用例全通过；`npm run build` 成功（Monaco 2,753.05 kB / 715.82 kB gzip 未增长）；EXE 重新打包 `Subsystem=2`，21.97 MB。

**七、修复「点应用修改后结果没刷新」（P10 补充）**

**现场现象**：点「应用修改」后列表仍是旧值，看起来像没刷新。

**根因**：ClickHouse 的 `ALTER TABLE ... UPDATE` 是**异步 mutation**，`ExecContext` 返回时 mutation 往往还没写完数据，前端紧接着的 `Query` 读到的仍是旧值。不是前端刷新逻辑问题。

**修法**：[service.go](file:///C:/works/go_works/meteor-ch-workbench/internal/clickhouse/service.go) 的 `Exec` 入口统一走 `ensureMutationSync`：

1. `isMutationStatement` 跳过前置 `--` / `/* */` 注释后按关键字判定：`ALTER ... UPDATE|DELETE`、独立 `DELETE FROM`。
2. 命中且未显式指定时，去掉结尾分号并追加 `SETTINGS mutations_sync = 2`（`2` = 等所有副本完成），使返回时数据已真实可读。
3. `hasSetting` 检测已存在 `mutations_sync` 时原样返回，尊重调用方设置（如希望异步可写 `mutations_sync = 0`）。
4. 非 mutation 语句（`SELECT` / `INSERT` / `CREATE` / `ALTER ... ADD COLUMN`）一律原样透传，零影响。

**验收**：新增 [mutation_test.go](file:///C:/works/go_works/meteor-ch-workbench/internal/clickhouse/mutation_test.go) 15 条表驱动用例全通过；`go build ./...`、`go test ./...` 全绿；`npm run build` 成功；EXE 重新打包 21.97 MB。

### 明确不做
- 目录按职责再细分（当前划分已够用）
- 多用户 / 权限 / 审计（非单人场景）
- 引入 AG Grid / Handsontable 等重型表格库
- 自动 SQL 格式化、连接池参数 UI 化

## 实施约定
- 每个阶段完成后运行校验：
  ```bash
  cd frontend && npx vue-tsc --noEmit
  cd frontend && npm run test
  cd frontend && npm run build
  go build ./...
  ```
- 前端改动务必重新构建 `dist`，详见 `.trae/rules/project_rules.md`