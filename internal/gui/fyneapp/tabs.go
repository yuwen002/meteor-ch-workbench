package fyneapp

import (
	"fmt"
	"strings"

	"github.com/yuwen002/meteor-ch-workbench/internal/clickhouse"
	"github.com/yuwen002/meteor-ch-workbench/internal/config"
)

type QueryTab struct {
	ID        string
	Title     string
	SQL       string
	QueryID   string
	Executing bool
	Result    *clickhouse.QueryResult
}

var tabIDCounter int

func NewQueryTab(sql string) *QueryTab {
	tabIDCounter++
	id := fmt.Sprintf("tab-%d", tabIDCounter)
	title := "未命名"
	for _, line := range splitLines(sql) {
		line = trimSpace(line)
		if line != "" && !hasPrefix(line, "--") && !hasPrefix(line, "#") {
			if len(line) > 40 {
				title = line[:40] + "..."
			} else {
				title = line
			}
			break
		}
	}
	return &QueryTab{
		ID:    id,
		Title: title,
		SQL:   sql,
	}
}

func splitLines(s string) []string {
	var lines []string
	start := 0
	for i := 0; i < len(s); i++ {
		if s[i] == '\n' {
			lines = append(lines, s[start:i])
			start = i + 1
		}
	}
	if start < len(s) {
		lines = append(lines, s[start:])
	}
	return lines
}

func trimSpace(s string) string {
	start, end := 0, len(s)
	for start < end && (s[start] == ' ' || s[start] == '\t') {
		start++
	}
	for end > start && (s[end-1] == ' ' || s[end-1] == '\t' || s[end-1] == '\r') {
		end--
	}
	return s[start:end]
}

func hasPrefix(s, prefix string) bool {
	if len(s) < len(prefix) {
		return false
	}
	return s[:len(prefix)] == prefix
}

func tabsToConfig(tabs []*QueryTab, activeIdx int) config.TabsFile {
	var result config.TabsFile
	if activeIdx >= 0 && activeIdx < len(tabs) {
		result.ActiveTabID = tabs[activeIdx].ID
	}
	for _, t := range tabs {
		result.Tabs = append(result.Tabs, config.TabState{
			ID:    t.ID,
			Title: t.Title,
			SQL:   t.SQL,
		})
	}
	return result
}

func tabsFromConfig(tf config.TabsFile) ([]*QueryTab, int) {
	var tabs []*QueryTab
	activeIdx := 0
	for i, ts := range tf.Tabs {
		tabs = append(tabs, &QueryTab{
			ID:    ts.ID,
			Title: ts.Title,
			SQL:   ts.SQL,
		})
		if ts.ID == tf.ActiveTabID {
			activeIdx = i
		}
	}
	if len(tabs) == 0 {
		tabs = append(tabs, NewQueryTab(""))
	}
	return tabs, activeIdx
}

func looksLikeSelect(sql string) bool {
	trimmed := strings.TrimSpace(sql)
	if trimmed == "" {
		return false
	}
	upper := strings.ToUpper(trimmed)
	return strings.HasPrefix(upper, "SELECT") ||
		strings.HasPrefix(upper, "WITH") ||
		strings.HasPrefix(upper, "SHOW") ||
		strings.HasPrefix(upper, "DESCRIBE") ||
		strings.HasPrefix(upper, "EXPLAIN")
}

func appendLimitOffset(sql string, limit, offset int) string {
	trimmed := strings.TrimRight(strings.TrimSpace(sql), "; \t\n\r")
	if trimmed == "" {
		return sql
	}
	upper := strings.ToUpper(trimmed)
	if strings.Contains(upper, "LIMIT") {
		return sql
	}
	if offset > 0 {
		return fmt.Sprintf("%s LIMIT %d OFFSET %d", trimmed, limit, offset)
	}
	return fmt.Sprintf("%s LIMIT %d", trimmed, limit)
}