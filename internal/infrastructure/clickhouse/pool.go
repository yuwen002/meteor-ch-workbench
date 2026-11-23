package clickhouse

import (
	"database/sql"
	"errors"
	"fmt"
	"sync"
	"time"

	_ "github.com/ClickHouse/clickhouse-go/v2"
	"github.com/yuwen002/meteor-ch-workbench/internal/domain/connection"
)

const (
	defaultMaxOpenConnections = 5
	defaultMaxIdleConnections = 2
	defaultConnectionLifetime = 30*time.Minute
)

type poolEntry struct {
	db *sql.DB
	fingerprint string
}

// PoolManager 按连接 ID 管理 database/sql 连接池。
// 参数变更时通过配置指纹识别旧池并关闭，指纹本身不包含可逆凭据。
type PoolManager struct {
	mu sync.Mutex
	pools map[string]poolEntry
	open func(string,string)(*sql.DB,error)
}

func NewPoolManager() *PoolManager {
	return &PoolManager{pools:make(map[string]poolEntry),open:sql.Open}
}

func newPoolManager(open func(string,string)(*sql.DB,error)) *PoolManager {
	return &PoolManager{pools:make(map[string]poolEntry),open:open}
}

func (m *PoolManager) Acquire(item connection.Connection) (*sql.DB,error) {
	if item.ID=="" { return nil,connection.ErrMissingID }
	dsn,err:=buildDSN(item)
	if err!=nil { return nil,err }
	fingerprint:=configFingerprint(dsn)
	m.mu.Lock()
	defer m.mu.Unlock()
	if existing,ok:=m.pools[item.ID];ok {
		if existing.fingerprint==fingerprint { return existing.db,nil }
		_ = existing.db.Close()
		delete(m.pools,item.ID)
	}
	db,err:=m.open("clickhouse",dsn)
	if err!=nil { return nil,fmt.Errorf("创建 ClickHouse 连接池（%s）: %w",endpointLabel(item),err) }
	db.SetMaxOpenConns(defaultMaxOpenConnections)
	db.SetMaxIdleConns(defaultMaxIdleConnections)
	db.SetConnMaxLifetime(defaultConnectionLifetime)
	m.pools[item.ID]=poolEntry{db:db,fingerprint:fingerprint}
	return db,nil
}

func (m *PoolManager) Invalidate(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	entry,ok:=m.pools[id]
	if !ok { return nil }
	delete(m.pools,id)
	if err:=entry.db.Close(); err!=nil { return fmt.Errorf("关闭 ClickHouse 连接池: %w",err) }
	return nil
}

func (m *PoolManager) Close() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	var closeErr error
	for id,entry:=range m.pools {
		if err:=entry.db.Close(); err!=nil { closeErr=errors.Join(closeErr,fmt.Errorf("关闭连接 %s: %w",id,err)) }
		delete(m.pools,id)
	}
	return closeErr
}
