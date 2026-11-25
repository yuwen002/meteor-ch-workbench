package workspace

// EditorState 管理单个 SQL 编辑器的文本与保存基线，不依赖 Gio 控件。
type EditorState struct {
	text      string
	savedText string
}

// Text 返回当前编辑器文本。
func (e EditorState) Text() string { return e.text }

// SetText 更新文本；Dirty 由当前文本与保存基线比较得出。
func (e *EditorState) SetText(text string) {
	if e == nil { return }
	e.text = text
}

// Dirty 表示当前文本是否偏离最近一次保存基线。
func (e EditorState) Dirty() bool { return e.text != e.savedText }

// MarkSaved 将当前文本设为保存基线。
func (e *EditorState) MarkSaved() {
	if e == nil { return }
	e.savedText = e.text
}

// Reset 清空文本并同步保存基线。
func (e *EditorState) Reset() {
	if e == nil { return }
	e.text, e.savedText = "", ""
}
