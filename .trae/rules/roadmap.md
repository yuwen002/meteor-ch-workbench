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
- SQL INSERT 格式仍**暂缓**（需目标表名，会改动 `ExportResultByFormat` 签名）；若后续要做，转义规则复用 `frontend/src/utils/sqlLiteral.ts`（P6-5 已落地），不要写第二套。
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
| 1 | P7-2 | EXPLAIN 执行计划 | 小 |
| 2 | P7-4 | 前端单元测试（Vitest） | 中 |
| 3 | P7-1 | `QueryEditor.vue` 拆分 | 大 |
| 4 | P7-3 | 结果集分页 | 中 |
| 5 | P7-5 | 错误信息友好化 | 小 |

执行顺序理由：先用 P7-2 这类小而稳的改动验证流程；紧接着立起 Vitest，使 P7-1 大重构有安全网；重构后再做 P7-3（它会动结果区，若先做会被重构搅乱）；P7-5 随手收尾。

#### P7-2 EXPLAIN 执行计划（优先级：高）

**问题**：调优 ClickHouse 最常用的手段就是看执行计划，当前完全没有入口，用户得手工敲 `EXPLAIN ...` 再自己读文本结果。

**方案**（前端即可实现，后端零改动）：
- 结果工具栏新增「执行计划」按钮，取编辑器当前选中 SQL（无选中则取全部），拼成 `EXPLAIN <sql>` 走现有 `Query` 通道。
- 提供四种模式下拉，因为 ClickHouse 这几种给出的信息差异很大：
  - `EXPLAIN`（默认 Plan）
  - `EXPLAIN AST`（语法树，排查解析问题）
  - `EXPLAIN PLAN`（带索引/分区裁剪细节）
  - `EXPLAIN PIPELINE`（执行管道，看并行度）
- 结果复用现有结果表格渲染，不新增组件；不改后端签名，`docs/API.md` 无需变更。
- 拼装逻辑抽为 `utils/explain.ts` 纯函数（`buildExplainSQL(sql, mode)` + 是否可直接 EXPLAIN 的判定），供 P7-4 补测试。

**边界**：仅当编辑器内容为单条可 EXPLAIN 的语句时启用按钮；空内容或多语句（按 `;` 切分后非空段 > 1）时禁用并给出提示，避免拼出非法 SQL。

**验收**：选中一条 SELECT 点「执行计划」，结果区出现计划；四种模式各自产出合理输出；空内容与多语句时按钮禁用。

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

**验收**：主文件 ≤ 600 行；类型检查与构建通过；以下人工回归全部正常——树右键（DDL / 建表 / 编辑 / DROP）、单元格与服务表头右键复制、> 200 行虚拟滚动、`Ctrl+Enter` 执行、标签切换后光标与滚动位置恢复。

#### P7-3 结果集分页（优先级：中）

**问题**：`DefaultQueryRowLimit = 50000` 是硬截断，用户只能看到前 5 万行，没有任何继续查看的办法（结果面板只提示 `truncated`）。

**方案（待决策，二选一）**：
- **A 前端拼 SQL（推荐）**：结果工具栏出现「下一页」时，对当前 SQL 追加 `LIMIT <n> OFFSET <m>` 重跑。后端零改动，`docs/API.md` 不变。限制：仅适用于单条 SELECT，判定逻辑抽到 `utils/pagination.ts` 纯函数并补测试；多语句或非 SELECT 时不显示按钮。
- **B 后端参数化**：`Query` 增加可选 `limit` / `offset`，接口更干净且能约束总量，但会改公开签名，需同步 `docs/API.md` 与 `api.ts`，并回归 P6-1 的取消链路与既有调用方。

倾向 A：分页是浏览行为，重跑 SQL 的成本 ClickHouse 完全承受得住，且不牵动后端契约。**若确认选 B，则本项工作量上调为「中偏大」**。

**验收**：截断后出现「下一页」，点击后展示后续行且表头一致；单条 SELECT 之外不出现该按钮；页码与偏移可回到首页。

#### P7-4 前端单元测试（优先级：中）

**问题**：这是**真实存在的空白**——`utils/` 下已有 `ddl.ts` / `resultTable.ts` / `sqlLiteral.ts` / `tabs.ts` 四个纯函数模块，但每次改动都靠临时 Node 脚本验证后丢弃，没有沉淀。路线图 P6-2 与 P6-5 的验收里写着"用 Node 脚本离线验证"，正是缺测试框架的直接证据。

**方案**：
- 引入 **Vitest**（与 Vite 同源，复用现有配置，几乎零额外成本），`npm run test` 脚本加入 `package.json`。
- **只测 `utils/` 纯函数**，不引入组件测试：组件依赖 Wails 运行时与 Monaco，模拟成本远高于收益。
- 首批覆盖：`sqlLiteral`（转义边界，已有 P6-5 的验证清单可直接转成用例）、`resultTable`（阈值与列宽上下限）、`tabs`（工厂与状态恢复）、`ddl`（生成规则）。
- 新建 `utils/explain.ts` / `utils/pagination.ts` 后同步补测试，遵循"纯函数进 `utils/` 就配测试"。

**与项目约定的衔接**：后端测试约定（表驱动、中文用例名、覆盖正常 + 边界 + 往返）同样适用于前端纯函数，保持风格一致。

**验收**：`npm run test` 通过；四个现有模块 + 新增模块均有覆盖；删除任一模块的边界处理后测试能失败（证明测试有效而非空转）。

#### P7-5 错误信息友好化（优先级：低）

**问题**：ClickHouse 报错很长，常带 `Code: xx. DB::Exception: ...` 与调用堆栈，当前原样塞进红色面板，用户要自己从长串文本里找重点。

**方案**：
- 新增 `utils/sqlError.ts` 纯函数：从原始错误文本提炼 `错误码` + `一句话原因` + 原始全文（折叠展示）。
- 维护一张高频错误码映射表（如 `60` 表不存在、`47` 未知标识符、`62` 语法错误、`241` 内存超限、`159` 查询超时），未命中则回退为原始文本首行。
- 结果面板保持原文可展开，避免"友好化"掩盖真实信息导致误判。
- 补 Vitest 用例（P7-4 已建框架）。

**验收**：常见错误显示为短提示且原文可展开；未知错误码不丢信息。

### 明确不做
- 目录按职责再细分（当前划分已够用）
- 多用户 / 权限 / 审计（非单人场景）
- 引入 AG Grid / Handsontable 等重型表格库
- 自动 SQL 格式化、连接池参数 UI 化

## 实施约定
- 每个阶段完成后运行校验：
  ```bash
  cd frontend && npx vue-tsc --noEmit
  cd frontend && npm run build
  go build ./...
  ```
- 前端改动务必重新构建 `dist`，详见 `.trae/rules/project_rules.md`