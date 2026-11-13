# 功能详细文档

> Meteor CH Workbench 完整功能说明

---

## 目录

- [1. 连接管理](#1-连接管理)
- [2. 分组（Folder）](#2-分组folder)
- [3. SQL 编辑器](#3-sql-编辑器)
- [4. 查询结果面板](#4-查询结果面板)
- [5. 数据库 & 表浏览器](#5-数据库--表浏览器)
- [6. 集群信息面板](#6-集群信息面板)
- [7. 侧边栏](#7-侧边栏)
- [8. 多语言（i18n）](#8-多语言i18n)
- [9. 配置持久化](#9-配置持久化)

---

## 1. 连接管理

### 创建连接

点击左侧栏顶部 **新建 +** 按钮（或直接点 `新建`），弹出连接表单：

| 字段 | 说明 |
|------|------|
| 名称 | 连接在侧边栏显示的别名（必填） |
| 所属分组 | 归属的 Folder，可选「不分组」 |
| 主机 | ClickHouse 服务器 IP / 域名 |
| 端口 | Native TCP 端口，默认 **9000** |
| 用户 | 默认 `default` |
| 密码 | 明文存储在配置文件中 |
| 数据库 | 可选，登录后默认 DB |
| 加密连接 (TLS) | 使用 TLS 协议（`opts.TLS`），默认严格校验服务端证书 |
| 数据压缩 | 启用 Native Protocol **LZ4 压缩** |

### 测试连接

填写完表单后点击 **测试连接**，工具会实际建立一次 TCP 连接，成功显示绿色提示，失败显示错误详情。

### 编辑 / 删除连接

在侧边栏 hover 一个连接项 → 显示编辑按钮（铅笔）和删除按钮（垃圾桶图标）。删除前有二次确认。

---

## 2. 分组（Folder）

### 创建分组

点击侧边栏的 **文件夹图标**（新建按钮右侧），输入分组名即可。

### 管理分组

- **重命名** — hover 分组标题 → 点击铅笔按钮
- **删除** — hover 分组标题 → 点击垃圾桶按钮
- 删除分组时，分组内的连接**不会被删除**，自动归入「未分组」区域
- 折叠 / 展开 — 点击分组标题切换

---

## 3. SQL 编辑器

### 操作

| 动作 | 快捷键 / 方式 |
|------|--------------|
| 执行当前查询 | **Ctrl+Enter**（mac: Cmd+Enter） |
| 仅执行选中部分 | 选中一段 SQL 后 Ctrl+Enter |
| 执行按钮 | 顶部工具栏 ▶ 绿色按钮 |
| 刷新元数据 | 顶部工具栏 ↻ 按钮（刷新数据库列表 + 集群列表） |

### 特性

- 基于 **Monaco Editor**，SQL 语法高亮
- 暗色主题（`ch-dark`），红色光标，行号显示
- 自动缩进、括号匹配、Ctrl/Cmd+Z 撤销
- 默认示例：`SELECT version()`

---

## 4. 查询结果面板

### 展示内容

- **列名** — 顶部粘性表头
- **数据行** — 等宽字体显示，NULL 值以灰色斜体展示
- **耗时 & 行数** — 工具栏右侧显示：`1,234 rows · 12.34 ms`
- **错误** — 红色错误面板，显示完整错误堆栈，支持换行

### 返回字段定义

```typescript
interface QueryResult {
  columns: string[]      // 列名
  rows: any[][]          // 二维数据
  row_count: number      // 总行数
  elapsed_ms: number     // 服务器执行耗时（毫秒）
  error?: string         // 错误信息（可选）
}
```

---

## 5. 数据库 & 表浏览器

位于 QueryEditor 左侧面板（`w-56`）：

### 数据库

- 点击数据库名 → 展开 / 收起
- 展开后加载该 DB 下所有表（调用 `main.App.GetTables`）
- **单击表名** → 将表名插入编辑器当前光标位置
- **双击表名** → 自动生成 `SELECT * FROM db.table LIMIT 10` 并立即执行

### 加载时机

- 切换激活连接时自动加载
- 点刷新按钮时重新加载
- 有缓存（`tablesByDB`），二次展开不再请求

---

## 6. 集群信息面板

在数据库面板下方，同样位于 QueryEditor 左侧：

### 数据源

查询 ClickHouse 系统表：`system.clusters`

### 展示内容

每个 Cluster 按名称分组，展开后显示所有节点：

| 字段 | 说明 |
|------|------|
| host_address:port | 节点地址 |
| 本地标记 | 绿色圆点 = 当前节点为本机 |
| Shard | 分片编号 |
| Replica | 副本编号 |
| Version | ClickHouse 版本 |
| version | 节点 ClickHouse 版本（best-effort，取不到为空） |
| shard_name / database_shard_name / database_replica_name | Replicated 库场景下的分片与副本标识 |

### 与 SQL 编辑器联动

- **点击集群名旁的插入按钮** → 把集群名插入到编辑器当前光标位置（写 ON CLUSTER、CLUSTER BY 语句时非常方便）
- **hover 集群行 / 副本标识** → 显示复制按钮，一键复制到剪贴板

### 为什么需要这个

写 CLUSTER 相关 SQL（`ON CLUSTER xxx`、`CREATE TABLE ON CLUSTER`、`CLUSTER BY`）时，不需要再手动去查集群名，侧边栏直接点一下就插入。

---

## 7. 侧边栏

### 布局

- 左侧主框架为一个可**拖拽调整宽度**的 sidebar（`App.vue`）
- 默认 `256px`，可拖范围 `160px ~ 480px`
- 拖拽手柄为 aside 和 main 之间的细竖条，hover 时高亮主题色
- 宽度自动保存到 `localStorage['meteor-ch.sidebarWidth']`

### 折叠 / 展开

- 点击侧边栏右上角的箭头按钮 → 折叠到 `w-12`（只剩图标）
- 折叠时拖拽手柄自动隐藏
- 展开时恢复上次保存的宽度

### 区域划分

```
┌─────────────────────────────────┐
│  [M] Meteor CH Workbench   EN ▾ ⇥│  ← Header
├─────────────────────────────────┤
│  [+ New]  [📁]                  │  ← Actions
├─────────────────────────────────┤
│  📂 Production                  │
│    🗄️ main-cluster             │
│    🗄️ backup-cluster           │
│  📂 Development                 │
│    🗄️ dev-node                 │
│  ○ 未分组                        │
│    🗄️ test-local                │
└─────────────────────────────────┘
```

---

## 8. 多语言（i18n）

### 当前支持

- ✅ 简体中文 (`zh`)
- ✅ English (`en`)

### 实现

- **vue-i18n@9** 作为底层
- 语言包位置：`frontend/src/locales/{zh,en}.ts`
- 初始化逻辑：`frontend/src/i18n.ts`

### 语言切换行为

1. 首次启动 → **自动检测浏览器 / 系统语言**
   - HTTP `Accept-Language` 含 `zh-*` → 用中文
   - 其他 → 英文
2. 用户点击右上角语言按钮手动切换
3. 选择存入 `localStorage['meteor-ch.locale']`
4. 下次启动优先读 localStorage → 没有再自动检测

### 添加新语言

1. 在 `frontend/src/locales/` 下新建 `xx.ts`，复制 `en.ts` 的 key 结构
2. 在 `i18n.ts` 中导入并注册
3. 语言切换下拉加一个选项

---

## 9. 配置持久化

### 文件位置

见 [README → 配置文件位置](../README.md#配置文件位置)。

### JSON 结构

```json
{
  "folders": [
    { "id": "ulid...", "name": "生产环境" }
  ],
  "connections": [
    {
      "id": "ulid...",
      "folder_id": "ulid...",
      "name": "主集群",
      "host": "10.0.0.1",
      "port": 9000,
      "user": "default",
      "password": "明文存储",
      "database": "default",
      "secure": false,
      "use_compress": true
    }
  ]
}
```

> ⚠️ 密码**明文**存储。如需生产环境部署，建议后续加密或改为系统 keychain。

### 读写时机

| 操作 | 行为 |
|------|------|
| 应用启动 | `store.loadAll()` → 读取整个文件 |
| 保存连接 | Go 层写入完整 `DataFile` |
| 保存分组 | 同上 |
| 删除 | 同上（删完重新持久化整个文件） |

### 后端 Manager 设计

`internal/config/manager.go` 采用 **全量覆写** 策略：每次任何 CRUD 操作都会重新写整个 `connections.json`。好处是文件永远自包含、无损坏风险；坏处是连接数极多时性能线性下降（一般几百个以内没感知）。

---

## 附录：前后端 API 完整列表

所有方法通过 `Call.ByName("main.App.Xxx", ...)` 调用，已在 `frontend/src/api.ts` 封装。

| 方法 | 前端封装 | 参数 | 返回 | 说明 |
|------|---------|------|------|------|
| `main.App.GetAll` | `api.GetAll()` | — | `DataFile` | 加载所有分组 + 连接 |
| `main.App.SaveConnection` | `api.SaveConnection(c)` | `Connection` | `void` | 新增或更新连接 |
| `main.App.DeleteConnection` | `api.DeleteConnection(id)` | `string` | `void` | 删除连接 |
| `main.App.TestConnection` | `api.TestConnection(c)` | `Connection` | `void` | 测试连通性 |
| `main.App.SaveFolder` | `api.SaveFolder(f)` | `Folder` | `void` | 新增或更新分组 |
| `main.App.DeleteFolder` | `api.DeleteFolder(id)` | `string` | `void` | 删除分组 |
| `main.App.Query` | `api.Query(id, sql)` | `string, string` | `QueryResult` | 执行 SQL |
| `main.App.GetDatabases` | `api.GetDatabases(id)` | `string` | `string[]` | 列出数据库 |
| `main.App.GetTables` | `api.GetTables(id, db)` | `string, string` | `TableInfo[]` | 列出表 |
| `main.App.GetClusters` | `api.GetClusters(id)` | `string` | `ClusterInfo[]` | 查询集群信息 |

### Connection 结构体（Go ↔ TS 共用字段）

| 字段 | Go 类型 | TS 类型 | JSON key | 说明 |
|------|---------|---------|----------|------|
| ID | `string` | `string` | `id` | ULID，自动生成 |
| Folder | `string` | `string?` | `folder_id` | 分组 ID，空为未分组 |
| Name | `string` | `string` | `name` | 显示名称 |
| Host | `string` | `string` | `host` | 主机 |
| Port | `uint16` | `number` | `port` | Native 端口 |
| User | `string` | `string` | `user` | 用户名 |
| Password | `string` | `string` | `password` | 密码 |
| Database | `string` | `string` | `database` | 默认 DB |
| Secure | `bool` | `boolean` | `secure` | TLS |
| UseCompress | `bool` | `boolean` | `use_compress` | LZ4 压缩 |