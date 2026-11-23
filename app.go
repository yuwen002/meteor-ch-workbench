package main

import (
	"context"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"time"

	"github.com/wailsapp/wails/v3/pkg/application"
	appsvc "github.com/yuwen002/meteor-ch-workbench/internal/app"
	"github.com/yuwen002/meteor-ch-workbench/internal/clickhouse"
	"github.com/yuwen002/meteor-ch-workbench/internal/config"
)

// App 是旧版 Wails 前端的适配对象。
// 业务用例逐步委托给 internal/app.Service；本结构暂时保留历史 API 签名，
// 以便原有 Vue 前端在原生 GUI 迁移期间继续工作。
type App struct {
	ctx     context.Context         // Wails 注入的应用上下文，随应用生命周期结束而关闭
	service *appsvc.Service         // 与 GUI 无关的应用服务
	conf  *config.Manager         // 配置管理器，负责 connections.json 的读写
	hist  *config.HistoryManager  // 查询历史管理器，负责 history.json 的读写
	tabs  *config.TabsManager     // 标签页管理器，负责 tabs.json 的读写
	saved *config.SavedSQLManager // 收藏 SQL 管理器，负责 saved_sql.json 的读写
	chSvc *clickhouse.Service     // ClickHouse 连接池与查询服务
	prefs *config.SettingsManager // 用户偏好设置，负责 settings.json 的读写
}

// NewApp 创建 App 实例，并初始化配置管理器、查询历史管理器和 ClickHouse 服务。
// 此时尚未连接数据库，连接是在首次调用查询类方法时惰性建立的。
func NewApp() *App {
	conf := config.NewManager()
	hist := config.NewHistoryManager(conf.ConfigDir())
	tabs := config.NewTabsManager(conf.ConfigDir())
	saved := config.NewSavedSQLManager(conf.ConfigDir())
	prefs := config.NewSettingsManager(conf.ConfigDir())
	chSvc := clickhouse.NewService()
	return &App{
		conf:    conf,
		hist:    hist,
		tabs:    tabs,
		saved:   saved,
		prefs:   prefs,
		chSvc:   chSvc,
		service: appsvc.NewService(conf, hist, tabs, saved, chSvc, prefs),
	}
}

// OnStartup 是 Wails 的生命周期回调，应用启动时被调用。
// 这里仅保存应用上下文，便于后续需要取消或超时控制的场景使用。
func (a *App) OnStartup(ctx context.Context) {
	a.ctx = ctx
}

// OnShutdown 是 Wails 的生命周期回调，应用退出时被调用。
// 负责关闭所有缓存的数据库连接池，避免残留连接与服务端泄漏。
func (a *App) OnShutdown(ctx context.Context) {
	a.chSvc.CloseAll()
}

// GetAll 一次性读取全部配置数据（文件夹 + 连接列表），供前端启动时初始化侧边栏。
func (a *App) GetAll() (*config.DataFile, error) {
	return a.service.GetAll()
}

// GetConnections 返回所有已保存的连接配置。
func (a *App) GetConnections() ([]config.Connection, error) {
	return a.service.GetConnections()
}

// SaveConnection 保存（新增或更新）一条连接配置。
// 保存成功后会使该连接对应的旧连接池失效，确保下次使用新参数重新建连。
func (a *App) SaveConnection(conn config.Connection) error {
	return a.service.SaveConnection(conn)
}

// DeleteConnection 删除指定 ID 的连接配置，并同步释放其连接池。
func (a *App) DeleteConnection(id string) error {
	return a.service.DeleteConnection(id)
}

// GetConnection 按 ID 读取单条连接配置，不存在时返回错误。
func (a *App) GetConnection(id string) (*config.Connection, error) {
	return a.service.GetConnection(id)
}

// TestConnection 使用传入的（可能是尚未保存的）连接参数尝试建立连接并 Ping 一次。
// 用于弹窗中的「测试连接」按钮，失败时错误信息会直接返回给前端展示。
func (a *App) TestConnection(conn config.Connection) error {
	return a.service.TestConnection(conn)
}

// Disconnect 断开指定连接的连接池，释放所有底层 TCP 连接。
// 下次再查询时会自动重新建连。
func (a *App) Disconnect(connectionID string) {
	a.service.Disconnect(connectionID)
}

// GetFolders 返回所有连接文件夹（侧边栏分组）。
func (a *App) GetFolders() ([]config.Folder, error) {
	return a.service.GetFolders()
}

// SaveFolder 保存（新增或更新）一个文件夹分组。
func (a *App) SaveFolder(f config.Folder) error {
	return a.service.SaveFolder(f)
}

// DeleteFolder 删除文件夹分组，其下的连接会被移动到根级（不会删除连接本身）。
func (a *App) DeleteFolder(id string) error {
	return a.service.DeleteFolder(id)
}

// ReorderFolders 按指定 ID 顺序重排文件夹分组，用于侧边栏拖拽排序。
func (a *App) ReorderFolders(ids []string) error {
	return a.service.ReorderFolders(ids)
}

// ReorderConnections 按指定 ID 顺序重排连接列表，用于侧边栏拖拽排序。
func (a *App) ReorderConnections(ids []string) error {
	return a.service.ReorderConnections(ids)
}

// ExportConfig 导出当前全部配置为 JSON 文本，供前端下载为配置文件。
func (a *App) ExportConfig() (string, error) {
	return a.service.ExportConfig()
}

// GetExportFormats 返回支持的全部导出格式元信息，供前端渲染格式选择。
func (a *App) GetExportFormats() []config.ExportFormat {
	return a.service.GetExportFormats()
}

// ExportConfigByFormat 按指定格式导出配置文本，返回内容、建议扩展名。
// includePassword 为 false 时会清空密码，便于安全分享。
func (a *App) ExportConfigByFormat(format string, includePassword bool) (string, string, error) {
	return a.service.ExportConfigByFormat(format, includePassword)
}

// SaveExportAs 弹出系统「另存为」对话框，让用户选择保存位置后写入导出内容。
// 返回实际保存的完整路径；用户取消时返回空字符串。
func (a *App) SaveExportAs(format string, includePassword bool) (string, error) {
	content, ext, err := a.conf.ExportByFormat(format, includePassword)
	if err != nil {
		return "", err
	}
	dialog := application.Get().Dialog.SaveFile()
	dialog.SetMessage("选择配置文件的保存位置")
	dialog.SetFilename("meteor-ch-config." + ext)
	dialog.SetButtonText("保存")
	if ext != "" {
		dialog.AddFilter(strings.ToUpper(ext), "*."+ext)
	}
	path, err := dialog.PromptForSingleSelection()
	if err != nil {
		if strings.Contains(err.Error(), "cancelled") {
			return "", nil
		}
		return "", err
	}
	if path == "" {
		return "", nil
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		return "", err
	}
	return path, nil
}

// ImportConfig 导入 JSON 配置文本，覆盖现有全部连接与文件夹。
func (a *App) ImportConfig(raw string) error {
	return a.service.ImportConfig(raw)
}

// GetImportFormats 返回支持的全部导入格式元信息。
func (a *App) GetImportFormats() []config.ImportFormat {
	return a.service.GetImportFormats()
}

// PickImportFile 弹出系统文件选择对话框，读取所选文件内容并返回文本与文件名。
// 用户取消时返回空字符串，不视为错误。
func (a *App) PickImportFile() (string, string, error) {
	dialog := application.Get().Dialog.OpenFile()
	dialog.SetMessage("选择要导入的配置文件")
	dialog.AddFilter("配置文件", "*.json;*.yaml;*.yml;*.csv;*.txt")
	dialog.AddFilter("所有文件", "*.*")
	path, err := dialog.PromptForSingleSelection()
	if err != nil {
		if strings.Contains(err.Error(), "cancelled") {
			return "", "", nil
		}
		return "", "", err
	}
	if path == "" {
		return "", "", nil
	}
	content, err := os.ReadFile(path)
	if err != nil {
		return "", "", err
	}
	name := path
	if i := strings.LastIndexAny(path, `/\`); i >= 0 {
		name = path[i+1:]
	}
	return string(content), name, nil
}

// ImportConfigWithOptions 按指定格式与模式（覆盖/合并）导入配置文本，返回导入结果概要。
func (a *App) ImportConfigWithOptions(raw string, opts config.ImportOptions) (config.ImportResult, error) {
	return a.service.ImportConfigWithOptions(raw, opts)
}

// Query 在指定连接上执行一段 SQL，返回列名与全部结果行。
// SQL 执行失败（如语法错误）不会返回 error，而是把错误信息放在 QueryResult.Error 中，
// 这样前端仍能正常渲染结果面板并展示错误文本。
//
// queryID 由前端生成并用于取消：调用 CancelQuery(queryID) 可立即中止本次查询，
// 此时返回 QueryResult.Error = "查询已取消"，已读到的部分行被丢弃。
func (a *App) Query(connectionID string, sql string, queryID string) (*clickhouse.QueryResult, error) {
	return a.service.Query(connectionID, sql, queryID)
}

// GetSettings 返回当前用户偏好设置，供前端设置面板回显。
func (a *App) GetSettings() (config.SettingsFile, error) {
	return a.service.GetSettings()
}

// SaveSettings 校验并保存用户偏好设置，返回保存后实际生效的值（可能被收敛到合法区间）。
// SQL 写入失败等基础错误走 Go error；越界输入则被 NormalizeRowLimit 静默修正。
func (a *App) SaveSettings(settings config.SettingsFile) (config.SettingsFile, error) {
	return a.service.SaveSettings(settings)
}

// CancelQuery 取消指定 queryID 正在执行的查询。
// 返回 true 表示查询确实在运行并已被取消；查询已结束时返回 false。
func (a *App) CancelQuery(queryID string) bool {
	return a.service.CancelQuery(queryID)
}

// recordHistory 把一次执行记录追加到查询历史。
// 历史写入失败不影响查询本身的结果，因此错误被静默忽略。
func (a *App) recordHistory(connectionID, sql string, durationMs float64, success bool) {
	if strings.TrimSpace(sql) == "" {
		return
	}
	_ = a.hist.Add(config.HistoryEntry{
		SQL:          sql,
		ConnectionID: connectionID,
		Timestamp:    time.Now(),
		DurationMs:   durationMs,
		Success:      success,
	})
}

// GetQueryHistory 返回全部查询历史，最新的排在最前。
func (a *App) GetQueryHistory() ([]config.HistoryEntry, error) {
	return a.service.GetQueryHistory()
}

// ClearQueryHistory 清空全部查询历史。
func (a *App) ClearQueryHistory() error {
	return a.service.ClearQueryHistory()
}

// DeleteQueryHistory 删除指定 ID 的一条查询历史记录。
func (a *App) DeleteQueryHistory(id string) error {
	return a.service.DeleteQueryHistory(id)
}

// GetSavedSQL 返回全部收藏的 SQL，最近更新的排在最前。
func (a *App) GetSavedSQL() ([]config.SavedSQL, error) {
	return a.service.GetSavedSQL()
}

// SaveSQL 收藏一条 SQL 并返回新建的记录。
// SQL 为空或重复收藏时返回 error，由前端提示用户。
func (a *App) SaveSQL(name string, sql string, connectionID string) (*config.SavedSQL, error) {
	return a.service.SaveSQL(name, sql, connectionID)
}

// UpdateSavedSQL 修改一条收藏记录的标题与 SQL 文本。
func (a *App) UpdateSavedSQL(id string, name string, sql string) (*config.SavedSQL, error) {
	return a.service.UpdateSavedSQL(id, name, sql)
}

// DeleteSavedSQL 删除指定 ID 的一条收藏记录。
func (a *App) DeleteSavedSQL(id string) error {
	return a.service.DeleteSavedSQL(id)
}

// GetTabs 返回上次保存的标签页快照，供前端启动时恢复编辑器工作现场。
func (a *App) GetTabs() (config.TabsFile, error) {
	return a.service.GetTabs()
}

// SaveTabs 保存当前全部标签页与活动标签，供下次启动恢复。
func (a *App) SaveTabs(snapshot config.TabsFile) error {
	return a.service.SaveTabs(snapshot)
}

// Execute 在指定连接上执行一条 DDL/DML 语句（CREATE/ALTER/DROP/INSERT 等）。
// SQL 执行失败不会返回 error，而是把错误信息放在 ExecResult.Error 中。
func (a *App) Execute(connectionID string, sql string) (*clickhouse.ExecResult, error) {
	return a.service.Execute(connectionID, sql)
}

// ExecuteBatch 在指定连接上顺序执行一批 DDL/DML 语句，只做一次跨进程调用。
// 用于批量提交单元格编辑生成的 ALTER ... UPDATE，避免逐条 RPC 的串行等待。
// 采用 continueOnError=true：单条失败不中断，一次收集全部失败点，
// 调用方可据回执精确回填未成功的语句。SQL 错误放在回执中，不返回 error。
func (a *App) ExecuteBatch(connectionID string, statements []string) (*clickhouse.ExecBatchResult, error) {
	return a.service.ExecuteBatch(connectionID, statements)
}

// GetResultExportFormats 返回支持的结果集导出格式元信息，供前端渲染格式选择。
func (a *App) GetResultExportFormats() []clickhouse.ResultExportFormat {
	return clickhouse.ResultExportFormats()
}

// SaveQueryResultAs 弹出系统「另存为」对话框，把结果集按指定格式导出到用户选择的位置。
// target 为目标表名，仅 INSERT 族格式使用（其余格式忽略）。
// 返回实际保存路径；用户取消时返回空字符串。
func (a *App) SaveQueryResultAs(format string, columns []string, rows [][]interface{}, target string) (string, error) {
	dialog := application.Get().Dialog.SaveFile()
	ext := clickhouse.ResultExportExtension(format)
	dialog.SetMessage("选择查询结果的保存位置")
	dialog.SetFilename("query-result." + ext)
	dialog.SetButtonText("保存")
	if ext != "" {
		dialog.AddFilter(strings.ToUpper(ext), "*."+ext)
	}
	path, err := dialog.PromptForSingleSelection()
	if err != nil {
		if strings.Contains(err.Error(), "cancelled") {
			return "", nil
		}
		return "", err
	}
	if path == "" {
		return "", nil
	}
	file, err := os.Create(path)
	if err != nil {
		return "", err
	}
	_, writeErr := clickhouse.ExportResultTo(file, format, columns, rows, target)
	closeErr := file.Close()
	if writeErr != nil {
		return "", writeErr
	}
	if closeErr != nil {
		return "", closeErr
	}
	return path, nil
}

// GetDatabases 查询指定连接上的数据库列表（SHOW DATABASES），用于侧边栏自动展开。
func (a *App) GetDatabases(connectionID string) ([]string, error) {
	return a.service.GetDatabases(connectionID)
}

// GetTables 查询指定数据库下的所有表（名称、引擎、注释）。
func (a *App) GetTables(connectionID string, database string) ([]clickhouse.TableInfo, error) {
	return a.service.GetTables(connectionID, database)
}

// GetViews 查询指定数据库下的所有视图信息（名称、引擎、注释）。
func (a *App) GetViews(connectionID string, database string) ([]clickhouse.TableInfo, error) {
	return a.service.GetViews(connectionID, database)
}

// GetTableSchema 查询指定表的列结构（列名、类型、注释）。
func (a *App) GetTableSchema(connectionID string, database string, table string) ([]clickhouse.ColumnInfo, error) {
	return a.service.GetTableSchema(connectionID, database, table)
}

// GetClusters 查询 system.clusters 中的集群拓扑信息（分片、副本、主机地址等）。
func (a *App) GetClusters(connectionID string) ([]clickhouse.ClusterInfo, error) {
	return a.service.GetClusters(connectionID)
}

// OpenConfigFolder 用系统文件管理器打开配置文件所在目录。
// 按操作系统选择不同的打开命令：Windows 用 explorer，macOS 用 open，其余用 xdg-open。
func (a *App) OpenConfigFolder() error {
	dir := a.service.ConfigDir()
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "windows":
		cmd = exec.Command("explorer", dir)
	case "darwin":
		cmd = exec.Command("open", dir)
	default:
		cmd = exec.Command("xdg-open", dir)
	}
	return cmd.Start()
}

// GetAppVersion 返回当前应用版本号，供前端关于页面展示。
func (a *App) GetAppVersion() string {
	return "0.1.0"
}
