package main

import (
	"context"
	"os"
	"os/exec"
	"runtime"
	"strings"

	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/yuwen002/meteor-ch-workbench/internal/clickhouse"
	"github.com/yuwen002/meteor-ch-workbench/internal/config"
)

// App 是暴露给前端的 Wails 服务对象。
// 它组合了配置管理器（负责连接与文件夹的读写）和 ClickHouse 服务（负责真正的查询交互），
// 前端通过绑定生成的 JS 方法调用这里导出的每一个公开方法。
type App struct {
	ctx   context.Context     // Wails 注入的应用上下文，随应用生命周期结束而关闭
	conf  *config.Manager     // 配置管理器，负责 connections.json 的读写
	chSvc *clickhouse.Service // ClickHouse 连接池与查询服务
}

// NewApp 创建 App 实例，并初始化配置管理器和 ClickHouse 服务。
// 此时尚未连接数据库，连接是在首次调用查询类方法时惰性建立的。
func NewApp() *App {
	return &App{
		conf:  config.NewManager(),
		chSvc: clickhouse.NewService(),
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
	return a.conf.LoadAll()
}

// GetConnections 返回所有已保存的连接配置。
func (a *App) GetConnections() ([]config.Connection, error) {
	return a.conf.ListConnections()
}

// SaveConnection 保存（新增或更新）一条连接配置。
// 保存成功后会使该连接对应的旧连接池失效，确保下次使用新参数重新建连。
func (a *App) SaveConnection(conn config.Connection) error {
	if err := a.conf.SaveConnection(conn); err != nil {
		return err
	}
	a.chSvc.Invalidate(conn.ID)
	return nil
}

// DeleteConnection 删除指定 ID 的连接配置，并同步释放其连接池。
func (a *App) DeleteConnection(id string) error {
	if err := a.conf.DeleteConnection(id); err != nil {
		return err
	}
	a.chSvc.Invalidate(id)
	return nil
}

// GetConnection 按 ID 读取单条连接配置，不存在时返回错误。
func (a *App) GetConnection(id string) (*config.Connection, error) {
	return a.conf.GetConnection(id)
}

// TestConnection 使用传入的（可能是尚未保存的）连接参数尝试建立连接并 Ping 一次。
// 用于弹窗中的「测试连接」按钮，失败时错误信息会直接返回给前端展示。
func (a *App) TestConnection(conn config.Connection) error {
	return a.chSvc.TestConnection(conn)
}

// Disconnect 断开指定连接的连接池，释放所有底层 TCP 连接。
// 下次再查询时会自动重新建连。
func (a *App) Disconnect(connectionID string) {
	a.chSvc.Invalidate(connectionID)
}

// GetFolders 返回所有连接文件夹（侧边栏分组）。
func (a *App) GetFolders() ([]config.Folder, error) {
	return a.conf.ListFolders()
}

// SaveFolder 保存（新增或更新）一个文件夹分组。
func (a *App) SaveFolder(f config.Folder) error {
	return a.conf.SaveFolder(f)
}

// DeleteFolder 删除文件夹分组，其下的连接会被移动到根级（不会删除连接本身）。
func (a *App) DeleteFolder(id string) error {
	return a.conf.DeleteFolder(id)
}

// ReorderFolders 按指定 ID 顺序重排文件夹分组，用于侧边栏拖拽排序。
func (a *App) ReorderFolders(ids []string) error {
	return a.conf.ReorderFolders(ids)
}

// ReorderConnections 按指定 ID 顺序重排连接列表，用于侧边栏拖拽排序。
func (a *App) ReorderConnections(ids []string) error {
	return a.conf.ReorderConnections(ids)
}

// ExportConfig 导出当前全部配置为 JSON 文本，供前端下载为配置文件。
func (a *App) ExportConfig() (string, error) {
	return a.conf.ExportData()
}

// GetExportFormats 返回支持的全部导出格式元信息，供前端渲染格式选择。
func (a *App) GetExportFormats() []config.ExportFormat {
	return config.ExportFormats()
}

// ExportConfigByFormat 按指定格式导出配置文本，返回内容、建议扩展名。
// includePassword 为 false 时会清空密码，便于安全分享。
func (a *App) ExportConfigByFormat(format string, includePassword bool) (string, string, error) {
	return a.conf.ExportByFormat(format, includePassword)
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
	return a.conf.ImportData(raw)
}

// GetImportFormats 返回支持的全部导入格式元信息。
func (a *App) GetImportFormats() []config.ImportFormat {
	return config.ImportFormats()
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
	return a.conf.ImportDataWithOptions(raw, opts)
}

// Query 在指定连接上执行一段 SQL，返回列名与全部结果行。
// SQL 执行失败（如语法错误）不会返回 error，而是把错误信息放在 QueryResult.Error 中，
// 这样前端仍能正常渲染结果面板并展示错误文本。
func (a *App) Query(connectionID string, sql string) (*clickhouse.QueryResult, error) {
	conn, err := a.conf.GetConnection(connectionID)
	if err != nil {
		return nil, err
	}
	return a.chSvc.Query(*conn, sql)
}

// Execute 在指定连接上执行一条 DDL/DML 语句（CREATE/ALTER/DROP/INSERT 等）。
// SQL 执行失败不会返回 error，而是把错误信息放在 ExecResult.Error 中。
func (a *App) Execute(connectionID string, sql string) (*clickhouse.ExecResult, error) {
	conn, err := a.conf.GetConnection(connectionID)
	if err != nil {
		return nil, err
	}
	return a.chSvc.Exec(*conn, sql)
}

// GetDatabases 查询指定连接上的数据库列表（SHOW DATABASES），用于侧边栏自动展开。
func (a *App) GetDatabases(connectionID string) ([]string, error) {
	conn, err := a.conf.GetConnection(connectionID)
	if err != nil {
		return nil, err
	}
	return a.chSvc.GetDatabases(*conn)
}

// GetTables 查询指定数据库下的所有表（名称、引擎、注释）。
func (a *App) GetTables(connectionID string, database string) ([]clickhouse.TableInfo, error) {
	conn, err := a.conf.GetConnection(connectionID)
	if err != nil {
		return nil, err
	}
	return a.chSvc.GetTables(*conn, database)
}

// GetViews 查询指定数据库下的所有视图信息（名称、引擎、注释）。
func (a *App) GetViews(connectionID string, database string) ([]clickhouse.TableInfo, error) {
	conn, err := a.conf.GetConnection(connectionID)
	if err != nil {
		return nil, err
	}
	return a.chSvc.GetViews(*conn, database)
}

// GetTableSchema 查询指定表的列结构（列名、类型、注释）。
func (a *App) GetTableSchema(connectionID string, database string, table string) ([]clickhouse.ColumnInfo, error) {
	conn, err := a.conf.GetConnection(connectionID)
	if err != nil {
		return nil, err
	}
	return a.chSvc.GetTableSchema(*conn, database, table)
}

// GetClusters 查询 system.clusters 中的集群拓扑信息（分片、副本、主机地址等）。
func (a *App) GetClusters(connectionID string) ([]clickhouse.ClusterInfo, error) {
	conn, err := a.conf.GetConnection(connectionID)
	if err != nil {
		return nil, err
	}
	return a.chSvc.GetClusters(*conn)
}

// OpenConfigFolder 用系统文件管理器打开配置文件所在目录。
// 按操作系统选择不同的打开命令：Windows 用 explorer，macOS 用 open，其余用 xdg-open。
func (a *App) OpenConfigFolder() error {
	dir := a.conf.ConfigDir()
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
