// Package connection 提供新 Gio 连接管理界面的状态模型，不依赖旧 GUI 服务。
package connection

import (
	"context"
	"errors"
	"sync"

	domain "github.com/yuwen002/meteor-ch-workbench/internal/domain/connection"
)

type FolderService interface {
	List(context.Context) ([]domain.Folder, error)
	Save(context.Context, domain.Folder) (domain.Folder, error)
	Delete(context.Context, string) error
	Reorder(context.Context, []string) error
}

type Service interface {
	List(context.Context) ([]domain.Connection, error)
	Save(context.Context, domain.Connection) (domain.Connection, error)
	Delete(context.Context, string) error
	Reorder(context.Context, []string) error
	Test(context.Context, domain.Connection) error
}

type Snapshot struct {
	Items      []domain.Connection
	Folders    []domain.Folder
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
	folderService FolderService
	folders []domain.Folder
	items      []domain.Connection
	selectedID string
	loading    bool
	saving     bool
	testingID  string
	err        string
	loadSeq    uint64
	testSeq    uint64
}

func NewModel(service Service, folders FolderService) (*Model, error) {
	if service == nil {
		return nil, errors.New("连接应用服务不能为空")
	}
	if folders == nil { return nil, errors.New("分组应用服务不能为空") }
	return &Model{service: service, folderService: folders, items: make([]domain.Connection, 0), folders: make([]domain.Folder, 0)}, nil
}

func (m *Model) Snapshot() Snapshot {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return Snapshot{
		Items: append([]domain.Connection(nil), m.items...),
		Folders: append([]domain.Folder(nil), m.folders...),
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
	var folders []domain.Folder
	if err == nil { folders, err = m.folderService.List(ctx) }
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
	m.folders = append([]domain.Folder(nil), folders...)
	if !containsID(m.items, m.selectedID) {
		m.selectedID = ""
	}
	return nil
}

func (m *Model) SaveFolder(ctx context.Context, item domain.Folder) (domain.Folder, error) {
	folder, err := m.folderService.Save(ctx, item)
	if err != nil { m.setError(err); return domain.Folder{}, err }
	m.mu.Lock()
	defer m.mu.Unlock()
	for i := range m.folders { if m.folders[i].ID == folder.ID { m.folders[i] = folder; return folder, nil } }
	m.folders = append(m.folders, folder)
	m.err = ""
	return folder, nil
}

func (m *Model) DeleteFolder(ctx context.Context, id string) error {
	if err := m.folderService.Delete(ctx, id); err != nil { m.setError(err); return err }
	m.mu.Lock()
	defer m.mu.Unlock()
	folders := make([]domain.Folder, 0, len(m.folders))
	for _, item := range m.folders { if item.ID != id { folders = append(folders, item) } }
	m.folders = folders
	for i := range m.items { if m.items[i].FolderID == id { m.items[i].FolderID = "" } }
	m.err = ""
	return nil
}

// ReorderFolders 按新的 ID 顺序持久化分组，并同步更新界面快照。
func (m *Model) ReorderFolders(ctx context.Context, ids []string) error {
	if err := m.folderService.Reorder(ctx, ids); err != nil { m.setError(err); return err }
	folders, err := m.folderService.List(ctx)
	if err != nil { m.setError(err); return err }
	m.mu.Lock()
	m.folders = append([]domain.Folder(nil), folders...)
	m.err = ""
	m.mu.Unlock()
	return nil
}

// ReorderConnections 按新的 ID 顺序持久化连接，并同步更新界面快照。
func (m *Model) ReorderConnections(ctx context.Context, ids []string) error {
	if err := m.service.Reorder(ctx, ids); err != nil { m.setError(err); return err }
	items, err := m.service.List(ctx)
	if err != nil { m.setError(err); return err }
	m.mu.Lock()
	m.items = append([]domain.Connection(nil), items...)
	m.err = ""
	m.mu.Unlock()
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
	m.testSeq++
	seq := m.testSeq
	m.testingID = item.ID
	m.err = ""
	m.mu.Unlock()

	err := m.service.Test(ctx, item)
	m.mu.Lock()
	defer m.mu.Unlock()
	if seq == m.testSeq {
		m.testingID = ""
		if err != nil {
			m.err = err.Error()
		}
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
