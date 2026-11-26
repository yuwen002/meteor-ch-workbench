package app

import "image/color"

// 轻量数据库 IDE 的统一视觉令牌。颜色集中管理，后续深色主题只需替换此调色板。
var (
	colorAppBackground   = color.NRGBA{R: 242, G: 245, B: 249, A: 255}
	colorSurface         = color.NRGBA{R: 255, G: 255, B: 255, A: 255}
	colorSurfaceSubtle   = color.NRGBA{R: 248, G: 250, B: 252, A: 255}
	colorSurfaceSelected = color.NRGBA{R: 232, G: 240, B: 254, A: 255}
	colorBorder          = color.NRGBA{R: 224, G: 230, B: 238, A: 255}
	colorTextPrimary     = color.NRGBA{R: 31, G: 41, B: 55, A: 255}
	colorTextSecondary   = color.NRGBA{R: 107, G: 119, B: 137, A: 255}
	colorTextMuted       = color.NRGBA{R: 154, G: 165, B: 181, A: 255}
	colorAccent          = color.NRGBA{R: 55, G: 93, B: 190, A: 255}
	colorAccentSoft      = color.NRGBA{R: 235, G: 241, B: 253, A: 255}
	colorSuccess         = color.NRGBA{R: 22, G: 138, B: 94, A: 255}
	colorWarning         = color.NRGBA{R: 183, G: 118, B: 20, A: 255}
)

// setPalette 将统一视觉令牌切换为对应主题，所有面板和自绘控件共用此调色板。
func setPalette(mode string) {
	if mode == "dark" {
		colorAppBackground = color.NRGBA{R: 22, G: 27, B: 35, A: 255}
		colorSurface = color.NRGBA{R: 30, G: 37, B: 48, A: 255}
		colorSurfaceSubtle = color.NRGBA{R: 38, G: 46, B: 59, A: 255}
		colorSurfaceSelected = color.NRGBA{R: 48, G: 67, B: 99, A: 255}
		colorBorder = color.NRGBA{R: 58, G: 68, B: 83, A: 255}
		colorTextPrimary = color.NRGBA{R: 229, G: 234, B: 242, A: 255}
		colorTextSecondary = color.NRGBA{R: 174, G: 185, B: 200, A: 255}
		colorTextMuted = color.NRGBA{R: 128, G: 141, B: 159, A: 255}
		colorAccent = color.NRGBA{R: 105, G: 145, B: 230, A: 255}
		colorAccentSoft = color.NRGBA{R: 42, G: 57, B: 82, A: 255}
		colorSuccess = color.NRGBA{R: 74, G: 190, B: 143, A: 255}
		colorWarning = color.NRGBA{R: 232, G: 174, B: 84, A: 255}
		return
	}
	colorAppBackground = color.NRGBA{R: 242, G: 245, B: 249, A: 255}
	colorSurface = color.NRGBA{R: 255, G: 255, B: 255, A: 255}
	colorSurfaceSubtle = color.NRGBA{R: 248, G: 250, B: 252, A: 255}
	colorSurfaceSelected = color.NRGBA{R: 232, G: 240, B: 254, A: 255}
	colorBorder = color.NRGBA{R: 224, G: 230, B: 238, A: 255}
	colorTextPrimary = color.NRGBA{R: 31, G: 41, B: 55, A: 255}
	colorTextSecondary = color.NRGBA{R: 107, G: 119, B: 137, A: 255}
	colorTextMuted = color.NRGBA{R: 154, G: 165, B: 181, A: 255}
	colorAccent = color.NRGBA{R: 55, G: 93, B: 190, A: 255}
	colorAccentSoft = color.NRGBA{R: 235, G: 241, B: 253, A: 255}
	colorSuccess = color.NRGBA{R: 22, G: 138, B: 94, A: 255}
	colorWarning = color.NRGBA{R: 183, G: 118, B: 20, A: 255}
}
