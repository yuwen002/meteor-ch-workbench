# 原生 GUI 迁移架构说明

## 目标与约束

本次迁移将桌面界面从 Wails WebView 逐步替换为 Gio 原生 GUI，保留 Go 业务逻辑、ClickHouse 驱动和现有本地数据格式。迁移期间旧 Wails + Vue 版本继续作为兼容基线，不提前删除。

- Community 核心采用 MIT 许可证。
- Pro 功能未来通过独立私有仓库和独立构建入口组合，不进入 Community 默认依赖。
- 不使用 Go `plugin` 作为跨平台扩展机制。
- 不以未经实测的框架宣传数据作为启动速度、内存或安装包体积结论。

## 目标分层

```text
Gio GUI / 现有 Wails Adapter
          |
          v
internal/app          GUI 无关的应用用例
          |
          +---- internal/config
          |
          +---- internal/clickhouse
          |
          +---- internal/platform（文件对话框、系统操作）
          |
          +---- internal/edition（后续：Community / Pro 能力接口）
```

## 当前落地

### 第一批：数据库与查询用例

新增 `internal/app.Service`，由应用入口注入配置管理、查询历史、标签页、收藏 SQL、ClickHouse 服务和用户设置依赖。已将以下业务流程从根目录 Wails App 方法委托到该服务：

- 连接保存、删除、测试和断开。
- SQL 查询、查询取消和查询历史记录。
- DDL/DML 单条执行与批量执行。
- 数据库、表、视图、表结构和集群元数据读取。

### 第二批：本地配置与工作区用例

已将以下用例纳入 `internal/app.Service`：

- 连接、文件夹列表及排序；配置导入、导出和格式元信息。
- 查询历史读取、删除和清空；收藏 SQL 增删改查。
- 标签页快照读写；用户设置读取与保存。
- 查询结果导出格式查询，以及向调用方提供的 `io.Writer` 写出结果。

新建连接时应用服务先生成 ID，再持久化并失效对应连接池，避免配置管理器内部生成 ID 后调用方仍持有空 ID 的问题。

根目录 `App` 仍保留 Wails 公开方法签名，作为旧前端兼容适配层。文件对话框和打开目录通过平台端口调用；文件内容读写通过 `internal/platform.FileSystem` 端口执行，默认由 `internal/platform/osfs` 使用 Go 标准库实现。应用服务新增 `Close`，由 GUI 生命周期回调统一释放 ClickHouse 连接池。

### 第三批：平台能力端口

新增 `internal/platform.Platform`，定义保存文件、打开文件和打开配置目录三个能力；同时定义独立的 `FileSystem` 端口，封装文件读取、覆盖写入和创建写入句柄。`internal/platform/wailsadapter` 提供 Wails v3 对话框实现，`internal/platform/osfs` 提供基于 Go 标准库的文件系统实现。根目录 `App` 通过 `NewAppWithPlatform` 注入平台实现，并由 `NewAppWithDependencies` 同时注入文件系统实现。配置导入、配置导出和查询结果导出均通过文件系统端口完成，不再直接调用 `os.ReadFile`、`os.WriteFile` 或 `os.Create`。取消文件对话框仍按原有约定返回空路径，不视为错误。

该接口让未来 Gio 入口可以注入 Gio 平台实现，而不需要让应用服务依赖 Wails 或 Gio。

## 尚未迁移

- Gio 对话框、剪贴板、系统通知等原生平台实现。
- Gio 原生文件对话框、剪贴板、系统通知等平台实现尚未完成。
- Wails 对话框适配器仍缺少可独立运行的交互测试；当前根目录测试覆盖的是 App 对平台端口的调用与错误传播。
- Community / Pro 功能接口与独立构建入口。

## 下一步顺序

1. 为应用服务引入可替换依赖接口，补充单元测试。
2. 已为旧 Wails 兼容入口补充 Fake Platform / Fake FileSystem 行为测试，覆盖文件选择取消、对话框错误、文件读取错误、Windows 路径文件名提取、结果导出取消、创建失败、写入失败和关闭失败；仍需执行 Go 测试验证。
3. 让旧 Wails 入口只承担绑定、生命周期和平台适配职责。
4. 执行 `gofmt`、`go test ./...`、`go vet ./...`、`go build ./...`，修复所有回归。
5. 再开始 Gio 主窗口、连接树和 SQL 编辑器技术原型。
6. 最后建立 Community / Pro 构建组合和跨平台性能基准。

## 验收原则

每批迁移都必须记录真实测试结果。当前代码通过 GitHub 文件接口提交，但本轮没有可用的本地 Go 执行环境；GitHub 当前 head 也未返回 Actions workflow runs 或 commit status，因此本批不能标记为测试通过。

原生 GUI 原型应以同一台机器、同一构建模式实测冷启动、空闲内存、查询期间内存、安装包压缩体积和大结果集滚动表现。目标值是优化方向，不是框架保证。
