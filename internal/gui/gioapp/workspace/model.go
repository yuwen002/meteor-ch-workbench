package workspace

import (
	"fmt"
	"strings"
)

// Tab 保存标签页身份与标题；编辑器内容由独立 EditorState 管理。
type Tab struct {
	ID     TabID
	Title  string
	Editor EditorState
}

// Tabs 管理标签集合与当前活动标签。所有操作均以稳定 TabID 为身份。
type Tabs struct {
	items  []Tab
	active TabID
	nextID TabID
	nextNo uint64
}

// NewTabs 创建包含一个空查询页的标签集合。
func NewTabs() *Tabs {
	first := Tab{ID: 1, Title: "查询 1"}
	return &Tabs{items: []Tab{first}, active: first.ID, nextID: first.ID, nextNo: 1}
}

// Items 返回标签快照，调用方修改结果不会影响内部状态。
func (t *Tabs) Items() []Tab {
	if t == nil {
		return nil
	}
	return append([]Tab(nil), t.items...)
}

// ActiveID 返回当前活动标签的稳定 ID。
func (t *Tabs) ActiveID() TabID {
	if t == nil {
		return 0
	}
	return t.active
}

// ActiveIndex 返回当前活动标签的 UI 索引。
func (t *Tabs) ActiveIndex() (int, bool) {
	if t == nil {
		return 0, false
	}
	return IndexOf(t.ids(), t.active)
}

// Get 按稳定 ID 读取标签。
func (t *Tabs) Get(id TabID) (Tab, bool) {
	if t == nil {
		return Tab{}, false
	}
	index, ok := IndexOf(t.ids(), id)
	if !ok {
		return Tab{}, false
	}
	return t.items[index], true
}

// Create 新建并激活一个标签。
func (t *Tabs) Create() Tab {
	if t == nil {
		return Tab{}
	}
	t.nextID++
	t.nextNo++
	tab := Tab{ID: t.nextID, Title: fmt.Sprintf("查询 %d", t.nextNo)}
	t.items = append(t.items, tab)
	t.active = tab.ID
	return tab
}

// Activate 切换活动标签；不存在的 ID 返回 false。
func (t *Tabs) Activate(id TabID) bool {
	if t == nil {
		return false
	}
	if _, ok := IndexOf(t.ids(), id); !ok {
		return false
	}
	t.active = id
	return true
}

// Close 关闭指定标签。至少保留一个标签；关闭活动标签时优先选中其右侧标签。
func (t *Tabs) Close(id TabID) (Tab, bool) {
	if t == nil || len(t.items) <= 1 {
		return Tab{}, false
	}
	index, ok := IndexOf(t.ids(), id)
	if !ok {
		return Tab{}, false
	}
	removed := t.items[index]
	t.items = append(t.items[:index], t.items[index+1:]...)
	if t.active == id {
		if index >= len(t.items) {
			index = len(t.items) - 1
		}
		t.active = t.items[index].ID
	}
	return removed, true
}

// UpdateSQL 更新标签对应的编辑器文本。
func (t *Tabs) UpdateSQL(id TabID, sql string) bool {
	index, ok := t.index(id)
	if !ok {
		return false
	}
	t.items[index].Editor.SetText(sql)
	return true
}

// MarkSaved 将标签当前编辑器内容标记为已保存。
func (t *Tabs) MarkSaved(id TabID) bool {
	index, ok := t.index(id)
	if !ok {
		return false
	}
	t.items[index].Editor.MarkSaved()
	return true
}

// Rename 修改标签标题；空白标题会被拒绝。
func (t *Tabs) Rename(id TabID, title string) bool {
	index, ok := t.index(id)
	if !ok || strings.TrimSpace(title) == "" {
		return false
	}
	t.items[index].Title = title
	return true
}

// Reorder 将指定标签移动到目标索引。
func (t *Tabs) Reorder(id TabID, to int) bool {
	if t == nil || to < 0 || to >= len(t.items) {
		return false
	}
	from, ok := t.index(id)
	if !ok {
		return false
	}
	if from == to {
		return true
	}
	tab := t.items[from]
	if from < to {
		copy(t.items[from:to], t.items[from+1:to+1])
	} else {
		copy(t.items[to+1:from+1], t.items[to:from])
	}
	t.items[to] = tab
	return true
}

func (t *Tabs) index(id TabID) (int, bool) {
	if t == nil {
		return 0, false
	}
	return IndexOf(t.ids(), id)
}

func (t *Tabs) ids() []TabID {
	ids := make([]TabID, len(t.items))
	for i := range t.items {
		ids[i] = t.items[i].ID
	}
	return ids
}
