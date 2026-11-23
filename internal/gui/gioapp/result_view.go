package gioapp

import (
	"fmt"
	"math/big"
	"sort"
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
		if column >= len(row) || !resultValueMatchesColumnCondition(row[column], filter) {
			return false
		}
	}
	return true
}

// resultValueMatchesColumnCondition 支持默认包含、!不包含、=、!=、>、<、>=、<= 及 NULL 判断。
func resultValueMatchesColumnCondition(value interface{}, expression string) bool {
	expression = strings.TrimSpace(expression)
	if expression == "" { return true }
	if strings.EqualFold(expression, "is:null") { return value == nil }
	if strings.EqualFold(expression, "is:not-null") { return value != nil }
	if strings.HasPrefix(strings.ToLower(expression), "between:") {
		if value == nil { return false }
		bounds := strings.SplitN(strings.TrimSpace(expression[len("between:"):]), "..", 2)
		if len(bounds) != 2 || strings.TrimSpace(bounds[0]) == "" || strings.TrimSpace(bounds[1]) == "" {
			return false
		}
		lower, upper := strings.TrimSpace(bounds[0]), strings.TrimSpace(bounds[1])
		actual := fmt.Sprint(value)
		if lowerCmp, lowerOK := compareResultNumbers(actual, lower); lowerOK {
			upperCmp, upperOK := compareResultNumbers(actual, upper)
			if !upperOK { return false }
			boundOrder, orderOK := compareResultNumbers(lower, upper)
			return orderOK && boundOrder <= 0 && lowerCmp >= 0 && upperCmp <= 0
		}
		actualTime, actualErr := parseResultTime(actual)
		lowerTime, lowerErr := parseResultTime(lower)
		upperTime, upperErr := parseResultTime(upper)
		return actualErr == nil && lowerErr == nil && upperErr == nil &&
			!actualTime.Before(lowerTime) && !actualTime.After(upperTime) && !lowerTime.After(upperTime)
	}

	operator, expected := "", expression
	for _, candidate := range []string{">=", "<=", "!=", "=", ">", "<", "!"} {
		if strings.HasPrefix(expression, candidate) {
			operator, expected = candidate, strings.TrimSpace(strings.TrimPrefix(expression, candidate))
			break
		}
	}
	actual := "NULL"
	if value != nil { actual = fmt.Sprint(value) }
	if operator == "" || operator == "!" {
		matched := resultValueMatchesFilter(value, expected)
		if operator == "!" { return !matched }
		return matched
	}
	if operator == "=" || operator == "!=" {
		matched := strings.EqualFold(actual, expected)
		if comparison, ok := compareResultNumbers(actual, expected); ok {
			matched = comparison == 0
		} else if leftTime, err := parseResultTime(actual); err == nil {
			if rightTime, rightErr := parseResultTime(expected); rightErr == nil {
				matched = leftTime.Equal(rightTime)
			}
		}
		if operator == "!=" { return !matched }
		return matched
	}
	if value == nil { return false }
	if comparison, ok := compareResultNumbers(actual, expected); ok {
		switch operator {
		case ">": return comparison > 0
		case "<": return comparison < 0
		case ">=": return comparison >= 0
		case "<=": return comparison <= 0
		}
		return false
	}

	// 时间列复用统一解析规则：带时区的 RFC3339 按其偏移比较；
	// 不带时区的 ClickHouse DateTime / DateTime64 文本按 UTC 解释。
	leftTime, leftTimeErr := parseResultTime(actual)
	rightTime, rightTimeErr := parseResultTime(expected)
	if leftTimeErr != nil || rightTimeErr != nil { return false }
	switch operator {
	case ">": return leftTime.After(rightTime)
	case "<": return leftTime.Before(rightTime)
	case ">=": return !leftTime.Before(rightTime)
	case "<=": return !leftTime.After(rightTime)
	default: return false
	}
}

func resultValueMatchesFilter(value interface{}, filter string) bool {
	text := "NULL"
	if value != nil { text = fmt.Sprint(value) }
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
	if comparison, ok := compareResultNumbers(ls, rs); ok {
		return comparison
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

// compareResultNumbers 使用有理数比较，避免 UInt64 和高精度 Decimal 转 float64 后丢失精度。
func compareResultNumbers(left, right string) (int, bool) {
	leftNumber, ok := new(big.Rat).SetString(strings.TrimSpace(left))
	if !ok {
		return 0, false
	}
	rightNumber, ok := new(big.Rat).SetString(strings.TrimSpace(right))
	if !ok {
		return 0, false
	}
	return leftNumber.Cmp(rightNumber), true
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

// resultColumnFilterSummary 返回非空列条件的可读摘要，忽略超出列定义的条件。
func resultColumnFilterSummary(columns []clickhouse.Column, filters []string) []string {
	limit := len(columns)
	if len(filters) < limit {
		limit = len(filters)
	}
	var summaries []string
	for i := 0; i < limit; i++ {
		expression := strings.TrimSpace(filters[i])
		if expression == "" {
			continue
		}
		summaries = append(summaries, fmt.Sprintf("%s: %s", columns[i].Name, expression))
	}
	return summaries
}
