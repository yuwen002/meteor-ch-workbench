package app

import (
	"bytes"
	"encoding/csv"
	"testing"

	queryapp "github.com/yuwen002/meteor-ch-workbench/internal/application/query"
)

func TestWriteResultExportUsesSourceRowIndexes(t *testing.T) {
	columns := []queryapp.Column{{Name: "name"}, {Name: "count"}}
	sourceRows := [][]any{
		{"alpha", 2},
		{"beta", 7},
		{"gamma", 4},
	}
	// 模拟筛选 / 排序后的视图：先输出源数据第 2 行，再输出第 0 行。
	view := []resultRowView{{Index: 1}, {Index: 0}}

	var output bytes.Buffer
	if err := writeResultExport(&output, columns, view, sourceRows, "csv"); err != nil {
		t.Fatalf("writeResultExport() error = %v", err)
	}

	records, err := csv.NewReader(&output).ReadAll()
	if err != nil {
		t.Fatalf("read exported CSV: %v", err)
	}
	want := [][]string{
		{"name", "count"},
		{"beta", "7"},
		{"alpha", "2"},
	}
	if len(records) != len(want) {
		t.Fatalf("exported records = %#v, want %#v", records, want)
	}
	for i := range want {
		if len(records[i]) != len(want[i]) {
			t.Fatalf("record %d = %#v, want %#v", i, records[i], want[i])
		}
		for j := range want[i] {
			if records[i][j] != want[i][j] {
				t.Errorf("record[%d][%d] = %q, want %q", i, j, records[i][j], want[i][j])
			}
		}
	}
}

func TestWriteResultExportHandlesInvalidSourceIndex(t *testing.T) {
	columns := []queryapp.Column{{Name: "value"}}
	view := []resultRowView{{Index: 99}}
	var output bytes.Buffer
	if err := writeResultExport(&output, columns, view, nil, "csv"); err != nil {
		t.Fatalf("writeResultExport() error = %v", err)
	}
	records, err := csv.NewReader(&output).ReadAll()
	if err != nil {
		t.Fatalf("read exported CSV: %v", err)
	}
	if len(records) != 2 || records[1][0] != "NULL" {
		t.Fatalf("exported records = %#v, want NULL for invalid source index", records)
	}
}
