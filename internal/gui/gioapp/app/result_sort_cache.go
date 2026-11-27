package app

// resultSortKeyCache 管理当前结果排序列的预计算键，避免 workspace 分散维护缓存有效性。
type resultSortKeyCache struct {
	keys   []resultSortKey
	column int
	valid  bool
}

// Reset 释放当前排序列的缓存；列定义或结果快照整体替换时调用。
func (c *resultSortKeyCache) Reset() {
	if c == nil {
		return
	}
	c.keys = nil
	c.column = -1
	c.valid = false
}

// Prepare 返回指定排序列的完整键缓存。
// 同一列仅为新追加的行构建键；切换列或结果行数缩小时重建，避免复用旧列 / 旧行键。
func (c *resultSortKeyCache) Prepare(rows [][]any, column int) []resultSortKey {
	if c == nil || column < 0 {
		if c != nil {
			c.Reset()
		}
		return nil
	}
	if !c.valid || c.column != column {
		c.keys = nil
		c.column = column
		c.valid = true
	}
	c.keys = extendResultSortKeys(rows, column, c.keys)
	return c.keys
}
