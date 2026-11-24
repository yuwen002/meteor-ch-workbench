// Package bootstrap 负责把新架构的应用用例与基础设施装配为可供 GUI 使用的服务集合。
package bootstrap

import (
	"errors"
	"os"
	"path/filepath"
	"strings"

	connectionapp "github.com/yuwen002/meteor-ch-workbench/internal/application/connection"
	folderapp "github.com/yuwen002/meteor-ch-workbench/internal/application/folder"
	metadataapp "github.com/yuwen002/meteor-ch-workbench/internal/application/metadata"
	queryapp "github.com/yuwen002/meteor-ch-workbench/internal/application/query"
	chinfra "github.com/yuwen002/meteor-ch-workbench/internal/infrastructure/clickhouse"
	"github.com/yuwen002/meteor-ch-workbench/internal/infrastructure/storage/connectionfile"
)

// Services 是新架构的应用服务容器。GUI 只需依赖这些应用层服务，不直接创建基础设施对象。
type Services struct {
	// ConfigPath 是本次运行实际使用的连接配置文件绝对路径，供界面诊断与用户备份。
	ConfigPath string
	Connections *connectionapp.Service
	Folders *folderapp.Service
	Queries     *queryapp.Service
	Metadata    *metadataapp.Service
	pools       *chinfra.PoolManager
}

// New 使用指定数据目录装配连接、查询和元数据服务。
func New(dataDir string) (*Services, error) {
	dataDir = strings.TrimSpace(dataDir)
	if dataDir == "" {
		return nil, errors.New("应用数据目录不能为空")
	}
	configPath, err := filepath.Abs(filepath.Join(dataDir, "connections.json"))
	if err != nil {
		return nil, err
	}
	repository, err := connectionfile.New(configPath)
	if err != nil {
		return nil, err
	}

	pools := chinfra.NewPoolManager()
	fail := func(err error) (*Services, error) {
		_ = pools.Close()
		return nil, err
	}

	ids := connectionfile.RandomIDGenerator{}
	connections, err := connectionapp.NewService(repository, chinfra.NewTester(0), ids)
	if err != nil {
		return fail(err)
	}
	folders, err := folderapp.NewService(repository, ids)
	if err != nil { return fail(err) }
	executor, err := chinfra.NewQueryExecutor(pools)
	if err != nil {
		return fail(err)
	}
	queries, err := queryapp.NewService(repository, executor)
	if err != nil {
		return fail(err)
	}
	reader, err := chinfra.NewMetadataReader(pools)
	if err != nil {
		return fail(err)
	}
	metadata, err := metadataapp.NewService(repository, reader)
	if err != nil {
		return fail(err)
	}

	return &Services{
		ConfigPath: configPath,
		Connections: connections,
		Folders: folders,
		Queries: queries,
		Metadata: metadata,
		pools: pools,
	}, nil
}

// defaultDataDir 返回稳定的应用配置目录。目录名与旧版应用保持一致，避免升级后丢失已有连接。
func defaultDataDir(configDir string) string {
	return filepath.Join(configDir, "meteor-ch-workbench")
}

// NewDefault 使用操作系统默认用户配置目录。
func NewDefault() (*Services, error) {
	configDir, err := os.UserConfigDir()
	if err != nil {
		return nil, err
	}
	dataDir := defaultDataDir(configDir)
	// 兼容上一轮迁移版本使用的目录名；目标目录始终采用旧应用的稳定路径。
	legacyPath := filepath.Join(configDir, "MeteorCHWorkbench", "connections.json")
	if err := migrateLegacyConnections(dataDir, legacyPath); err != nil {
		return nil, err
	}
	return New(dataDir)
}

// Close 释放所有由服务容器创建的 ClickHouse 连接池。
func (s *Services) Close() error {
	if s == nil || s.pools == nil {
		return nil
	}
	return s.pools.Close()
}
