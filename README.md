# Meteor CH Workbench

面向 ClickHouse 的轻量级原生桌面工作台。项目正在从旧 Wails/Vue 实现全量重写为 **Go + Gio 原生 GUI**；当前分支提供可启动的 Gio 预览版，功能仍在迁移中。

> 当前状态：开发预览（Preview），不是功能完整的正式发行版。旧版功能列表仅作迁移参考，不能视为当前版本均已可用。

## 当前预览版已接入

- Gio 原生桌面入口：`cmd/meteor-ch-gio`
- 本地连接配置读取，连接新增 / 编辑 / 删除、Native / HTTP、TLS 与连通性测试
- 连接分组管理、连接移动、分组排序和折叠
- 数据库、表和视图浏览，以及表字段、引擎、DDL、分区键、排序键和 TTL 详情
- SQL 编辑器、多标签、查询执行与取消；标签间 SQL、结果数据、筛选排序和列宽等视图状态隔离
- 查询结果表格、搜索筛选、列排序、列宽调整、单元格详情和复制
- CSV / TSV 流式导出、查询历史和收藏 SQL 面板
- 新的 domain / application / infrastructure 分层与独立 Gio 入口

## 预览运行

### 环境要求

- Go 版本以 `go.mod` 的 `go` 指令为准（当前 CI 使用 Go 1.26.11）
- Gio 桌面依赖：
  - Linux：需要 X11 / Wayland、XKB、OpenGL / EGL 等开发库
  - Windows：建议 Windows 10/11 x64；预览构建由 GitHub Actions 提供 `.exe`
  - macOS：需要 Xcode Command Line Tools

Linux（Debian / Ubuntu）安装依赖：

```bash
sudo apt-get update
sudo apt-get install -y build-essential pkg-config libxkbcommon-dev libwayland-dev libx11-dev libxkbcommon-x11-dev libxrandr-dev libxcursor-dev libxinerama-dev libxi-dev libgl1-mesa-dev libegl1-mesa-dev libvulkan-dev
```

### 启动开发预览

```bash
go run ./cmd/meteor-ch-gio
```

### 构建本地可执行文件

```bash
mkdir -p dist
go build -o dist/meteor-ch-workbench ./cmd/meteor-ch-gio
```

启动后从左侧连接栏新增 ClickHouse 连接，再选择连接加载数据库和表 / 视图。当前连接配置存储在用户配置目录下的 `meteor-ch-workbench/connections.json`；配置文件权限按平台能力限制。密码当前仍保存在本地连接 JSON 中，请勿在共享或不可信设备上保存生产凭据。

## 下载 CI 预览构建

每次分支 push / PR 更新后，GitHub Actions 会执行：

1. `go test ./...`
2. `go vet ./...`
3. 构建 Gio Linux amd64 预览二进制
4. 在 Windows runner 执行测试、vet 并构建 Gio Windows amd64 `.exe`
5. 分别上传 `meteor-ch-workbench-linux-amd64` 与 `meteor-ch-workbench-windows-amd64` Actions Artifact（保留 7 天）

在 GitHub 仓库的 **Actions → Go CI → 对应最新提交 → Artifacts** 下载预览包。只有最新 head SHA 的 CI 全部通过并成功上传 Artifact，才视为该提交具备 CI 预览构建。Linux 二进制需要目标机器具备 Gio 运行时图形库；它不是免依赖安装包。Windows 用户下载 `meteor-ch-workbench-windows-amd64` 并解压运行 `.exe`，首次启动仍需按 Windows 安全策略确认文件来源。

## 开发验证

```bash
gofmt -w ./cmd ./internal
go test ./...
go vet ./...
go build ./cmd/meteor-ch-gio
```

本地运行与 CI 构建需要桌面平台依赖。无图形环境的容器只能执行不依赖 Gio 原生图形库的测试，不能代替桌面启动验收。

## 当前尚未完成

- 设置模块与主题 / 布局 / 查询偏好持久化
- SQL 编辑器语法高亮、上下文补全、格式化和中文输入法体验
- 更完整的结果筛选、分页、导出格式及大结果集背压优化
- 可视化建表、编辑 / 删除表、集群拓扑等 ClickHouse 管理能力
- 系统钥匙串或独立凭据存储，以及旧配置导入 / 备份恢复的完整交互
- macOS CI、Windows / macOS / Linux 实际窗口验收和真实 ClickHouse 集成测试
- 继续拆分约 58 KB 的 Gio app/run.go，删除旧 Gio 单体窗口、旧业务层和 Vue 前端

## 架构方向

```text
cmd/meteor-ch-gio
  └── internal/gui/gioapp
        └── internal/application
              ├── internal/domain
              └── internal/infrastructure
                    ├── clickhouse
                    └── storage
```

详细重写范围和验收标准见 [Gio 重写计划](docs/architecture/GIO_REWRITE_PLAN.md) 与 [代码索引](docs/CODE_INDEX.md)。

## License

MIT
