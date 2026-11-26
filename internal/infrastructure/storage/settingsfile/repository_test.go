package settingsfile

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	domain "github.com/yuwen002/meteor-ch-workbench/internal/domain/settings"
)

func TestRepositoryDefaultsAndRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nested", "settings.json")
	repo, err := New(path); if err != nil { t.Fatal(err) }
	got, err := repo.Load(context.Background()); if err != nil { t.Fatal(err) }
	if got != domain.Default() { t.Fatalf("unexpected defaults: %+v", got) }
	want := domain.Preferences{Theme: domain.ThemeLight, QueryRowLimit: 25000, SidebarCollapsed: true, EditorRatio: 0.72}
	if err := repo.Save(context.Background(), want); err != nil { t.Fatal(err) }
	reloaded, err := repo.Load(context.Background()); if err != nil { t.Fatal(err) }
	if reloaded != want { t.Fatalf("got %+v, want %+v", reloaded, want) }
	info, err := os.Stat(path); if err != nil { t.Fatal(err) }
	if runtime.GOOS != "windows" && info.Mode().Perm() != 0o600 { t.Fatalf("settings permissions = %o, want 600", info.Mode().Perm()) }
}

func TestRepositoryLoadsLegacyPreferencesWithDefaultTheme(t *testing.T) {
	path := filepath.Join(t.TempDir(), "settings.json")
	legacy := []byte(`{"query_row_limit":25000,"sidebar_collapsed":true,"editor_ratio":0.72}`)
	if err := os.WriteFile(path, legacy, 0o600); err != nil {
		t.Fatal(err)
	}
	repo, err := New(path)
	if err != nil {
		t.Fatal(err)
	}
	got, err := repo.Load(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	want := domain.Preferences{
		Theme: domain.ThemeLight,
		QueryRowLimit: 25000,
		SidebarCollapsed: true,
		EditorRatio: 0.72,
	}
	if got != want {
		t.Fatalf("legacy settings = %+v, want %+v", got, want)
	}
}

func TestRepositoryRejectsCorruptJSON(t *testing.T) {
	path := filepath.Join(t.TempDir(), "settings.json")
	if err := os.WriteFile(path, []byte("{broken"), 0o600); err != nil { t.Fatal(err) }
	repo, _ := New(path)
	if _, err := repo.Load(context.Background()); err == nil { t.Fatal("expected parse error") }
}
