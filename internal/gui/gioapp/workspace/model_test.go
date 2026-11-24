package workspace

import "testing"

func TestTabsCreateActivateAndCloseByStableID(t *testing.T) {
	tabs := NewTabs()
	first := tabs.ActiveID()
	second := tabs.Create()
	third := tabs.Create()
	if tabs.ActiveID() != third.ID {
		t.Fatalf("active ID = %d, want %d", tabs.ActiveID(), third.ID)
	}
	if !tabs.Activate(first) {
		t.Fatal("expected first tab activation to succeed")
	}
	if _, ok := tabs.Close(second.ID); !ok {
		t.Fatal("expected closing second tab to succeed")
	}
	if got, ok := tabs.Get(third.ID); !ok || got.ID != third.ID {
		t.Fatalf("third tab was not preserved after removal: (%+v, %v)", got, ok)
	}
	if index, ok := tabs.ActiveIndex(); !ok || index != 0 {
		t.Fatalf("active index = (%d, %v), want (0, true)", index, ok)
	}
}

func TestTabsCloseActiveSelectsRightOrLast(t *testing.T) {
	tabs := NewTabs()
	first := tabs.ActiveID()
	second := tabs.Create()
	third := tabs.Create()
	if _, ok := tabs.Close(second.ID); !ok {
		t.Fatal("expected close to succeed")
	}
	if tabs.ActiveID() != third.ID {
		t.Fatalf("closing active middle tab selected %d, want %d", tabs.ActiveID(), third.ID)
	}
	if _, ok := tabs.Close(third.ID); !ok {
		t.Fatal("expected closing active last tab to succeed")
	}
	if tabs.ActiveID() != first {
		t.Fatalf("closing last tab selected %d, want %d", tabs.ActiveID(), first)
	}
	if _, ok := tabs.Close(first); ok {
		t.Fatal("must keep at least one tab")
	}
}

func TestTabsEditAndReorder(t *testing.T) {
	tabs := NewTabs()
	first := tabs.ActiveID()
	second := tabs.Create()
	if !tabs.UpdateSQL(first, "SELECT 1") {
		t.Fatal("expected SQL update to succeed")
	}
	if tab, _ := tabs.Get(first); tab.SQL != "SELECT 1" || !tab.Dirty {
		t.Fatalf("updated tab = %+v, want dirty SQL tab", tab)
	}
	if !tabs.MarkSaved(first) {
		t.Fatal("expected mark saved to succeed")
	}
	if tab, _ := tabs.Get(first); tab.Dirty {
		t.Fatal("saved tab must not be dirty")
	}
	if !tabs.Rename(first, "查询报表") {
		t.Fatal("expected rename to succeed")
	}
	if tabs.Rename(first, " ") {
		t.Fatal("blank title must be rejected")
	}
	if !tabs.Reorder(second.ID, 0) {
		t.Fatal("expected reorder to succeed")
	}
	items := tabs.Items()
	if items[0].ID != second.ID || items[1].ID != first {
		t.Fatalf("reordered IDs = [%d, %d], want [%d, %d]", items[0].ID, items[1].ID, second.ID, first)
	}
	if !tabs.Activate(first) {
		t.Fatal("active tab identity must survive reorder")
	}
	if index, ok := tabs.ActiveIndex(); !ok || index != 1 {
		t.Fatalf("active index after reorder = (%d, %v), want (1, true)", index, ok)
	}
}

func TestTabsRejectUnknownIDsAndInvalidIndexes(t *testing.T) {
	tabs := NewTabs()
	if tabs.Activate(999) || tabs.UpdateSQL(999, "SELECT 1") || tabs.MarkSaved(999) || tabs.Rename(999, "x") || tabs.Reorder(999, 0) {
		t.Fatal("operations on unknown tab ID must fail")
	}
	if tabs.Reorder(tabs.ActiveID(), 1) {
		t.Fatal("reorder to an out-of-range index must fail")
	}
	if _, ok := tabs.Get(999); ok {
		t.Fatal("unknown tab ID must not be found")
	}
}
