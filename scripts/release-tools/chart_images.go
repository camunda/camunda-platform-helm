// Copyright 2026 Camunda Services GmbH
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package main

import (
	"flag"
	"fmt"
	"io"
	"os"
	"strings"

	"gopkg.in/yaml.v3"
	"scripts/camunda-core/pkg/chartmeta"
)

type artifactHubImage struct {
	Name  string `yaml:"name"`
	Image string `yaml:"image"`
}

func artifactHubImagesYAML(images []string) ([]byte, error) {
	entries := make([]artifactHubImage, len(images))
	for i, image := range images {
		entries[i] = artifactHubImage{Name: imageName(image), Image: image}
	}
	return yaml.Marshal(entries)
}

// imageName extracts the repo basename Artifact Hub's security-report UI
// labels the image with, e.g. "docker.io/camunda/keycloak:26.3.3" -> "keycloak".
func imageName(ref string) string {
	if i := strings.LastIndex(ref, "/"); i != -1 {
		ref = ref[i+1:]
	}
	if i := strings.IndexAny(ref, "@:"); i != -1 {
		ref = ref[:i]
	}
	return ref
}

// validateImageRefs rejects any ref carrying an unresolved placeholder (e.g. a
// literal "$E2E_TESTS_CONSOLE_IMAGE_TAG" from a QA-only scenario layer that
// ResolvePaths should have excluded). Both annotations are publication
// artifacts now, so a ref that would never resolve to a real image must fail
// loud rather than get published.
func validateImageRefs(images []string) error {
	for _, ref := range images {
		if strings.ContainsAny(ref, "$ \t{}") {
			return fmt.Errorf("image ref %q looks unresolved; refusing to publish", ref)
		}
	}
	return nil
}

// runChartImages derives the chart's declared image set from its values.yaml
// (plus the chart-full-setup scenario layers) and prints it one fully-qualified
// reference per line, e.g. to record as the camunda.io/chart-images annotation:
//
//	release-tools chart-images --chart-dir "$chart_dir" > /tmp/chart-images.txt
//	yq -i '.annotations."camunda.io/chart-images" = load_str("/tmp/chart-images.txt")' "$chart_dir/Chart.yaml"
//
// With --artifacthub-out, it also writes the same references in the structured
// artifacthub.io/images format. It fails loud on an empty result rather than
// recording an empty set: a valid chart always declares images.
func runChartImages(args []string, stdout io.Writer) error {
	fs := flag.NewFlagSet("chart-images", flag.ContinueOnError)
	var chartDir, artifactHubOut string
	fs.StringVar(&chartDir, "chart-dir", "", "chart directory (the finalized chart, e.g. charts/camunda-platform-<v>)")
	fs.StringVar(&artifactHubOut, "artifacthub-out", "", "file to write artifacthub.io/images YAML")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if chartDir == "" {
		return fmt.Errorf("--chart-dir is required")
	}

	images, err := chartmeta.ImageSet(chartDir)
	if err != nil {
		return fmt.Errorf("derive image set: %w", err)
	}
	if len(images) == 0 {
		return fmt.Errorf("no images declared in %s/values.yaml; refusing to record an empty chart-images set", chartDir)
	}
	if err := validateImageRefs(images); err != nil {
		return err
	}

	// Both outputs are serialized from the same validated `images` slice before
	// either is written, so a failure here never leaves one annotation stale
	// relative to the other.
	var artifactHubImages []byte
	if artifactHubOut != "" {
		artifactHubImages, err = artifactHubImagesYAML(images)
		if err != nil {
			return fmt.Errorf("serialize Artifact Hub images: %w", err)
		}
	}

	if _, err := fmt.Fprintln(stdout, strings.Join(images, "\n")); err != nil {
		return err
	}
	if artifactHubOut != "" {
		if err := os.WriteFile(artifactHubOut, artifactHubImages, 0o644); err != nil {
			return fmt.Errorf("write %s: %w", artifactHubOut, err)
		}
	}
	return nil
}
