# Meteor CH Workbench

> 跨平台 ClickHouse GUI 工作台 / A cross-platform ClickHouse GUI workbench

一个基于 **Go + Fyne v2** 构建的轻量级 ClickHouse 图形化客户端，支持 Windows / macOS / Linux。

---

## 特性

- **多连接管理** — 连接分组、文件夹管理、连接测试
- **SQL 编辑器** — 内置编辑区，Ctrl/Cmd + Enter 快捷执行，Esc 停止查询
- **多标签页** — 同时开多个查询标签，各自独立结果，重启自动恢复
- **查询取消** — 长查询可随时停止（CancelQuery + context）
- **结果集分页** — 每页 50/100/200/500，上一页/下一页，自动 LIMIT/OFFSET 拼接
- **结果集导出** — 注册表自动感知格式，CSV / JSON / Markdown / INSERT / YAML 等
- **EXPLAIN 执行计划** — 一键查看查询计划
- **定时刷新** — 5s/10s/30s 自动重跑当前 SQL
- **单元格编辑** — 点击单元格生成 ALTER UPDATE 语句，支持 String / Number / DateTime 类型
- **查询历史** — 自动记录最近 200 条 SQL，点击回填编辑器
- **收藏 SQL** — 收藏常用查询，快速加载
- **配置导入导出** — JSON / YAML / CSV / DSN / DBeaver 多种格式，可脱敏、可合并
- **数据库 / 表 / 视图浏览** — 侧边栏树状结构（懒加载），单击预览
- **连接测试** — 创建前验证连通性
- **TLS / LZ4 压缩** — 支持加密传输与 Native Protocol 压缩

## 架构

```
┌──────────────────────────────────────────────────┐
│                 Fyne v2 GUI                      │
│  ┌─────────────────────┐  ┌──────────────────┐  │
│  │  fyneapp/main_window│  │  side_panels.go   │  │
│  │  result_table       │  │  connection_dialog│  │
│  │  tabs.go            │  │  result_export    │  │
│  └─────────┬───────────┘  └────────┬─────────┘  │
│            │      NewMainWindow     │            │
└────────────┼────────────────────────┼────────────┘
             │                       │
             ▼                       ▼
┌──────────────────────────────────────────────────┐
│              internal/app.App (纯 Go)             │
│  Query / CancelQuery / TestConnection /           │
│  GetDatabases / GetTables / GetViews /            │
│  GetClusters / SaveSQL / GetQueryHistory / ...    │
└──────────────────────┬───────────────────────────┘
                       │
                       ▼
          clickhouse-go/v2 driver
```

**单向依赖链**：`main.go → fyneapp → internal/app → clickhouse/config`，无循环依赖。后端 100% 复用，UI 只负责事件分发。

## 快速开始

### 前置依赖

- Go >= 1.22

### 开发模式

```bash
go run .
```

### 构建生产版本

```bash
go build -ldflags "-H windowsgui -s -w" -o meteor-ch-workbench.exe .
```

参数说明：
- `-H windowsgui`：PE 子系统设为 2（Windows GUI），避免弹出 DOS 黑窗口
- `-s -w`：去掉符号表，体积约 50 MB → 33 MB

### 校验命令

```bash
go build ./...    # 编译全量
go vet ./...      # 静态检查
go test ./...     # 后端单元测试
```

## 配置文件位置

| 系统 | 路径 |
|------|------|
| Windows | `%AppData%\meteor-ch-workbench\` |
| macOS | `~/Library/Application Support/meteor-ch-workbench/` |
| Linux | `~/.config/meteor-ch-workbench/` |

包含以下 JSON 文件：
- `connections.json` — 连接列表 + 文件夹分组
- `history.json` — 查询历史
- `tabs.json` — 标签页状态（重启恢复）

## 目录结构

```
.
├── main.go                           # Fyne 入口：NewMainWindow + Lifecycle
├── internal/
│   ├── app/
│   │   └── app.go                    # 业务门面：Query / CancelQuery / ...
│   ├── clickhouse/
│   │   ├── service.go                # ClickHouse 连接、查询、取消
│   │   ├── metadata.go               # 数据库 / 表 / 视图 / 集群元数据
│   │   ├── export.go                 # 结果集导出注册表（CSV/JSON/MD/INSERT/...）
│   │   └── import.go                 # 结果集导入注册表
│   ├── config/
│   │   ├── manager.go                # 连接 + 文件夹持久化
│   │   ├── history.go                # 查询历史持久化
│   │   └── saved_sql.go              # 收藏 SQL 持久化
│   └── gui/
│       └── fyneapp/
│           ├── main_window.go        # 主窗口：快捷键 + Tree 懒加载 + 标签页 + 分页
│           ├── result_table.go       # Fyne Table 封装 + OnSelected 回调
│           ├── result_export.go      # ShowFileSave + ExportResultTo
│           ├── connection_dialog.go  # 连接新建/编辑/删除/测试 + 文件夹
│           ├── side_panels.go        # 连接 / 历史 / 收藏 / 配置 四 Tab 侧边栏
│           └── tabs.go               # QueryTab 结构体 + 持久化 + LIMIT/OFFSET 辅助
└── docs/
    ├── API.md                        # 后端公开方法契约（纯 Go）
    ├── FEATURES.md                   # 用户视角功能说明
    ├── CODE_INDEX.md                 # 代码索引
    └── migration/
        └── FYNE_PLAN.md              # Fyne 迁移进度表
```

## 技术栈

| 层 | 技术 |
|----|------|
| GUI 框架 | [Fyne v2.8+](https://fyne.io/) |
| 后端 | Go 1.22+，[clickhouse-go/v2](https://github.com/ClickHouse/clickhouse-go) |
| 打包 | `go build` 直接产出可执行文件（33 MB） |

## 迁移历史

本项目原基于 Wails v3 + Vue 3 + TypeScript，2025 年起迁移至 Fyne v2。详见 [docs/migration/FYNE_PLAN.md](./docs/migration/FYNE_PLAN.md)。

## License

MIT