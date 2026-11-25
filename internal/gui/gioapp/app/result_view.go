package app

import (
	"encoding/csv"
	"fmt"
	"io"
	"sort"
	"strconv"
	"strings"

	"gioui.org/gesture"
	"gioui.org/io/pointer"
	"gioui.org/layout"
	"gioui.org/unit"
	"gioui.org/widget"

	queryapp "github.com/yuwen002/meteor-ch-workbench/internal/application/query"
)

type resultRowView struct {
	Index  int
	Values []any
}

type exportResult struct {
	format string
	err    error
}

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
		matched := true
		for columnIndex, editor := range s.columnFilterEditors {
			if columnIndex >= len(row) || !matchesResultColumnFilter(row[columnIndex], editor.Text()) {
				if strings.TrimSpace(editor.Text()) != "" {
					matched = false
					break
				}
			}
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
	s.resultCellText = make(map[int64]string)
	s.queryCellClicks = make(map[int64]*widget.Clickable)
	s.resultViewDirty = false
	s.queryRows.Position = layout.Position{}
}

func resultCellKey(row, column int) int64 {
	return int64(uint64(uint32(row))<<32 | uint64(uint32(column)))
}

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
		if filter == "" || i >= len(columns) {
			continue
		}
		summaries = append(summaries, columns[i].Name+": "+filter)
	}
	return summaries
}

func matchesResultColumnFilter(value any, expression string) bool {
	expression = strings.TrimSpace(expression)
	if expression == "" {
		return true
	}
	actual := resultCellRawValue(value)
	lowerActual := strings.ToLower(actual)
	lowerExpr := strings.ToLower(expression)
	if lowerExpr == "is:null" {
		return value == nil
	}
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
	}
	return strings.Compare(strings.ToLower(leftText), strings.ToLower(rightText))
}

func (s *workspace) layoutResultColumnResizeHandle(gtx layout.Context, index int) layout.Dimensions {
	if index < 0 || index >= len(s.columnDrags) {
		return layout.Dimensions{}
	}
	drag := &s.columnDrags[index]
	size := gtx.Constraints.Max
	if size.Y <= 0 {
		size.Y = gtx.Dp(unit.Dp(28))
	}
	size.X = gtx.Dp(unit.Dp(8))
	gtx.Constraints.Min = size
	gtx.Constraints.Max = size
	pointer.CursorColResize.Add(gtx.Ops)
	defer func() {
		drag.Add(gtx.Ops)
	}()
	if event, ok := drag.Update(gtx.Metric, gtx.Source, gesture.Horizontal); ok && event.Kind == pointer.Drag {
		x := float32(event.Position.X)
		if s.columnDragActive[index] {
			delta := x - s.columnDragLastX[index]
			width := s.columnWidths[index] + unit.Dp(delta/gtx.Metric.PxPerDp)
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
	}
	return layout.Dimensions{Size: size}
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
	for _, row := range rows {
		record := make([]string, len(columns))
		for i := range columns {
			if i < len(row) {
				record[i] = resultCellRawValue(row[i])
			}
		}
		if err := writer.Write(record); err != nil {
			return err
		}
	}
	writer.Flush()
	return writer.Error()
}

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
