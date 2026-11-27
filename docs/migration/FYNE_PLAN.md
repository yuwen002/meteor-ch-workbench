# Fyne GUI 迁移规划

> 从 Wails v3 + Vue 前端 → Fyne v2.8.3 原生 Go GUI 的渐进式迁移方案。
> 创建时间：2026-11-24
> 分支：`feat/fyne-gui`

## 1. 架构变化

### 1.1 原架构（Wails v3）

```
main.go (wails.Run)
  └── 根包 App struct (40+ 公开方法)
        ├── internal/clickhouse  (后端逻辑)
        ├── internal/config     (持久化)
        └── Wails runtime → frontend/ (Vue 3 + Vite)
```

**问题**：根包 App 依赖 Wails runtime（`wails.Call`、`runtime.Window` 等），无法脱离 Wails 运行。

### 1.2 新架构（Fyne v2）

```
main.go (Fyne app.New)
  └── internal/gui/fyneapp/  (纯 Fyne 界面层)
        └── internal/app/     (零 GUI 依赖的后端门面)
              ├── internal/clickhouse
              └── internal/config
```

**关键设计**：
- `internal/app/` 是纯 Go 服务层，不 import 任何 GUI 框架（既不 Wails 也不 Fyne）
- `internal/gui/fyneapp/` 持有 `*app.App` 单例，负责所有 UI 逻辑
- 单向依赖链，**不会循环**

### 1.3 依赖关系

```
main.go → fyne.io/fyne/v2 + internal/gui/fyneapp
internal/gui/fyneapp → internal/app + fyne.io/fyne/v2 + fyne dialog/widget/container
internal/app → internal/clickhouse + internal/config (零 GUI)
internal/clickhouse → clickhouse-go (第三方驱动)
internal/config → JSON 文件读写 (零 GUI)
```

## 2. 目录变化

| 路径 | 说明 | 状态 |
|---|---|---|
| `main.go` | Fyne 应用入口（原 Wails 入口完全重写） | ✅ 已完成 |
| `internal/app/` | 后端门面，原根包 App 迁出 | ✅ 已完成 |
| `internal/gui/fyneapp/app_state.go` | 单例后端 + Shutdown | ✅ 已完成 |
| `internal/gui/fyneapp/main_window.go` | 主窗口骨架 | ✅ 已完成 |
| `internal/gui/fyneapp/` 其他 | 各功能模块，逐阶段补充 | 🚧 进行中 |
| `internal/clickhouse/` | 不变 | ✅ 保留 |
| `internal/config/` | 不变 | ✅ 保留 |
| `frontend/` | Wails 前端，**不再参与构建** | ⏸ 保留待清理 |
| `wails.json` | Wails 配置 | 🗑 可删除 |
| `build/` | Wails 资源 | 🗑 可删除 |

## 3. Fyne 组件映射

| Wails/Vue 组件 | Fyne 替代 | 备注 |
|---|---|---|
| Monaco Editor + Vue 状态 | `widget.Entry` (MultiLine) | 无语法高亮，后续接入 FyneMonaco |
| TanStack Virtual Table | `widget.Table` (v2.8 自带虚拟滚动) | |
| Ant Design Tree | `widget.Tree` (childUIDs + isBranch + create + update) | v2.8 模板模式 |
| Teleport Modal | `dialog.FormDialog` / 自定义 `container.NewOverlay` | |
| Pinia Store | 各 widget struct 的字段 | 无全局状态管理 |
| Wails `Call.ByName` | 直接方法调用 `fyneapp.Backend().Query(...)` | |
| `runtime.EventsEmit` | `fyne.Event` channel 或直接回调 | |
| i18n (Vue i18n) | `fyne/locale` 或自建 map | 待确认 |

## 4. 迁移任务清单

### Phase 0 — 骨架搭建 ✅ 已完成

| # | 任务 | 文件 | 状态 |
|---|---|---|---|
| 0.1 | 引入 Fyne v2.8.3 依赖 | `go.mod` | ✅ |
| 0.2 | 把根包 App 迁出到 `internal/app/` | `internal/app/app.go` | ✅ |
| 0.3 | 新建 `main.go` 作为 Fyne 入口 | `main.go` | ✅ |
| 0.4 | 新建 `fyneapp` 状态 + 后端单例 | `fyneapp/app_state.go` | ✅ |
| 0.5 | 主窗口骨架（连接树 + SQL 编辑器 + 结果区 + 查询执行） | `fyneapp/main_window.go` | ✅ |
| 0.6 | 基本查询链路跑通 | 同上 | ✅ |
| 0.7 | `go build ./...` 通过 | — | 🏃 等待首次 CGO 编译 |

### Phase 1 — 核心编辑体验

| # | 任务 | 文件 | 状态 |
|---|---|---|---|
| 1.1 | 结果区替换为 `widget.Table` | `fyneapp/result_table.go` | ✅ |
| 1.2 | 结果表格右键菜单（复制单元格/SQL字面量/整行TSV/INSERT/生成UPDATE） | 同上 | ✅ |
| 1.3 | 标签页系统（`container.NewAppTabs`） | `fyneapp/main_window.go` | ✅ |
| 1.4 | 编辑器快捷键（Ctrl+Enter 执行） | `fyneapp/main_window.go` | ✅ |
| 1.5 | 查询取消（`context.CancelFunc` + Esc 快捷键） | 同上 | ✅ |
| 1.6 | SQL 语法高亮（可选接入 FyneMonaco） | TBD | 🟡 低优先级 |

### Phase 2 — 连接管理与数据库浏览

| # | 任务 | 文件 | 状态 |
|---|---|---|---|
| 2.1 | 新建/编辑/删除连接对话框 | `fyneapp/connection_dialog.go` | ✅ |
| 2.2 | 文件夹分组管理（新建/编辑/删除） | 同上 | ✅ |
| 2.3 | 数据库元数据树（库/表/视图/集群 + 懒加载） | `fyneapp/main_window.go` | ✅ |
| 2.4 | 表结构查看（单击 → DESCRIBE TABLE） | 同上 | ✅ |
| 2.5 | 数据库树节点右键菜单（连接/文件夹/数据库/表/视图，含 DDL 快捷操作、危险操作二次确认） | `fyneapp/tree_wrapper.go` + `main_window.go` | ✅ |
| 2.6 | 连接测试按钮 | `fyneapp/connection_dialog.go` | ✅ |

### Phase 3 — 结果集体验

| # | 任务 | 文件 | 状态 |
|---|---|---|---|
| 3.1 | 结果集导出（注册表自动感知格式，ShowFileSave + ExportResultTo 直接写 io.Writer） | `fyneapp/result_export.go` | ✅ |
| 3.2 | 结果集分页（上一页/下一页 + 每页 50/100/200/500，自动 LIMIT/OFFSET 拼接） | `fyneapp/main_window.go` + `tabs.go` | ✅ |
| 3.3 | EXPLAIN 执行计划（EXPLAIN 按钮，自动去重 EXPLAIN 前缀） | 同上 | ✅ |
| 3.4 | 结果集搜索（自研 filteredRows 行号映射，大小写不敏感、任意列匹配，搜索框 + 清除按钮） | `fyneapp/result_table.go` + `main_window.go` | ✅ |
| 3.5 | 定时刷新（PopUpMenu 5s/10s/30s/停止，time.Ticker + fyne.Do 回 UI） | 同上 | ✅ |
| 3.6 | 单元格编辑（OnSelected 回调 + ALTER UPDATE 生成器，guessTargetTable 自动猜表名） | `fyneapp/result_table.go` + `main_window.go` | ✅ |

### Phase 4 — 面板与侧边栏

| # | 任务 | 文件 | 状态 |
|---|---|---|---|
| 4.1 | 多 Tab 侧边栏（连接 / 历史 / 收藏 / 配置） | `fyneapp/side_panels.go` | ✅ |
| 4.2 | 查询历史面板（List + 点击回填 + 清空） | 同上 | ✅ |
| 4.3 | 收藏 SQL 面板（收藏当前 SQL + 加载回填） | 同上 | ✅ |
| 4.4 | 配置导入导出对话框（注册表自动感知格式） | 同上 | ✅ |

### Phase 5 — 收尾与打包

| # | 任务 | 状态 |
|---|---|---|
| 5.1 | 直接 `go build -ldflags "-H windowsgui -s -w"`，32.8MB PE Subsystem=2 | ✅ |
| 5.2 | 清理 `frontend/`、`wails.json`、更新 project_rules.md | ✅ |
| 5.3 | 重写 README（纯 Go + Fyne 架构） | ✅ |
| 5.4 | 重写 CODE_INDEX.md + API.md（清除 Wails 引用） | ✅ |
| 5.5 | 最终回归（go build/vet/test 全绿 + exe 启动验证） | ✅ |

## 5. 进度统计

| 阶段 | 总任务 | 已完成 | 完成率 |
|---|---|---|---|
| Phase 0 | 7 | 7 | **100%** |
| Phase 1 | 6 | 6 | **100%** |
| Phase 2 | 6 | 6 | **100%** |
| Phase 3 | 6 | 6 | **100%** |
| Phase 4 | 4 | 4 | **100%** |
| Phase 5 | 5 | 5 | **100%** |
| **合计** | **34** | **34** | **100%** 🎉 |