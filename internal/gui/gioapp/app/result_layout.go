package app

import (
	"fmt"
	"strings"
	"unicode/utf8"

	queryapp "github.com/yuwen002/meteor-ch-workbench/internal/application/query"
)

// resultColumnFlexWeights 根据列名、类型和前 40 行样本估算列宽。
// 使用有界权重，避免超长 JSON / String 列挤压其他字段。
func resultColumnFlexWeights(columns []queryapp.Column, rows [][]any, sampleRows int) []float32 {
	weights := make([]float32, len(columns))
	if sampleRows <= 0 || sampleRows > len(rows) {
		sampleRows = len(rows)
	}
	for i, column := range columns {
		width := utf8.RuneCountInString(column.Name)
		typeWidth := utf8.RuneCountInString(column.DatabaseType)
		if typeWidth > width {
			width = typeWidth
		}
		for rowIndex := 0; rowIndex < sampleRows; rowIndex++ {
			if i >= len(rows[rowIndex]) {
				continue
			}
			cellWidth := utf8.RuneCountInString(fmt.Sprint(rows[rowIndex][i]))
			if cellWidth > width {
				width = cellWidth
			}
		}
		if width < 10 {
			width = 10
		}
		if width > 34 {
			width = 34
		}
		weights[i] = float32(width)
	}
	return weights
}

const maxResultCellRunes = 80

// resultCellRawValue 返回单元格的完整值，不进行截断或换行替换。
func resultCellRawValue(value any) string {
	if value == nil { return "NULL" }
	switch typed := value.(type) {
	case string:
		return typed
	case []byte:
		return string(typed)
	default:
		return fmt.Sprint(value)
	}
}

// formatResultCell 将数据库值转换为紧凑、可读的单元格文本。
func formatResultCell(value any) string {
	if value == nil {
		return "NULL"
	}
	text := fmt.Sprint(value)
	text = strings.NewReplacer("\\r\\n", " ↵ ", "\\n", " ↵ ", "\\r", " ↵ ", "\\t", " ⇥ ", "\r\n", " ↵ ", "\n", " ↵ ", "\r", " ↵ ", "\t", " ⇥ ").Replace(text)
	if utf8.RuneCountInString(text) <= maxResultCellRunes {
		return text
	}
	runes := []rune(text)
	return string(runes[:maxResultCellRunes-1]) + "…"
}
