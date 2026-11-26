package folder

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/yuwen002/meteor-ch-workbench/internal/domain/connection"
)

// Service 提供连接分组列表、新建、重命名和删除用例。
type Service struct {
	repository Repository
	ids IDGenerator
}

func NewService(repository Repository, ids IDGenerator) (*Service, error) {
	if repository == nil { return nil, errors.New("分组仓库不能为空") }
	if ids == nil { return nil, errors.New("分组 ID 生成器不能为空") }
	return &Service{repository: repository, ids: ids}, nil
}

func (s *Service) List(ctx context.Context) ([]connection.Folder, error) {
	return s.repository.ListFolders(ctx)
}

// Save 新建或重命名分组；新分组由 IDGenerator 分配 ID。
func (s *Service) Save(ctx context.Context, item connection.Folder) (connection.Folder, error) {
	item.Name = strings.TrimSpace(item.Name)
	if item.ID == "" {
		id, err := s.ids.NewID()
		if err != nil { return connection.Folder{}, fmt.Errorf("生成分组 ID: %w", err) }
		if strings.TrimSpace(id) == "" { return connection.Folder{}, errors.New("分组 ID 生成器返回空值") }
		item.ID = id
	}
	if err := item.Validate(); err != nil { return connection.Folder{}, err }
	if err := s.repository.SaveFolder(ctx, item); err != nil {
		return connection.Folder{}, fmt.Errorf("保存分组: %w", err)
	}
	return item, nil
}

// Reorder 按给定 ID 顺序重排分组；未提供的分组保留原相对顺序。
func (s *Service) Reorder(ctx context.Context, ids []string) error {
	if err := ctx.Err(); err != nil { return err }
	return s.repository.ReorderFolders(ctx, append([]string(nil), ids...))
}

// Delete 删除分组；底层仓库负责将该分组内的连接移回根级。
func (s *Service) Delete(ctx context.Context, id string) error {
	id = strings.TrimSpace(id)
	if id == "" { return connection.ErrMissingFolderID }
	if err := s.repository.DeleteFolder(ctx, id); err != nil {
		return fmt.Errorf("删除分组: %w", err)
	}
	return nil
}
