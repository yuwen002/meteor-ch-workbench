package bootstrap

import (
	"context"
	"path/filepath"
	"testing"
)

func TestNewBuildsApplicationServices(t *testing.T) {
	services, err := New(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer services.Close()

	if services.Connections == nil || services.Folders == nil || services.Queries == nil || services.Metadata == nil {
		t.Fatal("新架构应用服务未完整装配")
	}
	items, err := services.Connections.List(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 0 {
		t.Fatalf("初始连接列表应为空，得到 %d 项", len(items))
	}
	folders, err := services.Folders.List(context.Background())
	if err != nil { t.Fatal(err) }
	if len(folders) != 0 { t.Fatalf("初始分组列表应为空，得到 %d 项", len(folders)) }
}

func TestNewRequiresDataDirectory(t *testing.T) {
	if _, err := New(" "); err == nil {
		t.Fatal("空数据目录应返回错误")
	}
}

func TestDefaultDataDirUsesLegacyCompatibleDirectoryName(t *testing.T) {
	configDir := filepath.Join("user", "config")
	want := filepath.Join(configDir, "meteor-ch-workbench")
	if got := defaultDataDir(configDir); got != want {
		t.Fatalf("默认配置目录 = %q，期望 %q", got, want)
	}
}
