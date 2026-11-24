// Package workspace 提供与 Gio 控件无关的 SQL 标签页标识操作。
package workspace

// TabID 是标签页在整个工作区生命周期内稳定的身份标识。
type TabID uint64

// IndexOf 根据稳定 ID 查找标签页当前索引。
func IndexOf(ids []TabID, target TabID) (int, bool) {
	for i, id := range ids {
		if id == target {
			return i, true
		}
	}
	return 0, false
}

// RemoveAt 删除指定位置的标签 ID；索引无效时原样返回并标记失败。
func RemoveAt(ids []TabID, index int) ([]TabID, bool) {
	if index < 0 || index >= len(ids) {
		return ids, false
	}
	result := make([]TabID, 0, len(ids)-1)
	result = append(result, ids[:index]...)
	result = append(result, ids[index+1:]...)
	return result, true
}
