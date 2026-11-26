package app

import queryapp "github.com/yuwen002/meteor-ch-workbench/internal/application/query"

// queryResultState 只保存当前活动标签的查询结果快照。
// 标签切换时由 sqlTabState 负责归档；此状态不持有 Gio 控件或执行生命周期。
type queryResultState struct {
	queryColumns []queryapp.Column
	queryData    [][]any
	queryStatus  string
}

// apply 将一个有效事件合并到活动结果快照，并返回需要由窗口层处理的 UI 更新标记。
func (s *queryResultState) apply(result queryEvent) (columnsChanged, rowsChanged, finished bool) {
	if s == nil {
		return false, false, false
	}
	if len(result.columns) > 0 {
		s.queryColumns = append([]queryapp.Column(nil), result.columns...)
		s.queryData = nil
		columnsChanged = true
	}
	if len(result.batch) > 0 {
		s.queryData = append(s.queryData, result.batch...)
		rowsChanged = true
	}
	if result.done {
		s.queryStatus = queryResultStatus(result, s.queryColumns)
		finished = true
	}
	return columnsChanged, rowsChanged, finished
}
