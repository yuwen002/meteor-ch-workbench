package app

import (
	"testing"

	queryapp "github.com/yuwen002/meteor-ch-workbench/internal/application/query"
)

func TestClassifySQLMode(t *testing.T) {
	tests := []struct {
		name string
		sql string
		want queryapp.Mode
	}{
		{name:"select",sql:"SELECT 1",want:queryapp.ModeRows},
		{name:"with cte",sql:"WITH sample AS (SELECT 1) SELECT * FROM sample",want:queryapp.ModeRows},
		{name:"show",sql:"SHOW DATABASES",want:queryapp.ModeRows},
		{name:"describe",sql:"DESCRIBE TABLE events",want:queryapp.ModeRows},
		{name:"leading line comment",sql:"-- inspect rows\nSELECT 1",want:queryapp.ModeRows},
		{name:"leading block comment",sql:"/* inspect rows */ SELECT 1",want:queryapp.ModeRows},
		{name:"insert",sql:"INSERT INTO events VALUES (1)",want:queryapp.ModeExec},
		{name:"create",sql:"CREATE TABLE events (id UInt64)",want:queryapp.ModeExec},
		{name:"alter",sql:"ALTER TABLE events DELETE WHERE id = 1",want:queryapp.ModeExec},
	}
	for _,tt:=range tests {
		t.Run(tt.name,func(t *testing.T) {
			if got:=classifySQLMode(tt.sql); got!=tt.want { t.Fatalf("classifySQLMode()=%q, want %q",got,tt.want) }
		})
	}
}
