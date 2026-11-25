package savedsql

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/yuwen002/meteor-ch-workbench/internal/domain/savedsql"
)

type memoryRepository struct{ items []savedsql.Entry; saveErr error }
func(r *memoryRepository)List(context.Context)([]savedsql.Entry,error){return append([]savedsql.Entry(nil),r.items...),nil}
func(r *memoryRepository)Save(_ context.Context,items []savedsql.Entry)error{if r.saveErr!=nil{return r.saveErr};r.items=append([]savedsql.Entry(nil),items...);return nil}
type fixedIDs struct{id string;err error}
func(f fixedIDs)NewID()(string,error){return f.id,f.err}

func TestAddNormalizesNameAndRejectsDuplicateSQL(t *testing.T){
	repo:=&memoryRepository{}
	s,err:=NewService(repo,fixedIDs{id:"saved-1"});if err!=nil{t.Fatal(err)}
	item,err:=s.Add(context.Background(),savedsql.Entry{SQL:"\n  SELECT * FROM events  ",ConnectionID:"conn"})
	if err!=nil{t.Fatal(err)}
	if item.ID!="saved-1"||item.Name!="SELECT * FROM events"||item.CreatedAt.IsZero()||item.UpdatedAt.IsZero(){t.Fatalf("收藏 SQL 未归一化: %+v",item)}
	if _,err:=s.Add(context.Background(),savedsql.Entry{SQL:"SELECT * FROM events"});!errors.Is(err,ErrDuplicate){t.Fatalf("重复 SQL 应被拒绝: %v",err)}
}

func TestUpdateAndDeleteSavedSQL(t *testing.T){
	repo:=&memoryRepository{}
	s,_:=NewService(repo,fixedIDs{id:"id"})
	item,err:=s.Add(context.Background(),savedsql.Entry{SQL:"SELECT 1"});if err!=nil{t.Fatal(err)}
	updated,err:=s.Update(context.Background(),item.ID,"常用查询","SELECT 2");if err!=nil{t.Fatal(err)}
	if updated.Name!="常用查询"||updated.SQL!="SELECT 2"||!updated.UpdatedAt.After(item.UpdatedAt){t.Fatalf("更新结果错误: %+v",updated)}
	if err:=s.Delete(context.Background(),item.ID);err!=nil{t.Fatal(err)}
	if len(repo.items)!=0{t.Fatalf("删除后仍有收藏: %+v",repo.items)}
	if err:=s.Delete(context.Background(),"missing");!errors.Is(err,ErrNotFound){t.Fatalf("期望 ErrNotFound，得到 %v",err)}
}

func TestSavedSQLHasNoEntryLimit(t *testing.T){
	repo:=&memoryRepository{}
	s,_:=NewService(repo,fixedIDs{id:"unused"})
	for i:=0;i<250;i++{
		item:=savedsql.Entry{ID:string(rune(i+1)),SQL:fmt.Sprintf("SELECT %d",i)}
		if _,err:=s.Add(context.Background(),item);err!=nil{t.Fatalf("第 %d 条收藏失败: %v",i,err)}
	}
	if len(repo.items)!=250{t.Fatalf("收藏不应被截断，得到 %d 条",len(repo.items))}
}

func TestAddPropagatesStorageError(t *testing.T){
	want:=errors.New("disk full")
	s,_:=NewService(&memoryRepository{saveErr:want},fixedIDs{id:"id"})
	if _,err:=s.Add(context.Background(),savedsql.Entry{SQL:"SELECT 1"});!errors.Is(err,want){t.Fatalf("存储错误未透传: %v",err)}
}
