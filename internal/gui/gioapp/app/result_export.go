package app

import (
	"encoding/csv"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	gioexplorer "gioui.org/x/explorer"

	queryapp "github.com/yuwen002/meteor-ch-workbench/internal/application/query"
)

type exportResult struct {
	format             string
	rows               int64
	bytes              int64
	duration           time.Duration
	err                error
	canceled           bool
	cleanupUnsupported bool
	cleanupErr         error
}

// startExport 将当前展示的结果（含筛选与排序）写入用户选择的文件。
// 新版服务容器未提供流式重查导出，因此导出范围以当前已加载结果为上限。
func (s *workspace) startExport(format string) {
	if s.exportRunning {
		return
	}
	if s.queryRunning {
		s.queryStatus = "查询执行中，不能导出结果"
		return
	}
	if s.explorer == nil {
		s.queryStatus = "当前平台不支持文件保存对话框"
		return
	}
	s.refreshResultView()
	if len(s.queryColumns) == 0 || len(s.resultView) == 0 {
		s.queryStatus = "当前结果为空，无法导出"
		return
	}
	columns := append([]queryapp.Column(nil), s.queryColumns...)
	rows := append([]resultRowView(nil), s.resultView...)
	sourceRows := append([][]any(nil), s.queryData...)
	s.exportRunning = true
	s.queryStatus = "正在导出当前结果…"

	go func() {
		file, err := s.explorer.CreateFile("query-result." + format)
		if err != nil {
			s.exportResults <- exportResult{format: format, err: err, canceled: errors.Is(err, gioexplorer.ErrUserDecline)}
			s.window.Invalidate()
			return
		}
		startedAt := time.Now()
		writer := &countingWriter{writer: file}
		writeErr := writeResultExport(writer, columns, rows, sourceRows, format)
		closeErr := file.Close()
		if writeErr != nil {
			err = writeErr
		} else {
			err = closeErr
		}
		result := exportResult{
			format:   format,
			rows:     int64(len(rows)),
			bytes:    writer.bytes,
			duration: time.Since(startedAt),
			err:      err,
			canceled: errors.Is(err, os.ErrClosed) || errors.Is(err, io.ErrClosedPipe),
		}
		if err != nil {
			supported, removeErr := removeIncompleteExport(file)
			result.cleanupUnsupported = !supported
			result.cleanupErr = removeErr
		}
		select {
		case s.exportResults <- result:
		default:
		}
		s.window.Invalidate()
	}()
}

func writeResultExport(w io.Writer, columns []queryapp.Column, rows []resultRowView, sourceRows [][]any, format string) error {
	writer := csv.NewWriter(w)
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
		cells := make([]string, len(columns))
		var values []any
		if row.Index >= 0 && row.Index < len(sourceRows) {
			values = sourceRows[row.Index]
		}
		for i := range columns {
			if i < len(values) {
				cells[i] = resultCellRawValue(values[i])
			} else {
				cells[i] = "NULL"
			}
		}
		if err := writer.Write(cells); err != nil {
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
			switch {
			case result.canceled:
				s.queryStatus = fmt.Sprintf("已取消导出（已写入 %s）", formatExportSize(result.bytes))
			case result.err != nil:
				s.queryStatus = fmt.Sprintf("导出中断：%s（已写入 %s）", result.err.Error(), formatExportSize(result.bytes))
			default:
				s.queryStatus = fmt.Sprintf(
					"已导出 %d 行 %s（%s，耗时 %s，平均 %.1f 行/秒；当前已加载结果）",
					result.rows, strings.ToUpper(result.format), formatExportSize(result.bytes),
					result.duration.Round(time.Millisecond), exportRowsPerSecond(result.rows, result.duration),
				)
			}
			if result.err != nil {
				if result.cleanupErr != nil {
					s.queryStatus += "；删除不完整文件失败：" + result.cleanupErr.Error()
				} else if result.cleanupUnsupported {
					s.queryStatus += "；当前平台无法自动删除不完整文件，请手动检查目标文件"
				}
			}
		default:
			return
		}
	}
}

// removeIncompleteExport 仅在句柄是本地 *os.File 时删除中断导出文件。
func removeIncompleteExport(file io.WriteCloser) (bool, error) {
	localFile, ok := file.(*os.File)
	if !ok {
		return false, nil
	}
	err := os.Remove(localFile.Name())
	if errors.Is(err, os.ErrNotExist) {
		return true, nil
	}
	return true, err
}

type countingWriter struct {
	writer io.Writer
	bytes  int64
}

func (w *countingWriter) Write(p []byte) (int, error) {
	n, err := w.writer.Write(p)
	w.bytes += int64(n)
	return n, err
}

func formatExportSize(size int64) string {
	const mib = 1024 * 1024
	if size >= mib {
		return fmt.Sprintf("%.2f MiB", float64(size)/mib)
	}
	return fmt.Sprintf("%.1f KiB", float64(size)/1024)
}

func exportRowsPerSecond(rows int64, duration time.Duration) float64 {
	seconds := duration.Seconds()
	if seconds <= 0 {
		return 0
	}
	return float64(rows) / seconds
}
