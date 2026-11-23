package connectionfile

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/yuwen002/meteor-ch-workbench/internal/domain/connection"
)

// legacyFile 仅用于读取旧 connections.json；不参与新应用运行时读写。
type legacyFile struct {
	Connections []legacyConnection `json:"connections"`
}
type legacyConnection struct {
	ID string `json:"id"`
	FolderID string `json:"folder_id"`
	Name string `json:"name"`
	Host string `json:"host"`
	Port int `json:"port"`
	Protocol string `json:"protocol"`
	User string `json:"user"`
	Password string `json:"password"`
	Database string `json:"database"`
	Secure bool `json:"secure"`
	UseCompress bool `json:"use_compress"`
}

// ImportLegacyJSON 将旧配置内容导入新存储。此方法只处理调用方明确提供的原始 JSON，
// 不会自行读取旧配置路径，也不会删除或覆盖旧文件。
func (r *Repository) ImportLegacyJSON(ctx context.Context, raw []byte) (int,error) {
	var old legacyFile
	if err:=json.Unmarshal(raw,&old); err!=nil { return 0,fmt.Errorf("解析旧连接配置: %w",err) }
	converted:=make([]connection.Connection,0,len(old.Connections))
	for i,item:=range old.Connections {
		protocol:=connection.Protocol(strings.TrimSpace(item.Protocol))
		if protocol=="" {
			switch item.Port { case 9000,9440: protocol=connection.ProtocolNative; default: protocol=connection.ProtocolHTTP }
		}
		convertedItem:=connection.Connection{
			ID:item.ID,FolderID:item.FolderID,Name:item.Name,Host:item.Host,Port:item.Port,
			Protocol:protocol,User:item.User,Password:item.Password,Database:item.Database,
			Secure:item.Secure,UseCompress:item.UseCompress,
		}
		if err:=convertedItem.Validate(); err!=nil { return 0,fmt.Errorf("旧配置第 %d 条连接无效: %w",i+1,err) }
		converted=append(converted,convertedItem)
	}
	seen:=make(map[string]struct{},len(converted))
	for _,item:=range converted {
		if _,ok:=seen[item.ID];ok { return 0,fmt.Errorf("%w: %s",ErrDuplicateID,item.ID) }
		seen[item.ID]=struct{}{}
	}
	for _,item:=range converted {
		if err:=r.Save(ctx,item); err!=nil { return 0,err }
	}
	return len(converted),nil
}
