package clickhouse

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"io"
	"reflect"
	"strings"
	"sync"
	"testing"

	appmetadata "github.com/yuwen002/meteor-ch-workbench/internal/application/metadata"
	"github.com/yuwen002/meteor-ch-workbench/internal/domain/connection"
)

const metadataTestDriverName = "meteor-workbench-metadata-test"
var registerMetadataDriver sync.Once
var metadataLastQuery string
var metadataLastArgs []driver.NamedValue

type metadataDriver struct{}
func (metadataDriver) Open(string) (driver.Conn,error) { return metadataConn{},nil }
type metadataConn struct{}
func (metadataConn) Prepare(string) (driver.Stmt,error) { return nil,errors.New("not implemented") }
func (metadataConn) Close() error { return nil }
func (metadataConn) Begin() (driver.Tx,error) { return nil,errors.New("not implemented") }
func (metadataConn) QueryContext(_ context.Context, statement string, args []driver.NamedValue) (driver.Rows,error) {
	metadataLastQuery=statement
	metadataLastArgs=append([]driver.NamedValue(nil),args...)
	switch {
	case statement=="SHOW DATABASES":
		return &metadataRows{columns:[]string{"name"},values:[][]driver.Value{{"default"},{"analytics"}}},nil
	case strings.Contains(statement,"system.tables") && strings.Contains(statement,"NOT LIKE"):
		return &metadataRows{columns:[]string{"name","engine","comment"},values:[][]driver.Value{{"events","MergeTree","event data"}}},nil
	case strings.Contains(statement,"system.tables") && strings.Contains(statement,"LIKE"):
		return &metadataRows{columns:[]string{"name","engine","comment"},values:[][]driver.Value{{"active_users","View","active users"}}},nil
	case strings.HasPrefix(statement,"DESCRIBE TABLE"):
		return &metadataRows{columns:[]string{"name","type","default_type","comment","primary_key"},values:[][]driver.Value{{"id","UInt64","","identifier",uint8(1)},{"created_at","DateTime","DEFAULT","",uint8(0)}}},nil
	case strings.Contains(statement,"create_table_query"):
		return &metadataRows{columns:[]string{"engine","comment","create_table_query","partition_key","sorting_key","primary_key"},values:[][]driver.Value{{"MergeTree","events","CREATE TABLE events (created_at DateTime) ENGINE = MergeTree ORDER BY created_at TTL created_at + INTERVAL 7 DAY SETTINGS index_granularity = 8192","","created_at","created_at"}}},nil
	default:
		return nil,errors.New("unexpected metadata SQL: "+statement)
	}
}
type metadataRows struct { columns []string; values [][]driver.Value; index int }
func (r *metadataRows) Columns() []string { return r.columns }
func (r *metadataRows) Close() error { return nil }
func (r *metadataRows) Next(dest []driver.Value) error {
	if r.index>=len(r.values) { return io.EOF }
	copy(dest,r.values[r.index]); r.index++; return nil
}
func (r *metadataRows) ColumnTypeScanType(index int) reflect.Type { return reflect.TypeOf("") }

func newMetadataReader(t *testing.T) (*MetadataReader,*PoolManager) {
	t.Helper()
	registerMetadataDriver.Do(func(){sql.Register(metadataTestDriverName,metadataDriver{})})
	pools:=newPoolManager(func(_,dsn string)(*sql.DB,error){return sql.Open(metadataTestDriverName,dsn)})
	reader,err:=NewMetadataReader(pools); if err!=nil {t.Fatal(err)}
	return reader,pools
}
func metadataConnection() connection.Connection {
	return connection.Connection{ID:"metadata-test",Name:"Metadata",Host:"localhost",Port:9000,Protocol:connection.ProtocolNative}
}

func TestMetadataReaderListDatabases(t *testing.T) {
	reader,pools:=newMetadataReader(t); defer pools.Close()
	got,err:=reader.ListDatabases(context.Background(),metadataConnection())
	if err!=nil {t.Fatal(err)}
	if !reflect.DeepEqual(got,[]string{"default","analytics"}) {t.Fatalf("数据库列表=%v",got)}
}

func TestMetadataReaderListsTablesAndViewsWithBoundDatabase(t *testing.T) {
	reader,pools:=newMetadataReader(t); defer pools.Close()
	tables,err:=reader.ListTables(context.Background(),metadataConnection(),"analytics' OR 1=1 --")
	if err!=nil {t.Fatal(err)}
	if len(tables)!=1 || tables[0].Name!="events" {t.Fatalf("表列表=%+v",tables)}
	if len(metadataLastArgs)!=1 || metadataLastArgs[0].Value!="analytics' OR 1=1 --" {t.Fatalf("数据库名未通过参数绑定: %+v",metadataLastArgs)}
	views,err:=reader.ListViews(context.Background(),metadataConnection(),"analytics")
	if err!=nil {t.Fatal(err)}
	if len(views)!=1 || views[0].Engine!="View" {t.Fatalf("视图列表=%+v",views)}
}

func TestMetadataReaderGetsSchemaByColumnName(t *testing.T) {
	reader,pools:=newMetadataReader(t); defer pools.Close()
	got,err:=reader.GetTableSchema(context.Background(),metadataConnection(),"analytics","events")
	if err!=nil {t.Fatal(err)}
	if len(got)!=2 || got[0].Name!="id" || got[0].Type!="UInt64" || !got[0].IsPrimary || !got[1].HasDefault {t.Fatalf("字段结构=%+v",got)}
	if !strings.Contains(metadataLastQuery,"`analytics`.`events`") {t.Fatalf("表名没有正确引用: %s",metadataLastQuery)}
}

func TestMetadataReaderGetsTableDetails(t *testing.T) {
	reader,pools:=newMetadataReader(t); defer pools.Close()
	got,err:=reader.GetTableDetails(context.Background(),metadataConnection(),"analytics","events")
	if err!=nil {t.Fatal(err)}
	if got.Engine!="MergeTree" || got.SortingKey!="created_at" || got.TTLExpression!="created_at + INTERVAL 7 DAY" {t.Fatalf("表详情=%+v",got)}
	if len(metadataLastArgs)!=2 || metadataLastArgs[0].Value!="analytics" || metadataLastArgs[1].Value!="events" {t.Fatalf("详情查询参数绑定错误: %+v",metadataLastArgs)}
}

var _ appmetadata.Reader = (*MetadataReader)(nil)
