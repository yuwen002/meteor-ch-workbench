package connection

import (
	"fmt"
	"strconv"
	"strings"

	"gioui.org/layout"
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
	view.database.SingleLine = true
	return view, nil
}


func (v *View) SetOnSave(onSave func(domain.Connection)) {
	v.onSave = onSave
}

func (v *View) layoutEditor(gtx layout.Context, theme *material.Theme) layout.Dimensions {
	return layout.Inset{Top: unit.Dp(8), Bottom: unit.Dp(8)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
			layout.Rigid(material.Subtitle2(theme, "新增连接").Layout),
			layout.Rigid(material.Editor(theme, &v.name, "连接名称").Layout),
			layout.Rigid(material.Editor(theme, &v.host, "主机地址").Layout),
			layout.Rigid(material.Editor(theme, &v.port, "端口（Native 默认 9000）").Layout),
			layout.Rigid(material.Editor(theme, &v.user, "用户名").Layout),
			layout.Rigid(material.Editor(theme, &v.password, "密码").Layout),
			layout.Rigid(material.Editor(theme, &v.database, "默认数据库（可选）").Layout),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				if v.protocolToggle.Clicked(gtx) {
					if v.protocol == domain.ProtocolNative {
						v.protocol = domain.ProtocolHTTP
					} else {
						v.protocol = domain.ProtocolNative
					}
				}
				return material.Button(theme, &v.protocolToggle, "协议："+string(v.protocol)).Layout(gtx)
			}),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				if v.secureToggle.Clicked(gtx) {
					v.secure = !v.secure
				}
				label := "TLS：关闭"
				if v.secure { label = "TLS：开启" }
				return material.Button(theme, &v.secureToggle, label).Layout(gtx)
			}),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				if v.formError != "" { return material.Caption(theme, v.formError).Layout(gtx) }
				return layout.Dimensions{}
			}),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return layout.Flex{Axis: layout.Horizontal}.Layout(gtx,
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						button := material.Button(theme, &v.save, "保存连接")
						if v.save.Clicked(gtx) {
							port, err := strconv.Atoi(strings.TrimSpace(v.port.Text()))
							if err != nil {
								v.formError = "端口必须是有效数字"
							} else if v.onSave == nil {
								v.formError = "连接保存服务尚未初始化"
							} else {
								v.formError = ""
								v.onSave(domain.Connection{
									Name: strings.TrimSpace(v.name.Text()),
									Host: strings.TrimSpace(v.host.Text()),
									Port: port,
									Protocol: v.protocol,
									User: strings.TrimSpace(v.user.Text()),
									Password: v.password.Text(),
									Database: strings.TrimSpace(v.database.Text()),
									Secure: v.secure,
								})
								v.formOpen = false
							}
						}
						return button.Layout(gtx)
					}),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						button := material.Button(theme, &v.cancel, "取消")
						if v.cancel.Clicked(gtx) {
							v.formOpen = false
							v.formError = ""
						}
						return button.Layout(gtx)
					}),
				)
			}),
		)
	})
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
						addButton := material.Button(theme, &v.add, "新增")
						button.TextSize = unit.Sp(12)
						if v.refresh.Clicked(gtx) && v.onRefresh != nil {
							v.onRefresh()
						}
						if v.add.Clicked(gtx) {
							v.formOpen = !v.formOpen
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
			if v.formOpen { return v.layoutEditor(gtx, theme) }
			return layout.Dimensions{}
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
