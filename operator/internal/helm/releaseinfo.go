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
	releasev1 "helm.sh/helm/v4/pkg/release/v1"
)

// ReleaseInfo is the operator's view of a Helm release.
//
// The controller depends on this rather than on Helm's own release type, so the
// Helm SDK stays behind one package boundary and the reconcile logic can be
// tested without it.
type ReleaseInfo struct {
	Name         string
	Namespace    string
	Revision     int
	ChartName    string
	ChartVersion string
	AppVersion   string
	Status       string
	// Owner is the operator ownership label stored on the release, empty when the
	// release was created outside the operator.
	Owner string
}

func newReleaseInfo(rel *releasev1.Release) *ReleaseInfo {
	if rel == nil {
		return nil
	}

	info := &ReleaseInfo{
		Name:      rel.Name,
		Namespace: rel.Namespace,
		Revision:  rel.Version,
		Owner:     rel.Labels[OwnerLabel],
	}
	if rel.Info != nil {
		info.Status = rel.Info.Status.String()
	}
	if rel.Chart != nil && rel.Chart.Metadata != nil {
		info.ChartName = rel.Chart.Metadata.Name
		info.ChartVersion = rel.Chart.Metadata.Version
		info.AppVersion = rel.Chart.Metadata.AppVersion
	}
	return info
}
