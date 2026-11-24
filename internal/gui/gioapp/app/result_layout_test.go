package app

import (
	"testing"

	queryapp "github.com/yuwen002/meteor-ch-workbench/internal/application/query"
)

func TestResultColumnFlexWeightsUsesContentAndBounds(t *testing.T) {
	columns := []queryapp.Column{
		{Name: "id", DatabaseType: "UInt64"},
		{Name: "描述", DatabaseType: "String"},
		{Name: "payload", DatabaseType: "String"},
	}
	rows := [][]any{
		{1, "正常", "short"},
		{2, "中文字段内容", "a very long JSON payload that should be capped"},
	}
	got := resultColumnFlexWeights(columns, rows, 40)
	if len(got) != 3 {
		t.Fatalf("weights length = %d, want 3", len(got))
	}
	if got[1] <= got[0] {
		t.Fatalf("Chinese content column weight = %v, want wider than id %v", got[1], got[0])
	}
	if got[2] != 34 {
		t.Fatalf("long content weight = %v, want capped at 34", got[2])
	}
}

func TestResultColumnFlexWeightsUsesOnlySampledRows(t *testing.T) {
	columns := []queryapp.Column{{Name: "value", DatabaseType: "String"}}
	rows := [][]any{{"short"}, {"this row is intentionally much longer"}}
	got := resultColumnFlexWeights(columns, rows, 1)
	if got[0] != 10 {
		t.Fatalf("sampled weight = %v, want minimum 10", got[0])
	}
}
