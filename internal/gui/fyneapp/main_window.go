package fyneapp

import (
	"fmt"
		"strings"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/widget"

	"github.com/yuwen002/meteor-ch-workbench/internal/clickhouse"
	"github.com/yuwen002/meteor-ch-workbench/internal/config"
)

type runShortcut struct {
	name string
}

func (s *runShortcut) ShortcutName() string  { return s.name }
func (s *runShortcut) Key() fyne.KeyName     { return fyne.KeyReturn }
func (s *runShortcut) Mod() fyne.KeyModifier { return fyne.KeyModifierControl }

type stopShortcut struct {
	name string
}

func (s *stopShortcut) ShortcutName() string  { return s.name }
func (s *stopShortcut) Key() fyne.KeyName     { return fyne.KeyEscape }
func (s *stopShortcut) Mod() fyne.KeyModifier { return 0 }

type tabItemData struct {
	*container.TabItem
	tab *QueryTab
}

type dbCacheEntry struct {
	databases []string
	tables    map[string][]clickhouse.TableInfo
	views     map[string][]clickhouse.TableInfo
	clusters  []clickhouse.ClusterInfo
	loaded    bool
}

type MainWindow struct {
	window fyne.Window

	sidebar        *widget.Tree
	sidebarWrapper *treeWrapper
	lastSelectedNode widget.TreeNodeID
	editor         *widget.Entry
	status         *widget.Label
	resultTable    *ResultTable
	pageSize       int
	pageOffset     int

	autoRefresh *time.Ticker
	refreshStop chan struct{}

	appTabs  *container.AppTabs
	tabItems []*tabItemData
	tabs     []*QueryTab
	activeIdx int

	connections []config.Connection
	folders     []config.Folder
	activeConnID string

	dbCache map[string]*dbCacheEntry

	connDialog *ConnectionDialog
	sidePanels *SidePanels
	exporter    *ResultExporter
}

func NewMainWindow(w fyne.Window) fyne.CanvasObject {
	mw := &MainWindow{
		window:  w,
		dbCache: make(map[string]*dbCacheEntry),
	}
	mw.connDialog = NewConnectionDialog(w, mw.refreshSidebar)
	mw.loadTabs()
	mw.buildUI()
	mw.refreshSidebar()
	mw.restoreActiveTab()
	mw.registerShortcuts()
	mw.setupLifecycle()
	return mw.layout()
}

func (mw *MainWindow) registerShortcuts() {
	canvas := mw.window.Canvas()
	canvas.AddShortcut(&runShortcut{name: "meteor.run"}, func(_ fyne.Shortcut) {
		mw.onRunQuery()
	})
	canvas.AddShortcut(&stopShortcut{name: "meteor.stop"}, func(_ fyne.Shortcut) {
		mw.onStopQuery()
	})
}

func (mw *MainWindow) setupLifecycle() {
	mw.window.SetCloseIntercept(func() {
		mw.saveTabs()
		mw.window.Close()
	})
}

func parseNodeID(id widget.TreeNodeID) (kind string, parts []string) {
	if id == "" {
		return "root", nil
	}
	idx := strings.Index(string(id), ":")
	if idx < 0 {
		return "unknown", nil
	}
	kind = string(id)[:idx]
	rest := string(id)[idx+1:]
	parts = strings.Split(rest, ":")
	return kind, parts
}

func (mw *MainWindow) buildUI() {
	mw.sidebar = widget.NewTree(
		mw.treeChildUIDs,
		mw.treeIsBranch,
		func(branch bool) fyne.CanvasObject {
			return widget.NewLabel("")
		},
		mw.treeUpdateNode,
	)
	mw.sidebar.OnSelected = mw.onSidebarSelected
	mw.sidebar.OnBranchOpened = mw.onBranchOpened
	mw.sidebarWrapper = newTreeWrapper(mw.sidebar, func() widget.TreeNodeID {
		return mw.lastSelectedNode
	}, mw.onTreeRightClick)

	mw.editor = widget.NewMultiLineEntry()
	mw.editor.SetText("-- Ctrl+Enter 执行查询\nSELECT 1")

	mw.status = widget.NewLabel("就绪")
	mw.status.Importance = widget.MediumImportance
	mw.pageSize = 100
	mw.pageOffset = 0

	mw.resultTable = NewResultTable()
	mw.resultTable.OnCellEdit(mw.onCellEdit)
	mw.resultTable.SetOnRightClick(mw.onResultTableRightClick)
	mw.rebuildAppTabs()
}

func (mw *MainWindow) treeChildUIDs(id widget.TreeNodeID) []widget.TreeNodeID {
	kind, parts := parseNodeID(id)
	switch kind {
	case "root":
		var ids []widget.TreeNodeID
		for _, f := range mw.folders {
			ids = append(ids, "folder:"+f.ID)
		}
		for _, c := range mw.connections {
			if c.FolderID == "" {
				ids = append(ids, "conn:"+c.ID)
			}
		}
		return ids
	case "folder":
		folderID := parts[0]
		var ids []widget.TreeNodeID
		for _, c := range mw.connections {
			if c.FolderID == folderID {
				ids = append(ids, "conn:"+c.ID)
			}
		}
		return ids
	case "conn":
		connID := parts[0]
		cache, ok := mw.dbCache[connID]
		if !ok || !cache.loaded {
			return nil
		}
		var ids []widget.TreeNodeID
		if len(cache.clusters) > 0 {
			ids = append(ids, "clusters:"+connID)
		}
		for _, db := range cache.databases {
			ids = append(ids, "db:"+connID+":"+db)
		}
		return ids
	case "clusters":
		connID := parts[0]
		cache, ok := mw.dbCache[connID]
		if !ok {
			return nil
		}
		var ids []widget.TreeNodeID
		for _, cl := range cache.clusters {
			ids = append(ids, "cluster_node:"+connID+":"+cl.Cluster+":"+cl.HostName)
		}
		return ids
	case "db":
		connID := parts[0]
		dbName := parts[1]
		cache, ok := mw.dbCache[connID]
		if !ok {
			return nil
		}
		var ids []widget.TreeNodeID
		for _, t := range cache.tables[dbName] {
			ids = append(ids, "tbl:"+connID+":"+dbName+":"+t.Name)
		}
		for _, v := range cache.views[dbName] {
			ids = append(ids, "view:"+connID+":"+dbName+":"+v.Name)
		}
		return ids
	case "cluster_node", "tbl", "view":
		return nil
	}
	return nil
}

func (mw *MainWindow) treeIsBranch(id widget.TreeNodeID) bool {
	kind, _ := parseNodeID(id)
	switch kind {
	case "root", "folder", "conn", "clusters", "db":
		return true
	}
	return false
}

func (mw *MainWindow) treeUpdateNode(id widget.TreeNodeID, branch bool, o fyne.CanvasObject) {
	label := o.(*widget.Label)
	kind, parts := parseNodeID(id)

	switch kind {
	case "root":
		label.SetText("所有连接")
	case "folder":
		for _, f := range mw.folders {
			if f.ID == parts[0] {
				label.SetText("[文件夹] " + f.Name)
				return
			}
		}
		label.SetText("[文件夹] " + parts[0])
	case "conn":
		for _, c := range mw.connections {
			if c.ID == parts[0] {
				prefix := "🔗 "
				if mw.activeConnID == c.ID {
					prefix = "🟢 "
				}
				label.SetText(prefix + c.Name + " (" + c.Host + ")")
				return
			}
		}
		label.SetText("🔗 " + parts[0])
	case "clusters":
		label.SetText("🗄 集群节点")
	case "db":
		label.SetText("📦 " + parts[1])
	case "tbl":
		label.SetText("📋 " + parts[3])
	case "view":
		label.SetText("👁 " + parts[3])
	case "cluster_node":
		label.SetText("🖥 " + parts[3])
	default:
		label.SetText(string(id))
	}
}

func (mw *MainWindow) onBranchOpened(id widget.TreeNodeID) {
	kind, parts := parseNodeID(id)
	if kind == "conn" {
		connID := parts[0]
		if _, ok := mw.dbCache[connID]; !ok {
			mw.dbCache[connID] = &dbCacheEntry{
				tables: make(map[string][]clickhouse.TableInfo),
				views:  make(map[string][]clickhouse.TableInfo),
			}
		}
		go mw.loadConnectionMeta(connID)
	}
}

func (mw *MainWindow) loadConnectionMeta(connID string) {
	cache := mw.dbCache[connID]

	dbs, err := Backend().GetDatabases(connID)
	if err != nil {
		fyne.Do(func() {
			mw.status.SetText(fmt.Sprintf("加载数据库失败: %v", err))
		})
		return
	}
	cache.databases = dbs

	clusters, _ := Backend().GetClusters(connID)
	cache.clusters = clusters

	for _, db := range dbs {
		tables, _ := Backend().GetTables(connID, db)
		cache.tables[db] = tables
		views, _ := Backend().GetViews(connID, db)
		cache.views[db] = views
	}
	cache.loaded = true

	fyne.Do(func() {
		mw.sidebar.Refresh()
		mw.status.SetText(fmt.Sprintf("已加载 %d 个数据库", len(dbs)))
	})
}

func (mw *MainWindow) rebuildAppTabs() {
	if len(mw.tabItems) == 0 {
		if len(mw.tabs) == 0 {
			mw.tabs = append(mw.tabs, NewQueryTab(""))
		}
		mw.activeIdx = 0
	}
	var items []*container.TabItem
	for _, t := range mw.tabs {
		title := t.Title
		if t.Executing {
			title = "... " + title
		}
		ti := &tabItemData{
			TabItem: container.NewTabItem(title, container.NewVBox()),
			tab:     t,
		}
		mw.tabItems = append(mw.tabItems, ti)
		items = append(items, ti.TabItem)
	}
	mw.appTabs = container.NewAppTabs(items...)
	mw.appTabs.OnSelected = func(sel *container.TabItem) {
		for i, ti := range mw.tabItems {
			if ti.TabItem == sel {
				mw.onTabSelected(i)
				return
			}
		}
	}
}

func (mw *MainWindow) refreshAppTabs() {
	var items []*container.TabItem
	for _, ti := range mw.tabItems {
		title := ti.tab.Title
		if ti.tab.Executing {
			title = "... " + title
		}
		ti.TabItem.Text = title
		items = append(items, ti.TabItem)
	}
	mw.appTabs.SetItems(items)
}

func (mw *MainWindow) layout() fyne.CanvasObject {
	runBtn := widget.NewButton("▶ 执行 (Ctrl+Enter)", mw.onRunQuery)
	runBtn.Importance = widget.HighImportance
	stopBtn := widget.NewButton("■ 停止 (Esc)", mw.onStopQuery)
	newTabBtn := widget.NewButton("+ 新标签", mw.onNewTab)

	toolbar := container.NewHBox(
		runBtn, stopBtn, newTabBtn,
		layout.NewSpacer(),
		widget.NewLabelWithStyle("Meteor CH Workbench", fyne.TextAlignTrailing, fyne.TextStyle{Bold: true}),
	)

	editorBox := container.NewBorder(
		toolbar, nil, nil, nil,
		mw.editor,
	)

	mw.exporter = NewResultExporter(mw.window)
	exportBtn := widget.NewButton("导出结果", func() {
		if mw.resultTable.Result() != nil {
			mw.exporter.Export(mw.resultTable.Result())
		}
	})

	pageSizeSelect := widget.NewSelect([]string{"50", "100", "200", "500"}, func(s string) {
		fmt.Sscanf(s, "%d", &mw.pageSize)
		mw.pageOffset = 0
	})
	pageSizeSelect.Selected = "100"

	prevBtn := widget.NewButton("上一页", func() {
		if mw.pageOffset >= mw.pageSize {
			mw.pageOffset -= mw.pageSize
			mw.onRunQuery()
		}
	})
	nextBtn := widget.NewButton("下一页", func() {
		mw.pageOffset += mw.pageSize
		mw.onRunQuery()
	})

	paging := container.NewHBox(
		widget.NewLabel("每页:"), pageSizeSelect,
		prevBtn, nextBtn,
	)

	explainBtn := widget.NewButton("EXPLAIN", func() {
		mw.onExplain()
	})

	refreshMenu := fyne.NewMenu("定时刷新",
		fyne.NewMenuItem("每 5 秒", func() { mw.onToggleAutoRefresh(5) }),
		fyne.NewMenuItem("每 10 秒", func() { mw.onToggleAutoRefresh(10) }),
		fyne.NewMenuItem("每 30 秒", func() { mw.onToggleAutoRefresh(30) }),
		fyne.NewMenuItem("停止", func() { mw.onToggleAutoRefresh(0) }),
	)
	refreshBtn := widget.NewButtonWithIcon("定时刷新", nil, func() {})
	refreshBtn.OnTapped = func() {
		widget.ShowPopUpMenuAtPosition(refreshMenu, mw.window.Canvas(), fyne.CurrentApp().Driver().AbsolutePositionForObject(refreshBtn))
	}

	searchEntry := widget.NewEntry()
	searchEntry.SetPlaceHolder("🔍 搜索结果...")
	searchEntry.OnChanged = func(text string) {
		mw.resultTable.SetSearch(text)
		mw.updateStatus()
	}
	searchEntry.Validator = nil

	clearSearchBtn := widget.NewButton("✕", func() {
		searchEntry.SetText("")
		mw.resultTable.SetSearch("")
		mw.updateStatus()
	})

	resultHeader := container.NewHBox(
		widget.NewLabel("查询结果"),
		layout.NewSpacer(),
		widget.NewLabel("搜索:"), searchEntry, clearSearchBtn,
		layout.NewSpacer(),
		paging,
		explainBtn,
		refreshBtn,
		exportBtn,
		mw.status,
	)
	resultBox := container.NewBorder(
		resultHeader, nil, nil, nil,
		mw.resultTable.Widget(),
	)

	mainSplit := container.NewVSplit(editorBox, resultBox)
	mainSplit.Offset = 0.55

	center := container.NewBorder(
		mw.appTabs, nil, nil, nil,
		mainSplit,
	)

	mw.sidePanels = NewSidePanels(mw.window, mw.editor, mw)
	return container.NewHSplit(mw.sidePanels.Content(), center)
}

func (mw *MainWindow) refreshSidebar() {
	data, err := Backend().GetAll()
	if err != nil {
		dialog.ShowError(err, mw.window)
		return
	}
	mw.connections = data.Connections
	mw.folders = data.Folders
	mw.sidebar.Refresh()
}

func (mw *MainWindow) onSidebarSelected(id widget.TreeNodeID) {
	mw.lastSelectedNode = id
	kind, parts := parseNodeID(id)
	switch kind {
	case "conn":
		mw.activeConnID = parts[0]
		mw.status.SetText(fmt.Sprintf("已选择连接: %s", mw.connName(mw.activeConnID)))
		mw.sidebar.Refresh()
	case "tbl", "view":
		connID := parts[0]
		dbName := parts[1]
		objName := parts[3]
		mw.activeConnID = connID
		fullName := fmt.Sprintf("`%s`.`%s`", dbName, objName)
		sqlSelect := fmt.Sprintf("SELECT * FROM %s LIMIT 100", fullName)
		sqlDescribe := fmt.Sprintf("DESCRIBE TABLE %s", fullName)
		dlg := dialog.NewConfirm("表操作", "点击确定执行 SELECT *，取消查看表结构。", func(ok bool) {
			if ok {
				mw.editor.SetText(sqlSelect)
				mw.status.SetText(fmt.Sprintf("SELECT * FROM %s LIMIT 100", fullName))
			} else {
				mw.editor.SetText(sqlDescribe)
				mw.status.SetText(fmt.Sprintf("DESCRIBE TABLE %s", fullName))
			}
		}, mw.window)
		dlg.Show()
		mw.sidebar.Refresh()
	case "db":
		connID := parts[0]
		dbName := parts[1]
		mw.activeConnID = connID
		mw.editor.SetText(fmt.Sprintf("USE `%s`;\nSHOW TABLES FROM `%s`", dbName, dbName))
		mw.status.SetText(fmt.Sprintf("已选择数据库: %s", dbName))
	}
}

func (mw *MainWindow) updateStatus() {
	if mw.resultTable == nil {
		return
	}
	res := mw.resultTable.Result()
	if res == nil {
		mw.status.SetText("就绪")
		return
	}
	total := mw.resultTable.FilteredRowCount()
	actualTotal := len(res.Rows)
	if mw.resultTable.SearchText() != "" {
		mw.status.SetText(fmt.Sprintf("共 %d 行（搜索匹配 %d 行）", actualTotal, total))
	} else {
		mw.status.SetText(fmt.Sprintf("共 %d 行", total))
	}
}

func (mw *MainWindow) onTreeRightClick(id widget.TreeNodeID, pos fyne.Position) {
	kind, parts := parseNodeID(id)

	makeMenu := func() *fyne.Menu {
		switch kind {
		case "conn":
			return fyne.NewMenu("",
				fyne.NewMenuItem("测试连接", func() { mw.onTestConn(parts[0]) }),
				fyne.NewMenuItem("编辑连接", func() { mw.connDialog.showEdit(parts[0]) }),
				fyne.NewMenuItem("删除连接", func() { mw.connDialog.showDelete(parts[0]) }),
			)
		case "folder":
			return fyne.NewMenu("",
				fyne.NewMenuItem("编辑文件夹", func() { mw.connDialog.showEditFolder(parts[0]) }),
				fyne.NewMenuItem("删除文件夹", func() { mw.onDeleteFolder(parts[0]) }),
			)
		case "db":
			connID := parts[0]
			dbName := parts[1]
			return fyne.NewMenu("",
				fyne.NewMenuItem("USE 数据库", func() {
					mw.activeConnID = connID
					mw.editor.SetText(fmt.Sprintf("USE `%s`", dbName))
					mw.status.SetText(fmt.Sprintf("USE `%s`", dbName))
				}),
				fyne.NewMenuItem("查看所有表", func() {
					mw.activeConnID = connID
					mw.editor.SetText(fmt.Sprintf("SHOW TABLES FROM `%s`", dbName))
					mw.onRunQuery()
				}),
			)
		case "tbl":
			connID := parts[0]
			dbName := parts[1]
			tblName := parts[3]
			fullName := fmt.Sprintf("`%s`.`%s`", dbName, tblName)
			return fyne.NewMenu("",
				fyne.NewMenuItem("SELECT * LIMIT 100", func() {
					mw.activeConnID = connID
					mw.editor.SetText(fmt.Sprintf("SELECT * FROM %s LIMIT 100", fullName))
				}),
				fyne.NewMenuItem("DESCRIBE TABLE", func() {
					mw.activeConnID = connID
					mw.editor.SetText(fmt.Sprintf("DESCRIBE TABLE %s", fullName))
				}),
				fyne.NewMenuItem("SHOW CREATE TABLE", func() {
					mw.activeConnID = connID
					mw.editor.SetText(fmt.Sprintf("SHOW CREATE TABLE %s", fullName))
				}),
				fyne.NewMenuItem("TRUNCATE TABLE ⚠️", func() {
					dlg := dialog.NewConfirm("确认危险操作",
						fmt.Sprintf("确定要清空表 %s 的全部数据吗？此操作不可恢复！", fullName),
						func(ok bool) {
							if !ok {
								return
							}
							mw.activeConnID = connID
							mw.editor.SetText(fmt.Sprintf("TRUNCATE TABLE %s", fullName))
							mw.onRunQuery()
						}, mw.window)
					dlg.SetDismissText("取消")
					dlg.Show()
				}),
				fyne.NewMenuItem("DROP TABLE ⚠️", func() {
					dlg := dialog.NewConfirm("确认危险操作",
						fmt.Sprintf("确定要删除表 %s 吗？此操作不可恢复！", fullName),
						func(ok bool) {
							if !ok {
								return
							}
							mw.activeConnID = connID
							mw.editor.SetText(fmt.Sprintf("DROP TABLE IF EXISTS %s", fullName))
							mw.onRunQuery()
						}, mw.window)
					dlg.SetDismissText("取消")
					dlg.Show()
				}),
			)
		case "view":
			connID := parts[0]
			dbName := parts[1]
			viewName := parts[3]
			fullName := fmt.Sprintf("`%s`.`%s`", dbName, viewName)
			return fyne.NewMenu("",
				fyne.NewMenuItem("SELECT * LIMIT 100", func() {
					mw.activeConnID = connID
					mw.editor.SetText(fmt.Sprintf("SELECT * FROM %s LIMIT 100", fullName))
				}),
				fyne.NewMenuItem("DESCRIBE VIEW", func() {
					mw.activeConnID = connID
					mw.editor.SetText(fmt.Sprintf("DESCRIBE TABLE %s", fullName))
				}),
				fyne.NewMenuItem("SHOW CREATE VIEW", func() {
					mw.activeConnID = connID
					mw.editor.SetText(fmt.Sprintf("SHOW CREATE TABLE %s", fullName))
				}),
			)
		default:
			return nil
		}
	}

	menu := makeMenu()
	if menu != nil {
		widget.ShowPopUpMenuAtPosition(menu, mw.window.Canvas(), pos)
	}
}

func (mw *MainWindow) onTestConn(connID string) {
	conn, err := Backend().GetConnection(connID)
	if err != nil || conn == nil {
		dialog.ShowError(fmt.Errorf("无法获取连接配置: %w", err), mw.window)
		return
	}
	err = Backend().TestConnection(*conn)
	if err != nil {
		dialog.ShowError(fmt.Errorf("连接失败: %w", err), mw.window)
	} else {
		dialog.ShowInformation("测试连接", fmt.Sprintf("连接成功！\n连接: %s", mw.connName(connID)), mw.window)
	}
}

func (mw *MainWindow) onDeleteFolder(folderID string) {
	dlg := dialog.NewConfirm("删除文件夹", "确定要删除此文件夹吗？文件夹内的连接不会被删除。", func(ok bool) {
		if !ok {
			return
		}
		err := Backend().DeleteFolder(folderID)
		if err != nil {
			dialog.ShowError(err, mw.window)
			return
		}
		mw.refreshSidebar()
	}, mw.window)
	dlg.SetDismissText("取消")
	dlg.Show()
}

func (mw *MainWindow) connName(id string) string {
	for _, c := range mw.connections {
		if c.ID == id {
			return c.Name
		}
	}
	return id
}

func (mw *MainWindow) onRunQuery() {
	if mw.activeConnID == "" {
		dialog.ShowInformation("提示", "请先在左侧选择一个连接", mw.window)
		return
	}
	if mw.activeIdx < 0 || mw.activeIdx >= len(mw.tabs) {
		return
	}
	tab := mw.tabs[mw.activeIdx]
	if tab.Executing {
		return
	}

	sql := mw.editor.Text
	if sql == "" {
		return
	}

	tab.SQL = sql
	tab.QueryID = fmt.Sprintf("q-%d", time.Now().UnixNano())
	tab.Executing = true
	mw.refreshAppTabs()
	mw.status.SetText("执行中...")
	mw.resultTable.Clear()
	go mw.runQueryInTab(tab)
}

func (mw *MainWindow) runQueryInTab(tab *QueryTab) {
	sql := tab.SQL
	if looksLikeSelect(sql) {
		sql = appendLimitOffset(sql, mw.pageSize, mw.pageOffset)
	}
	res, err := Backend().Query(mw.activeConnID, sql, tab.QueryID)
	fyne.Do(func() {
		tab.Executing = false
		mw.refreshAppTabs()
		if err != nil {
			dialog.ShowError(err, mw.window)
			mw.status.SetText("执行失败")
			return
		}
		tab.Result = res
		if res.Error != "" {
			mw.status.SetText(fmt.Sprintf("错误: %s", res.Error))
		} else {
			mw.status.SetText(fmt.Sprintf("耗时 %.1fms  行数 %d", res.ElapsedMs, len(res.Rows)))
			mw.resultTable.SetResult(res)
		}
		mw.saveTabs()
	})
}

func (mw *MainWindow) onStopQuery() {
	if mw.activeIdx < 0 || mw.activeIdx >= len(mw.tabs) {
		return
	}
	tab := mw.tabs[mw.activeIdx]
	if tab.QueryID == "" {
		return
	}
	if Backend().CancelQuery(tab.QueryID) {
		mw.status.SetText("正在取消...")
	}
}

func (mw *MainWindow) onExplain() {
	sql := strings.TrimSpace(mw.editor.Text)
	if sql == "" {
		dialog.ShowInformation("提示", "请先输入 SQL", mw.window)
		return
	}
	upper := strings.ToUpper(sql)
	if strings.HasPrefix(upper, "EXPLAIN") {
		sql = strings.TrimSpace(sql[len("EXPLAIN"):])
	}
	explainSQL := "EXPLAIN " + sql

	tab := mw.tabs[mw.activeIdx]
	if tab.Executing {
		return
	}
	tab.SQL = explainSQL
	tab.QueryID = fmt.Sprintf("q-%d", time.Now().UnixNano())
	tab.Executing = true
	mw.refreshAppTabs()
	mw.status.SetText("EXPLAIN 执行中...")
	mw.resultTable.Clear()
	go mw.runQueryInTab(tab)
}

func (mw *MainWindow) onToggleAutoRefresh(intervalSec int) {
	if mw.autoRefresh != nil {
		mw.refreshStop <- struct{}{}
		mw.autoRefresh.Stop()
		mw.autoRefresh = nil
		mw.status.SetText("定时刷新已停止")
		return
	}
	if intervalSec <= 0 {
		return
	}
	mw.refreshStop = make(chan struct{})
	mw.autoRefresh = time.NewTicker(time.Duration(intervalSec) * time.Second)
	mw.status.SetText(fmt.Sprintf("定时刷新已启用（每 %d 秒）", intervalSec))
	go func() {
		for {
			select {
			case <-mw.refreshStop:
				return
			case <-mw.autoRefresh.C:
				fyne.Do(func() { mw.onRunQuery() })
			}
		}
	}()
}

func (mw *MainWindow) onCellEdit(col, row int) {
	res := mw.resultTable.Result()
	if res == nil || row < 0 || row >= len(res.Rows) || col < 0 || col >= len(res.Columns) {
		return
	}
	tableEntry := widget.NewEntry()
	tableEntry.SetText(mw.guessTargetTable())

	var oldVal string
	if row < len(res.Rows) && col < len(res.Rows[row]) {
		oldVal = fmt.Sprintf("%v", res.Rows[row][col])
	}
	valEntry := widget.NewEntry()
	valEntry.SetText(oldVal)

	typeEntry := widget.NewSelect([]string{"String", "Number", "DateTime"}, func(_ string) {})
	typeEntry.Selected = "String"

	colLabel := res.Columns[col].Name

	content := container.NewVBox(
		widget.NewLabel(fmt.Sprintf("修改 %s", colLabel)),
		widget.NewForm(
			&widget.FormItem{Text: "目标表", Widget: tableEntry},
			&widget.FormItem{Text: "新值", Widget: valEntry},
			&widget.FormItem{Text: "值类型", Widget: typeEntry},
		),
	)

	dlg := dialog.NewCustom("生成 ALTER UPDATE", "生成 SQL", content, mw.window)
	dlg.Resize(fyne.NewSize(420, 240))
	dlg.SetOnClosed(func() {
		if tableEntry.Text == "" {
			return
		}
		newVal := valEntry.Text
		switch typeEntry.Selected {
		case "Number":
			mw.editor.SetText(fmt.Sprintf(
				"ALTER TABLE %s UPDATE %s = %v WHERE %s = %s",
				tableEntry.Text, colLabel, newVal, colLabel, oldVal,
			))
		case "DateTime":
			mw.editor.SetText(fmt.Sprintf(
				"ALTER TABLE %s UPDATE %s = toDateTime('%s') WHERE %s = '%s'",
				tableEntry.Text, colLabel, newVal, colLabel, oldVal,
			))
		default:
			mw.editor.SetText(fmt.Sprintf(
				"ALTER TABLE %s UPDATE %s = '%s' WHERE %s = '%s'",
				tableEntry.Text, colLabel, newVal, colLabel, oldVal,
			))
		}
	})
	dlg.Show()
}

func (mw *MainWindow) onResultTableRightClick(col, row int, pos fyne.Position) {
	res := mw.resultTable.Result()
	if res == nil {
		return
	}
	if row < 0 || row >= len(res.Rows) || col < 0 || col >= len(res.Columns) {
		return
	}

	cellVal := cellToString(res.Rows[row][col])
	rowTSV := rowToTSV(res.Rows[row])
	tableName := mw.guessTargetTable()
	insertSQL := buildInsertSQL(res.Columns, res.Rows[row], tableName)

	clipboard := mw.window.Clipboard()

	menu := fyne.NewMenu("",
		fyne.NewMenuItem("复制单元格", func() {
			clipboard.SetContent(cellVal)
		}),
		fyne.NewMenuItem("复制为 SQL 字面量", func() {
			clipboard.SetContent(sqlLiteral(res.Rows[row][col]))
		}),
		fyne.NewMenuItemSeparator(),
		fyne.NewMenuItem("复制整行 (TSV)", func() {
			clipboard.SetContent(rowTSV)
		}),
		fyne.NewMenuItem("复制整行 (INSERT)", func() {
			clipboard.SetContent(insertSQL)
		}),
		fyne.NewMenuItemSeparator(),
		fyne.NewMenuItem(fmt.Sprintf("在编辑器生成 UPDATE（%s）", res.Columns[col].Name), func() {
			colLabel := res.Columns[col].Name
			oldLiteral := sqlLiteral(res.Rows[row][col])
			var whereParts []string
			for ci, cv := range res.Rows[row] {
				colName := res.Columns[ci].Name
				whereParts = append(whereParts, fmt.Sprintf("`%s` = %s", colName, sqlLiteral(cv)))
			}
			sql := fmt.Sprintf("ALTER TABLE `%s` UPDATE `%s` = <新值> WHERE %s", tableName, colLabel, strings.Join(whereParts, " AND "))
			_ = oldLiteral
			mw.editor.SetText(sql)
		}),
	)

	widget.ShowPopUpMenuAtPosition(menu, mw.window.Canvas(), pos)
}

func (mw *MainWindow) guessTargetTable() string {
	if mw.activeIdx < 0 || mw.activeIdx >= len(mw.tabs) {
		return ""
	}
	sql := strings.ToUpper(strings.TrimSpace(mw.tabs[mw.activeIdx].SQL))
	idx := strings.Index(sql, "FROM ")
	if idx < 0 {
		idx = strings.Index(sql, "JOIN ")
	}
	if idx < 0 {
		return ""
	}
	rest := mw.tabs[mw.activeIdx].SQL[idx+5:]
	rest = strings.TrimSpace(rest)
	end := len(rest)
	for i, c := range rest {
		if c == ' ' || c == '\n' || c == '\r' || c == ';' {
			end = i
			break
		}
	}
	if end == 0 {
		return ""
	}
	return rest[:end]
}

func (mw *MainWindow) onNewTab() {
	newTab := NewQueryTab("")
	mw.tabs = append(mw.tabs, newTab)
	ti := &tabItemData{
		TabItem: container.NewTabItem(newTab.Title, container.NewVBox()),
		tab:     newTab,
	}
	mw.tabItems = append(mw.tabItems, ti)
	mw.refreshAppTabs()
	mw.appTabs.SelectIndex(len(mw.tabs) - 1)
}

func (mw *MainWindow) onTabSelected(idx int) {
	if idx == mw.activeIdx {
		return
	}
	if mw.activeIdx >= 0 && mw.activeIdx < len(mw.tabs) {
		mw.tabs[mw.activeIdx].SQL = mw.editor.Text
	}
	mw.activeIdx = idx
	mw.restoreActiveTab()
	mw.saveTabs()
}

func (mw *MainWindow) restoreActiveTab() {
	if mw.activeIdx < 0 || mw.activeIdx >= len(mw.tabs) {
		return
	}
	tab := mw.tabs[mw.activeIdx]
	mw.editor.SetText(tab.SQL)
	if tab.Result != nil {
		mw.resultTable.SetResult(tab.Result)
		mw.status.SetText(fmt.Sprintf("耗时 %.1fms  行数 %d", tab.Result.ElapsedMs, len(tab.Result.Rows)))
	} else {
		mw.resultTable.Clear()
		mw.status.SetText("就绪")
	}
}

func (mw *MainWindow) loadTabs() {
	tf, err := Backend().GetTabs()
	if err != nil || len(tf.Tabs) == 0 {
		mw.tabs = []*QueryTab{NewQueryTab("-- Ctrl+Enter 执行查询\nSELECT 1")}
		mw.activeIdx = 0
		return
	}
	mw.tabs, mw.activeIdx = tabsFromConfig(tf)
	if mw.activeIdx >= len(mw.tabs) {
		mw.activeIdx = 0
	}
}

func (mw *MainWindow) saveTabs() {
	if mw.activeIdx >= 0 && mw.activeIdx < len(mw.tabs) {
		mw.tabs[mw.activeIdx].SQL = mw.editor.Text
	}
	tf := tabsToConfig(mw.tabs, mw.activeIdx)
	_ = Backend().SaveTabs(tf)
}

func (mw *MainWindow) onNewConnection() {
	mw.connDialog.showNew()
}

func (mw *MainWindow) onEditConnection(connID string) {
	mw.connDialog.showEdit(connID)
}