package historyfile

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"testing"
	"time"

	"github.com/yuwen002/meteor-ch-workbench/internal/domain/queryhistory"
)

func TestRepositoryReadsLegacyHistoryJSON(t *testing.T){
	path:=filepath.Join(t.TempDir(),"history.json")
	raw:=`[{"id":"h1","sql":"SELECT 1","connection_id":"conn-1","timestamp":"2026-11-25T10:00:00Z","duration_ms":12.5,"success":true}]`
	if err:=os.WriteFile(path,[]byte(raw),0o600);err!=nil{t.Fatal(err)}
	repo,err:=New(path);if err!=nil{t.Fatal(err)}
	entries,err:=repo.List(context.Background());if err!=nil{t.Fatal(err)}
	if len(entries)!=1||entries[0].ID!="h1"||entries[0].SQL!="SELECT 1"||entries[0].DurationMs!=12.5||!entries[0].Success{t.Fatalf("旧历史格式未兼容: %+v",entries)}
}

func TestRepositorySaveAndReload(t *testing.T){
	path:=filepath.Join(t.TempDir(),"data","history.json")
	repo,err:=New(path);if err!=nil{t.Fatal(err)}
	want:=[]queryhistory.Entry{{ID:"h2",SQL:"SELECT 2",ConnectionID:"conn",Timestamp:time.Now().UTC(),Success:false}}
	if err:=repo.Save(context.Background(),want);err!=nil{t.Fatal(err)}
	got,err:=repo.List(context.Background());if err!=nil{t.Fatal(err)}
	if !reflect.DeepEqual(got,want){t.Fatalf("保存后读取不一致: got=%+v want=%+v",got,want)}
	info,err:=os.Stat(path);if err!=nil{t.Fatal(err)}
	if runtime.GOOS != "windows" && info.Mode().Perm()!=0o600{t.Fatalf("history.json 权限=%o，期望 600",info.Mode().Perm())}
}

func TestRepositoryMissingFileReturnsEmptyList(t *testing.T){
	repo,err:=New(filepath.Join(t.TempDir(),"missing.json"));if err!=nil{t.Fatal(err)}
	got,err:=repo.List(context.Background());if err!=nil{t.Fatal(err)}
	if got==nil||len(got)!=0{t.Fatalf("缺失文件应返回空列表: %#v",got)}
}


func TestRepositoryOverwritesExistingHistoryFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "history.json")
	repo, err := New(path)
	if err != nil { t.Fatal(err) }
	first := []queryhistory.Entry{{ID:"first",SQL:"SELECT 1"}}
	second := []queryhistory.Entry{{ID:"second",SQL:"SELECT 2"}}
	if err := repo.Save(context.Background(), first); err != nil { t.Fatal(err) }
	if err := repo.Save(context.Background(), second); err != nil { t.Fatal(err) }
	got, err := repo.List(context.Background())
	if err != nil { t.Fatal(err) }
	if !reflect.DeepEqual(got, second) { t.Fatalf("重复写入后内容=%+v，期望 %+v",got,second) }
	if _, err := os.Stat(path+".bak"); !os.IsNotExist(err) { t.Fatalf("临时备份应被清理，stat err=%v",err) }
}
