// Package settings 编排用户偏好设置用例。
package settings

import (
	"context"

	domain "github.com/yuwen002/meteor-ch-workbench/internal/domain/settings"
)

// Repository 负责偏好设置的读取和整体保存。
type Repository interface {
	Load(context.Context) (domain.Preferences, error)
	Save(context.Context, domain.Preferences) error
}
