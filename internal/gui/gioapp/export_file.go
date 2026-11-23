package gioapp

import (
	"errors"
	"fmt"
	"io"
	"os"
	"time"
)

// removeIncompleteExport 仅在保存句柄是本地 *os.File 时删除中断导出文件。
// Explorer 在部分平台返回非本地文件句柄，无法通过 Go os.Remove 安全删除。
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


// countingWriter 统计成功写入底层目标的字节数，适用于本地与系统文件选择器句柄。
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
