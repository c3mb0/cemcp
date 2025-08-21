package main

import (
	"encoding/json"
	"testing"
)

func TestSummaryForAlgorithm(t *testing.T) {
	tests := []struct {
		name  string
		input interface{}
		want  float64
	}{
		{"float", 1.23, 1.23},
		{"jsonNumber", json.Number("4.56"), 4.56},
		{"string", "7.89", 7.89},
		{"invalidString", "not-a-number", 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := summaryForAlgorithm(tt.input)
			if got != tt.want {
				t.Fatalf("summaryForAlgorithm(%v)=%v, want %v", tt.input, got, tt.want)
			}
		})
	}
}
