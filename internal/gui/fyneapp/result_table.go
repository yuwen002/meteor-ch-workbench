package fyneapp

import (
	"fmt"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/widget"

	"github.com/yuwen002/meteor-ch-workbench/internal/clickhouse"
)

type ResultTable struct {
	widget.BaseWidget
	table      *widget.Table
	result     *clickhouse.QueryResult
	onCellEdit func(col, row int)
	onRightClick func(col, row int, pos fyne.Position)

	searchText   string
	filteredRows []int

	lastCol int
	lastRow int
}

func NewResultTable() *ResultTable {
	rt := &ResultTable{}
	rt.lastCol = -1
	rt.lastRow = -1

	rt.table = widget.NewTableWithHeaders(
		func() (int, int) {
			if rt.result == nil || len(rt.result.Columns) == 0 {
				return 0, 0
			}
			rows := len(rt.result.Rows)
			if rt.searchText != "" && rt.filteredRows != nil {
				rows = len(rt.filteredRows)
			}
			return rows, len(rt.result.Columns)
		},
		func() fyne.CanvasObject {
			return widget.NewLabel("")
		},
		func(id widget.TableCellID, o fyne.CanvasObject) {
			cell := o.(*widget.Label)
			if rt.result == nil {
				cell.SetText("")
				return
			}
			row := id.Row
			col := id.Col

			actualRow := row
			if rt.searchText != "" && rt.filteredRows != nil {
				if row < 0 || row >= len(rt.filteredRows) {
					cell.SetText("")
					return
				}
				actualRow = rt.filteredRows[row]
			}

			if actualRow >= 0 && actualRow < len(rt.result.Rows) && col >= 0 && col < len(rt.result.Rows[actualRow]) {
				cell.SetText(cellToString(rt.result.Rows[actualRow][col]))
			} else {
				cell.SetText("")
			}
		},
	)

	rt.table.CreateHeader = func() fyne.CanvasObject {
		l := widget.NewLabel("")
		l.Importance = widget.HighImportance
		return l
	}
	rt.table.UpdateHeader = func(id widget.TableCellID, o fyne.CanvasObject) {
		if rt.result == nil {
			o.(*widget.Label).SetText("")
			return
		}
		if id.Row == -1 {
			col := id.Col
			if col >= 0 && col < len(rt.result.Columns) {
				colInfo := rt.result.Columns[col]
				text := colInfo.Name
				if colInfo.Type != "" {
					text += " (" + colInfo.Type + ")"
				}
				o.(*widget.Label).SetText(text)
			}
		} else if id.Col == -1 {
			labelText := fmt.Sprintf("%d", id.Row+1)
			if rt.searchText != "" && rt.filteredRows != nil && id.Row >= 0 && id.Row < len(rt.filteredRows) {
				labelText = fmt.Sprintf("%d", rt.filteredRows[id.Row]+1)
			}
			o.(*widget.Label).SetText(labelText)
		}
	}

	rt.table.ShowHeaderRow = true
	rt.table.ShowHeaderColumn = true
	rt.table.HideSeparators = false

	rt.ExtendBaseWidget(rt)
	return rt
}

func (rt *ResultTable) Widget() fyne.CanvasObject { return rt }

func (rt *ResultTable) CreateRenderer() fyne.WidgetRenderer {
	return rt.table.CreateRenderer()
}

func (rt *ResultTable) MinSize() fyne.Size {
	return rt.table.MinSize()
}

func (rt *ResultTable) SetResult(result *clickhouse.QueryResult) {
	rt.result = result
	rt.filteredRows = nil
	if rt.searchText != "" && result != nil {
		rt.recomputeFilter()
	}
	if result != nil && len(result.Columns) > 0 {
		rt.setColumnWidths(result)
	}
	rt.table.Refresh()
}

func (rt *ResultTable) Result() *clickhouse.QueryResult { return rt.result }

func (rt *ResultTable) Clear() {
	rt.result = nil
	rt.filteredRows = nil
	rt.lastCol = -1
	rt.lastRow = -1
	rt.table.Refresh()
}

func (rt *ResultTable) OnCellEdit(fn func(col, row int)) {
	rt.onCellEdit = fn
	rt.table.OnSelected = func(id widget.TableCellID) {
		if id.Row < 0 || id.Col < 0 {
			return
		}
		actualRow := id.Row
		if rt.searchText != "" && rt.filteredRows != nil {
			if id.Row >= len(rt.filteredRows) {
				return
			}
			actualRow = rt.filteredRows[id.Row]
		}
		rt.lastCol = id.Col
		rt.lastRow = actualRow
		if rt.onCellEdit != nil {
			rt.onCellEdit(id.Col, actualRow)
		}
	}
}

func (rt *ResultTable) SetOnRightClick(fn func(col, row int, pos fyne.Position)) {
	rt.onRightClick = fn
}

func (rt *ResultTable) TappedSecondary(e *fyne.PointEvent) {
	if rt.result == nil || rt.lastRow < 0 || rt.lastCol < 0 {
		return
	}
	if rt.lastRow >= len(rt.result.Rows) || rt.lastCol >= len(rt.result.Columns) {
		return
	}
	if rt.onRightClick != nil {
		rt.onRightClick(rt.lastCol, rt.lastRow, e.Position)
	}
}

func (rt *ResultTable) SetSearch(text string) {
	rt.searchText = strings.TrimSpace(text)
	rt.recomputeFilter()
	rt.table.Refresh()
}

func (rt *ResultTable) SearchText() string { return rt.searchText }

func (rt *ResultTable) FilteredRowCount() int {
	if rt.searchText == "" || rt.filteredRows == nil {
		if rt.result != nil {
			return len(rt.result.Rows)
		}
		return 0
	}
	return len(rt.filteredRows)
}

func (rt *ResultTable) recomputeFilter() {
	if rt.searchText == "" || rt.result == nil {
		rt.filteredRows = nil
		return
	}
	needle := strings.ToLower(rt.searchText)
	var matches []int
	for ri, row := range rt.result.Rows {
		for _, cell := range row {
			if strings.Contains(strings.ToLower(cellToString(cell)), needle) {
				matches = append(matches, ri)
				break
			}
		}
	}
	rt.filteredRows = matches
}

func (rt *ResultTable) setColumnWidths(result *clickhouse.QueryResult) {
	sampleSize := len(result.Rows)
	if sampleSize > 200 {
		sampleSize = 200
	}

	for ci, col := range result.Columns {
		maxLen := float32(len(col.Name))
		for ri := 0; ri < sampleSize; ri++ {
			if ri < len(result.Rows) && ci < len(result.Rows[ri]) {
				cellStr := cellToString(result.Rows[ri][ci])
				if float32(len(cellStr)) > maxLen {
					maxLen = float32(len(cellStr))
				}
			}
		}
		width := maxLen * 8.0
		if width < 80 {
			width = 80
		}
		if width > 420 {
			width = 420
		}
		rt.table.SetColumnWidth(ci, width)
	}
}

func cellToString(v interface{}) string {
	if v == nil {
		return ""
	}
	switch val := v.(type) {
	case string:
		return val
	case []byte:
		return string(val)
	default:
		s := fmt.Sprintf("%v", val)
		if idx := strings.Index(s, "{"); idx >= 0 && len(s) > idx+10 {
			return s[:idx+10] + "..."
		}
		return s
	}
}

func sqlLiteral(v interface{}) string {
	s := cellToString(v)
	if v == nil || s == "" {
		return "NULL"
	}
	if _, ok := v.(string); ok {
		return "'" + strings.ReplaceAll(s, "'", "''") + "'"
	}
	if _, ok := v.([]byte); ok {
		return "'" + strings.ReplaceAll(s, "'", "''") + "'"
	}
	switch val := v.(type) {
	case int, int32, int64, float32, float64, bool:
		return fmt.Sprintf("%v", val)
	default:
		return "'" + strings.ReplaceAll(s, "'", "''") + "'"
	}
}

func buildInsertSQL(columns []clickhouse.ColumnInfo, row []interface{}, tableName string) string {
	if tableName == "" {
		tableName = "your_table"
	}
	var colNames []string
	var values []string
	for ci := range columns {
		colNames = append(colNames, "`"+columns[ci].Name+"`")
		if ci < len(row) {
			values = append(values, sqlLiteral(row[ci]))
		} else {
			values = append(values, "NULL")
		}
	}
	return fmt.Sprintf("INSERT INTO %s (%s) VALUES (%s)", tableName, strings.Join(colNames, ", "), strings.Join(values, ", "))
}

func rowToTSV(row []interface{}) string {
	var parts []string
	for _, v := range row {
		s := cellToString(v)
		s = strings.ReplaceAll(s, "\t", "\\t")
		s = strings.ReplaceAll(s, "\n", "\\n")
		parts = append(parts, s)
	}
	return strings.Join(parts, "\t")
}