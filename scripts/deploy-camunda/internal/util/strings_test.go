package util

import "testing"

func TestFirstNonEmpty(t *testing.T) {
	tests := []struct {
		name   string
		vals   []string
		expect string
	}{
		{
			name:   "first non-empty wins",
			vals:   []string{"first", "second", "third"},
			expect: "first",
		},
		{
			name:   "skips empty strings",
			vals:   []string{"", "", "third"},
			expect: "third",
		},
		{
			name:   "skips whitespace-only strings",
			vals:   []string{"  ", "\t", "third"},
			expect: "third",
		},
		{
			name:   "returns empty when all empty",
			vals:   []string{"", "  ", ""},
			expect: "",
		},
		{
			name:   "no values returns empty",
			vals:   []string{},
			expect: "",
		},
		{
			name:   "nil returns empty",
			vals:   nil,
			expect: "",
		},
		{
			name:   "single value",
			vals:   []string{"only"},
			expect: "only",
		},
		{
			name:   "preserves value with leading/trailing spaces",
			vals:   []string{"", " value "},
			expect: " value ",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := FirstNonEmpty(tt.vals...)
			if result != tt.expect {
				t.Errorf("FirstNonEmpty(%v) = %q, want %q", tt.vals, result, tt.expect)
			}
		})
	}
}

func TestIsEmpty(t *testing.T) {
	tests := []struct {
		input  string
		expect bool
	}{
		{"", true},
		{"  ", true},
		{"\t", true},
		{"\n", true},
		{" \t\n ", true},
		{"a", false},
		{" a ", false},
		{"hello", false},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := IsEmpty(tt.input)
			if result != tt.expect {
				t.Errorf("IsEmpty(%q) = %v, want %v", tt.input, result, tt.expect)
			}
		})
	}
}

func TestIsNotEmpty(t *testing.T) {
	tests := []struct {
		input  string
		expect bool
	}{
		{"", false},
		{"  ", false},
		{"\t", false},
		{"a", true},
		{" a ", true},
		{"hello", true},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := IsNotEmpty(tt.input)
			if result != tt.expect {
				t.Errorf("IsNotEmpty(%q) = %v, want %v", tt.input, result, tt.expect)
			}
		})
	}
}

