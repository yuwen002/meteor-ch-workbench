// Package connection 编排连接管理用例，不依赖 GUI、文件存储或数据库驱动。
package connection

import (
	"context"

	"github.com/yuwen002/meteor-ch-workbench/internal/domain/connection"
)

// Repository 是连接配置的持久化端口。实现必须返回独立副本，避免外部修改内部状态。
type Repository interface {
	Get(ctx context.Context, id string) (connection.Connection, error)
	List(ctx context.Context) ([]connection.Connection, error)
	Save(ctx context.Context, item connection.Connection) error
	Delete(ctx context.Context, id string) error
	Reorder(ctx context.Context, ids []string) error
}

// Tester 执行真实连通性检查。实现负责连接池、超时与资源释放。
type Tester interface {
	Test(ctx context.Context, item connection.Connection) error
}

// IDGenerator 为新连接生成稳定且唯一的 ID。
type IDGenerator interface {
	NewID() (string, error)
}
