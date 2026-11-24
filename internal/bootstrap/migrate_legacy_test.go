package bootstrap

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestMigrateLegacyConnectionsImportsSavedConnections(t *testing.T) {
	root := t.TempDir()
	newDir := filepath.Join(root, "MeteorCHWorkbench")
	legacyDir := filepath.Join(root, "meteor-ch-workbench")
	if err := os.MkdirAll(legacyDir, 0o700); err != nil {
		t.Fatal(err)
	}
	legacyPath := filepath.Join(legacyDir, "connections.json")
	raw := `{"folders":[],"connections":[{"id":"existing-1","name":"生产集群","host":"db.example.com","port":9000,"protocol":"native","user":"admin","password":"secret","database":"default","secure":true,"use_compress":true}]}`
	if err := os.WriteFile(legacyPath, []byte(raw), 0o600); err != nil {
		t.Fatal(err)
	}

	if err := migrateLegacyConnections(newDir, legacyPath); err != nil {
		t.Fatal(err)
	}
	migrated, err := os.ReadFile(filepath.Join(newDir, "connections.json"))
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"existing-1", "db.example.com", "secret", "native", "true"} {
		if !strings.Contains(string(migrated), want) {
			t.Fatalf("迁移结果缺少 %q: %s", want, migrated)
		}
	}
	if _, err := os.Stat(legacyPath); err != nil {
		t.Fatalf("旧配置不应被删除: %v", err)
	}
}

func TestMigrateLegacyConnectionsDoesNotOverwriteNewConfig(t *testing.T) {
	root := t.TempDir()
	newDir := filepath.Join(root, "MeteorCHWorkbench")
	legacyPath := filepath.Join(root, "meteor-ch-workbench", "connections.json")
	if err := os.MkdirAll(newDir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(legacyPath), 0o700); err != nil {
		t.Fatal(err)
	}
	newConfig := `{"version":1,"connections":[]}`
	if err := os.WriteFile(filepath.Join(newDir, "connections.json"), []byte(newConfig), 0o600); err != nil {
		t.Fatal(err)
	}
	legacy := `{"connections":[{"id":"legacy","name":"旧连接","host":"localhost","port":9000,"protocol":"native"}]}`
	if err := os.WriteFile(legacyPath, []byte(legacy), 0o600); err != nil {
		t.Fatal(err)
	}

	if err := migrateLegacyConnections(newDir, legacyPath); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(filepath.Join(newDir, "connections.json"))
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != newConfig {
		t.Fatalf("新配置被覆盖: %s", got)
	}
}

func TestMigrateLegacyConnectionsSkipsMissingLegacyFile(t *testing.T) {
	if err := migrateLegacyConnections(filepath.Join(t.TempDir(), "new"), filepath.Join(t.TempDir(), "missing.json")); err != nil {
		t.Fatalf("旧配置不存在时应正常启动: %v", err)
	}
}
