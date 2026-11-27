package app

import (
	"errors"
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
	return buildResultRowViewWithPreparedSortKeysInto(nil, rows, filter, columnFilters, sortColumn, descending, sortKeys)
}

// buildResultRowViewWithPreparedSortKeysInto 在调用方提供的索引切片上构建视图。
// Gio 结果列表在同一事件循环内消费该切片，因此刷新时可以安全复用其底层数组。
func buildResultRowViewWithPreparedSortKeysInto(dst []resultRowView, rows [][]any, filter string, columnFilters []string, sortColumn int, descending bool, sortKeys []resultSortKey) []resultRowView {
	filter = strings.TrimSpace(filter)
	filterNeedle := strings.ToLower(filter)
	preparedColumnFilters := prepareResultColumnFilters(columnFilters)
	view := dst[:0]
	for i, row := range rows {
		if filterNeedle != "" && !resultRowMatchesLowerFilter(row, filterNeedle) {
			continue
		}
		if !resultRowMatchesPreparedColumnFilters(row, preparedColumnFilters) {
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
		minimumCapacity := len(rows) * 2
		if newCapacity < minimumCapacity {
			newCapacity = minimumCapacity
		}
		if newCapacity < len(rows) {
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
	text := resultValueText(value)
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

type resultColumnFilterKind uint8

const (
	resultColumnFilterContains resultColumnFilterKind = iota
	resultColumnFilterNull
	resultColumnFilterBetween
	resultColumnFilterCompare
)

type preparedResultColumnFilter struct {
	active        bool
	kind          resultColumnFilterKind
	operator      string
	expected      string
	expectedLower string
	number        *big.Rat
	timestamp     time.Time
	timeValid     bool
	lowerNumber   *big.Rat
	upperNumber   *big.Rat
	lowerTime     time.Time
	upperTime     time.Time
	rangeNumeric  bool
	rangeTime     bool
	valid         bool
}

// prepareResultColumnFilters 在扫描结果行之前解析列条件，避免逐行重复拆分表达式。
func prepareResultColumnFilters(filters []string) []preparedResultColumnFilter {
	prepared := make([]preparedResultColumnFilter, 0, len(filters))
	for _, raw := range filters {
		expression := strings.TrimSpace(raw)
		if expression == "" {
			prepared = append(prepared, preparedResultColumnFilter{})
			continue
		}
		condition := preparedResultColumnFilter{active: true, valid: true, expected: expression}
		if strings.EqualFold(expression, "is:null") || strings.EqualFold(expression, "is:not-null") {
			condition.kind = resultColumnFilterNull
			condition.operator = strings.ToLower(expression)
			prepared = append(prepared, condition)
			continue
		}
		if strings.HasPrefix(strings.ToLower(expression), "between:") {
			condition.kind = resultColumnFilterBetween
			bounds := strings.SplitN(strings.TrimSpace(expression[len("between:"):]), "..", 2)
			if len(bounds) != 2 || strings.TrimSpace(bounds[0]) == "" || strings.TrimSpace(bounds[1]) == "" {
				condition.valid = false
			} else {
				lower, upper := strings.TrimSpace(bounds[0]), strings.TrimSpace(bounds[1])
				condition.lowerNumber, condition.rangeNumeric = new(big.Rat).SetString(lower)
				if condition.rangeNumeric {
					condition.upperNumber, condition.rangeNumeric = new(big.Rat).SetString(upper)
					if condition.rangeNumeric {
						condition.rangeNumeric = condition.lowerNumber.Cmp(condition.upperNumber) <= 0
					}
				}
				if !condition.rangeNumeric {
					var lowerErr, upperErr error
					condition.lowerTime, lowerErr = parseResultTime(lower)
					condition.upperTime, upperErr = parseResultTime(upper)
					condition.rangeTime = lowerErr == nil && upperErr == nil && !condition.lowerTime.After(condition.upperTime)
				}
				condition.valid = condition.rangeNumeric || condition.rangeTime
			}
			prepared = append(prepared, condition)
			continue
		}
		condition.kind = resultColumnFilterContains
		if strings.HasPrefix(expression, "!") && !strings.HasPrefix(expression, "!=") {
			condition.operator = "!"
			condition.expected = strings.TrimSpace(strings.TrimPrefix(expression, "!"))
		} else {
			for _, candidate := range []string{">=", "<=", "!=", "=", ">", "<"} {
				if strings.HasPrefix(expression, candidate) {
					condition.operator = candidate
					condition.expected = strings.TrimSpace(strings.TrimPrefix(expression, candidate))
					condition.kind = resultColumnFilterCompare
					break
				}
			}
		}
		condition.expectedLower = strings.ToLower(condition.expected)
		if condition.kind == resultColumnFilterCompare {
			condition.number, condition.valid = new(big.Rat).SetString(strings.TrimSpace(condition.expected))
			if !condition.valid {
				parsedTime, errTime := parseResultTime(condition.expected)
				condition.timestamp = parsedTime
				condition.timeValid = errTime == nil
				condition.valid = condition.timeValid || condition.operator == "=" || condition.operator == "!="
			}
		}
		prepared = append(prepared, condition)
	}
	return prepared
}

func resultRowMatchesColumnFilters(row []any, filters []string) bool {
	return resultRowMatchesPreparedColumnFilters(row, prepareResultColumnFilters(filters))
}

func resultRowMatchesPreparedColumnFilters(row []any, filters []preparedResultColumnFilter) bool {
	for column, filter := range filters {
		if !filter.active {
			continue
		}
		if column >= len(row) || !filter.matches(row[column]) {
			return false
		}
	}
	return true
}

func (filter preparedResultColumnFilter) matches(value any) bool {
	if !filter.active {
		return true
	}
	if !filter.valid {
		return false
	}
	if filter.kind == resultColumnFilterNull {
		if filter.operator == "is:null" {
			return value == nil
		}
		return value != nil
	}
	actual := resultValueText(value)
	if filter.kind == resultColumnFilterContains {
		matched := resultTextContainsLowerNeedle(actual, filter.expectedLower)
		if filter.operator == "!" {
			return !matched
		}
		return matched
	}
	if filter.kind == resultColumnFilterBetween {
		if value == nil {
			return false
		}
		if filter.rangeNumeric {
			actualNumber, ok := new(big.Rat).SetString(strings.TrimSpace(actual))
			return ok && actualNumber.Cmp(filter.lowerNumber) >= 0 && actualNumber.Cmp(filter.upperNumber) <= 0
		}
		actualTime, err := parseResultTime(actual)
		return filter.rangeTime && err == nil && !actualTime.Before(filter.lowerTime) && !actualTime.After(filter.upperTime)
	}
	if filter.operator == "" || filter.operator == "!" {
		matched := strings.Contains(strings.ToLower(actual), filter.expectedLower)
		if filter.operator == "!" {
			return !matched
		}
		return matched
	}
	if filter.operator == "=" || filter.operator == "!=" {
		matched := strings.EqualFold(actual, filter.expected)
		if filter.number != nil {
			if actualNumber, ok := new(big.Rat).SetString(strings.TrimSpace(actual)); ok {
				matched = actualNumber.Cmp(filter.number) == 0
			}
		} else if filter.timeValid {
			if actualTime, err := parseResultTime(actual); err == nil {
				matched = actualTime.Equal(filter.timestamp)
			}
		}
		if filter.operator == "!=" {
			return !matched
		}
		return matched
	}
	if value == nil {
		return false
	}
	if filter.number != nil {
		if actualNumber, ok := new(big.Rat).SetString(strings.TrimSpace(actual)); ok {
			comparison := actualNumber.Cmp(filter.number)
			switch filter.operator {
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
	}
	if filter.timeValid {
		actualTime, err := parseResultTime(actual)
		if err == nil {
			switch filter.operator {
			case ">":
				return actualTime.After(filter.timestamp)
			case "<":
				return actualTime.Before(filter.timestamp)
			case ">=":
				return !actualTime.Before(filter.timestamp)
			case "<=":
				return !actualTime.After(filter.timestamp)
			}
		}
	}
	return false
}

// resultValueMatchesColumnCondition 保留单值调用入口，统一使用预解析条件。
func resultValueMatchesColumnCondition(value any, expression string) bool {
	return preparedResultColumnFilterFor(expression).matches(value)
}

func preparedResultColumnFilterFor(expression string) preparedResultColumnFilter {
	filters := prepareResultColumnFilters([]string{expression})
	if len(filters) == 0 {
		return preparedResultColumnFilter{valid: true, kind: resultColumnFilterContains, expectedLower: ""}
	}
	return filters[0]
}

func resultValueMatchesFilter(value any, filter string) bool {
	return resultTextContainsLowerNeedle(resultValueText(value), strings.ToLower(filter))
}

func resultRowMatchesFilter(row []any, filter string) bool {
	return resultRowMatchesLowerFilter(row, strings.ToLower(filter))
}

// resultValueText 保留结果筛选的既有格式语义，同时避免对原生 string 调用 fmt.Sprint。
func resultValueText(value any) string {
	if value == nil {
		return "NULL"
	}
	if text, ok := value.(string); ok {
		return text
	}
	return fmt.Sprint(value)
}


// resultTextContainsLowerNeedle performs case-insensitive substring matching.
// ASCII text is compared byte-by-byte without allocating a lowercased copy;
// non-ASCII text keeps strings.ToLower semantics for Unicode compatibility.
// needle must already be lowercased, as prepared by the caller.
func resultTextContainsLowerNeedle(value, needle string) bool {
	if needle == "" {
		return true
	}
	for i := 0; i < len(value); i++ {
		if value[i] >= 0x80 {
			return strings.Contains(strings.ToLower(value), needle)
		}
	}
	for i := 0; i < len(needle); i++ {
		if needle[i] >= 0x80 {
			return strings.Contains(strings.ToLower(value), needle)
		}
	}
	if len(needle) > len(value) {
		return false
	}
	for start := 0; start <= len(value)-len(needle); start++ {
		if asciiLower(value[start]) != needle[0] {
			continue
		}
		matched := true
		for offset := 1; offset < len(needle); offset++ {
			if asciiLower(value[start+offset]) != needle[offset] {
				matched = false
				break
			}
		}
		if matched {
			return true
		}
	}
	return false
}

func asciiLower(value byte) byte {
	if value >= 'A' && value <= 'Z' {
		return value + ('a' - 'A')
	}
	return value
}

// resultRowMatchesLowerFilter 接收已标准化的关键词，避免每一行重复转换筛选词大小写。
func resultRowMatchesLowerFilter(row []any, needle string) bool {
	for _, value := range row {
		if resultTextContainsLowerNeedle(resultValueText(value), needle) {
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

	ls, rs := resultValueText(left), resultValueText(right)
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

var errUnsupportedResultTime = errors.New("not a supported timestamp")

// hasResultDatePrefix avoids trying four time layouts for ordinary text and numeric values.
// Every supported layout starts with a fixed-width YYYY-MM-DD date.
func hasResultDatePrefix(value string) bool {
	if len(value) < 10 || value[4] != '-' || value[7] != '-' {
		return false
	}
	return value[0] >= '0' && value[0] <= '9' &&
		value[1] >= '0' && value[1] <= '9' &&
		value[2] >= '0' && value[2] <= '9' &&
		value[3] >= '0' && value[3] <= '9' &&
		value[5] >= '0' && value[5] <= '9' &&
		value[6] >= '0' && value[6] <= '9' &&
		value[8] >= '0' && value[8] <= '9' &&
		value[9] >= '0' && value[9] <= '9'
}

func parseResultTime(value string) (time.Time, error) {
	if !hasResultDatePrefix(value) {
		return time.Time{}, errUnsupportedResultTime
	}
	layouts := []string{time.RFC3339Nano, "2006-01-02 15:04:05.999999999", "2006-01-02 15:04:05", "2006-01-02"}
	for _, layout := range layouts {
		if parsed, err := time.Parse(layout, value); err == nil {
			return parsed, nil
		}
	}
	return time.Time{}, errUnsupportedResultTime
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
