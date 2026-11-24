package app

import "image/color"

// 轻量数据库 IDE 的统一视觉令牌。颜色集中管理，后续深色主题只需替换此调色板。
var (
	colorAppBackground = color.NRGBA{R: 242, G: 245, B: 249, A: 255}
	colorSurface = color.NRGBA{R: 255, G: 255, B: 255, A: 255}
	colorSurfaceSubtle = color.NRGBA{R: 248, G: 250, B: 252, A: 255}
	colorSurfaceSelected = color.NRGBA{R: 232, G: 240, B: 254, A: 255}
	colorBorder = color.NRGBA{R: 224, G: 230, B: 238, A: 255}
	colorTextPrimary = color.NRGBA{R: 31, G: 41, B: 55, A: 255}
	colorTextSecondary = color.NRGBA{R: 107, G: 119, B: 137, A: 255}
	colorAccent = color.NRGBA{R: 55, G: 93, B: 190, A: 255}
	colorAccentSoft = color.NRGBA{R: 235, G: 241, B: 253, A: 255}
)
