package app

import (
	"strings"

	queryapp "github.com/yuwen002/meteor-ch-workbench/internal/application/query"
)

// classifySQLMode 按语句首关键字区分返回结果集的查询与无结果集的执行语句。
func classifySQLMode(sqlText string) queryapp.Mode {
	remaining := strings.TrimSpace(sqlText)
	for {
		switch {
		case strings.HasPrefix(remaining, "--"):
			if end := strings.IndexByte(remaining, '\n'); end >= 0 {
				remaining = strings.TrimSpace(remaining[end+1:])
				continue
			}
			return queryapp.ModeExec
		case strings.HasPrefix(remaining, "/*"):
			if end := strings.Index(remaining[2:], "*/"); end >= 0 {
				remaining = strings.TrimSpace(remaining[end+4:])
				continue
			}
			return queryapp.ModeExec
		}
		break
	}
	fields := strings.Fields(remaining)
	if len(fields) == 0 {
		return queryapp.ModeRows
	}
	switch strings.ToUpper(strings.Trim(fields[0], ";")) {
	case "SELECT", "WITH", "SHOW", "DESCRIBE", "DESC", "EXPLAIN", "EXISTS":
		return queryapp.ModeRows
	default:
		return queryapp.ModeExec
	}
}
