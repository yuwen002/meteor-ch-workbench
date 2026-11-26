package connection

import (
	"context"
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
type sidebarRow struct {
	folder     *domain.Folder
	item       *domain.Connection
	unassigned bool
}

func buildSidebarRows(items []domain.Connection, folders []domain.Folder, collapsed map[string]bool) ([]sidebarRow, int) {
	rows := make([]sidebarRow, 0, len(items)+len(folders)+1)
	assigned := make(map[string][]domain.Connection)
	knownFolders := make(map[string]struct{}, len(folders))
	for _, folder := range folders {
		knownFolders[folder.ID] = struct{}{}
	}
	var unassigned []domain.Connection
	orphanedCount := 0
	for _, item := range items {
		if item.FolderID == "" {
			unassigned = append(unassigned, item)
			continue
		}
		if _, ok := knownFolders[item.FolderID]; !ok {
			// 分组记录缺失时仍展示连接，避免孤立连接在侧栏中不可见。
			unassigned = append(unassigned, item)
			orphanedCount++
			continue
		}
		assigned[item.FolderID] = append(assigned[item.FolderID], item)
	}
	for i := range folders {
		folder := folders[i]
		rows = append(rows, sidebarRow{folder: &folder})
		if !collapsed[folder.ID] {
			for j := range assigned[folder.ID] {
				item := assigned[folder.ID][j]
				rows = append(rows, sidebarRow{item: &item})
			}
		}
	}
	if len(unassigned) > 0 || len(folders) == 0 {
		rows = append(rows, sidebarRow{unassigned: true})
		for i := range unassigned {
			item := unassigned[i]
			rows = append(rows, sidebarRow{item: &item})
		}
	}
	return rows, orphanedCount
}

func nextFolderID(folders []domain.Folder, current string) string {
	if current == "" {
		if len(folders) > 0 {
			return folders[0].ID
		}
		return ""
	}
	for i, item := range folders {
		if item.ID == current {
			if i+1 < len(folders) {
				return folders[i+1].ID
			}
			return ""
		}
	}
	return ""
}

type View struct {
	model                  *Model
	diagnostic             string
	list                   widget.List
	folderPickerList       widget.List
	refresh                widget.Clickable
	rows                   map[string]*widget.Clickable
	onRefresh              func()
	onSelect               func(domain.Connection)
	selectedConnectionID   string
	connectedConnectionID  string
	connectingConnectionID string
	onSave                 func(domain.Connection)
	add                    widget.Clickable
	save                   widget.Clickable
	cancel                 widget.Clickable
	protocolToggle         widget.Clickable
	secureToggle           widget.Clickable
	name                   widget.Editor
	host                   widget.Editor
	port                   widget.Editor
	user                   widget.Editor
	password               widget.Editor
	database               widget.Editor
	protocol               domain.Protocol
	secure                 bool
	formOpen               bool
	formError              string
	folderAdd              widget.Clickable
	folderName             widget.Editor
	folderSave             widget.Clickable
	folderCancel           widget.Clickable
	folderOpen             bool
	editingFolderID        string
	folderErrors           string
	folderCollapsed        map[string]bool
	folderRename           map[string]*widget.Clickable
	folderUp               map[string]*widget.Clickable
	folderDown             map[string]*widget.Clickable
	folderDelete           map[string]*widget.Clickable
	folderMore             map[string]*widget.Clickable
	folderActionsOpen      map[string]bool
	folderMove             map[string]*widget.Clickable
	rowEdit                map[string]*widget.Clickable
	rowUp                  map[string]*widget.Clickable
	rowDown                map[string]*widget.Clickable
	rowDelete              map[string]*widget.Clickable
	rowTest                map[string]*widget.Clickable
	rowMore                map[string]*widget.Clickable
	rowActionsOpen         map[string]bool
	editingConnectionID    string
	formFolderID           string
	folderPick             widget.Clickable
	onDelete               func(string)
	deleteConfirmID        string
	deleteConfirm          widget.Clickable
	deleteCancel           widget.Clickable
	folderPickerOpen       bool
	folderPickOptions      map[string]*widget.Clickable
	folderPickTargetID     string
	folderDeleteConfirmID  string
	folderDeleteConfirm    widget.Clickable
}

func NewView(model *Model, onRefresh func(), onSelect func(domain.Connection)) (*View, error) {
	if model == nil {
		return nil, fmt.Errorf("连接状态模型不能为空")
	}
	view := &View{
		model:             model,
		list:              widget.List{List: layout.List{Axis: layout.Vertical}},
		folderPickerList:  widget.List{List: layout.List{Axis: layout.Vertical}},
		rows:              make(map[string]*widget.Clickable),
		folderCollapsed:   make(map[string]bool),
		folderRename:      make(map[string]*widget.Clickable),
		folderUp:          make(map[string]*widget.Clickable),
		folderDown:        make(map[string]*widget.Clickable),
		folderDelete:      make(map[string]*widget.Clickable),
		folderMore:        make(map[string]*widget.Clickable),
		folderActionsOpen: make(map[string]bool),
		folderMove:        make(map[string]*widget.Clickable),
		rowEdit:           make(map[string]*widget.Clickable),
		rowUp:             make(map[string]*widget.Clickable),
		rowDown:           make(map[string]*widget.Clickable),
		rowDelete:         make(map[string]*widget.Clickable),
		rowTest:           make(map[string]*widget.Clickable),
		rowMore:           make(map[string]*widget.Clickable),
		rowActionsOpen:    make(map[string]bool),
		folderPickOptions: make(map[string]*widget.Clickable),
		onRefresh:         onRefresh,
		onSelect:          onSelect,
		protocol:          domain.ProtocolNative,
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

func (v *View) SetOnDelete(onDelete func(string)) { v.onDelete = onDelete }

// SetConnectionStatus 区分列表选中、正在连接和已建立连接，避免把选中状态误显示为在线。
// SetDiagnostic 显示当前配置文件路径和读取状态，便于定位历史连接未显示的问题。
func (v *View) SetDiagnostic(message string) { v.diagnostic = message }

func (v *View) SetConnectionStatus(selectedID, connectedID, connectingID string) {
	v.selectedConnectionID = selectedID
	v.connectedConnectionID = connectedID
	v.connectingConnectionID = connectingID
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
		layout.Rigid(material.H6(theme, func() string {
			if v.editingConnectionID != "" {
				return "编辑连接"
			}
			return "新增连接"
		}()).Layout),
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
			label := "所属分组：未分组"
			for _, folder := range v.model.Snapshot().Folders {
				if folder.ID == v.formFolderID {
					label = "所属分组：" + folder.Name
					break
				}
			}
			if v.folderPick.Clicked(gtx) {
				v.folderPickTargetID = ""
				v.folderPickerOpen = true
			}
			button := material.Button(theme, &v.folderPick, label+"（选择）")
			button.TextSize = unit.Sp(12)
			return button.Layout(gtx)
		}),

		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
				layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
					if v.protocolToggle.Clicked(gtx) {
						if v.protocol == domain.ProtocolNative {
							v.protocol = domain.ProtocolHTTP
						} else {
							v.protocol = domain.ProtocolNative
						}
					}
					button := material.Button(theme, &v.protocolToggle, "协议："+string(v.protocol))
					button.TextSize = unit.Sp(13)
					return button.Layout(gtx)
				}),
				layout.Rigid(layout.Spacer{Width: unit.Dp(12)}.Layout),
				layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
					if v.secureToggle.Clicked(gtx) {
						v.secure = !v.secure
					}
					label := "TLS：关闭"
					if v.secure {
						label = "TLS：开启"
					}
					button := material.Button(theme, &v.secureToggle, label)
					button.TextSize = unit.Sp(13)
					return button.Layout(gtx)
				}),
			)
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			if v.formError == "" {
				return layout.Dimensions{}
			}
			return layout.Inset{Top: unit.Dp(8)}.Layout(gtx, material.Caption(theme, v.formError).Layout)
		}),
		layout.Rigid(layout.Spacer{Height: unit.Dp(14)}.Layout),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
				layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
					return layout.Dimensions{Size: image.Pt(gtx.Constraints.Max.X, 0)}
				}),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					button := material.Button(theme, &v.cancel, "取消")
					button.TextSize = unit.Sp(13)
					if v.cancel.Clicked(gtx) {
						v.formOpen = false
						v.formError = ""
					}
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
								ID: v.editingConnectionID, FolderID: v.formFolderID, Name: strings.TrimSpace(v.name.Text()), Host: strings.TrimSpace(v.host.Text()), Port: port,
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
			return layout.Inset{Bottom: unit.Dp(10)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				title := material.Label(theme, unit.Sp(14), "连接")
				title.Color = colorTextPrimary
				title.Font.Weight = 600
				return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
					layout.Flexed(1, title.Layout),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						if v.refresh.Clicked(gtx) && v.onRefresh != nil {
							v.onRefresh()
						}
						return compactButton(gtx, theme, &v.refresh, "刷新", false)
					}),
					layout.Rigid(layout.Spacer{Width: unit.Dp(6)}.Layout),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						if v.add.Clicked(gtx) {
							v.resetForm()
							v.formOpen = true
						}
						return compactButton(gtx, theme, &v.add, "新增", true)
					}),
					layout.Rigid(layout.Spacer{Width: unit.Dp(6)}.Layout),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						if v.folderAdd.Clicked(gtx) {
							v.editingFolderID = ""
							v.folderName.SetText("")
							v.folderErrors = ""
							v.folderOpen = true
						}
						return compactButton(gtx, theme, &v.folderAdd, "分组", false)
					}),
				)
			})
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			if snapshot.Loading {
				return statusLine(gtx, theme, colorTextMuted, "正在加载连接…")
			}
			if snapshot.Error != "" {
				return statusLine(gtx, theme, colorDanger, snapshot.Error)
			}
			return layout.Dimensions{}
		}),
		layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
			rows, orphanedCount := buildSidebarRows(snapshot.Items, snapshot.Folders, v.folderCollapsed)
			if len(rows) == 0 {
				return layout.Inset{Top: unit.Dp(16), Left: unit.Dp(8), Right: unit.Dp(8)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
					return layout.Flex{Axis: layout.Vertical, Alignment: layout.Middle}.Layout(gtx,
						layout.Rigid(material.Body1(theme, "暂无已保存连接").Layout),
						layout.Rigid(layout.Spacer{Height: unit.Dp(6)}.Layout),
						layout.Rigid(material.Caption(theme, "点击上方“新增”添加 ClickHouse 数据库").Layout),
						layout.Rigid(layout.Spacer{Height: unit.Dp(12)}.Layout),
						layout.Rigid(material.Caption(theme, v.diagnostic).Layout),
					)
				})
			}
			return material.List(theme, &v.list).Layout(gtx, len(rows), func(gtx layout.Context, index int) layout.Dimensions {
				row := rows[index]
				if row.folder != nil {
					return v.layoutFolderRow(gtx, theme, *row.folder, snapshot.Items)
				}
				if row.unassigned {
					label := "未分组"
					if orphanedCount > 0 {
						label = fmt.Sprintf("未分组（含 %d 条失效分组连接）", orphanedCount)
					}
					return layout.Inset{Top: unit.Dp(10), Bottom: unit.Dp(2), Left: unit.Dp(2)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
						text := material.Label(theme, unit.Sp(10), label)
						text.Color = colorTextMuted
						return text.Layout(gtx)
					})
				}
				if row.item == nil {
					return layout.Dimensions{}
				}
				return v.layoutConnectionRow(gtx, theme, *row.item)
			})
		}),
	)
}

// layoutFolderRow 绘制分组标题行，以及展开后的改名 / 删除操作。
func (v *View) layoutFolderRow(gtx layout.Context, theme *material.Theme, folder domain.Folder, items []domain.Connection) layout.Dimensions {
	if v.folderUp[folder.ID] == nil { v.folderUp[folder.ID] = new(widget.Clickable) }
	if v.folderDown[folder.ID] == nil { v.folderDown[folder.ID] = new(widget.Clickable) }
	if v.folderRename[folder.ID] == nil {
		v.folderRename[folder.ID] = new(widget.Clickable)
	}
	if v.folderDelete[folder.ID] == nil {
		v.folderDelete[folder.ID] = new(widget.Clickable)
	}
	if v.folderMore[folder.ID] == nil {
		v.folderMore[folder.ID] = new(widget.Clickable)
	}
	key := "folder:" + folder.ID
	if v.rows[key] == nil {
		v.rows[key] = new(widget.Clickable)
	}
	if v.rows[key].Clicked(gtx) {
		v.folderCollapsed[folder.ID] = !v.folderCollapsed[folder.ID]
	}
	prefix := "▾ "
	if v.folderCollapsed[folder.ID] {
		prefix = "▸ "
	}
	connectionCount := 0
	for _, item := range items {
		if item.FolderID == folder.ID {
			connectionCount++
		}
	}
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return layout.Inset{Top: unit.Dp(8), Bottom: unit.Dp(2)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
					layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
						return v.rows[key].Layout(gtx, func(gtx layout.Context) layout.Dimensions {
							return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
								layout.Rigid(func(gtx layout.Context) layout.Dimensions {
									name := material.Label(theme, unit.Sp(12), prefix+folder.Name)
									name.Color = colorTextSecondary
									name.Font.Weight = 600
									name.MaxLines = 1
									name.Truncator = "…"
									return name.Layout(gtx)
								}),
								layout.Rigid(layout.Spacer{Width: unit.Dp(6)}.Layout),
								layout.Rigid(func(gtx layout.Context) layout.Dimensions {
									count := material.Label(theme, unit.Sp(10), strconv.Itoa(connectionCount))
									count.Color = colorTextMuted
									return count.Layout(gtx)
								}),
							)
						})
					}),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						if v.folderMore[folder.ID].Clicked(gtx) {
							v.folderActionsOpen[folder.ID] = !v.folderActionsOpen[folder.ID]
						}
						label := "更多"
						if v.folderActionsOpen[folder.ID] {
							label = "收起"
						}
						return ghostButton(gtx, theme, v.folderMore[folder.ID], label)
					}),
				)
			})
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			if !v.folderActionsOpen[folder.ID] {
				return layout.Dimensions{}
			}
			return layout.Inset{Left: unit.Dp(14), Bottom: unit.Dp(2)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						if v.folderUp[folder.ID].Clicked(gtx) { v.moveFolder(folder.ID, -1) }
						return compactButton(gtx, theme, v.folderUp[folder.ID], "上移", false)
					}),
					layout.Rigid(layout.Spacer{Width: unit.Dp(4)}.Layout),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						if v.folderDown[folder.ID].Clicked(gtx) { v.moveFolder(folder.ID, 1) }
						return compactButton(gtx, theme, v.folderDown[folder.ID], "下移", false)
					}),
					layout.Rigid(layout.Spacer{Width: unit.Dp(6)}.Layout),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						if v.folderRename[folder.ID].Clicked(gtx) {
							v.editingFolderID = folder.ID
							v.folderName.SetText(folder.Name)
							v.folderErrors = ""
							v.folderOpen = true
						}
						return compactButton(gtx, theme, v.folderRename[folder.ID], "改名", false)
					}),
					layout.Rigid(layout.Spacer{Width: unit.Dp(6)}.Layout),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						if v.folderDelete[folder.ID].Clicked(gtx) {
							v.folderDeleteConfirmID = folder.ID
						}
						return compactButton(gtx, theme, v.folderDelete[folder.ID], "删除", false)
					}),
				)
			})
		}),
	)
}

// moveFolder 通过上移 / 下移操作调整分组顺序，并立即写回配置文件。
func (v *View) moveFolder(id string, delta int) {
	snapshot := v.model.Snapshot()
	ids := make([]string, len(snapshot.Folders))
	index := -1
	for i, folder := range snapshot.Folders {
		ids[i] = folder.ID
		if folder.ID == id { index = i }
	}
	target := index + delta
	if index < 0 || target < 0 || target >= len(ids) { return }
	ids[index], ids[target] = ids[target], ids[index]
	_ = v.model.ReorderFolders(context.Background(), ids)
}

// moveConnection 仅在同一分组内调整连接顺序，不会意外改变连接所属分组。
func (v *View) moveConnection(id string, delta int) {
	snapshot := v.model.Snapshot()
	ids := make([]string, len(snapshot.Items))
	index := -1
	for i, item := range snapshot.Items {
		ids[i] = item.ID
		if item.ID == id { index = i }
	}
	if index < 0 { return }
	target := index + delta
	for target >= 0 && target < len(snapshot.Items) && snapshot.Items[target].FolderID != snapshot.Items[index].FolderID {
		target += delta
	}
	if target < 0 || target >= len(snapshot.Items) { return }
	ids[index], ids[target] = ids[target], ids[index]
	_ = v.model.ReorderConnections(context.Background(), ids)
}

// layoutConnectionRow 绘制连接卡片：状态圆点 + 名称 + 状态徽标 + 地址，操作按钮折叠在「更多操作」内。
func (v *View) layoutConnectionRow(gtx layout.Context, theme *material.Theme, item domain.Connection) layout.Dimensions {
	if v.rows[item.ID] == nil {
		v.rows[item.ID] = new(widget.Clickable)
	}
	if v.rowMore[item.ID] == nil {
		v.rowMore[item.ID] = new(widget.Clickable)
	}
	if v.rowTest[item.ID] == nil {
		v.rowTest[item.ID] = new(widget.Clickable)
	}
	if v.rowUp[item.ID] == nil { v.rowUp[item.ID] = new(widget.Clickable) }
	if v.rowDown[item.ID] == nil { v.rowDown[item.ID] = new(widget.Clickable) }
	if v.rowEdit[item.ID] == nil {
		v.rowEdit[item.ID] = new(widget.Clickable)
	}
	if v.folderMove[item.ID] == nil {
		v.folderMove[item.ID] = new(widget.Clickable)
	}
	if v.rowDelete[item.ID] == nil {
		v.rowDelete[item.ID] = new(widget.Clickable)
	}
	click := v.rows[item.ID]
	selected := item.ID == v.selectedConnectionID
	state, dot := "未连接", colorTextMuted
	if item.ID == v.connectingConnectionID {
		state, dot = "连接中", colorWarning
	}
	if item.ID == v.connectedConnectionID {
		state, dot = "已连接", colorSuccess
	}
	return layout.Inset{Top: unit.Dp(3), Bottom: unit.Dp(3)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		return cardSurface(gtx, selected, func(gtx layout.Context) layout.Dimensions {
			return layout.Inset{Left: unit.Dp(10), Right: unit.Dp(8), Top: unit.Dp(8), Bottom: unit.Dp(6)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						if click.Clicked(gtx) {
							v.model.Select(item.ID)
							if v.onSelect != nil {
								v.onSelect(item)
							}
						}
						return click.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
							name := material.Label(theme, unit.Sp(13), item.Name)
							name.Color = colorTextPrimary
							name.Font.Weight = 600
							name.MaxLines = 1
							name.Truncator = "…"
							return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
								layout.Rigid(func(gtx layout.Context) layout.Dimensions {
									return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
										layout.Rigid(func(gtx layout.Context) layout.Dimensions {
											size := image.Pt(gtx.Dp(unit.Dp(7)), gtx.Dp(unit.Dp(7)))
											fillRoundRect(gtx, size, dot)
											return layout.Dimensions{Size: size}
										}),
										layout.Rigid(layout.Spacer{Width: unit.Dp(6)}.Layout),
										layout.Flexed(1, name.Layout),
										layout.Rigid(func(gtx layout.Context) layout.Dimensions {
											return stateBadge(gtx, theme, state, dot)
										}),
									)
								}),
								layout.Rigid(layout.Spacer{Height: unit.Dp(4)}.Layout),
								layout.Rigid(func(gtx layout.Context) layout.Dimensions {
									address := material.Label(theme, unit.Sp(11), fmt.Sprintf("%s:%d · %s", item.Host, item.Port, item.Protocol))
									address.Color = colorTextMuted
									return layout.Inset{Left: unit.Dp(13)}.Layout(gtx, address.Layout)
								}),
							)
						})
					}),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return layout.Inset{Left: unit.Dp(13), Top: unit.Dp(4)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
							if v.rowMore[item.ID].Clicked(gtx) {
								v.rowActionsOpen[item.ID] = !v.rowActionsOpen[item.ID]
							}
							label := "更多操作"
							if v.rowActionsOpen[item.ID] {
								label = "收起操作"
							}
							return ghostButton(gtx, theme, v.rowMore[item.ID], label)
						})
					}),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						if !v.rowActionsOpen[item.ID] {
							return layout.Dimensions{}
						}
						return layout.Inset{Top: unit.Dp(6)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
							return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
								layout.Rigid(func(gtx layout.Context) layout.Dimensions {
									if v.rowUp[item.ID].Clicked(gtx) { v.moveConnection(item.ID, -1) }
									return compactButton(gtx, theme, v.rowUp[item.ID], "上移", false)
								}),
								layout.Rigid(layout.Spacer{Width: unit.Dp(4)}.Layout),
								layout.Rigid(func(gtx layout.Context) layout.Dimensions {
									if v.rowDown[item.ID].Clicked(gtx) { v.moveConnection(item.ID, 1) }
									return compactButton(gtx, theme, v.rowDown[item.ID], "下移", false)
								}),
								layout.Rigid(layout.Spacer{Width: unit.Dp(6)}.Layout),
								layout.Rigid(func(gtx layout.Context) layout.Dimensions {
									if v.rowTest[item.ID].Clicked(gtx) && v.onSelect != nil {
										v.onSelect(item)
									}
									return compactButton(gtx, theme, v.rowTest[item.ID], "测试", false)
								}),
								layout.Rigid(layout.Spacer{Width: unit.Dp(6)}.Layout),
								layout.Rigid(func(gtx layout.Context) layout.Dimensions {
									if v.rowEdit[item.ID].Clicked(gtx) {
										v.loadConnectionForm(item)
									}
									return compactButton(gtx, theme, v.rowEdit[item.ID], "编辑", false)
								}),
								layout.Rigid(layout.Spacer{Width: unit.Dp(6)}.Layout),
								layout.Rigid(func(gtx layout.Context) layout.Dimensions {
									if v.folderMove[item.ID].Clicked(gtx) {
										v.folderPickTargetID = item.ID
										v.formFolderID = item.FolderID
										v.folderPickerOpen = true
									}
									return compactButton(gtx, theme, v.folderMove[item.ID], "分组", false)
								}),
								layout.Rigid(layout.Spacer{Width: unit.Dp(6)}.Layout),
								layout.Rigid(func(gtx layout.Context) layout.Dimensions {
									if v.rowDelete[item.ID].Clicked(gtx) {
										v.deleteConfirmID = item.ID
									}
									return compactButton(gtx, theme, v.rowDelete[item.ID], "删除", false)
								}),
							)
						})
					}),
				)
			})
		})
	})
}

func (v *View) loadConnectionForm(item domain.Connection) {
	v.editingConnectionID = item.ID
	v.formFolderID = item.FolderID
	v.name.SetText(item.Name)
	v.host.SetText(item.Host)
	v.port.SetText(strconv.Itoa(item.Port))
	v.user.SetText(item.User)
	v.password.SetText(item.Password)
	v.database.SetText(item.Database)
	v.protocol = item.Protocol
	v.secure = item.Secure
	v.formError = ""
	v.formOpen = true
}

func (v *View) resetForm() {
	v.editingConnectionID = ""
	v.formFolderID = ""
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
	if v.deleteConfirmID != "" {
		return v.deleteModal(gtx, theme)
	}
	if v.folderDeleteConfirmID != "" {
		return v.folderDeleteModal(gtx, theme)
	}
	if v.folderPickerOpen {
		return v.folderPickerModal(gtx, theme)
	}
	if !v.formOpen && !v.folderOpen {
		return layout.Dimensions{}
	}
	if v.folderOpen {
		return v.folderModal(gtx, theme)
	}
	return layout.Stack{Alignment: layout.Center}.Layout(gtx,
		layout.Expanded(func(gtx layout.Context) layout.Dimensions {
			paint.FillShape(gtx.Ops, color.NRGBA{R: 20, G: 25, B: 35, A: 105}, clip.Rect{Max: gtx.Constraints.Max}.Op())
			return layout.Dimensions{Size: gtx.Constraints.Max}
		}),
		layout.Stacked(func(gtx layout.Context) layout.Dimensions {
			width := gtx.Dp(unit.Dp(460))
			height := gtx.Dp(unit.Dp(570))
			if width > gtx.Constraints.Max.X-gtx.Dp(unit.Dp(32)) {
				width = gtx.Constraints.Max.X - gtx.Dp(unit.Dp(32))
			}
			if height > gtx.Constraints.Max.Y-gtx.Dp(unit.Dp(32)) {
				height = gtx.Constraints.Max.Y - gtx.Dp(unit.Dp(32))
			}
			if width < 0 {
				width = 0
			}
			if height < 0 {
				height = 0
			}
			gtx.Constraints = layout.Exact(image.Pt(width, height))
			paint.FillShape(gtx.Ops, color.NRGBA{R: 250, G: 251, B: 253, A: 255}, clip.Rect{Max: gtx.Constraints.Max}.Op())
			return layout.Inset{Top: unit.Dp(22), Bottom: unit.Dp(18), Left: unit.Dp(24), Right: unit.Dp(24)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				return v.layoutEditor(gtx, theme)
			})
		}),
	)
}

func (v *View) folderModal(gtx layout.Context, theme *material.Theme) layout.Dimensions {
	return layout.Stack{Alignment: layout.Center}.Layout(gtx,
		layout.Expanded(func(gtx layout.Context) layout.Dimensions {
			paint.FillShape(gtx.Ops, color.NRGBA{R: 20, G: 25, B: 35, A: 105}, clip.Rect{Max: gtx.Constraints.Max}.Op())
			return layout.Dimensions{Size: gtx.Constraints.Max}
		}),
		layout.Stacked(func(gtx layout.Context) layout.Dimensions {
			width := gtx.Dp(unit.Dp(400))
			height := gtx.Dp(unit.Dp(210))
			if width > gtx.Constraints.Max.X-gtx.Dp(unit.Dp(32)) {
				width = gtx.Constraints.Max.X - gtx.Dp(unit.Dp(32))
			}
			gtx.Constraints = layout.Exact(image.Pt(width, height))
			paint.FillShape(gtx.Ops, color.NRGBA{R: 250, G: 251, B: 253, A: 255}, clip.Rect{Max: gtx.Constraints.Max}.Op())
			return layout.Inset{Top: unit.Dp(20), Bottom: unit.Dp(16), Left: unit.Dp(22), Right: unit.Dp(22)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				title := "新建分组"
				if v.editingFolderID != "" {
					title = "重命名分组"
				}
				return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
					layout.Rigid(material.H6(theme, title).Layout),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return material.Editor(theme, &v.folderName, "分组名称").Layout(gtx)
					}),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						if v.folderErrors == "" {
							return layout.Dimensions{}
						}
						return material.Caption(theme, v.folderErrors).Layout(gtx)
					}),
					layout.Rigid(layout.Spacer{Height: unit.Dp(14)}.Layout),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
							layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
								return layout.Dimensions{Size: image.Pt(gtx.Constraints.Max.X, 0)}
							}),
							layout.Rigid(func(gtx layout.Context) layout.Dimensions {
								b := material.Button(theme, &v.folderCancel, "取消")
								if v.folderCancel.Clicked(gtx) {
									v.folderOpen = false
									v.folderErrors = ""
								}
								return layout.Inset{Right: unit.Dp(8)}.Layout(gtx, b.Layout)
							}),
							layout.Rigid(func(gtx layout.Context) layout.Dimensions {
								b := material.Button(theme, &v.folderSave, "保存")
								if v.folderSave.Clicked(gtx) {
									_, err := v.model.SaveFolder(context.Background(), domain.Folder{ID: v.editingFolderID, Name: strings.TrimSpace(v.folderName.Text())})
									if err != nil {
										v.folderErrors = err.Error()
									} else {
										v.folderOpen = false
										v.folderErrors = ""
									}
								}
								return b.Layout(gtx)
							}),
						)
					}),
				)
			})
		}),
	)
}

func (v *View) folderPickerModal(gtx layout.Context, theme *material.Theme) layout.Dimensions {
	snapshot := v.model.Snapshot()
	return layout.Stack{Alignment: layout.Center}.Layout(gtx,
		layout.Expanded(func(gtx layout.Context) layout.Dimensions {
			paint.FillShape(gtx.Ops, color.NRGBA{R: 20, G: 25, B: 35, A: 105}, clip.Rect{Max: gtx.Constraints.Max}.Op())
			return layout.Dimensions{Size: gtx.Constraints.Max}
		}),
		layout.Stacked(func(gtx layout.Context) layout.Dimensions {
			width := gtx.Dp(unit.Dp(340))
			height := gtx.Dp(unit.Dp(300))
			if width > gtx.Constraints.Max.X-gtx.Dp(unit.Dp(32)) {
				width = gtx.Constraints.Max.X - gtx.Dp(unit.Dp(32))
			}
			if height > gtx.Constraints.Max.Y-gtx.Dp(unit.Dp(32)) {
				height = gtx.Constraints.Max.Y - gtx.Dp(unit.Dp(32))
			}
			gtx.Constraints = layout.Exact(image.Pt(width, height))
			paint.FillShape(gtx.Ops, color.NRGBA{R: 250, G: 251, B: 253, A: 255}, clip.Rect{Max: gtx.Constraints.Max}.Op())
			return layout.Inset{Top: unit.Dp(18), Bottom: unit.Dp(14), Left: unit.Dp(20), Right: unit.Dp(20)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
					layout.Rigid(material.H6(theme, "选择所属分组").Layout),
					layout.Rigid(layout.Spacer{Height: unit.Dp(8)}.Layout),
					layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
						return material.List(theme, &v.folderPickerList).Layout(gtx, len(snapshot.Folders)+1, func(gtx layout.Context, index int) layout.Dimensions {
							key, label := "", "未分组"
							if index > 0 {
								folder := snapshot.Folders[index-1]
								key = folder.ID
								label = folder.Name
							}
							if v.folderPickOptions[key] == nil {
								v.folderPickOptions[key] = new(widget.Clickable)
							}
							click := v.folderPickOptions[key]
							selected := v.formFolderID == key
							if click.Clicked(gtx) {
								if v.folderPickTargetID != "" {
									for _, item := range snapshot.Items {
										if item.ID == v.folderPickTargetID {
											item.FolderID = key
											if v.onSave != nil {
												v.onSave(item)
											}
											break
										}
									}
								} else {
									v.formFolderID = key
								}
								v.folderPickerOpen = false
								v.folderPickTargetID = ""
							}
							if selected {
								label = "● " + label
							}
							return layout.Inset{Top: unit.Dp(4), Bottom: unit.Dp(4)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
								return click.Layout(gtx, func(gtx layout.Context) layout.Dimensions { return material.Body1(theme, label).Layout(gtx) })
							})
						})
					}),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						b := material.Button(theme, &v.folderCancel, "取消")
						if v.folderCancel.Clicked(gtx) {
							v.folderPickerOpen = false
							v.folderPickTargetID = ""
						}
						return layout.Flex{Axis: layout.Horizontal, Alignment: layout.End}.Layout(gtx, layout.Rigid(b.Layout))
					}),
				)
			})
		}),
	)
}

func (v *View) folderDeleteModal(gtx layout.Context, theme *material.Theme) layout.Dimensions {
	return layout.Stack{Alignment: layout.Center}.Layout(gtx,
		layout.Expanded(func(gtx layout.Context) layout.Dimensions {
			paint.FillShape(gtx.Ops, color.NRGBA{R: 20, G: 25, B: 35, A: 105}, clip.Rect{Max: gtx.Constraints.Max}.Op())
			return layout.Dimensions{Size: gtx.Constraints.Max}
		}),
		layout.Stacked(func(gtx layout.Context) layout.Dimensions {
			width := gtx.Dp(unit.Dp(390))
			height := gtx.Dp(unit.Dp(205))
			if width > gtx.Constraints.Max.X-gtx.Dp(unit.Dp(32)) {
				width = gtx.Constraints.Max.X - gtx.Dp(unit.Dp(32))
			}
			gtx.Constraints = layout.Exact(image.Pt(width, height))
			paint.FillShape(gtx.Ops, color.NRGBA{R: 250, G: 251, B: 253, A: 255}, clip.Rect{Max: gtx.Constraints.Max}.Op())
			return layout.Inset{Top: unit.Dp(20), Bottom: unit.Dp(16), Left: unit.Dp(22), Right: unit.Dp(22)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
					layout.Rigid(material.H6(theme, "删除分组").Layout),
					layout.Rigid(material.Body2(theme, "确定删除此分组吗？分组内的连接会保留，并移动到“未分组”。").Layout),
					layout.Rigid(layout.Spacer{Height: unit.Dp(18)}.Layout),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
							layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
								return layout.Dimensions{Size: image.Pt(gtx.Constraints.Max.X, 0)}
							}),
							layout.Rigid(func(gtx layout.Context) layout.Dimensions {
								b := material.Button(theme, &v.deleteCancel, "取消")
								if v.deleteCancel.Clicked(gtx) {
									v.folderDeleteConfirmID = ""
								}
								return layout.Inset{Right: unit.Dp(8)}.Layout(gtx, b.Layout)
							}),
							layout.Rigid(func(gtx layout.Context) layout.Dimensions {
								b := material.Button(theme, &v.folderDeleteConfirm, "确认删除")
								if v.folderDeleteConfirm.Clicked(gtx) {
									if err := v.model.DeleteFolder(context.Background(), v.folderDeleteConfirmID); err != nil {
										v.folderErrors = err.Error()
									} else {
										delete(v.folderCollapsed, v.folderDeleteConfirmID)
										v.folderErrors = ""
									}
									v.folderDeleteConfirmID = ""
								}
								return b.Layout(gtx)
							}),
						)
					}),
				)
			})
		}),
	)
}

func (v *View) deleteModal(gtx layout.Context, theme *material.Theme) layout.Dimensions {
	return layout.Stack{Alignment: layout.Center}.Layout(gtx,
		layout.Expanded(func(gtx layout.Context) layout.Dimensions {
			paint.FillShape(gtx.Ops, color.NRGBA{R: 20, G: 25, B: 35, A: 105}, clip.Rect{Max: gtx.Constraints.Max}.Op())
			return layout.Dimensions{Size: gtx.Constraints.Max}
		}),
		layout.Stacked(func(gtx layout.Context) layout.Dimensions {
			width := gtx.Dp(unit.Dp(360))
			height := gtx.Dp(unit.Dp(180))
			if width > gtx.Constraints.Max.X-gtx.Dp(unit.Dp(32)) {
				width = gtx.Constraints.Max.X - gtx.Dp(unit.Dp(32))
			}
			gtx.Constraints = layout.Exact(image.Pt(width, height))
			paint.FillShape(gtx.Ops, color.NRGBA{R: 250, G: 251, B: 253, A: 255}, clip.Rect{Max: gtx.Constraints.Max}.Op())
			return layout.Inset{Top: unit.Dp(22), Bottom: unit.Dp(18), Left: unit.Dp(22), Right: unit.Dp(22)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
					layout.Rigid(material.H6(theme, "删除连接").Layout),
					layout.Rigid(material.Body2(theme, "确定删除这个连接吗？此操作无法撤销。").Layout),
					layout.Rigid(layout.Spacer{Height: unit.Dp(20)}.Layout),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
							layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
								return layout.Dimensions{Size: image.Pt(gtx.Constraints.Max.X, 0)}
							}),
							layout.Rigid(func(gtx layout.Context) layout.Dimensions {
								b := material.Button(theme, &v.deleteCancel, "取消")
								if v.deleteCancel.Clicked(gtx) {
									v.deleteConfirmID = ""
								}
								return layout.Inset{Right: unit.Dp(8)}.Layout(gtx, b.Layout)
							}),
							layout.Rigid(func(gtx layout.Context) layout.Dimensions {
								b := material.Button(theme, &v.deleteConfirm, "确认删除")
								if v.deleteConfirm.Clicked(gtx) {
									if v.onDelete != nil {
										v.onDelete(v.deleteConfirmID)
									}
									v.deleteConfirmID = ""
								}
								return b.Layout(gtx)
							}),
						)
					}),
				)
			})
		}),
	)
}
