package app

import (
	"os"
	"os/exec"
	"runtime"
	"strings"
	"time"

	"github.com/yuwen002/meteor-ch-workbench/internal/clickhouse"
	"github.com/yuwen002/meteor-ch-workbench/internal/config"
)

type App struct {
	conf  *config.Manager
	hist  *config.HistoryManager
	tabs  *config.TabsManager
	saved *config.SavedSQLManager
	chSvc *clickhouse.Service
	prefs *config.SettingsManager
}

func New() *App {
	conf := config.NewManager()
	return &App{
		conf:  conf,
		hist:  config.NewHistoryManager(conf.ConfigDir()),
		tabs:  config.NewTabsManager(conf.ConfigDir()),
		saved: config.NewSavedSQLManager(conf.ConfigDir()),
		prefs: config.NewSettingsManager(conf.ConfigDir()),
		chSvc: clickhouse.NewService(),
	}
}

func (a *App) Shutdown() { a.chSvc.CloseAll() }

func (a *App) GetAll() (*config.DataFile, error) { return a.conf.LoadAll() }
func (a *App) GetConnections() ([]config.Connection, error) {
	return a.conf.ListConnections()
}
func (a *App) SaveConnection(conn config.Connection) error {
	if err := a.conf.SaveConnection(conn); err != nil {
		return err
	}
	a.chSvc.Invalidate(conn.ID)
	return nil
}
func (a *App) DeleteConnection(id string) error {
	if err := a.conf.DeleteConnection(id); err != nil {
		return err
	}
	a.chSvc.Invalidate(id)
	return nil
}
func (a *App) GetConnection(id string) (*config.Connection, error) { return a.conf.GetConnection(id) }
func (a *App) TestConnection(conn config.Connection) error         { return a.chSvc.TestConnection(conn) }
func (a *App) Disconnect(connectionID string)                       { a.chSvc.Invalidate(connectionID) }

func (a *App) GetFolders() ([]config.Folder, error) { return a.conf.ListFolders() }
func (a *App) SaveFolder(f config.Folder) error    { return a.conf.SaveFolder(f) }
func (a *App) DeleteFolder(id string) error         { return a.conf.DeleteFolder(id) }
func (a *App) ReorderFolders(ids []string) error    { return a.conf.ReorderFolders(ids) }
func (a *App) ReorderConnections(ids []string) error {
	return a.conf.ReorderConnections(ids)
}

func (a *App) ExportConfig() (string, error)            { return a.conf.ExportData() }
func (a *App) GetExportFormats() []config.ExportFormat  { return config.ExportFormats() }
func (a *App) ExportConfigByFormat(format string, includePassword bool) (string, string, error) {
	return a.conf.ExportByFormat(format, includePassword)
}
func (a *App) WriteExportToPath(format string, includePassword bool, path string) error {
	content, _, err := a.conf.ExportByFormat(format, includePassword)
	if err != nil {
		return err
	}
	return os.WriteFile(path, []byte(content), 0o644)
}

func (a *App) ImportConfig(raw string) error           { return a.conf.ImportData(raw) }
func (a *App) GetImportFormats() []config.ImportFormat { return config.ImportFormats() }
func (a *App) ReadFile(path string) (string, string, error) {
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
func (a *App) ImportConfigWithOptions(raw string, opts config.ImportOptions) (config.ImportResult, error) {
	return a.conf.ImportDataWithOptions(raw, opts)
}

func (a *App) Query(connectionID string, sql string, queryID string) (*clickhouse.QueryResult, error) {
	conn, err := a.conf.GetConnection(connectionID)
	if err != nil {
		return nil, err
	}
	res, err := a.chSvc.QueryWithLimit(*conn, sql, queryID, a.prefs.RowLimit())
	if err != nil {
		return nil, err
	}
	a.recordHistory(connectionID, sql, res.ElapsedMs, res.Error == "")
	return res, nil
}
func (a *App) CancelQuery(queryID string) bool { return a.chSvc.CancelQuery(queryID) }

func (a *App) Execute(connectionID string, sql string) (*clickhouse.ExecResult, error) {
	conn, err := a.conf.GetConnection(connectionID)
	if err != nil {
		return nil, err
	}
	return a.chSvc.Exec(*conn, sql)
}
func (a *App) ExecuteBatch(connectionID string, statements []string) (*clickhouse.ExecBatchResult, error) {
	conn, err := a.conf.GetConnection(connectionID)
	if err != nil {
		return nil, err
	}
	return a.chSvc.ExecBatch(*conn, statements, true)
}

func (a *App) GetResultExportFormats() []clickhouse.ResultExportFormat {
	return clickhouse.ResultExportFormats()
}
func (a *App) WriteResultExportToPath(format string, columns []string, rows [][]interface{}, target string, path string) error {
	file, err := os.Create(path)
	if err != nil {
		return err
	}
	_, writeErr := clickhouse.ExportResultTo(file, format, columns, rows, target)
	closeErr := file.Close()
	if writeErr != nil {
		return writeErr
	}
	return closeErr
}

func (a *App) GetDatabases(connectionID string) ([]string, error) {
	conn, err := a.conf.GetConnection(connectionID)
	if err != nil {
		return nil, err
	}
	return a.chSvc.GetDatabases(*conn)
}
func (a *App) GetTables(connectionID string, database string) ([]clickhouse.TableInfo, error) {
	conn, err := a.conf.GetConnection(connectionID)
	if err != nil {
		return nil, err
	}
	return a.chSvc.GetTables(*conn, database)
}
func (a *App) GetViews(connectionID string, database string) ([]clickhouse.TableInfo, error) {
	conn, err := a.conf.GetConnection(connectionID)
	if err != nil {
		return nil, err
	}
	return a.chSvc.GetViews(*conn, database)
}
func (a *App) GetTableSchema(connectionID string, database string, table string) ([]clickhouse.ColumnInfo, error) {
	conn, err := a.conf.GetConnection(connectionID)
	if err != nil {
		return nil, err
	}
	return a.chSvc.GetTableSchema(*conn, database, table)
}
func (a *App) GetClusters(connectionID string) ([]clickhouse.ClusterInfo, error) {
	conn, err := a.conf.GetConnection(connectionID)
	if err != nil {
		return nil, err
	}
	return a.chSvc.GetClusters(*conn)
}

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

func (a *App) GetQueryHistory() ([]config.HistoryEntry, error) { return a.hist.List() }
func (a *App) ClearQueryHistory() error                       { return a.hist.Clear() }
func (a *App) DeleteQueryHistory(id string) error              { return a.hist.Delete(id) }

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

func (a *App) GetSavedSQL() ([]config.SavedSQL, error) { return a.saved.List() }
func (a *App) SaveSQL(name string, sql string, connectionID string) (*config.SavedSQL, error) {
	item, err := a.saved.Add(config.SavedSQL{Name: name, SQL: sql, ConnectionID: connectionID})
	if err != nil {
		return nil, err
	}
	return &item, nil
}
func (a *App) UpdateSavedSQL(id string, name string, sql string) (*config.SavedSQL, error) {
	item, err := a.saved.Update(id, name, sql)
	if err != nil {
		return nil, err
	}
	return &item, nil
}
func (a *App) DeleteSavedSQL(id string) error { return a.saved.Delete(id) }

func (a *App) GetTabs() (config.TabsFile, error)        { return a.tabs.Load() }
func (a *App) SaveTabs(snapshot config.TabsFile) error  { return a.tabs.Save(snapshot) }
func (a *App) GetSettings() (config.SettingsFile, error) { return a.prefs.Load() }
func (a *App) SaveSettings(settings config.SettingsFile) (config.SettingsFile, error) {
	if err := a.prefs.Save(settings); err != nil {
		return config.SettingsFile{}, err
	}
	return a.prefs.Load()
}

func (a *App) GetAppVersion() string { return "0.2.0-fyne" }