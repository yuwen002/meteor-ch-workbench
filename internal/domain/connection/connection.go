// Package connection 定义数据库连接的领域模型与校验规则。
package connection

import (
	"errors"
	"fmt"
	"strings"
)

type Protocol string

const (
	ProtocolNative Protocol = "native"
	ProtocolHTTP   Protocol = "http"
)

var (
	ErrMissingID   = errors.New("连接 ID 不能为空")
	ErrMissingName = errors.New("连接名称不能为空")
	ErrMissingHost = errors.New("主机地址不能为空")
	ErrInvalidPort = errors.New("端口必须在 1 到 65535 之间")
	ErrInvalidProtocol = errors.New("连接协议必须是 native 或 http")
)

// Connection 是与 GUI、JSON 文件和 ClickHouse 驱动无关的连接定义。
// 密码仅用于传递给凭据存储/驱动端口，不应被日志或 UI DTO 直接序列化。
type Connection struct {
	ID          string
	FolderID    string
	Name        string
	Host        string
	Port        int
	Protocol    Protocol
	User        string
	Password    string
	Database    string
	Secure      bool
	UseCompress bool
}

// Validate 检查连接配置的必要字段，不执行网络访问。
func (c Connection) Validate() error {
	if strings.TrimSpace(c.ID) == "" { return ErrMissingID }
	if strings.TrimSpace(c.Name) == "" { return ErrMissingName }
	if strings.TrimSpace(c.Host) == "" { return ErrMissingHost }
	if c.Port < 1 || c.Port > 65535 { return ErrInvalidPort }
	if c.Protocol != ProtocolNative && c.Protocol != ProtocolHTTP {
		return fmt.Errorf("%w: %q", ErrInvalidProtocol, c.Protocol)
	}
	return nil
}

// Endpoint 返回不含用户名、密码和数据库名的安全网络端点描述。
func (c Connection) Endpoint() string {
	return fmt.Sprintf("%s:%d", strings.TrimSpace(c.Host), c.Port)
}
