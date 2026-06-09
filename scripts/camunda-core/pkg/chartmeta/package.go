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

package chartmeta

import (
	"fmt"
	"strings"
)

// ChartImagesAnnotation is the Chart.yaml annotation holding the chart's full
// declared image set, one ref per line.
const ChartImagesAnnotation = "camunda.io/chart-images"

// ChartImages reads the camunda.io/chart-images annotation (a literal block, one
// ref per line) from a Chart.yaml and returns the refs.
func ChartImages(chartYAMLPath string) ([]string, error) {
	m, err := readValues(chartYAMLPath)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", chartYAMLPath, err)
	}
	anns, _ := m["annotations"].(map[string]any)
	raw, _ := scalarString(anns[ChartImagesAnnotation])
	var images []string
	for _, line := range strings.Split(raw, "\n") {
		if s := strings.TrimSpace(line); s != "" {
			images = append(images, s)
		}
	}
	return images, nil
}

// PackageMetadata is the metadata read from a pulled artifact's Chart.yaml.
type PackageMetadata struct {
	Version           string // .version
	AppVersion        string // .appVersion with a trailing ".x" stripped (the chart dir id, e.g. 8.10)
	Prerelease        string // artifacthub.io/prerelease annotation, default "false"
	ReleaseTag        string // camunda-platform-{AppVersion}-{Version}
	CosignBundle      string // camunda-platform-{Version}-cosign-bundle.json
	CosignVerify      string // camunda-platform-{Version}-cosign-verify.sh
	ImageVersions     string // camunda.io/component-image-versions annotation ("" if absent)
	HasImageOverrides bool   // whether the camunda.io/imageOverrides annotation is present/non-empty
	IsLatestStable    *bool  // AppVersion == chart-versions supportStandard[0]; nil when not evaluated
}

// ReadPackageMetadata parses the extracted chartYAMLPath. When chartVersionsPath
// is non-empty it also computes IsLatestStable against
// .camundaVersions.supportStandard[0].
func ReadPackageMetadata(chartYAMLPath, chartVersionsPath string) (PackageMetadata, error) {
	m, err := readValues(chartYAMLPath)
	if err != nil {
		return PackageMetadata{}, fmt.Errorf("read %s: %w", chartYAMLPath, err)
	}
	version, _ := scalarString(m["version"])
	appRaw, _ := scalarString(m["appVersion"])
	app := stripDotX(appRaw)

	anns, _ := m["annotations"].(map[string]any)

	prerelease := "false"
	if raw, ok := anns["artifacthub.io/prerelease"]; ok && raw != nil {
		if s, ok := scalarString(raw); ok {
			prerelease = s
		}
	}
	imageVersions, _ := scalarString(anns["camunda.io/component-image-versions"])
	imageOverrides, _ := scalarString(anns["camunda.io/imageOverrides"])

	meta := PackageMetadata{
		Version:           version,
		AppVersion:        app,
		Prerelease:        prerelease,
		ReleaseTag:        fmt.Sprintf("camunda-platform-%s-%s", app, version),
		CosignBundle:      fmt.Sprintf("camunda-platform-%s-cosign-bundle.json", version),
		CosignVerify:      fmt.Sprintf("camunda-platform-%s-cosign-verify.sh", version),
		ImageVersions:     imageVersions,
		HasImageOverrides: imageOverrides != "",
	}
	if chartVersionsPath != "" {
		latest, err := latestSupportStandard(chartVersionsPath)
		if err != nil {
			return meta, err
		}
		stable := app == latest
		meta.IsLatestStable = &stable
	}
	return meta, nil
}

// stripDotX removes the first "<any-char>x" occurrence, turning an appVersion
// like "8.10.x" into "8.10".
func stripDotX(s string) string {
	for i := 0; i+1 < len(s); i++ {
		if s[i+1] == 'x' {
			return s[:i] + s[i+2:]
		}
	}
	return s
}

// latestSupportStandard returns .camundaVersions.supportStandard[0] from a
// chart-versions.yaml, the "latest stable" Camunda minor.
func latestSupportStandard(path string) (string, error) {
	cv, err := readValues(path)
	if err != nil {
		return "", fmt.Errorf("read %s: %w", path, err)
	}
	camundaVersions, _ := cv["camundaVersions"].(map[string]any)
	arr, _ := camundaVersions["supportStandard"].([]any)
	if len(arr) == 0 {
		return "", fmt.Errorf("camundaVersions.supportStandard is empty in %s", path)
	}
	s, _ := scalarString(arr[0])
	return s, nil
}
