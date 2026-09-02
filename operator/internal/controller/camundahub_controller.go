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
	"fmt"
	"time"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"sync"

	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/yaml"

	"operator/api/v1alpha1"
	operatorhelm "operator/internal/helm"
	"operator/internal/phase"
	"operator/internal/values"
)

const (
	// finalizer keeps the CamundaHub around long enough to decide what happens to
	// the Helm release it owns.
	finalizer = "camunda.io/camundahub"

	// releaseTargetIndex indexes CamundaHub objects by the release they target, so
	// two objects claiming one release are caught before either writes.
	releaseTargetIndex = "spec.releaseTarget"

	// ReasonReleaseConflict reports that another CamundaHub targets this release.
	ReasonReleaseConflict = "ReleaseConflict"

	// ReasonChartLacksHubRole reports a chart with no topology-role support.
	ReasonChartLacksHubRole = "ChartLacksHubRole"

	// requeueAfterSteady is the drift re-check interval for a healthy release.
	requeueAfterSteady = 10 * time.Minute
	// requeueAfterBlocked paces retries of a decision that needs human action.
	requeueAfterBlocked = 2 * time.Minute
)

// CamundaHubReconciler reconciles a CamundaHub into a Helm release.
type CamundaHubReconciler struct {
	client.Client
	Scheme   *runtime.Scheme
	Recorder record.EventRecorder

	// RESTGetter builds the Helm action configuration. Injected so tests can
	// supply one that never reaches a cluster.
	RESTGetter genericclioptions.RESTClientGetter
	// ChartCacheDir holds charts pulled from OCI registries.
	ChartCacheDir string
	// DriverFor builds the Helm driver for a namespace. Overridable in tests.
	DriverFor func(genericclioptions.RESTClientGetter, string) (Releaser, error)

	locks     *releaseLocks
	locksOnce sync.Once
}

// Releaser is the Helm surface the reconciler depends on. The interface exists so
// the controller can be tested without Helm or a cluster, and so the SDK could be
// swapped for the helm binary without touching reconcile logic.
type Releaser interface {
	ResolveChart(ref operatorhelm.ChartRef, cacheDir string) (string, error)
	Capabilities(chartPath string) (operatorhelm.Capabilities, error)
	Get(name string) (*operatorhelm.ReleaseInfo, error)
	Template(ctx context.Context, o operatorhelm.Options) (string, error)
	Install(ctx context.Context, o operatorhelm.Options) (*operatorhelm.ReleaseInfo, error)
	Upgrade(ctx context.Context, o operatorhelm.Options) (*operatorhelm.ReleaseInfo, error)
	Uninstall(name string, timeout time.Duration) error
}

// +kubebuilder:rbac:groups=camunda.io,resources=camundahubs,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=camunda.io,resources=camundahubs/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=camunda.io,resources=camundahubs/finalizers,verbs=update
// +kubebuilder:rbac:groups="",resources=secrets;configmaps;services;serviceaccounts,verbs=get;list;watch;create;update;patch;delete
// Pods are read-only: the phased upgrade observes the chart's
// camunda.io/upgrade-phase pod label to decide a phase has converged.
// +kubebuilder:rbac:groups="",resources=pods,verbs=get;list;watch
// +kubebuilder:rbac:groups="",resources=namespaces,verbs=get;list;watch;create
// +kubebuilder:rbac:groups="",resources=events,verbs=create;patch
// +kubebuilder:rbac:groups=apps,resources=deployments;statefulsets,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=networking.k8s.io,resources=ingresses,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=policy,resources=poddisruptionbudgets,verbs=get;list;watch;create;update;patch;delete

// Reconcile drives one CamundaHub towards its desired Helm release.
func (r *CamundaHubReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	var hub v1alpha1.CamundaHub
	if err := r.Get(ctx, req.NamespacedName, &hub); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	driver, err := r.driver(releaseNamespace(&hub))
	if err != nil {
		return ctrl.Result{}, err
	}

	if !hub.DeletionTimestamp.IsZero() {
		return r.finalize(ctx, &hub, driver)
	}

	if !controllerutilContains(hub.Finalizers, finalizer) {
		hub.Finalizers = append(hub.Finalizers, finalizer)
		if err := r.Update(ctx, &hub); err != nil {
			return ctrl.Result{}, err
		}
	}

	unlock := r.releaseLock(releaseTargetKey(&hub))
	defer unlock()

	conflict, err := r.conflictingHub(ctx, &hub)
	if err != nil {
		return ctrl.Result{}, err
	}
	if conflict != "" {
		message := fmt.Sprintf(
			"CamundaHub %q already targets Helm release %q; two objects must not manage one release",
			conflict, releaseTargetKey(&hub))
		r.event(&hub, corev1.EventTypeWarning, ReasonReleaseConflict, message)
		setReady(&hub, metav1.ConditionFalse, ReasonReleaseConflict, message)
		if statusErr := r.Status().Update(ctx, &hub); statusErr != nil {
			return ctrl.Result{}, statusErr
		}
		return ctrl.Result{RequeueAfter: requeueAfterBlocked}, nil
	}

	result, err := r.reconcileRelease(ctx, &hub, driver)
	if statusErr := r.Status().Update(ctx, &hub); statusErr != nil {
		logger.Error(statusErr, "updating status")
		if err == nil {
			err = statusErr
		}
	}
	return result, err
}

func (r *CamundaHubReconciler) reconcileRelease(
	ctx context.Context, hub *v1alpha1.CamundaHub, driver Releaser,
) (ctrl.Result, error) {
	hub.Status.ObservedGeneration = hub.Generation

	chartPath, err := driver.ResolveChart(chartRef(hub), r.ChartCacheDir)
	if err != nil {
		setCondition(hub, v1alpha1.ConditionChartResolved, metav1.ConditionFalse, "ResolveFailed", err.Error())
		setReady(hub, metav1.ConditionFalse, "ResolveFailed", err.Error())
		return ctrl.Result{RequeueAfter: requeueAfterBlocked}, nil
	}
	setCondition(hub, v1alpha1.ConditionChartResolved, metav1.ConditionTrue, "Resolved", chartPath)

	merged, err := r.composeValues(ctx, hub)
	if err != nil {
		setCondition(hub, v1alpha1.ConditionValuesValid, metav1.ConditionFalse, "InvalidValues", err.Error())
		setReady(hub, metav1.ConditionFalse, "InvalidValues", err.Error())
		return ctrl.Result{RequeueAfter: requeueAfterBlocked}, nil
	}
	setCondition(hub, v1alpha1.ConditionValuesValid, metav1.ConditionTrue, "Valid", "")

	checksum, err := values.Checksum(merged)
	if err != nil {
		return ctrl.Result{}, err
	}
	desiredRevision := fmt.Sprintf("%s@%s", hub.Spec.Chart.Version, checksum)

	opts := operatorhelm.Options{
		ReleaseName:       releaseName(hub),
		Namespace:         releaseNamespace(hub),
		ChartPath:         chartPath,
		Values:            merged,
		Timeout:           timeout(hub),
		CreateNamespace:   hub.Spec.Release.CreateNamespace,
		RollbackOnFailure: rollbackOnFailure(hub),
		OwnerRef:          ownerRef(hub),
		OwnerNamespace:    hub.Namespace,
		OwnerName:         hub.Name,
	}

	live, err := driver.Get(opts.ReleaseName)
	if err != nil {
		return ctrl.Result{}, err
	}

	drift := driftResult{Digest: hub.Status.ManifestDigest}
	if hub.Spec.Drift != v1alpha1.DriftOff {
		if manifest, err := driver.Template(ctx, opts); err == nil {
			drift = r.detectDrift(ctx, manifest, hub.Status.ManifestDigest, opts.Namespace)
		}
	}

	// A phased upgrade owns the release while it runs: each phase is its own Helm
	// revision, so the ordinary install/upgrade decision must not also act.
	caps, err := driver.Capabilities(chartPath)
	if err != nil {
		return ctrl.Result{}, err
	}

	if !caps.TopologyRoles {
		message := fmt.Sprintf(
			"chart %s version %s does not support global.topology.mode, so the hub role cannot be "+
				"selected; that key would be silently ignored and the chart would deploy the whole "+
				"platform, including an Orchestration Cluster. A CamundaHub requires chart 15.x "+
				"(Camunda 8.10) or later",
			hub.Spec.Chart.Name, hub.Spec.Chart.Version)
		r.event(hub, corev1.EventTypeWarning, ReasonChartLacksHubRole, message)
		setCondition(hub, v1alpha1.ConditionReleaseDeployed, metav1.ConditionFalse,
			ReasonChartLacksHubRole, message)
		setReady(hub, metav1.ConditionFalse, ReasonChartLacksHubRole, message)
		return ctrl.Result{RequeueAfter: requeueAfterBlocked}, nil
	}

	pendingRevisionChange := live != nil && hub.Status.LastAppliedRevision != desiredRevision
	step, err := r.phaseStep(ctx, hub, caps, pendingRevisionChange)
	if err != nil {
		return ctrl.Result{}, err
	}

	if step.Act != phase.ActDone && !hub.Spec.Paused {
		return r.applyPhaseStep(ctx, hub, driver, step, opts, desiredRevision, drift.Digest)
	}

	if hub.Status.Phase != "" {
		// The sequence finished; stop reporting a migration in progress.
		hub.Status.Phase = ""
		setCondition(hub, v1alpha1.ConditionMigrating, metav1.ConditionFalse,
			phase.ReasonComplete, step.Message)
	}
	if hub.Spec.Upgrade.Phased && !caps.UpgradePhases {
		setCondition(hub, v1alpha1.ConditionMigrating, metav1.ConditionFalse,
			phase.ReasonChartUnsupported, step.Message)
		r.event(hub, corev1.EventTypeWarning, phase.ReasonChartUnsupported, step.Message)
	}

	decision := Decide(releaseState(live), Desired{
		Revision:      desiredRevision,
		Owner:         ownerRef(hub),
		AdoptExisting: hub.Spec.Adoption.AdoptExisting,
		Paused:        hub.Spec.Paused,
		DriftDetected: drift.Drifted,
		CorrectDrift:  hub.Spec.Drift == v1alpha1.DriftCorrect,
	}, Observed{
		Revision: hub.Status.LastAppliedRevision,
		Adopted:  alreadyRecorded(hub, opts.ReleaseName, opts.Namespace),
	})

	return r.apply(ctx, hub, driver, decision, opts, desiredRevision, drift, live)
}

func (r *CamundaHubReconciler) apply(
	ctx context.Context,
	hub *v1alpha1.CamundaHub,
	driver Releaser,
	decision Decision,
	opts operatorhelm.Options,
	desiredRevision string,
	drift driftResult,
	live *operatorhelm.ReleaseInfo,
) (ctrl.Result, error) {
	switch decision.Action {
	case ActionBlocked:
		r.event(hub, corev1.EventTypeWarning, decision.Reason, decision.Message)
		setCondition(hub, v1alpha1.ConditionReleaseDeployed, metav1.ConditionFalse, decision.Reason, decision.Message)
		setReady(hub, metav1.ConditionFalse, decision.Reason, decision.Message)
		return ctrl.Result{RequeueAfter: requeueAfterBlocked}, nil

	case ActionAdopt:
		// Adoption writes nothing to the cluster. It records what is already there,
		// so a release that already matches never sees a Helm operation and its pods
		// are never restarted.
		r.event(hub, corev1.EventTypeNormal, "Adopted",
			fmt.Sprintf("adopted existing Helm release %q at revision %d without reinstalling",
				opts.ReleaseName, live.Revision))
		recordRelease(hub, live)
		hub.Status.LastAppliedRevision = desiredRevision
		hub.Status.ManifestDigest = drift.Digest
		setCondition(hub, v1alpha1.ConditionReleaseDeployed, metav1.ConditionTrue, "Adopted", "")
		setReady(hub, metav1.ConditionTrue, "Adopted", "")
		return ctrl.Result{Requeue: true}, nil

	case ActionInstall, ActionUpgrade:
		var (
			rel *operatorhelm.ReleaseInfo
			err error
		)
		if decision.Action == ActionInstall {
			rel, err = driver.Install(ctx, opts)
		} else {
			rel, err = driver.Upgrade(ctx, opts)
		}
		if err != nil {
			hub.Status.LastFailure = err.Error()
			r.event(hub, corev1.EventTypeWarning, string(decision.Action)+"Failed", err.Error())
			setCondition(hub, v1alpha1.ConditionReleaseDeployed, metav1.ConditionFalse,
				string(decision.Action)+"Failed", err.Error())
			setReady(hub, metav1.ConditionFalse, string(decision.Action)+"Failed", err.Error())
			return ctrl.Result{RequeueAfter: requeueAfterBlocked}, nil
		}

		hub.Status.LastFailure = ""
		recordRelease(hub, rel)
		hub.Status.LastAppliedRevision = desiredRevision
		hub.Status.ManifestDigest = drift.Digest
		r.event(hub, corev1.EventTypeNormal, string(decision.Action)+"d",
			fmt.Sprintf("release %q at revision %d", rel.Name, rel.Revision))
		setCondition(hub, v1alpha1.ConditionDrifted, metav1.ConditionFalse, "Applied", "")
		setCondition(hub, v1alpha1.ConditionReleaseDeployed, metav1.ConditionTrue, decision.Reason, "")
		setReady(hub, metav1.ConditionTrue, decision.Reason, "")
		return ctrl.Result{RequeueAfter: requeueAfterSteady}, nil

	default:
		if live != nil {
			recordRelease(hub, live)
		}
		if decision.Reason == "DriftDetected" {
			setCondition(hub, v1alpha1.ConditionDrifted, metav1.ConditionTrue, drift.Reason, drift.Message)
			r.event(hub, corev1.EventTypeWarning, drift.Reason, drift.Message)
		} else {
			setCondition(hub, v1alpha1.ConditionDrifted, metav1.ConditionFalse, decision.Reason, "")
		}
		setCondition(hub, v1alpha1.ConditionReleaseDeployed, metav1.ConditionTrue, decision.Reason, decision.Message)
		setReady(hub, metav1.ConditionTrue, decision.Reason, decision.Message)
		return ctrl.Result{RequeueAfter: requeueAfterSteady}, nil
	}
}

// finalize decides what happens to the Helm release when the CamundaHub goes away.
//
// Retain is the default and is deliberate: deleting a CamundaHub must not be able
// to take down a production Hub by accident. Neither policy touches Identity or
// Keycloak records, which ADR-0095 leaves to a human.
func (r *CamundaHubReconciler) finalize(
	ctx context.Context, hub *v1alpha1.CamundaHub, driver Releaser,
) (ctrl.Result, error) {
	if !controllerutilContains(hub.Finalizers, finalizer) {
		return ctrl.Result{}, nil
	}

	if hub.Spec.DeletionPolicy == v1alpha1.DeletionDelete {
		if err := driver.Uninstall(releaseName(hub), timeout(hub)); err != nil {
			return ctrl.Result{}, err
		}
		r.event(hub, corev1.EventTypeNormal, "Uninstalled",
			fmt.Sprintf("uninstalled release %q; history, PersistentVolumeClaims and Secrets were kept",
				releaseName(hub)))
	} else {
		r.event(hub, corev1.EventTypeNormal, "Retained",
			fmt.Sprintf("CamundaHub deleted; Helm release %q was left running per deletionPolicy Retain",
				releaseName(hub)))
	}

	r.event(hub, corev1.EventTypeWarning, v1alpha1.ConditionCleanupRequired,
		"Identity and Keycloak clients provisioned for this Hub are not removed by the operator; "+
			"remove them manually if this deployment is gone for good")

	hub.Finalizers = removeString(hub.Finalizers, finalizer)
	return ctrl.Result{}, r.Update(ctx, hub)
}

func (r *CamundaHubReconciler) composeValues(
	ctx context.Context, hub *v1alpha1.CamundaHub,
) (map[string]any, error) {
	sources := make([]map[string]any, 0, len(hub.Spec.ValuesFrom))
	for _, ref := range hub.Spec.ValuesFrom {
		vals, err := r.readValuesSource(ctx, hub.Namespace, ref)
		if err != nil {
			return nil, err
		}
		if vals != nil {
			sources = append(sources, vals)
		}
	}

	var inline map[string]any
	if hub.Spec.Values != nil {
		decoded, err := values.Decode(hub.Spec.Values.Raw)
		if err != nil {
			return nil, err
		}
		inline = decoded
	}

	return values.Compose(sources, inline)
}

func (r *CamundaHubReconciler) readValuesSource(
	ctx context.Context, namespace string, ref v1alpha1.ValuesSource,
) (map[string]any, error) {
	key := ref.Key
	if key == "" {
		key = "values.yaml"
	}
	name := types.NamespacedName{Namespace: namespace, Name: ref.Name}

	var raw []byte
	switch ref.Kind {
	case "ConfigMap":
		var cm corev1.ConfigMap
		if err := r.Get(ctx, name, &cm); err != nil {
			return nil, skipIfOptional(err, ref)
		}
		raw = []byte(cm.Data[key])
	case "Secret":
		var secret corev1.Secret
		if err := r.Get(ctx, name, &secret); err != nil {
			return nil, skipIfOptional(err, ref)
		}
		raw = secret.Data[key]
	default:
		return nil, fmt.Errorf("valuesFrom kind %q must be ConfigMap or Secret", ref.Kind)
	}

	if len(raw) == 0 {
		if ref.Optional {
			return nil, nil
		}
		return nil, fmt.Errorf("valuesFrom %s/%s has no key %q", ref.Kind, ref.Name, key)
	}

	var out map[string]any
	if err := yaml.Unmarshal(raw, &out); err != nil {
		return nil, fmt.Errorf("parse valuesFrom %s/%s key %q: %w", ref.Kind, ref.Name, key, err)
	}
	return out, nil
}

func skipIfOptional(err error, ref v1alpha1.ValuesSource) error {
	if ref.Optional && apierrors.IsNotFound(err) {
		return nil
	}
	return fmt.Errorf("read valuesFrom %s/%s: %w", ref.Kind, ref.Name, err)
}

func (r *CamundaHubReconciler) driver(namespace string) (Releaser, error) {
	if r.DriverFor != nil {
		return r.DriverFor(r.RESTGetter, namespace)
	}
	return operatorhelm.NewDriver(r.RESTGetter, namespace)
}

func (r *CamundaHubReconciler) event(hub *v1alpha1.CamundaHub, eventType, reason, message string) {
	if r.Recorder != nil {
		r.Recorder.Event(hub, eventType, reason, message)
	}
}

func (r *CamundaHubReconciler) releaseLock(key string) func() {
	r.locksOnce.Do(func() {
		if r.locks == nil {
			r.locks = newReleaseLocks()
		}
	})
	return r.locks.lock(key)
}

// conflictingHub returns the name of another CamundaHub targeting the same
// release, or an empty string when this object is the only claimant.
func (r *CamundaHubReconciler) conflictingHub(
	ctx context.Context, hub *v1alpha1.CamundaHub,
) (string, error) {
	var hubs v1alpha1.CamundaHubList
	if err := r.List(ctx, &hubs,
		client.MatchingFields{releaseTargetIndex: releaseTargetKey(hub)},
	); err != nil {
		// Without the index there is nothing to check; the ownership label still
		// prevents one object from overwriting another's release.
		return "", nil
	}

	for i := range hubs.Items {
		other := &hubs.Items[i]
		if other.UID == hub.UID {
			continue
		}
		if !other.DeletionTimestamp.IsZero() {
			continue
		}
		// Earliest creation wins, so the outcome does not depend on reconcile order.
		if other.CreationTimestamp.Before(&hub.CreationTimestamp) {
			return other.Namespace + "/" + other.Name, nil
		}
	}
	return "", nil
}

// SetupWithManager registers the controller and the release-target index.
func (r *CamundaHubReconciler) SetupWithManager(ctx context.Context, mgr ctrl.Manager) error {
	if err := mgr.GetFieldIndexer().IndexField(ctx, &v1alpha1.CamundaHub{}, releaseTargetIndex,
		func(obj client.Object) []string {
			hub, ok := obj.(*v1alpha1.CamundaHub)
			if !ok {
				return nil
			}
			return []string{releaseTargetKey(hub)}
		}); err != nil {
		return fmt.Errorf("index %s: %w", releaseTargetIndex, err)
	}

	return ctrl.NewControllerManagedBy(mgr).
		For(&v1alpha1.CamundaHub{}).
		Named("camundahub").
		Complete(r)
}
