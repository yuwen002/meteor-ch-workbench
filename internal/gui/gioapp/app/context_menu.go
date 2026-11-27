package app

import (
	"fmt"
	"io"
	"strings"

	"gioui.org/io/clipboard"
	"gioui.org/io/event"
	"gioui.org/io/pointer"
	"gioui.org/layout"
	"gioui.org/op/clip"
	"gioui.org/unit"
	"gioui.org/widget"
	"gioui.org/widget/material"
)

type rightClickTag struct{}

type contextMenuState struct {
	open     bool
	kind     string
	database string
	object   string
	row      int
	column   int
	clicks   [10]widget.Clickable
}

func (s *workspace) openContextMenu(kind, database, object string, row, column int) {
	s.contextMenu = contextMenuState{open: true, kind: kind, database: database, object: object, row: row, column: column}
}

func (s *workspace) watchSecondaryClick(gtx layout.Context, tag *rightClickTag, onClick func()) {
	// 将事件目标限制在当前控件的可见区域内，避免右键命中整块父容器。
	area := clip.Rect{Max: gtx.Constraints.Max}.Push(gtx.Ops)
	event.Op(gtx.Ops, tag)
	area.Pop()
	for {
		ev, ok := gtx.Event(pointer.Filter{Target: tag, Kinds: pointer.Press})
		if !ok {
			break
		}
		pe, ok := ev.(pointer.Event)
		if ok && pe.Buttons.Contain(pointer.ButtonSecondary) {
			onClick()
			if s.window != nil { s.window.Invalidate() }
		}
	}
}

type contextAction struct {
	label string
	run   func(layout.Context)
}

func (s *workspace) contextActions() []contextAction {
	menu := s.contextMenu
	switch menu.kind {
	case "database":
		return []contextAction{
			{"复制数据库名", func(gtx layout.Context) { s.copyText(gtx, menu.database) }},
			{"创建表 SQL", func(gtx layout.Context) { s.queryEditor.SetText("CREATE TABLE IF NOT EXISTS `" + strings.ReplaceAll(menu.database, "`", "``") + "`.`table_name` (\n    id UInt64\n) ENGINE = MergeTree ORDER BY id;"); s.contextMenu.open = false }},
			{"刷新数据库对象", func(gtx layout.Context) { s.selectDatabase(menu.database); s.contextMenu.open = false }},
		}
	case "object":
		qualified := "`" + strings.ReplaceAll(menu.database, "`", "``") + "`.`" + strings.ReplaceAll(menu.object, "`", "``") + "`"
		return []contextAction{
			{"预览对象", func(gtx layout.Context) { s.explorerSelection.SetSelectedObject(menu.database + "." + menu.object); s.loadTableSchema(menu.object); s.contextMenu.open = false }},
			{"生成 SELECT", func(gtx layout.Context) { s.queryEditor.SetText("SELECT *\nFROM " + qualified + "\nLIMIT 100;"); s.contextMenu.open = false }},
			{"查看 DDL", func(gtx layout.Context) { s.queryEditor.SetText("SHOW CREATE TABLE " + qualified + ";"); s.contextMenu.open = false }},
			{"生成 ALTER 模板", func(gtx layout.Context) { s.queryEditor.SetText("-- 请在此处填写 ALTER TABLE 变更\nALTER TABLE " + qualified + " ADD COLUMN column_name String;"); s.contextMenu.open = false }},
			{"生成 DROP SQL", func(gtx layout.Context) { s.queryEditor.SetText("-- 请确认对象后再执行删除\nDROP TABLE IF EXISTS " + qualified + ";"); s.contextMenu.open = false }},
			{"复制完整名称", func(gtx layout.Context) { s.copyText(gtx, menu.database+"."+menu.object); s.contextMenu.open = false }},
		}
	case "cell":
		return []contextAction{
			{"复制单元格", func(gtx layout.Context) { s.copyText(gtx, s.contextCellValue()) }},
			{"复制 SQL 字面量", func(gtx layout.Context) { s.copyText(gtx, sqlLiteral(s.contextCellRawValue())) }},
			{"复制整行", func(gtx layout.Context) { s.copyText(gtx, s.contextRowTSV()) }},
			{"复制 INSERT", func(gtx layout.Context) { s.copyText(gtx, s.contextInsertSQL(false)) }},
			{"复制全部 INSERT", func(gtx layout.Context) { s.copyText(gtx, s.contextInsertSQL(true)) }},
			{"按值筛选", func(gtx layout.Context) { if menu.column >= 0 && menu.column < len(s.columnFilterEditors) { s.columnFilterEditors[menu.column].SetText(s.contextCellValue()); s.resultViewDirty = true }; s.contextMenu.open = false }},
			{"升序", func(gtx layout.Context) { s.setResultSort(menu.column, false); s.contextMenu.open = false }},
			{"降序", func(gtx layout.Context) { s.setResultSort(menu.column, true); s.contextMenu.open = false }},
		}
	case "header":
		name := ""
		if menu.column >= 0 && menu.column < len(s.queryColumns) { name = s.queryColumns[menu.column].Name }
		return []contextAction{
			{"复制列名", func(gtx layout.Context) { s.copyText(gtx, name) }},
			{"复制全部列名", func(gtx layout.Context) { names := make([]string, len(s.queryColumns)); for i,c := range s.queryColumns { names[i] = c.Name }; s.copyText(gtx, strings.Join(names, "\t")) }},
			{"复制列值", func(gtx layout.Context) { s.copyText(gtx, s.contextColumnValues(menu.column, false)) }},
			{"复制去重列值", func(gtx layout.Context) { s.copyText(gtx, s.contextColumnValues(menu.column, true)) }},
			{"仅搜索此列", func(gtx layout.Context) {
				keyword := strings.TrimSpace(s.resultFilter.Text())
				if keyword == "" {
					s.queryStatus = "请先输入全局关键字，再选择仅搜索此列"
				} else if menu.column >= 0 && menu.column < len(s.columnFilterEditors) {
					s.columnFilterEditors[menu.column].SetText(keyword)
					s.columnFilterLast[menu.column] = keyword
					s.resultFilter.SetText("")
					s.resultFilterLast = ""
					s.resultViewDirty = true
					s.queryRows.Position = layout.Position{}
				}
				s.contextMenu.open = false
			}},
			{"升序", func(gtx layout.Context) { s.setResultSort(menu.column, false); s.contextMenu.open = false }},
			{"降序", func(gtx layout.Context) { s.setResultSort(menu.column, true); s.contextMenu.open = false }},
			{"隐藏列", func(gtx layout.Context) { if len(s.hiddenResultColumns) < len(s.queryColumns)-1 { s.hiddenResultColumns[menu.column] = true }; s.contextMenu.open = false }},
			{"显示全部列", func(gtx layout.Context) { clear(s.hiddenResultColumns); s.contextMenu.open = false }},
		}
	default:
		return nil
	}
}

func (s *workspace) layoutContextMenu(gtx layout.Context, scope string) layout.Dimensions {
	if !s.contextMenu.open { return layout.Dimensions{} }
	if scope == "results" && s.contextMenu.kind != "cell" && s.contextMenu.kind != "header" { return layout.Dimensions{} }
	if scope == "navigator" && s.contextMenu.kind != "database" && s.contextMenu.kind != "object" { return layout.Dimensions{} }
	actions := s.contextActions()
	return layout.Inset{Top: unit.Dp(4), Bottom: unit.Dp(4), Left: unit.Dp(8), Right: unit.Dp(8)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return styledText(s.theme, "右键操作", unit.Sp(10), colorAccent)(gtx)
			}),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				children := make([]layout.FlexChild, 0, len(actions)*2+1)
				for i, action := range actions {
					index, current := i, action
					children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						if s.contextMenu.clicks[index].Clicked(gtx) { current.run(gtx) }
						button := material.Button(s.theme, &s.contextMenu.clicks[index], current.label)
						button.TextSize = unit.Sp(10)
						return layout.Inset{Right: unit.Dp(4), Bottom: unit.Dp(3)}.Layout(gtx, button.Layout)
					}))
				}
				children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					close := &s.contextMenu.clicks[9]
					if close.Clicked(gtx) { s.contextMenu.open = false }
					button := material.Button(s.theme, close, "关闭")
					button.TextSize = unit.Sp(10)
					return button.Layout(gtx)
				}))
				return layout.Flex{Axis: layout.Vertical}.Layout(gtx, children...)
			}),
		)
	})
}

func (s *workspace) copyText(gtx layout.Context, value string) {
	gtx.Execute(clipboard.WriteCmd{Type: "application/text", Data: io.NopCloser(strings.NewReader(value))})
	s.queryStatus = "已请求复制到剪贴板"
}

func (s *workspace) contextCellRawValue() any {
	menu := s.contextMenu
	if menu.row < 0 || menu.row >= len(s.resultView) { return nil }
	rowIndex := s.resultView[menu.row].Index
	if rowIndex < 0 || rowIndex >= len(s.queryData) || menu.column < 0 || menu.column >= len(s.queryData[rowIndex]) { return nil }
	return s.queryData[rowIndex][menu.column]
}

func (s *workspace) contextCellValue() string {
	return resultCellRawValue(s.contextCellRawValue())
}

func (s *workspace) contextRowTSV() string {
	menu := s.contextMenu
	if menu.row < 0 || menu.row >= len(s.resultView) { return "" }
	rowIndex := s.resultView[menu.row].Index
	if rowIndex < 0 || rowIndex >= len(s.queryData) { return "" }
	values := make([]string, len(s.queryColumns))
	for i := range values { if i < len(s.queryData[rowIndex]) { values[i] = resultCellRawValue(s.queryData[rowIndex][i]) } }
	return strings.Join(values, "\t")
}

func (s *workspace) contextInsertSQL(all bool) string {
	menu := s.contextMenu
	if menu.row < 0 || menu.row >= len(s.resultView) { return "" }
	start, end := menu.row, menu.row+1
	if all { start, end = 0, len(s.resultView) }
	columns := make([]string, len(s.queryColumns))
	for i, col := range s.queryColumns { columns[i] = "`" + strings.ReplaceAll(col.Name, "`", "``") + "`" }
	var b strings.Builder
	fmt.Fprintf(&b, "INSERT INTO `table_name` (%s) VALUES\n", strings.Join(columns, ", "))
	wroteRow := false
	for i := start; i < end; i++ {
		rowIndex := s.resultView[i].Index
		if rowIndex < 0 || rowIndex >= len(s.queryData) { continue }
		values := make([]string, len(s.queryColumns))
		for col := range values { if col < len(s.queryData[rowIndex]) { values[col] = sqlLiteral(s.queryData[rowIndex][col]) } else { values[col] = "NULL" } }
		if wroteRow { b.WriteString(",\n") }
		b.WriteString("(" + strings.Join(values, ", ") + ")")
		wroteRow = true
	}
	b.WriteString(";")
	return b.String()
}

func (s *workspace) contextColumnValues(column int, distinct bool) string {
	if column < 0 || column >= len(s.queryColumns) { return "" }
	values := make([]string, 0, len(s.resultView))
	seen := make(map[string]struct{})
	for _, item := range s.resultView {
		if item.Index < 0 || item.Index >= len(s.queryData) || column >= len(s.queryData[item.Index]) { continue }
		value := resultCellRawValue(s.queryData[item.Index][column])
		if distinct { if _, ok := seen[value]; ok { continue }; seen[value] = struct{}{} }
		values = append(values, value)
	}
	return strings.Join(values, "\n")
}

func sqlLiteral(value any) string {
	if value == nil { return "NULL" }
	switch v := value.(type) {
	case string: return "'" + strings.ReplaceAll(v, "'", "''") + "'"
	case []byte: return "'" + strings.ReplaceAll(string(v), "'", "''") + "'"
	case bool: if v { return "1" }; return "0"
	default: return fmt.Sprint(v)
	}
}

func (s *workspace) setResultSort(column int, descending bool) {
	if column < 0 || column >= len(s.queryColumns) { return }
	s.resultSortColumn, s.resultSortDescending = column, descending
	s.resultViewDirty = true
	s.queryRows.Position = layout.Position{}
}

func pruneRightClickTags(tags map[int64]*rightClickTag, position layout.Position, rowCount, overscan int) {
	if overscan < 0 { overscan = 0 }
	first := position.First - overscan
	if first < 0 { first = 0 }
	last := position.First + position.Count + overscan
	if last > rowCount { last = rowCount }
	for key := range tags {
		row := int(uint32(uint64(key) >> 32))
		if row < first || row >= last { delete(tags, key) }
	}
}
