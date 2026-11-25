package app

import (
	"image"
	"image/color"
	"strings"

	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/unit"
	"gioui.org/widget/material"
)

// 统一的间距刻度，所有留白都从这组令牌取值，避免各处手写 magic number。
var (
	spaceXS = unit.Dp(4)
	spaceS  = unit.Dp(8)
	spaceM  = unit.Dp(12)
	spaceL  = unit.Dp(16)
	spaceXL = unit.Dp(24)

	radiusS = unit.Dp(6)
	radiusM = unit.Dp(10)
	radiusL = unit.Dp(14)

	compactBreakpoint = unit.Dp(1100)
)

// isCompact 统一窄屏判定，避免各布局各自使用不同阈值。
func isCompact(gtx layout.Context) bool {
	return gtx.Constraints.Max.X < gtx.Dp(compactBreakpoint)
}

// adaptiveMaxLines 在窄屏允许两行展示，宽屏保持单行以免破坏表格密度。
func adaptiveMaxLines(gtx layout.Context) int {
	if isCompact(gtx) {
		return 2
	}
	return 1
}

// statusForeground 让失败与中断类消息在状态区更醒目。
func statusForeground(message string) color.NRGBA {
	for _, marker := range []string{"失败", "错误", "中断", "无法", "未连接"} {
		if strings.Contains(message, marker) {
			return colorWarning
		}
	}
	return colorTextMuted
}

// fillRounded 在给定尺寸上绘制圆角底色，不改变布局约束，可作为装饰层使用。
// 半径为 0 时退化为普通矩形，避免圆角路径在零半径下生成空区域。
func fillRounded(gtx layout.Context, size image.Point, radius unit.Dp, background color.NRGBA) {
	if size.X <= 0 || size.Y <= 0 {
		return
	}
	if radius <= 0 {
		paint.FillShape(gtx.Ops, background, clip.Rect{Max: size}.Op())
		return
	}
	r := gtx.Dp(radius)
	// 圆角半径不得超过短边的一半，否则 clip.RRect 会生成退化路径并裁掉大片区域。
	if limit := min(size.X, size.Y) / 2; r > limit {
		r = limit
	}
	rr := clip.RRect{Rect: image.Rectangle{Max: size}, NE: r, NW: r, SE: r, SW: r}
	paint.FillShape(gtx.Ops, background, rr.Op(gtx.Ops))
}

// surface 为任意子组件提供圆角底色。先录制子组件再按真实尺寸绘制底色，最后回放内容，
// 保证背景永远在内容之下且不吞掉主轴上多余的空间。
func (s *workspace) surface(gtx layout.Context, background color.NRGBA, radius unit.Dp, child layout.Widget) layout.Dimensions {
	macro := op.Record(gtx.Ops)
	dims := child(gtx)
	call := macro.Stop()
	fillRounded(gtx, dims.Size, radius, background)
	call.Add(gtx.Ops)
	return dims
}

// card 绘制圆角卡片并铺满父级给出的可用区域，用于欢迎页这类需要整块底色的场景。
func (s *workspace) card(gtx layout.Context, child layout.Widget) layout.Dimensions {
	src := gtx
	src.Constraints.Min = src.Constraints.Max
	return s.surface(src, colorSurface, radiusM, child)
}

// topSurface 是顶栏 / 状态栏专用底色：紧贴窗口上下边缘，因此使用直角矩形铺满整行。
func (s *workspace) topSurface(gtx layout.Context, background color.NRGBA, child layout.Widget) layout.Dimensions {
	macro := op.Record(gtx.Ops)
	childGtx := gtx
	childGtx.Constraints.Min = image.Point{}
	dims := child(childGtx)
	call := macro.Stop()
	size := image.Pt(gtx.Constraints.Max.X, dims.Size.Y)
	paint.FillShape(gtx.Ops, background, clip.Rect{Max: size}.Op())
	call.Add(gtx.Ops)
	return layout.Dimensions{Size: size}
}

// divider 绘制一条水平细分隔线。
func (s *workspace) divider(gtx layout.Context) layout.Dimensions {
	height := gtx.Dp(unit.Dp(1))
	size := image.Pt(gtx.Constraints.Max.X, height)
	paint.FillShape(gtx.Ops, colorBorder, clip.Rect{Max: size}.Op())
	return layout.Dimensions{Size: size}
}

// styledText 返回指定字号与颜色的文字布局，避免各处重复构造 material.Label。
func styledText(theme *material.Theme, text string, size unit.Sp, foreground color.NRGBA) layout.Widget {
	return func(gtx layout.Context) layout.Dimensions {
		label := material.Label(theme, size, text)
		label.Color = foreground
		return label.Layout(gtx)
	}
}

// statusPill 绘制带状态圆点的胶囊徽标，用于连接状态等场景。
func (s *workspace) statusPill(gtx layout.Context, dot color.NRGBA, text string, maxLines int) layout.Dimensions {
	return s.surface(gtx, colorSurfaceSubtle, unit.Dp(999), func(gtx layout.Context) layout.Dimensions {
		return layout.Inset{Top: spaceXS, Bottom: spaceXS, Left: spaceM, Right: spaceM}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					size := image.Pt(gtx.Dp(unit.Dp(8)), gtx.Dp(unit.Dp(8)))
					fillRounded(gtx, size, unit.Dp(4), dot)
					return layout.Dimensions{Size: size}
				}),
				layout.Rigid(layout.Spacer{Width: spaceS}.Layout),
				layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
					label := material.Label(s.theme, unit.Sp(12), text)
					label.Color = colorTextSecondary
					if maxLines > 0 {
						label.MaxLines = maxLines
					}
					return label.Layout(gtx)
				}),
			)
		})
	})
}
