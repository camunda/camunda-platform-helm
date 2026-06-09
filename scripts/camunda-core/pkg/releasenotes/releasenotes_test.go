package releasenotes

import (
	"strings"
	"testing"
)

func TestHelmCLIVersion(t *testing.T) {
	cases := []struct {
		app, pin, want string
	}{
		{"8.8", "4.1.4", "3.20.2"},       // v3-only minor, pin is v4 → clamp
		{"8.8", "3.20.1", "3.20.1"},      // v3-only minor, pin is v3 → pin
		{"8.0", "4.1.4", "3.20.2"},       // lower bound of clamp range
		{"8.9", "4.1.4", "3.20.2,4.1.4"}, // transitional minor, v4 pin → dual
		{"8.9", "3.20.1", "3.20.1"},      // transitional, v3 pin → pin
		{"8.10", "4.1.4", "4.1.4"},       // 8.10+ → pin as-is
		{"8.11", "4.1.4", "4.1.4"},
		{"8.7", "4.1.4", "3.20.2"},
	}
	for _, c := range cases {
		if got := HelmCLIVersion(c.app, c.pin); got != c.want {
			t.Errorf("HelmCLIVersion(%q,%q)=%q want %q", c.app, c.pin, got, c.want)
		}
	}
}

const sampleReleaseNotes = `## [camunda-platform-15.0.0](https://example/tag) (2026-06-04)

### Features

- Add orchestration.hostNetwork value (#6210)
- [TLS B1] add global.tls.caBundle helper part 1 of helm#3498 (#6039)

### Refactor

- restructure "quoted" identity block (#6300)

### Fixes

- Fail early when Helm CLI < v4 (#6156)

### Documentation

- update README (#6400)

### Release Info

- noise that must be ignored
`

func TestCliffGroups(t *testing.T) {
	toml := `
commit_parsers = [
{ message = "^feat(\\(.+\\))?(!)?:", group = "<!-- 0 -->Features" },
{ message = "^refactor(\\(.+\\))?(!)?:", group = "<!-- 1 -->Refactor" },
{ message = "^fix(\\(.+\\))?(!)?:", group = "<!-- 2 -->Fixes" },
{ message = "^docs(\\(.+\\))?(!)?:", group = "<!-- 3 -->Documentation" },
{ message = "^revert(\\(.+\\))?(!)?:", group = "<!-- 4 -->Revert" },
]
`
	got := CliffGroups(toml)
	want := []string{"Features", "Refactor", "Fixes", "Documentation", "Revert"}
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Errorf("CliffGroups=%v want %v", got, want)
	}
}

func TestArtifactHubChanges(t *testing.T) {
	groups := []string{"Features", "Refactor", "Fixes", "Documentation", "Revert"}
	block, has := ArtifactHubChanges(sampleReleaseNotes, groups)
	if !has {
		t.Fatal("expected change items")
	}
	want := strings.Join([]string{
		"annotations:",
		"  artifacthub.io/changes: |",
		"    - kind: added",
		"      description: \"Add orchestration.hostNetwork value\"",
		"    - kind: added",
		"      description: \"[TLS B1] add global.tls.caBundle helper part 1 of helm#3498\"",
		"    - kind: changed",
		"      description: \"restructure quoted identity block\"",
		"    - kind: fixed",
		"      description: \"Fail early when Helm CLI < v4\"",
		"",
	}, "\n")
	if block != want {
		t.Errorf("block mismatch:\n got:\n%s\nwant:\n%s", block, want)
	}
}

func TestArtifactHubChangesEmpty(t *testing.T) {
	// Only docs section → no kac-mapped entries.
	md := "### Documentation\n\n- update README (#6400)\n"
	block, has := ArtifactHubChanges(md, []string{"Features", "Refactor", "Fixes", "Documentation"})
	if has {
		t.Error("expected no items for docs-only notes")
	}
	if block != "annotations:\n  artifacthub.io/changes: |\n" {
		t.Errorf("empty block should be header only, got:\n%s", block)
	}
}

func TestCleanDescription(t *testing.T) {
	cases := map[string]string{
		"foo (#1234)":              "foo",
		"[TLS] bar helm#3498 (#6)": "[TLS] bar helm#3498",
		`has "quotes" (#9)`:        "has quotes",
		"no parens here":           "no parens here",
		"trailing (a) (b)":         "trailing", // greedy from first " ("
	}
	for in, want := range cases {
		if got := cleanDescription(in); got != want {
			t.Errorf("cleanDescription(%q)=%q want %q", in, got, want)
		}
	}
}
