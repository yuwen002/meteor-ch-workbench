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

// resultCellReplacer 在包级复用，避免每次格式化单元格都重建替换表。
var resultCellReplacer = strings.NewReplacer(
	"\\r\\n", " ↵ ", "\\n", " ↵ ", "\\r", " ↵ ", "\\t", " ⇥ ",
	"\r\n", " ↵ ", "\n", " ↵ ", "\r", " ↵ ", "\t", " ⇥ ",
)

// resultCellRawValue 返回单元格的完整值，不进行截断或换行替换。
func resultCellRawValue(value any) string {
	if value == nil {
		return "NULL"
	}
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
	text := resultCellRawValue(value)
	// 绝大多数单元格没有控制字符，先判断再替换，避免每次都分配替换结果。
	if strings.Contains(text, "\\") || strings.ContainsAny(text, "\r\n\t") {
		text = resultCellReplacer.Replace(text)
	}
	if utf8.RuneCountInString(text) <= maxResultCellRunes {
		return text
	}
	// 按 rune 边界定位截断点，避免 []rune(text) 的整串分配。
	count, cut := 0, len(text)
	for index, r := range text {
		count++
		if count == maxResultCellRunes-1 {
			cut = index + utf8.RuneLen(r)
			break
		}
	}
	return text[:cut] + "…"
}
