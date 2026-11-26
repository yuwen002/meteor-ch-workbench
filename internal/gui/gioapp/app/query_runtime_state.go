package app

import (
	"context"

	workspaceui "github.com/yuwen002/meteor-ch-workbench/internal/gui/gioapp/workspace"
)

// queryRuntimeState 聚合 SQL 查询执行代次、取消函数和运行标记。
// 它只由 Gio 窗口事件循环读写；执行器 goroutine 通过独立 Context 与结果队列通信。
type queryRuntimeState struct {
	queryExecution  workspaceui.QueryExecution
	queryCancelFunc context.CancelFunc
	queryRunning    bool
}
