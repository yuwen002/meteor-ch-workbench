package clickhouse

import (
	"fmt"
	"net"
	"net/url"
	"strconv"
	"strings"

	"github.com/yuwen002/meteor-ch-workbench/internal/domain/connection"
)

func buildDSN(item connection.Connection) (string,error) {
	if err:=item.ValidateForTest(); err!=nil { return "",err }
	scheme:="clickhouse"
	if item.Protocol==connection.ProtocolHTTP { scheme="http" }
	if item.Secure {
		if item.Protocol==connection.ProtocolHTTP { scheme="https" } else { scheme="clickhouse+secure" }
	}
	username:=strings.TrimSpace(item.User)
	if username=="" { username="default" }
	u:=url.URL{Scheme:scheme,Host:net.JoinHostPort(strings.TrimSpace(item.Host),strconv.Itoa(item.Port))}
	u.User=url.UserPassword(username,item.Password)
	if item.Database!="" { u.Path="/"+item.Database }
	q:=u.Query()
	if item.Database!="" { q.Set("database",item.Database) }
	if item.UseCompress { q.Set("compress","true") }
	u.RawQuery=q.Encode()
	return u.String(),nil
}

func endpointLabel(item connection.Connection) string {
	return fmt.Sprintf("%s://%s",item.Protocol,item.Endpoint())
}
