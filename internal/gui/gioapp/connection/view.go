package connection

import (
	"fmt"

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
}

func NewView(model *Model, onRefresh func(), onSelect func(domain.Connection)) (*View, error) {
	if model == nil {
		return nil, fmt.Errorf("连接状态模型不能为空")
	}
	return &View{
		model: model,
		list: widget.List{List: layout.List{Axis: layout.Vertical}},
		rows: make(map[string]*widget.Clickable),
		onRefresh: onRefresh,
		onSelect: onSelect,
	}, nil
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
						button.TextSize = unit.Sp(12)
						if v.refresh.Clicked(gtx) && v.onRefresh != nil {
							v.onRefresh()
						}
						return button.Layout(gtx)
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
