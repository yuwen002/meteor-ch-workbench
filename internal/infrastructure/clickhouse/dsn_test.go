package clickhouse

import (
	"net/url"
	"testing"

	"github.com/yuwen002/meteor-ch-workbench/internal/domain/connection"
)

func TestBuildDSNProtocolAndTLS(t *testing.T) {
	tests:=[]struct{name string;protocol connection.Protocol;secure bool;want string}{
		{"native","native",false,"clickhouse"},
		{"native tls","native",true,"clickhouse+secure"},
		{"http","http",false,"http"},
		{"https","http",true,"https"},
	}
	for _,tt:=range tests { t.Run(tt.name,func(t *testing.T){
		item:=connection.Connection{ID:"c1",Name:"db",Host:"localhost",Port:9000,Protocol:tt.protocol,User:"default"}
		item.Secure=tt.secure
		got,err:=buildDSN(item); if err!=nil { t.Fatal(err) }
		u,err:=url.Parse(got); if err!=nil { t.Fatal(err) }
		if u.Scheme!=tt.want { t.Fatalf("scheme=%q want=%q DSN=%q",u.Scheme,tt.want,got) }
	}) }
}
func TestBuildDSNEscapesCredentialsAndDatabase(t *testing.T) {
	item:=connection.Connection{ID:"c1",Name:"db",Host:"db.local",Port:8123,Protocol:connection.ProtocolHTTP,User:"user@corp",Password:"p@ss:/word",Database:"analytics db",UseCompress:true}
	dsn,err:=buildDSN(item); if err!=nil { t.Fatal(err) }
	u,err:=url.Parse(dsn); if err!=nil { t.Fatal(err) }
	password,_:=u.User.Password()
	if password!=item.Password { t.Fatalf("密码解析不一致: %q",password) }
	username,_:=u.User.Username()
	if username!=item.User { t.Fatalf("用户名解析不一致: %q",username) }
	if u.Query().Get("database")!=item.Database || u.Query().Get("compress")!="true" { t.Fatalf("query 参数不正确: %s",u.RawQuery) }
}
func TestBuildDSNRejectsInvalidConnection(t *testing.T) {
	if _,err:=buildDSN(connection.Connection{}); err==nil { t.Fatal("期望无效连接报错") }
}
func TestFingerprintDoesNotExposeDSN(t *testing.T) {
	dsn:="clickhouse://user:secret@localhost:9000"
	got:=configFingerprint(dsn)
	if got==dsn || len(got)!=64 { t.Fatalf("指纹格式不正确: %q",got) }
}
