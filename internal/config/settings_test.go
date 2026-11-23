package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestNormalizeRowLimit(t *testing.T) {
	cases := []struct {
		name string
		in   int
		want int
	}{
		{"零表示不限制", 0, RowLimitUnlimited},
		{"负数退回默认值", -1, DefaultRowLimit},
		{"小于下限退回默认值", RowLimitMin - 1, DefaultRowLimit},
		{"恰好等于下限保留", RowLimitMin, RowLimitMin},
		{"区间内原样保留", 20000, 20000},
		{"恰好等于上限保留", RowLimitMax, RowLimitMax},
		{"超过上限截断", RowLimitMax + 1, RowLimitMax},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := NormalizeRowLimit(c.in); got != c.want {
				t.Fatalf("NormalizeRowLimit(%d) = %d, want %d", c.in, got, c.want)
			}
		})
	}
}

func TestSettingsManagerDefaultsWhenMissing(t *testing.T) {
	dir := t.TempDir()
	m := NewSettingsManager(dir)
	got := m.RowLimit()
	if got != DefaultRowLimit {
		t.Fatalf("missing settings should fall back to default %d, got %d", DefaultRowLimit, got)
	}
	sf, err := m.Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if sf.QueryRowLimit != DefaultRowLimit {
		t.Fatalf("want default %d, got %d", DefaultRowLimit, sf.QueryRowLimit)
	}
}

func TestSettingsManagerRoundTrip(t *testing.T) {
	dir := t.TempDir()
	m := NewSettingsManager(dir)
	if err := m.Save(SettingsFile{QueryRowLimit: 12345}); err != nil {
		t.Fatalf("Save: %v", err)
	}
	if got := m.RowLimit(); got != 12345 {
		t.Fatalf("RowLimit after save = %d, want 12345", got)
	}

	m2 := NewSettingsManager(dir)
	sf, err := m2.Load()
	if err != nil {
		t.Fatalf("reload Load: %v", err)
	}
	if sf.QueryRowLimit != 12345 {
		t.Fatalf("reloaded value = %d, want 12345", sf.QueryRowLimit)
	}

	data, err := os.ReadFile(filepath.Join(dir, "settings.json"))
	if err != nil {
		t.Fatalf("read settings.json: %v", err)
	}
	if len(data) == 0 {
		t.Fatal("settings.json should not be empty")
	}
}

func TestSettingsManagerSaveNormalizesInput(t *testing.T) {
	dir := t.TempDir()
	m := NewSettingsManager(dir)
	if err := m.Save(SettingsFile{QueryRowLimit: -5}); err != nil {
		t.Fatalf("Save: %v", err)
	}
	if got := m.RowLimit(); got != DefaultRowLimit {
		t.Fatalf("negative input should normalize to %d, got %d", DefaultRowLimit, got)
	}
	if err := m.Save(SettingsFile{QueryRowLimit: RowLimitMax + 100}); err != nil {
		t.Fatalf("Save: %v", err)
	}
	if got := m.RowLimit(); got != RowLimitMax {
		t.Fatalf("overflow input should clamp to %d, got %d", RowLimitMax, got)
	}
}

func TestSettingsManagerLoadCorruptedFile(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "settings.json"), []byte("{not json"), 0o644); err != nil {
		t.Fatalf("write corrupt file: %v", err)
	}
	m := NewSettingsManager(dir)
	if got := m.RowLimit(); got != DefaultRowLimit {
		t.Fatalf("corrupt file should fall back to default %d, got %d", DefaultRowLimit, got)
	}
}

func TestNormalizeSettingsLayout(t *testing.T) {
	cases := []struct {
		name string
		input SettingsFile
		want float64
	}{
		{name: "旧配置缺少比例", input: SettingsFile{QueryRowLimit: 8000}, want: DefaultEditorRatio},
		{name: "低于最小比例", input: SettingsFile{QueryRowLimit: 8000, EditorRatio: 0.1}, want: DefaultEditorRatio},
		{name: "高于最大比例", input: SettingsFile{QueryRowLimit: 8000, EditorRatio: 0.9}, want: DefaultEditorRatio},
		{name: "合法比例保留", input: SettingsFile{QueryRowLimit: 8000, EditorRatio: 0.7}, want: 0.7},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := NormalizeSettings(tc.input)
			if got.EditorRatio != tc.want {
				t.Fatalf("EditorRatio = %v, want %v", got.EditorRatio, tc.want)
			}
		})
	}
}

func TestSettingsManagerPersistsWorkspaceLayout(t *testing.T) {
	dir := t.TempDir()
	manager := NewSettingsManager(dir)
	want := SettingsFile{QueryRowLimit: 12000, SidebarCollapsed: true, EditorRatio: 0.73}
	if err := manager.Save(want); err != nil {
		t.Fatalf("Save: %v", err)
	}
	reloaded, err := NewSettingsManager(dir).Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if reloaded != want {
		t.Fatalf("reloaded settings = %+v, want %+v", reloaded, want)
	}
}
