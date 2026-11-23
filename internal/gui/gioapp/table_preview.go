package gioapp

import (
	"fmt"
	"strings"
)

// buildTablePreviewSQL 构造表数据预览 SQL，并对库名、表名中的反引号做转义。
func buildTablePreviewSQL(database, table string, limit int) string {
	if limit < 1 {
		limit = 100
	}
	return fmt.Sprintf("SELECT * FROM %s.%s LIMIT %d;",
		quoteCHIdentifier(database), quoteCHIdentifier(table), limit)
}

func quoteCHIdentifier(identifier string) string {
	return "`" + strings.ReplaceAll(identifier, "`", "``") + "`"
}
