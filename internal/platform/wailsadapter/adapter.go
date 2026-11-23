// Package wailsadapter 将平台能力端口适配到 Wails v3。
package wailsadapter

import (
	"os/exec"
	"runtime"
	"strings"

	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/yuwen002/meteor-ch-workbench/internal/platform"
)

// Adapter 提供基于 Wails 系统对话框和操作系统命令的平台实现。
type Adapter struct{}

// New 创建 Wails 平台适配器。
func New() *Adapter { return &Adapter{} }

// SaveFile 打开系统保存文件对话框。用户取消时返回空路径且不返回错误。
func (a *Adapter) SaveFile(request platform.SaveFileRequest) (string, error) {
	dialog := application.Get().Dialog.SaveFile()
	dialog.SetMessage(request.Title)
	dialog.SetFilename(request.FileName)
	if request.ButtonText != "" {
		dialog.SetButtonText(request.ButtonText)
	}
	for _, filter := range request.Filters {
		dialog.AddFilter(filter.Name, filter.Pattern)
	}
	path, err := dialog.PromptForSingleSelection()
	if err != nil {
		if strings.Contains(err.Error(), "cancelled") {
			return "", nil
		}
		return "", err
	}
	return path, nil
}

// OpenFile 打开系统文件选择对话框。用户取消时返回空路径且不返回错误。
func (a *Adapter) OpenFile(request platform.OpenFileRequest) (string, error) {
	dialog := application.Get().Dialog.OpenFile()
	dialog.SetMessage(request.Title)
	for _, filter := range request.Filters {
		dialog.AddFilter(filter.Name, filter.Pattern)
	}
	path, err := dialog.PromptForSingleSelection()
	if err != nil {
		if strings.Contains(err.Error(), "cancelled") {
			return "", nil
		}
		return "", err
	}
	return path, nil
}

// OpenDirectory 使用当前操作系统的默认文件管理器打开目录。
func (a *Adapter) OpenDirectory(path string) error {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "windows":
		cmd = exec.Command("explorer", path)
	case "darwin":
		cmd = exec.Command("open", path)
	default:
		cmd = exec.Command("xdg-open", path)
	}
	return cmd.Start()
}
