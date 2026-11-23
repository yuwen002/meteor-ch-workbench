package clickhouse

import (
	"database/sql"
	"database/sql/driver"
	"errors"
	"testing"

	"github.com/yuwen002/meteor-ch-workbench/internal/domain/connection"
)

type poolTestDriver struct{}
func (poolTestDriver) Open(string)(driver.Conn,error) { return poolTestConn{},nil }
type poolTestConn struct{}
func (poolTestConn) Prepare(string)(driver.Stmt,error){ return nil,errors.New("not implemented") }
func (poolTestConn) Close()error{return nil}
func (poolTestConn) Begin()(driver.Tx,error){return nil,errors.New("not implemented")}

func TestPoolManagerReusesPoolAndInvalidatesChangedConfig(t *testing.T) {
	const driverName="meteor-workbench-pool-test"
	sql.Register(driverName,poolTestDriver{})
	manager:=newPoolManager(func(_,dsn string)(*sql.DB,error){return sql.Open(driverName,dsn)})
	item:=connection.Connection{ID:"conn-1",Name:"db",Host:"localhost",Port:9000,Protocol:connection.ProtocolNative}
	first,err:=manager.Acquire(item); if err!=nil {t.Fatal(err)}
	reused,err:=manager.Acquire(item); if err!=nil {t.Fatal(err)}
	if first!=reused {t.Fatal("相同连接参数应复用连接池")}
	item.Password="changed"
	updated,err:=manager.Acquire(item); if err!=nil {t.Fatal(err)}
	if updated==first {t.Fatal("连接参数变化后必须替换连接池")}
	if err:=manager.Invalidate(item.ID); err!=nil {t.Fatal(err)}
	if err:=manager.Close(); err!=nil {t.Fatal(err)}
}
func TestPoolManagerRequiresConnectionID(t *testing.T) {
	manager:=NewPoolManager()
	_,err:=manager.Acquire(connection.Connection{Name:"db",Host:"localhost",Port:9000,Protocol:connection.ProtocolNative})
	if !errors.Is(err,connection.ErrMissingID) {t.Fatalf("期望缺少 ID 错误，得到 %v",err)}
}
