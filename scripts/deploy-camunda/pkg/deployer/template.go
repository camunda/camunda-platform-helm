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

package deployer

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"scripts/camunda-core/pkg/helm"
	"scripts/camunda-core/pkg/logging"
	"scripts/deploy-camunda/pkg/types"
)

// renderTemplates builds and executes `helm template` to render manifests to disk.
func renderTemplates(ctx context.Context, o types.Options) error {
	var chartArg string
	if o.Chart != "" {
		chartArg = o.Chart
	} else {
		chartArg = filepath.Clean(o.ChartPath)
	}

	args := []string{
		"template",
		o.ReleaseName,
		chartArg,
		"-n", o.Namespace,
	}

	// Include CRDs by default unless explicitly disabled
	if o.IncludeCRDs {
		args = append(args, "--include-crds")
	}

	// Values files in order
	for _, v := range o.ValuesFiles {
		args = append(args, "-f", v)
	}

	// Optional post-renderer
	if o.PostRendererPath != "" {
		args = append(args, "--post-renderer", o.PostRendererPath)
	}

	// Determine output dir (default: ./rendered/<release>)
	outputDir := o.RenderOutputDir
	if outputDir == "" {
		outputDir = filepath.Join(".", "rendered", o.ReleaseName)
	}
	if err := os.MkdirAll(outputDir, 0o755); err != nil {
		return fmt.Errorf("failed to create render output dir %q: %w", outputDir, err)
	}
	args = append(args, "--output-dir", outputDir)

	logging.Logger.Info().
		Str("outputDir", outputDir).
		Str("release", o.ReleaseName).
		Str("namespace", o.Namespace).
		Msg("Rendering Helm templates to directory")

	if err := helm.Run(ctx, args, ""); err != nil {
		return fmt.Errorf("helm template failed: %w", err)
	}

	logging.Logger.Info().Str("outputDir", outputDir).Msg("Templates rendered successfully")
	return nil
}

func RenderTopologyContract(ctx context.Context, o types.Options) ([]byte, error) {
	chartArg := o.Chart
	if chartArg == "" {
		chartArg = filepath.Clean(o.ChartPath)
	}
	args := []string{"template", o.ReleaseName, chartArg, "-n", o.Namespace, "--api-versions", "camunda.io/topology-contract", "--show-only", "templates/common/topology-contract.yaml"}
	if o.Chart != "" && o.Version != "" {
		args = append(args, "--version", o.Version)
	}
	args = append(args, composeKubeArgs(o.Kubeconfig, o.KubeContext)...)
	args = appendHelmValueArgs(args, o)
	out, err := helm.RunCapture(ctx, args, "")
	if err != nil {
		return nil, fmt.Errorf("helm topology contract render failed: %w", err)
	}
	return out, nil
}
