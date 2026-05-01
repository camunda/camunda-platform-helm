package matrix

import (
	"reflect"
	"testing"
)

func TestParseHelmSetPairs(t *testing.T) {
	tests := []struct {
		name string
		in   []string
		want map[string]string
	}{
		{name: "nil input", in: nil, want: nil},
		{name: "empty slice", in: []string{}, want: nil},
		{
			name: "single pair",
			in:   []string{"a=1"},
			want: map[string]string{"a": "1"},
		},
		{
			name: "multiple pairs",
			in:   []string{"a=1", "b=two", "c.d=three=four"},
			want: map[string]string{"a": "1", "b": "two", "c.d": "three=four"},
		},
		{
			name: "skip malformed entries (no =, leading =)",
			in:   []string{"valid=1", "noequals", "=onlyvalue"},
			want: map[string]string{"valid": "1"},
		},
		{
			name: "all malformed → nil",
			in:   []string{"x", "y"},
			want: nil,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := parseHelmSetPairs(tc.in)
			if !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("got %#v, want %#v", got, tc.want)
			}
		})
	}
}

func TestMergeHelmSets(t *testing.T) {
	tests := []struct {
		name           string
		base, override map[string]string
		want           map[string]string
	}{
		{name: "both nil", want: nil},
		{
			name: "base only",
			base: map[string]string{"a": "1"},
			want: map[string]string{"a": "1"},
		},
		{
			name:     "override only",
			override: map[string]string{"a": "1"},
			want:     map[string]string{"a": "1"},
		},
		{
			name:     "override wins on conflict",
			base:     map[string]string{"a": "old", "b": "kept"},
			override: map[string]string{"a": "new"},
			want:     map[string]string{"a": "new", "b": "kept"},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := mergeHelmSets(tc.base, tc.override)
			if !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("got %#v, want %#v", got, tc.want)
			}
		})
	}
}
