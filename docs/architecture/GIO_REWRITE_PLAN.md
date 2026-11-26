# Gio 原生工作台全量重写计划

> 状态：执行中。此文档优先于此前渐进迁移、保留现有 Gio 原型的旧计划。
> 最后更新：2026-11-26

> 当前已完成 / 未完成清单、测试状态和下一步开发顺序统一查看 [开发进度与已完成功能](../DEVELOPMENT_STATUS.md)。本文负责长期目标架构、阶段定义与验收标准；下方历史执行记录不作为最新状态的唯一来源。

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

## 6. 当前执行记录（2026-11-24）

### 阶段 A 已落地的第一块
- 新增 `internal/domain/connection`：独立连接领域模型、协议类型、必要字段校验与不泄露凭据的端点描述。
- 新增 `internal/application/connection`：连接仓库 / 连通性测试 / ID 生成端口，以及列表、保存、删除、测试用例。
- 新增领域与应用层单元测试：覆盖必填字段、端口和协议边界、ID 生成失败、保存归一化、删除不存在连接、测试不落盘及底层错误链。
- 以上代码不依赖 Gio、旧 `internal/app`、旧 `internal/config`、旧 `internal/clickhouse`，属于新架构代码；但目前尚未接入 Gio，也没有新的 ClickHouse / storage 基础设施实现。

### 阶段 B：连接存储首批实现
- 新增 `internal/infrastructure/storage/connectionfile`：独立版本化 JSON 文件仓库，实现连接列表、保存、删除。
- 写入采用临时文件 + Sync + Rename；配置目录权限 0700、文件权限 0600，避免继续沿用旧配置管理器的宽松权限。
- 增加显式 `ImportLegacyJSON` 一次性导入入口：旧文件由调用方读取并传入，不在运行时依赖旧配置包；支持缺失 ID / protocol 归一化、重复 ID 校验和单次写入。
- 增加存储测试：新增/更新/列表/删除、文件权限、未知版本、重复 ID、旧配置导入及无效数据不产生部分写入。
- **尚未完成**：新仓库还未接入 Gio；密码目前仍随新配置文件保存，依靠本地文件权限保护，后续需要独立凭据存储；尚未有真实 ClickHouse Tester 实现。
- 测试文件已提交，但当前仅通过 GitHub 文件 API 写入，未在本地执行。

### 阶段 B：ClickHouse 连接基础设施首批实现
- 新增 `internal/infrastructure/clickhouse`：通过官方 clickhouse-go/v2 的 database/sql 驱动构建 Native / HTTP DSN，支持 TLS scheme、数据库、压缩及凭据 URL 转义。
- 新增连接池管理器：按连接 ID 复用连接池；连接参数（包含凭据）变化时通过 SHA-256 指纹识别并替换旧池；支持单连接失效和全量关闭。
- 新增连通性测试器：每次使用临时连接池，Ping 使用调用方 context + 默认 5 秒超时，结束后释放资源；错误信息只包含协议和 host:port，不包含用户名 / 密码。
- 新增测试覆盖 DSN 协议与 TLS scheme、凭据和数据库名转义、参数指纹、取消上下文、Ping 失败后的资源释放、超时、连接池复用与配置变化。
- **尚未完成**：没有真实 ClickHouse 实例集成测试；新连接池尚未接入查询用例或 Gio；TLS 使用驱动默认证书校验，尚无自定义 CA / 客户端证书配置 UI。

### 当前仍未完成项

- Gio 入口仍调用 `internal/app.NewDefaultService`，新连接用例和新连接仓库尚未接入运行时。
- `gioapp` 仍直接引用旧 `internal/app`、`internal/config`、`internal/clickhouse` 类型。
- `window.go` 仍为大型单体原型，GUI 需要按新设计整体重写。
- Vue `frontend/` 尚未删除，当前只作为行为参考。
- 新 ClickHouse infrastructure 目前只有连接池和连通性测试首批实现；查询执行、元数据、取消、凭据独立存储、历史与工作区存储尚未实现。
- 新连接存储和 ClickHouse 测试尚未实际执行；当前 Actions 查询无运行记录，最新提交状态为 pending。


### 阶段 C：SQL 查询执行首批实现（2026-11-24）

- 新增 `internal/application/query`：定义查询 / 执行请求、列元数据、批次回调、执行摘要、连接读取端口和执行器端口；用例按连接 ID 读取新连接仓库，再委托执行器。
- 新连接文件仓库增加 `Get(ctx, id)`，查询用例不再需要读取完整连接列表。
- 新增 `internal/infrastructure/clickhouse/query_executor.go`：使用新连接池执行 QueryContext / ExecContext；查询结果通过批次回调流式交付，不在应用层累积完整结果集。
- 默认查询超时 30 秒、结果行数上限 10,000、批次大小 256；支持请求级覆盖、context 取消、列名 / ClickHouse 类型 / Go 扫描类型元数据、截断标记和 DDL/DML 受影响行数。
- 增加应用用例测试和 fake database/sql driver 测试，覆盖连接读取、请求校验、列元数据、批次、行数截断、Exec 结果、取消和回调失败后的 Rows 释放。
- **尚未完成**：尚未在本地运行 gofmt / go test / go vet / build；仓库当前没有 `.github/workflows/`，因此没有可触发的 GitHub Actions 工作流。查询执行器尚未接入 Gio GUI，也没有真实 ClickHouse 集成测试。结果回调的消费方必须自行限制 UI 队列 / 缓冲区，不能把所有批次重新累积到内存。


### 阶段 D：ClickHouse 元数据读取首批实现（2026-11-24）

- 新增 `internal/application/metadata`：定义数据库对象、字段结构、表详情 DTO，以及 Reader 端口和应用服务；按连接 ID 读取连接配置，校验数据库 / 表名称。
- 新增 `internal/infrastructure/clickhouse/metadata.go`：查询数据库列表、普通表、视图、DESCRIBE 字段结构，以及 system.tables 中的引擎、注释、完整建表 SQL、分区键、排序键、主键和 TTL 摘要。
- 数据库和表名称值使用 database/sql 参数绑定；DESCRIBE 标识符通过反引号引用并转义，避免把用户输入直接拼成 SQL 标识符。
- DESCRIBE 结果按列名匹配字段，不依赖 ClickHouse 版本间返回列的固定顺序；主键与默认值标志兼容多种驱动值类型。
- 新增应用服务测试、元数据 SQL 测试和 TTL / 标识符解析测试。
- **尚未完成**：未运行本地 gofmt / go test / go vet / build；仓库无 GitHub Actions workflow；尚无真实 ClickHouse 集成测试，尚未接入 Gio GUI。

### 元数据字段标记兼容修正（2026-11-24）

- `GetTableSchema` 同时识别 `is_in_primary_key` 与 `primary_key`，兼容不同 ClickHouse / 驱动返回的字段标记名称。
- 默认值标记同时识别 `default_type`、`default_kind` 和 `default_expression`，并采用 OR 合并，避免返回列顺序或字段版本差异导致标记被覆盖。
- fake driver 测试切换为 `is_in_primary_key` + `default_kind` 组合，验证新版字段元数据路径。
- 尚未运行本地 gofmt / go test；仍需使用目标 ClickHouse 22.5 实例验证真实 DESCRIBE 输出。


### 新架构应用服务装配入口（2026-11-24）

- 新增 `internal/bootstrap`：集中创建新连接仓库、连接管理用例、ClickHouse 连接池、SQL 查询用例和元数据用例；GUI 不需要自行实例化基础设施对象。
- 新增 `New(dataDir)` 与 `NewDefault()`，数据文件使用独立的 `connections.json`；`Services.Close` 统一释放新连接池。
- 连接应用服务新增 `Get(ctx, id)`，连接仓库端口增加按 ID 查询能力，使连接应用服务可以作为查询和元数据用例的 ConnectionLookup。
- 新增服务装配测试和连接按 ID 读取测试。
- 当前 Gio 仍使用旧 `internal/app.Service`，本次只建立新架构组合根，下一步将创建独立 Gio 应用状态 / presenter，逐步替换旧窗口调用；不把旧服务作为新架构兼容层。
- 尚未运行本地 gofmt / go test / go vet / build；装配测试仅已编写。


### 元数据字段来源修正与连接侧栏首个 UI 组件（2026-11-24）

- 将表字段读取从 `DESCRIBE TABLE` 改为查询 `system.columns`，显式读取 `name`、`type`、`default_kind`、`default_expression`、`comment` 和 `is_in_primary_key`。
- 该调整避免把 `DESCRIBE TABLE` 的字段集误当作包含主键标记；同时数据库名、表名改为查询参数，避免拼接用户输入。
- fake driver 测试同步改为模拟 `system.columns` 返回，并检查数据库 / 表参数绑定以及主键、默认值标记。
- 新增 `internal/gui/gioapp/connection/view.go`：独立 Gio 连接侧栏，展示连接名称、地址、协议和选中状态，提供刷新回调与错误 / 加载状态；点击连接更新 Model 选择并通知窗口层。
- 侧栏目前尚未挂载到主窗口，未替换旧连接 UI；主窗口仍依赖旧服务。新视图代码和测试尚未在本地执行，仍需 gofmt、go test 和实际 Gio 窗口验收。

### 新 Gio 连接管理状态模型（2026-11-24）

- 新增 `internal/gui/gioapp/connection` 状态模型，依赖新连接应用服务接口，不再引用旧 `internal/app.Service`、旧配置管理器或旧 ClickHouse 业务模型。
- 提供连接列表快照、选中连接、加载 / 保存 / 测试状态、错误状态，以及 Reload / Select / Save / Delete / Test 操作；耗时服务调用在状态锁外执行，列表刷新通过序号避免较早请求覆盖较新结果。
- 增加纯逻辑测试，覆盖列表加载、选择校验、保存与删除后的状态更新、错误传播。
- 这是新 GUI 的连接功能状态层，尚未接入 Gio 布局与事件循环；旧单体 `window.go` 仍为当前运行界面，不能计为连接管理 GUI 重写完成。


### 新 Gio 入口与连接 / 数据库浏览接线（2026-11-24）

- cmd/meteor-ch-gio 已从旧 internal/app.NewDefaultService 切换为 internal/bootstrap.NewDefault，窗口改为调用独立的新架构 internal/gui/gioapp/app.Run；退出时统一关闭新连接池。
- 新 Gio 工作区已挂载连接侧栏状态模型与视图，支持加载本地连接、选择连接、连通性测试、读取数据库列表，以及读取指定数据库的表 / 视图列表。
- 连接和数据库加载使用请求序号校验，旧请求即使在相同连接 / 数据库重复选择场景下返回，也不会覆盖当前界面状态。
- 切换连接或数据库时取消前一个请求；结果队列满时丢弃已失效结果，避免后台 goroutine 因窗口退出而永久阻塞。
- 本次只接通连接与数据库对象浏览基础流程；连接编辑表单、表字段 / DDL 详情、SQL 编辑器与结果集仍未接入新工作区。
- 旧 internal/gui/gioapp/window.go 仍保留为旧实现代码，但新桌面入口不再调用旧 internal/app.Service；后续应按模块补齐新界面，再删除旧窗口与旧业务运行时代码。
- 当前环境仍未执行本地 gofmt、go test ./...、go vet ./... 或 go build ./cmd/meteor-ch-gio；需要本地完整编译验证 Gio API 和包间类型兼容性。


### GUI 可用性整顿第一轮（2026-11-24）

- 暂停扩展新后端功能，优先整理当前新 Gio 工作区：顶部应用栏、连接侧栏、数据库 / 表视图浏览区、底部统一状态反馈；重新分配连接区与浏览区的空间，并补充数据库 / 对象数量和空状态提示。
- 增加连接刷新、数据库对象刷新操作；数据库和对象列表初始化为纵向滚动列表。
- 点击表对象后，通过新 metadata application service 并行工作流依次读取字段结构与表详情，展示字段名、类型、主键 / 默认值标记、引擎、主键、分区键、排序键和 TTL；请求支持取消与序号校验。
- 修复连接快速切换时状态模型只允许一个连接测试的问题：新测试可以取代旧测试状态，旧测试晚返回不再清除新测试的状态；新增并发竞争回归测试。
- 切换连接时清空上一连接遗留的数据库、表结构和对象选中状态。
- 仍未完成：连接新增 / 编辑 / 删除的完整表单交互、分组管理、SQL 编辑器、多标签、执行 / 取消、结果表格、筛选导出、历史和收藏；当前 GUI 只能算新工作区第一版，不是可用产品完成态。
- 未执行本地 gofmt、go test、go vet 或 build；需要本地 Gio 编译和实际窗口截图验收后才能确认布局效果。

### 预览构建门禁（2026-11-24）

- README 已改为当前 Gio 原生预览版说明，不再把旧 Wails / Vue 功能清单描述成当前可用能力。
- Go CI 在 Linux runner 安装 Gio 所需图形开发库，依次执行 `go test ./...`、`go vet ./...`、构建 `cmd/meteor-ch-gio`。
- CI 构建成功后上传 `meteor-ch-workbench-linux-amd64` Artifact，保留 7 天，可从 Actions 对应最新 head run 下载预览二进制。
- 预览验收口径：Artifact 必须来自 PR 当前 head SHA，且该 SHA 的测试、vet、build 全部成功；旧 SHA 的成功构建不算当前版本可预览。
- Linux Artifact 是可执行预览二进制，不是免依赖安装包；目标机器仍需 Gio 对应的图形运行库。Windows / macOS 产物和真实桌面窗口验收仍未完成。
- 当前连接侧栏已出现新增连接表单，但编辑、删除、分组及完整保存反馈仍待实现；SQL 编辑器和结果表格也尚未接入新工作区。
- 本轮未宣称构建通过；提交后需等待最新 head 的 CI run 完成，并按实际日志修复失败项。


### 新工作区连接创建流程（2026-11-24）

- 连接侧栏增加新增连接表单，支持连接名称、主机、端口、用户名、密码、默认数据库、Native / HTTP 协议和 TLS 开关。
- 新连接通过连接应用服务保存，由服务层生成 ID 并执行领域校验；保存成功后自动测试连接并加载数据库。
- 密码输入使用遮蔽显示。当前连接表单仅覆盖新增，编辑、删除、分组和排序仍待实现。
- 连接密码当前仍随本地连接 JSON 持久化；文件权限为 0600，尚未迁移到系统钥匙串 / 独立凭据存储。
- Go CI 已增加 Linux Gio 原生依赖安装、模块预下载、全量测试、vet 和桌面二进制构建；当前以最新 head 的运行结果作为验收依据。


## 7. 最新迁移进度（2026-11-25，按全量重写口径）

本节覆盖并修正此前「旧功能迁移」口径。只统计新架构和新 Gio 入口，不把旧 Vue / 旧服务 / 旧单体窗口已有功能计入完成。

| 阶段 | 权重 | 当前完成度 | 已有证据 | 主要剩余项 |
|---|---:|---:|---|---|
| A. 新架构骨架 | 15% | 80% | domain/connection、application/connection/query/metadata/folder、bootstrap 已建立；Gio 入口已改用 bootstrap | 统一错误与分页/流式契约；为全部用例补齐 fake 测试；继续拆分 GUI presenter |
| B. 连接与配置 | 20% | 65% | 新连接领域模型、版本化连接 JSON 仓库、原子写入、旧 JSON 显式导入；新 Gio 已接连接新增/编辑/删除、测试、分组与连接归组 | 系统钥匙串/凭据隔离；设置和工作区独立存储；连接排序持久化、损坏文件恢复与备份 |
| C. 查询与元数据 | 20% | 50% | 新 query executor 支持 QueryContext/ExecContext、批次回调、超时/取消/行数上限；新 metadata 支持库/表/视图/字段/DDL；新 Gio 已接入对象浏览和表详情 | 查询历史/收藏/设置的新存储与用例；真实 ClickHouse 集成测试；批次到 UI 的有界背压与资源释放验收 |
| D. Gio 工作区重写 | 25% | 30% | 新 Gio 入口和连接/分组侧栏已接新 application；数据库对象与表详情已接新 metadata；新窗口已开始承载 SQL 工作区 | app/run.go 仍约 51 KB，状态/布局/事件仍耦合；需拆 workspace/editor/result/explorer/history/snippets；补窄窗口、主题和交互回归 |
| E. 删除旧程序 | 10% | 0% | Wails 根入口、绑定门面和配置已删除 | 旧 internal/app、internal/clickhouse、internal/config、旧 gioapp/window.go、frontend Vue 及相关依赖仍在；需等替代验收后删除 |
| F. 完整质量验收 | 10% | 40% | 当前 head 的 Windows 与 Linux CI 已完成 go test、go vet、构建并上传预览产物 | macOS CI、真实桌面交互、真实 ClickHouse 集成、凭据迁移、安装包和性能基准仍缺 |

按上述阶段权重计算：15%×80% + 20%×65% + 20%×50% + 25%×30% + 10%×0% + 10%×40% = **46.5%，约 47%**。

这代表「新架构全量重写」的进度，不是旧功能可用率。若只看新 Gio 用户可见功能，连接新增/编辑/删除/分组、数据库对象浏览和表详情已有新入口；完整 SQL 编辑、结果、历史/收藏、设置及 ClickHouse 管理功能仍未全部完成，因此不宜把旧原型功能算作已迁移。

### 当前 head 验收记录

- 分支：`feat/native-gui-foundation`
- head：`8db7fe4089a535535ea7a01e9cfe1aabb8c4f18b`
- CI Run：`45436110042`，Windows preview 与 Linux Test and build 均为 success；两端均执行测试、vet 和构建。
- PR #1：Open / Draft / 未合并，base 为 main；分支比 main ahead 554、behind 1。
- CI 通过不等于桌面交互验收；本轮没有真实 Windows/macOS/Linux 桌面运行和 ClickHouse 服务端集成验证。

### 下一阶段执行顺序

1. **先拆出 workspace 状态与 SQL 标签生命周期**：将标签集合、活动标签、关闭/重命名/排序、未保存判断和查询运行态归档从 app/run.go 提取为独立 workspace 包，补纯逻辑测试；避免继续向 51 KB 窗口文件堆功能。
2. 再拆 editor 与 result：编辑器状态、执行控制、结果批次有界队列、筛选排序和导出分别独立。
3. 新建 history / snippets / settings 的 domain、application、storage 和 Gio 组件，不能复用旧 config manager。
4. 新组件全部接入并验收后，才删除旧业务包和 Vue frontend。


## 8. 最新迁移进度（2026-11-26，严格全量重写口径）

本次按阶段 A–F 重新核对代码结构、Gio 新入口、应用服务、基础设施和最新 CI。统计规则沿用第 5 节：旧 Vue、旧业务服务和旧 Gio 单体窗口中的功能不计入新架构完成；只有新入口实际接入且有相应测试的能力才计入。此数字是工程进度估算，不等于用户可见功能覆盖率。

| 阶段 | 权重 | 完成度 | 当前判断 | 主要剩余 |
|---|---:|---:|---|---|
| A. 新架构骨架 | 15% | 80% | domain / application / infrastructure / bootstrap 已建立，Gio 新入口使用新服务 | 统一错误与流式契约、补齐边界测试，继续降低 GUI 与用例细节耦合 |
| B. 连接与配置 | 20% | 65% | 新连接仓库、旧 JSON 一次性迁移、连接 CRUD / 测试 / 分组已有新入口 | 系统钥匙串或独立凭据存储、设置与工作区版本化存储、备份恢复及损坏配置处理 |
| C. 查询与元数据 | 20% | 55% | 新查询执行器、取消 / 超时 / 批次回调、数据库对象与表详情、历史 / 收藏应用服务及存储已建立并接入 | 真实 ClickHouse 集成测试、历史与收藏完整验收、查询批次背压和资源释放、更多元数据能力 |
| D. Gio 工作区重写 | 25% | 35% | 新 Gio 入口已接连接分组、对象浏览、SQL 编辑 / 多标签、结果视图、历史和收藏 | app/run.go 仍约 77 KB；需继续拆分 workspace / editor / result / explorer / history / snippets / settings，补齐设置和窄窗口交互验收 |
| E. 删除旧程序 | 10% | 0% | 根 Wails 启动入口与绑定门面已删除 | 删除旧 internal/app、旧 internal/config、旧 internal/clickhouse、旧 gioapp/window.go、Vue frontend/ 和不再需要的依赖 |
| F. 完整质量验收 | 10% | 45% | 当前 head 的 Linux 与 Windows CI 已通过测试、vet 和构建 | macOS CI、三平台真实桌面交互、真实 ClickHouse 集成、凭据迁移、安装包与性能基准 |

加权估算：15%×80% + 20%×65% + 20%×55% + 25%×35% + 10%×0% + 10%×45% = **49.25%，约 49%**。

- **全量重写工程进度：约 49%，剩余约 51%。**
- **旧功能在新 Gio 中的可见替代：**已有连接管理、分组、对象浏览、SQL 编辑 / 多标签、结果视图、历史和收藏等主要路径；但因设置、ClickHouse 管理操作、完整编辑器能力和跨平台验收仍缺，不把旧功能列表的总数直接当成完成率。
- 旧版 docs/FEATURES.md 中的功能描述是历史行为清单，不表示所有功能已在新架构中完成。

### 本次快照的验收基准

- 分支：feat/native-gui-foundation
- head：e7a40986e2489ca168e4711130ca001549371fe0
- 当前 head CI：Run 45763276733，Linux 与 Windows 检查均成功。
- PR #1：Open / Draft / 未合并，base 为 main。
- CI 通过不等于真实桌面验收；当前没有本轮本地 Go 测试或 Windows / macOS GUI 实机验证记录。

### 下一步开发顺序

1. 继续把 SQL 标签状态、结果视图状态与异步请求生命周期从 app/run.go 拆到独立 workspace / result 模块，补齐切换、关闭标签和迟到响应测试。
2. 将查询结果批次处理与 UI 有界队列独立出来，避免大结果集切换标签时重复复制全部数据。
3. 建立 settings 的 domain / application / storage 和 Gio 页面，迁移主题、行数上限、布局与刷新偏好。
4. 补齐真实 ClickHouse 集成测试与 macOS CI；完成后再分批删除旧业务包和 Vue 前端。

### SQL 查询事件生产与消费解耦（2026-11-26）

- 将查询执行、批次回调、有限队列入队与取消收敛在 `query_events.go`；将事件有效性检查、查询历史写入、活动标签结果更新和非活动标签结果归档拆到 `query_result_consumer.go`。
- 查询事件按查询 generation 与稳定 TabID 双重校验；标签已关闭或查询已被新请求 / 取消操作取代时，迟到事件直接丢弃。
- 增加过期 generation、已关闭标签、有效标签事件的单元测试；保留队列背压、取消退出和查询结果分批测试。
- `sql_tabs.go` 继续负责标签连接上下文和结果视图状态；`run.go` 只负责事件循环中的通道分发，SQL 查询事件不再在窗口文件中展开处理。
- 当前代码 CI：Linux 与 Windows 均完成 `go test ./...`、`go vet ./...`、Gio 预览构建及 Artifact 上传，Run 45768140849 成功。
- 本轮仍未执行本地测试或实际桌面 GUI 验收。下一步应把查询结果状态归档进一步下沉到独立 workspace/result 模型，并重点验证大结果集切换标签时的内存占用；随后再处理 settings 模块。


## 2026-11-26：SQL 标签状态归档与文档基线修订

- 新建标签、切换标签、关闭活动标签均通过统一状态归档函数保存编辑器和结果视图状态，包含筛选条件、排序方向、列宽、滚动位置及选中单元格。
- 查询结果快照复制行切片，并对 `[]byte` 值独立复制，避免驱动缓冲区或活动结果修改时污染其他标签的快照。
- 增加状态归档与字节值隔离测试；CI 曾在 `328c7ac` 因缺少 `cloneQueryRows` 编译失败，本轮补齐实现后重新运行 CI，不把旧失败或其他 SHA 的成功记录混作当前验收。
- 文档口径调整：`docs/FEATURES.md` 是旧版完整功能行为清单，`docs/API.md` 是旧 Wails 绑定 API 归档；当前 Gio 功能状态以 README 和最新迁移记录为准。
- 下一步优先验证标签关闭后的状态清理、标签重排时稳定 TabID 与点击控件映射，再继续拆分结果状态和设置模块。旧程序只在新功能替代并验收后删除。


### SQL 标签控件与生命周期（2026-11-26）

- 标签切换 / 关闭控件由索引切片改为 `map[TabID]*widget.Clickable`，避免重排后控件状态错配。
- 关闭标签通过 TabID 定位，并同步删除标签结果状态和两个 Clickable 映射；至少保留一个标签的既有规则不变。
- 测试覆盖稳定 ID 控件绑定和目标控件清理。后续继续补充关闭活动 / 非活动标签的集成生命周期测试，并将结果快照状态逐步从窗口层抽离。


### 窗口关闭与资源释放（2026-11-26）

- 后台任务通过 workspace.goWorker 统一登记，并继承窗口生命周期 Context；窗口销毁后先取消请求，再等待任务退出。
- 历史实现曾尝试在 Gio 主循环返回后等待 Run goroutine 并 defer 关闭连接池；Windows 的 Gio app.Main 不会返回，该清理顺序不可用。当前实现已调整为 gioapp.Run 返回后直接关闭 services，再显式结束进程。
- 生命周期单测覆盖 WaitGroup 等待和取消后拒绝新任务；Linux / Windows CI 负责测试、vet 和构建。
- 尚未完成：在 Windows / Linux / macOS 实际桌面环境验证关闭窗口后进程退出、文件选择器关闭、查询取消及连接池释放；CI 构建成功不能替代这项实机验收。
- 正确构建入口：go build ./cmd/meteor-ch-gio；根目录不存在 main.go。


### 窗口关闭竞态加固（2026-11-26）
- 后台任务注册现在由 `lifecycleMu` 保护：检查窗口关闭状态与 `WaitGroup.Add` 在同一临界区内完成。
- `shutdownWorkers` 先关闭新任务入口并取消窗口生命周期，再等待已登记任务全部退出，避免 `WaitGroup.Wait` 与并发 `Add` 在计数归零时发生竞态。
- DestroyEvent 先取消当前请求和活动查询，再关闭任务入口并等待任务退出，随后 `gioapp.Run` 返回；当前 Windows 入口在窗口 goroutine 中关闭服务容器并显式结束进程，不再依赖 `runDone` 或 `gio.Main()` 返回。
- 新增并发注册 / 关闭交错测试。CI 通过仍不替代 Windows、macOS、Linux 实机关闭与进程退出验证。


### 窗口关闭竞态加固（2026-11-26)
- `goWorker` 通过 `lifecycleMu` 将关闭状态检查和 `WaitGroup.Add` 串行化；`shutdownWorkers` 关闭任务入口、取消 Context 后等待已登记任务。
- 最新代码验收基线：`280e603b05a7d245a6ea039fb87f02be320d7a46`；CI Run `45778693845` 的 Linux / Windows 测试、vet 与预览构建均成功。
- 导出数据写入阶段响应生命周期取消；系统文件保存对话框仍不能由 Context 主动中断，桌面端关闭行为待实机验证。


### 保存对话框与窗口关闭解耦（2026-11-26）
- Gio Explorer 的原生 CreateFile 不提供 Context 取消能力，因此不能放入 shutdownWorkers 的 WaitGroup，否则窗口关闭会等待用户完成对话框操作。
- 保存对话框调用拆为独立等待阶段；返回后先检查窗口生命周期，若窗口已关闭则关闭并尝试删除目标文件，不再启动写入任务。
- goWorker 返回是否成功登记，处理“文件对话框返回时窗口已开始关闭”的竞态；登记失败会关闭并清理刚创建的文件。
- invalidateIfActive 在生命周期锁内检查关闭状态并调用窗口 Invalidate，避免独立对话框 goroutine 在窗口销毁后访问窗口。
- 当前仍需目标桌面平台验证原生对话框是否随父窗口正常关闭；CI 不能替代此项验证。


### 启动窗口尺寸与 Windows 退出生命周期（2026-11-26）
- `cmd/meteor-ch-gio` 启动时按 Windows 主显示器工作区和系统 DPI 计算 Gio 逻辑尺寸，初始窗口目标占用约 82% 工作区，并限制最大尺寸及边缘留白。
- 小屏幕按可用工作区缩小，避免固定 1080×660 在高 DPI / 小屏幕设备上超出可见区域。
- 非 Windows 平台当前使用保守默认尺寸；后续需分别接入 macOS / Linux 工作区与 DPI 查询，不能把回退值视为已完成跨平台自适应。
- Gio 0.10.3 的 Windows `app.Main` 进入主线程的永久等待；窗口 DestroyEvent 后，窗口事件循环返回时必须完成服务关闭并主动结束进程，否则主 goroutine 仍停在 `app.Main`，可能触发 runtime deadlock fatal error。
- 启动 goroutine 现在在 `gioapp.Run` 返回后显式关闭服务，再按正常关闭 / 错误退出码结束进程；不得依赖 `app.Main()` 返回后再执行清理。
- 仍需 Windows 实机验证正常关闭、启动初始化失败和窗口创建失败三种退出路径。


### 收藏 SQL 面板拆分与控件身份修复（2026-11-26）
- 收藏 SQL 的加载、保存、重命名、删除和面板布局从 `run.go` 拆分至 `saved_sql_view.go`，让窗口入口文件继续聚焦事件循环和页面组合。
- 收藏列表的打开 / 改名 / 删除控件由按可见索引访问的切片改为按稳定收藏 ID 维护的控件映射。
- 修复搜索过滤后使用过滤索引访问原始控件切片造成的错绑问题；刷新列表时保留仍存在条目的控件身份，并清理已删除条目的控件引用。
- 新增控件身份稳定与已删除条目清理测试；最新 head CI 尚在执行，未提前标记通过。


### 设置模块新架构接入（2026-11-26）
- 新增 `domain/settings`，集中定义查询行数范围、默认值和偏好归一化规则。
- 新增 `application/settings` 与 `infrastructure/storage/settingsfile`，采用应用层仓库端口和本地 JSON 持久化；文件使用受限权限、临时文件写入和替换回滚。
- Bootstrap 服务容器统一装配 Settings 服务，设置文件位于应用数据目录下的 `settings.json`，不改变既有连接配置路径。
- Gio 启动时加载偏好；设置面板可修改查询结果行数上限（0 表示不限制）、侧栏折叠状态和编辑器 / 结果区高度比例，新执行的查询立即使用新的行数上限。
- 设置偏好新增 `theme`（`light` / `dark`），启动时恢复主题；设置面板可即时切换并持久化深浅色，统一替换工作区视觉令牌和 Material 主题前景 / 背景色。
- 查询执行器的截断判断仅在 `MaxRows > 0` 时生效；`MaxRows == 0` 不再在首行前误判为截断，负数仍使用安全默认上限。
- 查询执行器现将 `MaxRows=0` 解释为不限制，负数才使用安全默认值；对应测试覆盖了完整结果返回。
- 主题切换尚未接入；本轮 CI 未完成前不标记验收通过。


### SQL 工作区布局拆分（2026-11-26）

- 将 `clearQueryCellSelection`、`layoutSchemaStrip`、`layoutQueryWorkspace`、`layoutQueryPanel`、`layoutQueryToolbar`、`layoutTabStrip`、`layoutSQLTab`、`layoutSQLEditor` 和 `layoutStatusBar` 从 `run.go` 移至 `query_workspace_view.go`。
- 拆分只移动视图方法，不改变 `workspace` 状态归属、查询执行流程、标签状态保存 / 恢复和事件处理顺序。
- 后续继续把对象导航 / Explorer 视图与状态拆分，并为标签切换、关闭、窄窗口布局和设置比例增加回归覆盖。
- 本次拆分的 CI 结果以最新 head SHA 对应的 Actions 为准；提交前一基线 `ee5ff0685f4de2af866ada2d45fd0867aeac83f2` 的 Linux / Windows CI 已通过。


### 2026-11-27：Explorer 视图拆分

- 对象导航布局已从窗口主文件抽离至 `internal/gui/gioapp/app/explorer_view.go`，覆盖数据库节点、表 / 视图分组、对象节点、资源树行和分组标题。
- `run.go` 保留窗口事件循环、服务调用、页面组合及状态字段；本轮不移动元数据请求状态，避免把纯视图拆分扩大为行为重构。
- 后续先梳理数据库展开 / 折叠、对象选中和异步元数据请求的状态边界，再决定提取 explorer presenter/model；同时补充快速切换数据库时迟到结果与选中状态一致性测试。


### 2026-11-27：Explorer 异步响应校验抽取

- 新增 `explorer_state.go`，把对象列表 / 表结构响应是否仍有效的判断从 `drainResults` 提取为纯函数。
- 对象列表响应必须同时匹配请求序号、连接 ID 和数据库名；表结构响应还必须匹配表名。切换连接、数据库或表后，迟到响应不能更新当前导航状态。
- 新增对应表驱动单元测试，覆盖有效响应与各上下文维度失效。
- 本次只抽离纯校验规则，异步请求创建、取消、请求序号递增仍由 `workspace` 管理；后续再考虑将其聚合为独立 Explorer 请求协调器。
