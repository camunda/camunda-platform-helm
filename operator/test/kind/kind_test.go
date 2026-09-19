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

//go:build kind

// Package kind runs the operator against a real Kubernetes cluster.
//
// Everything in the unit suites is proven against a fake Helm driver, which shows
// that the operator decides not to write. Only a real cluster can show that an
// adopted release keeps the same pods. That is the difference these tests exist
// to close.
//
// Build-tagged so `go test ./...` stays cluster-free. Run with:
//
//	make operator.test-kind
package kind

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/suite"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/config"
	sigsyaml "sigs.k8s.io/yaml"

	"operator/api/v1alpha1"
	"operator/internal/controller"
)

const (
	releaseName = "camunda"
	crdPath     = "../../config/crd/bases/camunda.io_camundahubs.yaml"
	chartPath   = "testdata/chart"
)

type KindTest struct {
	suite.Suite

	client     ctrlclient.Client
	reconciler *controller.CamundaHubReconciler
	chartAbs   string
	namespaces []string
}

func TestKind(t *testing.T) {
	suite.Run(t, new(KindTest))
}

func (s *KindTest) SetupSuite() {
	abs, err := filepath.Abs(chartPath)
	s.Require().NoError(err)
	s.chartAbs = abs

	s.applyCRD()

	scheme := runtime.NewScheme()
	s.Require().NoError(clientgoscheme.AddToScheme(scheme))
	s.Require().NoError(v1alpha1.AddToScheme(scheme))

	cfg, err := config.GetConfig()
	s.Require().NoError(err, "no kubeconfig; create a cluster with: kind create cluster")

	c, err := ctrlclient.New(cfg, ctrlclient.Options{Scheme: scheme})
	s.Require().NoError(err)
	s.client = c

	s.reconciler = &controller.CamundaHubReconciler{
		Client:     c,
		Scheme:     scheme,
		Recorder:   record.NewFakeRecorder(100),
		RESTGetter: genericclioptions.NewConfigFlags(false),
	}
}

func (s *KindTest) TearDownSuite() {
	for _, ns := range s.namespaces {
		_ = s.kubectl("delete", "namespace", ns, "--wait=false", "--ignore-not-found")
	}
}

// TestOperatorInstallsRelease is the baseline: a CamundaHub with no pre-existing
// release produces a real Helm release and real workloads.
func (s *KindTest) TestOperatorInstallsRelease() {
	ns := s.namespace("op-install")

	s.applyHub(ns, "initial", true)
	s.reconcileUntilReady(ns)

	s.Equal(1, s.helmRevision(ns), "a fresh install is revision 1")

	var deploy appsv1.Deployment
	s.Require().NoError(s.client.Get(context.Background(),
		types.NamespacedName{Namespace: ns, Name: releaseName + "-hub"}, &deploy))
	s.Equal(int32(1), *deploy.Spec.Replicas)

	// The operator forces the hub topology role; the chart records what it saw.
	var cm corev1.ConfigMap
	s.Require().NoError(s.client.Get(context.Background(),
		types.NamespacedName{Namespace: ns, Name: releaseName + "-hub-topology"}, &cm))
	s.Equal("hub", cm.Data["topologyMode"],
		"the operator must render with global.topology.mode=hub")
}

// TestAdoptionDoesNotRestartPods is the guarantee this whole suite exists for.
//
// A customer installs the chart with the Helm CLI, then hands the release to the
// operator. Nothing may be reinstalled, no Helm revision may be created, and the
// running Deployment must be the same object with the same pods.
func (s *KindTest) TestAdoptionDoesNotRestartPods() {
	ns := s.namespace("op-adopt")

	// 1. The customer's existing Helm-CLI installation.
	s.helm("install", releaseName, s.chartAbs,
		"--namespace", ns, "--create-namespace",
		"--set", "global.topology.mode=hub",
		"--set", "message=initial")

	before := s.deployment(ns)
	beforeRevision := s.helmRevision(ns)
	s.Require().Equal(1, beforeRevision)

	// 2. Hand it to the operator with matching values.
	s.applyHub(ns, "initial", true)
	s.reconcileUntilReady(ns)

	after := s.deployment(ns)

	s.Equal(beforeRevision, s.helmRevision(ns),
		"adoption must not create a Helm revision")
	s.Equal(before.UID, after.UID,
		"the Deployment must be the same object, not recreated")
	s.Equal(before.Generation, after.Generation,
		"an unchanged Deployment generation means the pod template was never touched, so no pods restarted")
	s.Equal(before.Status.ObservedGeneration, after.Status.ObservedGeneration)

	var hub v1alpha1.CamundaHub
	s.Require().NoError(s.client.Get(context.Background(),
		types.NamespacedName{Namespace: ns, Name: releaseName}, &hub))
	s.Equal(1, hub.Status.HelmRelease.Revision,
		"status records the release that was already there")
}

// TestAdoptionThenValuesChangeUpgradesInPlace shows the follow-on behaviour: once
// adopted, a real values change upgrades the existing release rather than
// replacing it.
func (s *KindTest) TestAdoptionThenValuesChangeUpgradesInPlace() {
	ns := s.namespace("op-adopt-upgrade")

	s.helm("install", releaseName, s.chartAbs,
		"--namespace", ns, "--create-namespace",
		"--set", "global.topology.mode=hub",
		"--set", "message=initial")

	before := s.deployment(ns)

	s.applyHub(ns, "initial", true)
	s.reconcileUntilReady(ns)
	s.Require().Equal(1, s.helmRevision(ns))

	// Change a value the chart renders into the pod template.
	s.applyHub(ns, "changed", true)
	s.reconcileUntilReady(ns)

	after := s.deployment(ns)

	s.Equal(2, s.helmRevision(ns), "a values change upgrades the adopted release")
	s.Equal(before.UID, after.UID, "an upgrade must not recreate the Deployment")
	s.Greater(after.Generation, before.Generation,
		"the pod template changed, so this rollout is expected")
	s.Equal("changed",
		after.Spec.Template.Annotations["camunda.io/test-message"])
}

// TestUnownedReleaseIsNotSeized proves the refusal path against a real release:
// without consent the operator leaves someone else's release completely alone.
func (s *KindTest) TestUnownedReleaseIsNotSeized() {
	ns := s.namespace("op-no-seize")

	s.helm("install", releaseName, s.chartAbs,
		"--namespace", ns, "--create-namespace",
		"--set", "global.topology.mode=hub",
		"--set", "message=initial")

	before := s.deployment(ns)

	s.applyHub(ns, "changed", false) // adoptExisting is false
	_, err := s.reconcile(ns)
	s.Require().NoError(err)

	after := s.deployment(ns)

	s.Equal(1, s.helmRevision(ns), "the release must not be touched")
	s.Equal(before.UID, after.UID)
	s.Equal(before.Generation, after.Generation)
	s.Equal("initial", after.Spec.Template.Annotations["camunda.io/test-message"],
		"the operator's desired values must not have been applied")

	var hub v1alpha1.CamundaHub
	s.Require().NoError(s.client.Get(context.Background(),
		types.NamespacedName{Namespace: ns, Name: releaseName}, &hub))
	s.Equal("AdoptionRequired", readyReason(&hub))
}

// TestRetainDeletionLeavesReleaseRunning proves the default deletion policy on a
// real cluster: removing the CamundaHub must not take the workload down.
func (s *KindTest) TestRetainDeletionLeavesReleaseRunning() {
	ns := s.namespace("op-retain")

	s.applyHub(ns, "initial", true)
	s.reconcileUntilReady(ns)
	before := s.deployment(ns)

	var hub v1alpha1.CamundaHub
	s.Require().NoError(s.client.Get(context.Background(),
		types.NamespacedName{Namespace: ns, Name: releaseName}, &hub))
	s.Require().NoError(s.client.Delete(context.Background(), &hub))

	_, err := s.reconcile(ns)
	s.Require().NoError(err)

	after := s.deployment(ns)
	s.Equal(before.UID, after.UID,
		"deletionPolicy Retain must leave the running workload untouched")
	s.Equal(1, s.helmRevision(ns), "the Helm release must survive")
}

// --- helpers ---

// namespace returns a name unique across runs.
//
// TearDownSuite deletes namespaces without waiting, so a run started soon after
// another can still see the previous ones in Terminating. A random suffix keeps
// reruns from colliding with a namespace that is on its way out.
func (s *KindTest) namespace(prefix string) string {
	buf := make([]byte, 4)
	_, err := rand.Read(buf)
	s.Require().NoError(err)

	ns := fmt.Sprintf("%s-%s", prefix, hex.EncodeToString(buf))
	s.namespaces = append(s.namespaces, ns)
	return ns
}

func (s *KindTest) applyCRD() {
	out, err := exec.Command("kubectl", "apply", "-f", crdPath).CombinedOutput()
	s.Require().NoErrorf(err, "applying CRD: %s", string(out))
}

func (s *KindTest) applyHub(ns, message string, adopt bool) {
	s.applyHubWithChart(ns, message, s.chartAbs, adopt)
}

func (s *KindTest) applyHubWithChart(ns, message, chartPath string, adopt bool) {
	ctx := context.Background()

	if err := s.client.Create(ctx, &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{Name: ns},
	}); err != nil && !strings.Contains(err.Error(), "already exists") {
		s.Require().NoError(err)
	}

	vals := map[string]any{"message": message}
	raw, err := json.Marshal(vals)
	s.Require().NoError(err)

	hub := &v1alpha1.CamundaHub{
		ObjectMeta: metav1.ObjectMeta{Name: releaseName, Namespace: ns},
		Spec: v1alpha1.CamundaHubSpec{
			Chart: v1alpha1.ChartSource{
				Repository: chartPath,
				Version:    "0.0.1-test",
			},
			// Short timeout: a wait that stalls should fail the test quickly
			// rather than sit on the operator's default of 15 minutes.
			Release: v1alpha1.ReleaseSpec{
				CreateNamespace: true,
				Timeout:         &metav1.Duration{Duration: 90 * time.Second},
			},
			Values:   &apiextensionsv1.JSON{Raw: raw},
			Adoption: v1alpha1.AdoptionSpec{AdoptExisting: adopt},
			Drift:    v1alpha1.DriftDetect,
		},
	}

	var existing v1alpha1.CamundaHub
	err = s.client.Get(ctx, types.NamespacedName{Namespace: ns, Name: releaseName}, &existing)
	if err == nil {
		existing.Spec = hub.Spec
		s.Require().NoError(s.client.Update(ctx, &existing))
		return
	}
	s.Require().NoError(s.client.Create(ctx, hub))
}

func (s *KindTest) reconcile(ns string) (ctrl.Result, error) {
	return s.reconciler.Reconcile(context.Background(), ctrl.Request{
		NamespacedName: types.NamespacedName{Namespace: ns, Name: releaseName},
	})
}

// reconcileUntilReady drives the loop to a steady state. Adoption deliberately
// requeues immediately so the following pass can converge, so a single call is
// not always enough.
func (s *KindTest) reconcileUntilReady(ns string) {
	for i := 0; i < 5; i++ {
		result, err := s.reconcile(ns)
		s.Require().NoError(err)
		if !result.Requeue {
			s.Require().Truef(s.ready(ns), "not Ready after %d pass(es): %s", i+1, s.conditions(ns))
			return
		}
	}
	s.Failf("no steady state", "reconcile did not settle within 5 passes: %s", s.conditions(ns))
}

func (s *KindTest) ready(ns string) bool {
	var hub v1alpha1.CamundaHub
	s.Require().NoError(s.client.Get(context.Background(),
		types.NamespacedName{Namespace: ns, Name: releaseName}, &hub))
	for _, c := range hub.Status.Conditions {
		if c.Type == v1alpha1.ConditionReady {
			return c.Status == metav1.ConditionTrue
		}
	}
	return false
}

func (s *KindTest) conditions(ns string) string {
	var hub v1alpha1.CamundaHub
	if err := s.client.Get(context.Background(),
		types.NamespacedName{Namespace: ns, Name: releaseName}, &hub); err != nil {
		return err.Error()
	}
	var b strings.Builder
	for _, c := range hub.Status.Conditions {
		fmt.Fprintf(&b, "[%s=%s %s: %s] ", c.Type, c.Status, c.Reason, c.Message)
	}
	return b.String()
}

func (s *KindTest) deployment(ns string) *appsv1.Deployment {
	var deploy appsv1.Deployment
	s.Require().NoError(s.client.Get(context.Background(),
		types.NamespacedName{Namespace: ns, Name: releaseName + "-hub"}, &deploy))
	return &deploy
}

func (s *KindTest) helmRevision(ns string) int {
	out := s.helm("history", releaseName, "--namespace", ns, "-o", "json")

	var history []struct {
		Revision int `json:"revision"`
	}
	s.Require().NoError(json.Unmarshal([]byte(out), &history))
	s.Require().NotEmpty(history)
	return history[len(history)-1].Revision
}

func (s *KindTest) helm(args ...string) string {
	bin := os.Getenv("HELM_BIN")
	if bin == "" {
		bin = "helm"
	}
	out, err := exec.Command(bin, args...).CombinedOutput()
	s.Require().NoErrorf(err, "helm %s: %s", strings.Join(args, " "), string(out))
	return string(out)
}

func (s *KindTest) kubectl(args ...string) error {
	return exec.Command("kubectl", args...).Run()
}

func readyReason(hub *v1alpha1.CamundaHub) string {
	for _, c := range hub.Status.Conditions {
		if c.Type == v1alpha1.ConditionReady {
			return c.Reason
		}
	}
	return ""
}

// TestPhasedUpgradeSequence drives the Camunda Hub 8.9 to 8.10 database
// migration end to end against a real cluster.
//
// The sequence and its guarantees: quiesce stops every writer, the migration
// cannot start until a backup is confirmed, the migration runs on one pod that
// the Service refuses to route to, and only then is service restored.
func (s *KindTest) TestPhasedUpgradeSequence() {
	ns := s.namespace("op-phase")

	// A release already running normally.
	s.applyHub(ns, "initial", false)
	s.reconcileUntilReady(ns)
	s.Require().Equal(1, s.helmRevision(ns))
	s.Require().Equal(int32(1), *s.deployment(ns).Spec.Replicas)

	// Ask for a phased upgrade that also changes a value.
	s.mutateHub(ns, func(hub *v1alpha1.CamundaHub) {
		hub.Spec.Upgrade.Phased = true
		hub.Spec.Values = &apiextensionsv1.JSON{Raw: []byte(`{"message":"migrated"}`)}
	})

	// 1. Quiesce.
	_, err := s.reconcile(ns)
	s.Require().NoError(err)
	s.Equal(v1alpha1.PhaseQuiesce, s.hub(ns).Status.Phase, "the sequence starts by quiescing")
	s.Equal(int32(0), *s.deployment(ns).Spec.Replicas, "quiesce stops the REST API")
	s.Equal(int32(0), *s.websockets(ns).Spec.Replicas, "quiesce stops websockets too")

	// 2. Once quiesced, the migration is gated on a confirmed backup.
	s.driveUntil(ns, "blocked on backup confirmation", func() bool {
		return readyReason(s.hub(ns)) == "AwaitingBackupConfirmation"
	})
	s.Equal(v1alpha1.PhaseQuiesce, s.hub(ns).Status.Phase,
		"without a confirmed backup the sequence must not advance")
	s.Empty(s.hubPods(ns), "nothing may be running that could write to the database")

	// 3. Confirming the backup releases the migration.
	s.mutateHub(ns, func(hub *v1alpha1.CamundaHub) {
		hub.Spec.Upgrade.BackupVerified = true
	})
	_, err = s.reconcile(ns)
	s.Require().NoError(err)

	s.Equal(v1alpha1.PhaseMigrate, s.hub(ns).Status.Phase)
	s.Equal(int32(1), *s.deployment(ns).Spec.Replicas, "the migration runs on exactly one pod")
	s.Equal(int32(0), *s.websockets(ns).Spec.Replicas, "websockets stay down during migration")

	// The traffic-isolation guarantee: the Service still selects only "normal",
	// so the migrating pod is never routed to.
	var svc corev1.Service
	s.Require().NoError(s.client.Get(context.Background(),
		types.NamespacedName{Namespace: ns, Name: releaseName + "-hub"}, &svc))
	s.Equal("normal", svc.Spec.Selector["camunda.io/upgrade-phase"],
		"the Service selector must stay pinned to normal throughout the migration")

	s.driveUntil(ns, "migration pod ready", func() bool {
		pods := s.hubPods(ns)
		return len(pods) > 0 && pods[0].Labels["camunda.io/upgrade-phase"] == "migrate"
	})
	s.assertNoEndpoints(ns, "the migrating pod must receive no traffic")

	// 4. Service is restored.
	s.driveUntil(ns, "back to normal", func() bool {
		return s.hub(ns).Status.Phase == "" && readyCondition(s.hub(ns))
	})

	hub := s.hub(ns)
	s.Empty(hub.Status.Phase, "a finished sequence clears status.phase")
	s.True(readyCondition(hub))
	s.Equal(int32(1), *s.deployment(ns).Spec.Replicas, "replicas are restored")
	s.Equal(int32(1), *s.websockets(ns).Spec.Replicas, "websockets are restored")
	s.Equal("migrated", s.deployment(ns).Spec.Template.Annotations["camunda.io/test-message"],
		"the upgrade the sequence was carrying is applied")

	// install + quiesce + migrate + normal
	s.Equal(4, s.helmRevision(ns), "each phase is its own Helm revision")
}

// TestPhasedUpgradeDegradesOnUnsupportedChart covers the case that matters until
// the chart change ships: asking for a phased upgrade against a chart with no
// phase support must still upgrade, with a warning rather than a failure.
func (s *KindTest) TestPhasedUpgradeDegradesOnUnsupportedChart() {
	ns := s.namespace("op-phase-unsupported")

	chart := s.chartWithoutPhases()

	s.applyHubWithChart(ns, "initial", chart, false)
	s.reconcileUntilReady(ns)

	s.mutateHub(ns, func(hub *v1alpha1.CamundaHub) {
		hub.Spec.Upgrade.Phased = true
		hub.Spec.Values = &apiextensionsv1.JSON{Raw: []byte(`{"message":"changed"}`)}
	})
	s.reconcileUntilReady(ns)

	s.Empty(s.hub(ns).Status.Phase, "no sequence runs against a chart without phase support")
	s.Equal(2, s.helmRevision(ns), "the upgrade still happens, in one step")
	s.Equal("changed", s.deployment(ns).Spec.Template.Annotations["camunda.io/test-message"])
}

// --- phase test helpers ---

func (s *KindTest) hub(ns string) *v1alpha1.CamundaHub {
	var hub v1alpha1.CamundaHub
	s.Require().NoError(s.client.Get(context.Background(),
		types.NamespacedName{Namespace: ns, Name: releaseName}, &hub))
	return &hub
}

func (s *KindTest) mutateHub(ns string, mutate func(*v1alpha1.CamundaHub)) {
	hub := s.hub(ns)
	mutate(hub)
	s.Require().NoError(s.client.Update(context.Background(), hub))
}

func (s *KindTest) websockets(ns string) *appsv1.Deployment {
	var deploy appsv1.Deployment
	s.Require().NoError(s.client.Get(context.Background(),
		types.NamespacedName{Namespace: ns, Name: releaseName + "-hub-websockets"}, &deploy))
	return &deploy
}

// hubPods lists the pods the chart labels with the active phase, which is the
// same signal the operator uses to decide a phase has converged.
func (s *KindTest) hubPods(ns string) []corev1.Pod {
	var pods corev1.PodList
	s.Require().NoError(s.client.List(context.Background(), &pods,
		ctrlclient.InNamespace(ns),
		ctrlclient.HasLabels{"camunda.io/upgrade-phase"}))

	live := make([]corev1.Pod, 0, len(pods.Items))
	for _, pod := range pods.Items {
		if pod.DeletionTimestamp.IsZero() {
			live = append(live, pod)
		}
	}
	return live
}

func (s *KindTest) assertNoEndpoints(ns, msg string) {
	var eps corev1.Endpoints
	err := s.client.Get(context.Background(),
		types.NamespacedName{Namespace: ns, Name: releaseName + "-hub"}, &eps)
	if err != nil {
		return // no Endpoints object at all is also no traffic
	}
	for _, subset := range eps.Subsets {
		s.Empty(subset.Addresses, msg)
	}
}

// driveUntil reconciles repeatedly until cond holds. Phase transitions depend on
// pods actually starting and stopping, so the controller has to be driven while
// the cluster catches up.
func (s *KindTest) driveUntil(ns, what string, cond func() bool) {
	deadline := time.Now().Add(2 * time.Minute)
	for time.Now().Before(deadline) {
		if cond() {
			return
		}
		_, err := s.reconcile(ns)
		s.Require().NoError(err)
		time.Sleep(2 * time.Second)
	}
	s.Failf("timed out", "waiting for %s; conditions: %s", what, s.conditions(ns))
}

func readyCondition(hub *v1alpha1.CamundaHub) bool {
	for _, c := range hub.Status.Conditions {
		if c.Type == v1alpha1.ConditionReady {
			return c.Status == metav1.ConditionTrue
		}
	}
	return false
}

// chartWithoutPhases copies the fixture chart and strips camundaHub.upgrade.phase
// from its values, standing in for a chart line that predates the phase contract.
func (s *KindTest) chartWithoutPhases() string {
	dir := s.T().TempDir()
	out := filepath.Join(dir, "chart")
	s.Require().NoError(copyTree(s.chartAbs, out))

	s.Require().NoError(os.WriteFile(filepath.Join(out, "values.yaml"),
		[]byte("replicas: 1\nmessage: initial\nglobal:\n  topology:\n    mode: combined\n"), 0o600))
	// Without the values key the phase templates cannot render, so drop them too.
	s.Require().NoError(os.Remove(filepath.Join(out, "templates", "deployment-websockets.yaml")))
	s.Require().NoError(os.WriteFile(filepath.Join(out, "templates", "deployment.yaml"), []byte(`
apiVersion: apps/v1
kind: Deployment
metadata:
  name: {{ .Release.Name }}-hub
spec:
  replicas: {{ .Values.replicas }}
  selector:
    matchLabels:
      app: {{ .Release.Name }}-hub
  template:
    metadata:
      labels:
        app: {{ .Release.Name }}-hub
      annotations:
        camunda.io/test-message: {{ .Values.message | quote }}
    spec:
      containers:
        - name: pause
          image: registry.k8s.io/pause:3.9
`), 0o600))
	return out
}

func copyTree(src, dst string) error {
	return filepath.WalkDir(src, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		target := filepath.Join(dst, rel)
		if d.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		return os.WriteFile(target, data, 0o600)
	})
}

// TestResolvesChartFromOCIRegistry covers the path every real deployment uses and
// that the other tests, which point at a local directory, never exercise: pulling
// the chart from an OCI registry.
//
// Skipped unless OCI_REGISTRY names a reachable plain-HTTP registry holding the
// fixture chart. make operator.test-kind-oci sets one up.
func (s *KindTest) TestResolvesChartFromOCIRegistry() {
	registryHost := os.Getenv("OCI_REGISTRY")
	if registryHost == "" {
		s.T().Skip("set OCI_REGISTRY to a plain-HTTP registry holding the fixture chart")
	}

	ns := s.namespace("op-oci")
	s.applyHubWithChartSource(ns, v1alpha1.ChartSource{
		Repository: "oci://" + registryHost + "/charts",
		Name:       "camunda-platform",
		Version:    "0.0.1-test",
		PlainHTTP:  true,
	})
	s.reconcileUntilReady(ns)

	s.Equal(1, s.helmRevision(ns), "the chart was pulled from the registry and installed")

	var deploy appsv1.Deployment
	s.Require().NoError(s.client.Get(context.Background(),
		types.NamespacedName{Namespace: ns, Name: releaseName + "-hub"}, &deploy))
}

// TestDriftDetectsDeletedObject proves drift detection against a real cluster:
// deleting a released object is reported, and Correct mode puts it back.
func (s *KindTest) TestDriftDetectsDeletedObject() {
	ns := s.namespace("op-drift")

	s.applyHub(ns, "initial", false)
	s.reconcileUntilReady(ns)

	// Someone deletes part of the release by hand.
	var cm corev1.ConfigMap
	s.Require().NoError(s.client.Get(context.Background(),
		types.NamespacedName{Namespace: ns, Name: releaseName + "-hub-topology"}, &cm))
	s.Require().NoError(s.client.Delete(context.Background(), &cm))

	// Detect mode reports it and changes nothing.
	_, err := s.reconcile(ns)
	s.Require().NoError(err)
	s.Equal("ObjectsMissing", conditionReasonOf(s.hub(ns), v1alpha1.ConditionDrifted))
	s.Equal(1, s.helmRevision(ns), "Detect mode must not modify the release")

	err = s.client.Get(context.Background(),
		types.NamespacedName{Namespace: ns, Name: releaseName + "-hub-topology"}, &cm)
	s.Require().Error(err, "Detect mode must not put the object back")

	// Correct mode repairs it.
	s.mutateHub(ns, func(hub *v1alpha1.CamundaHub) { hub.Spec.Drift = v1alpha1.DriftCorrect })
	s.driveUntil(ns, "drift corrected", func() bool {
		return s.client.Get(context.Background(),
			types.NamespacedName{Namespace: ns, Name: releaseName + "-hub-topology"}, &cm) == nil
	})
	s.Equal(2, s.helmRevision(ns), "correcting drift is an upgrade")
}

func (s *KindTest) applyHubWithChartSource(ns string, chart v1alpha1.ChartSource) {
	ctx := context.Background()
	if err := s.client.Create(ctx, &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{Name: ns},
	}); err != nil && !strings.Contains(err.Error(), "already exists") {
		s.Require().NoError(err)
	}

	raw, err := json.Marshal(map[string]any{"message": "initial"})
	s.Require().NoError(err)

	s.Require().NoError(s.client.Create(ctx, &v1alpha1.CamundaHub{
		ObjectMeta: metav1.ObjectMeta{Name: releaseName, Namespace: ns},
		Spec: v1alpha1.CamundaHubSpec{
			Chart: chart,
			Release: v1alpha1.ReleaseSpec{
				CreateNamespace: true,
				Timeout:         &metav1.Duration{Duration: 90 * time.Second},
			},
			Values: &apiextensionsv1.JSON{Raw: raw},
			Drift:  v1alpha1.DriftDetect,
		},
	}))
}

func conditionReasonOf(hub *v1alpha1.CamundaHub, condType string) string {
	for _, c := range hub.Status.Conditions {
		if c.Type == condType {
			return c.Reason
		}
	}
	return ""
}

// TestInstallsRealCamundaChart installs the actual camunda-platform 8.10 chart in
// the hub topology role.
//
// The other kind tests use a fixture chart so they measure the operator rather
// than the chart. This one closes the gap that leaves: until it existed, no test
// had ever pointed the operator at the chart it is built to manage. It asserts
// the workloads Camunda Hub is made of are created with the names the 8.10 chart
// gives them.
//
// spec.release.atomic is false so Helm does not wait for readiness: the images
// are private alpha builds and the release needs a database and an identity
// provider that a kind cluster does not have. What is under test is that the
// operator can drive the real chart to a deployed release, not that Camunda runs.
func (s *KindTest) TestInstallsRealCamundaChart() {
	chartPath, err := filepath.Abs("../../../charts/camunda-platform-8.10")
	s.Require().NoError(err)
	if _, err := os.Stat(filepath.Join(chartPath, "charts")); err != nil {
		s.T().Skip("chart dependencies missing; run: make helm.dependency-update chartPath=charts/camunda-platform-8.10")
	}

	ns := s.namespace("op-real")
	s.installRealChart(ns, chartPath)

	ctx := context.Background()
	s.Equal(1, s.helmRevision(ns))

	// The three workloads a hub-role release is made of, at the names the 8.10
	// chart gives them. Web Modeler deliberately keeps its 8.9 resource names so
	// the 8.9-to-8.10 transition is a rolling update rather than a recreate.
	for _, name := range []string{
		releaseName + "-identity",
		releaseName + "-web-modeler-restapi",
		releaseName + "-web-modeler-websockets",
	} {
		var deploy appsv1.Deployment
		s.Require().NoErrorf(s.client.Get(ctx,
			types.NamespacedName{Namespace: ns, Name: name}, &deploy),
			"expected Deployment %s", name)
	}

	// Hub mode runs no orchestration workload.
	var sts appsv1.StatefulSetList
	s.Require().NoError(s.client.List(ctx, &sts, ctrlclient.InNamespace(ns)))
	s.Empty(sts.Items, "a hub-role release must not deploy the Orchestration Cluster")
}

// TestCRDValidationRejectsBadSpecs checks the generated CRD schema against a real
// API server.
//
// The unit suites use a fake client, which does not enforce OpenAPI validation, so
// nothing there would notice if an enum or a required field were lost during
// regeneration. These assertions run through actual admission.
func (s *KindTest) TestCRDValidationRejectsBadSpecs() {
	ns := s.namespace("op-crdval")
	ctx := context.Background()
	s.Require().NoError(s.client.Create(ctx, &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{Name: ns},
	}))

	valid := func() *v1alpha1.CamundaHub {
		return &v1alpha1.CamundaHub{
			ObjectMeta: metav1.ObjectMeta{Name: "probe", Namespace: ns},
			Spec: v1alpha1.CamundaHubSpec{
				Chart: v1alpha1.ChartSource{Repository: s.chartAbs, Version: "0.0.1-test"},
			},
		}
	}

	cases := []struct {
		name   string
		mutate func(*v1alpha1.CamundaHub)
	}{
		{"drift outside the enum", func(h *v1alpha1.CamundaHub) { h.Spec.Drift = "Sometimes" }},
		{"deletionPolicy outside the enum", func(h *v1alpha1.CamundaHub) { h.Spec.DeletionPolicy = "Shred" }},
		{"approval outside the enum", func(h *v1alpha1.CamundaHub) { h.Spec.Upgrade.Approval = "Whenever" }},
		{"valuesFrom kind outside the enum", func(h *v1alpha1.CamundaHub) {
			h.Spec.ValuesFrom = []v1alpha1.ValuesSource{{Kind: "Deployment", Name: "x"}}
		}},
		{"missing chart repository", func(h *v1alpha1.CamundaHub) { h.Spec.Chart.Repository = "" }},
		{"missing chart version", func(h *v1alpha1.CamundaHub) { h.Spec.Chart.Version = "" }},
	}

	for _, tc := range cases {
		s.Run(tc.name, func() {
			hub := valid()
			tc.mutate(hub)
			hub.Name = "probe-" + strings.ReplaceAll(strings.ToLower(tc.name), " ", "-")
			s.Error(s.client.Create(ctx, hub), "the API server should have rejected this")
		})
	}
}

// TestCRDAppliesDocumentedDefaults pins the defaults the CRD promises, so a
// regeneration that drops one is caught rather than silently changing behaviour.
func (s *KindTest) TestCRDAppliesDocumentedDefaults() {
	ns := s.namespace("op-crddefault")
	ctx := context.Background()
	s.Require().NoError(s.client.Create(ctx, &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{Name: ns},
	}))

	hub := &v1alpha1.CamundaHub{
		ObjectMeta: metav1.ObjectMeta{Name: "defaults", Namespace: ns},
		Spec: v1alpha1.CamundaHubSpec{
			Chart: v1alpha1.ChartSource{Repository: s.chartAbs, Version: "0.0.1-test"},
		},
	}
	s.Require().NoError(s.client.Create(ctx, hub))

	var stored v1alpha1.CamundaHub
	s.Require().NoError(s.client.Get(ctx,
		types.NamespacedName{Namespace: ns, Name: "defaults"}, &stored))

	s.Equal(v1alpha1.DriftDetect, stored.Spec.Drift, "drift defaults to reporting, not correcting")
	s.Equal(v1alpha1.DeletionRetain, stored.Spec.DeletionPolicy,
		"deleting a CamundaHub must not delete a running release by default")
	s.Equal(v1alpha1.ApprovalManual, stored.Spec.Upgrade.Approval)
	s.Equal("camunda-platform", stored.Spec.Chart.Name)
	s.Require().NotNil(stored.Spec.Release.Timeout, "the 15m default must actually be applied")
	s.Equal(15*time.Minute, stored.Spec.Release.Timeout.Duration)
	s.Require().NotNil(stored.Spec.Release.Atomic)
	s.True(*stored.Spec.Release.Atomic, "releases roll back on failure unless told otherwise")
	s.False(stored.Spec.Adoption.AdoptExisting, "adoption is never implicit")
	s.False(stored.Spec.Upgrade.Phased)
	s.False(stored.Spec.Upgrade.BackupVerified, "the backup gate is closed by default")
}

// TestRealChartWorkloadsStart takes the real-chart test one step further: it waits
// for the workload the operator produced to actually be accepted by a kubelet.
//
// What this proves that API-server acceptance does not: the container image
// reference resolves and pulls, and every ConfigMap and Secret key the pod spec
// references exists. A missing key surfaces as CreateContainerConfigError, which
// is a real class of chart-and-values bug that rendering cannot catch.
//
// Covers the two hub-role images published publicly. camunda/hub, which backs the
// Web Modeler REST API, is a private alpha build and cannot be pulled here, so it
// is excluded rather than asserted and failing.
//
// This does not prove Camunda is healthy: neither workload has a database or an
// identity provider in a kind cluster, so neither becomes Ready. Proving the
// platform works is the SM-8.10 Playwright suite's job, not this one.
//
// Gated because it pulls from Docker Hub, which is rate-limited on shared CI
// runners; the repository's kind workflow authenticates for this reason.
func (s *KindTest) TestRealChartWorkloadsStart() {
	if os.Getenv("RUN_REAL_IMAGES") == "" {
		s.T().Skip("set RUN_REAL_IMAGES=1 to pull real Camunda images from Docker Hub")
	}

	chartPath, err := filepath.Abs("../../../charts/camunda-platform-8.10")
	s.Require().NoError(err)
	if _, err := os.Stat(filepath.Join(chartPath, "charts")); err != nil {
		s.T().Skip("chart dependencies missing; run: make helm.dependency-update chartPath=charts/camunda-platform-8.10")
	}

	ns := s.namespace("op-runtime")
	s.installRealChart(ns, chartPath)

	for _, workload := range []string{
		releaseName + "-identity",
		releaseName + "-web-modeler-websockets",
	} {
		s.Run(workload, func() { s.requireContainerStarts(ns, workload) })
	}
}

// requireContainerStarts waits for a workload's container to get past image pull
// and container configuration.
func (s *KindTest) requireContainerStarts(ns, workload string) {
	deadline := time.Now().Add(5 * time.Minute)
	var lastState string

	for time.Now().Before(deadline) {
		pods := s.podsForApp(ns, workload)
		if len(pods) > 0 {
			started, state := containerProgress(pods[0])
			lastState = state

			s.Require().NotContains(state, "CreateContainerConfigError",
				"%s references a ConfigMap or Secret key that does not exist: %s", workload, state)
			s.Require().NotContains(state, "InvalidImageName",
				"%s has an invalid rendered image reference: %s", workload, state)

			if started {
				return // image pulled, container configured and started
			}
		}
		time.Sleep(5 * time.Second)
	}
	s.Failf("container never started", "%s last observed as: %s", workload, lastState)
}

func (s *KindTest) installRealChart(ns, chartPath string) {
	values, err := os.ReadFile(filepath.Join("testdata", "real-hub-values.yaml"))
	s.Require().NoError(err)

	var vals map[string]any
	s.Require().NoError(sigsyaml.Unmarshal(values, &vals))
	delete(vals["global"].(map[string]any), "topology")
	raw, err := json.Marshal(vals)
	s.Require().NoError(err)

	ctx := context.Background()
	s.Require().NoError(s.client.Create(ctx, &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{Name: ns},
	}))

	atomic := false
	s.Require().NoError(s.client.Create(ctx, &v1alpha1.CamundaHub{
		ObjectMeta: metav1.ObjectMeta{Name: releaseName, Namespace: ns},
		Spec: v1alpha1.CamundaHubSpec{
			Chart: v1alpha1.ChartSource{Repository: chartPath, Version: "15.0.0-alpha4"},
			Release: v1alpha1.ReleaseSpec{
				CreateNamespace: true,
				Timeout:         &metav1.Duration{Duration: 3 * time.Minute},
				Atomic:          &atomic,
			},
			Values: &apiextensionsv1.JSON{Raw: raw},
			Drift:  v1alpha1.DriftOff,
		},
	}))
	s.reconcileUntilReady(ns)
}

func (s *KindTest) podsForApp(ns, deployName string) []corev1.Pod {
	var pods corev1.PodList
	s.Require().NoError(s.client.List(context.Background(), &pods, ctrlclient.InNamespace(ns)))

	var out []corev1.Pod
	for _, pod := range pods.Items {
		if strings.HasPrefix(pod.Name, deployName) && pod.DeletionTimestamp.IsZero() {
			out = append(out, pod)
		}
	}
	return out
}

// containerProgress reports whether a container has started, and a description of
// its current state for diagnostics.
func containerProgress(pod corev1.Pod) (bool, string) {
	if len(pod.Status.ContainerStatuses) == 0 {
		return false, fmt.Sprintf("phase=%s, no container status yet", pod.Status.Phase)
	}

	cs := pod.Status.ContainerStatuses[0]
	switch {
	case cs.State.Running != nil:
		return true, "Running"
	case cs.State.Terminated != nil:
		// A container that ran and exited still proves the image pulled and the
		// configuration resolved, which is what this test is about.
		return true, fmt.Sprintf("Terminated(%s)", cs.State.Terminated.Reason)
	case cs.State.Waiting != nil:
		if cs.RestartCount > 0 {
			return true, fmt.Sprintf("Waiting(%s) after %d restart(s)", cs.State.Waiting.Reason, cs.RestartCount)
		}
		return false, fmt.Sprintf("Waiting(%s): %s", cs.State.Waiting.Reason, cs.State.Waiting.Message)
	default:
		return false, "unknown"
	}
}

// TestRefusesRealCamunda89Chart points the operator at the actual 8.9 chart.
//
// Chart 14.x has no global.topology.mode, and its values schema accepts unknown
// keys under global, so the hub role is silently ignored rather than rejected:
// rendering that chart with it produces connectors, identity and a Zeebe
// StatefulSet. This asserts the operator refuses before writing, so a CamundaHub
// can never quietly deploy an Orchestration Cluster.
func (s *KindTest) TestRefusesRealCamunda89Chart() {
	chartPath, err := filepath.Abs("../../../charts/camunda-platform-8.9")
	s.Require().NoError(err)
	if _, err := os.Stat(filepath.Join(chartPath, "charts")); err != nil {
		s.T().Skip("8.9 chart dependencies missing; run: make helm.dependency-update chartPath=charts/camunda-platform-8.9")
	}

	ns := s.namespace("op-89")
	ctx := context.Background()
	s.Require().NoError(s.client.Create(ctx, &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{Name: ns},
	}))

	raw, err := json.Marshal(map[string]any{
		"identity": map[string]any{"enabled": true},
		"orchestration": map[string]any{
			"data": map[string]any{"secondaryStorage": map[string]any{"type": "elasticsearch"}},
		},
	})
	s.Require().NoError(err)

	s.Require().NoError(s.client.Create(ctx, &v1alpha1.CamundaHub{
		ObjectMeta: metav1.ObjectMeta{Name: releaseName, Namespace: ns},
		Spec: v1alpha1.CamundaHubSpec{
			Chart:  v1alpha1.ChartSource{Repository: chartPath, Version: "14.8.5"},
			Values: &apiextensionsv1.JSON{Raw: raw},
			Drift:  v1alpha1.DriftOff,
		},
	}))

	_, err = s.reconcile(ns)
	s.Require().NoError(err)

	hub := s.hub(ns)
	s.Equal("ChartLacksHubRole", readyReason(hub))
	s.False(readyCondition(hub))

	// Nothing may have been created, least of all a StatefulSet.
	var sts appsv1.StatefulSetList
	s.Require().NoError(s.client.List(ctx, &sts, ctrlclient.InNamespace(ns)))
	s.Empty(sts.Items, "refusing must happen before any Orchestration Cluster is deployed")

	var deploys appsv1.DeploymentList
	s.Require().NoError(s.client.List(ctx, &deploys, ctrlclient.InNamespace(ns)))
	s.Empty(deploys.Items, "no workload may be created from an unsupported chart")
}
