package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestSanitizeOrphanFolders(t *testing.T) {
	cases := []struct {
		name      string
		folders   []Folder
		conns     []Connection
		wantIDs   []string
		wantEmpty []int
		wantChg   bool
	}{
		{
			name:    "全部匹配不修改",
			folders: []Folder{{ID: "f1", Name: "生产"}},
			conns: []Connection{
				{ID: "c1", FolderID: "f1", Name: "conn1"},
				{ID: "c2", FolderID: "", Name: "conn2"},
			},
			wantIDs:   []string{"f1"},
			wantEmpty: []int{},
			wantChg:   false,
		},
		{
			name:    "孤儿 folder_id 被清空",
			folders: []Folder{{ID: "f1", Name: "生产"}},
			conns: []Connection{
				{ID: "c1", FolderID: "f1", Name: "正常"},
				{ID: "c2", FolderID: "ghost_id", Name: "孤儿"},
				{ID: "c3", FolderID: "", Name: "根级"},
			},
			wantIDs:   []string{"f1"},
			wantEmpty: []int{1},
			wantChg:   true,
		},
		{
			name:      "无文件夹全部清空",
			folders:   []Folder{},
			conns:     []Connection{{ID: "c1", FolderID: "anything", Name: "全孤儿"}},
			wantIDs:   nil,
			wantEmpty: []int{0},
			wantChg:   true,
		},
		{
			name:      "nil folders 视为空",
			folders:   nil,
			conns:     []Connection{{ID: "c1", FolderID: "x", Name: ""}},
			wantIDs:   nil,
			wantEmpty: []int{0},
			wantChg:   true,
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			df := DataFile{Folders: c.folders, Connections: c.conns}
			got := sanitizeOrphanFolders(&df)
			if got != c.wantChg {
				t.Fatalf("changed = %v, want %v", got, c.wantChg)
			}
			for _, idx := range c.wantEmpty {
				if df.Connections[idx].FolderID != "" {
					t.Errorf("conn[%d].FolderID = %q, want empty", idx, df.Connections[idx].FolderID)
				}
			}
		})
	}
}

func TestLoadAllAutoSanitizesOrphanFolders(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "connections.json")

	orphanJSON := map[string]any{
		"version": 1,
		"folders": []map[string]any{
			{"id": "aa0ac036640e733e1b95d37eac5498a4", "name": "test1"},
		},
		"connections": []map[string]any{
			{
				"id":         "conn1",
				"folder_id":  "973a0def0aa1aa2d",
				"name":       "开发",
				"host":       "10.0.0.1",
				"port":       28123,
				"protocol":   "http",
				"user":       "default",
				"password":   "secret",
				"use_compress": true,
			},
			{
				"id":         "conn2",
				"folder_id":  "aa0ac036640e733e1b95d37eac5498a4",
				"name":       "正常文件夹内",
				"host":       "10.0.0.2",
				"port":       9000,
				"protocol":   "native",
				"user":       "default",
				"password":   "secret",
				"use_compress": true,
			},
		},
	}
	raw, _ := json.MarshalIndent(orphanJSON, "", "  ")
	if err := os.WriteFile(path, raw, 0o644); err != nil {
		t.Fatal(err)
	}

	m := &Manager{path: path, dir: dir}

	data, err := m.LoadAll()
	if err != nil {
		t.Fatal(err)
	}
	if len(data.Connections) != 2 {
		t.Fatalf("want 2 conns, got %d", len(data.Connections))
	}
	if data.Connections[0].FolderID != "" {
		t.Errorf("orphan folder_id should be cleared, got %q", data.Connections[0].FolderID)
	}
	if data.Connections[1].FolderID != "aa0ac036640e733e1b95d37eac5498a4" {
		t.Errorf("valid folder_id should be preserved, got %q", data.Connections[1].FolderID)
	}

	raw2, _ := os.ReadFile(path)
	var onDisk DataFile
	if err := json.Unmarshal(raw2, &onDisk); err != nil {
		t.Fatal(err)
	}
	if onDisk.Connections[0].FolderID != "" {
		t.Errorf("flush: orphan folder_id not persisted, got %q", onDisk.Connections[0].FolderID)
	}
}

func TestImportDataAlsoSanitizesOrphans(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "connections.json")

	m := &Manager{path: path, dir: dir}

	raw := `{
  "folders": [{"id": "keep", "name": "保留"}],
  "connections": [
    {"id": "a", "folder_id": "ghost", "name": "孤儿", "host": "h1", "port": 8123},
    {"id": "b", "folder_id": "keep", "name": "正常", "host": "h2", "port": 8123}
  ]
}`
	if err := m.ImportData(raw); err != nil {
		t.Fatal(err)
	}

	conns, err := m.ListConnections()
	if err != nil {
		t.Fatal(err)
	}
	if conns[0].FolderID != "" {
		t.Errorf("import: orphan not cleared, got %q", conns[0].FolderID)
	}
	if conns[1].FolderID != "keep" {
		t.Errorf("import: valid folder lost, got %q", conns[1].FolderID)
	}
}