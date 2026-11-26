// Package savedsqlfile 将收藏 SQL 持久化到 saved_sql.json。
package savedsqlfile

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/yuwen002/meteor-ch-workbench/internal/domain/savedsql"
)

type Repository struct { mu sync.Mutex; path string }

func New(path string)(*Repository,error){
	path=strings.TrimSpace(path);if path==""{return nil,errors.New("收藏 SQL 文件路径不能为空")}
	absolute,err:=filepath.Abs(path);if err!=nil{return nil,fmt.Errorf("解析收藏 SQL 路径: %w",err)}
	return &Repository{path:absolute},nil
}

func(r *Repository)List(ctx context.Context)([]savedsql.Entry,error){
	if err:=ctx.Err();err!=nil{return nil,err}
	r.mu.Lock();defer r.mu.Unlock()
	data,err:=os.ReadFile(r.path)
	if errors.Is(err,os.ErrNotExist){return []savedsql.Entry{},nil}
	if err!=nil{return nil,fmt.Errorf("读取收藏 SQL: %w",err)}
	if len(data)==0{return []savedsql.Entry{},nil}
	var items []savedsql.Entry
	if err:=json.Unmarshal(data,&items);err!=nil{return nil,fmt.Errorf("解析收藏 SQL: %w",err)}
	if items==nil{items=[]savedsql.Entry{}}
	return items,nil
}

func(r *Repository)Save(ctx context.Context,items []savedsql.Entry)error{
	if err:=ctx.Err();err!=nil{return err}
	r.mu.Lock();defer r.mu.Unlock()
	if err:=os.MkdirAll(filepath.Dir(r.path),0o700);err!=nil{return fmt.Errorf("创建收藏 SQL 目录: %w",err)}
	if items==nil{items=[]savedsql.Entry{}}
	data,err:=json.MarshalIndent(items,"","  ");if err!=nil{return err}
	data=append(data,'\n')
	file,err:=os.CreateTemp(filepath.Dir(r.path),".saved-sql-*.tmp");if err!=nil{return err}
	temp:=file.Name();defer os.Remove(temp)
	if err:=file.Chmod(0o600);err!=nil{_ = file.Close();return err}
	if _,err:=file.Write(data);err!=nil{_ = file.Close();return err}
	if err:=file.Sync();err!=nil{_ = file.Close();return err}
	if err:=file.Close();err!=nil{return err}
	if err:=ctx.Err();err!=nil{return err}
	if err:=replace(temp,r.path);err!=nil{return fmt.Errorf("替换收藏 SQL 文件: %w",err)}
	return nil
}

func replace(temp,target string)error{
	if err:=os.Rename(temp,target);err==nil{return nil}
	backup:=target+".bak";_ = os.Remove(backup)
	if err:=os.Rename(target,backup);err!=nil{return err}
	if err:=os.Rename(temp,target);err!=nil{
		if restoreErr:=os.Rename(backup,target);restoreErr!=nil{return fmt.Errorf("替换失败: %v；恢复失败: %w",err,restoreErr)}
		return err
	}
	_ = os.Remove(backup);return nil
}

type RandomIDGenerator struct{}
func(RandomIDGenerator)NewID()(string,error){
	var raw [16]byte
	if _,err:=rand.Read(raw[:]);err!=nil{return "",err}
	return hex.EncodeToString(raw[:]),nil
}
