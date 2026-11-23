# Gio 原生工作台全量重写计划

> 状态：执行中。此文档优先于此前渐进迁移、保留现有 Gio 原型的旧计划。
> 最后更新：2026-11-24

## 1. 重写范围

本项目不是把 Wails 的界面换成 Gio 后就算迁移完成。用户已明确要求：已经写出的程序也需要更改。因此现有 Gio 原型、旧 Vue 前端、旧应用服务和旧数据访问实现都不是最终架构。允许参考已有功能和行为，但不要求保留现有代码结构、组件组织、状态模型或 API。

最终交付必须是 Gio 原生 GUI + 新应用用例层 + 新领域模型 + 新基础设施适配层。旧代码只作行为参考，不能作为运行时兼容入口，也不能把简单搬迁、包装或改名算作重写完成。

## 2. 现有程序必须重写的范围

### 2.1 Gio GUI（全部重新设计与组织）

现有 internal/gui/gioapp/window.go 是约 82 KB 的单体窗口实现，混合了窗口事件循环、业务状态、异步任务、连接树、SQL 标签、历史 / 收藏、查询结果、筛选排序、导出和大量布局代码。该文件视为原型，不作为最终结构继续堆叠功能。

新 GUI 应按页面 / 组件 / 状态职责拆分，推荐结构：

- internal/gui/gioapp/app/：窗口启动、主题、全局导航、生命周期。
- internal/gui/gioapp/workspace/：工作区、SQL 标签、分栏布局和恢复。
- internal/gui/gioapp/connection/：连接列表、连接编辑、分组、连接状态。
- internal/gui/gioapp/explorer/：数据库、表、视图、系统表和集群拓扑。
- internal/gui/gioapp/editor/：SQL 编辑、执行控制、补全、格式化、错误定位。
- internal/gui/gioapp/result/：虚拟表格、列宽、排序、筛选、分页、单元格详情、复制和导出。
- internal/gui/gioapp/history/：查询历史。
- internal/gui/gioapp/snippets/：收藏 SQL / 片段。
- internal/gui/gioapp/settings/：主题、行数上限、布局和偏好设置。
- internal/gui/gioapp/components/：通用按钮、弹窗、表单、状态栏、空态和确认交互。

上述是目标职责边界，不要求一次性创建空目录。每迁移一个垂直功能，就把状态、事件处理、布局和测试一起迁出单体窗口。完成后删除旧 window.go 单体实现和旧原型测试。

### 2.2 用户交互与视觉设计（不照搬现有 GUI）

采用适合 ClickHouse 的个人桌面工作台布局：左侧连接 / 数据库对象树，中间 SQL 多标签编辑区，下方查询结果，右侧按需显示表结构 / DDL / 查询详情。保留 balanced 风格，但重新统一间距、层级、图标、空状态、错误反馈、加载反馈、快捷键、窄窗口布局和高密度表格交互。

必须重新检查并实现：
- 首次启动引导、连接新增 / 编辑 / 测试 / 删除 / 分组与排序。
- 多连接切换、断连状态、连接失败恢复、密码输入与本地安全存储。
- SQL 编辑器、多标签、未保存提示、执行 / 取消、历史回填。
- 结果虚拟滚动、列宽、复制、排序、筛选、NULL 展示、导出进度与取消。
- 表 / 视图 / 系统表浏览、表结构、DDL、集群拓扑。
- 查询历史、收藏 SQL、设置、导入导出。
- 深浅主题、窗口缩放、窄布局、键盘操作和可访问性。

不能仅因现有原型已有某个控件就视为新 GUI 验收通过。

## 3. 新后端目标边界

- internal/domain/：连接、查询、元数据、工作区、历史、收藏、设置等领域类型与规则，不依赖 Gio、文件格式或 ClickHouse 驱动。
- internal/application/：用例编排，例如连接管理、执行查询、取消查询、浏览元数据、管理历史 / 收藏、导入导出工作区。
- internal/infrastructure/clickhouse/：ClickHouse 驱动、连接池、协议、查询扫描、取消、元数据 SQL。
- internal/infrastructure/storage/：本地配置、迁移、原子写入、备份和恢复。
- internal/platform/：Gio 所需的系统文件选择、剪贴板、打开目录等平台能力；不引入 Wails 适配。
- internal/gui/gioapp/：只依赖 application 的用例接口和 domain DTO，不直接依赖旧 internal/app、旧 internal/config 或 ClickHouse 驱动模型。

ClickHouse 官方驱动可以保留为基础设施依赖；旧 internal/clickhouse.Service 的业务接口和内部实现不要求保留。配置文件可设计迁移导入器读取旧格式，但导入器必须是一次性数据迁移工具，不是旧业务服务兼容层。

## 4. 推荐迁移阶段与验收标准

### 阶段 A：新架构骨架
- 建立 domain / application / infrastructure 的依赖方向。
- 定义错误类型、结果分页 / 流式接口、取消机制和生命周期。
- 为每个用例定义接口与 fake 测试。
- Gio 入口不再实例化 internal/app.NewDefaultService。
验收：gioapp 不 import 旧 internal/app、internal/config、internal/clickhouse。

### 阶段 B：连接与配置
- 新连接模型、连接仓库、加密凭据存储策略、连接池生命周期。
- 新设置和工作区存储，采用原子写入与版本化 schema。
- 提供旧 JSON 配置的一次性导入和校验。
验收：新建 / 编辑 / 删除 / 测试 / 导入连接均由新用例完成；单元测试覆盖损坏配置、权限、写入中断和密码处理。

### 阶段 C：查询与元数据
- 新查询执行、取消、超时、行数上限、流式读取与资源释放。
- 新数据库 / 表 / 视图 / 表结构 / DDL / 集群元数据用例。
- 新查询历史记录策略。
验收：不依赖旧 internal/clickhouse.Service；fake 驱动测试与真实 ClickHouse 集成测试分层运行。

### 阶段 D：重写 GUI 工作区
- 按本文件第 2 节拆分单体 window.go。
- 所有界面通过 application 用例交互，后台 goroutine 结果回传使用明确的请求 ID / context，避免过期响应覆盖。
- 逐个恢复 SQL 多标签、历史、收藏、结果表格、筛选排序、导出等能力。
验收：布局 / 状态纯逻辑测试、交互回归测试、窄窗口与主题检查齐备。

### 阶段 E：删除旧程序
- 删除旧 internal/app、旧 internal/clickhouse 业务封装、旧 internal/config 管理器和旧平台抽象中不再使用的部分。
- 删除旧 frontend/ Vue 工程及 Wails 生成产物、配置、脚本和依赖。
- 清理 go.mod、文档、忽略规则和 CI。
验收：全仓库无 Wails/Vue 运行依赖、无旧业务包 import；唯一桌面入口为 Gio。

### 阶段 F：完整质量验收
- gofmt、go test ./...、go vet ./...、go build ./cmd/meteor-ch-gio。
- CI 覆盖 Windows / macOS / Linux 可构建性；至少一套真实 ClickHouse 集成测试。
- 验证旧配置导入、升级、备份恢复、取消查询、导出中断清理。
- 记录冷启动、空闲内存、典型查询内存、安装包体积的真实测量值。

## 5. 迁移进度统计口径

不再沿用历史 PR 描述中的 63% / 66% 等数字。新统计只按本计划阶段和验收项计算：
- 已实现但仍依赖旧服务：标记“原型可用 / 待重写”，不计入新架构完成度。
- 已搬到 Gio 但仍在单体 window.go：标记“原型功能”，不计入 GUI 重写完成度。
- 只有新领域 / 用例 / 基础设施实现、GUI 接入、测试与验收完成后，才计为完成。
- 旧代码删除与全平台验收独立统计，不能用功能数量代替。

## 6. 当前明确未完成项

- Gio 入口仍调用 internal/app.NewDefaultService。
- gioapp 仍直接引用旧 internal/app、internal/config、internal/clickhouse 类型。
- window.go 仍为大型单体原型。
- Vue frontend/ 尚未删除。
- 新 domain / application / infrastructure 目录尚未建立。
- 当前没有 GitHub Actions workflow；此前新增测试没有在本执行环境运行。
