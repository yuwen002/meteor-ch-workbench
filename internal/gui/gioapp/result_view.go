package gioapp

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/yuwen002/meteor-ch-workbench/internal/clickhouse"
)

type resultRowView struct {
	Index int
	Values []interface{}
}

// buildResultRowView 仅构造 UI 展示顺序，不修改 ClickHouse 返回的原始结果。
func buildResultRowView(rows [][]interface{}, filter string, sortColumn int, descending bool) []resultRowView {
	return buildResultRowViewWithColumnFilters(rows, filter, nil, sortColumn, descending)
}

// buildResultRowViewWithColumnFilters 将全局关键字和各列条件按 AND 组合。
func buildResultRowViewWithColumnFilters(rows [][]interface{}, filter string, columnFilters []string, sortColumn int, descending bool) []resultRowView {
	filter = strings.TrimSpace(filter)
	view := make([]resultRowView, 0, len(rows))
	for i, row := range rows {
		if filter != "" && !resultRowMatchesFilter(row, filter) {
			continue
		}
		if !resultRowMatchesColumnFilters(row, columnFilters) {
			continue
		}
		view = append(view, resultRowView{Index: i, Values: row})
	}
	if sortColumn >= 0 {
		sort.SliceStable(view, func(i, j int) bool {
			var left, right interface{}
			if sortColumn < len(view[i].Values) { left = view[i].Values[sortColumn] }
			if sortColumn < len(view[j].Values) { right = view[j].Values[sortColumn] }
			if left == nil && right != nil { return false }
			if left != nil && right == nil { return true }
			if left == nil && right == nil { return false }
			cmp := compareResultValues(left, right)
			if descending { return cmp > 0 }
			return cmp < 0
		})
	}
	return view
}

func resultRowMatchesColumnFilters(row []interface{}, filters []string) bool {
	for column, filter := range filters {
		filter = strings.TrimSpace(filter)
		if filter == "" {
			continue
		}
		if column >= len(row) || !resultValueMatchesFilter(row[column], filter) {
			return false
		}
	}
	return true
}

func resultValueMatchesFilter(value interface{}, filter string) bool {
	text := "NULL"
	if value != nil {
		text = fmt.Sprint(value)
	}
	return strings.Contains(strings.ToLower(text), strings.ToLower(filter))
}

func resultRowMatchesFilter(row []interface{}, filter string) bool {
	needle := strings.ToLower(filter)
	for _, value := range row {
		if resultValueMatchesFilter(value, needle) {
			return true
		}
	}
	return false
}

// compareResultValues 将 NULL 放在末尾；同类型数值和时间按语义比较，其余按不区分大小写的文本比较。
func compareResultValues(left, right interface{}) int {
	leftNull, rightNull := left == nil, right == nil
	if leftNull && rightNull { return 0 }
	if leftNull { return 1 }
	if rightNull { return -1 }

	ls, rs := fmt.Sprint(left), fmt.Sprint(right)
	lf, le := strconv.ParseFloat(ls, 64)
	rf, re := strconv.ParseFloat(rs, 64)
	if le == nil && re == nil {
		switch {
		case lf < rf: return -1
		case lf > rf: return 1
		default: return 0
		}
	}
	lt, lte := parseResultTime(ls)
	rt, rte := parseResultTime(rs)
	if lte == nil && rte == nil {
		switch {
		case lt.Before(rt): return -1
		case lt.After(rt): return 1
		default: return 0
		}
	}
	ls, rs = strings.ToLower(ls), strings.ToLower(rs)
	switch {
	case ls < rs: return -1
	case ls > rs: return 1
	default: return 0
	}
}

func parseResultTime(value string) (time.Time, error) {
	layouts := []string{time.RFC3339Nano, "2006-01-02 15:04:05.999999999", "2006-01-02 15:04:05", "2006-01-02"}
	for _, layout := range layouts {
		if parsed, err := time.Parse(layout, value); err == nil { return parsed, nil }
	}
	return time.Time{}, fmt.Errorf("not a supported timestamp")
}

// resultColumnLabel 显示当前列的排序方向。
func resultColumnLabel(columns []clickhouse.ColumnInfo, column, sortColumn int, descending bool) string {
	if column < 0 || column >= len(columns) { return "" }
	label := columns[column].Name
	if column == sortColumn {
		if descending { return label + " ↓" }
		return label + " ↑"
	}
	return label
}
