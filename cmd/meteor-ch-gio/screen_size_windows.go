//go:build windows

package main

import (
	"syscall"
	"unsafe"

	"gioui.org/unit"
)

const (
	spiGetWorkArea = 0x0030
	smCXScreen     = 0
	smCYScreen     = 1
)

type screenRect struct {
	left, top, right, bottom int32
}

var (
	user32                    = syscall.NewLazyDLL("user32.dll")
	procSystemParametersInfoW = user32.NewProc("SystemParametersInfoW")
	procGetSystemMetrics      = user32.NewProc("GetSystemMetrics")
	procGetDpiForSystem       = user32.NewProc("GetDpiForSystem")
)

// initialWindowSize 获取 Windows 主显示器工作区，并按系统 DPI 换算为 Gio 逻辑尺寸。
// SPI_GETWORKAREA 会扣除任务栏等系统区域；API 不可用时退回屏幕分辨率。
func initialWindowSize() (unit.Dp, unit.Dp) {
	var area screenRect
	ok, _, _ := procSystemParametersInfoW.Call(
		spiGetWorkArea,
		0,
		uintptr(unsafe.Pointer(&area)),
		0,
	)
	width := int(area.right - area.left)
	height := int(area.bottom - area.top)
	if ok == 0 || width <= 0 || height <= 0 {
		w, _, _ := procGetSystemMetrics.Call(smCXScreen)
		h, _, _ := procGetSystemMetrics.Call(smCYScreen)
		width, height = int(w), int(h)
	}
	if width <= 0 || height <= 0 {
		return fitInitialWindowSize(1080, 660)
	}

	dpi, _, _ := procGetDpiForSystem.Call()
	if dpi == 0 {
		dpi = 96
	}
	scale := float64(dpi) / 96
	logicalWidth := int(float64(width) / scale)
	logicalHeight := int(float64(height) / scale)
	return fitInitialWindowSize(logicalWidth, logicalHeight)
}
