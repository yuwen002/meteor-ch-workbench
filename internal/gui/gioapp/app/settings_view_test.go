package app

import "testing"

func TestParseSettingsRowLimit(t *testing.T) {
	cases := []struct{name, input string; want int; wantErr bool}{
		{"unlimited", "0", 0, false},
		{"minimum", "100", 100, false},
		{"maximum", "1000000", 1000000, false},
		{"whitespace", " 5000 ", 5000, false},
		{"below minimum", "99", 0, true},
		{"above maximum", "1000001", 0, true},
		{"not integer", "10k", 0, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := parseSettingsRowLimit(tc.input)
			if (err != nil) != tc.wantErr { t.Fatalf("error = %v, wantErr %v", err, tc.wantErr) }
			if got != tc.want { t.Fatalf("value = %d, want %d", got, tc.want) }
		})
	}
}

func TestParseSettingsEditorRatio(t *testing.T) {
	for _, tc := range []struct{input string; want float64; wantErr bool}{
		{"0.45", 0.45, false},
		{"0.35", 0.35, false},
		{"0.80", 0.80, false},
		{"0.34", 0, true},
		{"0.81", 0, true},
		{"auto", 0, true},
	} {
		got, err := parseSettingsEditorRatio(tc.input)
		if (err != nil) != tc.wantErr { t.Fatalf("input %q error = %v", tc.input, err) }
		if got != tc.want { t.Fatalf("input %q got %v, want %v", tc.input, got, tc.want) }
	}
}
