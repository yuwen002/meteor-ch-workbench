// 本文件负责把查询结果集序列化为可导出的文本（CSV / JSON），
// 以及定义查询结果的默认行数上限，避免超大结果全量塞入 webview 导致界面卡死。
package clickhouse

import (
	"encoding/csv"
	"encoding/json"
	"errors"
	"fmt"
	"io"
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
// RenderTo 直接写 io.Writer，供「另存为」边生成边落盘，避免先拼出完整
// 字符串再写文件造成的双份内存占用；Render 是 RenderTo 的 bytes.Buffer
// 适配，两者输出必须逐字节一致（export_test.go 有往返一致性用例）。
// 统一接收 target（INSERT 族需要目标表名）。CSV / JSON / Markdown
// 忽略该参数，保证「同一输入必然同一输出」的纯数据渲染语义不变。
var resultExportRegistry = []struct {
	Meta     ResultExportFormat
	RenderTo func(io.Writer, []string, [][]interface{}, string) error
}{
	{
		Meta: ResultExportFormat{
			ID: "csv", Name: "CSV", Extension: "csv", Mime: "text/csv",
			Description: "逗号分隔表格，可直接用 Excel 打开",
		},
		RenderTo: renderResultCSVTo,
	},
	{
		Meta: ResultExportFormat{
			ID: "json", Name: "JSON", Extension: "json", Mime: "application/json",
			Description: "对象数组，键为列名，便于二次处理",
		},
		RenderTo: renderResultJSONTo,
	},
	{
		Meta: ResultExportFormat{
			ID: "markdown", Name: "Markdown", Extension: "md", Mime: "text/markdown",
			Description: "Git 风格的表格，可直接贴进文档或 Issue",
		},
		RenderTo: renderResultMarkdownTo,
	},
	{
		Meta: ResultExportFormat{
			ID: "insert", Name: "SQL INSERT", Extension: "sql", Mime: "text/plain",
			Description: "可直接回灌 ClickHouse 的 INSERT 语句，需指定目标表",
		},
		RenderTo: renderResultInsertTo,
	},
	{
		Meta: ResultExportFormat{
			ID: "jsonl", Name: "JSON Lines", Extension: "jsonl", Mime: "application/x-ndjson",
			Description: "每行一个 JSON 对象，适合流式处理与 jq / 数据管道，无内存峰值",
		},
		RenderTo: renderResultJSONLTo,
	},
	{
		Meta: ResultExportFormat{
			ID: "tsv", Name: "TSV", Extension: "tsv", Mime: "text/tab-separated-values",
			Description: "制表符分隔，粘贴到表格工具不易串列，ClickHouse 原生格式",
		},
		RenderTo: renderResultTSVTo,
	},
	{
		Meta: ResultExportFormat{
			ID: "xlsx", Name: "Excel", Extension: "xlsx",
			Mime:        "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet",
			Description: "真正的 .xlsx 工作簿，表头冻结、列宽自适应，可直接交付给非技术同事",
		},
		RenderTo: renderResultXLSXTo,
	},
	{
		Meta: ResultExportFormat{
			ID: "sql_ddl", Name: "SQL DDL + INSERT", Extension: "sql", Mime: "text/plain",
			Description: "先 CREATE TABLE 再 INSERT，可在新库一键建表回灌，需指定目标表",
		},
		RenderTo: renderResultDDLTo,
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
// 内部走 bytes.Buffer 适配 ExportResultTo，供剪贴板复制与测试复用。
func ExportResultByFormat(format string, columns []string, rows [][]interface{}, target string) (string, string, error) {
	var buf strings.Builder
	ext, err := ExportResultTo(&buf, format, columns, rows, target)
	if err != nil {
		return "", "", err
	}
	return buf.String(), ext, nil
}

// ResultExportExtension 返回指定格式的建议扩展名，未知格式返回空串。
// 供「另存为」在弹对话框前设置默认文件名与过滤器，无需先渲染内容。
func ResultExportExtension(format string) string {
	for _, f := range resultExportRegistry {
		if f.Meta.ID == format {
			return f.Meta.Extension
		}
	}
	return ""
}

// ExportResultTo 按指定格式把结果集增量写入 w，并返回该格式的建议扩展名。
// 「另存为」直接传 os.File，边格式化边落盘，避免完整字符串常驻内存。
// 与 ExportResultByFormat 输出逐字节一致。
func ExportResultTo(w io.Writer, format string, columns []string, rows [][]interface{}, target string) (string, error) {
	for _, f := range resultExportRegistry {
		if f.Meta.ID == format {
			if err := f.RenderTo(w, columns, rows, target); err != nil {
				return "", err
			}
			return f.Meta.Extension, nil
		}
	}
	return "", errors.New("unsupported format: " + format)
}

// renderResultCSVTo 把结果集渲染为 CSV，首行为列名。
// 单元格内换行、逗号、双引号由 encoding/csv 自动处理。
// csv.Writer 自带缓冲，每写一行即刷新，内存占用与总行数无关。
func renderResultCSVTo(w io.Writer, columns []string, rows [][]interface{}, _ string) error {
	cw := csv.NewWriter(w)
	if err := cw.Write(columns); err != nil {
		return err
	}
	for _, row := range rows {
		record := make([]string, len(row))
		for i, v := range row {
			record[i] = cellText(v)
		}
		if err := cw.Write(record); err != nil {
			return err
		}
	}
	cw.Flush()
	return cw.Error()
}

// renderResultJSONTo 把结果集渲染为对象数组，键为列名。
// 缺少列的单元格以 null 填充，保证每行键集合一致。
//
// 输出必须与 json.MarshalIndent(items, "", "  ") 逐字节一致：
// 每行单独 MarshalIndent 后再缩进两格拼接，避免为整表构造一次性 []map，
// 同时保证键顺序稳定（encoding/json 对 map 按键名排序）。
func renderResultJSONTo(w io.Writer, columns []string, rows [][]interface{}, _ string) error {
	if len(rows) == 0 {
		_, err := io.WriteString(w, "[]")
		return err
	}
	if _, err := io.WriteString(w, "[\n"); err != nil {
		return err
	}
	for i, row := range rows {
		if i > 0 {
			if _, err := io.WriteString(w, ",\n"); err != nil {
				return err
			}
		}
		item := make(map[string]interface{}, len(columns))
		for ci, col := range columns {
			if ci < len(row) {
				item[col] = row[ci]
			} else {
				item[col] = nil
			}
		}
		b, err := json.MarshalIndent(item, "", "  ")
		if err != nil {
			return err
		}
		indented := strings.ReplaceAll(string(b), "\n", "\n  ")
		if _, err := io.WriteString(w, "  "+indented); err != nil {
			return err
		}
	}
	_, err := io.WriteString(w, "\n]")
	return err
}

// renderResultMarkdownTo 把结果集渲染为 Markdown 表格。
// 首行为列名，第二行为对齐分隔行；单元格内的 | 与换行会破坏表格结构，
// 因此 | 转义为 \|、换行转成 <br>、反斜杠本身也要先转义，避免吃掉后面的字符。
func renderResultMarkdownTo(w io.Writer, columns []string, rows [][]interface{}, _ string) error {
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
	if err := flushBuilder(w, &b); err != nil {
		return err
	}
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
		if b.Len() >= 4096 {
			if err := flushBuilder(w, &b); err != nil {
				return err
			}
		}
	}
	return flushBuilder(w, &b)
}

// renderResultInsertTo 把结果集渲染为可直接回灌 ClickHouse 的 INSERT 语句。
// target 为目标表名；缺少列的单元格以 NULL 填充，转义规则与前端保持一致。
func renderResultInsertTo(w io.Writer, columns []string, rows [][]interface{}, target string) error {
	table := strings.TrimSpace(target)
	if table == "" {
		return errors.New("insert target table is required")
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
		if b.Len() >= 4096 {
			if err := flushBuilder(w, &b); err != nil {
				return err
			}
		}
	}
	b.WriteString(";\n")
	return flushBuilder(w, &b)
}

// flushBuilder 把缓冲内容写入 w 并清空缓冲区，用于控制流式渲染的峰值内存。
func flushBuilder(w io.Writer, b *strings.Builder) error {
	if b.Len() == 0 {
		return nil
	}
	if _, err := io.WriteString(w, b.String()); err != nil {
		return err
	}
	b.Reset()
	return nil
}

// renderResultJSONLTo 把结果集渲染为 JSON Lines（每行一个 JSON 对象，无分隔逗号）。
// 与 JSON 格式的区别：不整表包成数组，任意行都能独立解析，适合 jq / 流式管道；
// 且无需回读已写内容，天然支持极大的结果集。
// 缺少列的单元格以 null 填充，键顺序与 JSON 格式一致（map 序列化按键名排序）。
func renderResultJSONLTo(w io.Writer, columns []string, rows [][]interface{}, _ string) error {
	var b strings.Builder
	for _, row := range rows {
		item := make(map[string]interface{}, len(columns))
		for ci, col := range columns {
			if ci < len(row) {
				item[col] = row[ci]
			} else {
				item[col] = nil
			}
		}
		line, err := json.Marshal(item)
		if err != nil {
			return err
		}
		b.Write(line)
		b.WriteByte('\n')
		if b.Len() >= 4096 {
			if err := flushBuilder(w, &b); err != nil {
				return err
			}
		}
	}
	return flushBuilder(w, &b)
}

// renderResultTSVTo 把结果集渲染为 TSV，首行为列名。
// 单元格内的制表符、换行、回车会破坏「一行一记录」的结构，
// 按 ClickHouse 原生 TSV 约定转义为 \t / \n / \r，反斜杠本身先双写。
func renderResultTSVTo(w io.Writer, columns []string, rows [][]interface{}, _ string) error {
	var b strings.Builder
	for i, col := range columns {
		if i > 0 {
			b.WriteByte('\t')
		}
		b.WriteString(escapeTSVCell(col))
	}
	b.WriteByte('\n')
	for _, row := range rows {
		for i := range columns {
			if i > 0 {
				b.WriteByte('\t')
			}
			if i < len(row) {
				b.WriteString(escapeTSVCell(cellText(row[i])))
			}
		}
		b.WriteByte('\n')
		if b.Len() >= 4096 {
			if err := flushBuilder(w, &b); err != nil {
				return err
			}
		}
	}
	return flushBuilder(w, &b)
}

// escapeTSVCell 转义会破坏 TSV 行结构或与 ClickHouse 转义语法冲突的字符。
// 顺序很关键：反斜杠必须最先双写，否则会把后面新增的转义符再转义一遍。
func escapeTSVCell(s string) string {
	s = strings.ReplaceAll(s, `\`, `\\`)
	s = strings.ReplaceAll(s, "\t", `\t`)
	s = strings.ReplaceAll(s, "\r\n", `\n`)
	s = strings.ReplaceAll(s, "\n", `\n`)
	s = strings.ReplaceAll(s, "\r", `\r`)
	return s
}

// renderResultDDLTo 把结果集渲染为「建表 + 插入」的完整 SQL 脚本。
// 列类型按取值推断：整数 → Int64、浮点 → Float64、布尔 → UInt8、其余 → String，
// 便于在没有原表结构时先建出一张可用表；target 为目标表名（必填）。
// 数据部分复用 renderResultInsertTo，转义规则与纯 INSERT 导出一致。
func renderResultDDLTo(w io.Writer, columns []string, rows [][]interface{}, target string) error {
	table := strings.TrimSpace(target)
	if table == "" {
		return errors.New("ddl target table is required")
	}
	var b strings.Builder
	b.WriteString("CREATE TABLE IF NOT EXISTS ")
	b.WriteString(table)
	b.WriteString(" (\n")
	for i, col := range columns {
		b.WriteString("  ")
		b.WriteString(escapeInsertIdentifier(col))
		b.WriteByte(' ')
		b.WriteString(inferColumnType(rows, i))
		if i < len(columns)-1 {
			b.WriteByte(',')
		}
		b.WriteByte('\n')
	}
	b.WriteString(");\n")
	if err := flushBuilder(w, &b); err != nil {
		return err
	}
	return renderResultInsertTo(w, columns, rows, target)
}

// inferColumnType 按该列首个非空值的类型推断 SQL 列类型，全部为空时回落 String。
func inferColumnType(rows [][]interface{}, colIndex int) string {
	for _, row := range rows {
		if colIndex >= len(row) {
			continue
		}
		switch row[colIndex].(type) {
		case nil:
			continue
		case bool:
			return "UInt8"
		case int, int8, int16, int32, int64, uint, uint8, uint16, uint32, uint64:
			return "Int64"
		case float32, float64:
			return "Float64"
		default:
			return "String"
		}
	}
	return "String"
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
		// Array / Map 可能由驱动返回为不同具体切片或映射类型，
		// 统一按 JSON 编码，避免 fmt 的 [a b] / map[k:v] 表示不稳定。
		kind := reflect.TypeOf(v).Kind()
		if kind == reflect.Array || kind == reflect.Slice || kind == reflect.Map {
			if encoded, err := json.Marshal(v); err == nil {
				return string(encoded)
			}
		}
		return fmt.Sprintf("%v", v)
	}
}
