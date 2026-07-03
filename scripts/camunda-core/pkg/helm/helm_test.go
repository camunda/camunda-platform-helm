package helm

import "testing"

func TestDetectWaitFlagFromVersionOutput(t *testing.T) {
	tests := []struct {
		name string
		out  string
		want string
	}{
		{name: "helm v3 short", out: "v3.20.1+g8a36c9b\n", want: "--wait"},
		{name: "helm v4 short", out: "v4.1.4+g05fa379\n", want: "--wait=legacy"},
		{name: "leading whitespace", out: "  v4.0.0\n", want: "--wait=legacy"},
		{name: "empty falls back to wait", out: "", want: "--wait"},
		{name: "garbage falls back to wait", out: "not a version\n", want: "--wait"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := waitFlagFromOutput([]byte(tt.out)); got != tt.want {
				t.Errorf("waitFlagFromOutput(%q) = %q, want %q", tt.out, got, tt.want)
			}
		})
	}
}
