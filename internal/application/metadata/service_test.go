package metadata

import (
	"context"
	"errors"
	"testing"

	"github.com/yuwen002/meteor-ch-workbench/internal/domain/connection"
)

type lookupStub struct { item connection.Connection; err error; id string }
func (s *lookupStub) Get(_ context.Context, id string) (connection.Connection, error) { s.id=id; return s.item,s.err }

type readerStub struct { databases []string; objects []Object; columns []Column; details TableDetails; called string }
func (s *readerStub) ListDatabases(context.Context, connection.Connection) ([]string,error) { s.called="databases"; return s.databases,nil }
func (s *readerStub) ListTables(context.Context, connection.Connection,string) ([]Object,error) { s.called="tables"; return s.objects,nil }
func (s *readerStub) ListViews(context.Context, connection.Connection,string) ([]Object,error) { s.called="views"; return s.objects,nil }
func (s *readerStub) GetTableSchema(context.Context, connection.Connection,string,string) ([]Column,error) { s.called="schema"; return s.columns,nil }
func (s *readerStub) GetTableDetails(context.Context, connection.Connection,string,string) (TableDetails,error) { s.called="details"; return s.details,nil }

func TestMetadataServiceListDatabases(t *testing.T) {
	lookup:=&lookupStub{item:connection.Connection{ID:"c1"}}
	reader:=&readerStub{databases:[]string{"default","analytics"}}
	service,err:=NewService(lookup,reader); if err!=nil {t.Fatal(err)}
	got,err:=service.ListDatabases(context.Background()," c1 "); if err!=nil {t.Fatal(err)}
	if len(got)!=2 || lookup.id!="c1" || reader.called!="databases" {t.Fatalf("结果不正确: %+v",got)}
}

func TestMetadataServiceValidatesObjectNames(t *testing.T) {
	lookup:=&lookupStub{}
	reader:=&readerStub{}
	service,err:=NewService(lookup,reader); if err!=nil {t.Fatal(err)}
	if _,err:=service.ListTables(context.Background(),"c1"," "); !errors.Is(err,ErrMissingDatabase) {t.Fatalf("数据库校验错误: %v",err)}
	if _,err:=service.GetTableSchema(context.Background(),"c1","db"," "); !errors.Is(err,ErrMissingTable) {t.Fatalf("表名校验错误: %v",err)}
	if _,err:=service.ListDatabases(context.Background()," "); !errors.Is(err,ErrMissingConnectionID) {t.Fatalf("连接 ID 校验错误: %v",err)}
	if reader.called!="" {t.Fatal("校验失败不应调用 reader")}
}

func TestMetadataServicePropagatesConnectionError(t *testing.T) {
	want:=errors.New("storage failed")
	lookup:=&lookupStub{err:want}
	service,err:=NewService(lookup,&readerStub{}); if err!=nil {t.Fatal(err)}
	_,err=service.ListDatabases(context.Background(),"c1")
	if !errors.Is(err,want) {t.Fatalf("未保留连接错误: %v",err)}
}
