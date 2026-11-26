package settings

import (
	"context"
	"errors"
	"sync"

	domain "github.com/yuwen002/meteor-ch-workbench/internal/domain/settings"
)

// Service 提供线程安全的偏好读取和更新。
type Service struct {
	mu sync.Mutex
	repository Repository
	cached domain.Preferences
	loaded bool
}

func NewService(repository Repository) (*Service, error) {
	if repository == nil { return nil, errors.New("偏好设置仓库不能为空") }
	return &Service{repository: repository, cached: domain.Default()}, nil
}

func (s *Service) Load(ctx context.Context) (domain.Preferences, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.loaded { return s.cached, nil }
	value, err := s.repository.Load(ctx)
	if err != nil { return domain.Default(), err }
	s.cached = domain.Normalize(value)
	s.loaded = true
	return s.cached, nil
}

func (s *Service) Save(ctx context.Context, value domain.Preferences) (domain.Preferences, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	value = domain.Normalize(value)
	if err := s.repository.Save(ctx, value); err != nil { return domain.Preferences{}, err }
	s.cached = value
	s.loaded = true
	return value, nil
}
