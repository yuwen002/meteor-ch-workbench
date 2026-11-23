package clickhouse

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/yuwen002/meteor-ch-workbench/internal/domain/connection"
)

type blockingPingDriver struct{}
func (blockingPingDriver) Open(string)(driver.Conn,error) { return blockingPingConn{},nil }
type blockingPingConn struct{}
func (blockingPingConn) Prepare(string)(driver.Stmt,error){ return nil,errors.New("not implemented") }
func (blockingPingConn) Close()error{return nil}
func (blockingPingConn) Begin()(driver.Tx,error){return nil,errors.New("not implemented")}
func (blockingPingConn) Ping(ctx context.Context)error{ <-ctx.Done(); return ctx.Err() }

func TestTesterEnforcesTimeout(t *testing.T) {
	const driverName="meteor-workbench-clickhouse-timeout"
	sql.Register(driverName,blockingPingDriver{})
	tester:=newTester(15*time.Millisecond,func(_,dsn string)(*sql.DB,error){return sql.Open(driverName,dsn)})
	item:=connection.Connection{Name:"db",Host:"localhost",Port:9000,Protocol:connection.ProtocolNative}
	err:=tester.Test(context.Background(),item)
	if err==nil || !strings.Contains(err.Error(),"连接测试超时") { t.Fatalf("期望超时错误，得到 %v",err) }
}

func TestTesterRejectsCanceledContextBeforeOpening(t *testing.T) {
	opened:=false
	tester:=newTester(time.Second,func(string,string)(*sql.DB,error){ opened=true; return nil,nil })
	ctx,cancel:=context.WithCancel(context.Background()); cancel()
	err:=tester.Test(ctx,connection.Connection{Name:"db",Host:"localhost",Port:9000,Protocol:connection.ProtocolNative})
	if !errors.Is(err,context.Canceled) { t.Fatalf("期望 context canceled，得到 %v",err) }
	if opened { t.Fatal("取消后不应打开数据库") }
}
func TestTesterClosesPoolAfterPingFailure(t *testing.T) {
	closed:=false
	driverName:="test-clickhouse-ping-failure"
	sql.Register(driverName,closeTrackingDriver{closed:&closed})
	tester:=newTester(time.Second,func(_,dsn string)(*sql.DB,error){ return sql.Open(driverName,dsn) })
	item:=connection.Connection{Name:"db",Host:"localhost",Port:9000,Protocol:connection.ProtocolNative}
	err:=tester.Test(context.Background(),item)
	if err==nil || !strings.Contains(err.Error(),"连接测试失败") { t.Fatalf("期望 Ping 失败，得到 %v",err) }
	if !closed { t.Fatal("Ping 失败后应释放连接") }
}
type closeTrackingDriver struct { closed *bool }
func (d closeTrackingDriver) Open(string)(driver.Conn,error) { return closeTrackingConn{closed:d.closed},nil }
type closeTrackingConn struct { closed *bool }
func(c closeTrackingConn) Prepare(string)(driver.Stmt,error){ return nil,errors.New("not implemented") }
func(c closeTrackingConn) Close()error{ *c.closed=true; return nil }
func(c closeTrackingConn) Begin()(driver.Tx,error){ return nil,errors.New("not implemented") }
func(c closeTrackingConn) Ping(context.Context)error{ return errors.New("offline") }
