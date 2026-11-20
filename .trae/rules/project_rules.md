# 项目开发规则（AI 必读）

## 前端改动必须重新构建
- `npx vue-tsc --noEmit` 只做类型检查，不产出文件。
- Wails 运行时加载的是 `frontend/dist/`，改完 `frontend/src/` 后必须执行：
  ```bash
  cd frontend && npm run build
  ```
- 若运行的是已打包的可执行文件，需按下方「Windows 打包」重新构建并重启程序。

## Windows 打包
- 本项目是 Wails v3，本机 CLI 为 v2（不兼容），**不能用 `wails build`**，直接用 go 构建：
  ```bash
  go build -ldflags "-H windowsgui -s -w" -o meteor-ch-workbench.exe .
  ```
- `-H windowsgui` 必须加，否则 PE 子系统为 CONSOLE（Subsystem=3），Windows 会弹出一个 DOS 黑窗口；加了之后为 Subsystem=2。
- `-s -w` 去掉符号表，体积约 29 MB → 22 MB。
- 校验是否生效（PowerShell）：
  ```powershell
  $b=[System.IO.File]::ReadAllBytes((Resolve-Path 'meteor-ch-workbench.exe')); $pe=[BitConverter]::ToInt32($b,0x3C); [BitConverter]::ToUInt16($b,$pe+0x5C)  # 期望输出 2
  ```

## 校验命令
```bash
cd frontend && npx vue-tsc --noEmit   # 类型检查
cd frontend && npm run test           # 前端纯函数单元测试（Vitest，改 utils/ 必须跑）
cd frontend && npm run build          # 必须，产出 dist
go build ./...                        # 后端编译
go test ./...                         # 后端测试（改 internal/ 必须跑）
```

## 其他约定
- Wails 绑定：`Call.ByName('main.App.<方法名>', ...args)`，统一封装在 `frontend/src/api.ts`。
- 前端 i18n key 前缀分组：`common.*` / `editor.*` / `menu.*` 等，中英文都要补。

## 后端改动约定
- **SQL 错误不入 Go error**：`Query` / `Execute` 的 SQL 执行错误放返回体的 `.error` 字段，Go error 返回 nil。前端从 `.error` 读，不要用 try/catch 捕获。
- **注册表模式**：新增导出/导入/结果集导出格式，只需向对应 `export.go` / `import.go` 的注册表追加一项，不要改动核心分发逻辑。前端通过 `Get*Formats()` 自动感知。
- **持久化模式**：`connections.json` / `history.json` / `tabs.json` 统一采用「内存缓存 + `sync.RWMutex` + 变更后全量写回」。新增持久化文件照 `config/manager.go` 实现。
- **脱敏必须深拷贝**：配置导出的 `sanitize` 必须先深拷贝再清空密码，绝不可直接改源数据。
- **改字段同步三处**：后端 struct → `frontend/src/types.ts` → `docs/API.md`。

## 后端测试约定
- 测试文件与被测文件同目录，命名 `*_test.go`。
- 采用表驱动风格，用例名用中文描述场景。
- 必须覆盖：正常路径 + 边界（空值 / 缺列 / 非法输入）+ 往返一致性（导出后再解析回来）。
- 改 `internal/config` 或 `internal/clickhouse` 的解析/渲染逻辑，必须同步补或改测试。

## 前端结构约定
- 组件层不直接写 `Call.ByName`，一律走 `api.ts`。
- `utils/` 只放纯函数（如 `ddl.ts` / `tabs.ts`），不引入 store 或 API 调用；**每个纯函数模块必须配同名 `*.test.ts`**（Vitest），测试沿用后端风格：表驱动 + 中文用例名 + 覆盖正常 / 边界。
- Modal 统一用 `<Teleport to="body">`，z-index 2000 或 3000，关闭靠 `v-if + @click.self + Esc`。
- 数据库树外层需要 `group` 类，内层按钮才能用 `group-hover:opacity-100` 显示。

## 已知技术债（改动前注意）
- Monaco 走 `editor.api.js` 精简入口 + SQL 语言包，两者均在 `QueryEditor.vue` 的 `onMounted` 内动态 import（**不要改回 `import * as monaco from 'monaco-editor'`，那会重新拉进全部语言包与 LSP，体积翻倍到 4 MB+**）。子路径必须写 `monaco-editor/editor/editor.api.js`：其 `exports` 为 `"./*": "./esm/vs/*.js"`，自带 `esm/vs` 前缀的写法会被重复拼接并报 TS2307。当前 Monaco chunk 2,753 kB（约 716 kB gzip），属编辑器核心固有体积，`chunkSizeWarningLimit` 已设为 3000。
- `manualChunks` 用函数形式而非对象形式，`monaco-editor/esm/vs/languages/definitions` 单独拆到 `monaco-languages`（约 11 kB）。
- 结果表格已接入 `@tanstack/vue-virtual`（`utils/resultTable.ts` 提供阈值与列宽纯函数）。超过 `VIRTUAL_THRESHOLD = 200` 行时走虚拟滚动，**行高必须保持 28px 固定**，否则估算高度与真实高度不一致会产生滚动抖动。
- `clickhouse/service.go` 的 `Query` 已支持取消（`register` + `CancelQuery`，`running` map 存 context.CancelFunc）。调用方必须传 `queryID` 才能取消，传空字符串退化为不可取消。
- `connections.json` 中密码为明文存储，P5 加密已主动跳过。

## 文档同步约定
- 后端新增/修改公开方法 → 更新 `docs/API.md`。
- 用户可见的功能变化 → 更新 `docs/FEATURES.md`。
- 新增文件、改变文件职责、关闭遗留问题 → 更新 `docs/CODE_INDEX.md`。
- 阶段完成后 → 在 `.trae/rules/roadmap.md` 勾选对应条目。
- 只写文档不改代码时，无需重新构建。

## 文档文件分工（避免重复）
| 文件 | 定位 |
|---|---|
| `.trae/PROJECT.md` | AI 快速检索地图：怎么改 X 看哪 |
| `docs/CODE_INDEX.md` | 代码索引：每个文件里有什么 |
| `docs/API.md` | Go → TS 接口契约 |
| `docs/FEATURES.md` | 用户视角功能说明 |
| `.trae/rules/roadmap.md` | 已完成能力 + 后续计划（唯一真相源，不要再拆分子文件） |