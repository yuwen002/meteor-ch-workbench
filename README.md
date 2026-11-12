# Meteor CH Workbench

> 跨平台 ClickHouse GUI 工作台 / A cross-platform ClickHouse GUI workbench

一个基于 **Wails v3 + Vue 3 + TypeScript + Tailwind CSS + Monaco Editor** 构建的轻量级 ClickHouse 图形化客户端，支持 Windows / macOS / Linux。

---

## ✨ 特性

- 🌐 **中英文双语** — 自动检测系统语言，一键切换，偏好持久化
- 🗂️ **多连接管理** — 连接分组、重命名、拖拽分组，一目了然
- ⚡ **SQL 编辑器** — Monaco Editor，语法高亮，Ctrl/Cmd + Enter 快捷执行
- 📊 **实时查询结果** — 表格渲染、耗时统计、行数统计，错误即时提示
- 🗄️ **数据库 / 数据库表浏览** — 侧边栏树状结构，双击快速 SELECT
- 🌩️ **集群信息面板** — 一键查看 cluster、shard、replica、UUID，点击插入 SQL
- ↔️ **可拖拽侧边栏** — 宽度自由调整，记忆上次状态
- 🔌 **连接测试** — 创建前验证连通性
- 🔐 **TLS / LZ4 压缩** — 支持加密传输与 Native Protocol 压缩

## 架构

```
┌──────────────────────────────────────────────┐
│                  Wails v3                     │
│  ┌─────────────┐        ┌─────────────────┐  │
│  │  Vue 3 SPA  │  Call  │  Go Backend     │  │
│  │  Monaco Ed. │ ─────► │  main.App       │  │
│  │  i18n zh/en │        │  clickhouse-driver │
│  └─────────────┘        └─────────────────┘  │
└──────────────────────────────────────────────┘
```

## 快速开始

### 前置依赖

- Go >= 1.22
- Node.js >= 18
- [Wails CLI](https://wails.io/docs/gettingstarted/installation) v3

### 开发模式

```bash
wails dev
```

### 构建生产版本

```bash
wails build
```

前端单独构建（可选）：

```bash
cd frontend
npm install
npm run dev      # Vite dev server
npm run build    # 产物输出到 frontend/dist
```

## 配置文件位置

连接信息保存在用户配置目录下的 `connections.json`：

| 系统 | 路径 |
|------|------|
| Windows | `%AppData%\meteor-ch-workbench\connections.json` |
| macOS | `~/Library/Application Support/meteor-ch-workbench/connections.json` |
| Linux | `~/.config/meteor-ch-workbench/connections.json` |

## 目录结构

```
.
├── app.go                      # Wails App 服务，暴露给前端调用的所有方法
├── main.go                     # 入口，注册 Wails Service
├── wails.json                  # Wails 项目配置
├── internal/
│   ├── clickhouse/service.go   # ClickHouse 连接、查询、集群、表元数据
│   └── config/manager.go       # JSON 配置读写、Folder/Connection 管理
└── frontend/
    ├── src/
    │   ├── App.vue             # 主框架：侧边栏 + 语言切换 + 拖拽分屏
    │   ├── main.ts
    │   ├── i18n.ts             # vue-i18n 初始化
    │   ├── api.ts              # 所有 Wails Call.ByName 封装
    │   ├── types.ts            # 共享类型定义
    │   ├── stores/app.ts       # Pinia 全局状态
    │   ├── locales/            # 语言包
    │   │   ├── zh.ts
    │   │   └── en.ts
    │   ├── components/
    │   │   ├── ConnectionModal.vue
    │   │   └── FolderModal.vue
    │   └── views/
    │       └── QueryEditor.vue # SQL 编辑器 + 数据库/集群面板 + 结果表
    ├── vite.config.ts
    ├── tailwind.config.js
    └── package.json
```

## 后端 ↔ 前端 调用约定

Wails v3 自动绑定规则：前端通过 `Call.ByName("<包名>.<Service类型>.<方法名>", ...)` 调用。

本项目中 Go 入口为 `package main`，注册的 Service 类型为 `App`，因此调用格式为：

```ts
import { Call } from '@wailsio/runtime'
await Call.ByName('main.App.GetAll')
await Call.ByName('main.App.Query', connectionID, sql)
```

已在 `frontend/src/api.ts` 中统一封装，组件层直接 `import { api } from '../api'` 使用。

## 技术栈

| 层 | 技术 |
|----|------|
| 桌面框架 | [Wails v3](https://wails.io/) |
| 后端 | Go 1.22+，[clickhouse-go/v2](https://github.com/ClickHouse/clickhouse-go) |
| 前端 | Vue 3 `<script setup>` + TypeScript |
| UI | Tailwind CSS |
| 编辑器 | Monaco Editor |
| 状态 | Pinia |
| 国际化 | vue-i18n@9 |
| 构建 | Vite |

## 详细功能文档

参见 [docs/FEATURES.md](./docs/FEATURES.md)。

## License

MIT