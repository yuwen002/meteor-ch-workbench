package clickhouse

import (
	"bytes"
	"encoding/json"
	"errors"
	"math"
	"strings"
	"testing"

	excelize "github.com/xuri/excelize/v2"
)

// TestRenderResultJSONL 覆盖 JSON Lines 的逐行对象、缺列补 null 与空结果。
func TestRenderResultJSONL(t *testing.T) {
	cases := []struct {
		name  string
		cols  []string
		rows  [][]interface{}
		lines []string
	}{
		{
			name:  "两行各成一行 JSON",
			cols:  []string{"a", "b"},
			rows:  [][]interface{}{{1, "x"}, {2, "y"}},
			lines: []string{`{"a":1,"b":"x"}`, `{"a":2,"b":"y"}`},
		},
		{
			name:  "缺列的行补 null",
			cols:  []string{"a", "b"},
			rows:  [][]interface{}{{1}},
			lines: []string{`{"a":1,"b":null}`},
		},
		{
			name:  "零行输出空内容",
			cols:  []string{"a"},
			rows:  nil,
			lines: nil,
		},
		{
			name:  "单元格含换行与引号仍是单行",
			cols:  []string{"t"},
			rows:  [][]interface{}{{"l1\nl2"}, {`q"x`}},
			lines: []string{`{"t":"l1\nl2"}`, `{"t":"q\"x"}`},
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			out, ext, err := ExportResultByFormat("jsonl", c.cols, c.rows, "")
			if err != nil {
				t.Fatalf("ExportResultByFormat jsonl: %v", err)
			}
			if ext != "jsonl" {
				t.Fatalf("want extension jsonl, got %q", ext)
			}
			if len(c.lines) == 0 {
				if out != "" {
					t.Fatalf("want empty output, got %q", out)
				}
				return
			}
			got := strings.Split(strings.TrimSuffix(out, "\n"), "\n")
			if len(got) != len(c.lines) {
				t.Fatalf("want %d lines, got %d: %q", len(c.lines), len(got), out)
			}
			for i, want := range c.lines {
				if got[i] != want {
					t.Fatalf("line %d = %q, want %q", i, got[i], want)
				}
			}
		})
	}
}

// TestRenderResultJSONLIsValidJSONPerLine 确认每行都能独立反序列化。
func TestRenderResultJSONLIsValidJSONPerLine(t *testing.T) {
	out, _, err := ExportResultByFormat("jsonl", []string{"a", "b"}, [][]interface{}{
		{1, nil}, {2, "x"},
	}, "")
	if err != nil {
		t.Fatalf("ExportResultByFormat jsonl: %v", err)
	}
	for i, line := range strings.Split(strings.TrimSuffix(out, "\n"), "\n") {
		var v map[string]interface{}
		if err := json.Unmarshal([]byte(line), &v); err != nil {
			t.Fatalf("line %d is not valid JSON: %v (%q)", i, err, line)
		}
		if len(v) != 2 {
			t.Fatalf("line %d should keep both keys, got %v", i, v)
		}
	}
}

// TestEscapeTSVCell 覆盖 TSV 单元格转义（顺序敏感，反斜杠必须最先双写）。
func TestEscapeTSVCell(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want string
	}{
		{"制表符转 t", "a\tb", `a\tb`},
		{"换行转 n", "line1\nline2", `line1\nline2`},
		{"CRLF 只转一个 n", "line1\r\nline2", `line1\nline2`},
		{"单独 CR 转 r", "line1\rline2", `line1\rline2`},
		{"反斜杠先双写", `a\b`, `a\\b`},
		{"反斜杠加制表符", "a\\\tb", `a\\\tb`},
		{"空字符串", "", ""},
		{"普通文本不变", "hello", "hello"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := escapeTSVCell(c.in); got != c.want {
				t.Fatalf("escapeTSVCell(%q) = %q, want %q", c.in, got, c.want)
			}
		})
	}
}

// TestRenderResultTSV 覆盖 TSV 结构：表头行、单元格数量、NULL 为空串。
func TestRenderResultTSV(t *testing.T) {
	cases := []struct {
		name  string
		cols  []string
		rows  [][]interface{}
		lines []string
	}{
		{
			name:  "表头加两行数据",
			cols:  []string{"a", "b"},
			rows:  [][]interface{}{{1, nil}, {"x", "y"}},
			lines: []string{"a\tb", "1\t", "x\ty"},
		},
		{
			name:  "缺列的行留空单元格",
			cols:  []string{"a", "b", "c"},
			rows:  [][]interface{}{{1}},
			lines: []string{"a\tb\tc", "1\t\t"},
		},
		{
			name:  "零行只输出表头",
			cols:  []string{"a"},
			rows:  nil,
			lines: []string{"a"},
		},
		{
			name:  "单元格内的制表符被转义不会多出一列",
			cols:  []string{"a", "b"},
			rows:  [][]interface{}{{"x\ty", 2}},
			lines: []string{"a\tb", `x\ty` + "\t2"},
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			out, ext, err := ExportResultByFormat("tsv", c.cols, c.rows, "")
			if err != nil {
				t.Fatalf("ExportResultByFormat tsv: %v", err)
			}
			if ext != "tsv" {
				t.Fatalf("want extension tsv, got %q", ext)
			}
			got := strings.Split(strings.TrimSuffix(out, "\n"), "\n")
			if len(got) != len(c.lines) {
				t.Fatalf("want %d lines, got %d: %q", len(c.lines), len(got), out)
			}
			for i, want := range c.lines {
				if got[i] != want {
					t.Fatalf("line %d = %q, want %q", i, got[i], want)
				}
			}
			for i, line := range got {
				if fields := len(strings.Split(line, "\t")); fields != len(c.cols) {
					t.Fatalf("line %d has %d fields, want %d: %q", i, fields, len(c.cols), line)
				}
			}
		})
	}
}

// TestInferColumnType 覆盖列类型推断（取首个非空值，全空回落 String）。
func TestInferColumnType(t *testing.T) {
	cases := []struct {
		name string
		rows [][]interface{}
		col  int
		want string
	}{
		{"整数推 Int64", [][]interface{}{{1}, {2}}, 0, "Int64"},
		{"浮点推 Float64", [][]interface{}{{1.5}}, 0, "Float64"},
		{"布尔推 UInt8", [][]interface{}{{true}}, 0, "UInt8"},
		{"字符串推 String", [][]interface{}{{"x"}}, 0, "String"},
		{"首个非空决定类型", [][]interface{}{{nil}, {7}}, 0, "Int64"},
		{"全部为空回落 String", [][]interface{}{{nil}, {nil}}, 0, "String"},
		{"无行回落 String", nil, 0, "String"},
		{"某行缺该列时跳过", [][]interface{}{{1}, {2, "x"}}, 1, "String"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := inferColumnType(c.rows, c.col); got != c.want {
				t.Fatalf("inferColumnType = %q, want %q", got, c.want)
			}
		})
	}
}

// TestRenderResultDDLStructure 覆盖建表语句 + 插入语句的组合结构。
func TestRenderResultDDLStructure(t *testing.T) {
	out, ext, err := ExportResultByFormat("sql_ddl",
		[]string{"id", "name", "ok"},
		[][]interface{}{{1, "x'y", true}, {2, nil, false}},
		"`db`.`t`",
	)
	if err != nil {
		t.Fatalf("ExportResultByFormat sql_ddl: %v", err)
	}
	if ext != "sql" {
		t.Fatalf("want extension sql, got %q", ext)
	}
	if !strings.HasPrefix(out, "CREATE TABLE IF NOT EXISTS `db`.`t` (\n") {
		t.Fatalf("unexpected DDL head: %q", out)
	}
	if !strings.Contains(out, "  `id` Int64,\n") {
		t.Fatalf("id column type missing: %q", out)
	}
	if !strings.Contains(out, "  `name` String,\n") {
		t.Fatalf("name column type missing: %q", out)
	}
	if !strings.Contains(out, "  `ok` UInt8\n") {
		t.Fatalf("ok column type missing: %q", out)
	}
	if !strings.Contains(out, ");\nINSERT INTO `db`.`t` (`id`, `name`, `ok`) VALUES (1, 'x\\'y', 1), (2, NULL, 0);") {
		t.Fatalf("insert part mismatch: %q", out)
	}
}

// TestRenderResultDDLEdgeCases 覆盖目标表缺失与零行等边界。
func TestRenderResultDDLEdgeCases(t *testing.T) {
	cases := []struct {
		name    string
		cols    []string
		rows    [][]interface{}
		target  string
		wantErr bool
		wantHas string
	}{
		{"目标表为空报错", []string{"a"}, [][]interface{}{{1}}, "", true, ""},
		{"目标表仅空白报错", []string{"a"}, [][]interface{}{{1}}, "  ", true, ""},
		{"零行仍输出建表骨架", []string{"a"}, nil, "t", false, "CREATE TABLE IF NOT EXISTS t"},
		{"零列退化", nil, nil, "t", false, "CREATE TABLE IF NOT EXISTS t (\n);\n"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			out, _, err := ExportResultByFormat("sql_ddl", c.cols, c.rows, c.target)
			if c.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("ExportResultByFormat sql_ddl: %v", err)
			}
			if !strings.Contains(out, c.wantHas) {
				t.Fatalf("output %q does not contain %q", out, c.wantHas)
			}
		})
	}
}

// TestColumnAxis 覆盖列下标到 Excel 列名的换算。
func TestColumnAxis(t *testing.T) {
	cases := []struct {
		name    string
		index   int
		want    string
		wantErr bool
	}{
		{"首列 A", 0, "A", false},
		{"第 26 列 Z", 25, "Z", false},
		{"第 27 列 AA", 26, "AA", false},
		{"第 28 列 AB", 27, "AB", false},
		{"末列 XFD", 16383, "XFD", false},
		{"超出上限报错", 16384, "", true},
		{"负下标报错", -1, "", true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, err := columnAxis(c.index)
			if c.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("columnAxis: %v", err)
			}
			if got != c.want {
				t.Fatalf("columnAxis(%d) = %q, want %q", c.index, got, c.want)
			}
		})
	}
}

// TestTruncateRunes 覆盖按字符截断，确保多字节字符不被切断。
func TestTruncateRunes(t *testing.T) {
	cases := []struct {
		name string
		in   string
		max  int
		want string
	}{
		{"未超长原样返回", "abc", 32767, "abc"},
		{"超长截断并追加省略号", "abcdefg", 6, "abc..."},
		{"中文按字符计数", "数据库导出", 5, "数据库导出"},
		{"中文超长不切半个字符", "数据库导出测试", 5, "数据..."},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := truncateRunes(c.in, c.max); got != c.want {
				t.Fatalf("truncateRunes(%q, %d) = %q, want %q", c.in, c.max, got, c.want)
			}
		})
	}
}

// TestDisplayWidth 覆盖 CJK 宽度估算。
func TestDisplayWidth(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want int
	}{
		{"纯 ASCII", "abc", 3},
		{"中文双宽", "中文", 4},
		{"中英混排", "a中", 3},
		{"空字符串", "", 0},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := displayWidth(c.in); got != c.want {
				t.Fatalf("displayWidth(%q) = %d, want %d", c.in, got, c.want)
			}
		})
	}
}

// TestRenderResultXLSXContent 打开生成的工作簿，校验表头、类型与单元格内容。
func TestRenderResultXLSXContent(t *testing.T) {
	var buf bytes.Buffer
	ext, err := ExportResultTo(&buf, "xlsx", []string{"id", "name", "ok"}, [][]interface{}{
		{1, "x", true},
		{2, nil, false},
	}, "")
	if err != nil {
		t.Fatalf("ExportResultTo xlsx: %v", err)
	}
	if ext != "xlsx" {
		t.Fatalf("want extension xlsx, got %q", ext)
	}
	f, err := excelize.OpenReader(bytes.NewReader(buf.Bytes()))
	if err != nil {
		t.Fatalf("generated xlsx is not readable: %v", err)
	}
	defer f.Close()

	sheets := f.GetSheetList()
	if len(sheets) != 1 || sheets[0] != xlsxSheetName {
		t.Fatalf("unexpected sheets: %v", sheets)
	}

	header, err := f.GetRows(xlsxSheetName, excelize.Options{RawCellValue: true})
	if err != nil {
		t.Fatalf("GetRows: %v", err)
	}
	if len(header) != 3 {
		t.Fatalf("want 3 rows (header + 2 data), got %d", len(header))
	}
	if header[0][0] != "id" || header[0][1] != "name" || header[0][2] != "ok" {
		t.Fatalf("unexpected header: %v", header[0])
	}
	if header[1][0] != "1" || header[1][1] != "x" || header[1][2] != "1" {
		t.Fatalf("unexpected row 1: %v", header[1])
	}
	if header[2][1] != "" {
		t.Fatalf("nil cell should be empty, got %q", header[2][1])
	}

	vals, err := f.GetCellValue(xlsxSheetName, "A2")
	if err != nil {
		t.Fatalf("GetCellValue: %v", err)
	}
	if vals != "1" {
		t.Fatalf("A2 = %q, want 1", vals)
	}
}

// TestRenderResultXLSXEdgeCases 覆盖零行零列与超长文本。
func TestRenderResultXLSXEdgeCases(t *testing.T) {
	cases := []struct {
		name    string
		cols    []string
		rows    [][]interface{}
		wantRow string
	}{
		{"零行只含表头", []string{"a", "b"}, nil, ""},
		{"零列零行", nil, nil, ""},
		{"超长文本被截断", []string{"t"}, [][]interface{}{{strings.Repeat("数", xlsxMaxCellRunes+100)}}, "..."},
		{"非有限浮点转文本", []string{"f"}, [][]interface{}{{mathNaN()}}, "NaN"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			var buf bytes.Buffer
			if _, err := ExportResultTo(&buf, "xlsx", c.cols, c.rows, ""); err != nil {
				t.Fatalf("ExportResultTo xlsx: %v", err)
			}
			f, err := excelize.OpenReader(bytes.NewReader(buf.Bytes()))
			if err != nil {
				t.Fatalf("generated xlsx is not readable: %v", err)
			}
			defer f.Close()
			if len(c.rows) == 0 {
				return
			}
			rows, err := f.GetRows(xlsxSheetName, excelize.Options{RawCellValue: true})
			if err != nil {
				t.Fatalf("GetRows: %v", err)
			}
			if len(rows) != 2 {
				t.Fatalf("want header + 1 row, got %d", len(rows))
			}
			got := rows[1][0]
			if !strings.Contains(got, c.wantRow) {
				t.Fatalf("cell = %q, want it to contain %q", got, c.wantRow)
			}
			if len([]rune(got)) > xlsxMaxCellRunes {
				t.Fatalf("cell not truncated: %d runes", len([]rune(got)))
			}
		})
	}
}

// TestRenderResultXLSXWriteError 覆盖 Writer 失败时把错误透传出去。
func TestRenderResultXLSXWriteError(t *testing.T) {
	w := &failingWriter{err: errors.New("disk full")}
	if _, err := ExportResultTo(w, "xlsx", []string{"a"}, [][]interface{}{{1}}, ""); err == nil {
		t.Fatal("expected write error to be returned")
	}
}

// mathNaN 返回 NaN，单独封装避免在表驱动用例里反复 import math。
func mathNaN() float64 {
	return math.NaN()
}
