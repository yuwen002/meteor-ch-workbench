package clickhouse

import (
	"bytes"
	"encoding/csv"
	"errors"
	"math"
	"strings"
	"testing"
)

func TestRenderResultCSVEscapes(t *testing.T) {
	cols := []string{"a", "b"}
	rows := [][]interface{}{
		{"x,y", nil},
		{`he said "hi"`, "line\nbreak"},
	}
	out, _, err := ExportResultByFormat("csv", cols, rows, "")
	if err != nil {
		t.Fatalf("ExportResultByFormat csv: %v", err)
	}
	records, err := csv.NewReader(strings.NewReader(out)).ReadAll()
	if err != nil {
		t.Fatalf("csv parse: %v", err)
	}
	if len(records) != 3 {
		t.Fatalf("want 3 records, got %d", len(records))
	}
	if records[0][0] != "a" || records[0][1] != "b" {
		t.Fatalf("unexpected header: %v", records[0])
	}
	if records[1][0] != "x,y" || records[1][1] != "" {
		t.Fatalf("unexpected row1: %v", records[1])
	}
	if records[2][0] != `he said "hi"` || records[2][1] != "line\nbreak" {
		t.Fatalf("unexpected row2: %v", records[2])
	}
}

func TestRenderResultJSONPadsMissingCells(t *testing.T) {
	cols := []string{"a", "b", "c"}
	rows := [][]interface{}{
		{1, "two", []interface{}{3, 4}},
		{5, "six"},
	}
	out, _, err := ExportResultByFormat("json", cols, rows, "")
	if err != nil {
		t.Fatalf("ExportResultByFormat json: %v", err)
	}
	if !strings.Contains(out, `"a"`) || !strings.Contains(out, `"c": null`) {
		t.Fatalf("expected null padding for missing cell, got: %s", out)
	}
}

func TestExportResultByFormatUnknown(t *testing.T) {
	if _, _, err := ExportResultByFormat("xml", []string{"a"}, nil, ""); err == nil {
		t.Fatal("expected error for unsupported format")
	}
}

func TestEscapeMarkdownCell(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want string
	}{
		{"竖线需转义", "a|b", `a\|b`},
		{"换行转 br", "line1\nline2", "line1<br>line2"},
		{"CRLF 转单个 br", "line1\r\nline2", "line1<br>line2"},
		{"单独 CR 转 br", "line1\rline2", "line1<br>line2"},
		{"反斜杠先双写再处理竖线", `a\|b`, `a\\\|b`},
		{"已有换行与竖线混合", "a|b\r\nc|d", `a\|b<br>c\|d`},
		{"空字符串", "", ""},
		{"普通文本不变", "hello", "hello"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := escapeMarkdownCell(c.in); got != c.want {
				t.Fatalf("escapeMarkdownCell(%q) = %q, want %q", c.in, got, c.want)
			}
		})
	}
}

func TestRenderResultMarkdownStructure(t *testing.T) {
	out, _, err := ExportResultByFormat("markdown", []string{"a", "b"}, [][]interface{}{
		{"x,y", nil},
		{`he said "hi"`, "line\nbreak"},
	}, "")
	if err != nil {
		t.Fatalf("ExportResultByFormat markdown: %v", err)
	}
	lines := strings.Split(strings.TrimSuffix(out, "\n"), "\n")
	if len(lines) != 4 {
		t.Fatalf("want 4 lines (header + separator + 2 rows), got %d: %q", len(lines), out)
	}
	if lines[0] != "| a | b |" {
		t.Fatalf("unexpected header: %q", lines[0])
	}
	if lines[1] != "| --- | --- |" {
		t.Fatalf("unexpected separator: %q", lines[1])
	}
	if lines[2] != "| x,y |  |" {
		t.Fatalf("unexpected row1: %q", lines[2])
	}
	if lines[3] != `| he said "hi" | line<br>break |` {
		t.Fatalf("unexpected row2: %q", lines[3])
	}
	for i, line := range lines {
		if !strings.HasPrefix(line, "|") || !strings.HasSuffix(line, "|") {
			t.Fatalf("line %d not wrapped by pipe: %q", i, line)
		}
	}
}

func TestRenderResultMarkdownEscapesBreakingChars(t *testing.T) {
	out, _, err := ExportResultByFormat("markdown", []string{"c|1", "c2"}, [][]interface{}{
		{"a|b", "multi\nline"},
		{`back\slash`, "quote\"x"},
	}, "")
	if err != nil {
		t.Fatalf("ExportResultByFormat markdown: %v", err)
	}
	for i, line := range strings.Split(strings.TrimSuffix(out, "\n"), "\n") {
		if line == "" {
			continue
		}
		if !strings.HasPrefix(line, "|") || !strings.HasSuffix(line, "|") {
			t.Fatalf("line %d not wrapped by pipe: %q", i, line)
		}
		body := strings.TrimSuffix(strings.TrimPrefix(line, "|"), "|")
		unescaped := strings.ReplaceAll(body, `\|`, "")
		if strings.Count(unescaped, "|") != 1 {
			t.Fatalf("line %d has wrong cell count: %q", i, line)
		}
	}
	if !strings.Contains(out, `a\|b`) {
		t.Fatalf("pipe not escaped in cell: %s", out)
	}
	if !strings.Contains(out, "multi<br>line") {
		t.Fatalf("newline not converted to br: %s", out)
	}
	if !strings.Contains(out, `c\|1`) {
		t.Fatalf("pipe not escaped in header: %s", out)
	}
}

func TestRenderResultMarkdownEdgeCases(t *testing.T) {
	cases := []struct {
		name string
		cols []string
		rows [][]interface{}
		want string
	}{
		{"零行只输出表头与分隔行", []string{"a", "b"}, nil, "| a | b |\n| --- | --- |\n"},
		{"缺列的行补空单元格", []string{"a", "b", "c"}, [][]interface{}{{1}}, "| a | b | c |\n| --- | --- | --- |\n| 1 |  |  |\n"},
		{"零列输出退化表格（实际流程不会出现）", nil, [][]interface{}{{1}}, "|  |\n|\n|  |\n"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			out, _, err := ExportResultByFormat("markdown", c.cols, c.rows, "")
			if err != nil {
				t.Fatalf("ExportResultByFormat markdown: %v", err)
			}
			if out != c.want {
				t.Fatalf("got %q, want %q", out, c.want)
			}
		})
	}
}

func TestExportResultByFormatMarkdown(t *testing.T) {
	out, ext, err := ExportResultByFormat("markdown", []string{"a"}, [][]interface{}{{1}}, "")
	if err != nil {
		t.Fatalf("ExportResultByFormat: %v", err)
	}
	if ext != "md" {
		t.Fatalf("want extension md, got %q", ext)
	}
	if !strings.Contains(out, "| a |") {
		t.Fatalf("unexpected markdown output: %s", out)
	}
}

// TestInsertLiteral 覆盖单元格值 → SQL 字面量，规则与前端 sqlLiteral.ts 对齐。
func TestInsertLiteral(t *testing.T) {
	cases := []struct {
		name string
		in   interface{}
		want string
	}{
		{"nil 输出 NULL", nil, "NULL"},
		{"普通字符串", "abc", "'abc'"},
		{"单引号反斜杠转义", "a'b", `'a\'b'`},
		{"反斜杠先双写", `a\b`, `'a\\b'`},
		{"换行保持原样", "line1\nline2", "'line1\nline2'"},
		{"空字符串", "", "''"},
		{"整数", 42, "42"},
		{"浮点数", 1.5, "1.5"},
		{"NaN 转 NULL", math.NaN(), "NULL"},
		{"Inf 转 NULL", math.Inf(1), "NULL"},
		{"true 转 1", true, "1"},
		{"false 转 0", false, "0"},
		{"字节串按字符串处理", []byte("a'b"), `'a\'b'`},
		{"中文", "中文", "'中文'"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := insertLiteral(c.in); got != c.want {
				t.Fatalf("insertLiteral(%v) = %q, want %q", c.in, got, c.want)
			}
		})
	}
}

// TestEscapeInsertIdentifier 覆盖标识符反引号包裹与内部反引号双写。
func TestEscapeInsertIdentifier(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want string
	}{
		{"普通列名", "a", "`a`"},
		{"内部反引号双写", "a`b", "`a``b`"},
		{"中文列名", "用户", "`用户`"},
		{"空列名", "", "``"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := escapeInsertIdentifier(c.in); got != c.want {
				t.Fatalf("escapeInsertIdentifier(%q) = %q, want %q", c.in, got, c.want)
			}
		})
	}
}

func TestRenderResultInsertStructure(t *testing.T) {
	out, _, err := ExportResultByFormat(
		"insert",
		[]string{"a", "b"},
		[][]interface{}{{"x'y", nil}, {"plain", 1}},
		"`db`.`t`",
	)
	if err != nil {
		t.Fatalf("ExportResultByFormat insert: %v", err)
	}
	lines := strings.Split(strings.TrimSuffix(out, "\n"), "\n")
	if len(lines) != 1 {
		t.Fatalf("want 1 line (single INSERT statement), got %d: %q", len(lines), out)
	}
	want := "INSERT INTO `db`.`t` (`a`, `b`) VALUES ('x\\'y', NULL), ('plain', 1);"
	if lines[0] != want {
		t.Fatalf("unexpected statement: %q, want %q", lines[0], want)
	}
}

func TestRenderResultInsertEdgeCases(t *testing.T) {
	cases := []struct {
		name    string
		cols    []string
		rows    [][]interface{}
		target  string
		wantErr bool
		wantHas string
	}{
		{"目标表为空报错", []string{"a"}, [][]interface{}{{1}}, "", true, ""},
		{"目标表仅空白报错", []string{"a"}, [][]interface{}{{1}}, "   ", true, ""},
		{"零行只输出语句骨架", []string{"a"}, nil, "t", false, "INSERT INTO t (`a`) VALUES ;"},
		{"缺列的行补 NULL", []string{"a", "b"}, [][]interface{}{{1}}, "t", false, "(1, NULL);"},
		{"零列退化", nil, [][]interface{}{{}}, "t", false, "INSERT INTO t () VALUES ();"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			out, _, err := ExportResultByFormat("insert", c.cols, c.rows, c.target)
			if c.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("ExportResultByFormat insert: %v", err)
			}
			if !strings.Contains(out, c.wantHas) {
				t.Fatalf("output %q does not contain %q", out, c.wantHas)
			}
		})
	}
}

func TestExportResultByFormatInsert(t *testing.T) {
	out, ext, err := ExportResultByFormat("insert", []string{"a"}, [][]interface{}{{1}}, "test_table")
	if err != nil {
		t.Fatalf("ExportResultByFormat: %v", err)
	}
	if ext != "sql" {
		t.Fatalf("want extension sql, got %q", ext)
	}
	if !strings.Contains(out, "INSERT INTO test_table") {
		t.Fatalf("unexpected insert output: %s", out)
	}
}

func TestExportResultByFormatIgnoresTargetForOtherFormats(t *testing.T) {
	csvOut, _, err := ExportResultByFormat("csv", []string{"a"}, [][]interface{}{{1}}, "")
	if err != nil {
		t.Fatalf("ExportResultByFormat csv: %v", err)
	}
	csvOutWithTarget, _, err := ExportResultByFormat("csv", []string{"a"}, [][]interface{}{{1}}, "some_table")
	if err != nil {
		t.Fatalf("ExportResultByFormat csv with target: %v", err)
	}
	if csvOut != csvOutWithTarget {
		t.Fatalf("target must not affect csv output: %q vs %q", csvOut, csvOutWithTarget)
	}
}

func TestResultExportFormatsRegistry(t *testing.T) {
	formats := ResultExportFormats()
	if len(formats) == 0 {
		t.Fatal("expected at least one export format")
	}
	seen := map[string]bool{}
	for _, f := range formats {
		if f.ID == "" || f.Name == "" || f.Extension == "" {
			t.Fatalf("incomplete format meta: %+v", f)
		}
		if seen[f.ID] {
			t.Fatalf("duplicate format id: %s", f.ID)
		}
		seen[f.ID] = true
	}
}

// TestExportResultToMatchesByFormat 确保流式写出与一次性渲染逐字节一致，
// 这是「另存为」改为边生成边落盘的前提。
func TestExportResultToMatchesByFormat(t *testing.T) {
	cases := []struct {
		format string
		cols   []string
		rows   [][]interface{}
		target string
	}{
		{"csv", []string{"a", "b"}, [][]interface{}{{"x,y", nil}, {1, "z"}}, ""},
		{"csv", []string{"a"}, nil, ""},
		{"json", []string{"a", "b"}, [][]interface{}{{"x", 1}, {"y", nil}}, ""},
		{"json", []string{"a"}, nil, ""},
		{"json", []string{"a", "b", "c"}, [][]interface{}{{1}}, ""},
		{"markdown", []string{"a|1", "b"}, [][]interface{}{{"x\ny", 2}}, ""},
		{"markdown", []string{"a"}, nil, ""},
		{"insert", []string{"a", "b"}, [][]interface{}{{"x'y", nil}, {3, "z"}}, "t"},
		{"insert", []string{"a"}, nil, "t"},
	}
	for _, c := range cases {
		t.Run(c.format, func(t *testing.T) {
			byFormat, ext1, err := ExportResultByFormat(c.format, c.cols, c.rows, c.target)
			if err != nil {
				t.Fatalf("ExportResultByFormat: %v", err)
			}
			var buf bytes.Buffer
			ext2, err := ExportResultTo(&buf, c.format, c.cols, c.rows, c.target)
			if err != nil {
				t.Fatalf("ExportResultTo: %v", err)
			}
			if ext1 != ext2 {
				t.Fatalf("extension mismatch: %q vs %q", ext1, ext2)
			}
			if buf.String() != byFormat {
				t.Fatalf("stream output differs from buffered output:\nstream=%q\nbuffer=%q", buf.String(), byFormat)
			}
		})
	}
}

// TestExportResultToWriteError 覆盖 Writer 失败时把错误透传出去的边界。
func TestExportResultToWriteError(t *testing.T) {
	errWriter := errors.New("disk full")
	w := &failingWriter{err: errWriter}
	if _, err := ExportResultTo(w, "csv", []string{"a"}, [][]interface{}{{1}}, ""); err == nil {
		t.Fatal("expected write error to be returned")
	}
}

// TestExportResultToUnknownFormat 未知格式返回错误且不写任何内容。
func TestExportResultToUnknownFormat(t *testing.T) {
	var buf bytes.Buffer
	if _, err := ExportResultTo(&buf, "xml", []string{"a"}, nil, ""); err == nil {
		t.Fatal("expected error for unknown format")
	}
	if buf.Len() != 0 {
		t.Fatalf("nothing should be written, got %q", buf.String())
	}
}

// TestResultExportExtension 扩展名查询与注册表一致，未知格式返回空串。
func TestResultExportExtension(t *testing.T) {
	for _, f := range ResultExportFormats() {
		if got := ResultExportExtension(f.ID); got != f.Extension {
			t.Fatalf("%s extension = %q, want %q", f.ID, got, f.Extension)
		}
	}
	if got := ResultExportExtension("xml"); got != "" {
		t.Fatalf("unknown format extension = %q, want empty", got)
	}
}

// failingWriter 在写入时立即返回固定错误，用于模拟磁盘写入失败。
type failingWriter struct{ err error }

func (f *failingWriter) Write(p []byte) (int, error) { return 0, f.err }
