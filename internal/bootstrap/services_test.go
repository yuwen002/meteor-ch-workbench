package bootstrap

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/yuwen002/meteor-ch-workbench/internal/domain/savedsql"
)

func TestNewBuildsApplicationServices(t *testing.T) {
	services, err := New(t.TempDir())
	if err != nil { t.Fatal(err) }
	defer services.Close()

	if services.Connections == nil || services.Folders == nil || services.History == nil || services.SavedSQL == nil || services.Settings == nil || services.Queries == nil || services.Metadata == nil {
		t.Fatal("新架构应用服务未完整装配")
	}
	if services.ConfigPath == "" || !filepath.IsAbs(services.ConfigPath) { t.Fatalf("连接配置路径应为绝对路径，得到 %q", services.ConfigPath) }
	if filepath.Base(services.ConfigPath) != "connections.json" { t.Fatalf("连接配置文件名错误: %q", services.ConfigPath) }

	items, err := services.Connections.List(context.Background())
	if err != nil { t.Fatal(err) }
	if len(items) != 0 { t.Fatalf("初始连接列表应为空，得到 %d 项", len(items)) }
	folders, err := services.Folders.List(context.Background())
	if err != nil { t.Fatal(err) }
	if len(folders) != 0 { t.Fatalf("初始分组列表应为空，得到 %d 项", len(folders)) }

	saved, err := services.SavedSQL.Add(context.Background(), savedsql.Entry{SQL: "SELECT 1"})
	if err != nil { t.Fatal(err) }
	if saved.ID == "" { t.Fatal("收藏 SQL 应自动生成 ID") }
	list, err := services.SavedSQL.List(context.Background())
	if err != nil { t.Fatal(err) }
	if len(list) != 1 || list[0].SQL != "SELECT 1" { t.Fatalf("收藏 SQL 服务未持久化: %+v", list) }
	if err := services.SavedSQL.Delete(context.Background(), saved.ID); err != nil { t.Fatal(err) }
	remaining, err := services.SavedSQL.List(context.Background())
	if err != nil { t.Fatal(err) }
	if len(remaining) != 0 { t.Fatalf("删除后仍有收藏 SQL: %+v", remaining) }
}

func TestNewRequiresDataDirectory(t *testing.T) {
	if _, err := New(" "); err == nil { t.Fatal("空数据目录应返回错误") }
}

func TestDefaultDataDirUsesLegacyCompatibleDirectoryName(t *testing.T) {
	configDir := filepath.Join("user", "config")
	want := filepath.Join(configDir, "meteor-ch-workbench")
	if got := defaultDataDir(configDir); got != want { t.Fatalf("默认配置目录 = %q，期望 %q", got, want) }
}

func TestSettingsPreferencesPersistInApplicationDataDirectory(t *testing.T) {
	dataDir := t.TempDir()
	services, err := New(dataDir)
	if err != nil { t.Fatal(err) }
	preferences, err := services.Settings.Load(context.Background())
	if err != nil { t.Fatal(err) }
	if preferences.QueryRowLimit != 10000 { t.Fatalf("default query row limit = %d, want 10000", preferences.QueryRowLimit) }
	preferences.QueryRowLimit = 25000
	if _, err := services.Settings.Save(context.Background(), preferences); err != nil { t.Fatal(err) }
	if err := services.Close(); err != nil { t.Fatal(err) }

	reloaded, err := New(dataDir)
	if err != nil { t.Fatal(err) }
	defer reloaded.Close()
	got, err := reloaded.Settings.Load(context.Background())
	if err != nil { t.Fatal(err) }
	if got.QueryRowLimit != 25000 { t.Fatalf("persisted row limit = %d, want 25000", got.QueryRowLimit) }
}
