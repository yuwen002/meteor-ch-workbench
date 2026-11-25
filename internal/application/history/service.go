package history

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/yuwen002/meteor-ch-workbench/internal/domain/queryhistory"
)

const MaxEntries = 200

var ErrNotFound = errors.New("查询历史记录不存在")

// Service 提供查询历史的读取、追加、删除和清空用例。
type Service struct {
	repository Repository
	ids IDGenerator
}

func NewService(repository Repository, ids IDGenerator) (*Service, error) {
	if repository == nil { return nil, errors.New("查询历史仓库不能为空") }
	if ids == nil { return nil, errors.New("查询历史 ID 生成器不能为空") }
	return &Service{repository: repository, ids: ids}, nil
}

func (s *Service) List(ctx context.Context) ([]queryhistory.Entry, error) {
	return s.repository.List(ctx)
}

// Add 记录一次 SQL 执行；最新记录排在首位，最多保留 200 条。
func (s *Service) Add(ctx context.Context, entry queryhistory.Entry) (queryhistory.Entry, error) {
	entry.SQL = strings.TrimSpace(entry.SQL)
	if entry.SQL == "" { return queryhistory.Entry{}, errors.New("SQL 不能为空") }
	if entry.ConnectionID == "" { return queryhistory.Entry{}, errors.New("连接 ID 不能为空") }
	if entry.ID == "" {
		id, err := s.ids.NewID()
		if err != nil { return queryhistory.Entry{}, fmt.Errorf("生成查询历史 ID: %w", err) }
		if strings.TrimSpace(id) == "" { return queryhistory.Entry{}, errors.New("查询历史 ID 生成器返回空值") }
		entry.ID = id
	}
	if entry.Timestamp.IsZero() { entry.Timestamp = time.Now() }
	items, err := s.repository.List(ctx)
	if err != nil { return queryhistory.Entry{}, err }
	items = append([]queryhistory.Entry{entry}, items...)
	if len(items) > MaxEntries { items = items[:MaxEntries] }
	if err := s.repository.Save(ctx, items); err != nil { return queryhistory.Entry{}, fmt.Errorf("保存查询历史: %w", err) }
	return entry, nil
}

func (s *Service) Delete(ctx context.Context, id string) error {
	items, err := s.repository.List(ctx)
	if err != nil { return err }
	filtered := make([]queryhistory.Entry, 0, len(items))
	found := false
	for _, item := range items {
		if item.ID == id { found = true; continue }
		filtered = append(filtered, item)
	}
	if !found { return ErrNotFound }
	return s.repository.Save(ctx, filtered)
}

func (s *Service) Clear(ctx context.Context) error {
	return s.repository.Save(ctx, []queryhistory.Entry{})
}
