package app

import (
	"image"

	"gioui.org/gesture"
	"gioui.org/io/pointer"
	"gioui.org/layout"
	"gioui.org/op/clip"
	"gioui.org/unit"
	"gioui.org/widget"
	"gioui.org/widget/material"

	queryapp "github.com/yuwen002/meteor-ch-workbench/internal/application/query"
)

const (
	minResultColumnWidth          = unit.Dp(112)
	maxResultColumnWidth          = unit.Dp(320)
	resultColumnResizeHandleWidth = unit.Dp(8)
)

// resultCellKey 用整数键代替 fmt.Sprintf，减少每个可见单元格的字符串分配。
func resultCellKey(row, column int) int64 {
	return int64(uint32(row))<<32 | int64(uint32(column))
}

// defaultResultColumnWidth 把采样权重换算成 dp，并保持当前表格的视觉宽度范围。
func defaultResultColumnWidth(weight float32) unit.Dp {
	width := unit.Dp(weight * 9)
	if width < minResultColumnWidth {
		return minResultColumnWidth
	}
	if width > maxResultColumnWidth {
		return maxResultColumnWidth
	}
	return width
}

func adjustResultColumnWidth(current unit.Dp, delta unit.Dp) unit.Dp {
	width := current + delta
	if width < minResultColumnWidth {
		return minResultColumnWidth
	}
	if width > maxResultColumnWidth {
		return maxResultColumnWidth
	}
	return width
}

// resetResultWidgets 在列定义变化时重建每列筛选、排序与拖动状态。
func (s *workspace) resetResultWidgets(columns []queryapp.Column) {
	s.resultSortColumn = -1
	s.resultSortDescending = false
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
	weights := resultColumnFlexWeights(columns, s.queryData, 40)
	for i := range columns {
		width := defaultResultColumnWidth(weights[i])
		s.columnWidths[i] = width
	}
	s.resultView = nil
	s.resultViewDirty = true
	s.resultSortKeyCache.Reset()
	s.queryRows.Position = layout.Position{}
	s.releaseResultCellCaches()
}

// releaseResultCellCaches 释放与当前结果视图绑定的临时缓存，避免关闭/切换标签后保留大 map。
func (s *workspace) releaseResultCellCaches() {
	s.queryCellClicks = nil
	s.queryCellRightClicks = nil
	s.resultCellText = nil
	s.resultHeaderChildren = nil
	s.resultRowChildren = nil
	s.resultPixelWidths = nil
}

// pruneResultCellCaches 保留当前可见行及少量邻近行的单元格缓存。
func pruneResultCellCaches(clicks map[int64]*widget.Clickable, texts map[int64]string, position layout.Position, rowCount, overscan int) {
	if overscan < 0 {
		overscan = 0
	}
	first := position.First - overscan
	if first < 0 {
		first = 0
	}
	last := position.First + position.Count + overscan
	if last > rowCount {
		last = rowCount
	}
	if last < first {
		last = first
	}
	for key := range clicks {
		row := int(uint32(uint64(key) >> 32))
		if row < first || row >= last {
			delete(clicks, key)
		}
	}
	for key := range texts {
		row := int(uint32(uint64(key) >> 32))
		if row < first || row >= last {
			delete(texts, key)
		}
	}
}

// syncResultFilters 检测筛选条件变化，变化时标记展示行需要重建并回到顶部。
func (s *workspace) syncResultFilters() {
	changed := false
	if text := s.resultFilter.Text(); text != s.resultFilterLast {
		s.resultFilterLast = text
		changed = true
	}
	for i := range s.columnFilterEditors {
		if i >= len(s.columnFilterLast) {
			s.columnFilterLast = append(s.columnFilterLast, "")
		}
		text := s.columnFilterEditors[i].Text()
		if text != s.columnFilterLast[i] {
			s.columnFilterLast[i] = text
			changed = true
		}
	}
	if changed {
		s.resultViewDirty = true
		s.queryRows.Position = layout.Position{}
	}
}

// refreshResultView 按需重建筛选/排序后的展示行，避免每帧重复排序全部结果。
func (s *workspace) refreshResultView() {
	if len(s.columnWidths) != len(s.queryColumns) {
		s.resetResultWidgets(s.queryColumns)
	}
	s.syncResultFilters()
	if !s.resultViewDirty {
		return
	}
	// 展示行顺序变化后，旧缓存键不再对应同一单元格，直接复用 map 内存清空。
	clear(s.queryCellClicks)
	clear(s.resultCellText)
	filters := make([]string, len(s.columnFilterEditors))
	for i := range s.columnFilterEditors {
		filters[i] = s.columnFilterEditors[i].Text()
	}
	sortKeys := s.resultSortKeyCache.Prepare(s.queryData, s.resultSortColumn)
	s.resultView = buildResultRowViewWithPreparedSortKeysInto(s.resultView[:0], s.queryData, s.resultFilter.Text(), filters, s.resultSortColumn, s.resultSortDescending, sortKeys)
	if len(s.resultView) == 0 {
		s.queryRows.Position = layout.Position{}
	} else if s.queryRows.Position.First >= len(s.resultView) {
		s.queryRows.Position.First = len(s.resultView) - 1
	}
	s.resultViewDirty = false
}

// layoutResultColumnResizeHandle 在表头右侧提供可拖动列宽的手柄。
func (s *workspace) layoutResultColumnResizeHandle(gtx layout.Context, columnIndex int) layout.Dimensions {
	handleWidth := gtx.Dp(resultColumnResizeHandleWidth)
	gtx.Constraints.Min.X, gtx.Constraints.Max.X = handleWidth, handleWidth
	area := clip.Rect(image.Rect(0, 0, handleWidth, gtx.Constraints.Max.Y)).Push(gtx.Ops)
	pointer.CursorColResize.Add(gtx.Ops)
	s.columnDrags[columnIndex].Add(gtx.Ops)
	area.Pop()

	for {
		event, ok := s.columnDrags[columnIndex].Update(gtx.Metric, gtx.Source, gesture.Horizontal)
		if !ok {
			break
		}
		if event.Kind != pointer.Drag {
			continue
		}
		if !s.columnDragActive[columnIndex] {
			s.columnDragLastX[columnIndex] = event.Position.X
			s.columnDragActive[columnIndex] = true
			continue
		}
		deltaPx := int(event.Position.X - s.columnDragLastX[columnIndex])
		deltaDp := unit.Dp(gtx.Metric.PxToDp(deltaPx))
		s.columnWidths[columnIndex] = adjustResultColumnWidth(s.columnWidths[columnIndex], deltaDp)
		s.columnDragLastX[columnIndex] = event.Position.X
	}
	if !s.columnDrags[columnIndex].Pressed() {
		s.columnDragActive[columnIndex] = false
	}

	return layout.Center.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		label := material.Caption(s.theme, "⋮")
		label.Color = colorTextMuted
		return label.Layout(gtx)
	})
}
