// Package platform 定义桌面应用需要的操作系统能力端口。
package platform

import "io"

// FileFilter 描述文件对话框中的文件类型筛选条件。
type FileFilter struct {
	Name    string
	Pattern string
}

// SaveFileRequest 描述保存文件对话框的展示参数。
type SaveFileRequest struct {
	Title      string
	FileName   string
	ButtonText string
	Filters    []FileFilter
}

// OpenFileRequest 描述打开文件对话框的展示参数。
type OpenFileRequest struct {
	Title   string
	Filters []FileFilter
}

// Platform 封装 GUI 适配层需要的平台能力。
// 业务服务不应直接依赖 Wails、Gio 或操作系统命令。
type Platform interface {
	SaveFile(request SaveFileRequest) (string, error)
	OpenFile(request OpenFileRequest) (string, error)
	OpenDirectory(path string) error
}

// FileSystem 封装应用需要的本地文件读写能力。
// 将其与对话框接口分离，便于 Gio 等其他 GUI 入口复用并独立测试。
type FileSystem interface {
	ReadFile(path string) ([]byte, error)
	WriteFile(path string, content []byte, perm io.FileMode) error
	Create(path string) (io.WriteCloser, error)
}
