# 项目开发规则（Gio 原生架构）

## 架构约束

- 当前唯一桌面入口是 `cmd/meteor-ch-gio`，GUI 使用 Gio。
- 不新增 Wails 绑定、Vue API 桥接或旧前端兼容入口。
- 新业务按 `domain → application → infrastructure` 边界组织；Gio 通过 application/bootstrap 使用能力。
- 旧 `internal/app`、`internal/config`、`internal/clickhouse` 和 `frontend/` 仅作迁移参考，不得作为新功能依赖。
- 连接配置路径及旧格式兼容属于数据兼容要求，不能通过要求用户重新建连接来绕过。

## 开发前检查

1. 检查分支、PR 状态、最新 head SHA。
2. 检查当前 head 对应 Actions；旧 SHA 的成功不能代表新提交成功。
3. 阅读目标模块及测试，再做最小范围修改。
4. PR #1 保持 Draft；未经明确授权不得合并或修改 base。

## 校验命令

```bash
go test ./...
go vet ./...
go build ./cmd/meteor-ch-gio
```

- 改动 Go 代码必须运行相关测试；完整验收尽量运行以上三项。
- Windows GUI / Gio 依赖需由 Windows CI 构建验证；界面行为还需实机验证。
- 不能把未运行的本地命令描述为通过。
- CI 失败必须定位到具体 job、step 和日志根因后再修复。

## Gio 与并发规则

- Gio 控件只在窗口事件循环中读写；后台 goroutine 通过结果 channel 回传状态。
- 异步请求应携带序号或稳定 ID；过期结果必须丢弃。
- 窗口销毁时取消活动请求；后台 goroutine 不得因向已退出窗口发送结果而永久阻塞。
- SQL 标签以稳定 TabID 关联；禁止以可变数组索引关联查询结果。
- 纯状态逻辑优先抽到不依赖 Gio 的小模型，并提供表驱动单元测试。
- UI 视觉调整必须有明确需求；不要为了重构状态模型改变用户已确认的布局。

## 数据与安全

- 本地文件持久化应采用临时文件 + 原子替换，防止中断写入损坏配置。
- 连接配置读写必须兼容历史文件字段与默认值。
- 不在日志、错误提示或测试快照中泄露密码、Token 等凭据。
- 导入配置应验证格式、处理 ID 冲突，并提供可预测的覆盖/合并语义。

## 提交规范

采用 Conventional Commits，中文描述，例如：

```text
feat(query): 支持查询历史检索与重新执行
refactor(gui): 拆分 SQL 编辑器状态模型
test(workspace): 覆盖标签关闭后的状态恢复
docs(architecture): 更新原生 GUI 目录说明
```

不为纯目录美化做无意义搬迁；功能迁移、测试、文档应保持在同一交付链路中。
