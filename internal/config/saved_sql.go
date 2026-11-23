// 本文件负责「已保存 SQL」的持久化：把用户手动收藏的常用语句记录到 saved_sql.json。
// 与查询历史（history.go）语义不同：历史是自动流水、有上限、可被清空；
// 收藏是用户资产，无条数上限，且不参与「清空历史」的删除。
// 读写模式与 config.Manager 保持一致：内存缓存 + 读写锁 + 全量写回。
package config

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// SavedSQL 是一条手动收藏的 SQL 记录。
type SavedSQL struct {
	ID           string    `json:"id"`            // 记录唯一标识
	Name         string    `json:"name"`          // 用户命名的标题，便于检索
	SQL          string    `json:"sql"`           // 收藏的 SQL 文本
	ConnectionID string    `json:"connection_id"` // 收藏时所在的连接 ID，可为空
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

// SavedSQLManager 负责 saved_sql.json 的读写与内存缓存。
type SavedSQLManager struct {
	mu      sync.RWMutex // 保护 entries 的读写锁
	path    string       // 收藏文件完整路径
	entries []SavedSQL
	loaded  bool // 是否已从磁盘加载过
}

// NewSavedSQLManager 创建收藏管理器，复用配置目录。
func NewSavedSQLManager(configDir string) *SavedSQLManager {
	return &SavedSQLManager{
		path:    filepath.Join(configDir, "saved_sql.json"),
		entries: []SavedSQL{},
	}
}

// load 从磁盘读取收藏记录（仅在首次调用时真正读盘）。
func (s *SavedSQLManager) load() error {
	if s.loaded {
		return nil
	}
	data, err := os.ReadFile(s.path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			s.loaded = true
			return nil
		}
		return err
	}
	if len(data) > 0 {
		var entries []SavedSQL
		if err := json.Unmarshal(data, &entries); err != nil {
			return err
		}
		if entries != nil {
			s.entries = entries
		}
	}
	s.loaded = true
	return nil
}

// List 返回全部收藏记录，按更新时间倒序（最近修改的在前）。
func (s *SavedSQLManager) List() ([]SavedSQL, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.load(); err != nil {
		return nil, err
	}
	out := make([]SavedSQL, len(s.entries))
	copy(out, s.entries)
	return out, nil
}

// Add 追加一条收藏记录并写回磁盘。
// 空 SQL 会被拒绝；同一 SQL 文本（忽略首尾空白）已存在时返回 ErrSavedSQLDuplicate。
func (s *SavedSQLManager) Add(item SavedSQL) (SavedSQL, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.load(); err != nil {
		return SavedSQL{}, err
	}
	sql := strings.TrimSpace(item.SQL)
	if sql == "" {
		return SavedSQL{}, errors.New("saved sql is empty")
	}
	for _, e := range s.entries {
		if strings.TrimSpace(e.SQL) == sql {
			return SavedSQL{}, ErrSavedSQLDuplicate
		}
	}
	now := time.Now()
	if item.ID == "" {
		item.ID = newSavedSQLID()
	}
	item.SQL = sql
	item.Name = strings.TrimSpace(item.Name)
	if item.Name == "" {
		item.Name = firstLine(sql)
	}
	item.CreatedAt = now
	item.UpdatedAt = now
	s.entries = append([]SavedSQL{item}, s.entries...)
	return item, s.flush()
}

// Update 按 ID 更新标题，可选同时替换 SQL 文本；ID 不存在时返回 ErrSavedSQLNotFound。
func (s *SavedSQLManager) Update(id, name, sql string) (SavedSQL, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.load(); err != nil {
		return SavedSQL{}, err
	}
	idx := -1
	for i, e := range s.entries {
		if e.ID == id {
			idx = i
			break
		}
	}
	if idx < 0 {
		return SavedSQL{}, ErrSavedSQLNotFound
	}
	trimmed := strings.TrimSpace(sql)
	if trimmed == "" {
		return SavedSQL{}, errors.New("saved sql is empty")
	}
	for _, existing := range s.entries {
		if existing.ID != id && strings.TrimSpace(existing.SQL) == trimmed {
			return SavedSQL{}, ErrSavedSQLDuplicate
		}
	}
	if name = strings.TrimSpace(name); name == "" {
		name = firstLine(trimmed)
	}
	updated := s.entries[idx]
	updated.Name = name
	updated.SQL = trimmed
	updated.UpdatedAt = time.Now()
	s.entries[idx] = updated
	return updated, s.flush()
}

// Delete 按 ID 删除一条收藏记录；ID 不存在时返回 ErrSavedSQLNotFound。
func (s *SavedSQLManager) Delete(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.load(); err != nil {
		return err
	}
	filtered := s.entries[:0]
	found := false
	for _, e := range s.entries {
		if e.ID != id {
			filtered = append(filtered, e)
		} else {
			found = true
		}
	}
	if !found {
		return ErrSavedSQLNotFound
	}
	s.entries = filtered
	return s.flush()
}

// flush 把内存中的收藏记录序列化写回磁盘。
func (s *SavedSQLManager) flush() error {
	data, err := json.MarshalIndent(s.entries, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(s.path, data, 0o644)
}

// ErrSavedSQLDuplicate 表示同一 SQL 文本已被收藏过。
var ErrSavedSQLDuplicate = errors.New("saved sql already exists")

// ErrSavedSQLNotFound 表示目标收藏记录不存在。
var ErrSavedSQLNotFound = errors.New("saved sql not found")

// firstLine 取第一条非空行作为默认标题，超长时截断，避免列表展示过宽。
func firstLine(sql string) string {
	for _, line := range strings.Split(sql, "\n") {
		if t := strings.TrimSpace(line); t != "" {
			if r := []rune(t); len(r) > 80 {
				return string(r[:80])
			}
			return t
		}
	}
	return ""
}

// newSavedSQLID 生成一个 16 位十六进制 ID，与连接 ID 生成方式保持一致。
func newSavedSQLID() string {
	var c Connection
	c.GenID()
	return c.ID
}
