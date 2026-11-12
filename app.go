package main

import (
	"context"
	"os/exec"
	"runtime"

	"github.com/yuwen002/meteor-ch-workbench/internal/clickhouse"
	"github.com/yuwen002/meteor-ch-workbench/internal/config"
)

type App struct {
	ctx   context.Context
	conf  *config.Manager
	chSvc *clickhouse.Service
}

func NewApp() *App {
	return &App{
		conf:  config.NewManager(),
		chSvc: clickhouse.NewService(),
	}
}

func (a *App) OnStartup(ctx context.Context) {
	a.ctx = ctx
}

func (a *App) OnShutdown(ctx context.Context) {
	a.chSvc.CloseAll()
}

func (a *App) GetAll() (*config.DataFile, error) {
	return a.conf.LoadAll()
}

func (a *App) GetConnections() ([]config.Connection, error) {
	return a.conf.ListConnections()
}

func (a *App) SaveConnection(conn config.Connection) error {
	return a.conf.SaveConnection(conn)
}

func (a *App) DeleteConnection(id string) error {
	return a.conf.DeleteConnection(id)
}

func (a *App) GetConnection(id string) (*config.Connection, error) {
	return a.conf.GetConnection(id)
}

func (a *App) TestConnection(conn config.Connection) error {
	return a.chSvc.TestConnection(conn)
}

func (a *App) GetFolders() ([]config.Folder, error) {
	return a.conf.ListFolders()
}

func (a *App) SaveFolder(f config.Folder) error {
	return a.conf.SaveFolder(f)
}

func (a *App) DeleteFolder(id string) error {
	return a.conf.DeleteFolder(id)
}

func (a *App) Query(connectionID string, sql string) (*clickhouse.QueryResult, error) {
	conn, err := a.conf.GetConnection(connectionID)
	if err != nil {
		return nil, err
	}
	return a.chSvc.Query(*conn, sql)
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

func (a *App) GetAppVersion() string {
	return "0.1.0"
}