package gioapp

import (
	"fmt"
	"strings"
)

// buildTablePreviewSQL 构造表数据预览 SQL，并对库名、表名中的反引号做转义。
func buildTablePreviewSQL(database, table string, limit int) string {
	return buildTablePreviewPageSQL(database, table, limit, 0)
}

// buildTablePreviewPageSQL 构造指定页的表预览 SQL。页码从 0 开始。
func buildTablePreviewPageSQL(database, table string, limit, page int) string {
	if limit < 1 { limit = 100 }
	if page < 0 { page = 0 }
	offset := page * limit
	return fmt.Sprintf("SELECT * FROM %s.%s LIMIT %d OFFSET %d;",
		quoteCHIdentifier(database), quoteCHIdentifier(table), limit, offset)
}

func quoteCHIdentifier(identifier string) string {
	return "`" + strings.ReplaceAll(identifier, "`", "``") + "`"
}
