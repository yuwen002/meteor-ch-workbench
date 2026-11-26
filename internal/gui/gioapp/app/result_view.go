package app

import (
	"encoding/csv"
	"fmt"
	"math/big"
	"image"
	"io"
	"sort"
	"strconv"
	"strings"
	"time"

	"gioui.org/gesture"
	"gioui.org/io/pointer"
	"gioui.org/op/clip"
	"gioui.org/layout"
	"gioui.org/unit"
	"gioui.org/widget"

	queryapp "github.com/yuwen002/meteor-ch-workbench/internal/application/query"
)

type resultRowView struct {
	Index  int
	Values []any
}

// buildResultRowView 仅构造 UI 展示顺序，不修改查询返回的原始结果。
func buildResultRowView(rows [][]any, filter string, sortColumn int, descending bool) []resultRowView {
	return buildResultRowViewWithColumnFilters(rows, filter, nil, sortColumn, descending)
type exportResult struct {
	format string
	err    error
}

// buildResultRowViewWithColumnFilters 将全局关键字和各列条件按 AND 组合。
func buildResultRowViewWithColumnFilters(rows [][]any, filter string, columnFilters []string, sortColumn int, descending bool) []resultRowView {
	filter = strings.TrimSpace(filter)
	view := make([]resultRowView, 0, len(rows))
	for i, row := range rows {
		if filter != "" && !resultRowMatchesFilter(row, filter) {
func (s *workspace) resetResultWidgets(columns []queryapp.Column) {
	s.resultView = nil
	s.resultViewDirty = true
	s.resultCellText = make(map[int64]string)
	s.queryCellClicks = make(map[int64]*widget.Clickable)
	s.resultHeaderChildren = nil
	s.resultRowChildren = nil
	s.resultPixelWidths = nil
	s.resultFilter.SetText("")
	s.resultFilterLast = ""
	s.columnFilterEditors = make([]widget.Editor, len(columns))
	s.columnFilterLast = make([]string, len(columns))
	s.columnFilterClears = make([]widget.Clickable, len(columns))
	s.sortClicks = make([]widget.Clickable, len(columns))
	s.columnWidths = make([]unit.Dp, len(columns))
	s.columnDrags = make([]gesture.Drag, len(columns))
	s.columnDragLastX = make([]float32, len(columns))
	s.columnDragActive = make([]bool, len(columns))
	s.resultSortColumn = -1
	s.resultSortDescending = false
	s.queryRows.Position = layout.Position{}
	s.queryHorizontal.Position = layout.Position{}
	if len(columns) == 0 {
		return
	}
	weights := resultColumnFlexWeights(columns, nil, 0)
	for i, weight := range weights {
		width := unit.Dp(weight * 9)
		if width < 112 {
			width = 112
		}
		if width > 320 {
			width = 320
		}
		s.columnWidths[i] = width
	}
}

func (s *workspace) refreshResultView() {
	if s.resultFilter.Text() != s.resultFilterLast {
		s.resultFilterLast = s.resultFilter.Text()
		s.resultViewDirty = true
	}
	for i := range s.columnFilterEditors {
		value := s.columnFilterEditors[i].Text()
		if i >= len(s.columnFilterLast) {
			continue
		}
		if !resultRowMatchesColumnFilters(row, columnFilters) {
			continue
		if value != s.columnFilterLast[i] {
			s.columnFilterLast[i] = value
			s.resultViewDirty = true
		}
	}
	if !s.resultViewDirty {
		return
	}
	s.resultView = make([]resultRowView, 0, len(s.queryData))
	keyword := strings.ToLower(strings.TrimSpace(s.resultFilter.Text()))
	for rowIndex, row := range s.queryData {
		if keyword != "" {
			matched := false
			for _, value := range row {
				if strings.Contains(strings.ToLower(resultCellRawValue(value)), keyword) {
					matched = true
					break
				}
			}
			if !matched {
				continue
			}
		}
		view = append(view, resultRowView{Index: i, Values: row})
	}
	if sortColumn >= 0 {
		sort.SliceStable(view, func(i, j int) bool {
			var left, right any
			if sortColumn < len(view[i].Values) {
				left = view[i].Values[sortColumn]
		matched := true
		for columnIndex, editor := range s.columnFilterEditors {
			if columnIndex >= len(row) || !matchesResultColumnFilter(row[columnIndex], editor.Text()) {
				if strings.TrimSpace(editor.Text()) != "" {
					matched = false
					break
				}
			}
			if sortColumn < len(view[j].Values) {
				right = view[j].Values[sortColumn]
			}
			if left == nil && right != nil {
				return false
			}
			if left != nil && right == nil {
				return true
		}
		if matched {
			values := append([]any(nil), row...)
			s.resultView = append(s.resultView, resultRowView{Index: rowIndex, Values: values})
		}
	}
	if s.resultSortColumn >= 0 && s.resultSortColumn < len(s.queryColumns) {
		column := s.resultSortColumn
		sort.SliceStable(s.resultView, func(i, j int) bool {
			left, right := any(nil), any(nil)
			if column < len(s.resultView[i].Values) {
				left = s.resultView[i].Values[column]
			}
			if left == nil && right == nil {
				return false
			if column < len(s.resultView[j].Values) {
				right = s.resultView[j].Values[column]
			}
			cmp := compareResultValues(left, right)
			if s.resultSortDescending {
				return cmp > 0
			}
			return cmp < 0
		})
	}
	return view
	s.resultCellText = make(map[int64]string)
	s.queryCellClicks = make(map[int64]*widget.Clickable)
	s.resultViewDirty = false
	s.queryRows.Position = layout.Position{}
}

func resultCellKey(row, column int) int64 {
	return int64(uint64(uint32(row))<<32 | uint64(uint32(column)))
}

func resultRowMatchesColumnFilters(row []any, filters []string) bool {
	for column, filter := range filters {
func resultColumnLabel(columns []queryapp.Column, index, sortColumn int, descending bool) string {
	if index < 0 || index >= len(columns) {
		return ""
	}
	label := columns[index].Name
	if index == sortColumn {
		if descending {
			label += " ↓"
		} else {
			label += " ↑"
		}
	}
	return label
}

func resultColumnFilterSummary(columns []queryapp.Column, filters []string) []string {
	var summaries []string
	for i, filter := range filters {
		filter = strings.TrimSpace(filter)
		if filter == "" {
		if filter == "" || i >= len(columns) {
			continue
		}
		if column >= len(row) || !resultValueMatchesColumnCondition(row[column], filter) {
			return false
		}
	}
	return true
		summaries = append(summaries, columns[i].Name+": "+filter)
	}
	return summaries
}

// resultValueMatchesColumnCondition 支持包含、!不包含、=、!=、>、<、>=、<=、区间和 NULL 判断。
func resultValueMatchesColumnCondition(value any, expression string) bool {
func matchesResultColumnFilter(value any, expression string) bool {
	expression = strings.TrimSpace(expression)
	if expression == "" {
		return true
	}
	if strings.EqualFold(expression, "is:null") {
	actual := resultCellRawValue(value)
	lowerActual := strings.ToLower(actual)
	lowerExpr := strings.ToLower(expression)
	if lowerExpr == "is:null" {
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
	if strings.HasPrefix(lowerExpr, "between:") {
		bounds := strings.SplitN(expression[len("between:"):], "..", 2)
		if len(bounds) != 2 {
			return false
		}
		v, vok := parseResultNumber(actual)
		lo, lok := parseResultNumber(strings.TrimSpace(bounds[0]))
		hi, hik := parseResultNumber(strings.TrimSpace(bounds[1]))
		return vok && lok && hik && v >= lo && v <= hi
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
	for _, operator := range []string{"!=", ">=", "<=", "=", ">", "<"} {
		if !strings.HasPrefix(expression, operator) {
			continue
		}
		expected := strings.TrimSpace(expression[len(operator):])
		cmp := compareResultValues(value, expected)
		switch operator {
		case "=":
			return cmp == 0
		case "!=":
			return cmp != 0
		case ">":
			return cmp > 0
		case "<":
			return cmp < 0
		case ">=":
			return cmp >= 0
		case "<=":
			return cmp <= 0
		}
	}
	if strings.HasPrefix(lowerExpr, "!") {
		return !strings.Contains(lowerActual, strings.TrimSpace(lowerExpr[1:]))
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
	if strings.HasPrefix(lowerExpr, "不包含") {
		return !strings.Contains(lowerActual, strings.TrimSpace(lowerExpr[len("不包含"):]))
	}
	if strings.HasPrefix(lowerExpr, "包含") {
		return strings.Contains(lowerActual, strings.TrimSpace(lowerExpr[len("包含"):]))
	}
	return strings.Contains(lowerActual, lowerExpr)
}

func parseResultNumber(value string) (float64, bool) {
	number, err := strconv.ParseFloat(strings.TrimSpace(value), 64)
	return number, err == nil
}

func compareResultValues(left, right any) int {
	if left == nil && right == nil {
	leftNull, rightNull := left == nil, right == nil
	if leftNull && rightNull {
		return 0
	}
	if left == nil {
		return -1
	}
	if right == nil {
		return 1
	}
	leftText, rightText := resultCellRawValue(left), resultCellRawValue(right)
	leftNumber, leftOK := parseResultNumber(leftText)
	rightNumber, rightOK := parseResultNumber(rightText)
	if leftOK && rightOK {
		switch {
		case leftNumber < rightNumber:
			return -1
		case leftNumber > rightNumber:
			return 1
		default:
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
	return strings.Compare(strings.ToLower(leftText), strings.ToLower(rightText))
}

func (s *workspace) layoutResultColumnResizeHandle(gtx layout.Context, index int) layout.Dimensions {
	if index < 0 || index >= len(s.columnDrags) {
		return layout.Dimensions{}
// compareResultNumbers 使用有理数比较，避免 UInt64 和高精度 Decimal 转 float64 后丢失精度。
func compareResultNumbers(left, right string) (int, bool) {
	leftNumber, ok := new(big.Rat).SetString(strings.TrimSpace(left))
	if !ok {
		return 0, false
	}
	drag := &s.columnDrags[index]
	size := image.Pt(gtx.Dp(unit.Dp(8)), gtx.Dp(unit.Dp(36)))
	gtx.Constraints.Min = size
	gtx.Constraints.Max = size
	defer clip.Rect{Max: size}.Push(gtx.Ops).Pop()
	pointer.CursorColResize.Add(gtx.Ops)
	drag.Add(gtx.Ops)
	if event, ok := drag.Update(gtx.Metric, gtx.Source, gesture.Horizontal); ok && event.Kind == pointer.Drag {
		x := float32(event.Position.X)
		if s.columnDragActive[index] {
			delta := x - s.columnDragLastX[index]
			width := s.columnWidths[index] + gtx.Metric.PxToDp(int(delta))
			if width < 72 {
				width = 72
			}
			if width > 640 {
				width = 640
			}
			s.columnWidths[index] = width
		}
		s.columnDragLastX[index] = x
		s.columnDragActive[index] = true
	} else if !drag.Pressed() {
		s.columnDragActive[index] = false
	rightNumber, ok := new(big.Rat).SetString(strings.TrimSpace(right))
	if !ok {
		return 0, false
	}
	return leftNumber.Cmp(rightNumber), true
	return layout.Dimensions{Size: size}
}

func parseResultTime(value string) (time.Time, error) {
	layouts := []string{time.RFC3339Nano, "2006-01-02 15:04:05.999999999", "2006-01-02 15:04:05", "2006-01-02"}
	for _, layout := range layouts {
		if parsed, err := time.Parse(layout, value); err == nil {
			return parsed, nil
		}
func (s *workspace) startExport(format string) {
	if s.exportRunning || s.explorer == nil || len(s.queryColumns) == 0 {
		return
	}
	s.refreshResultView()
	columns := append([]queryapp.Column(nil), s.queryColumns...)
	rows := make([][]any, len(s.resultView))
	for i, row := range s.resultView {
		rows[i] = append([]any(nil), row.Values...)
	}
	return time.Time{}, fmt.Errorf("not a supported timestamp")
	s.exportRunning = true
	s.queryStatus = fmt.Sprintf("正在导出 %d 行…", len(rows))
	go func() {
		extension := ".csv"
		if format == "tsv" {
			extension = ".tsv"
		}
		writer, err := s.explorer.CreateFile("query-result" + extension)
		if err == nil {
			err = writeQueryExport(writer, columns, rows, format)
			closeErr := writer.Close()
			if err == nil {
				err = closeErr
			}
		}
		select {
		case s.exportResults <- exportResult{format: format, err: err}:
		default:
		}
		s.window.Invalidate()
	}()
}

// resultColumnLabel 显示当前列的排序方向。
func resultColumnLabel(columns []queryapp.Column, column, sortColumn int, descending bool) string {
	if column < 0 || column >= len(columns) {
		return ""
func writeQueryExport(destination io.Writer, columns []queryapp.Column, rows [][]any, format string) error {
	writer := csv.NewWriter(destination)
	if format == "tsv" {
		writer.Comma = '\t'
	}
	header := make([]string, len(columns))
	for i, column := range columns {
		header[i] = column.Name
	}
	if err := writer.Write(header); err != nil {
		return err
	}
	label := columns[column].Name
	if column == sortColumn {
		if descending {
			return label + " ↓"
	for _, row := range rows {
		record := make([]string, len(columns))
		for i := range columns {
			if i < len(row) {
				record[i] = resultCellRawValue(row[i])
			}
		}
		return label + " ↑"
		if err := writer.Write(record); err != nil {
			return err
		}
	}
	return label
	writer.Flush()
	return writer.Error()
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
func (s *workspace) drainExportResults() {
	for {
		select {
		case result := <-s.exportResults:
			s.exportRunning = false
			if result.err != nil {
				s.queryStatus = "导出失败：" + result.err.Error()
			} else if result.format == "tsv" {
				s.queryStatus = "查询结果已导出为 TSV"
			} else {
				s.queryStatus = "查询结果已导出为 CSV"
			}
		default:
			return
		}
	}
}
