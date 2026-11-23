// Package connectionfile 提供新架构使用的本地连接配置存储。
package connectionfile

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

	"github.com/yuwen002/meteor-ch-workbench/internal/domain/connection"
)

const schemaVersion = 1

var (
	ErrInvalidFile = errors.New("连接配置文件格式无效")
	ErrDuplicateID = errors.New("连接配置包含重复 ID")
)

type fileData struct {
	Version     int                    `json:"version"`
	Connections []connectionRecord     `json:"connections"`
}

type connectionRecord struct {
	ID          string `json:"id"`
	FolderID    string `json:"folder_id,omitempty"`
	Name        string `json:"name"`
	Host        string `json:"host"`
	Port        int    `json:"port"`
	Protocol    string `json:"protocol"`
	User        string `json:"user,omitempty"`
	Password    string `json:"password,omitempty"`
	Database    string `json:"database,omitempty"`
	Secure      bool   `json:"secure,omitempty"`
	UseCompress bool   `json:"use_compress,omitempty"`
}

func toRecord(item connection.Connection) connectionRecord {
	return connectionRecord{
		ID:item.ID, FolderID:item.FolderID, Name:item.Name, Host:item.Host,
		Port:item.Port, Protocol:string(item.Protocol), User:item.User,
		Password:item.Password, Database:item.Database, Secure:item.Secure,
		UseCompress:item.UseCompress,
	}
}
func (r connectionRecord) toDomain() connection.Connection {
	return connection.Connection{
		ID:r.ID, FolderID:r.FolderID, Name:r.Name, Host:r.Host, Port:r.Port,
		Protocol:connection.Protocol(r.Protocol), User:r.User, Password:r.Password,
		Database:r.Database, Secure:r.Secure, UseCompress:r.UseCompress,
	}
}

// Repository 将连接保存到独立版本化 JSON 文件。
// 文件包含本地连接密码，因此目录和文件分别限制为当前用户可访问。
type Repository struct {
	mu sync.Mutex
	path string
}

func New(path string) (*Repository, error) {
	path = strings.TrimSpace(path)
	if path == "" { return nil, errors.New("连接配置文件路径不能为空") }
	absolute, err := filepath.Abs(path)
	if err != nil { return nil, fmt.Errorf("解析连接配置路径: %w", err) }
	return &Repository{path:absolute}, nil
}

func (r *Repository) List(ctx context.Context) ([]connection.Connection, error) {
	if err:=ctx.Err(); err!=nil { return nil, err }
	r.mu.Lock()
	defer r.mu.Unlock()
	data, err:=r.read()
	if err!=nil { return nil, err }
	items:=make([]connection.Connection,0,len(data.Connections))
	for _,record:=range data.Connections {
		if err:=ctx.Err(); err!=nil { return nil, err }
		items=append(items,record.toDomain())
	}
	return items,nil
}

func (r *Repository) Save(ctx context.Context, item connection.Connection) error {
	if err:=ctx.Err(); err!=nil { return err }
	if err:=item.Validate(); err!=nil { return err }
	r.mu.Lock()
	defer r.mu.Unlock()
	data,err:=r.read()
	if err!=nil { return err }
	record:=toRecord(item)
	found:=false
	for i:=range data.Connections {
		if data.Connections[i].ID==item.ID {
			data.Connections[i]=record
			found=true
			break
		}
	}
	if !found { data.Connections=append(data.Connections,record) }
	if err:=ctx.Err(); err!=nil { return err }
	return r.write(data)
}

func (r *Repository) Delete(ctx context.Context, id string) error {
	if err:=ctx.Err(); err!=nil { return err }
	if strings.TrimSpace(id)=="" { return connection.ErrMissingID }
	r.mu.Lock()
	defer r.mu.Unlock()
	data,err:=r.read()
	if err!=nil { return err }
	filtered:=make([]connectionRecord,0,len(data.Connections))
	found:=false
	for _,item:=range data.Connections {
		if item.ID==id { found=true; continue }
		filtered=append(filtered,item)
	}
	if !found { return os.ErrNotExist }
	data.Connections=filtered
	if err:=ctx.Err(); err!=nil { return err }
	return r.write(data)
}

func (r *Repository) read() (fileData,error) {
	data,err:=os.ReadFile(r.path)
	if errors.Is(err,os.ErrNotExist) { return fileData{Version:schemaVersion,Connections:[]connectionRecord{}},nil }
	if err!=nil { return fileData{},fmt.Errorf("读取连接配置: %w",err) }
	var decoded fileData
	if err:=json.Unmarshal(data,&decoded); err!=nil { return fileData{},fmt.Errorf("%w: %v",ErrInvalidFile,err) }
	if decoded.Version!=schemaVersion { return fileData{},fmt.Errorf("%w: 不支持版本 %d",ErrInvalidFile,decoded.Version) }
	if decoded.Connections==nil { decoded.Connections=[]connectionRecord{} }
	seen:=make(map[string]struct{},len(decoded.Connections))
	for _,item:=range decoded.Connections {
		if strings.TrimSpace(item.ID)=="" { return fileData{},fmt.Errorf("%w: ID 不能为空",ErrInvalidFile) }
		if _,ok:=seen[item.ID];ok { return fileData{},fmt.Errorf("%w: %s",ErrDuplicateID,item.ID) }
		seen[item.ID]=struct{}{}
		if item.Protocol=="" {
			switch item.Port { case 9000,9440: item.Protocol=string(connection.ProtocolNative); default: item.Protocol=string(connection.ProtocolHTTP) }
		}
	}
	return decoded,nil
}

func (r *Repository) write(data fileData) error {
	if err:=os.MkdirAll(filepath.Dir(r.path),0o700); err!=nil { return fmt.Errorf("创建连接配置目录: %w",err) }
	encoded,err:=json.MarshalIndent(data,"","  ")
	if err!=nil { return fmt.Errorf("编码连接配置: %w",err) }
	encoded=append(encoded,'\n')
	file,err:=os.CreateTemp(filepath.Dir(r.path),".connections-*.tmp")
	if err!=nil { return fmt.Errorf("创建连接配置临时文件: %w",err) }
	temp:=file.Name()
	defer os.Remove(temp)
	if err:=file.Chmod(0o600); err!=nil { _=file.Close(); return fmt.Errorf("设置连接配置权限: %w",err) }
	if _,err:=file.Write(encoded); err!=nil { _=file.Close(); return fmt.Errorf("写入连接配置: %w",err) }
	if err:=file.Sync(); err!=nil { _=file.Close(); return fmt.Errorf("同步连接配置: %w",err) }
	if err:=file.Close(); err!=nil { return fmt.Errorf("关闭连接配置: %w",err) }
	if err:=os.Rename(temp,r.path); err!=nil { return fmt.Errorf("原子替换连接配置: %w",err) }
	return nil
}

// RandomIDGenerator 使用 crypto/rand 生成 128 位随机连接 ID。
type RandomIDGenerator struct{}
func (RandomIDGenerator) NewID() (string,error) {
	var raw [16]byte
	if _,err:=rand.Read(raw[:]); err!=nil { return "",fmt.Errorf("生成随机连接 ID: %w",err) }
	return hex.EncodeToString(raw[:]),nil
}
