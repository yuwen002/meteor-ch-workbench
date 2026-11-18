package config

import (
	"os"
	"path/filepath"
	"testing"
)

func newTestHistoryManager(t *testing.T) *HistoryManager {
	t.Helper()
	h := NewHistoryManager(t.TempDir())
	return h
}

func TestHistoryAddPrependsAndPersists(t *testing.T) {
	h := newTestHistoryManager(t)

	if err := h.Add(HistoryEntry{SQL: "SELECT 1", ConnectionID: "c1", Success: true}); err != nil {
		t.Fatalf("Add: %v", err)
	}
	if err := h.Add(HistoryEntry{SQL: "SELECT 2", ConnectionID: "c1", Success: true}); err != nil {
		t.Fatalf("Add: %v", err)
	}

	list, err := h.List()
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(list) != 2 {
		t.Fatalf("want 2 entries, got %d", len(list))
	}
	if list[0].SQL != "SELECT 2" {
		t.Fatalf("newest entry should be first, got %q", list[0].SQL)
	}
	for _, e := range list {
		if e.ID == "" {
			t.Fatal("entry ID should be auto-generated")
		}
		if e.Timestamp.IsZero() {
			t.Fatal("entry Timestamp should be auto-filled")
		}
	}
}

func TestHistoryRejectsNothingAndTrimsToLimit(t *testing.T) {
	h := newTestHistoryManager(t)
	for i := 0; i < HistoryMaxEntries+5; i++ {
		if err := h.Add(HistoryEntry{SQL: "SELECT " + string(rune('a'+i%26)), ConnectionID: "c"}); err != nil {
			t.Fatalf("Add #%d: %v", i, err)
		}
	}
	list, err := h.List()
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(list) != HistoryMaxEntries {
		t.Fatalf("want %d entries, got %d", HistoryMaxEntries, len(list))
	}
}

func TestHistoryClearAndDelete(t *testing.T) {
	h := newTestHistoryManager(t)
	_ = h.Add(HistoryEntry{SQL: "A", ConnectionID: "c"})
	_ = h.Add(HistoryEntry{SQL: "B", ConnectionID: "c"})

	list, _ := h.List()
	if len(list) != 2 {
		t.Fatalf("want 2 entries, got %d", len(list))
	}
	if err := h.Delete(list[0].ID); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	list, _ = h.List()
	if len(list) != 1 || list[0].SQL != "A" {
		t.Fatalf("unexpected list after delete: %+v", list)
	}

	if err := h.Clear(); err != nil {
		t.Fatalf("Clear: %v", err)
	}
	list, _ = h.List()
	if len(list) != 0 {
		t.Fatalf("want empty after clear, got %d", len(list))
	}
}

func TestHistoryReloadFromDisk(t *testing.T) {
	dir := t.TempDir()
	h1 := NewHistoryManager(dir)
	_ = h1.Add(HistoryEntry{SQL: "SELECT 42", ConnectionID: "c"})
	_ = h1.Add(HistoryEntry{SQL: "SELECT 43", ConnectionID: "c"})

	h2 := NewHistoryManager(dir)
	list, err := h2.List()
	if err != nil {
		t.Fatalf("List on reload: %v", err)
	}
	if len(list) != 2 || list[0].SQL != "SELECT 43" {
		t.Fatalf("unexpected reloaded list: %+v", list)
	}
	if _, err := os.Stat(filepath.Join(dir, "history.json")); err != nil {
		t.Fatalf("history.json not written: %v", err)
	}
}

func TestHistoryCorruptedFileReturnsError(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "history.json"), []byte("{not json"), 0o644); err != nil {
		t.Fatal(err)
	}
	h := NewHistoryManager(dir)
	if _, err := h.List(); err == nil {
		t.Fatal("expected error for corrupted history file")
	}
}
