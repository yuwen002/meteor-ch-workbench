package fyneapp

import (
	"fmt"
	"strconv"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/widget"

	"github.com/yuwen002/meteor-ch-workbench/internal/config"
)

type ConnectionDialog struct {
	window   fyne.Window
	onSaved  func()
}

func NewConnectionDialog(w fyne.Window, onSaved func()) *ConnectionDialog {
	return &ConnectionDialog{window: w, onSaved: onSaved}
}

func (cd *ConnectionDialog) showNew() {
	cd.buildEditDialog("", nil)
}

func (cd *ConnectionDialog) showEdit(connID string) {
	var existing *config.Connection
	if connID != "" {
		if conn, err := Backend().GetConnection(connID); err == nil && conn != nil {
			existing = conn
		}
	}
	cd.buildEditDialog(connID, existing)
}

func (cd *ConnectionDialog) showDelete(connID string) {
	conn, err := Backend().GetConnection(connID)
	name := connID
	if err == nil && conn != nil {
		name = conn.Name
	}
	dialog.ShowConfirm("删除连接", fmt.Sprintf("确定要删除连接 %q 吗？", name), func(ok bool) {
		if !ok {
			return
		}
		if err := Backend().DeleteConnection(connID); err != nil {
			dialog.ShowError(err, cd.window)
			return
		}
		if cd.onSaved != nil {
			cd.onSaved()
		}
	}, cd.window)
}

func (cd *ConnectionDialog) showEditFolder(folderID string) {
	folders, _ := Backend().GetFolders()
	var existing *config.Folder
	for _, f := range folders {
		if f.ID == folderID {
			fcopy := f
			existing = &fcopy
			break
		}
	}
	if existing == nil {
		dialog.ShowError(fmt.Errorf("文件夹不存在: %s", folderID), cd.window)
		return
	}

	name := widget.NewEntry()
	name.SetText(existing.Name)

	formItems := []*widget.FormItem{
		{Text: "文件夹名称", Widget: name},
	}
	dlg := dialog.NewForm("编辑文件夹", "保存", "取消", formItems, func(ok bool) {
		if !ok {
			return
		}
		if name.Text == "" {
			dialog.ShowInformation("提示", "名称不能为空", cd.window)
			return
		}
		existing.Name = name.Text
		if err := Backend().SaveFolder(*existing); err != nil {
			dialog.ShowError(err, cd.window)
			return
		}
		if cd.onSaved != nil {
			cd.onSaved()
		}
	}, cd.window)
	dlg.Resize(fyne.NewSize(360, 180))
	dlg.Show()
}

func (cd *ConnectionDialog) buildEditDialog(connID string, existing *config.Connection) {
	name := widget.NewEntry()
	host := widget.NewEntry()
	host.SetText("localhost")
	port := widget.NewEntry()
	port.SetText("9000")
	user := widget.NewEntry()
	user.SetText("default")
	password := widget.NewPasswordEntry()
	database := widget.NewEntry()

	var folderNames []string
	var folderIDs []string
	folderNames = append(folderNames, "(根级)")
	folderIDs = append(folderIDs, "")
	folders, _ := Backend().GetFolders()
	for _, f := range folders {
		folderNames = append(folderNames, f.Name)
		folderIDs = append(folderIDs, f.ID)
	}
	folderSelect := widget.NewSelect(folderNames, func(_ string) {})
	folderSelect.Selected = folderNames[0]

	isNew := existing == nil
	title := "新建连接"
	if !isNew {
		title = "编辑连接"
		name.SetText(existing.Name)
		host.SetText(existing.Host)
		port.SetText(strconv.Itoa(existing.Port))
		user.SetText(existing.User)
		password.SetText(existing.Password)
		database.SetText(existing.Database)
		if existing.FolderID != "" {
			for i, fid := range folderIDs {
				if fid == existing.FolderID {
					folderSelect.Selected = folderNames[i]
					break
				}
			}
		}
	}

	formItems := []*widget.FormItem{
		{Text: "连接名称", Widget: name},
		{Text: "所属文件夹", Widget: folderSelect},
		{Text: "主机", Widget: host},
		{Text: "端口", Widget: port},
		{Text: "用户名", Widget: user},
		{Text: "密码", Widget: password},
		{Text: "默认数据库", Widget: database},
	}

	var dlg dialog.Dialog
	saveBtn := widget.NewButton("保存", func() {
		p, _ := strconv.Atoi(port.Text)
		if p == 0 {
			p = 9000
		}
		folderIdx := 0
		for i, fn := range folderNames {
			if fn == folderSelect.Selected {
				folderIdx = i
				break
			}
		}
		folderID := folderIDs[folderIdx]

		conn := config.Connection{
			Name:     name.Text,
			FolderID: folderID,
			Host:     host.Text,
			Port:     p,
			User:     user.Text,
			Password: password.Text,
			Database: database.Text,
		}
		if isNew {
			conn.GenID()
		} else {
			conn.ID = connID
		}
		conn.Normalize()

		if err := Backend().SaveConnection(conn); err != nil {
			dialog.ShowError(err, cd.window)
			return
		}
		if cd.onSaved != nil {
			cd.onSaved()
		}
		dlg.Hide()
	})
	saveBtn.Importance = widget.HighImportance

	testBtn := widget.NewButton("测试连接", func() {
		p, _ := strconv.Atoi(port.Text)
		if p == 0 {
			p = 9000
		}
		conn := config.Connection{
			Name:     name.Text,
			Host:     host.Text,
			Port:     p,
			User:     user.Text,
			Password: password.Text,
			Database: database.Text,
		}
		conn.Normalize()
		if err := Backend().TestConnection(conn); err != nil {
			dialog.ShowError(fmt.Errorf("连接失败: %w", err), cd.window)
		} else {
			dialog.ShowInformation("成功", "连接测试通过", cd.window)
		}
	})

	cancelBtn := widget.NewButton("取消", func() {
		dlg.Hide()
	})

	deleteBtn := widget.NewButton("删除连接", func() {
		dialog.ShowConfirm("删除连接", fmt.Sprintf("确定要删除连接 %q 吗？", name.Text), func(ok bool) {
			if !ok {
				return
			}
			if err := Backend().DeleteConnection(connID); err != nil {
				dialog.ShowError(err, cd.window)
				return
			}
			if cd.onSaved != nil {
				cd.onSaved()
			}
			dlg.Hide()
		}, cd.window)
	})
	deleteBtn.Importance = widget.DangerImportance
	if isNew {
		deleteBtn.Hidden = true
	}

	dlg = dialog.NewForm(title, "", "", formItems, func(_ bool) {}, cd.window)

	formBox := container.NewVBox()
	for _, item := range formItems {
		formBox.Add(container.NewBorder(nil, nil, widget.NewLabel(item.Text+"  "), nil, item.Widget))
	}

	buttons := container.NewHBox(
		saveBtn, testBtn, cancelBtn, deleteBtn,
	)

	content := container.NewBorder(
		nil, buttons, nil, nil,
		container.NewScroll(formBox),
	)

	custom := dialog.NewCustom(title, "", content, cd.window)
	custom.Resize(fyne.NewSize(500, 480))
	dlg = custom
	dlg.Show()
}

func (cd *ConnectionDialog) showNewFolder() {
	name := widget.NewEntry()

	items := []*widget.FormItem{
		{Text: "文件夹名称", Widget: name},
	}
	dlg := dialog.NewForm("新建文件夹", "创建", "取消", items, func(ok bool) {
		if !ok {
			return
		}
		if name.Text == "" {
			dialog.ShowInformation("提示", "名称不能为空", cd.window)
			return
		}
		f := config.Folder{Name: name.Text}
		f.GenID()
		if err := Backend().SaveFolder(f); err != nil {
			dialog.ShowError(err, cd.window)
			return
		}
		if cd.onSaved != nil {
			cd.onSaved()
		}
	}, cd.window)
	dlg.Resize(fyne.NewSize(360, 180))
	dlg.Show()
}