// 本文件负责查询历史的持久化：把每次成功执行的 SQL 记录到 history.json，
// 便于用户在结果面板中一键回填最近用过的语句。
// 读写模式与 config.Manager 保持一致：内存缓存 + 读写锁 + 全量写回。
package config

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// HistoryMaxEntries 是查询历史保留的最大条数，超出后丢弃最旧的记录。
const HistoryMaxEntries = 200

// HistoryEntry 是一条查询历史记录。
type HistoryEntry struct {
	ID           string    `json:"id"`            // 记录唯一标识，用于前端去重与删除
	SQL          string    `json:"sql"`           // 执行的 SQL 文本
	ConnectionID string    `json:"connection_id"` // 执行所用的连接 ID
	Timestamp    time.Time `json:"timestamp"`     // 执行完成时间
	DurationMs   float64   `json:"duration_ms"`   // 执行耗时（毫秒）
	Success      bool      `json:"success"`       // 是否执行成功
}

// HistoryManager 负责 history.json 的读写与内存缓存。
type HistoryManager struct {
	mu      sync.RWMutex // 保护 entries 的读写锁
	path    string       // 历史文件完整路径
	entries []HistoryEntry
	loaded  bool // 是否已从磁盘加载过
}

// NewHistoryManager 创建查询历史管理器，复用配置目录。
func NewHistoryManager(configDir string) *HistoryManager {
	return &HistoryManager{
		path:    filepath.Join(configDir, "history.json"),
		entries: []HistoryEntry{},
	}
}

// load 从磁盘读取历史记录（仅在首次调用时真正读盘）。
func (h *HistoryManager) load() error {
	if h.loaded {
		return nil
	}
	data, err := os.ReadFile(h.path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			h.loaded = true
			return nil
		}
		return err
	}
	if len(data) > 0 {
		var entries []HistoryEntry
		if err := json.Unmarshal(data, &entries); err != nil {
			return err
		}
		if entries != nil {
			h.entries = entries
		}
	}
	h.loaded = true
	return nil
}

// List 返回全部历史记录，按时间倒序（最新的在前）。
func (h *HistoryManager) List() ([]HistoryEntry, error) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if err := h.load(); err != nil {
		return nil, err
	}
	out := make([]HistoryEntry, len(h.entries))
	copy(out, h.entries)
	return out, nil
}

// Add 追加一条历史记录，超过上限时丢弃最旧的条目并写回磁盘。
func (h *HistoryManager) Add(entry HistoryEntry) error {
	h.mu.Lock()
	defer h.mu.Unlock()
	if err := h.load(); err != nil {
		return err
	}
	if entry.ID == "" {
		entry.ID = newHistoryID()
	}
	if entry.Timestamp.IsZero() {
		entry.Timestamp = time.Now()
	}
	h.entries = append([]HistoryEntry{entry}, h.entries...)
	if len(h.entries) > HistoryMaxEntries {
		h.entries = h.entries[:HistoryMaxEntries]
	}
	return h.flush()
}

// Clear 清空全部历史记录并写回磁盘。
func (h *HistoryManager) Clear() error {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.entries = []HistoryEntry{}
	h.loaded = true
	return h.flush()
}

// Delete 按 ID 删除一条历史记录。
func (h *HistoryManager) Delete(id string) error {
	h.mu.Lock()
	defer h.mu.Unlock()
	if err := h.load(); err != nil {
		return err
	}
	filtered := h.entries[:0]
	for _, e := range h.entries {
		if e.ID != id {
			filtered = append(filtered, e)
		}
	}
	h.entries = filtered
	return h.flush()
}

// flush 把内存中的历史记录序列化写回磁盘。
func (h *HistoryManager) flush() error {
	data, err := json.MarshalIndent(h.entries, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(h.path, data, 0o644)
}

// newHistoryID 生成一个 16 位十六进制 ID，与连接 ID 生成方式保持一致。
func newHistoryID() string {
	var c Connection
	c.GenID()
	return c.ID
}
