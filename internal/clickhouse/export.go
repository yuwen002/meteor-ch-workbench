// 本文件负责把查询结果集序列化为可导出的文本（CSV / JSON），
// 以及定义查询结果的默认行数上限，避免超大结果全量塞入 webview 导致界面卡死。
package clickhouse

import (
	"encoding/csv"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"strings"
)

// DefaultQueryRowLimit 是 Query 默认返回的最大行数。
// 超过该值时只保留前 N 行，并通过 QueryResult.Truncated 标记告知前端，
// 防止 `SELECT *` 之类的大结果把整个 webview 拖垮。
const DefaultQueryRowLimit = 50000

// ResultExportFormat 描述一种结果集导出格式，供前端下拉展示。
type ResultExportFormat struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Extension   string `json:"extension"`
	Mime        string `json:"mime"`
	Description string `json:"description"`
}

// resultExportRegistry 按注册顺序保存所有结果集导出格式实现。
// 新增格式只需在此追加一项，前端通过 ResultExportFormats 自动感知。
//
// Render 统一接收 target（INSERT 族需要目标表名）。CSV / JSON / Markdown
// 忽略该参数，保证「同一输入必然同一输出」的纯数据渲染语义不变。
var resultExportRegistry = []struct {
	Meta   ResultExportFormat
	Render func([]string, [][]interface{}, string) (string, error)
}{
	{
		Meta: ResultExportFormat{
			ID: "csv", Name: "CSV", Extension: "csv", Mime: "text/csv",
			Description: "逗号分隔表格，可直接用 Excel 打开",
		},
		Render: renderResultCSV,
	},
	{
		Meta: ResultExportFormat{
			ID: "json", Name: "JSON", Extension: "json", Mime: "application/json",
			Description: "对象数组，键为列名，便于二次处理",
		},
		Render: renderResultJSON,
	},
	{
		Meta: ResultExportFormat{
			ID: "markdown", Name: "Markdown", Extension: "md", Mime: "text/markdown",
			Description: "Git 风格的表格，可直接贴进文档或 Issue",
		},
		Render: renderResultMarkdown,
	},
	{
		Meta: ResultExportFormat{
			ID: "insert", Name: "SQL INSERT", Extension: "sql", Mime: "text/plain",
			Description: "可直接回灌 ClickHouse 的 INSERT 语句，需指定目标表",
		},
		Render: renderResultInsert,
	},
}

// ResultExportFormats 返回全部结果集导出格式元信息，供前端渲染格式选择。
func ResultExportFormats() []ResultExportFormat {
	formats := make([]ResultExportFormat, 0, len(resultExportRegistry))
	for _, f := range resultExportRegistry {
		formats = append(formats, f.Meta)
	}
	return formats
}

// ExportResultByFormat 按指定格式把列名与结果行渲染为文本，并返回建议扩展名。
// target 仅 INSERT 族格式使用，其余格式忽略。
func ExportResultByFormat(format string, columns []string, rows [][]interface{}, target string) (string, string, error) {
	for _, f := range resultExportRegistry {
		if f.Meta.ID == format {
			out, err := f.Render(columns, rows, target)
			if err != nil {
				return "", "", err
			}
			return out, f.Meta.Extension, nil
		}
	}
	return "", "", errors.New("unsupported format: " + format)
}

// renderResultCSV 把结果集渲染为 CSV，首行为列名。
// 单元格内换行、逗号、双引号由 encoding/csv 自动处理。
func renderResultCSV(columns []string, rows [][]interface{}, _ string) (string, error) {
	var b strings.Builder
	w := csv.NewWriter(&b)
	if err := w.Write(columns); err != nil {
		return "", err
	}
	for _, row := range rows {
		record := make([]string, len(row))
		for i, v := range row {
			record[i] = cellText(v)
		}
		if err := w.Write(record); err != nil {
			return "", err
		}
	}
	w.Flush()
	return b.String(), w.Error()
}

// renderResultJSON 把结果集渲染为对象数组，键为列名。
// 缺少列的单元格以 null 填充，保证每行键集合一致。
func renderResultJSON(columns []string, rows [][]interface{}, _ string) (string, error) {
	items := make([]map[string]interface{}, 0, len(rows))
	for _, row := range rows {
		item := make(map[string]interface{}, len(columns))
		for i, col := range columns {
			if i < len(row) {
				item[col] = row[i]
			} else {
				item[col] = nil
			}
		}
		items = append(items, item)
	}
	b, err := json.MarshalIndent(items, "", "  ")
	if err != nil {
		return "", err
	}
	return string(b), nil
}

// renderResultMarkdown 把结果集渲染为 Markdown 表格。
// 首行为列名，第二行为对齐分隔行；单元格内的 | 与换行会破坏表格结构，
// 因此 | 转义为 \|、换行转成 <br>、反斜杠本身也要先转义，避免吃掉后面的字符。
func renderResultMarkdown(columns []string, rows [][]interface{}, _ string) (string, error) {
	var b strings.Builder
	b.WriteString("| ")
	for i, col := range columns {
		if i > 0 {
			b.WriteString(" | ")
		}
		b.WriteString(escapeMarkdownCell(col))
	}
	b.WriteString(" |\n|")
	for range columns {
		b.WriteString(" --- |")
	}
	b.WriteString("\n")
	for _, row := range rows {
		b.WriteString("| ")
		for i := range columns {
			if i > 0 {
				b.WriteString(" | ")
			}
			if i < len(row) {
				b.WriteString(escapeMarkdownCell(cellText(row[i])))
			}
		}
		b.WriteString(" |\n")
	}
	return b.String(), nil
}

// renderResultInsert 把结果集渲染为可直接回灌 ClickHouse 的 INSERT 语句。
// target 为目标表名；缺少列的单元格以 NULL 填充，转义规则与前端保持一致。
func renderResultInsert(columns []string, rows [][]interface{}, target string) (string, error) {
	table := strings.TrimSpace(target)
	if table == "" {
		return "", errors.New("insert target table is required")
	}
	var b strings.Builder
	b.WriteString("INSERT INTO ")
	b.WriteString(table)
	b.WriteString(" (")
	for i, col := range columns {
		if i > 0 {
			b.WriteString(", ")
		}
		b.WriteString(escapeInsertIdentifier(col))
	}
	b.WriteString(") VALUES ")
	for r, row := range rows {
		if r > 0 {
			b.WriteString(", ")
		}
		b.WriteString("(")
		for i := range columns {
			if i > 0 {
				b.WriteString(", ")
			}
			if i < len(row) {
				b.WriteString(insertLiteral(row[i]))
			} else {
				b.WriteString("NULL")
			}
		}
		b.WriteString(")")
	}
	b.WriteString(";\n")
	return b.String(), nil
}

// escapeInsertIdentifier 用反引号包裹列名，并转义内部反引号。
func escapeInsertIdentifier(s string) string {
	return "`" + strings.ReplaceAll(s, "`", "``") + "`"
}

// insertLiteral 把单元格值转为 SQL 字面量。
// 规则与前端 frontend/src/utils/sqlLiteral.ts 的 sqlLiteral 保持一致：
// 反斜杠先双写、单引号转义为 \'、布尔转 1/0、非有限浮点转 NULL，
// 避免前后端两条导出路径产生不同的转义结果。
func insertLiteral(v interface{}) string {
	if v == nil {
		return "NULL"
	}
	switch t := v.(type) {
	case string:
		return escapeInsertString(t)
	case []byte:
		return escapeInsertString(string(t))
	case bool:
		if t {
			return "1"
		}
		return "0"
	case float32, float64:
		if math.IsNaN(toFloat64(t)) || math.IsInf(toFloat64(t), 0) {
			return "NULL"
		}
		return fmt.Sprintf("%v", t)
	case int, int8, int16, int32, int64, uint, uint8, uint16, uint32, uint64:
		return fmt.Sprintf("%v", t)
	default:
		return escapeInsertString(fmt.Sprintf("%v", t))
	}
}

// escapeInsertString 按 ClickHouse 反斜杠转义规则包裹字符串字面量。
func escapeInsertString(s string) string {
	s = strings.ReplaceAll(s, `\`, `\\`)
	s = strings.ReplaceAll(s, "'", `\'`)
	return "'" + s + "'"
}

func toFloat64(v interface{}) float64 {
	switch t := v.(type) {
	case float32:
		return float64(t)
	case float64:
		return t
	}
	return 0
}

// escapeMarkdownCell 转义会破坏 Markdown 表格的字符。
// 顺序很关键：反斜杠必须最先处理，否则会把后面新增的转义符再转义一遍。
func escapeMarkdownCell(s string) string {
	s = strings.ReplaceAll(s, `\`, `\\`)
	s = strings.ReplaceAll(s, "|", `\|`)
	s = strings.ReplaceAll(s, "\r\n", "<br>")
	s = strings.ReplaceAll(s, "\n", "<br>")
	s = strings.ReplaceAll(s, "\r", "<br>")
	return s
}

// cellText 把单元格值转为 CSV 文本，NULL 输出空串，其余使用 fmt 默认格式化。
func cellText(v interface{}) string {
	if v == nil {
		return ""
	}
	switch t := v.(type) {
	case string:
		return t
	case []byte:
		return string(t)
	default:
		return fmt.Sprintf("%v", v)
	}
}
