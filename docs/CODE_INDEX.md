# 代码索引

> 本文以 `feat/native-gui-foundation` 当前运行代码为准。旧实现仅用于功能盘点和迁移参考，不代表运行时依赖。

## 当前运行入口

```text
cmd/meteor-ch-gio
  └─ internal/bootstrap
       ├─ internal/application       应用用例
       ├─ internal/domain            领域模型
       ├─ internal/infrastructure    ClickHouse 与本地存储适配器
       └─ internal/platform          操作系统能力
  └─ internal/gui/gioapp/app         Gio 主窗口与交互
       ├─ internal/gui/gioapp/connection 连接侧栏模型与视图
       └─ internal/gui/gioapp/workspace  SQL 工作区 / 标签模型
```

- `cmd/meteor-ch-gio/main.go`：唯一桌面启动入口，创建 Gio 窗口并装配新架构服务。
- `internal/bootstrap`：组合根，创建连接仓库、连接 / 分组 / 查询 / 元数据 / 查询历史 / 收藏 SQL 服务及 ClickHouse 连接池；负责旧连接配置迁移。
- `internal/domain/connection`：连接、分组等领域实体与规则。
- `internal/application/connection`：连接列表、保存、删除、测试等用例及端口。
- `internal/application/folder`：连接分组 CRUD 与排序用例。
- `internal/application/history`、`internal/domain/queryhistory`：查询历史用例与领域记录，最新优先、最多保留 200 条。
- `internal/application/savedsql`、`internal/domain/savedsql`：收藏 SQL 用例，无条数上限，支持重命名 / 修改 / 删除和重复 SQL 检查。
- `internal/application/query`：查询执行用例、结果批次与执行摘要。
- `internal/application/metadata`：数据库、表、视图、字段和表详情用例。
- `internal/infrastructure/clickhouse`：Native / HTTP / TLS 连接、连接池、查询执行、元数据读取。
- `internal/infrastructure/storage/connectionfile`：版本化连接 JSON 仓库、原子写入、旧格式导入及排序持久化。
- `internal/infrastructure/storage/historyfile`：兼容旧 `history.json` 数组格式的查询历史仓库。
- `internal/infrastructure/storage/savedsqlfile`：兼容旧 `saved_sql.json` 数组格式的收藏 SQL 仓库。
- `internal/platform/osfs`：本地文件系统适配。
- `internal/gui/gioapp/app`：当前实际使用的 Gio 窗口与交互；`run.go` 负责主事件循环和布局编排，SQL 标签生命周期在 `sql_tabs.go`，查询事件生产/消费在 `query_events.go` / `query_result_consumer.go`，结果展示与筛选在 `result_view.go` / `result_panel.go`。
- `internal/gui/gioapp/connection`：连接侧栏模型、样式和视图。
- `internal/gui/gioapp/workspace`：与 Gio 控件解耦的 SQL 标签工作区模型。

## 当前目录状态

| 目录 | 状态 | 说明 |
|---|---|---|
| `cmd/meteor-ch-gio` | 当前入口 | 保留 |
| `internal/domain` | 新架构 | 领域规则，不依赖 GUI / 驱动 |
| `internal/application` | 新架构 | 用例与端口 |
| `internal/infrastructure` | 新架构 | 数据库与存储适配器 |
| `internal/bootstrap` | 新架构 | 应用组合根 |
| `internal/platform` | 新架构 | 操作系统能力；逐步收敛到实际使用的适配器 |
| `internal/gui/gioapp/app` | 当前 GUI | 当前运行中的 Gio 实现 |
| `internal/gui/gioapp/connection` | 当前 GUI | 连接侧栏组件 |
| `internal/gui/gioapp/workspace` | 当前 GUI | SQL 工作区状态模型 |
| `internal/gui/gioapp/window.go` 及同级旧文件 | 待清理旧实现 | 旧 Gio 原型，仍引用旧 `internal/app`、`internal/config`、`internal/clickhouse`；不是新入口 |
| `internal/app` | 待迁移后删除 | 旧应用服务门面，当前新入口不依赖 |
| `internal/config` | 待迁移后删除 | 旧配置、历史、收藏、标签和设置持久化；先逐项迁移并验证数据兼容 |
| `internal/clickhouse` | 待迁移后删除 | 旧 ClickHouse 服务与导出实现；新基础设施已覆盖部分能力，剩余导出等功能仍需迁移 |
| `frontend` | 历史参考 | Vue 前端不参与 Gio 启动；作为功能与交互盘点资料，功能验收完成后删除 |
| `.trae` | 开发辅助资料 | 检查其项目说明是否与新架构一致，避免继续引用 Wails 旧入口 |

## 目录调整建议

当前分层方向正确，不建议为了“整齐”而大规模改名或搬动业务包。需要分阶段收敛：

1. **先迁移功能，再删除旧代码**：连接分组、配置导入导出、查询历史、收藏 SQL、标签持久化、设置、结果导出等功能逐项迁入新架构并补测试。
2. **旧实现隔离**：在迁移验收前，旧目录只作为参考，不再增加新功能；后续以单独清理提交删除旧 Gio 原型和旧业务包。
3. **整理 Gio 目录**：当前 `internal/gui/gioapp/app` 的 `app` 层级略显重复。待当前窗口代码稳定后，可把主窗口实现收敛到 `internal/gui/gioapp`，保留 `connection`、`workspace` 子包；移动前先检查包内引用和测试。
4. **补齐基础设施边界**：历史与收藏 SQL 的应用服务和文件仓库已迁入；接下来补齐 Gio 历史 / 收藏面板、标签页快照和设置持久化，再迁移配置导入导出与结果导出，不要重新塞回旧 `internal/config`。
5. **文档同步**：目录或入口变更时同步更新本索引和架构迁移文档。

## 依赖规则

- GUI 依赖 `application` / `bootstrap` 提供的服务，不直接创建数据库连接或访问 JSON 文件。
- `application` 依赖领域模型与端口，不依赖 Gio、文件格式或 ClickHouse 驱动。
- `infrastructure` 实现应用层定义的端口。
- `domain` 不依赖其他内部层。
- 旧 `internal/app`、`internal/config`、`internal/clickhouse` 不得成为新功能的依赖方向。

## 当前验收基线

- 当前分支：`feat/native-gui-foundation`
- 当前代码验收基线：`e99b27a32184bc842e57cd6f6162815604630eed`（窗口关闭任务入口竞态修复、并发注册保护及导出取消；保存对话框关闭等待优化正在开发）
- PR #1：Open / Draft / 未合并，base 为 `main`。
- 对应 Go CI Run `45778693845`：当前验收基线的 Linux / Windows 测试、go vet 与预览构建均成功。nux / Windows 检查已触发，检查结果以 Actions 页面为准；文档提交后仍需以新 head 的 CI 作为最终验收。
- CI 成功不等于 Windows 实机 GUI 验收；窗口布局、连接加载、真实 ClickHouse 查询仍需在目标环境验证。


## SQL 标签点击控件生命周期

- `workspace.sqlTabClicks` 与 `workspace.sqlTabCloseClicks` 均按 `workspace.TabID` 建立映射，不再按可变列表索引保存控件。
- 标签切换与关闭回调均使用稳定 TabID；列表重排后，原标签的点击状态仍绑定原标签，不会因索引变化而错位。
- 关闭标签时同步删除其切换 / 关闭 Clickable 引用与 `sqlTabStates` 状态，其他标签控件保持不变。
- 回归测试覆盖重排前后控件身份稳定，以及关闭后仅释放目标标签控件。


## 窗口关闭与后台任务生命周期

- `internal/gui/gioapp/app/lifecycle.go` 提供 `lifecycleParent`、`goWorker` 和 `shutdownWorkers`：后台任务统一继承窗口生命周期 Context，并登记到 `workspace.workers`。`lifecycleMu` 将“检查关闭状态 + WaitGroup.Add”与关闭入口串行化，避免 `Wait` 与并发 `Add` 竞态。
- `run.go` 收到 `gio.DestroyEvent` 后先取消当前请求和活动查询，再通过 `shutdownWorkers` 原子关闭任务注册入口、取消生命周期并等待已登记任务退出；Run 返回后，窗口 goroutine 关闭 `runDone`；main 在 `gio.Main()` 返回后等待 `runDone`，再由 defer `services.Close()` 关闭 ClickHouse 连接池。
- 查询执行、查询历史写入、历史 / 收藏读写、连接与元数据请求、结果导出均纳入后台任务登记。文件选择器由 Gio Explorer 提供，关闭行为仍需在目标桌面平台实测。
- lifecycle_test.go 覆盖后台任务等待、生命周期取消后拒绝启动新任务、并发注册与关闭竞态，以及父 Context 选择；`lifecycleMu` 保证任务入口关闭与 WaitGroup.Add 串行化。
- result_export.go 的 writeResultExportContext 在每行写入前检查生命周期 Context；窗口关闭后停止继续处理大型 CSV / TSV 导出，关闭文件并尝试清理不完整的本地文件。Gio Explorer 的文件保存对话框本身仍需用户关闭或返回，当前 API 不支持由 Context 主动打断对话框。
- `result_export_test.go` 覆盖已取消 Context 下不写出表头或数据，以及导出过程中取消后提前停止写入。
- 当前导出限制：Gio Explorer 的 CreateFile 系统保存对话框不接受 Context；拆分后窗口关闭不再等待该对话框，但原生对话框自身如何随父窗口销毁，仍需在 Windows / macOS / Linux 实机确认。

- `internal/infrastructure/clickhouse/pool_test.go` 验证 `PoolManager.Close()` 会关闭已创建的 `sql.DB`，关闭后不能再获取连接，且重复调用关闭保持幂等。
