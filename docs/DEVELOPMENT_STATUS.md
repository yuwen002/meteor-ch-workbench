# 开发进度与已完成功能

> 本文是当前开发状态的快速入口，按代码是否接入、是否测试、是否完成桌面验收分开记录。
> 最后更新：2026-11-27
> 当前分支：`feat/native-gui-foundation`
> 最近代码变更基线：Explorer 选中状态已抽取；Run `45857969115` 的 Linux `go test ./...` 发现 `sql_tabs_test.go` 仍引用旧 workspace 字段，已在提交 `eb3f575` 修正，当前 head 的 CI Run `45858167585` 排队中。
>
> **统计规则：**「已实现」只表示代码路径已完成并接入，不自动等于本地测试通过或真实桌面验收通过。没有对应证据的项目不得标记为验收完成。本文不使用容易失真的单一迁移百分比。

## 1. 当前整体状态

Meteor CH Workbench 正在从旧 Wails / Vue 实现全量重写为 Go + Gio 原生桌面应用。当前以 `cmd/meteor-ch-gio` 为唯一桌面入口；旧 `frontend/`、旧 `internal/app`、`internal/config`、`internal/clickhouse` 和旧 Gio 原型仍有部分保留，仅用于功能与行为对照，不能算作新版本已完成。

当前版本定位为 **开发预览版（Preview）**，尚不是功能完整的正式发行版。

## 2. 已实现并接入新 Gio 入口

以下条目表示新入口已有对应实现；具体测试与平台验收状态见第 3 节。

### 架构与基础设施
- [x] Gio 独立启动入口及 bootstrap 组合根。
- [x] domain / application / infrastructure 分层已建立。
- [x] 新连接领域模型、连接应用用例和版本化 JSON 连接仓库。
- [x] 旧连接 JSON 的显式迁移导入路径。
- [x] ClickHouse Native / HTTP 连接配置、TLS scheme、连接池和连通性测试器。
- [x] 查询执行用例与 ClickHouse QueryContext / ExecContext 执行器，支持超时、取消、批次回调和行数上限。
- [x] 数据库、表、视图、字段结构和表详情元数据读取。
- [x] 查询历史、收藏 SQL 的领域 / 应用服务及本地文件存储。

### 当前用户界面
- [x] 连接新增、编辑、删除、连通性测试。
- [x] 连接分组、移动、排序、折叠及旧分组配置兼容。
- [x] 数据库、表、视图浏览及表结构 / DDL / 引擎 / 分区键 / 排序键 / 主键 / TTL 详情。
- [x] SQL 编辑、多标签、标签切换、关闭、重命名、排序和工作区基础恢复。
- [x] 查询执行、取消、查询结果展示。
- [x] 查询结果本地筛选、排序、列宽调整、单元格详情和复制。
- [x] 查询历史面板和收藏 SQL 面板。
- [x] CSV / TSV 结果导出；写入阶段支持取消、进度和不完整本地文件清理尝试。
- [x] 窗口后台任务登记、生命周期 Context 取消、任务关闭入口保护和连接池关闭同步。
- [x] 保存文件对话框与窗口关闭等待解耦：对话框返回后重新检查生命周期，窗口已关闭则不启动导出并尝试关闭 / 清理文件。
- [x] 收藏 SQL 面板布局与增删改查动作已从 `run.go` 拆分到 `saved_sql_view.go`；列表控件按收藏 ID 绑定，搜索过滤后不会错绑其他收藏的改名 / 删除按钮。
- [x] SQL 工作区布局已从 `run.go` 拆分到 `query_workspace_view.go`，包含表结构条、查询工作区 / 工具栏、标签栏 / 标签、SQL 编辑器与状态栏；状态模型仍由 `workspace` 统一持有。
- [x] Explorer 对象导航视图已从 `run.go` 拆分到 `explorer_view.go`，包含数据库节点、表 / 视图分组、对象节点和树行绘制；连接 / 元数据状态仍由 `workspace` 统一持有。
- [x] Windows 启动窗口尺寸根据主显示器工作区和系统 DPI 计算，不再固定 1080×660；非 Windows 暂使用安全回退尺寸。

## 3. 已有测试或 CI 证据，但仍需区分验收范围

- [x] 生命周期单元测试覆盖任务等待、取消后拒绝启动、并发注册与关闭竞态。
- [x] 导出写入测试覆盖预先取消和大结果写入中途取消。
- [x] 标签状态归档、稳定 TabID 控件绑定和查询事件有效性已有对应单元测试。
- [x] 最近历史基线 `cdfe0c1` 的 CI Run [45779342076](https://github.com/yuwen002/meteor-ch-workbench/actions/runs/45779342076) 已完成，Linux / Windows 检查成功。
- [x] Windows 启动退出修复和窗口尺寸计算：提交 `eb11e16a` 的 CI Run [45780318904](https://github.com/yuwen002/meteor-ch-workbench/actions/runs/45780318904) 中 Linux、Windows 的测试、vet 和构建均成功。
- [x] 收藏 SQL 面板拆分与控件稳定绑定：最新提交 `16c0b432` 的 CI Run [45791265139](https://github.com/yuwen002/meteor-ch-workbench/actions/runs/45791265139) Linux / Windows 检查成功。
- [x] 新架构设置模块、设置文件持久化、查询行数 / 侧栏 / 编辑器比例和深浅色主题切换已接入；提交 `ee5ff06` 对应 CI Run [45798026060](https://github.com/yuwen002/meteor-ch-workbench/actions/runs/45798026060) 的 Linux / Windows 检查成功。
- [x] Explorer 视图拆分已通过 CI：提交 `54b10ac` 对应 Run [45855866259](https://github.com/yuwen002/meteor-ch-workbench/actions/runs/45855866259)，Linux / Windows 测试、vet、预览构建成功。
- [x] 提取 Explorer 元数据响应有效性判断到 `explorer_state.go`，覆盖过期序号、连接、数据库和表的响应过滤。
- [x] 新增 `explorer_request_state.go`，统一管理 Explorer 请求序号与当前取消函数；开始新请求时取消旧请求，关闭窗口时取消并使排队响应失效。
- [x] 新增 `explorer_selection.go`，聚合数据库 / 表 / 视图 / 字段与选中上下文；连接重置、数据库切换、表切换和结构清理通过状态方法完成。
- [x] 修复 `sql_tabs.go` 仍引用旧 `requestSeq/cancelRequest` 的 CI 编译错误，统一使用 `explorerRequests.Cancel()`。

**注意：** GitHub Actions 成功只能证明对应提交在 CI 环境中的测试、静态检查和构建通过，不等于真实桌面交互已验收。

## 4. 尚未完成 / 正在推进

### 高优先级：当前迁移主线
- [x] Explorer 请求序号 / 取消函数已聚合到 `explorerRequestState`；数据库 / 表 / 视图 / 字段与选中上下文已聚合到 `explorerSelection`，UI Clickable 仍由 workspace 持有。
- [ ] 为 Explorer 补充快速切换连接 / 数据库 / 表、迟到响应、空数据库、刷新和窄窗口交互回归测试。
- [ ] 将 workspace、editor、result、explorer、history、snippets、settings 按职责继续拆分；已有子模块不重复创建。
- [ ] 检查所有后台 goroutine 和 `goWorker` 调用点，补齐生命周期边界测试。
- [ ] 验证保存对话框在父窗口销毁后的真实平台行为，以及关闭后进程是否退出。
- [ ] Windows 实机验收 Gio `app.Main` 的窗口关闭退出路径，确认不再触发 `all goroutines are asleep - deadlock`；检查初始化失败时退出码。
- [x] 设置的 domain / application / storage 已接入新架构；查询行数上限设置已接入 Gio UI 和新查询请求。
- [x] 完成深浅色主题设置入口与偏好持久化；[ ] 补齐整页控件的主题视觉回归测试。

### 功能迁移
- [x] 设置页面第一版：深浅色主题、查询行数、侧栏折叠和编辑器比例；仍需验证主题下输入框 / 原生弹窗等平台控件的一致性。
- [ ] SQL 语法高亮、上下文补全、格式化和中文输入法体验。
- [ ] 完整结果集分页、更多导出格式和大结果集背压优化。
- [ ] 配置导入、导出、备份和恢复的完整新 UI 流程。
- [ ] 系统钥匙串或独立凭据存储；当前连接密码仍需进一步完善安全存储策略。
- [ ] 可视化建表、编辑 / 删除表和集群拓扑等 ClickHouse 管理能力。
- [ ] 工作区运行态、标签结果快照的持久化策略与崩溃恢复。

### 清理与发布
- [ ] 完成新入口功能替代和验收后，删除旧 Gio 原型、旧业务包和 Vue `frontend/`。
- [ ] 为 macOS / Linux 实现工作区与 DPI 查询，完成真正的跨平台启动窗口自适应。
- [ ] 完善 macOS CI，并完成 Windows / macOS / Linux 真实桌面验收。
- [ ] 增加真实 ClickHouse 集成测试。
- [ ] 测量冷启动、空闲内存、典型查询内存和发布包体积。
- [ ] 完成 Community / Pro 独立构建入口与发布流程。

## 5. 本轮开发记录：窗口关闭、导出生命周期与启动尺寸

- [x] 关闭窗口时先取消活动请求和查询，再关闭后台任务注册入口、取消生命周期 Context 并等待已登记任务退出。
- [x] 窗口事件循环返回后显式关闭服务容器和 ClickHouse 连接池；Windows 的 `gio.Main()` 不会返回，因此随后由窗口 goroutine 显式结束进程。
- [x] 导出写入阶段响应取消，关闭文件并尝试清理不完整的本地文件。
- [x] 原生保存对话框从关闭等待的 WaitGroup 中拆出；返回后检查窗口生命周期，避免窗口已销毁后继续导出。
- [x] `goWorker` 返回任务登记结果；`invalidateIfActive` 避免关闭后访问已销毁窗口。
- [ ] Windows / macOS / Linux 实机验证原生保存对话框随父窗口关闭的行为。
- [ ] 实机验证关闭窗口后进程退出、查询取消、文件句柄释放和连接池释放。
- [ ] Windows 验证主显示器工作区 / DPI 计算在 100%、125%、150%、200% 缩放下的窗口尺寸。

## 6. 当前开发顺序

1. 等待 Explorer 状态模型拆分后的最新 head CI 完成；若失败，先按失败 job / step / 日志修复。
2. Windows 实机验收 app.Main 退出修复和不同 DPI / 屏幕尺寸下的初始窗口。
3. 补充保存对话框返回、窗口关闭、任务登记失败等生命周期边界测试。
4. 全量检查后台任务入口及窗口关闭后回调，排除遗漏的 goroutine 和窗口访问。
5. 继续拆分 Explorer 请求生命周期与导航状态；本轮已抽取异步响应有效性判断并增加回归测试，下一步评估请求序号 / 取消函数的统一所有权。
6. 验证主题切换与设置面板回归测试，再继续拆分 workspace / editor / result 模块。
7. 在功能替代验收后再清理旧代码，不提前删除仍用于兼容对照的数据迁移逻辑。

## 7. 文档职责

| 文档 | 职责 |
|---|---|
| [README](../README.md) | 项目介绍、运行方式、当前预览版概况 |
| **开发进度与已完成功能（本文）** | 当前完成 / 未完成 / 验收状态和下一步任务，以此作为继续开发的快速入口 |
| [Gio 全量重写计划](architecture/GIO_REWRITE_PLAN.md) | 目标架构、重写原则、阶段和验收标准 |
| [原生 GUI 迁移记录](architecture/NATIVE_GUI_MIGRATION.md) | 迁移过程、历史决策和详细开发记录 |
| [代码索引](CODE_INDEX.md) | 当前目录结构、代码职责和依赖方向 |
| [功能说明](FEATURES.md) | 旧版功能行为清单，用于迁移盘点和回归对照；不能直接当作 Gio 当前功能清单 |
| [API 文档归档](API.md) | 旧 Wails API 参考资料，不代表新 Gio 对外 API |

## 8. 更新规则

每完成一个可独立验收的功能，必须同步更新本文：
1. 代码完成后，先标记为「已实现」，不要提前标记测试通过。
2. 对应单元测试 / CI 完成后，补充测试证据和提交 SHA。
3. 真实桌面交互完成后，再勾选平台验收项，并注明操作系统。
4. 新增或调整下一步任务时，检查是否与已完成项重复。
5. 不以提交数量、代码行数或旧版功能清单条目数计算迁移百分比。


## 2026-11-27：Explorer 视图拆分

- 将 `layoutNavigator`、`layoutDatabaseNode`、`layoutObjectNode`、`treeRow` 和 `treeGroupLabel` 从 `run.go` 移至 `explorer_view.go`。
- `run.go` 从 1,122 行减少到 926 行；视图方法仍接收原有 `workspace`，未改变数据库选择、对象加载、请求序号和元数据状态生命周期。
- 当前只完成视图层拆分，不将其误记为 Explorer 状态模型已独立；下一步应先为数据库展开 / 折叠和对象选择提取纯状态协调逻辑，再决定是否建立 presenter。
- 本轮未执行本地 gofmt、go test、go vet、build 或真实桌面 GUI 验收；以最新 head 的 Linux / Windows CI 为代码验收依据。


## 2026-11-27：Explorer 异步响应状态校验

- [x] 将数据库对象列表响应校验提取为 `isCurrentObjectsResult`，统一检查请求序号、连接 ID、数据库名。
- [x] 将表结构响应校验提取为 `isCurrentSchemaResult`，额外检查表名，避免切换表后迟到的旧表结构覆盖新选择。
- [x] 新增 `explorer_state_test.go`，覆盖当前响应、旧序号、不同连接、不同数据库和不同表等场景。
- [ ] 最新提交的 Linux / Windows CI 尚待完成；未声称本地测试已执行。

### 2026-11-27：Explorer 请求生命周期状态收敛

- 将 `requestSeq` 与 `cancelRequest` 从 `workspace` 字段收敛到 `explorerRequestState`，由 `Begin`、`Cancel`、`Sequence` 统一管理。
- 连接测试 / 数据库列表、数据库对象列表、表结构请求均通过同一状态对象生成请求序号并登记取消函数；切换目标时会先取消上一请求。
- 窗口 DestroyEvent 调用 `Cancel`，既取消当前 Explorer 请求，也递增序号，避免已经进入结果队列的响应在关闭路径继续被当作有效数据。
- 新增请求状态测试，覆盖新请求取消旧请求、显式取消使序号失效、取消后仍可开始新请求。
- 本轮代码已提交，最新 head CI 尚待完成；未执行本地 gofmt / go test / go vet / build，也未进行真实桌面验收。


## 9. 本轮开发记录：Explorer 选中上下文拆分（2026-11-27）

- 新增 `explorer_selection.go`，聚合数据库列表、当前数据库 / 表 / 对象、表 / 视图列表、字段和表详情；workspace 通过嵌入该状态模型保持现有视图访问方式，避免一次性大范围改动控件引用。
- 连接切换 / 删除时重置整个 Explorer 选择状态；数据库切换 / 折叠时清理上一个数据库的表、视图、表选择和表结构；选择表时清理旧结构，异步响应成功后再统一写入新结构。
- 新增 3 组纯状态测试，覆盖数据库切换清理、连接重置、表结构设置与清理。当前尚未有本地 gofmt / test / vet / build 记录，等待最新 head CI 验证。
- CI Run `45857092505` 的 Linux / Windows `go test ./...` 均失败，根因为 `sql_tabs.go` 两个重置方法仍引用已移除的 `requestSeq/cancelRequest`；已改为调用 `explorerRequests.Cancel()`，并统一使用 Explorer 状态模型清理选中上下文。后续提交 CI 尚待确认。


## 10. 2026-11-27：修复 Explorer 状态拆分后的标签回归测试

- CI Run [45857969115](https://github.com/yuwen002/meteor-ch-workbench/actions/runs/45857969115) 的 Linux `Test and build` 在 `go test ./...` 阶段失败；失败点不是生产代码，而是 `sql_tabs_test.go` 两个测试仍使用已嵌入 `explorerSelection` / `explorerRequestState` 取代的旧 workspace 字段。
- 将测试初始化改为显式构造 `explorerSelection` 与 `explorerRequestState`，断言同步改为检查新状态对象；保留连接清理、数据库对象清理、取消函数释放和请求序号递增的原有验证意图。
- 修复提交：`eb3f5754444193a418f0a7d83127e37a3ef29bf1`（`test(explorer): 同步标签连接测试与导航状态模型`）。
- 新 head CI Run [45858167585](https://github.com/yuwen002/meteor-ch-workbench/actions/runs/45858167585) 已排队，尚无测试 / vet / build 通过结论；未执行本地 Go 命令，也未进行真实桌面验收。
- PR #1 仍保持 Open / Draft / 未合并，base 为 `main`。


### Explorer 表切换状态回归测试补充（2026-11-27）

- 新增 `TestExplorerSelectionSelectTableClearsPreviousSchema`，验证切换表时清除旧字段 / DDL 详情，同时保留当前数据库选择。
- 测试提交：`4e10718249236f900ea7fb7405e4a2ee1919d57b`（`test(explorer): 覆盖切换表时旧结构清理`）。
- 当前 head 后续文档提交的 CI 结果仍需以最新 SHA 对应的 Actions 为准；此前 Run `45858260758` 仅对应更早提交，不作为当前 head 验收。


## 11. 2026-11-27：收拢 SQL 查询执行生命周期状态

- 当前 Explorer 状态拆分对应 CI Run [45858357453](https://github.com/yuwen002/meteor-ch-workbench/actions/runs/45858357453) 已成功完成 Linux / Windows 测试、vet 与预览构建。
- 新增 `internal/gui/gioapp/app/query_runtime_state.go`，将查询代次（`workspaceui.QueryExecution`）、执行取消函数和运行标记收拢到 `queryRuntimeState`。
- `workspace` 通过匿名嵌入该状态对象维持现有调用方式，查询事件处理、标签切换和取消流程的行为不变；同步调整查询结果与 SQL 标签测试中的状态初始化。
- 本轮尚未获得新 head 的 CI 结果；未执行本地 Go 测试 / vet / build，也未进行真实桌面 GUI 验收。


## 12. 2026-11-27：活动查询结果快照状态拆分

- 新增 `query_result_state.go`，将当前活动标签的列元数据、结果行和状态文本聚合为 `queryResultState`；`workspace` 通过匿名嵌入保留既有 UI 字段访问方式。
- 查询事件消费者将列替换、批次追加和完成状态更新委托给结果状态对象；消费者继续负责事件代次校验、历史写入、活动 / 非活动标签分流，以及 Gio 控件重置和结果视图刷新。
- `sqlTabState` 仍负责非活动标签及切换时的独立结果快照、筛选、排序和滚动状态；没有将 Gio 控件或查询取消函数放入结果快照模型。
- 新增 `query_result_state_test.go`，覆盖列元数据替换、批次追加、查询完成和错误完成时保留已有快照。
- 当前提交后需等待最新 head 的 Linux / Windows CI；未执行本地 gofmt、go test、go vet、build 或真实桌面 GUI 验收。


### 活动查询结果状态拆分后的测试初始化修复（2026-11-27）

- 首次 CI Run [45859859384](https://github.com/yuwen002/meteor-ch-workbench/actions/runs/45859859384) 的 Linux `go test ./...` 编译失败，根因是 `workspace` 已将 `queryColumns/queryData/queryStatus` 嵌入到 `queryResultState`，但 `query_events_test.go`、`result_view_test.go` 和 `sql_tabs_test.go` 的 workspace 复合字面量仍使用旧的提升字段名。Go 复合字面量不能通过字段提升初始化嵌入字段。
- 已将上述测试初始化改为显式构造 `queryResultState{...}`，保留原断言意图；没有回退生产代码拆分。
- 修复后的最新 head CI 尚在排队 / 执行，必须以最新 SHA 的 Linux / Windows 测试、vet 和构建结果作为验收结论。


### 第二轮 CI 编译问题修复（2026-11-27）

- head `aec331c595c3f40f0c31f60e2d1211ae8d99d777` 的 Linux 和 Windows `go test ./...` 均失败，Windows 日志给出了剩余位置：`sql_tabs_test.go:273`、`sql_tabs_test.go:368` 仍有内嵌字段 `queryColumns` 的单字段复合字面量。
- 已将这两处初始化改为 `queryResultState: queryResultState{queryColumns: columns}`。这是同一类测试初始化遗漏，不涉及生产逻辑。
- 修复提交后需继续以最新分支 head 的两平台 CI 结果验收；先前失败的 CI 不能作为最新提交的验收依据。


### SQL 标签结果快照复制优化（2026-11-27）

- 标签切换归档结果时不再逐行复制 `[][]any`，改为共享只读结果行切片，消除每次切换时与结果行数成正比的行容器和字节值复制。
- 安全边界来自查询结果事件：ClickHouse 执行器复制驱动返回的可变字节值，GUI 发布批次前复制行容器；事件入队后按不可变数据处理。
- 活动结果追加新批次时只扩展当前切片，不修改已有行；列变化或新查询仍通过替换 / 清空切片释放旧快照。
- 新增测试验证标签归档与活动结果共享行存储，以及活动结果追加后已保存快照的长度和内容保持稳定。
- 当前提交的 Linux / Windows 全量测试、vet 和预览构建仍以最新 head Actions 结果为准；尚未进行本地性能基准或真实桌面大结果集验收。


### 结果表格滚动缓存收敛（2026-11-27）

- 结果行已通过 Gio `widget.List` 虚拟化布局，但单元格点击控件与格式化文本缓存此前按访问过的行保留，滚动长结果集后会逐步累积。
- 本次在列表布局完成后使用 `Position.First/Count`，只保留可见行及上下各 2 行缓存；新增测试覆盖可见区、overscan、空视口和越界位置。
- 本次仅收敛单元格 UI 缓存，不改变查询结果、筛选排序索引或标签快照；仍需 CI 双平台验收及真实桌面长距离滚动验证。


### 2026-11-27：结果视图索引切片复用

- 筛选 / 排序刷新此前每次都分配新的 `[]resultRowView`，即使旧视图已经不再被当前 Gio 帧使用。
- 新增带目标切片的构建入口，刷新时复用 `workspace.resultView` 的底层数组；只写入源行索引，不修改共享只读查询结果。视图变长超过已有容量时仍由 Go 正常扩容。
- 新增测试覆盖缩小筛选、重新扩展、排序结果和底层数组复用，并确认源数据未被修改。
- 将活动 Gio 结果视图基准扩展到 1 万、10 万、50 万行，分别覆盖全局关键字筛选与组合筛选 / 排序；基准数据在计时前构建，报告内存分配。
- 本轮尚未在本地执行基准；性能数据需在开发机使用 `go test ./internal/gui/gioapp/app -run '^$' -bench BenchmarkBuildResultRowViewLargeResult -benchmem` 实测。CI 负责编译与常规测试，不将基准结果视为已测量。
