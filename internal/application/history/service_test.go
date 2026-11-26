package history

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/yuwen002/meteor-ch-workbench/internal/domain/queryhistory"
)

type memoryRepository struct { entries []queryhistory.Entry; saveErr error }
func (r *memoryRepository) List(context.Context) ([]queryhistory.Entry,error) { return append([]queryhistory.Entry(nil),r.entries...),nil }
func (r *memoryRepository) Save(_ context.Context, entries []queryhistory.Entry) error {
	if r.saveErr != nil { return r.saveErr }
	r.entries=append([]queryhistory.Entry(nil),entries...)
	return nil
}
type fixedIDs struct { id string; err error }
func (f fixedIDs) NewID()(string,error){return f.id,f.err}

func TestAddPrependsAndNormalizesHistory(t *testing.T){
	repo:=&memoryRepository{entries:[]queryhistory.Entry{{ID:"old",SQL:"SELECT 1"}}}
	s,err:=NewService(repo,fixedIDs{id:"new"})
	if err!=nil{t.Fatal(err)}
	entry,err:=s.Add(context.Background(),queryhistory.Entry{SQL:"  SELECT 2  ",ConnectionID:"conn"})
	if err!=nil{t.Fatal(err)}
	if entry.ID!="new"||entry.SQL!="SELECT 2"||entry.Timestamp.IsZero(){t.Fatalf("新增记录不正确: %+v",entry)}
	if len(repo.entries)!=2||repo.entries[0].ID!="new"{t.Fatalf("最新记录未置顶: %+v",repo.entries)}
}

func TestAddKeepsOnlyNewest200(t *testing.T){
	repo:=&memoryRepository{}
	for i:=0;i<MaxEntries;i++{repo.entries=append(repo.entries,queryhistory.Entry{ID:string(rune(i+1))})}
	s,_:=NewService(repo,fixedIDs{id:"new"})
	if _,err:=s.Add(context.Background(),queryhistory.Entry{SQL:"SELECT 1",ConnectionID:"conn"});err!=nil{t.Fatal(err)}
	if len(repo.entries)!=MaxEntries||repo.entries[0].ID!="new"{t.Fatalf("历史上限未生效，数量=%d",len(repo.entries))}
}

func TestDeleteAndClearHistory(t *testing.T){
	repo:=&memoryRepository{entries:[]queryhistory.Entry{{ID:"a"},{ID:"b"}}}
	s,_:=NewService(repo,fixedIDs{id:"new"})
	if err:=s.Delete(context.Background(),"a");err!=nil{t.Fatal(err)}
	if len(repo.entries)!=1||repo.entries[0].ID!="b"{t.Fatalf("删除结果错误: %+v",repo.entries)}
	if err:=s.Delete(context.Background(),"missing");!errors.Is(err,ErrNotFound){t.Fatalf("期望 ErrNotFound，得到 %v",err)}
	if err:=s.Clear(context.Background());err!=nil{t.Fatal(err)}
	if len(repo.entries)!=0{t.Fatalf("清空后仍有记录: %+v",repo.entries)}
}

func TestAddRejectsEmptySQLAndPropagatesStorageError(t *testing.T){
	s,_:=NewService(&memoryRepository{},fixedIDs{id:"id"})
	if _,err:=s.Add(context.Background(),queryhistory.Entry{SQL:"  ",ConnectionID:"conn"});err==nil{t.Fatal("空 SQL 应被拒绝")}
	want:=errors.New("disk full")
	repo:=&memoryRepository{saveErr:want}
	s,_=NewService(repo,fixedIDs{id:"id"})
	if _,err:=s.Add(context.Background(),queryhistory.Entry{SQL:"SELECT 1",ConnectionID:"conn",Timestamp:time.Now()});!errors.Is(err,want){t.Fatalf("存储错误未透传: %v",err)}
}
