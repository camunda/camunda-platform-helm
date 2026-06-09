package harbortag

import (
	"reflect"
	"testing"
)

func TestIsRolling(t *testing.T) {
	tests := []struct {
		tag  string
		kind Kind
		want bool
	}{
		{"13-dev-latest", Dev, true},
		{"13.4.0-dev-abc1234", Dev, false},
		{"13-rc-latest", RC, true},
		{"13.4.0-rc", RC, false},
		{"13-dev-latest", RC, false},
	}
	for _, tt := range tests {
		if got := IsRolling(tt.tag, tt.kind); got != tt.want {
			t.Errorf("IsRolling(%q,%s)=%v want %v", tt.tag, tt.kind, got, tt.want)
		}
	}
}

func TestResolveConcrete(t *testing.T) {
	tags := []string{"13-dev-latest", "garbage", "13.4.0-dev-abc1234", "13.5.0-dev-def5678"}
	got, err := ResolveConcrete(tags, Dev)
	if err != nil {
		t.Fatalf("ResolveConcrete: %v", err)
	}
	if got != "13.4.0-dev-abc1234" { // first concrete, mirrors `head -1`
		t.Errorf("ResolveConcrete dev = %q, want 13.4.0-dev-abc1234", got)
	}
	if _, err := ResolveConcrete([]string{"13-dev-latest"}, Dev); err == nil {
		t.Error("expected error when no concrete dev tag present")
	}
}

func TestParseDevTag(t *testing.T) {
	tests := []struct {
		tag  string
		want DevTag
	}{
		{
			"13.4.0-dev-abc1234",
			DevTag{ResolvedTag: "13.4.0-dev-abc1234", Version: "13.4.0", SHA: "abc1234", ChartMajor: "13", RCTag: "13.4.0-rc", RCLatestTag: "13-rc-latest"},
		},
		{
			"14.0.0-alpha2-dev-deadbeef",
			DevTag{ResolvedTag: "14.0.0-alpha2-dev-deadbeef", Version: "14.0.0-alpha2", SHA: "deadbeef", ChartMajor: "14", RCTag: "14.0.0-alpha2-rc", RCLatestTag: "14-rc-latest"},
		},
	}
	for _, tt := range tests {
		got, err := ParseDevTag(tt.tag)
		if err != nil {
			t.Fatalf("ParseDevTag(%q): %v", tt.tag, err)
		}
		if !reflect.DeepEqual(got, tt.want) {
			t.Errorf("ParseDevTag(%q)=%+v want %+v", tt.tag, got, tt.want)
		}
	}
	for _, bad := range []string{"13.4.0-rc", "13-dev-latest", "13.4-dev-abc", "not-a-tag"} {
		if _, err := ParseDevTag(bad); err == nil {
			t.Errorf("ParseDevTag(%q) expected error", bad)
		}
	}
}

func TestParseRcTag(t *testing.T) {
	got, err := ParseRcTag("14.0.0-alpha2-rc")
	if err != nil {
		t.Fatalf("ParseRcTag: %v", err)
	}
	want := RcTag{ResolvedTag: "14.0.0-alpha2-rc", Version: "14.0.0-alpha2", ChartMajor: "14"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("ParseRcTag=%+v want %+v", got, want)
	}
	if _, err := ParseRcTag("13.4.0-dev-abc1234"); err == nil {
		t.Error("expected error for dev tag passed to ParseRcTag")
	}
}

func TestFindDevTagAndCommitSHA(t *testing.T) {
	tags := []string{"13.4.0-rc", "13-rc-latest", "13.4.0-dev-abc1234", "13-dev-latest"}
	dev := FindDevTag(tags)
	if dev != "13.4.0-dev-abc1234" {
		t.Errorf("FindDevTag=%q want 13.4.0-dev-abc1234", dev)
	}
	if sha := CommitSHAFromDevTag(dev); sha != "abc1234" {
		t.Errorf("CommitSHAFromDevTag=%q want abc1234", sha)
	}
	if FindDevTag([]string{"13.4.0-rc"}) != "" {
		t.Error("FindDevTag want empty when no dev tag")
	}
}
