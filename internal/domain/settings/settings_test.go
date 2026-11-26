package settings

import "testing"

func TestNormalizeRowLimit(t *testing.T) {
	cases := []struct{name string; input, want int}{
		{"unlimited", 0, 0},
		{"negative", -1, DefaultRowLimit},
		{"below minimum", 99, DefaultRowLimit},
		{"minimum", 100, 100},
		{"valid", 5000, 5000},
		{"above maximum", 1000001, RowLimitMax},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := NormalizeRowLimit(tc.input); got != tc.want { t.Fatalf("got %d, want %d", got, tc.want) }
		})
	}
}

func TestNormalizeTheme(t *testing.T) {
	if got := Normalize(Preferences{Theme: ThemeDark}).Theme; got != ThemeDark {
		t.Fatalf("dark theme changed: %q", got)
	}
	for _, input := range []string{"", "system", "unknown"} {
		if got := Normalize(Preferences{Theme: input}).Theme; got != ThemeLight {
			t.Fatalf("theme %q normalized to %q, want light", input, got)
		}
	}
}

func TestNormalizePreferences(t *testing.T) {
	got := Normalize(Preferences{QueryRowLimit: 5000, SidebarCollapsed: true, EditorRatio: 0.7})
	if got.QueryRowLimit != 5000 || !got.SidebarCollapsed || got.EditorRatio != 0.7 { t.Fatalf("valid preferences changed: %+v", got) }
	got = Normalize(Preferences{QueryRowLimit: -1, EditorRatio: 0.9})
	if got.QueryRowLimit != DefaultRowLimit || got.EditorRatio != DefaultEditorRatio { t.Fatalf("invalid preferences not normalized: %+v", got) }
}
