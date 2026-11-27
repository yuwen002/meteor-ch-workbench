# 项目开发规则（AI 必读）

## 构建与打包
- 本项目是纯 Go + Fyne v2，**不需要前端构建**，直接 `go build`。
- Windows 打包命令：
  ```bash
  go build -ldflags "-H windowsgui -s -w" -o meteor-ch-workbench.exe .
  ```
- `-H windowsgui` 必须加，否则 PE 子系统为 CONSOLE（Subsystem=3），Windows 会弹出 DOS 黑窗口；加了之后为 Subsystem=2。
- `-s -w` 去掉符号表，体积约从 50 MB 降到 ~33 MB。
- 校验是否生效（PowerShell）：
  ```powershell
  $b=[System.IO.File]::ReadAllBytes((Resolve-Path 'meteor-ch-workbench.exe')); $pe=[BitConverter]::ToInt32($b,0x3C); [BitConverter]::ToUInt16($b,$pe+0x5C)  # 期望输出 2
  ```

## 校验命令
```bash
go build ./...      # 编译全量
go vet ./...        # 静态检查
go test ./...       # 后端测试（改 internal/ 或 fyneapp/ 必须跑）
```

## 后端改动约定
- **SQL 错误不入 Go error**：`Query` / `Execute` 的 SQL 执行错误放返回体的 `.error` 字段，Go error 返回 nil。UI 层从 `.error` 读。
- **注册表模式**：新增导出/导入/结果集导出格式，只需向对应 `export.go` / `import.go` 的注册表追加一项，不要改动核心分发逻辑。UI 层通过 `Get*Formats()` 自动感知。
- **持久化模式**：`connections.json` / `history.json` / `tabs.json` 统一采用「内存缓存 + `sync.RWMutex` + 变更后全量写回」。新增持久化文件照 `config/manager.go` 实现。
- **脱敏必须深拷贝**：配置导出的 `sanitize` 必须先深拷贝再清空密码，绝不可直接改源数据。
- **改字段同步两处**：后端 struct → `docs/API.md`（不再需要 `frontend/src/types.ts`）。

## 后端测试约定
- 测试文件与被测文件同目录，命名 `*_test.go`。
- 采用表驱动风格，用例名用中文描述场景。
- 必须覆盖：正常路径 + 边界（空值 / 缺列 / 非法输入）+ 往返一致性（导出后再解析回来）。
- 改 `internal/config` 或 `internal/clickhouse` 的解析/渲染逻辑，必须同步补或改测试。

## Fyne GUI 约定
- **入口唯一**：`main.go` 创建 `fyneapp.NewMainWindow(w)`，Lifecycle 停止时调 `fyneapp.Shutdown()`。
- **单向依赖链**：`main.go → fyneapp → internal/app → clickhouse/config`，禁止循环依赖。所有业务逻辑走 `internal/app.App`，UI 只负责事件分发。
- **快捷键用 `Canvas.AddShortcut` + 自定义 `KeyboardShortcut`**：不要尝试在 `KeyEvent` 里塞 State（Fyne 没这字段）。`runShortcut` / `stopShortcut` 是 Ctrl+Enter / Esc 的实现范式。
- **Tree 用 4 参数模板**：`widget.NewTree(childUIDs, isBranch, create, update)`，childUIDs 返回子节点 ID 数组，create 造模板，update 填内容。
- **Table 回调字段名**：选中回调是 `OnSelected`（不是 `OnCellSelected`），签名 `func(TableCellID)`。
- **PopUpMenu 定位**：`widget.ShowPopUpMenuAtPosition(menu, canvas, fyne.CurrentApp().Driver().AbsolutePositionForObject(btn))`。
- **ShowFileSave 直接写 io.Writer**：`dialog.ShowFileSave(func(writer fyne.URIWriteCloser, err error) { ... })`，writer 本身就是 io.WriteCloser，配合 `clickhouse.ExportResultTo` 零拷贝。
- **懒加载用 `OnBranchOpened`**：Tree 懒加载数据库/表/视图/集群元数据，本地 `dbCache` 缓存避免重复请求。
- **异步回 UI 必须 `fyne.Do`**：goroutine 里不能直接调 widget 方法，用 `fyne.Do(func() { ... })` 包起来。
- **定时刷新用 `time.Ticker` + channel 停止**：参考 `MainWindow.onToggleAutoRefresh`。
- **对话框关闭回调 `SetOnClosed`**：`dialog.NewCustom("标题", "关闭按钮文本", content, w)` + `.SetOnClosed(func() { ... })`。
- **Modal 弹出**：`widget.NewModalPopUp(content, canvas).Show()` — 不要用 `dialog.NewCustom` 后不设置按钮。
- **HSplit / VSplit 比例**：设置 `.Offset` 控制分割位置（0.0 ~ 1.0）。
- **AppTabs 选中回调参数是 `*TabItem`**：需要遍历 `tabItems` 数组映射回 `activeIdx`。

## 已知技术债（改动前注意）
- `clickhouse/service.go` 的 `Query` 已支持取消（`register` + `CancelQuery`，`running` map 存 context.CancelFunc）。调用方必须传 `queryID` 才能取消，传空字符串退化为不可取消。
- `connections.json` 中密码为明文存储，P5 加密已主动跳过。
- Fyne `widget.Table` 无内建搜索/排序。**搜索已自研**（`result_table.go` 的 `filteredRows` 行号映射），排序暂未实现。
- Fyne `widget.Table` 行高由内容决定，无法强制固定；大量数据会有高度抖动。
- `OnBranchOpened` 回调是 Tree 懒加载唯一入口，但 Fyne 会在滚动时频繁触发，注意去重。
- 结果集分页是**前端拼接 LIMIT/OFFSET**（`appendLimitOffset`），不支持 ClickHouse 的 `IN` 过滤式分页。已有 LIMIT 的 SQL 跳过拼接。

## 文档同步约定
- 后端新增/修改公开方法 → 更新 `docs/API.md`。
- 用户可见的功能变化 → 更新 `docs/FEATURES.md`。
- 新增文件、改变文件职责、关闭遗留问题 → 更新 `docs/CODE_INDEX.md`。
- Fyne GUI 阶段完成 → 在 `docs/migration/FYNE_PLAN.md` 勾选对应条目。
- 只写文档不改代码时，无需重新构建。

## 文档文件分工（避免重复）
| 文件 | 定位 |
|---|---|
| `.trae/PROJECT.md` | AI 快速检索地图：怎么改 X 看哪 |
| `docs/CODE_INDEX.md` | 代码索引：每个文件里有什么 |
| `docs/API.md` | 后端公开方法契约（纯 Go） |
| `docs/FEATURES.md` | 用户视角功能说明 |
| `docs/migration/FYNE_PLAN.md` | Fyne 迁移进度表（唯一真相源） |