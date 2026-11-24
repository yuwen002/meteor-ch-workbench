package connection

import (
	"image"
	"image/color"

	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/unit"
	"gioui.org/widget"
	"gioui.org/widget/material"
)

// 侧栏独立的配色令牌，与主界面调色板保持同一套灰蓝基调。
var (
	colorTextPrimary   = color.NRGBA{R: 28, G: 38, B: 52, A: 255}
	colorTextSecondary = color.NRGBA{R: 106, G: 119, B: 138, A: 255}
	colorTextMuted     = color.NRGBA{R: 154, G: 165, B: 181, A: 255}
	colorBorder        = color.NRGBA{R: 226, G: 232, B: 240, A: 255}
	colorRowHover      = color.NRGBA{R: 247, G: 249, B: 252, A: 255}
	colorRowSelected   = color.NRGBA{R: 236, G: 242, B: 254, A: 255}
	colorAccentBorder  = color.NRGBA{R: 190, G: 209, B: 245, A: 255}
	colorAccent        = color.NRGBA{R: 45, G: 88, B: 190, A: 255}
	colorSuccess       = color.NRGBA{R: 22, G: 138, B: 94, A: 255}
	colorWarning       = color.NRGBA{R: 183, G: 118, B: 20, A: 255}
	colorDanger        = color.NRGBA{R: 197, G: 60, B: 60, A: 255}
)

// fillRoundRect 在给定尺寸上填充圆角矩形，不改变布局约束。
func fillRoundRect(gtx layout.Context, size image.Point, background color.NRGBA) {
	if size.X <= 0 || size.Y <= 0 {
		return
	}
	r := gtx.Dp(unit.Dp(4))
	rr := clip.RRect{Rect: image.Rectangle{Max: size}, NE: r, NW: r, SE: r, SW: r}
	paint.FillShape(gtx.Ops, background, rr.Op(gtx.Ops))
}

// cardSurface 绘制连接条目的卡片背景：选中时使用浅蓝底 + 蓝色描边，未选中为透明。
func cardSurface(gtx layout.Context, selected bool, child layout.Widget) layout.Dimensions {
	macro := op.Record(gtx.Ops)
	childGtx := gtx
	childGtx.Constraints.Min = image.Point{}
	dims := child(childGtx)
	call := macro.Stop()
	if selected {
		r := gtx.Dp(unit.Dp(8))
		rr := clip.RRect{Rect: image.Rectangle{Max: dims.Size}, NE: r, NW: r, SE: r, SW: r}
		paint.FillShape(gtx.Ops, colorRowSelected, rr.Op(gtx.Ops))
		stroke := clip.Stroke{
			Path: clip.RRect{
				Rect: image.Rectangle{Min: image.Pt(1, 1), Max: image.Pt(dims.Size.X-1, dims.Size.Y-1)},
				NE:   r, NW: r, SE: r, SW: r,
			}.Path(gtx.Ops),
			Width: float32(gtx.Dp(unit.Dp(1))),
		}.Op()
		paint.FillShape(gtx.Ops, colorAccentBorder, stroke)
	}
	call.Add(gtx.Ops)
	return dims
}

// compactButton 是标题栏用的低高度文字按钮：主操作填充蓝色，其余为浅灰底。
func compactButton(gtx layout.Context, theme *material.Theme, click *widget.Clickable, label string, primary bool) layout.Dimensions {
	macro := op.Record(gtx.Ops)
	dims := material.Clickable(gtx, click, func(gtx layout.Context) layout.Dimensions {
		text := material.Label(theme, unit.Sp(11), label)
		if primary {
			text.Color = color.NRGBA{R: 255, G: 255, B: 255, A: 255}
		} else {
			text.Color = colorTextSecondary
		}
		return layout.Inset{Left: unit.Dp(9), Right: unit.Dp(9), Top: unit.Dp(5), Bottom: unit.Dp(5)}.Layout(gtx, text.Layout)
	})
	call := macro.Stop()
	r := gtx.Dp(unit.Dp(6))
	rr := clip.RRect{Rect: image.Rectangle{Max: dims.Size}, NE: r, NW: r, SE: r, SW: r}
	if primary {
		paint.FillShape(gtx.Ops, colorAccent, rr.Op(gtx.Ops))
	} else {
		paint.FillShape(gtx.Ops, colorRowHover, rr.Op(gtx.Ops))
	}
	call.Add(gtx.Ops)
	return dims
}

// ghostButton 是条目内的弱化按钮文字，展开 / 收起操作使用。点击状态由调用方在绘制按钮前读取。
func ghostButton(gtx layout.Context, theme *material.Theme, click *widget.Clickable, label string) layout.Dimensions {
	text := material.Label(theme, unit.Sp(10), label)
	text.Color = colorTextMuted
	return material.Clickable(gtx, click, text.Layout)
}

// stateBadge 绘制连接状态徽标：小圆点 + 文字，比纯文字更容易扫读。
func stateBadge(gtx layout.Context, theme *material.Theme, label string, dot color.NRGBA) layout.Dimensions {
	return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			size := image.Pt(gtx.Dp(unit.Dp(6)), gtx.Dp(unit.Dp(6)))
			fillRoundRect(gtx, size, dot)
			return layout.Dimensions{Size: size}
		}),
		layout.Rigid(layout.Spacer{Width: unit.Dp(4)}.Layout),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			badge := material.Label(theme, unit.Sp(10), label)
			badge.Color = dot
			return badge.Layout(gtx)
		}),
	)
}

// statusLine 是加载 / 错误提示的统一行样式。
func statusLine(gtx layout.Context, theme *material.Theme, foreground color.NRGBA, text string) layout.Dimensions {
	label := material.Label(theme, unit.Sp(11), text)
	label.Color = foreground
	return layout.Inset{Bottom: unit.Dp(6)}.Layout(gtx, label.Layout)
}
