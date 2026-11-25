# Meteor CH Workbench — 项目地图

> 本文件描述当前实际运行架构。旧 Vue/Wails 实现只作为功能迁移参考，不是当前启动入口。
> 更新时间：2026-11-25

## 产品与技术方向

- 产品：面向 ClickHouse 的桌面数据库工作台。
- 桌面 GUI：Go + Gio（原生窗口）；不再以 Wails / Vue 作为运行时。
- 数据库驱动：clickhouse-go/v2。
- Go 版本：以 `go.mod` 为准。
- 开源策略：MIT；未来收费能力通过独立发行/独立模块提供，不污染基础开源能力。
- 界面、文档、提交说明使用中文；版本号避免无必要推进。

## 当前唯一启动入口

```text
cmd/meteor-ch-gio/main.go
  └─ internal/bootstrap
       ├─ internal/domain
       ├─ internal/application
       ├─ internal/infrastructure
       └─ internal/platform
  └─ internal/gui/gioapp/app
       ├─ internal/gui/gioapp/connection
       └─ internal/gui/gioapp/workspace
```

## 目录职责

| 路径 | 职责 |
|---|---|
| `cmd/meteor-ch-gio` | Gio 桌面程序入口 |
| `internal/bootstrap` | 组合根，创建仓库、服务和依赖 |
| `internal/domain` | 领域实体与规则，不依赖 GUI / 驱动 |
| `internal/application` | 连接、分组、查询、元数据等应用用例与端口 |
| `internal/infrastructure/clickhouse` | ClickHouse 连接池、查询与元数据适配 |
| `internal/infrastructure/storage` | 本地配置及文件仓库实现 |
| `internal/platform` | 文件系统、系统对话框等平台能力 |
| `internal/gui/gioapp/app` | 当前主窗口、事件循环、UI 编排 |
| `internal/gui/gioapp/connection` | 连接侧栏模型与视图 |
| `internal/gui/gioapp/workspace` | SQL 标签集合、稳定 ID、编辑器纯状态模型 |
| `docs` | 用户文档、架构、API 与代码索引 |
| `frontend` | 历史 Vue 前端，仅作迁移盘点；不参与 Gio 启动 |
| `internal/app`、`internal/config`、`internal/clickhouse` | 历史 Go 业务实现，迁移验收前只读参考 |
| `internal/gui/gioapp/window.go` 等旧文件 | 旧 Gio 原型，非当前入口 |

## 关键设计约束

1. 新功能只进入新分层，不得重新依赖旧 `internal/app`、`internal/config`、`internal/clickhouse`。
2. GUI 通过 application 服务工作；不得在控件回调中直接创建数据库连接或读写 JSON。
3. application 依赖 domain 与端口；infrastructure 实现端口；domain 不依赖其他内部层。
4. 连接配置必须兼容历史 `connections.json`，不能要求用户重新创建已有连接。
5. 配置写入必须原子化；敏感字段不得写入日志。
6. 异步请求必须处理取消、过期结果和窗口退出，避免旧响应覆盖新状态。
7. SQL 标签以稳定 TabID 标识，不以数组索引关联异步结果。
8. 编辑器状态使用独立纯 Go 模型；Gio widget 只负责交互和绘制。
9. 不以 CI 构建成功代替 Windows 实机 GUI 验收；不宣称未实际执行的测试通过。

## 常用校验

```bash
go test ./...
go vet ./...
go build ./cmd/meteor-ch-gio
```

Windows 预览构建由 GitHub Actions 的 Windows preview job 验证。修改 Gio 布局后仍需在 Windows 实机启动，检查连接列表、未连接空态、连接加载、SQL 执行和结果表格。

## 迁移原则

- 先迁移功能与数据兼容，再删除旧目录。
- 历史、收藏 SQL、标签快照、设置、导入导出、结果导出必须分别完成新架构实现和测试。
- 不为目录整齐创建空包；新包随真实功能落地。
- GUI 文件拆分以状态边界和职责为依据，不做纯机械拆文件。
- 变更入口、目录职责或验收标准时，同步更新 `docs/CODE_INDEX.md`、架构迁移文档及本文件。
