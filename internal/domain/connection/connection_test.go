package connection

import (
	"errors"
	"testing"
)

func TestValidate(t *testing.T) {
	base := Connection{ID:"id", Name:"name", Host:"localhost", Port:9000, Protocol:ProtocolNative}
	tests := []struct{name string; mutate func(*Connection); want error}{
		{"合法连接", func(*Connection){}, nil},
		{"缺少 ID", func(c *Connection){c.ID=""}, ErrMissingID},
		{"缺少名称", func(c *Connection){c.Name=" "}, ErrMissingName},
		{"缺少主机", func(c *Connection){c.Host=" "}, ErrMissingHost},
		{"端口越界", func(c *Connection){c.Port=65536}, ErrInvalidPort},
		{"协议不支持", func(c *Connection){c.Protocol="grpc"}, ErrInvalidProtocol},
	}
	for _, tt := range tests { t.Run(tt.name, func(t *testing.T) {
		item:=base; tt.mutate(&item); err:=item.Validate()
		if tt.want==nil { if err!=nil {t.Fatalf("不应报错: %v",err)}; return }
		if !errors.Is(err,tt.want) {t.Fatalf("期望 %v，得到 %v",tt.want,err)}
	}) }
}
func TestEndpointDoesNotExposeCredentials(t *testing.T) {
	item:=Connection{Host:"db.internal",Port:9000,User:"admin",Password:"secret",Database:"private"}
	if got:=item.Endpoint(); got!="db.internal:9000" {t.Fatalf("端点不安全或错误: %q",got)}
}
