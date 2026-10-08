package handlers

import (
	"testing"
)

func TestExtractGameID(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    int32
		wantErr bool
	}{
		{name: "Pure number", input: "1", want: 1, wantErr: false},
		{name: "Pure number larger", input: "42", want: 42, wantErr: false},
		{name: "URL without scheme", input: "verleih.fius.de/1", want: 1, wantErr: false},
		{name: "HTTPS URL", input: "https://verleih.fius.de/1", want: 1, wantErr: false},
		{name: "HTTP URL with trailing slash", input: "http://verleih.fius.de/123/", want: 123, wantErr: false},
		{name: "Subpath URL", input: "https://verleih.fius.de/games/42", want: 42, wantErr: false},
		{name: "Query param id", input: "https://verleih.fius.de?id=105", want: 105, wantErr: false},
		{name: "Query param id in query string", input: "https://verleih.fius.de/lookup?id=99&source=scan", want: 99, wantErr: false},
		{name: "Prefixed with ID: #", input: "ID: #105", want: 105, wantErr: false},
		{name: "Prefixed with hash", input: "#42", want: 42, wantErr: false},
		{name: "Prefixed with ID", input: "ID 42", want: 42, wantErr: false},
		{name: "URL with hash fragment", input: "verleih.fius.de/77#details", want: 77, wantErr: false},
		{name: "Whitespace padded", input: "   https://verleih.fius.de/204   ", want: 204, wantErr: false},
		{name: "Empty string", input: "", want: 0, wantErr: true},
		{name: "Invalid text without numbers", input: "verleih.fius.de/abc", want: 0, wantErr: true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := ExtractGameID(tc.input)
			if (err != nil) != tc.wantErr {
				t.Fatalf("ExtractGameID(%q) error = %v, wantErr %v", tc.input, err, tc.wantErr)
			}
			if got != tc.want {
				t.Errorf("ExtractGameID(%q) = %d, want %d", tc.input, got, tc.want)
			}
		})
	}
}
