package clickhouse

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	_ "github.com/ClickHouse/clickhouse-go/v2"
	"github.com/yuwen002/meteor-ch-workbench/internal/domain/connection"
)

const defaultTestTimeout = 5*time.Second

// Tester 对单次连接测试使用临时连接池，确保测试的是当前表单参数，完成后立即释放资源。
type Tester struct {
	timeout time.Duration
	open func(string,string)(*sql.DB,error)
}

func NewTester(timeout time.Duration) *Tester {
	if timeout<=0 { timeout=defaultTestTimeout }
	return &Tester{timeout:timeout,open:sql.Open}
}

func newTester(timeout time.Duration,open func(string,string)(*sql.DB,error)) *Tester {
	return &Tester{timeout:timeout,open:open}
}

func (t *Tester) Test(ctx context.Context,item connection.Connection) error {
	if err:=item.ValidateForTest(); err!=nil { return err }
	if err:=ctx.Err(); err!=nil { return err }
	dsn,err:=buildDSN(item)
	if err!=nil { return err }
	db,err:=t.open("clickhouse",dsn)
	if err!=nil { return fmt.Errorf("打开 ClickHouse 连接（%s）: %w",endpointLabel(item),err) }
	defer db.Close()
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(0)
	testCtx,cancel:=context.WithTimeout(ctx,t.timeout)
	defer cancel()
	if err:=db.PingContext(testCtx); err!=nil {
		if errors.Is(testCtx.Err(),context.DeadlineExceeded) { return fmt.Errorf("连接测试超时（%s，限制 %s）: %w",endpointLabel(item),t.timeout,err) }
		return fmt.Errorf("连接测试失败（%s）: %w",endpointLabel(item),err)
	}
	return nil
}
