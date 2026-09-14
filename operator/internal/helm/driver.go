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

// Package helm drives the camunda-platform chart through the Helm v4 Go SDK.
//
// The SDK version is pinned to the same Helm version the repository pins for the
// CLI in .tool-versions. That equality is what makes operator-rendered manifests
// byte-identical to `helm template`, and it is asserted by TestSDKVersionMatchesToolVersions.
package helm

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"time"

	"helm.sh/helm/v4/pkg/action"
	"helm.sh/helm/v4/pkg/chart/v2/loader"
	"helm.sh/helm/v4/pkg/kube"
	"helm.sh/helm/v4/pkg/registry"
	releasev1 "helm.sh/helm/v4/pkg/release/v1"
	"helm.sh/helm/v4/pkg/storage/driver"
	"k8s.io/cli-runtime/pkg/genericclioptions"
)

const (
	// OwnerLabel marks a Helm release as managed by this operator. Its absence on
	// an existing release is what forces the explicit adoption path.
	//
	// Its value is the owning object's UID rather than its name: label values may
	// not contain "/", are capped at 63 characters, and a UID additionally tells a
	// recreated CamundaHub apart from the one it replaced.
	OwnerLabel = "camunda.io/managed-by-operator"

	// OwnerNamespaceLabel and OwnerNameLabel are informational, so a human running
	// `helm get metadata` can see which object owns a release. Nothing decides on
	// them.
	OwnerNamespaceLabel = "camunda.io/owner-namespace"
	OwnerNameLabel      = "camunda.io/owner-name"

	// defaultMaxHistory bounds stored release revisions.
	defaultMaxHistory = 10

	// storageDriver keeps release state in Secrets, matching the Helm CLI default.
	storageDriver = "secret"

	// fieldManager is the server-side-apply manager the operator applies as.
	//
	// Helm otherwise derives it from os.Args[0], so the CLI applies as "helm"
	// while the operator would apply as its own binary name. Server-side apply
	// then treats an upgrade of a CLI-installed release as a cross-manager
	// conflict and refuses it — which would break the adoption path this operator
	// exists to support. Applying as "helm" keeps operator-driven and CLI-driven
	// operations indistinguishable to the API server, which is the same parity
	// property the render tests assert.
	fieldManager = "helm"
)

func init() {
	kube.ManagedFieldsManager = fieldManager
}

// Driver performs Helm operations against one namespace.
type Driver struct {
	cfg    *action.Configuration
	getter genericclioptions.RESTClientGetter
	ns     string
}

// NewDriver builds a Driver bound to namespace ns.
//
// Helm stores release state in the release namespace, so a Driver is per-namespace
// rather than global.
func NewDriver(getter genericclioptions.RESTClientGetter, ns string) (*Driver, error) {
	cfg, err := newConfiguration(getter, ns)
	if err != nil {
		return nil, err
	}
	return &Driver{cfg: cfg, getter: getter, ns: ns}, nil
}

func newConfiguration(getter genericclioptions.RESTClientGetter, ns string) (*action.Configuration, error) {
	cfg := new(action.Configuration)
	if err := cfg.Init(namespacedGetter{RESTClientGetter: getter, namespace: ns}, ns, storageDriver); err != nil {
		return nil, fmt.Errorf("init helm configuration for namespace %q: %w", ns, err)
	}

	rc, err := registry.NewClient()
	if err != nil {
		return nil, fmt.Errorf("create OCI registry client: %w", err)
	}
	cfg.RegistryClient = rc

	return cfg, nil
}

// Get returns the release, or (nil, nil) when no release by that name exists.
func (d *Driver) Get(name string) (*ReleaseInfo, error) {
	rel, err := action.NewGet(d.cfg).Run(name)
	if err != nil {
		if errors.Is(err, driver.ErrReleaseNotFound) {
			return nil, nil
		}
		return nil, fmt.Errorf("get release %q: %w", name, err)
	}
	return toInfo(rel)
}

// Options describes a single render, install or upgrade.
type Options struct {
	ReleaseName     string
	Namespace       string
	ChartPath       string
	Values          map[string]any
	Timeout         time.Duration
	CreateNamespace bool
	// RollbackOnFailure reverts a failed operation. It must be false for the
	// migrate phase of a Hub upgrade, where the database schema has already moved.
	RollbackOnFailure bool
	// OwnerRef is the owning object's UID, written as a release label.
	OwnerRef string
	// OwnerNamespace and OwnerName are recorded alongside it for humans.
	OwnerNamespace string
	OwnerName      string
}

// Template renders the chart without contacting the cluster.
//
// This mirrors `helm template` exactly — client-side dry run with Replace set to
// skip the name-availability check — because the parity test diffs the two.
//
// It runs on a throwaway action.Configuration rather than the Driver's. A dry run
// permanently swaps cfg.Releases for an in-memory store (helm v4 install.go
// "mem := driver.NewMemory()"), so sharing one Configuration would make every
// later install or upgrade write to memory and silently never reach the cluster.
func (d *Driver) Template(ctx context.Context, o Options) (string, error) {
	ch, err := loader.Load(o.ChartPath)
	if err != nil {
		return "", fmt.Errorf("load chart %q: %w", o.ChartPath, err)
	}

	cfg, err := newConfiguration(d.getter, d.ns)
	if err != nil {
		return "", err
	}

	inst := action.NewInstall(cfg)
	inst.ReleaseName = o.ReleaseName
	inst.Namespace = o.Namespace
	inst.Timeout = o.Timeout
	inst.DryRunStrategy = action.DryRunClient
	inst.Replace = true
	inst.WaitStrategy = kube.HookOnlyStrategy

	rel, err := inst.RunWithContext(ctx, ch, o.Values)
	if err != nil {
		return "", fmt.Errorf("render chart: %w", err)
	}
	typed, err := asRelease(rel)
	if err != nil {
		return "", err
	}
	return typed.Manifest, nil
}

// Install creates the release.
func (d *Driver) Install(ctx context.Context, o Options) (*ReleaseInfo, error) {
	ch, err := loader.Load(o.ChartPath)
	if err != nil {
		return nil, fmt.Errorf("load chart %q: %w", o.ChartPath, err)
	}
	rel, err := d.newInstall(o).RunWithContext(ctx, ch, o.Values)
	if err != nil {
		return nil, fmt.Errorf("install release %q: %w", o.ReleaseName, err)
	}
	return toInfo(rel)
}

// Upgrade upgrades an existing release in place.
func (d *Driver) Upgrade(ctx context.Context, o Options) (*ReleaseInfo, error) {
	ch, err := loader.Load(o.ChartPath)
	if err != nil {
		return nil, fmt.Errorf("load chart %q: %w", o.ChartPath, err)
	}

	up := action.NewUpgrade(d.cfg)
	up.Namespace = o.Namespace
	up.Timeout = o.Timeout
	up.MaxHistory = defaultMaxHistory
	up.RollbackOnFailure = o.RollbackOnFailure
	up.CleanupOnFail = true
	up.WaitStrategy = waitStrategy(o)
	up.Labels = ownerLabels(o)

	rel, err := up.RunWithContext(ctx, o.ReleaseName, ch, o.Values)
	if err != nil {
		return nil, fmt.Errorf("upgrade release %q: %w", o.ReleaseName, err)
	}
	return toInfo(rel)
}

// Uninstall removes the release, keeping history so the action stays reversible.
// Helm does not remove PersistentVolumeClaims, and the operator never removes
// externally provisioned Identity or Keycloak records.
func (d *Driver) Uninstall(name string, timeout time.Duration) error {
	un := action.NewUninstall(d.cfg)
	un.KeepHistory = true
	un.Timeout = timeout
	if _, err := un.Run(name); err != nil {
		if errors.Is(err, driver.ErrReleaseNotFound) {
			return nil
		}
		return fmt.Errorf("uninstall release %q: %w", name, err)
	}
	return nil
}

func (d *Driver) newInstall(o Options) *action.Install {
	inst := action.NewInstall(d.cfg)
	inst.ReleaseName = o.ReleaseName
	inst.Namespace = o.Namespace
	inst.Timeout = o.Timeout
	inst.CreateNamespace = o.CreateNamespace
	inst.RollbackOnFailure = o.RollbackOnFailure
	inst.WaitStrategy = waitStrategy(o)
	inst.Labels = ownerLabels(o)
	return inst
}

// waitStrategy is mandatory in Helm v4: an unset strategy fails the operation
// with "wait strategy not set".
//
// Rolling back on failure only means anything if the operation waits long enough
// to observe one, so RollbackOnFailure implies watching resources. Without it the
// controller returns as soon as the manifests are applied and lets the next
// reconcile observe the result.
func waitStrategy(o Options) kube.WaitStrategy {
	if o.RollbackOnFailure {
		return kube.StatusWatcherStrategy
	}
	return kube.HookOnlyStrategy
}

func ownerLabels(o Options) map[string]string {
	if o.OwnerRef == "" {
		return nil
	}
	labels := map[string]string{OwnerLabel: o.OwnerRef}
	if v := truncateLabel(o.OwnerNamespace); v != "" {
		labels[OwnerNamespaceLabel] = v
	}
	if v := truncateLabel(o.OwnerName); v != "" {
		labels[OwnerNameLabel] = v
	}
	return labels
}

// truncateLabel keeps a value within the 63-character limit Kubernetes enforces
// on label values. Only informational labels are truncated; the ownership label
// carries a UID, which always fits.
func truncateLabel(v string) string {
	const maxLabelValueLength = 63
	if len(v) > maxLabelValueLength {
		return v[:maxLabelValueLength]
	}
	return v
}

// asRelease narrows the SDK's untyped Releaser to the v1 release type.
func asRelease(r any) (*releasev1.Release, error) {
	typed, ok := r.(*releasev1.Release)
	if !ok {
		return nil, fmt.Errorf("unexpected release type %T from helm SDK", r)
	}
	return typed, nil
}

// toInfo narrows and then projects a Releaser into the operator's own view.
func toInfo(r any) (*ReleaseInfo, error) {
	typed, err := asRelease(r)
	if err != nil {
		return nil, err
	}
	return newReleaseInfo(typed), nil
}

// ManifestDigest is the drift-detection identity of a rendered manifest.
func ManifestDigest(manifest string) string {
	sum := sha256.Sum256([]byte(manifest))
	return "sha256:" + hex.EncodeToString(sum[:])
}
