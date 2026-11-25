// Package history 编排查询历史用例。
package history

import (
	"context"

	"github.com/yuwen002/meteor-ch-workbench/internal/domain/queryhistory"
)

// Repository 读写完整的查询历史列表。
type Repository interface {
	List(context.Context) ([]queryhistory.Entry, error)
	Save(context.Context, []queryhistory.Entry) error
}

// IDGenerator 为历史记录生成唯一 ID。
type IDGenerator interface {
	NewID() (string, error)
}
