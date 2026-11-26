package clickhouse

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"sync"
	"testing"

	"github.com/yuwen002/meteor-ch-workbench/internal/domain/connection"
)

type poolTestDriver struct{}

func (poolTestDriver) Open(string) (driver.Conn, error) { return poolTestConn{}, nil }

type poolTestConn struct{}

func (poolTestConn) Prepare(string) (driver.Stmt, error) { return nil, errors.New("not implemented") }
func (poolTestConn) Close() error                       { return nil }
func (poolTestConn) Begin() (driver.Tx, error)          { return nil, errors.New("not implemented") }

var registerPoolTestDriver sync.Once

func newTestPoolManager() *PoolManager {
	const driverName = "meteor-workbench-pool-test"
	registerPoolTestDriver.Do(func() {
		sql.Register(driverName, poolTestDriver{})
	})
	return newPoolManager(func(_, dsn string) (*sql.DB, error) {
		return sql.Open(driverName, dsn)
	})
}

func testConnection() connection.Connection {
	return connection.Connection{
		ID: "conn-1", Name: "db", Host: "localhost", Port: 9000,
		Protocol: connection.ProtocolNative,
	}
}

func TestPoolManagerReusesPoolAndInvalidatesChangedConfig(t *testing.T) {
	manager := newTestPoolManager()
	item := testConnection()
	first, err := manager.Acquire(item)
	if err != nil {
		t.Fatal(err)
	}
	reused, err := manager.Acquire(item)
	if err != nil {
		t.Fatal(err)
	}
	if first != reused {
		t.Fatal("相同连接参数应复用连接池")
	}
	item.Password = "changed"
	updated, err := manager.Acquire(item)
	if err != nil {
		t.Fatal(err)
	}
	if updated == first {
		t.Fatal("连接参数变化后必须替换连接池")
	}
	if err := manager.Invalidate(item.ID); err != nil {
		t.Fatal(err)
	}
	if err := manager.Close(); err != nil {
		t.Fatal(err)
	}
}

func TestPoolManagerCloseReleasesActivePools(t *testing.T) {
	manager := newTestPoolManager()
	db, err := manager.Acquire(testConnection())
	if err != nil {
		t.Fatal(err)
	}
	if err := db.Ping(); err != nil {
		t.Fatalf("关闭前连接池应可用：%v", err)
	}

	if err := manager.Close(); err != nil {
		t.Fatalf("关闭连接池失败：%v", err)
	}
	if _, err := db.Conn(context.Background()); !errors.Is(err, sql.ErrDBClosed) {
		t.Fatalf("Close 后连接池仍可获取连接，错误 = %v", err)
	}
	if err := manager.Close(); err != nil {
		t.Fatalf("重复关闭应保持幂等，错误：%v", err)
	}
}

func TestPoolManagerRequiresConnectionID(t *testing.T) {
	manager := NewPoolManager()
	_, err := manager.Acquire(connection.Connection{
		Name: "db", Host: "localhost", Port: 9000, Protocol: connection.ProtocolNative,
	})
	if !errors.Is(err, connection.ErrMissingID) {
		t.Fatalf("期望缺少 ID 错误，得到 %v", err)
	}
}
