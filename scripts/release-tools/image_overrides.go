package main

import (
	"flag"
	"fmt"
	"os"
	"strconv"

	"scripts/camunda-core/pkg/chartmeta"
)

// overrideKeys is the fixed component order for the imageOverrides annotation
// (the block is only consumed as a presence flag, so the order is cosmetic).
var overrideKeys = []string{
	"orchestration", "zeebe", "zeebe-gateway", "operate", "tasklist",
	"console", "modeler", "connectors", "optimize", "identity",
}

// runImageOverrides collects the *-image-tag inputs into the
// `camunda.io/imageOverrides` annotation source and records whether any were provided.
//
//	release-tools image-overrides --orchestration <tag> --zeebe <tag> ... --out /tmp/image-overrides.yaml
//
// Writes HAS_IMAGE_OVERRIDES=<bool> to $GITHUB_ENV and, when any override is
// present, the YAML block to --out.
func runImageOverrides(args []string) error {
	fs := flag.NewFlagSet("image-overrides", flag.ContinueOnError)
	var out string
	vals := make(map[string]*string, len(overrideKeys))
	for _, k := range overrideKeys {
		vals[k] = fs.String(k, "", "image tag override for "+k)
	}
	fs.StringVar(&out, "out", "/tmp/image-overrides.yaml", "file to write the overrides YAML block to when any are present")
	if err := fs.Parse(args); err != nil {
		return err
	}

	overrides := make([]chartmeta.ImageOverride, 0, len(overrideKeys))
	for _, k := range overrideKeys {
		overrides = append(overrides, chartmeta.ImageOverride{Key: k, Value: *vals[k]})
	}
	block, has := chartmeta.ImageOverrides(overrides)

	if err := newGitHubEnv().set("HAS_IMAGE_OVERRIDES", strconv.FormatBool(has)); err != nil {
		return err
	}
	if has {
		if err := os.WriteFile(out, []byte(block), 0o644); err != nil {
			return fmt.Errorf("write %s: %w", out, err)
		}
		fmt.Fprintf(os.Stderr, "Image override inputs detected:\n%s", block)
	}
	return nil
}
