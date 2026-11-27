package app

import "testing"

func TestResultSortKeyCacheLifecycle(t *testing.T) {
	rows := [][]any{{"a", 30}, {"b", 10}, {"c", 20}}
	var cache resultSortKeyCache

	first := cache.Prepare(rows, 1)
	if len(first) != len(rows) || first[0].number == nil || first[0].number.RatString() != "30" {
		t.Fatalf("initial sort cache = %#v", first)
	}
	firstAddress := &first[0]
	if got := cache.Prepare(rows, 1); &got[0] != firstAddress {
		t.Fatal("preparing the same column should reuse the existing cache")
	}

	switched := cache.Prepare(rows, 0)
	if len(switched) != len(rows) || switched[0].text != "a" || switched[0].numeric {
		t.Fatalf("switching sort column reused stale keys: %#v", switched)
	}
	if &switched[0] == firstAddress {
		t.Fatal("switching sort columns should discard the previous column cache")
	}

	shrunk := cache.Prepare(rows[:1], 0)
	if len(shrunk) != 1 || shrunk[0].text != "a" {
		t.Fatalf("shrinking rows retained stale sort keys: %#v", shrunk)
	}

	if got := cache.Prepare(rows, -1); got != nil {
		t.Fatalf("disabled sorting returned keys: %#v", got)
	}
	if cache.valid || cache.keys != nil {
		t.Fatalf("disabled sorting should clear cache state: %#v", cache)
	}
}
