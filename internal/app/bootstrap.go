package app

import (
	"github.com/yuwen002/meteor-ch-workbench/internal/clickhouse"
	"github.com/yuwen002/meteor-ch-workbench/internal/config"
)

// NewDefaultService 使用应用默认的本地配置目录装配完整业务服务。
// Wails、Gio 等 GUI 入口应共用此构造函数，避免各自创建不同的配置与连接池实例。
func NewDefaultService() *Service {
	conf := config.NewManager()
	configDir := conf.ConfigDir()

	return NewService(
		conf,
		config.NewHistoryManager(configDir),
		config.NewTabsManager(configDir),
		config.NewSavedSQLManager(configDir),
		clickhouse.NewService(),
		config.NewSettingsManager(configDir),
	)
}
