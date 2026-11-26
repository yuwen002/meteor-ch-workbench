package app

import (
	"strings"

	"github.com/yuwen002/meteor-ch-workbench/internal/domain/savedsql"
)

// filterSavedSQL 按收藏名称或 SQL 内容进行不区分大小写的子串筛选。
func filterSavedSQL(items []savedsql.Entry, keyword string) []savedsql.Entry {
	keyword = strings.ToLower(strings.TrimSpace(keyword))
	if keyword == "" {
		return append([]savedsql.Entry(nil), items...)
	}
	filtered := make([]savedsql.Entry, 0, len(items))
	for _, item := range items {
		if strings.Contains(strings.ToLower(item.Name), keyword) ||
			strings.Contains(strings.ToLower(item.SQL), keyword) {
			filtered = append(filtered, item)
		}
	}
	return filtered
}
