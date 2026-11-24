package app

import (
	"fmt"
	"unicode/utf8"

	queryapp "github.com/yuwen002/meteor-ch-workbench/internal/application/query"
)

// resultColumnFlexWeights 根据列名、类型和前 40 行样本估算列宽。
// 使用有界权重，避免超长 JSON / String 列挤压其他字段。
func resultColumnFlexWeights(columns []queryapp.Column, rows [][]any, sampleRows int) []float32 {
	weights := make([]float32, len(columns))
	for i, column := range columns {
		width := utf8.RuneCountInString(column.Name)
		typeWidth := utf8.RuneCountInString(column.DatabaseType)
		if typeWidth > width {
			width = typeWidth
		}
		if sampleRows <= 0 || sampleRows > len(rows) {
			sampleRows = len(rows)
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
