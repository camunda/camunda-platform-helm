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

package controller

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/suite"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	"operator/api/v1alpha1"
	operatorhelm "operator/internal/helm"
)

// fakeReleaser records every call so a test can assert not just the end state but
// that no Helm operation happened at all.
type fakeReleaser struct {
	live     *operatorhelm.ReleaseInfo
	manifest string
	caps     operatorhelm.Capabilities
	// noTopologyRoles stands in for a chart line older than 8.10, which has no
	// global.topology.mode. Supported is the default so every other test does not
	// have to say so.
	noTopologyRoles bool

	calls        []string
	installCount int
	upgradeCount int
}

func (f *fakeReleaser) ResolveChart(operatorhelm.ChartRef, string) (string, error) {
	f.calls = append(f.calls, "ResolveChart")
	return "/fake/chart", nil
}

func (f *fakeReleaser) Capabilities(string) (operatorhelm.Capabilities, error) {
	f.calls = append(f.calls, "Capabilities")
	caps := f.caps
	caps.TopologyRoles = !f.noTopologyRoles
	return caps, nil
}

func (f *fakeReleaser) Get(string) (*operatorhelm.ReleaseInfo, error) {
	f.calls = append(f.calls, "Get")
	return f.live, nil
}

func (f *fakeReleaser) Template(context.Context, operatorhelm.Options) (string, error) {
	f.calls = append(f.calls, "Template")
	return f.manifest, nil
}

func (f *fakeReleaser) Install(_ context.Context, o operatorhelm.Options) (*operatorhelm.ReleaseInfo, error) {
	f.calls = append(f.calls, "Install")
	f.installCount++
	f.live = &operatorhelm.ReleaseInfo{
		Name: o.ReleaseName, Namespace: o.Namespace, Revision: 1,
		ChartName: "camunda-platform", Status: "deployed", Owner: o.OwnerRef,
	}
	return f.live, nil
}

func (f *fakeReleaser) Upgrade(_ context.Context, o operatorhelm.Options) (*operatorhelm.ReleaseInfo, error) {
	f.calls = append(f.calls, "Upgrade")
	f.upgradeCount++
	rev := 1
	if f.live != nil {
		rev = f.live.Revision + 1
	}
	f.live = &operatorhelm.ReleaseInfo{
		Name: o.ReleaseName, Namespace: o.Namespace, Revision: rev,
		ChartName: "camunda-platform", Status: "deployed", Owner: o.OwnerRef,
	}
	return f.live, nil
}

func (f *fakeReleaser) Uninstall(string, time.Duration) error {
	f.calls = append(f.calls, "Uninstall")
	f.live = nil
	return nil
}

func (f *fakeReleaser) mutatingCalls() int { return f.installCount + f.upgradeCount }

type ReconcileTest struct {
	suite.Suite
	scheme *runtime.Scheme
}

func TestReconcile(t *testing.T) {
	suite.Run(t, new(ReconcileTest))
}

func (s *ReconcileTest) SetupTest() {
	s.scheme = runtime.NewScheme()
	s.Require().NoError(scheme.AddToScheme(s.scheme))
	s.Require().NoError(v1alpha1.AddToScheme(s.scheme))
}

func (s *ReconcileTest) newHub(mutate func(*v1alpha1.CamundaHub)) *v1alpha1.CamundaHub {
	hub := &v1alpha1.CamundaHub{
		ObjectMeta: metav1.ObjectMeta{
			Name:       "camunda",
			Namespace:  "camunda-hub",
			Generation: 1,
			Finalizers: []string{finalizer},
		},
		Spec: v1alpha1.CamundaHubSpec{
			Chart: v1alpha1.ChartSource{
				Repository: "oci://example.test/camunda",
				Name:       "camunda-platform",
				Version:    "15.0.0-alpha4",
			},
			Values: &apiextensionsv1.JSON{Raw: []byte(`{"identity":{"enabled":true}}`)},
			Drift:  v1alpha1.DriftDetect,
		},
	}
	if mutate != nil {
		mutate(hub)
	}
	return hub
}

func (s *ReconcileTest) reconcile(hub *v1alpha1.CamundaHub, rel *fakeReleaser) (*v1alpha1.CamundaHub, ctrl.Result) {
	c := fake.NewClientBuilder().
		WithScheme(s.scheme).
		WithObjects(hub).
		WithStatusSubresource(&v1alpha1.CamundaHub{}).
		Build()

	r := &CamundaHubReconciler{
		Client:   c,
		Scheme:   s.scheme,
		Recorder: record.NewFakeRecorder(50),
		DriverFor: func(genericclioptions.RESTClientGetter, string) (Releaser, error) {
			return rel, nil
		},
	}

	result, err := r.Reconcile(context.Background(), ctrl.Request{
		NamespacedName: types.NamespacedName{Name: hub.Name, Namespace: hub.Namespace},
	})
	s.Require().NoError(err)

	var out v1alpha1.CamundaHub
	s.Require().NoError(c.Get(context.Background(),
		client.ObjectKeyFromObject(hub), &out))
	return &out, result
}

func (s *ReconcileTest) TestInstallsWhenReleaseAbsent() {
	rel := &fakeReleaser{manifest: "kind: Deployment\n"}

	out, _ := s.reconcile(s.newHub(nil), rel)

	s.Equal(1, rel.installCount)
	s.Equal(0, rel.upgradeCount)
	s.Equal("camunda", out.Status.HelmRelease.Name)
	s.Equal(1, out.Status.HelmRelease.Revision)
	s.True(conditionTrue(out, v1alpha1.ConditionReady))
	s.NotEmpty(out.Status.LastAppliedRevision)
}

// TestAdoptionPerformsNoHelmWrite is the migration guarantee for customers who
// already installed the chart with the Helm CLI: taking the release over must not
// reinstall it, must not upgrade it, and therefore must not restart any pod.
func (s *ReconcileTest) TestAdoptionPerformsNoHelmWrite() {
	rel := &fakeReleaser{
		manifest: "kind: Deployment\n",
		live: &operatorhelm.ReleaseInfo{
			Name: "camunda", Namespace: "camunda-hub", Revision: 7,
			ChartName: "camunda-platform", ChartVersion: "15.0.0-alpha4",
			Status: "deployed", Owner: "",
		},
	}

	hub := s.newHub(func(h *v1alpha1.CamundaHub) {
		h.Spec.Adoption.AdoptExisting = true
	})
	out, _ := s.reconcile(hub, rel)

	s.Zero(rel.mutatingCalls(), "adoption must not install or upgrade")
	s.NotContains(rel.calls, "Install")
	s.NotContains(rel.calls, "Upgrade")
	s.Equal(7, out.Status.HelmRelease.Revision, "the existing revision is recorded, not incremented")
	s.True(conditionTrue(out, v1alpha1.ConditionReady))
}

// TestUnownedReleaseIsBlockedWithoutConsent guards the other side of adoption:
// the operator must never silently seize a release someone else installed.
func (s *ReconcileTest) TestUnownedReleaseIsBlockedWithoutConsent() {
	rel := &fakeReleaser{
		manifest: "kind: Deployment\n",
		live: &operatorhelm.ReleaseInfo{
			Name: "camunda", Namespace: "camunda-hub", Revision: 7,
			ChartName: "camunda-platform", Status: "deployed", Owner: "",
		},
	}

	out, result := s.reconcile(s.newHub(nil), rel)

	s.Zero(rel.mutatingCalls())
	s.False(conditionTrue(out, v1alpha1.ConditionReady))
	s.Equal(ReasonAdoptionRequired, conditionReason(out, v1alpha1.ConditionReady))
	s.Positive(result.RequeueAfter, "a blocked reconcile must retry rather than stop")
}

func (s *ReconcileTest) TestPausedPerformsNoWrite() {
	rel := &fakeReleaser{manifest: "kind: Deployment\n"}

	hub := s.newHub(func(h *v1alpha1.CamundaHub) { h.Spec.Paused = true })
	_, _ = s.reconcile(hub, rel)

	s.Zero(rel.mutatingCalls(), "spec.paused must stop all Helm writes")
}

func (s *ReconcileTest) TestConflictingTopologyModeIsRejected() {
	rel := &fakeReleaser{manifest: "kind: Deployment\n"}

	hub := s.newHub(func(h *v1alpha1.CamundaHub) {
		h.Spec.Values = &apiextensionsv1.JSON{
			Raw: []byte(`{"global":{"topology":{"mode":"orchestration"}}}`),
		}
	})
	out, _ := s.reconcile(hub, rel)

	s.Zero(rel.mutatingCalls())
	s.False(conditionTrue(out, v1alpha1.ConditionValuesValid))
	s.Contains(conditionMessage(out, v1alpha1.ConditionValuesValid), "orchestration")
}

func (s *ReconcileTest) TestRetainDeletionLeavesReleaseRunning() {
	rel := &fakeReleaser{
		manifest: "kind: Deployment\n",
		live: &operatorhelm.ReleaseInfo{
			Name: "camunda", Namespace: "camunda-hub", Revision: 3,
			ChartName: "camunda-platform", Status: "deployed", Owner: "camunda-hub/camunda",
		},
	}

	hub := s.newHub(func(h *v1alpha1.CamundaHub) {
		now := metav1.Now()
		h.DeletionTimestamp = &now
		h.Spec.DeletionPolicy = v1alpha1.DeletionRetain
	})

	c := fake.NewClientBuilder().
		WithScheme(s.scheme).
		WithObjects(hub).
		WithStatusSubresource(&v1alpha1.CamundaHub{}).
		Build()

	r := &CamundaHubReconciler{
		Client: c, Scheme: s.scheme, Recorder: record.NewFakeRecorder(50),
		DriverFor: func(genericclioptions.RESTClientGetter, string) (Releaser, error) { return rel, nil },
	}

	_, err := r.Reconcile(context.Background(), ctrl.Request{
		NamespacedName: types.NamespacedName{Name: hub.Name, Namespace: hub.Namespace},
	})
	s.Require().NoError(err)

	s.NotContains(rel.calls, "Uninstall",
		"deletionPolicy Retain must leave the Helm release in place")
	s.NotNil(rel.live)
}

func conditionTrue(hub *v1alpha1.CamundaHub, condType string) bool {
	for _, c := range hub.Status.Conditions {
		if c.Type == condType {
			return c.Status == metav1.ConditionTrue
		}
	}
	return false
}

func conditionReason(hub *v1alpha1.CamundaHub, condType string) string {
	for _, c := range hub.Status.Conditions {
		if c.Type == condType {
			return c.Reason
		}
	}
	return ""
}

func conditionMessage(hub *v1alpha1.CamundaHub, condType string) string {
	for _, c := range hub.Status.Conditions {
		if c.Type == condType {
			return c.Message
		}
	}
	return ""
}

// TestPhasedUpgradeStartsByQuiescing covers the phase path without a cluster: a
// pending change plus spec.upgrade.phased must quiesce first, not upgrade
// straight to the target.
func (s *ReconcileTest) TestPhasedUpgradeStartsByQuiescing() {
	rel := &fakeReleaser{
		manifest: "kind: Deployment\n",
		caps:     operatorhelm.Capabilities{UpgradePhases: true},
		live: &operatorhelm.ReleaseInfo{
			Name: "camunda", Namespace: "camunda-hub", Revision: 1,
			ChartName: "camunda-platform", Status: "deployed", Owner: "uid-under-test",
		},
	}

	hub := s.newHub(func(h *v1alpha1.CamundaHub) {
		h.UID = "uid-under-test"
		h.Spec.Upgrade.Phased = true
		h.Status.HelmRelease = v1alpha1.HelmReleaseStatus{Name: "camunda", Namespace: "camunda-hub"}
		h.Status.LastAppliedRevision = "15.0.0-alpha4@stale"
	})
	out, _ := s.reconcile(hub, rel)

	s.Equal(v1alpha1.PhaseQuiesce, out.Status.Phase)
	s.Equal(1, rel.upgradeCount, "quiescing is applied as a Helm upgrade")
	s.False(conditionTrue(out, v1alpha1.ConditionReady),
		"a release mid-migration is not Ready")
}

// TestPhasedUpgradeWaitsForBackupConfirmation is the gate that makes the
// irreversible migration safe.
func (s *ReconcileTest) TestPhasedUpgradeWaitsForBackupConfirmation() {
	rel := &fakeReleaser{
		manifest: "kind: Deployment\n",
		caps:     operatorhelm.Capabilities{UpgradePhases: true},
		live: &operatorhelm.ReleaseInfo{
			Name: "camunda", Namespace: "camunda-hub", Revision: 2,
			ChartName: "camunda-platform", Status: "deployed", Owner: "uid-under-test",
		},
	}

	// Quiesced with no pods left, which is convergence for that phase.
	hub := s.newHub(func(h *v1alpha1.CamundaHub) {
		h.UID = "uid-under-test"
		h.Spec.Upgrade.Phased = true
		h.Status.Phase = v1alpha1.PhaseQuiesce
		h.Status.HelmRelease = v1alpha1.HelmReleaseStatus{Name: "camunda", Namespace: "camunda-hub"}
		h.Status.LastAppliedRevision = "15.0.0-alpha4@stale"
	})
	out, _ := s.reconcile(hub, rel)

	s.Zero(rel.upgradeCount, "the migration must not start without a confirmed backup")
	s.Equal(v1alpha1.PhaseQuiesce, out.Status.Phase)
	s.Equal("AwaitingBackupConfirmation", conditionReason(out, v1alpha1.ConditionReady))
}

// TestPhasedUpgradeIgnoredWhenChartLacksSupport keeps the operator usable against
// chart versions that predate the phase contract.
func (s *ReconcileTest) TestPhasedUpgradeIgnoredWhenChartLacksSupport() {
	rel := &fakeReleaser{
		manifest: "kind: Deployment\n",
		caps:     operatorhelm.Capabilities{UpgradePhases: false},
		live: &operatorhelm.ReleaseInfo{
			Name: "camunda", Namespace: "camunda-hub", Revision: 1,
			ChartName: "camunda-platform", Status: "deployed", Owner: "uid-under-test",
		},
	}

	hub := s.newHub(func(h *v1alpha1.CamundaHub) {
		h.UID = "uid-under-test"
		h.Spec.Upgrade.Phased = true
		h.Status.HelmRelease = v1alpha1.HelmReleaseStatus{Name: "camunda", Namespace: "camunda-hub"}
		h.Status.LastAppliedRevision = "15.0.0-alpha4@stale"
	})
	out, _ := s.reconcile(hub, rel)

	s.Empty(out.Status.Phase, "no sequence is started")
	s.Equal(1, rel.upgradeCount, "the upgrade still happens, in one step")
	s.True(conditionTrue(out, v1alpha1.ConditionReady))
	s.Equal("ChartDoesNotSupportPhases", conditionReason(out, v1alpha1.ConditionMigrating))
}

// TestConcurrentReconcilesInstallOnce exercises the per-release lock directly.
//
// controller-runtime serialises reconciles of one object, but nothing serialises
// two objects that name the same release, and Helm has no distributed lock:
// concurrent operations fail with "another operation is in progress" and can
// leave a release pending. Driving Reconcile from several goroutines at once is
// the closest reproduction of that race.
func (s *ReconcileTest) TestConcurrentReconcilesInstallOnce() {
	rel := &lockingReleaser{fakeReleaser: fakeReleaser{manifest: "kind: Deployment\n"}}
	hub := s.newHub(nil)

	c := fake.NewClientBuilder().
		WithScheme(s.scheme).
		WithObjects(hub).
		WithStatusSubresource(&v1alpha1.CamundaHub{}).
		Build()

	r := &CamundaHubReconciler{
		Client:   c,
		Scheme:   s.scheme,
		Recorder: record.NewFakeRecorder(200),
		DriverFor: func(genericclioptions.RESTClientGetter, string) (Releaser, error) {
			return rel, nil
		},
	}

	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, _ = r.Reconcile(context.Background(), ctrl.Request{
				NamespacedName: types.NamespacedName{Name: hub.Name, Namespace: hub.Namespace},
			})
		}()
	}
	wg.Wait()

	s.Equal(1, rel.installCount, "concurrent reconciles must produce exactly one install")
	s.Zero(rel.maxConcurrent-1, "no two Helm operations may overlap on one release")
}

// TestSecondHubTargetingSameReleaseIsBlocked covers the other half: two distinct
// CamundaHub objects naming one release. The older object keeps it, so the
// outcome does not depend on which reconciles first.
func (s *ReconcileTest) TestSecondHubTargetingSameReleaseIsBlocked() {
	rel := &fakeReleaser{manifest: "kind: Deployment\n"}

	older := s.newHub(func(h *v1alpha1.CamundaHub) {
		h.Name = "older"
		h.UID = "uid-older"
		h.CreationTimestamp = metav1.NewTime(time.Now().Add(-time.Hour))
		h.Spec.Release.Name = "camunda"
	})
	newer := s.newHub(func(h *v1alpha1.CamundaHub) {
		h.Name = "newer"
		h.UID = "uid-newer"
		h.CreationTimestamp = metav1.NewTime(time.Now())
		h.Spec.Release.Name = "camunda"
	})

	c := fake.NewClientBuilder().
		WithScheme(s.scheme).
		WithObjects(older, newer).
		WithStatusSubresource(&v1alpha1.CamundaHub{}).
		WithIndex(&v1alpha1.CamundaHub{}, releaseTargetIndex, func(obj client.Object) []string {
			return []string{releaseTargetKey(obj.(*v1alpha1.CamundaHub))}
		}).
		Build()

	r := &CamundaHubReconciler{
		Client:   c,
		Scheme:   s.scheme,
		Recorder: record.NewFakeRecorder(50),
		DriverFor: func(genericclioptions.RESTClientGetter, string) (Releaser, error) {
			return rel, nil
		},
	}

	_, err := r.Reconcile(context.Background(), ctrl.Request{
		NamespacedName: types.NamespacedName{Name: "newer", Namespace: newer.Namespace},
	})
	s.Require().NoError(err)

	var out v1alpha1.CamundaHub
	s.Require().NoError(c.Get(context.Background(),
		types.NamespacedName{Name: "newer", Namespace: newer.Namespace}, &out))

	s.Zero(rel.mutatingCalls(), "the newer object must not touch the release")
	s.Equal(ReasonReleaseConflict, conditionReason(&out, v1alpha1.ConditionReady))
	s.Contains(conditionMessage(&out, v1alpha1.ConditionReady), "older")
}

// lockingReleaser records whether two Helm operations ever overlapped.
type lockingReleaser struct {
	fakeReleaser
	mu            sync.Mutex
	inFlight      int
	maxConcurrent int
}

func (l *lockingReleaser) enter() func() {
	l.mu.Lock()
	l.inFlight++
	if l.inFlight > l.maxConcurrent {
		l.maxConcurrent = l.inFlight
	}
	l.mu.Unlock()

	time.Sleep(5 * time.Millisecond)

	return func() {
		l.mu.Lock()
		l.inFlight--
		l.mu.Unlock()
	}
}

func (l *lockingReleaser) Install(ctx context.Context, o operatorhelm.Options) (*operatorhelm.ReleaseInfo, error) {
	defer l.enter()()
	return l.fakeReleaser.Install(ctx, o)
}

func (l *lockingReleaser) Upgrade(ctx context.Context, o operatorhelm.Options) (*operatorhelm.ReleaseInfo, error) {
	defer l.enter()()
	return l.fakeReleaser.Upgrade(ctx, o)
}

// TestChartWithoutTopologyRolesIsRefused guards against the worst silent failure
// available to this operator.
//
// Chart 14.x (Camunda 8.9) has no global.topology.mode and its schema accepts
// unknown keys under global, so the hub role would be accepted and ignored and the
// chart would render the whole platform, Orchestration Cluster StatefulSet
// included. A CamundaHub that quietly deployed Zeebe is not an acceptable outcome,
// so the operator refuses before writing anything.
func (s *ReconcileTest) TestChartWithoutTopologyRolesIsRefused() {
	rel := &fakeReleaser{manifest: "kind: Deployment\n", noTopologyRoles: true}

	out, result := s.reconcile(s.newHub(nil), rel)

	s.Zero(rel.mutatingCalls(), "nothing may be installed from a chart that cannot select the hub role")
	s.False(conditionTrue(out, v1alpha1.ConditionReady))
	s.Equal(ReasonChartLacksHubRole, conditionReason(out, v1alpha1.ConditionReady))
	s.Contains(conditionMessage(out, v1alpha1.ConditionReady), "Orchestration Cluster",
		"the message must say what would otherwise be deployed")
	s.Positive(result.RequeueAfter)
}
