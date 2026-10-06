package tests

import (
	"testing"

	"leiheV2/internal/bggutil"
)

func TestParseBGGRating(t *testing.T) {
	tests := []struct {
		value string
		want  float64
		valid bool
	}{
		{value: "8.1", want: 8.1, valid: true},
		{value: "1.0", want: 1, valid: true},
		{value: "10", want: 10, valid: true},
		{value: "N/A", valid: false},
		{value: "0", valid: false},
		{value: "10.1", valid: false},
	}
	for _, test := range tests {
		got, valid := bggutil.ParseRating(test.value)
		if valid != test.valid || (valid && got != test.want) {
			t.Errorf("ParseRating(%q) = (%v, %v), want (%v, %v)", test.value, got, valid, test.want, test.valid)
		}
	}
}
