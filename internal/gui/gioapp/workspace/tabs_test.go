package workspace

import "testing"

func TestIndexOfUsesStableIDAfterRemoval(t *testing.T) {
	ids, ok := RemoveAt([]TabID{11, 22, 33}, 1)
	if !ok { t.Fatal("expected removal to succeed") }
	index, found := IndexOf(ids, 33)
	if !found || index != 1 { t.Fatalf("IndexOf(33) = (%d, %v), want (1, true)", index, found) }
	if _, found := IndexOf(ids, 22); found { t.Fatal("removed tab ID must not be found") }
}

func TestRemoveAtRejectsInvalidIndex(t *testing.T) {
	original := []TabID{1, 2}
	for _, index := range []int{-1, 2} {
		got, ok := RemoveAt(original, index)
		if ok { t.Fatalf("RemoveAt(%d) unexpectedly succeeded", index) }
		if len(got) != len(original) || got[0] != 1 || got[1] != 2 { t.Fatalf("invalid removal changed IDs: %v", got) }
	}
}

func TestIndexOfMissingID(t *testing.T) {
	if _, ok := IndexOf([]TabID{1, 2}, 3); ok { t.Fatal("missing ID must not be found") }
}
