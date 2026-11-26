package main

import "gioui.org/unit"

// fitInitialWindowSize 根据屏幕可用工作区计算初始窗口逻辑尺寸。
// 目标占用约 82% 的工作区，同时限制极端宽屏尺寸，并确保小屏不溢出。
func fitInitialWindowSize(availableWidth, availableHeight int) (unit.Dp, unit.Dp) {
	if availableWidth < 1 {
		availableWidth = 1080
	}
	if availableHeight < 1 {
		availableHeight = 660
	}

	width := availableWidth * 82 / 100
	height := availableHeight * 82 / 100
	if width < 760 {
		width = 760
	}
	if height < 480 {
		height = 480
	}
	if width > 1440 {
		width = 1440
	}
	if height > 1000 {
		height = 1000
	}

	// 留出至少 5% 的边缘空间；低分辨率屏幕优先保证窗口完整可见。
	maxWidth := availableWidth * 95 / 100
	maxHeight := availableHeight * 95 / 100
	if width > maxWidth {
		width = maxWidth
	}
	if height > maxHeight {
		height = maxHeight
	}
	if width < 1 {
		width = 1
	}
	if height < 1 {
		height = 1
	}
	return unit.Dp(width), unit.Dp(height)
}
