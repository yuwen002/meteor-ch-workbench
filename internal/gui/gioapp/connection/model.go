// Package connection 提供新 Gio 连接管理界面的状态模型，不依赖旧 GUI 服务。
package connection

import (
	"context"
	"errors"
	"sync"

	domain "github.com/yuwen002/meteor-ch-workbench/internal/domain/connection"
)

type Service interface {
	List(context.Context) ([]domain.Connection, error)
	Save(context.Context, domain.Connection) (domain.Connection, error)
	Delete(context.Context, string) error
	Test(context.Context, domain.Connection) error
}

type Snapshot struct {
	Items      []domain.Connection
	SelectedID string
	Loading    bool
	Saving     bool
	TestingID  string
	Error      string
}

// Model 管理连接列表及编辑 / 测试操作状态。耗时操作在锁外执行，允许 Gio 事件循环异步调用。
type Model struct {
	mu         sync.RWMutex
	service    Service
	items      []domain.Connection
	selectedID string
	loading    bool
	saving     bool
	testingID  string
	err        string
	loadSeq    uint64
}

func NewModel(service Service) (*Model, error) {
	if service == nil {
		return nil, errors.New("连接应用服务不能为空")
	}
	return &Model{service: service, items: make([]domain.Connection, 0)}, nil
}

func (m *Model) Snapshot() Snapshot {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return Snapshot{
		Items: append([]domain.Connection(nil), m.items...),
		SelectedID: m.selectedID,
		Loading: m.loading,
		Saving: m.saving,
		TestingID: m.testingID,
		Error: m.err,
	}
}

func (m *Model) Reload(ctx context.Context) error {
	m.mu.Lock()
	m.loadSeq++
	seq := m.loadSeq
	m.loading = true
	m.err = ""
	m.mu.Unlock()

	items, err := m.service.List(ctx)
	m.mu.Lock()
	defer m.mu.Unlock()
	if seq != m.loadSeq {
		return err
	}
	m.loading = false
	if err != nil {
		m.err = err.Error()
		return err
	}
	m.items = append([]domain.Connection(nil), items...)
	if !containsID(m.items, m.selectedID) {
		m.selectedID = ""
	}
	return nil
}

func (m *Model) Select(id string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	if id != "" && !containsID(m.items, id) {
		return false
	}
	m.selectedID = id
	return true
}

func (m *Model) Save(ctx context.Context, item domain.Connection) (domain.Connection, error) {
	m.mu.Lock()
	if m.saving {
		m.mu.Unlock()
		return domain.Connection{}, errors.New("连接正在保存")
	}
	m.saving = true
	m.err = ""
	m.mu.Unlock()

	saved, err := m.service.Save(ctx, item)
	m.mu.Lock()
	defer m.mu.Unlock()
	m.saving = false
	if err != nil {
		m.err = err.Error()
		return domain.Connection{}, err
	}
	replaced := false
	for i := range m.items {
		if m.items[i].ID == saved.ID {
			m.items[i] = saved
			replaced = true
			break
		}
	}
	if !replaced {
		m.items = append(m.items, saved)
	}
	m.selectedID = saved.ID
	return saved, nil
}

func (m *Model) Delete(ctx context.Context, id string) error {
	if err := m.service.Delete(ctx, id); err != nil {
		m.setError(err)
		return err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	filtered := make([]domain.Connection, 0, len(m.items))
	for _, item := range m.items {
		if item.ID != id {
			filtered = append(filtered, item)
		}
	}
	m.items = filtered
	if m.selectedID == id {
		m.selectedID = ""
	}
	m.err = ""
	return nil
}

func (m *Model) Test(ctx context.Context, item domain.Connection) error {
	m.mu.Lock()
	if m.testingID != "" {
		m.mu.Unlock()
		return errors.New("已有连接测试正在进行")
	}
	m.testingID = item.ID
	m.err = ""
	m.mu.Unlock()

	err := m.service.Test(ctx, item)
	m.mu.Lock()
	defer m.mu.Unlock()
	m.testingID = ""
	if err != nil {
		m.err = err.Error()
	}
	return err
}

func (m *Model) setError(err error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.err = err.Error()
}

func containsID(items []domain.Connection, id string) bool {
	if id == "" {
		return false
	}
	for _, item := range items {
		if item.ID == id {
			return true
		}
	}
	return false
}
