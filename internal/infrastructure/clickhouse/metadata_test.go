package clickhouse

import "testing"

func TestQuoteIdentifierEscapesBackticks(t *testing.T) {
	if got:=quoteIdentifier("db`name"); got!="`db``name`" {t.Fatalf("标识符转义结果=%q",got)}
}

func TestExtractTTL(t *testing.T) {
	tests:=[]struct{name,sql,want string}{
		{name:"无 TTL",sql:"CREATE TABLE t (id UInt64) ENGINE = MergeTree ORDER BY id"},
		{name:"TTL 子句",sql:"CREATE TABLE t (ts DateTime) ENGINE = MergeTree ORDER BY ts TTL ts + INTERVAL 7 DAY SETTINGS index_granularity = 8192",want:"ts + INTERVAL 7 DAY"},
		{name:"忽略大小写",sql:"CREATE TABLE t (ts DateTime) ENGINE = MergeTree ORDER BY ts ttl ts + INTERVAL 1 DAY",want:"ts + INTERVAL 1 DAY"},
	}
	for _,tt:=range tests {t.Run(tt.name,func(t *testing.T){if got:=extractTTL(tt.sql);got!=tt.want{t.Fatalf("TTL=%q，期望 %q",got,tt.want)}})}
}

func TestMetadataTruthy(t *testing.T) {
	for _,value:=range []any{true,int64(1),uint8(1),"1",[]byte("true")} {if !metadataTruthy(value){t.Errorf("%T(%v) 应为 true",value,value)}}
	for _,value:=range []any{nil,false,int64(0),"0"} {if metadataTruthy(value){t.Errorf("%T(%v) 应为 false",value,value)}}
}
