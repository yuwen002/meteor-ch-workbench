package connection

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/yuwen002/meteor-ch-workbench/internal/domain/connection"
)

var (
	ErrNotFound = errors.New("连接不存在")
)

// Service 提供连接列表、保存、删除和连通性测试用例。
type Service struct {
	repository Repository
	tester Tester
	ids IDGenerator
}

func NewService(repository Repository, tester Tester, ids IDGenerator) (*Service, error) {
	if repository == nil { return nil, errors.New("连接仓库不能为空") }
	if tester == nil { return nil, errors.New("连接测试器不能为空") }
	if ids == nil { return nil, errors.New("连接 ID 生成器不能为空") }
	return &Service{repository: repository, tester: tester, ids: ids}, nil
}

func (s *Service) Get(ctx context.Context, id string) (connection.Connection, error) {
	id = strings.TrimSpace(id)
	if id == "" {
		return connection.Connection{}, connection.ErrMissingID
	}
	item, err := s.repository.Get(ctx, id)
	if err != nil {
		return connection.Connection{}, fmt.Errorf("读取连接: %w", err)
	}
	return item, nil
}

func (s *Service) List(ctx context.Context) ([]connection.Connection, error) {
	return s.repository.List(ctx)
}

// Save 校验并保存连接。新连接由 IDGenerator 分配 ID；更新连接保留原 ID。
func (s *Service) Save(ctx context.Context, item connection.Connection) (connection.Connection, error) {
	item.Name = strings.TrimSpace(item.Name)
	item.Host = strings.TrimSpace(item.Host)
	item.User = strings.TrimSpace(item.User)
	item.Database = strings.TrimSpace(item.Database)
	if item.ID == "" {
		id, err := s.ids.NewID()
		if err != nil { return connection.Connection{}, fmt.Errorf("生成连接 ID: %w", err) }
		if strings.TrimSpace(id) == "" { return connection.Connection{}, errors.New("连接 ID 生成器返回空值") }
		item.ID = id
	}
	if err := item.Validate(); err != nil { return connection.Connection{}, err }

	if err := s.repository.Save(ctx, item); err != nil {
		return connection.Connection{}, fmt.Errorf("保存连接: %w", err)
	}
	return item, nil
}

func (s *Service) Delete(ctx context.Context, id string) error {
	id = strings.TrimSpace(id)
	if id == "" { return connection.ErrMissingID }
	items, err := s.repository.List(ctx)
	if err != nil { return fmt.Errorf("读取连接列表: %w", err) }
	found := false
	for _, item := range items { if item.ID == id { found = true; break } }
	if !found { return ErrNotFound }
	if err := s.repository.Delete(ctx, id); err != nil { return fmt.Errorf("删除连接: %w", err) }
	return nil
}

// Reorder 按给定 ID 顺序重排连接；未提供的连接保留原相对顺序。
func (s *Service) Reorder(ctx context.Context, ids []string) error {
	if err := ctx.Err(); err != nil { return err }
	return s.repository.Reorder(ctx, append([]string(nil), ids...))
}

// Test 在保存前执行连通性测试；不会修改或持久化连接配置。
func (s *Service) Test(ctx context.Context, item connection.Connection) error {
	item.Name = strings.TrimSpace(item.Name)
	item.Host = strings.TrimSpace(item.Host)
	if err := item.ValidateForTest(); err != nil { return err }
	if err := s.tester.Test(ctx, item); err != nil {
		return fmt.Errorf("连接测试失败（%s）: %w", item.Endpoint(), err)
	}
	return nil
}
