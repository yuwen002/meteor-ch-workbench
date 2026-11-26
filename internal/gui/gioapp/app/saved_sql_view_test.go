package app

import (
	"testing"

	savedsqldomain "github.com/yuwen002/meteor-ch-workbench/internal/domain/savedsql"
)

func TestFilterSavedSQL(t *testing.T) {
	items := []savedsqldomain.Entry{
		{ID: "1", Name: "日常巡检", SQL: "SELECT 1"},
		{ID: "2", Name: "订单统计", SQL: "SELECT count() FROM orders"},
		{ID: "3", Name: "ClickHouse 版本", SQL: "SELECT version()"},
	}

	tests := []struct {
		name string
		keyword string
		want []string
	}{
		{name: "空关键字返回全部", want: []string{"1", "2", "3"}},
		{name: "按名称搜索且忽略大小写", keyword: "CLICKHOUSE", want: []string{"3"}},
		{name: "按 SQL 内容搜索", keyword: "orders", want: []string{"2"}},
		{name: "去除关键字首尾空格", keyword: "  日常  ", want: []string{"1"}},
		{name: "无匹配结果", keyword: "missing"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := filterSavedSQL(items, tt.keyword)
			if len(got) != len(tt.want) {
				t.Fatalf("结果数量 = %d，期望 %d", len(got), len(tt.want))
			}
			for i, id := range tt.want {
				if got[i].ID != id {
					t.Fatalf("第 %d 条 ID = %q，期望 %q", i, got[i].ID, id)
				}
			}
		})
	}
}


func TestSyncSavedSQLControlsKeepsIdentityAndPrunesRemovedItems(t *testing.T) {
	var state workspace
	first := []savedsqldomain.Entry{
		{ID: "alpha", Name: "Alpha"},
		{ID: "beta", Name: "Beta"},
	}
	state.syncSavedSQLControls(first)
	betaControls := state.savedSQLControls["beta"]
	if betaControls == nil {
		t.Fatal("expected controls for beta")
	}

	// Reordering and filtering must not bind a visible row to another item's controls.
	state.syncSavedSQLControls([]savedsqldomain.Entry{
		{ID: "beta", Name: "Beta"},
	})
	if state.savedSQLControls["beta"] != betaControls {
		t.Fatal("controls identity changed after reorder/filter")
	}
	if _, ok := state.savedSQLControls["alpha"]; ok {
		t.Fatal("controls for removed item were not pruned")
	}
}
