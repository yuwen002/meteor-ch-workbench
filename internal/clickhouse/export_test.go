package clickhouse

import (
	"encoding/csv"
	"strings"
	"testing"
)

func TestRenderResultCSVEscapes(t *testing.T) {
	cols := []string{"a", "b"}
	rows := [][]interface{}{
		{"x,y", nil},
		{`he said "hi"`, "line\nbreak"},
	}
	out, err := renderResultCSV(cols, rows)
	if err != nil {
		t.Fatalf("renderResultCSV: %v", err)
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
	out, err := renderResultJSON(cols, rows)
	if err != nil {
		t.Fatalf("renderResultJSON: %v", err)
	}
	if !strings.Contains(out, `"a"`) || !strings.Contains(out, `"c": null`) {
		t.Fatalf("expected null padding for missing cell, got: %s", out)
	}
}

func TestExportResultByFormatUnknown(t *testing.T) {
	if _, _, err := ExportResultByFormat("xml", []string{"a"}, nil); err == nil {
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
	out, err := renderResultMarkdown([]string{"a", "b"}, [][]interface{}{
		{"x,y", nil},
		{`he said "hi"`, "line\nbreak"},
	})
	if err != nil {
		t.Fatalf("renderResultMarkdown: %v", err)
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
	out, err := renderResultMarkdown([]string{"c|1", "c2"}, [][]interface{}{
		{"a|b", "multi\nline"},
		{`back\slash`, "quote\"x"},
	})
	if err != nil {
		t.Fatalf("renderResultMarkdown: %v", err)
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
			out, err := renderResultMarkdown(c.cols, c.rows)
			if err != nil {
				t.Fatalf("renderResultMarkdown: %v", err)
			}
			if out != c.want {
				t.Fatalf("got %q, want %q", out, c.want)
			}
		})
	}
}

func TestExportResultByFormatMarkdown(t *testing.T) {
	out, ext, err := ExportResultByFormat("markdown", []string{"a"}, [][]interface{}{{1}})
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
