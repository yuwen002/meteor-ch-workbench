package gioapp

import (
	"errors"
	"io"
	"os"
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
