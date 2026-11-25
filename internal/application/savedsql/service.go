package savedsql

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/yuwen002/meteor-ch-workbench/internal/domain/savedsql"
)

var (
	ErrDuplicate = errors.New("该 SQL 已收藏")
	ErrNotFound = errors.New("收藏 SQL 不存在")
)

type Service struct {
	mu sync.Mutex
	repository Repository
	ids IDGenerator
}

func NewService(repository Repository, ids IDGenerator) (*Service,error) {
	if repository==nil { return nil,errors.New("收藏 SQL 仓库不能为空") }
	if ids==nil { return nil,errors.New("收藏 SQL ID 生成器不能为空") }
	return &Service{repository:repository,ids:ids},nil
}

func (s *Service) List(ctx context.Context)([]savedsql.Entry,error) {
	return s.repository.List(ctx)
}

// Add 收藏 SQL；同一 SQL 忽略首尾空白后不可重复，标题为空时取首个非空行。
func (s *Service) Add(ctx context.Context,item savedsql.Entry)(savedsql.Entry,error) {
	s.mu.Lock(); defer s.mu.Unlock()
	item.SQL=strings.TrimSpace(item.SQL)
	if item.SQL=="" { return savedsql.Entry{},errors.New("SQL 不能为空") }
	items,err:=s.repository.List(ctx); if err!=nil{return savedsql.Entry{},err}
	for _,existing:=range items { if strings.TrimSpace(existing.SQL)==item.SQL{return savedsql.Entry{},ErrDuplicate} }
	if item.ID=="" {
		id,err:=s.ids.NewID(); if err!=nil{return savedsql.Entry{},fmt.Errorf("生成收藏 SQL ID: %w",err)}
		if strings.TrimSpace(id)=="" { return savedsql.Entry{},errors.New("收藏 SQL ID 生成器返回空值") }
		item.ID=id
	}
	now:=time.Now()
	item.Name=normalizeName(item.Name,item.SQL)
	item.CreatedAt=now
	item.UpdatedAt=now
	items=append([]savedsql.Entry{item},items...)
	if err:=s.repository.Save(ctx,items);err!=nil{return savedsql.Entry{},fmt.Errorf("保存收藏 SQL: %w",err)}
	return item,nil
}

func (s *Service) Update(ctx context.Context,id,name,sqlText string)(savedsql.Entry,error) {
	s.mu.Lock(); defer s.mu.Unlock()
	sqlText=strings.TrimSpace(sqlText)
	if sqlText=="" { return savedsql.Entry{},errors.New("SQL 不能为空") }
	items,err:=s.repository.List(ctx);if err!=nil{return savedsql.Entry{},err}
	index:=-1
	for i,item:=range items {
		if item.ID==id { index=i; continue }
		if strings.TrimSpace(item.SQL)==sqlText { return savedsql.Entry{},ErrDuplicate }
	}
	if index<0{return savedsql.Entry{},ErrNotFound}
	item:=items[index]
	item.Name=normalizeName(name,sqlText)
	item.SQL=sqlText
	item.UpdatedAt=time.Now()
	items[index]=item
	if err:=s.repository.Save(ctx,items);err!=nil{return savedsql.Entry{},fmt.Errorf("更新收藏 SQL: %w",err)}
	return item,nil
}

func (s *Service) Delete(ctx context.Context,id string)error {
	s.mu.Lock();defer s.mu.Unlock()
	items,err:=s.repository.List(ctx);if err!=nil{return err}
	filtered:=make([]savedsql.Entry,0,len(items));found:=false
	for _,item:=range items { if item.ID==id {found=true;continue};filtered=append(filtered,item) }
	if !found{return ErrNotFound}
	return s.repository.Save(ctx,filtered)
}

func normalizeName(name,sqlText string)string{
	if name=strings.TrimSpace(name);name!="" {return name}
	for _,line:=range strings.Split(sqlText,"\n"){
		if line=strings.TrimSpace(line);line!="" {
			if utf8.RuneCountInString(line)>80 { runes:=[]rune(line);return string(runes[:80]) }
			return line
		}
	}
	return "未命名 SQL"
}
