package app

import (
	"testing"

	metadataapp "github.com/yuwen002/meteor-ch-workbench/internal/application/metadata"
)

func TestIsCurrentObjectsResult(t *testing.T) {
	base := objectsResult{seq: 7, connectionID: "conn-a", database: "analytics", tables: []metadataapp.Object{{Name: "events"}}}
	tests := []struct {
		name string
		result objectsResult
		seq uint64
		connectionID, database string
		want bool
	}{
		{name: "current response", result: base, seq: 7, connectionID: "conn-a", database: "analytics", want: true},
		{name: "stale sequence", result: base, seq: 8, connectionID: "conn-a", database: "analytics"},
		{name: "different connection", result: base, seq: 7, connectionID: "conn-b", database: "analytics"},
		{name: "different database", result: base, seq: 7, connectionID: "conn-a", database: "default"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isCurrentObjectsResult(tt.result, tt.seq, tt.connectionID, tt.database); got != tt.want {
				t.Fatalf("isCurrentObjectsResult() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestIsCurrentSchemaResult(t *testing.T) {
	base := schemaResult{seq: 11, connectionID: "conn-a", database: "analytics", table: "events"}
	tests := []struct {
		name string
		result schemaResult
		seq uint64
		connectionID, database, table string
		want bool
	}{
		{name: "current response", result: base, seq: 11, connectionID: "conn-a", database: "analytics", table: "events", want: true},
		{name: "stale sequence", result: base, seq: 12, connectionID: "conn-a", database: "analytics", table: "events"},
		{name: "different connection", result: base, seq: 11, connectionID: "conn-b", database: "analytics", table: "events"},
		{name: "different database", result: base, seq: 11, connectionID: "conn-a", database: "default", table: "events"},
		{name: "different table", result: base, seq: 11, connectionID: "conn-a", database: "analytics", table: "users"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isCurrentSchemaResult(tt.result, tt.seq, tt.connectionID, tt.database, tt.table); got != tt.want {
				t.Fatalf("isCurrentSchemaResult() = %v, want %v", got, tt.want)
			}
		})
	}
}
