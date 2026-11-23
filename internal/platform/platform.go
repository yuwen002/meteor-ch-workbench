// Package platform 定义桌面应用需要的操作系统能力端口。
package platform

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
