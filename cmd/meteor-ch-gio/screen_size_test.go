package main

import "testing"

func TestFitInitialWindowSize(t *testing.T) {
	tests := []struct {
		name          string
		width, height int
		wantWidth     float32
		wantHeight    float32
	}{
		{name: "large display", width: 2560, height: 1440, wantWidth: 1440, wantHeight: 1000},
		{name: "standard display", width: 1920, height: 1080, wantWidth: 1440, wantHeight: 885},
		{name: "small display", width: 800, height: 600, wantWidth: 760, wantHeight: 570},
		{name: "invalid dimensions use fallback", width: 0, height: 0, wantWidth: 885, wantHeight: 541},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			width, height := fitInitialWindowSize(tt.width, tt.height)
			if float32(width) != tt.wantWidth || float32(height) != tt.wantHeight {
				t.Fatalf("fitInitialWindowSize() = %v x %v, want %v x %v", width, height, tt.wantWidth, tt.wantHeight)
			}
		})
	}
}
