// 本文件负责用户偏好设置的持久化：目前只有「查询结果行数上限」一项。
// 读写模式与 HistoryManager / TabsManager 保持一致：内存缓存 + 读写锁 + 全量写回。
package config

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sync"
)

// 行数上限的可选范围。0 表示不限制（慎用，超大结果可能拖垮前端）。
const (
	RowLimitUnlimited = 0
	RowLimitMin       = 100
	RowLimitMax       = 1000000
	// DefaultRowLimit 是未配置时生效的行数上限，与 clickhouse.DefaultQueryRowLimit 保持一致。
	DefaultRowLimit = 50000
)

// SettingsFile 是 settings.json 的完整结构。
type SettingsFile struct {
	QueryRowLimit int `json:"query_row_limit"` // 查询结果行数上限，0 表示不限制
}

// SettingsManager 负责 settings.json 的读写与内存缓存。
type SettingsManager struct {
	mu     sync.RWMutex
	path   string
	data   SettingsFile
	loaded bool
}

// NewSettingsManager 创建设置持久化管理器，复用配置目录。
func NewSettingsManager(configDir string) *SettingsManager {
	return &SettingsManager{
		path: filepath.Join(configDir, "settings.json"),
		data: SettingsFile{QueryRowLimit: DefaultRowLimit},
	}
}

// load 从磁盘读取设置（仅首次调用时真正读盘）。文件不存在时保持默认值。
func (s *SettingsManager) load() error {
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
		var sf SettingsFile
		if err := json.Unmarshal(data, &sf); err != nil {
			return err
		}
		s.data = SettingsFile{QueryRowLimit: NormalizeRowLimit(sf.QueryRowLimit)}
	}
	s.loaded = true
	return nil
}

// Load 返回当前设置。文件不存在或非法时返回默认设置。
func (s *SettingsManager) Load() (SettingsFile, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.load(); err != nil {
		return SettingsFile{QueryRowLimit: DefaultRowLimit}, err
	}
	return s.data, nil
}

// RowLimit 返回当前生效的查询行数上限，供查询路径直接读取。
// 读盘失败时退回默认值，保证查询不会因为设置文件损坏而失败。
func (s *SettingsManager) RowLimit() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.load(); err != nil {
		return DefaultRowLimit
	}
	return s.data.QueryRowLimit
}

// Save 校验并覆盖保存设置，随后全量写回磁盘。
func (s *SettingsManager) Save(settings SettingsFile) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.data = SettingsFile{QueryRowLimit: NormalizeRowLimit(settings.QueryRowLimit)}
	s.loaded = true
	return s.flush()
}

// flush 把内存中的设置写回磁盘。
func (s *SettingsManager) flush() error {
	data, err := json.MarshalIndent(s.data, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(s.path, data, 0o644)
}

// NormalizeRowLimit 把任意输入的行数上限收敛到合法区间：
// 负数与小于 RowLimitMin 的正数视为默认值，超过 RowLimitMax 的截断到上限。
// 显式传入 0 表示不限制，原样保留。
func NormalizeRowLimit(v int) int {
	if v == RowLimitUnlimited {
		return RowLimitUnlimited
	}
	if v < RowLimitMin {
		return DefaultRowLimit
	}
	if v > RowLimitMax {
		return RowLimitMax
	}
	return v
}
