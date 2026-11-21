// 本文件负责把结果集渲染为真正的 Excel 工作簿（.xlsx）。
// xlsx 是 zip 容器，无法像 CSV / JSON 那样逐步写文本，因此这里先用 excelize
// 在内存中构建工作簿，再把它包装成实现 io.Writer 的单次写出，其余格式的
// 流式契约保持不变（ExportResultTo 的签名无需特殊处理）。
package clickhouse

import (
	"errors"
	"fmt"
	"io"
	"math"
	"strings"
	"unicode/utf8"

	excelize "github.com/xuri/excelize/v2"
)

// xlsxSheetName 是导出工作簿中唯一工作表的名字。
const xlsxSheetName = "Result"

// xlsxMaxCellRunes 是单个单元格允许的最大字符数。
// xlsx 规范限制单元格文本不超过 32767 个字符，超长内容截断并追加省略标记，
// 避免导出一张含超长文本的表时 Excel 打开即报文件损坏。
const xlsxMaxCellRunes = 32767

// renderResultXLSXTo 把结果集渲染为 Excel 工作簿并写入 w。
// target 不参与渲染（与 CSV / JSON 一样忽略），保证同一输入必然同一输出。
func renderResultXLSXTo(w io.Writer, columns []string, rows [][]interface{}, _ string) error {
	data, err := buildXLSX(columns, rows)
	if err != nil {
		return err
	}
	if _, err := w.Write(data); err != nil {
		return err
	}
	return nil
}

// buildXLSX 构建工作簿字节流：首行表头（加粗 + 冻结）、数据按原始类型写入
// （数值仍是数值，便于 Excel 直接求和），列宽按内容自适应。
func buildXLSX(columns []string, rows [][]interface{}) ([]byte, error) {
	f := excelize.NewFile()
	defer f.Close()

	sheet, err := f.NewSheet(xlsxSheetName)
	if err != nil {
		return nil, err
	}
	f.SetActiveSheet(sheet)
	if err := f.DeleteSheet("Sheet1"); err != nil {
		return nil, err
	}

	headerStyle, err := f.NewStyle(&excelize.Style{
		Font: &excelize.Font{Bold: true},
	})
	if err != nil {
		return nil, err
	}

	for i, col := range columns {
		axis, err := columnAxis(i)
		if err != nil {
			return nil, err
		}
		if err := f.SetCellStr(xlsxSheetName, axis+"1", col); err != nil {
			return nil, err
		}
		if err := f.SetCellStyle(xlsxSheetName, axis+"1", axis+"1", headerStyle); err != nil {
			return nil, err
		}
	}
	if len(columns) > 0 {
		lastAxis, err := columnAxis(len(columns) - 1)
		if err != nil {
			return nil, err
		}
		if err := f.AutoFilter(xlsxSheetName, "A1:"+lastAxis+"1", nil); err != nil {
			return nil, err
		}
	}
	if err := f.SetPanes(xlsxSheetName, &excelize.Panes{
		Freeze:      true,
		YSplit:      1,
		TopLeftCell: "A2",
		ActivePane:  "bottomLeft",
	}); err != nil {
		return nil, err
	}

	for r, row := range rows {
		for c := range columns {
			axis, err := columnAxis(c)
			if err != nil {
				return nil, err
			}
			cell := fmt.Sprintf("%s%d", axis, r+2)
			var v interface{}
			if c < len(row) {
				v = row[c]
			}
			if err := setXLSXCell(f, cell, v); err != nil {
				return nil, err
			}
		}
	}

	for i := range columns {
		axis, err := columnAxis(i)
		if err != nil {
			return nil, err
		}
		if err := f.SetColWidth(xlsxSheetName, axis, axis, xlsxColumnWidth(columns, rows, i)); err != nil {
			return nil, err
		}
	}

	buf, err := f.WriteToBuffer()
	if err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// setXLSXCell 按值类型写入单元格：nil 留空、布尔写 TRUE/FALSE、数值写数字、
// 其余走字符串。[]byte 与时间类值统一转文本，避免类型被 Excel 误解为日期序列号。
func setXLSXCell(f *excelize.File, cell string, v interface{}) error {
	if v == nil {
		return nil
	}
	switch t := v.(type) {
	case string:
		return f.SetCellStr(xlsxSheetName, cell, truncateRunes(t, xlsxMaxCellRunes))
	case []byte:
		return f.SetCellStr(xlsxSheetName, cell, truncateRunes(string(t), xlsxMaxCellRunes))
	case bool:
		return f.SetCellBool(xlsxSheetName, cell, t)
	case float32:
		if !isFinite(float64(t)) {
			return f.SetCellStr(xlsxSheetName, cell, fmt.Sprintf("%v", t))
		}
		return f.SetCellValue(xlsxSheetName, cell, float64(t))
	case float64:
		if !isFinite(t) {
			return f.SetCellStr(xlsxSheetName, cell, fmt.Sprintf("%v", t))
		}
		return f.SetCellValue(xlsxSheetName, cell, t)
	case int, int8, int16, int32, int64, uint, uint8, uint16, uint32, uint64:
		if err := f.SetCellValue(xlsxSheetName, cell, t); err != nil {
			return f.SetCellStr(xlsxSheetName, cell, fmt.Sprintf("%v", t))
		}
		return nil
	default:
		return f.SetCellStr(xlsxSheetName, cell, truncateRunes(fmt.Sprintf("%v", t), xlsxMaxCellRunes))
	}
}

// xlsxColumnWidth 按列内容的显示宽度估算列宽。
// 中日韩字符按 2 个字符宽度计算，避免中文列名被挤成 ###；
// 结果夹在 [8, 60] 之间，超长文本列不会把表格撑到无法阅读。
func xlsxColumnWidth(columns []string, rows [][]interface{}, colIndex int) float64 {
	width := 0
	if colIndex < len(columns) {
		width = displayWidth(columns[colIndex])
	}
	for _, row := range rows {
		if colIndex >= len(row) {
			continue
		}
		if w := displayWidth(cellText(row[colIndex])); w > width {
			width = w
		}
	}
	const minWidth, maxWidth = 8.0, 60.0
	w := float64(width) + 2
	if w < minWidth {
		return minWidth
	}
	if w > maxWidth {
		return maxWidth
	}
	return w
}

// displayWidth 估算字符串在等宽表格中的显示宽度，CJK 与全角字符计 2。
func displayWidth(s string) int {
	w := 0
	for _, r := range s {
		if isWideRune(r) {
			w += 2
		} else {
			w++
		}
	}
	return w
}

// isWideRune 判断是否为中日韩文字、全角符号等占两列的字符。
func isWideRune(r rune) bool {
	switch {
	case r >= 0x1100 && r <= 0x115F:
		return true
	case r >= 0x2E80 && r <= 0xA4CF:
		return true
	case r >= 0xAC00 && r <= 0xD7A3:
		return true
	case r >= 0xF900 && r <= 0xFAFF:
		return true
	case r >= 0xFE30 && r <= 0xFE6F:
		return true
	case r >= 0xFF00 && r <= 0xFF60:
		return true
	case r >= 0xFFE0 && r <= 0xFFE6:
		return true
	case r >= 0x1F300 && r <= 0x1F64F:
		return true
	case r >= 0x20000 && r <= 0x3FFFD:
		return true
	}
	return false
}

// truncateRunes 按字符（非字节）截断到 max 个字符，超长时追加省略号。
// 必须按 rune 截断，否则会把多字节的 UTF-8 字符切成半个字节导致文件损坏。
func truncateRunes(s string, max int) string {
	if utf8.RuneCountInString(s) <= max {
		return s
	}
	runes := []rune(s)
	return string(runes[:max-3]) + "..."
}

// isFinite 判断浮点是否有限（非 NaN、非 Inf）。
func isFinite(v float64) bool {
	return !math.IsNaN(v) && !math.IsInf(v, 0)
}

// columnAxis 把 0 起的列下标转成 Excel 列名（0 → A, 25 → Z, 26 → AA）。
// xlsx 单表上限 16384 列，超出时报错而不是静默丢列。
func columnAxis(index int) (string, error) {
	if index < 0 {
		return "", errors.New("negative column index")
	}
	if index >= 16384 {
		return "", errors.New("too many columns for xlsx export")
	}
	name, err := excelize.ColumnNumberToName(index + 1)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(name), nil
}
