package config

import (
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"testing"
)

func newTestSavedSQLManager(t *testing.T) *SavedSQLManager {
	t.Helper()
	return NewSavedSQLManager(t.TempDir())
}

func TestSavedSQLAddAndList(t *testing.T) {
	s := newTestSavedSQLManager(t)

	first, err := s.Add(SavedSQL{SQL: "SELECT 1", ConnectionID: "c1"})
	if err != nil {
		t.Fatalf("Add: %v", err)
	}
	if first.ID == "" {
		t.Fatal("ID should be auto-generated")
	}
	if first.Name != "SELECT 1" {
		t.Fatalf("empty name should fall back to first line, got %q", first.Name)
	}
	if first.CreatedAt.IsZero() || first.UpdatedAt.IsZero() {
		t.Fatal("timestamps should be auto-filled")
	}

	if _, err := s.Add(SavedSQL{Name: "第二条", SQL: "SELECT 2"}); err != nil {
		t.Fatalf("Add: %v", err)
	}

	list, err := s.List()
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(list) != 2 {
		t.Fatalf("want 2 entries, got %d", len(list))
	}
	if list[0].SQL != "SELECT 2" {
		t.Fatalf("newest entry should be first, got %q", list[0].SQL)
	}
}

func TestSavedSQLNoLimit(t *testing.T) {
	s := newTestSavedSQLManager(t)
	total := HistoryMaxEntries + 20
	for i := 0; i < total; i++ {
		if _, err := s.Add(SavedSQL{SQL: "SELECT " + strconv.Itoa(i)}); err != nil {
			t.Fatalf("Add #%d: %v", i, err)
		}
	}
	list, err := s.List()
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(list) != total {
		t.Fatalf("saved sql should not be trimmed, want %d got %d", total, len(list))
	}
}

func TestSavedSQLAddEdgeCases(t *testing.T) {
	cases := []struct {
		name      string
		item      SavedSQL
		wantErrIs error
	}{
		{"空 SQL 被拒绝", SavedSQL{SQL: "   "}, nil},
		{"重复 SQL 被拒绝", SavedSQL{SQL: "SELECT 1"}, ErrSavedSQLDuplicate},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			s := newTestSavedSQLManager(t)
			if _, err := s.Add(SavedSQL{SQL: "SELECT 1"}); err != nil {
				t.Fatalf("seed Add: %v", err)
			}
			_, err := s.Add(c.item)
			if err == nil {
				t.Fatal("expected error, got nil")
			}
			if c.wantErrIs != nil && !errors.Is(err, c.wantErrIs) {
				t.Fatalf("want %v, got %v", c.wantErrIs, err)
			}
		})
	}
}

func TestSavedSQLDuplicateIgnoresWhitespace(t *testing.T) {
	s := newTestSavedSQLManager(t)
	if _, err := s.Add(SavedSQL{SQL: "SELECT 1"}); err != nil {
		t.Fatalf("seed Add: %v", err)
	}
	if _, err := s.Add(SavedSQL{SQL: "  SELECT 1  \n"}); !errors.Is(err, ErrSavedSQLDuplicate) {
		t.Fatalf("want duplicate error, got %v", err)
	}
}

func TestSavedSQLUpdate(t *testing.T) {
	s := newTestSavedSQLManager(t)
	item, err := s.Add(SavedSQL{Name: "旧名称", SQL: "SELECT 1"})
	if err != nil {
		t.Fatalf("Add: %v", err)
	}

	updated, err := s.Update(item.ID, "新名称", "SELECT 2")
	if err != nil {
		t.Fatalf("Update: %v", err)
	}
	if updated.Name != "新名称" || updated.SQL != "SELECT 2" {
		t.Fatalf("unexpected updated item: %+v", updated)
	}
	if updated.CreatedAt != item.CreatedAt {
		t.Fatal("CreatedAt should be preserved")
	}
	if !updated.UpdatedAt.After(item.UpdatedAt) && !updated.UpdatedAt.Equal(item.UpdatedAt) {
		t.Fatal("UpdatedAt should not go backwards")
	}

	if _, err := s.Update("missing-id", "x", "SELECT 3"); !errors.Is(err, ErrSavedSQLNotFound) {
		t.Fatalf("want not found error, got %v", err)
	}
	if _, err := s.Update(item.ID, "空 SQL", "  "); err == nil {
		t.Fatal("empty sql should be rejected on update")
	}
}

func TestSavedSQLUpdateEmptyNameFallsBackToFirstLine(t *testing.T) {
	s := newTestSavedSQLManager(t)
	item, _ := s.Add(SavedSQL{Name: "原名", SQL: "SELECT 1"})
	updated, err := s.Update(item.ID, "", "  SELECT 42\nFROM t")
	if err != nil {
		t.Fatalf("Update: %v", err)
	}
	if updated.Name != "SELECT 42" {
		t.Fatalf("want first line as name, got %q", updated.Name)
	}
}

func TestSavedSQLDelete(t *testing.T) {
	s := newTestSavedSQLManager(t)
	a, _ := s.Add(SavedSQL{SQL: "A"})
	b, _ := s.Add(SavedSQL{SQL: "B"})

	if err := s.Delete(a.ID); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	list, _ := s.List()
	if len(list) != 1 || list[0].ID != b.ID {
		t.Fatalf("unexpected list after delete: %+v", list)
	}

	if err := s.Delete("missing-id"); !errors.Is(err, ErrSavedSQLNotFound) {
		t.Fatalf("want not found error, got %v", err)
	}
}

func TestSavedSQLSurvivesHistoryClear(t *testing.T) {
	dir := t.TempDir()
	s := NewSavedSQLManager(dir)
	if _, err := s.Add(SavedSQL{SQL: "SELECT keep"}); err != nil {
		t.Fatalf("Add: %v", err)
	}

	h := NewHistoryManager(dir)
	_ = h.Add(HistoryEntry{SQL: "SELECT temp"})
	if err := h.Clear(); err != nil {
		t.Fatalf("Clear history: %v", err)
	}

	list, err := s.List()
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(list) != 1 {
		t.Fatalf("saved sql must not be touched by history clear, got %d", len(list))
	}
}

func TestSavedSQLReloadFromDisk(t *testing.T) {
	dir := t.TempDir()
	s1 := NewSavedSQLManager(dir)
	if _, err := s1.Add(SavedSQL{SQL: "SELECT 1"}); err != nil {
		t.Fatalf("Add: %v", err)
	}
	if _, err := s1.Add(SavedSQL{SQL: "SELECT 2"}); err != nil {
		t.Fatalf("Add: %v", err)
	}

	s2 := NewSavedSQLManager(dir)
	list, err := s2.List()
	if err != nil {
		t.Fatalf("List on reload: %v", err)
	}
	if len(list) != 2 || list[0].SQL != "SELECT 2" {
		t.Fatalf("unexpected reloaded list: %+v", list)
	}
	if _, err := os.Stat(filepath.Join(dir, "saved_sql.json")); err != nil {
		t.Fatalf("saved_sql.json not written: %v", err)
	}
}

func TestSavedSQLEmptyFileAndCorrupted(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "saved_sql.json")

	if err := os.WriteFile(path, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	s := NewSavedSQLManager(dir)
	list, err := s.List()
	if err != nil {
		t.Fatalf("List on empty file: %v", err)
	}
	if len(list) != 0 {
		t.Fatalf("want empty list, got %d", len(list))
	}

	if err := os.WriteFile(path, []byte("{not json"), 0o644); err != nil {
		t.Fatal(err)
	}
	s2 := NewSavedSQLManager(dir)
	if _, err := s2.List(); err == nil {
		t.Fatal("corrupted file should return error")
	}
}
