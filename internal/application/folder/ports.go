// Package folder 编排连接分组管理用例。
package folder

import (
	"context"

	"github.com/yuwen002/meteor-ch-workbench/internal/domain/connection"
)

// Repository 是连接分组的持久化端口。
type Repository interface {
	ListFolders(context.Context) ([]connection.Folder, error)
	SaveFolder(context.Context, connection.Folder) error
	DeleteFolder(context.Context, string) error
}

// IDGenerator 为新分组生成稳定 ID。
type IDGenerator interface {
	NewID() (string, error)
}
