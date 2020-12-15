package gen

import "testing"

func Test_makeValidIdentifier(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"&opts0", "&opts0"},
		{"8", "_8"},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			if got := makeValidIdentifier(tt.input); got != tt.expected {
				t.Errorf("makeValidIdentifier() = %v, want %v", got, tt.expected)
			}
		})
	}
}
