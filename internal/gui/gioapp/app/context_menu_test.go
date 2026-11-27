package app

import "testing"

func TestSQLLiteral(t *testing.T) {
	tests := []struct {
		name string
		value any
		want string
	}{
		{name: "null", value: nil, want: "NULL"},
		{name: "string quote", value: "O'Reilly", want: "'O''Reilly'"},
		{name: "integer", value: int64(42), want: "42"},
		{name: "boolean true", value: true, want: "1"},
		{name: "boolean false", value: false, want: "0"},
		{name: "bytes", value: []byte("a'b"), want: "'a''b'"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := sqlLiteral(tt.value); got != tt.want {
				t.Fatalf("sqlLiteral(%#v) = %q, want %q", tt.value, got, tt.want)
			}
		})
	}
}

func TestContextMenuActionsAreAvailableForEachTarget(t *testing.T) {
	state := &workspace{}
	for _, kind := range []string{"database", "object", "cell", "header"} {
		state.openContextMenu(kind, "default", "events", 0, 0)
		actions := state.contextActions()
		if len(actions) == 0 {
			t.Fatalf("context menu %q has no actions", kind)
		}
		for _, action := range actions {
			if action.label == "" || action.run == nil {
				t.Fatalf("context menu %q contains an incomplete action: %#v", kind, action)
			}
		}
	}
}
