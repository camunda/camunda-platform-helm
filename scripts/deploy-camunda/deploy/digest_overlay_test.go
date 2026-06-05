package deploy

import (
	"strings"
	"testing"
)

// digestOverlay is a representative chart-root values-digest.yaml: every
// component is pinned by digest, mirroring the real overlay shape (including the
// nested webModeler.restapi block).
const digestOverlay = `
orchestration:
  image:
    repository: camunda/camunda
    tag: 8.10-SNAPSHOT
    digest: "sha256:orchestrationdigest"
connectors:
  image:
    repository: camunda/connectors-bundle
    tag: 8.10-SNAPSHOT
    digest: "sha256:connectorsdigest"
webModeler:
  restapi:
    image:
      repository: camunda/web-modeler-restapi
      tag: 8.10-SNAPSHOT
      digest: "sha256:restapidigest"
`

// imageHasDigest reports whether <component path>.image in the given doc still
// carries a digest key. path is dot-separated (e.g. "webModeler.restapi").
func imageHasDigest(t *testing.T, doc map[string]any, path string) bool {
	t.Helper()
	node := doc
	for _, seg := range strings.Split(path, ".") {
		child, ok := node[seg].(map[string]any)
		if !ok {
			t.Fatalf("path %q: segment %q not a map", path, seg)
		}
		node = child
	}
	img, ok := node["image"].(map[string]any)
	if !ok {
		t.Fatalf("path %q: no image map", path)
	}
	_, has := img["digest"]
	return has
}

func TestNeutralizeOverriddenDigests(t *testing.T) {
	tests := []struct {
		name         string
		extraValues  string // contents of a single --extra-values file
		wantChanged  bool   // expect a sanitized temp file (path differs from original)
		wantStripped []string
		wantKept     []string
	}{
		{
			name: "tag override without digest strips that component only",
			extraValues: `
orchestration:
  image:
    registry: registry.camunda.cloud
    repository: team-camunda/camunda
    tag: 8.10.0-SNAPSHOT-mq-run-a1
`,
			wantChanged:  true,
			wantStripped: []string{"orchestration"},
			wantKept:     []string{"connectors", "webModeler.restapi"},
		},
		{
			name: "explicit digest in extra-values is preserved (overlay untouched)",
			extraValues: `
orchestration:
  image:
    registry: registry.camunda.cloud
    repository: team-camunda/camunda
    digest: "sha256:callerpinneddigest"
`,
			wantChanged: false,
			wantKept:    []string{"orchestration", "connectors", "webModeler.restapi"},
		},
		{
			name: "repository-only override still strips digest",
			extraValues: `
connectors:
  image:
    repository: team-camunda/connectors-bundle
`,
			wantChanged:  true,
			wantStripped: []string{"connectors"},
			wantKept:     []string{"orchestration", "webModeler.restapi"},
		},
		{
			name: "nested webModeler override strips only that path",
			extraValues: `
webModeler:
  restapi:
    image:
      repository: team-camunda/web-modeler-restapi
      tag: snapshot-a1
`,
			wantChanged:  true,
			wantStripped: []string{"webModeler.restapi"},
			wantKept:     []string{"orchestration", "connectors"},
		},
		{
			name: "extra-values without any image override leaves overlay untouched",
			extraValues: `
global:
  ingress:
    enabled: true
`,
			wantChanged: false,
			wantKept:    []string{"orchestration", "connectors", "webModeler.restapi"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			overlayPath := writeTempYAML(t, dir, "values-digest.yaml", digestOverlay)
			extraPath := writeTempYAML(t, dir, "extra-values.yaml", tt.extraValues)

			got, err := neutralizeOverriddenDigests(overlayPath, []string{extraPath}, dir)
			if err != nil {
				t.Fatalf("neutralizeOverriddenDigests: %v", err)
			}

			if tt.wantChanged {
				if got == overlayPath {
					t.Fatalf("expected a sanitized file, got original path %q", got)
				}
			} else {
				if got != overlayPath {
					t.Fatalf("expected original path unchanged, got %q", got)
				}
			}

			doc := readYAMLMap(t, got)
			for _, p := range tt.wantStripped {
				if imageHasDigest(t, doc, p) {
					t.Errorf("expected digest stripped at %q, but it is still present", p)
				}
			}
			for _, p := range tt.wantKept {
				if !imageHasDigest(t, doc, p) {
					t.Errorf("expected digest kept at %q, but it was removed", p)
				}
			}
		})
	}
}

// TestNeutralizeOverriddenDigestsNoDigestInOverlay verifies that when the caller
// overrides a component whose overlay block has no digest, the overlay is
// returned unchanged (nothing to strip).
func TestNeutralizeOverriddenDigestsNoDigestInOverlay(t *testing.T) {
	dir := t.TempDir()
	overlay := `
orchestration:
  image:
    repository: camunda/camunda
    tag: 8.10-SNAPSHOT
`
	extra := `
orchestration:
  image:
    repository: team-camunda/camunda
    tag: snapshot-a1
`
	overlayPath := writeTempYAML(t, dir, "values-digest.yaml", overlay)
	extraPath := writeTempYAML(t, dir, "extra-values.yaml", extra)

	got, err := neutralizeOverriddenDigests(overlayPath, []string{extraPath}, dir)
	if err != nil {
		t.Fatalf("neutralizeOverriddenDigests: %v", err)
	}
	if got != overlayPath {
		t.Fatalf("expected original path unchanged, got %q", got)
	}
}

// TestNeutralizeOverriddenDigestsMultipleExtraFiles verifies overrides are
// aggregated across multiple --extra-values files.
func TestNeutralizeOverriddenDigestsMultipleExtraFiles(t *testing.T) {
	dir := t.TempDir()
	overlayPath := writeTempYAML(t, dir, "values-digest.yaml", digestOverlay)
	extra1 := writeTempYAML(t, dir, "extra1.yaml", `
orchestration:
  image:
    tag: snapshot-a1
`)
	extra2 := writeTempYAML(t, dir, "extra2.yaml", `
connectors:
  image:
    tag: snapshot-a1
`)

	got, err := neutralizeOverriddenDigests(overlayPath, []string{extra1, extra2}, dir)
	if err != nil {
		t.Fatalf("neutralizeOverriddenDigests: %v", err)
	}
	if got == overlayPath {
		t.Fatal("expected a sanitized file")
	}
	doc := readYAMLMap(t, got)
	if imageHasDigest(t, doc, "orchestration") {
		t.Error("orchestration digest should be stripped")
	}
	if imageHasDigest(t, doc, "connectors") {
		t.Error("connectors digest should be stripped")
	}
	if !imageHasDigest(t, doc, "webModeler.restapi") {
		t.Error("webModeler.restapi digest should be kept")
	}
}
