package cache

import (
	"testing"
	"time"
)

func TestParseDescription(t *testing.T) {
	tests := []struct {
		name    string
		desc    string
		want    Entry
		wantErr bool
	}{
		{
			name: "valid description",
			desc: "hash:abc123def456,ts:1700000000",
			want: Entry{
				Hash:      "abc123def456",
				Timestamp: time.Unix(1700000000, 0),
			},
		},
		{
			name:    "missing hash",
			desc:    "ts:1700000000",
			wantErr: true,
		},
		{
			name:    "invalid timestamp",
			desc:    "hash:abc123,ts:notanumber",
			wantErr: true,
		},
		{
			name: "hash only",
			desc: "hash:abc123",
			want: Entry{
				Hash:      "abc123",
				Timestamp: time.Time{},
			},
		},
		{
			name:    "empty description",
			desc:    "",
			wantErr: true,
		},
		{
			name:    "invalidated marker",
			desc:    "invalidated",
			wantErr: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := ParseDescription(tc.desc)
			if tc.wantErr {
				if err == nil {
					t.Error("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got.Hash != tc.want.Hash {
				t.Errorf("hash: got %q, want %q", got.Hash, tc.want.Hash)
			}
			if !got.Timestamp.Equal(tc.want.Timestamp) {
				t.Errorf("timestamp: got %v, want %v", got.Timestamp, tc.want.Timestamp)
			}
		})
	}
}

func TestFormatDescription(t *testing.T) {
	ts := time.Unix(1700000000, 0)
	got := FormatDescription("abc123", ts)
	want := "hash:abc123,ts:1700000000"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestStatusContext(t *testing.T) {
	got := StatusContext("8.9", "oske", "install")
	want := "ci-cache/8.9/oske/install"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestCheck(t *testing.T) {
	now := time.Now()
	recentTS := now.Add(-1 * time.Hour)
	oldTS := now.Add(-48 * time.Hour)

	statuses := []commitStatus{
		{
			State:       "success",
			Context:     "ci-cache/8.9/oske/install",
			Description: FormatDescription("hash_a", recentTS),
		},
		{
			State:       "success",
			Context:     "ci-cache/8.9/eske/upgrade-minor",
			Description: FormatDescription("hash_b", oldTS),
		},
		{
			State:       "error",
			Context:     "ci-cache/8.9/kemt/install",
			Description: "invalidated",
		},
	}

	tests := []struct {
		name        string
		context     string
		currentHash string
		ttl         time.Duration
		want        bool
	}{
		{
			name:        "matching hash, within TTL",
			context:     "ci-cache/8.9/oske/install",
			currentHash: "hash_a",
			ttl:         24 * time.Hour,
			want:        true,
		},
		{
			name:        "matching hash, TTL disabled",
			context:     "ci-cache/8.9/oske/install",
			currentHash: "hash_a",
			ttl:         0,
			want:        true,
		},
		{
			name:        "hash mismatch",
			context:     "ci-cache/8.9/oske/install",
			currentHash: "different_hash",
			ttl:         24 * time.Hour,
			want:        false,
		},
		{
			name:        "matching hash, expired TTL",
			context:     "ci-cache/8.9/eske/upgrade-minor",
			currentHash: "hash_b",
			ttl:         24 * time.Hour,
			want:        false,
		},
		{
			name:        "matching hash, expired but TTL disabled",
			context:     "ci-cache/8.9/eske/upgrade-minor",
			currentHash: "hash_b",
			ttl:         0,
			want:        true,
		},
		{
			name:        "invalidated entry",
			context:     "ci-cache/8.9/kemt/install",
			currentHash: "any_hash",
			ttl:         24 * time.Hour,
			want:        false,
		},
		{
			name:        "non-existent context",
			context:     "ci-cache/8.9/nosec/install",
			currentHash: "any_hash",
			ttl:         24 * time.Hour,
			want:        false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := Check(statuses, tc.context, tc.currentHash, tc.ttl)
			if got != tc.want {
				t.Errorf("got %v, want %v", got, tc.want)
			}
		})
	}
}

func TestFormatAndParseRoundTrip(t *testing.T) {
	hash := "abcdef1234567890abcdef1234567890abcdef1234567890abcdef1234567890"
	ts := time.Unix(1700000000, 0)

	desc := FormatDescription(hash, ts)
	entry, err := ParseDescription(desc)
	if err != nil {
		t.Fatalf("ParseDescription: %v", err)
	}

	if entry.Hash != hash {
		t.Errorf("hash: got %q, want %q", entry.Hash, hash)
	}
	if !entry.Timestamp.Equal(ts) {
		t.Errorf("timestamp: got %v, want %v", entry.Timestamp, ts)
	}
}
