package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestTabsSaveAndReload(t *testing.T) {
	dir := t.TempDir()
	tm := NewTabsManager(dir)

	tabs := []TabState{
		{ID: "t1", Title: "Query 1", SQL: "SELECT 1"},
		{ID: "t2", Title: "Charts", SQL: "SELECT 2"},
	}
	if err := tm.Save(TabsFile{Tabs: tabs, ActiveTabID: "t2"}); err != nil {
		t.Fatalf("Save: %v", err)
	}

	reloaded := NewTabsManager(dir)
	got, err := reloaded.Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(got.Tabs) != 2 {
		t.Fatalf("want 2 tabs, got %d", len(got.Tabs))
	}
	if got.ActiveTabID != "t2" {
		t.Fatalf("want active t2, got %q", got.ActiveTabID)
	}
	if got.Tabs[1].Title != "Charts" || got.Tabs[1].SQL != "SELECT 2" {
		t.Fatalf("unexpected tab content: %+v", got.Tabs[1])
	}
}

func TestTabsMissingFileReturnsEmpty(t *testing.T) {
	tm := NewTabsManager(t.TempDir())
	got, err := tm.Load()
	if err != nil {
		t.Fatalf("Load on missing file should not error: %v", err)
	}
	if len(got.Tabs) != 0 {
		t.Fatalf("want empty tabs, got %d", len(got.Tabs))
	}
}

func TestTabsInvalidActiveFallsBackToFirst(t *testing.T) {
	tm := NewTabsManager(t.TempDir())
	tabs := []TabState{{ID: "a", Title: "A"}, {ID: "b", Title: "B"}}
	if err := tm.Save(TabsFile{Tabs: tabs, ActiveTabID: "missing"}); err != nil {
		t.Fatalf("Save: %v", err)
	}
	got, _ := tm.Load()
	if got.ActiveTabID != "a" {
		t.Fatalf("want active fallback to a, got %q", got.ActiveTabID)
	}
}

func TestTabsTrimsToMaxCount(t *testing.T) {
	tm := NewTabsManager(t.TempDir())
	tabs := make([]TabState, 0, TabMaxCount+10)
	for i := 0; i < TabMaxCount+10; i++ {
		tabs = append(tabs, TabState{ID: "x", Title: "T"})
	}
	if err := tm.Save(TabsFile{Tabs: tabs, ActiveTabID: "x"}); err != nil {
		t.Fatalf("Save: %v", err)
	}
	got, _ := tm.Load()
	if len(got.Tabs) != TabMaxCount {
		t.Fatalf("want %d tabs, got %d", TabMaxCount, len(got.Tabs))
	}
}

func TestTabsEmptySaveClearsActive(t *testing.T) {
	tm := NewTabsManager(t.TempDir())
	if err := tm.Save(TabsFile{Tabs: []TabState{}, ActiveTabID: "whatever"}); err != nil {
		t.Fatalf("Save: %v", err)
	}
	got, _ := tm.Load()
	if got.ActiveTabID != "" {
		t.Fatalf("want empty active id, got %q", got.ActiveTabID)
	}
}

func TestTabsCorruptedFileReturnsError(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "tabs.json"), []byte("{oops"), 0o644); err != nil {
		t.Fatal(err)
	}
	tm := NewTabsManager(dir)
	if _, err := tm.Load(); err == nil {
		t.Fatal("expected error for corrupted tabs file")
	}
}
