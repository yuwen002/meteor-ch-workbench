package app

import (
	"fmt"
	"math/big"
	"sort"
	"strings"
	"time"

	queryapp "github.com/yuwen002/meteor-ch-workbench/internal/application/query"
)

// resultRowView 只保存源结果行索引，避免为每一行重复持有 Values 切片头。
type resultRowView struct {
	Index int
}

// buildResultRowView 仅构造 UI 展示顺序，不修改查询返回的原始结果。
func buildResultRowView(rows [][]any, filter string, sortColumn int, descending bool) []resultRowView {
	return buildResultRowViewWithColumnFilters(rows, filter, nil, sortColumn, descending)
}

// buildResultRowViewWithColumnFilters 将全局关键字和各列条件按 AND 组合。
func buildResultRowViewWithColumnFilters(rows [][]any, filter string, columnFilters []string, sortColumn int, descending bool) []resultRowView {
	var sortKeys []resultSortKey
	if sortColumn >= 0 {
		sortKeys = extendResultSortKeys(rows, sortColumn, nil)
	}
	return buildResultRowViewWithPreparedSortKeys(rows, filter, columnFilters, sortColumn, descending, sortKeys)
}

// buildResultRowViewWithPreparedSortKeys 使用调用方维护的排序键缓存构建展示索引。
// 缓存只包含当前排序列的键，不复制原始结果行。
func buildResultRowViewWithPreparedSortKeys(rows [][]any, filter string, columnFilters []string, sortColumn int, descending bool, sortKeys []resultSortKey) []resultRowView {
	filter = strings.TrimSpace(filter)
	filterNeedle := strings.ToLower(filter)
	view := make([]resultRowView, 0, len(rows))
	for i, row := range rows {
		if filterNeedle != "" && !resultRowMatchesLowerFilter(row, filterNeedle) {
			continue
		}
		if !resultRowMatchesColumnFilters(row, columnFilters) {
			continue
		}
		view = append(view, resultRowView{Index: i})
	}
	if sortColumn >= 0 {
		if len(sortKeys) < len(rows) {
			sortKeys = extendResultSortKeys(rows, sortColumn, sortKeys)
		}
		sort.SliceStable(view, func(i, j int) bool {
			cmp := compareResultSortKeys(sortKeys[view[i].Index], sortKeys[view[j].Index])
			if descending {
				return cmp > 0
			}
			return cmp < 0
		})
	}
	return view
}

// extendResultSortKeys 只为新增结果行计算排序键，供筛选变化和排序方向切换复用。
func extendResultSortKeys(rows [][]any, sortColumn int, keys []resultSortKey) []resultSortKey {
	if sortColumn < 0 {
		return nil
	}
	if len(keys) > len(rows) {
		keys = nil
	}
	if cap(keys) < len(rows) {
		newCapacity := cap(keys) * 2
		if newCapacity < len(rows) {
			newCapacity = len(rows)
		}
		if newCapacity == 0 {
			newCapacity = len(rows)
		}
		extended := make([]resultSortKey, len(keys), newCapacity)
		copy(extended, keys)
		keys = extended
	}
	for i := len(keys); i < len(rows); i++ {
		var value any
		if sortColumn < len(rows[i]) {
			value = rows[i][sortColumn]
		}
		keys = append(keys, newResultSortKey(value))
	}
	return keys
}

// resultSortKey 将排序值的昂贵转换预计算一次，供稳定排序的比较器复用。
type resultSortKey struct {
	null      bool
	numeric   bool
	timeValid bool
	number    big.Rat
	timestamp time.Time
	text      string
}

func newResultSortKey(value any) resultSortKey {
	if value == nil {
		return resultSortKey{null: true}
	}
	text := fmt.Sprint(value)
	key := resultSortKey{text: strings.ToLower(text)}
	_, key.numeric = key.number.SetString(strings.TrimSpace(text))
	if !key.numeric {
		var timeErr error
		key.timestamp, timeErr = parseResultTime(text)
		key.timeValid = timeErr == nil
	}
	return key
}

// compareResultSortKeys 保持原有排序语义：NULL 置底，随后依次尝试数值、时间和不区分大小写文本。
func compareResultSortKeys(left, right resultSortKey) int {
	if left.null && right.null {
		return 0
	}
	if left.null {
		return 1
	}
	if right.null {
		return -1
	}
	if left.numeric && right.numeric {
		return left.number.Cmp(&right.number)
	}
	if left.timeValid && right.timeValid {
		switch {
		case left.timestamp.Before(right.timestamp):
			return -1
		case left.timestamp.After(right.timestamp):
			return 1
		default:
			return 0
		}
	}
	switch {
	case left.text < right.text:
		return -1
	case left.text > right.text:
		return 1
	default:
		return 0
	}
}

func resultRowMatchesColumnFilters(row []any, filters []string) bool {
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

// resultValueMatchesColumnCondition 支持包含、!不包含、=、!=、>、<、>=、<=、区间和 NULL 判断。
func resultValueMatchesColumnCondition(value any, expression string) bool {
	expression = strings.TrimSpace(expression)
	if expression == "" {
		return true
	}
	if strings.EqualFold(expression, "is:null") {
		return value == nil
	}
	if strings.EqualFold(expression, "is:not-null") {
		return value != nil
	}
	if strings.HasPrefix(strings.ToLower(expression), "between:") {
		if value == nil {
			return false
		}
		bounds := strings.SplitN(strings.TrimSpace(expression[len("between:"):]), "..", 2)
		if len(bounds) != 2 || strings.TrimSpace(bounds[0]) == "" || strings.TrimSpace(bounds[1]) == "" {
			return false
		}
		lower, upper := strings.TrimSpace(bounds[0]), strings.TrimSpace(bounds[1])
		actual := fmt.Sprint(value)
		if lowerCmp, lowerOK := compareResultNumbers(actual, lower); lowerOK {
			upperCmp, upperOK := compareResultNumbers(actual, upper)
			if !upperOK {
				return false
			}
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
	if value != nil {
		actual = fmt.Sprint(value)
	}
	if operator == "" || operator == "!" {
		matched := resultValueMatchesFilter(value, expected)
		if operator == "!" {
			return !matched
		}
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
		if operator == "!=" {
			return !matched
		}
		return matched
	}
	if value == nil {
		return false
	}
	if comparison, ok := compareResultNumbers(actual, expected); ok {
		switch operator {
		case ">":
			return comparison > 0
		case "<":
			return comparison < 0
		case ">=":
			return comparison >= 0
		case "<=":
			return comparison <= 0
		}
		return false
	}
	leftTime, leftTimeErr := parseResultTime(actual)
	rightTime, rightTimeErr := parseResultTime(expected)
	if leftTimeErr != nil || rightTimeErr != nil {
		return false
	}
	switch operator {
	case ">":
		return leftTime.After(rightTime)
	case "<":
		return leftTime.Before(rightTime)
	case ">=":
		return !leftTime.Before(rightTime)
	case "<=":
		return !leftTime.After(rightTime)
	default:
		return false
	}
}

func resultValueMatchesFilter(value any, filter string) bool {
	text := "NULL"
	if value != nil {
		text = fmt.Sprint(value)
	}
	return strings.Contains(strings.ToLower(text), strings.ToLower(filter))
}

func resultRowMatchesFilter(row []any, filter string) bool {
	return resultRowMatchesLowerFilter(row, strings.ToLower(filter))
}

// resultRowMatchesLowerFilter 接收已标准化的关键词，避免每一行重复转换筛选词大小写。
func resultRowMatchesLowerFilter(row []any, needle string) bool {
	for _, value := range row {
		text := "NULL"
		if value != nil {
			text = fmt.Sprint(value)
		}
		if strings.Contains(strings.ToLower(text), needle) {
			return true
		}
	}
	return false
}

// compareResultValues 将 NULL 放在末尾；数值和时间按语义比较，其余按文本比较。
func compareResultValues(left, right any) int {
	leftNull, rightNull := left == nil, right == nil
	if leftNull && rightNull {
		return 0
	}
	if leftNull {
		return 1
	}
	if rightNull {
		return -1
	}

	ls, rs := fmt.Sprint(left), fmt.Sprint(right)
	if comparison, ok := compareResultNumbers(ls, rs); ok {
		return comparison
	}
	lt, lte := parseResultTime(ls)
	rt, rte := parseResultTime(rs)
	if lte == nil && rte == nil {
		switch {
		case lt.Before(rt):
			return -1
		case lt.After(rt):
			return 1
		default:
			return 0
		}
	}
	ls, rs = strings.ToLower(ls), strings.ToLower(rs)
	switch {
	case ls < rs:
		return -1
	case ls > rs:
		return 1
	default:
		return 0
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
		if parsed, err := time.Parse(layout, value); err == nil {
			return parsed, nil
		}
	}
	return time.Time{}, fmt.Errorf("not a supported timestamp")
}

// resultColumnLabel 显示当前列的排序方向。
func resultColumnLabel(columns []queryapp.Column, column, sortColumn int, descending bool) string {
	if column < 0 || column >= len(columns) {
		return ""
	}
	label := columns[column].Name
	if column == sortColumn {
		if descending {
			return label + " ↓"
		}
		return label + " ↑"
	}
	return label
}

// resultColumnFilterSummary 返回非空列条件的可读摘要，忽略超出列定义的条件。
func resultColumnFilterSummary(columns []queryapp.Column, filters []string) []string {
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
