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
	raw := `{"folders":[{"id":"folder-prod","name":"生产环境"}],"connections":[{"id":"existing-1","folder_id":"folder-prod","name":"生产集群","host":"db.example.com","port":9000,"protocol":"native","user":"admin","password":"secret","database":"default","secure":true,"use_compress":true}]}`
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
	for _, want := range []string{"existing-1", "folder-prod", "生产环境", "folder_id", "db.example.com", "secret", "native", "true"} {
		if !strings.Contains(string(migrated), want) {
			t.Fatalf("迁移结果缺少 %q: %s", want, migrated)
		}
	}
	if _, err := os.Stat(legacyPath); err != nil {
		t.Fatalf("旧配置不应被删除: %v", err)
	}
}

func TestMigrateLegacyConnectionsRestoresLegacyWhenNewConfigIsEmpty(t *testing.T) {
	root := t.TempDir()
	newDir := filepath.Join(root, "MeteorCHWorkbench")
	legacyPath := filepath.Join(root, "meteor-ch-workbench", "connections.json")
	if err := os.MkdirAll(newDir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(legacyPath), 0o700); err != nil {
		t.Fatal(err)
	}
	newConfig := `{"version":1,"folders":[],"connections":[]}`
	if err := os.WriteFile(filepath.Join(newDir, "connections.json"), []byte(newConfig), 0o600); err != nil {
		t.Fatal(err)
	}
	legacy := `{"folders":[{"id":"legacy-folder","name":"旧分组"}],"connections":[{"id":"legacy","folder_id":"legacy-folder","name":"旧连接","host":"localhost","port":9000,"protocol":"native"}]}`
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
	for _, want := range []string{`"version": 1`, "legacy-folder", "旧分组", "legacy", "旧连接"} {
		if !strings.Contains(string(got), want) {
			t.Fatalf("恢复结果缺少 %q: %s", want, got)
		}
	}
	if _, err := os.Stat(legacyPath); err != nil {
		t.Fatalf("历史配置不应被删除: %v", err)
	}
}

func TestMigrateLegacyConnectionsDoesNotOverwriteNonEmptyNewConfig(t *testing.T) {
	root := t.TempDir()
	newDir := filepath.Join(root, "MeteorCHWorkbench")
	legacyPath := filepath.Join(root, "meteor-ch-workbench", "connections.json")
	if err := os.MkdirAll(newDir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(legacyPath), 0o700); err != nil {
		t.Fatal(err)
	}
	newConfig := `{"version":1,"folders":[],"connections":[{"id":"current","name":"当前连接","host":"127.0.0.1","port":9000,"protocol":"native"}]}`
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
		t.Fatalf("非空新配置被覆盖: %s", got)
	}
}

func TestMigrateLegacyConnectionsSkipsMissingLegacyFile(t *testing.T) {
	if err := migrateLegacyConnections(filepath.Join(t.TempDir(), "new"), filepath.Join(t.TempDir(), "missing.json")); err != nil {
		t.Fatalf("旧配置不存在时应正常启动: %v", err)
	}
}

func TestMigrateLegacyConnectionsUpgradesLegacyFileAtNewPath(t *testing.T) {
	dataDir := t.TempDir()
	target := filepath.Join(dataDir, "connections.json")
	legacy := `{"folders":[{"id":"legacy-folder","name":"旧版分组"}],"connections":[{"id":"old-existing","folder_id":"legacy-folder","name":"旧版生产连接","host":"127.0.0.1","port":9000,"protocol":"native","user":"default","password":"keep-me","database":"analytics","secure":false,"use_compress":true}]}`
	if err := os.WriteFile(target, []byte(legacy), 0o600); err != nil {
		t.Fatal(err)
	}

	// 模拟 Windows：新旧路径大小写不同，但解析到同一个配置文件。
	if err := migrateLegacyConnections(dataDir, filepath.Join(dataDir, "METEOR-CH-WORKBENCH", "connections.json")); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(target)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{`"version": 1`, "legacy-folder", "旧版分组", "old-existing", "旧版生产连接", "keep-me", "analytics"} {
		if !strings.Contains(string(got), want) {
			t.Fatalf("升级后的配置缺少 %q: %s", want, got)
		}
	}
}

func TestMigrateLegacyConnectionsKeepsOriginalWhenLegacyDataInvalid(t *testing.T) {
	dataDir := t.TempDir()
	target := filepath.Join(dataDir, "connections.json")
	legacy := `{"connections":[{"id":"broken","name":"","host":"","port":0}]}`
	if err := os.WriteFile(target, []byte(legacy), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := migrateLegacyConnections(dataDir, filepath.Join(dataDir, "old", "connections.json")); err == nil {
		t.Fatal("无效旧配置应返回迁移错误")
	}
	got, err := os.ReadFile(target)
	if err != nil {
		t.Fatalf("原始配置不应丢失: %v", err)
	}
	if string(got) != legacy {
		t.Fatalf("原始配置被修改: %s", got)
	}
}
