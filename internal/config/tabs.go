// 本文件负责编辑器标签页的持久化：把用户打开的查询标签（标题、SQL、活动标签）
// 保存到 tabs.json，使重启应用后能恢复到上次的工作现场。
// 读写模式与 HistoryManager 保持一致：内存缓存 + 读写锁 + 全量写回。
package config

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sync"
)

// TabMaxCount 是允许持久化的标签页数量上限，防止异常情况下文件无限膨胀。
const TabMaxCount = 50

// TabState 是一个查询标签页的持久化快照。
// 注意：只保存 SQL 文本与标题，查询结果不保存（结果随连接状态变化，无复用价值）。
type TabState struct {
	ID    string `json:"id"`    // 标签唯一标识，用于恢复时定位活动标签
	Title string `json:"title"` // 标签显示名称
	SQL   string `json:"sql"`   // 标签内的 SQL 文本
}

// TabsFile 是 tabs.json 的完整结构。
type TabsFile struct {
	Tabs        []TabState `json:"tabs"`          // 全部标签
	ActiveTabID string     `json:"active_tab_id"` // 上次活动的标签 ID
}

// TabsManager 负责 tabs.json 的读写与内存缓存。
type TabsManager struct {
	mu     sync.RWMutex
	path   string
	data   TabsFile
	loaded bool
}

// NewTabsManager 创建标签持久化管理器，复用配置目录。
func NewTabsManager(configDir string) *TabsManager {
	return &TabsManager{
		path: filepath.Join(configDir, "tabs.json"),
		data: TabsFile{Tabs: []TabState{}},
	}
}

// load 从磁盘读取标签快照（仅首次调用时真正读盘）。
func (t *TabsManager) load() error {
	if t.loaded {
		return nil
	}
	data, err := os.ReadFile(t.path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			t.loaded = true
			return nil
		}
		return err
	}
	if len(data) > 0 {
		var tf TabsFile
		if err := json.Unmarshal(data, &tf); err != nil {
			return err
		}
		if tf.Tabs != nil {
			t.data = tf
		}
	}
	t.loaded = true
	return nil
}

// Load 返回上次保存的标签快照。文件不存在时返回空结构，调用方据此创建默认标签。
func (t *TabsManager) Load() (TabsFile, error) {
	t.mu.Lock()
	defer t.mu.Unlock()
	if err := t.load(); err != nil {
		return TabsFile{Tabs: []TabState{}}, err
	}
	out := TabsFile{
		Tabs:        make([]TabState, len(t.data.Tabs)),
		ActiveTabID: t.data.ActiveTabID,
	}
	copy(out.Tabs, t.data.Tabs)
	return out, nil
}

// Save 覆盖保存全部标签快照并写回磁盘。
// 超过 TabMaxCount 的部分会被截断；活动标签 ID 不在列表中时退回第一个标签。
func (t *TabsManager) Save(snapshot TabsFile) error {
	t.mu.Lock()
	defer t.mu.Unlock()
	if snapshot.Tabs == nil {
		snapshot.Tabs = []TabState{}
	}
	if len(snapshot.Tabs) > TabMaxCount {
		snapshot.Tabs = snapshot.Tabs[:TabMaxCount]
	}
	activeValid := false
	for _, tab := range snapshot.Tabs {
		if tab.ID == snapshot.ActiveTabID {
			activeValid = true
			break
		}
	}
	if !activeValid {
		if len(snapshot.Tabs) > 0 {
			snapshot.ActiveTabID = snapshot.Tabs[0].ID
		} else {
			snapshot.ActiveTabID = ""
		}
	}
	t.data = snapshot
	t.loaded = true
	return t.flush()
}

// flush 把内存中的标签快照写回磁盘。
func (t *TabsManager) flush() error {
	data, err := json.MarshalIndent(t.data, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(t.path, data, 0o644)
}
