// Package osfs 提供基于 Go 标准库的本地文件系统实现。
package osfs

import (
	"io"
	"io/fs"
	"os"

	"github.com/yuwen002/meteor-ch-workbench/internal/platform"
)

// FileSystem 将本地文件操作适配到 platform.FileSystem。
type FileSystem struct{}

// 确保标准库实现完整满足文件系统端口。
var _ platform.FileSystem = (*FileSystem)(nil)

// New 创建操作系统文件系统适配器。
func New() *FileSystem { return &FileSystem{} }

// ReadFile 读取指定文件的全部内容。
func (f *FileSystem) ReadFile(path string) ([]byte, error) {
	return os.ReadFile(path)
}

// WriteFile 按指定权限创建或覆盖文件。
func (f *FileSystem) WriteFile(path string, content []byte, perm fs.FileMode) error {
	return os.WriteFile(path, content, perm)
}

// Create 创建或截断文件，并返回可关闭的写入句柄。
func (f *FileSystem) Create(path string) (io.WriteCloser, error) {
	return os.Create(path)
}
