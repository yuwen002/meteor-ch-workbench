package connection

import (
	"fmt"
	"image"
	"image/color"
	"strconv"
	"strings"

	"gioui.org/layout"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/unit"
	"gioui.org/widget"
	"gioui.org/widget/material"

	domain "github.com/yuwen002/meteor-ch-workbench/internal/domain/connection"
)

// View 是连接侧栏的 Gio 展示组件。数据操作由 Model / 应用服务负责，
// 组件只处理点击并通过回调通知上层窗口。
type View struct {
	model     *Model
	list      widget.List
	refresh   widget.Clickable
	rows      map[string]*widget.Clickable
	onRefresh func()
	onSelect  func(domain.Connection)
	onSave func(domain.Connection)
	add widget.Clickable
	save widget.Clickable
	cancel widget.Clickable
	protocolToggle widget.Clickable
	secureToggle widget.Clickable
	name widget.Editor
	host widget.Editor
	port widget.Editor
	user widget.Editor
	password widget.Editor
	database widget.Editor
	protocol domain.Protocol
	secure bool
	formOpen bool
	formError string
}

func NewView(model *Model, onRefresh func(), onSelect func(domain.Connection)) (*View, error) {
	if model == nil {
		return nil, fmt.Errorf("连接状态模型不能为空")
	}
	view := &View{
		model: model,
		list: widget.List{List: layout.List{Axis: layout.Vertical}},
		rows: make(map[string]*widget.Clickable),
		onRefresh: onRefresh,
		onSelect: onSelect,
		protocol: domain.ProtocolNative,
	}
	view.port.SetText("9000")
	view.user.SetText("default")
	view.name.SingleLine = true
	view.host.SingleLine = true
	view.port.SingleLine = true
	view.user.SingleLine = true
	view.password.SingleLine = true
	view.password.Mask = '*'
	view.database.SingleLine = true
	return view, nil
}


func (v *View) SetOnSave(onSave func(domain.Connection)) {
	v.onSave = onSave
}

func (v *View) layoutEditor(gtx layout.Context, theme *material.Theme) layout.Dimensions {
	field := func(editor *widget.Editor, hint string) layout.Widget {
		return func(gtx layout.Context) layout.Dimensions {
			return layout.Inset{Bottom: unit.Dp(9)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				return material.Editor(theme, editor, hint).Layout(gtx)
			})
		}
	}
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(material.H6(theme, "新增连接").Layout),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return layout.Inset{Top: unit.Dp(4), Bottom: unit.Dp(16)}.Layout(gtx, material.Caption(theme, "填写 ClickHouse 服务地址和认证信息").Layout)
		}),
		layout.Rigid(field(&v.name, "连接名称")),
		layout.Rigid(field(&v.host, "主机地址，例如 127.0.0.1")),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{Axis: layout.Horizontal}.Layout(gtx,
				layout.Flexed(1, field(&v.port, "端口（默认 9000）")),
				layout.Rigid(layout.Spacer{Width: unit.Dp(12)}.Layout),
				layout.Flexed(1, field(&v.database, "默认数据库（可选）")),
			)
		}),
		layout.Rigid(field(&v.user, "用户名")),
		layout.Rigid(field(&v.password, "密码")),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
				layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
					if v.protocolToggle.Clicked(gtx) {
						if v.protocol == domain.ProtocolNative { v.protocol = domain.ProtocolHTTP } else { v.protocol = domain.ProtocolNative }
					}
					button := material.Button(theme, &v.protocolToggle, "协议："+string(v.protocol))
					button.TextSize = unit.Sp(13)
					return button.Layout(gtx)
				}),
				layout.Rigid(layout.Spacer{Width: unit.Dp(12)}.Layout),
				layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
					if v.secureToggle.Clicked(gtx) { v.secure = !v.secure }
					label := "TLS：关闭"
					if v.secure { label = "TLS：开启" }
					button := material.Button(theme, &v.secureToggle, label)
					button.TextSize = unit.Sp(13)
					return button.Layout(gtx)
				}),
			)
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			if v.formError == "" { return layout.Dimensions{} }
			return layout.Inset{Top: unit.Dp(8)}.Layout(gtx, material.Caption(theme, v.formError).Layout)
		}),
		layout.Rigid(layout.Spacer{Height: unit.Dp(14)}.Layout),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
				layout.Flexed(1, layout.Spacer{Width: unit.Dp(0)}.Layout),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					button := material.Button(theme, &v.cancel, "取消")
					button.TextSize = unit.Sp(13)
					if v.cancel.Clicked(gtx) { v.formOpen = false; v.formError = "" }
					return layout.Inset{Right: unit.Dp(8)}.Layout(gtx, button.Layout)
				}),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					button := material.Button(theme, &v.save, "保存连接")
					button.TextSize = unit.Sp(13)
					if v.save.Clicked(gtx) {
						port, err := strconv.Atoi(strings.TrimSpace(v.port.Text()))
						if err != nil {
							v.formError = "端口必须是有效数字"
						} else if v.onSave == nil {
							v.formError = "连接保存服务尚未初始化"
						} else {
							v.formError = ""
							v.onSave(domain.Connection{
								Name: strings.TrimSpace(v.name.Text()), Host: strings.TrimSpace(v.host.Text()), Port: port,
								Protocol: v.protocol, User: strings.TrimSpace(v.user.Text()), Password: v.password.Text(),
								Database: strings.TrimSpace(v.database.Text()), Secure: v.secure,
							})
							v.formOpen = false
						}
					}
					return button.Layout(gtx)
				}),
			)
		}),
	)
}

// Layout 绘制连接列表、刷新入口和当前加载 / 错误状态。
func (v *View) Layout(gtx layout.Context, theme *material.Theme) layout.Dimensions {
	snapshot := v.model.Snapshot()
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return layout.Inset{Bottom: unit.Dp(8)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
					layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
						return material.Subtitle1(theme, "连接").Layout(gtx)
					}),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						button := material.Button(theme, &v.refresh, "刷新")
						addButton := material.Button(theme, &v.add, "新增连接")
						button.TextSize = unit.Sp(12)
						if v.refresh.Clicked(gtx) && v.onRefresh != nil {
							v.onRefresh()
						}
						if v.add.Clicked(gtx) {
							v.resetForm()
							v.formOpen = true
						}
						return layout.Flex{Axis: layout.Horizontal}.Layout(gtx,
							layout.Rigid(button.Layout),
							layout.Rigid(addButton.Layout),
						)
					}),
				)
			})
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			if snapshot.Loading {
				return material.Caption(theme, "正在加载连接…").Layout(gtx)
			}
			if snapshot.Error != "" {
				return material.Caption(theme, snapshot.Error).Layout(gtx)
			}
			return layout.Dimensions{}
		}),
		layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
			return material.List(theme, &v.list).Layout(gtx, len(snapshot.Items), func(gtx layout.Context, index int) layout.Dimensions {
				item := snapshot.Items[index]
				click := v.rows[item.ID]
				if click == nil {
					click = new(widget.Clickable)
					v.rows[item.ID] = click
				}
				if click.Clicked(gtx) {
					v.model.Select(item.ID)
					if v.onSelect != nil {
						v.onSelect(item)
					}
				}
				return layout.Inset{Top: unit.Dp(2), Bottom: unit.Dp(2), Left: unit.Dp(4), Right: unit.Dp(4)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
					return click.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
						label := item.Name
						if item.ID == snapshot.SelectedID {
							label = "● " + label
						}
						return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
							layout.Rigid(material.Body1(theme, label).Layout),
							layout.Rigid(material.Caption(theme, fmt.Sprintf("%s:%d · %s", item.Host, item.Port, item.Protocol)).Layout),
						)
					})
				})
			})
		}),
	)
}

func (v *View) resetForm() {
 v.name.SetText("")
 v.host.SetText("")
 v.port.SetText("9000")
 v.user.SetText("default")
 v.password.SetText("")
 v.database.SetText("")
 v.protocol = domain.ProtocolNative
 v.secure = false
 v.formError = ""
}

// Modal 在窗口中央绘制连接表单，不再占用左侧连接列表空间。
func (v *View) Modal(gtx layout.Context, theme *material.Theme) layout.Dimensions {
 if !v.formOpen { return layout.Dimensions{} }
 return layout.Stack{Alignment: layout.Center}.Layout(gtx,
  layout.Expanded(func(gtx layout.Context) layout.Dimensions {
   paint.FillShape(gtx.Ops, color.NRGBA{R:20,G:25,B:35,A:105}, clip.Rect{Max:gtx.Constraints.Max}.Op())
   return layout.Dimensions{Size:gtx.Constraints.Max}
  }),
  layout.Stacked(func(gtx layout.Context) layout.Dimensions {
   width := gtx.Dp(unit.Dp(460))
   height := gtx.Dp(unit.Dp(570))
   if width > gtx.Constraints.Max.X-gtx.Dp(unit.Dp(32)) { width = gtx.Constraints.Max.X-gtx.Dp(unit.Dp(32)) }
   if height > gtx.Constraints.Max.Y-gtx.Dp(unit.Dp(32)) { height = gtx.Constraints.Max.Y-gtx.Dp(unit.Dp(32)) }
   if width < 0 { width = 0 }; if height < 0 { height = 0 }
   gtx.Constraints = layout.Exact(image.Pt(width,height))
   paint.FillShape(gtx.Ops, color.NRGBA{R:250,G:251,B:253,A:255}, clip.Rect{Max:gtx.Constraints.Max}.Op())
   return layout.Inset{Top:unit.Dp(22),Bottom:unit.Dp(18),Left:unit.Dp(24),Right:unit.Dp(24)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
    return v.layoutEditor(gtx,theme)
   })
  }),
 )
}
