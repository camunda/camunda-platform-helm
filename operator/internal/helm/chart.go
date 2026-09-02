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

package helm

import (
	"fmt"
	"os"
	"strings"

	"helm.sh/helm/v4/pkg/action"
	"helm.sh/helm/v4/pkg/cli"
	"helm.sh/helm/v4/pkg/registry"
)

// ChartRef identifies the chart to render.
type ChartRef struct {
	// Repository is an OCI reference (oci://host/path) or a local directory.
	Repository string
	// Name is appended to an OCI repository. Ignored for local paths.
	Name string
	// Version is the chart SemVer.
	Version string
	// PlainHTTP contacts the registry over HTTP instead of HTTPS.
	PlainHTTP bool
}

// ResolveChart returns a local path to the chart, downloading it when needed.
//
// Resolution goes through Helm's own LocateChart, so an OCI reference is fetched
// and cached exactly as the CLI would fetch it. A repository that is already a
// local directory is used in place, which is what the parity and kind tests need.
func (d *Driver) ResolveChart(ref ChartRef, cacheDir string) (string, error) {
	if isLocalPath(ref.Repository) {
		if _, err := os.Stat(ref.Repository); err != nil {
			return "", fmt.Errorf("local chart path %q: %w", ref.Repository, err)
		}
		return ref.Repository, nil
	}

	settings := cli.New()
	if cacheDir != "" {
		settings.RepositoryCache = cacheDir
	}

	// NewInstall copies the Driver's registry client into ChartPathOptions, so
	// resolution follows the same path an install would take.
	inst := action.NewInstall(d.cfg)
	if ref.PlainHTTP {
		// The Driver's shared client speaks HTTPS; a plain-HTTP registry needs its
		// own.
		rc, err := registry.NewClient(registry.ClientOptPlainHTTP())
		if err != nil {
			return "", fmt.Errorf("create plain-HTTP registry client: %w", err)
		}
		inst.SetRegistryClient(rc)
	}

	opts := inst.ChartPathOptions
	opts.Version = ref.Version
	opts.PlainHTTP = ref.PlainHTTP

	located, err := opts.LocateChart(ociRef(ref), settings)
	if err != nil {
		return "", fmt.Errorf("resolve chart %s version %s: %w", ociRef(ref), ref.Version, err)
	}
	return located, nil
}

func ociRef(ref ChartRef) string {
	base := strings.TrimSuffix(ref.Repository, "/")
	if ref.Name == "" {
		return base
	}
	if strings.HasSuffix(base, "/"+ref.Name) {
		return base
	}
	return base + "/" + ref.Name
}

func isLocalPath(repository string) bool {
	if repository == "" {
		return false
	}
	if registry.IsOCI(repository) {
		return false
	}
	return strings.HasPrefix(repository, "/") ||
		strings.HasPrefix(repository, "./") ||
		strings.HasPrefix(repository, "../")
}
