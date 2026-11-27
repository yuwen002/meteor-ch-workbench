package fyneapp

import (
	"fmt"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/widget"

	"github.com/yuwen002/meteor-ch-workbench/internal/clickhouse"
)

type ResultExporter struct {
	window fyne.Window
}

func NewResultExporter(w fyne.Window) *ResultExporter {
	return &ResultExporter{window: w}
}

func (re *ResultExporter) Export(result *clickhouse.QueryResult) {
	if result == nil || len(result.Columns) == 0 {
		dialog.ShowInformation("提示", "没有可导出的结果集", re.window)
		return
	}

	formats := Backend().GetResultExportFormats()
	if len(formats) == 0 {
		dialog.ShowInformation("提示", "未找到可用的导出格式", re.window)
		return
	}

	var formatNames []string
	for _, f := range formats {
		formatNames = append(formatNames, fmt.Sprintf("%s (%s)", f.Name, f.Extension))
	}

	formatSelect := widget.NewSelect(formatNames, func(_ string) {})
	formatSelect.Selected = formatNames[0]

	cols := make([]string, len(result.Columns))
	for i, c := range result.Columns {
		cols[i] = c.Name
	}
	rows := make([][]interface{}, len(result.Rows))
	copy(rows, result.Rows)

	exportBtn := widget.NewButton("导出", func() {
		idx := 0
		for i, name := range formatNames {
			if name == formatSelect.Selected {
				idx = i
				break
			}
		}
		format := formats[idx]

		dialog.ShowFileSave(func(writer fyne.URIWriteCloser, err error) {
			if err != nil {
				dialog.ShowError(fmt.Errorf("打开文件失败: %w", err), re.window)
				return
			}
			if writer == nil {
				return
			}
			defer writer.Close()

			_, err = clickhouse.ExportResultTo(writer, format.ID, cols, rows, "")
			if err != nil {
				dialog.ShowError(fmt.Errorf("导出失败: %w", err), re.window)
				return
			}
			dialog.ShowInformation("导出成功",
				fmt.Sprintf("已导出 %d 行到 %s", len(rows), writer.URI().Name()),
				re.window)
		}, re.window)
	})
	exportBtn.Importance = widget.HighImportance

	content := container.NewVBox(
		widget.NewForm(
			&widget.FormItem{Text: "导出格式", Widget: formatSelect},
		),
		container.NewHBox(exportBtn),
	)

	custom := dialog.NewCustom("导出结果集", "", content, re.window)
	custom.Resize(fyne.NewSize(400, 180))
	custom.Show()
}
