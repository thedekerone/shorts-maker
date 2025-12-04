package services

import "testing"

func TestOutputToStrings(t *testing.T) {
	tests := []struct {
		name  string
		input any
		want  []string
	}{
		{
			name:  "slice of strings",
			input: []any{"one", "two"},
			want:  []string{"one", "two"},
		},
		{
			name:  "single string",
			input: "hello",
			want:  []string{"hello"},
		},
		{
			name:  "mixed slice",
			input: []any{"one", 2},
			want:  nil,
		},
		{
			name:  "unsupported type",
			input: map[string]string{"a": "b"},
			want:  nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := outputToStrings(tt.input)
			if len(got) != len(tt.want) {
				if !(got == nil && tt.want == nil) {
					t.Fatalf("length mismatch: got %v want %v", got, tt.want)
				}
			} else {
				for i := range got {
					if got[i] != tt.want[i] {
						t.Fatalf("index %d: got %q want %q", i, got[i], tt.want[i])
					}
				}
			}
		})
	}
}

func TestEstimateTokens(t *testing.T) {
	tests := []struct {
		word string
		want int
	}{
		{"a", 1},
		{"abcd", 2},
		{"abcdefgh", 3},
	}

	for _, tt := range tests {
		if got := estimateTokens(tt.word); got != tt.want {
			t.Fatalf("estimateTokens(%q) = %d, want %d", tt.word, got, tt.want)
		}
	}
}
