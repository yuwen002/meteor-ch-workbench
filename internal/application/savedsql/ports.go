// Package savedsql 编排收藏 SQL 用例。
package savedsql

import (
	"context"

	"github.com/yuwen002/meteor-ch-workbench/internal/domain/savedsql"
)

type Repository interface {
	List(context.Context) ([]savedsql.Entry, error)
	Save(context.Context, []savedsql.Entry) error
}

type IDGenerator interface {
	NewID() (string, error)
}
