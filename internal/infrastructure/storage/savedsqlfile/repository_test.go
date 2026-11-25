package savedsqlfile

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"github.com/yuwen002/meteor-ch-workbench/internal/domain/savedsql"
)

func TestRepositoryReadsLegacySavedSQLJSON(t *testing.T){
	path:=filepath.Join(t.TempDir(),"saved_sql.json")
	raw:=`[{"id":"s1","name":"常用查询","sql":"SELECT 1","connection_id":"conn-1","created_at":"2026-11-25T10:00:00Z","updated_at":"2026-11-25T11:00:00Z"}]`
	if err:=os.WriteFile(path,[]byte(raw),0o600);err!=nil{t.Fatal(err)}
	repo,err:=New(path);if err!=nil{t.Fatal(err)}
	items,err:=repo.List(context.Background());if err!=nil{t.Fatal(err)}
	if len(items)!=1||items[0].ID!="s1"||items[0].Name!="常用查询"||items[0].ConnectionID!="conn-1"{t.Fatalf("旧收藏格式未兼容: %+v",items)}
}

func TestRepositorySavesAndOverwrites(t *testing.T){
	path:=filepath.Join(t.TempDir(),"saved_sql.json")
	repo,err:=New(path);if err!=nil{t.Fatal(err)}
	first:=[]savedsql.Entry{{ID:"s1",SQL:"SELECT 1",CreatedAt:time.Now()}}
	second:=[]savedsql.Entry{{ID:"s2",SQL:"SELECT 2",CreatedAt:time.Now()}}
	if err:=repo.Save(context.Background(),first);err!=nil{t.Fatal(err)}
	if err:=repo.Save(context.Background(),second);err!=nil{t.Fatal(err)}
	got,err:=repo.List(context.Background());if err!=nil{t.Fatal(err)}
	if !reflect.DeepEqual(got,second){t.Fatalf("覆盖保存结果=%+v，期望 %+v",got,second)}
	if _,err:=os.Stat(path+".bak");!os.IsNotExist(err){t.Fatalf("备份文件未清理: %v",err)}
}
