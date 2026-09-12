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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"operator/api/v1alpha1"
	operatorhelm "operator/internal/helm"
	"operator/internal/phase"
)

// requeueAfterPhase paces the wait between phase transitions. Scaling to zero and
// starting one migration pod are quick, so this is much shorter than the steady
// drift interval.
const requeueAfterPhase = 10 * time.Second

// phaseConverged reports whether the running Hub pods match the phase that was
// last applied.
//
// It reads the chart's own camunda.io/upgrade-phase pod label rather than
// Deployment names, so it does not depend on the chart's resource naming and
// works against any chart that implements the phase contract.
func (r *CamundaHubReconciler) phaseConverged(
	ctx context.Context, namespace string, target v1alpha1.UpgradePhase,
) (bool, error) {
	var pods corev1.PodList
	if err := r.List(ctx, &pods,
		client.InNamespace(namespace),
		client.HasLabels{operatorhelm.UpgradePhaseLabel},
	); err != nil {
		return false, fmt.Errorf("list Camunda Hub pods in %q: %w", namespace, err)
	}

	live := make([]corev1.Pod, 0, len(pods.Items))
	for _, pod := range pods.Items {
		if pod.DeletionTimestamp.IsZero() {
			live = append(live, pod)
		}
	}

	if target == v1alpha1.PhaseQuiesce {
		// Quiesced means nothing is running that could write to the database.
		return len(live) == 0, nil
	}

	// migrate and normal both converge when every Hub pod is in the expected
	// phase and ready: for migrate that is the single migration pod finishing its
	// startup migration, for normal it is service being restored.
	if len(live) == 0 {
		return false, nil
	}
	for _, pod := range live {
		if pod.Labels[operatorhelm.UpgradePhaseLabel] != string(target) {
			return false, nil
		}
		if !podReady(pod) {
			return false, nil
		}
	}
	return true, nil
}

func podReady(pod corev1.Pod) bool {
	for _, c := range pod.Status.Conditions {
		if c.Type == corev1.PodReady {
			return c.Status == corev1.ConditionTrue
		}
	}
	return false
}

// phaseStep computes the current step of a phased upgrade.
func (r *CamundaHubReconciler) phaseStep(
	ctx context.Context,
	hub *v1alpha1.CamundaHub,
	caps operatorhelm.Capabilities,
	pendingRevisionChange bool,
) (phase.Step, error) {
	state := phase.State{
		Requested:             hub.Spec.Upgrade.Phased,
		ChartSupportsPhases:   caps.UpgradePhases,
		PendingRevisionChange: pendingRevisionChange,
		Current:               hub.Status.Phase,
		BackupVerified:        hub.Spec.Upgrade.BackupVerified,
	}

	if state.Current != "" {
		converged, err := r.phaseConverged(ctx, releaseNamespace(hub), state.Current)
		if err != nil {
			return phase.Step{}, err
		}
		state.Converged = converged
	}

	return phase.Next(state), nil
}

// applyPhaseStep performs one transition of the migration sequence.
//
// Each phase is its own Helm revision and status.phase is written before the
// upgrade, so a manager that dies mid-sequence resumes at the right step instead
// of restarting the migration.
func (r *CamundaHubReconciler) applyPhaseStep(
	ctx context.Context,
	hub *v1alpha1.CamundaHub,
	driver Releaser,
	step phase.Step,
	opts operatorhelm.Options,
	desiredRevision string,
	manifestDigest string,
) (ctrl.Result, error) {
	switch step.Act {
	case phase.ActBlocked:
		hub.Status.Phase = step.Target
		setCondition(hub, v1alpha1.ConditionMigrating, metav1.ConditionTrue, step.Reason, step.Message)
		setReady(hub, metav1.ConditionFalse, step.Reason, step.Message)
		r.event(hub, corev1.EventTypeWarning, step.Reason, step.Message)
		return ctrl.Result{RequeueAfter: requeueAfterBlocked}, nil

	case phase.ActWait:
		setCondition(hub, v1alpha1.ConditionMigrating, metav1.ConditionTrue, step.Reason, step.Message)
		setReady(hub, metav1.ConditionFalse, step.Reason, step.Message)
		return ctrl.Result{RequeueAfter: requeueAfterPhase}, nil

	case phase.ActApply:
		phased := opts
		phased.Values = operatorhelm.WithUpgradePhase(opts.Values, string(step.Target))
		phased.RollbackOnFailure = opts.RollbackOnFailure && step.AllowRollback

		rel, err := driver.Upgrade(ctx, phased)
		if err != nil {
			hub.Status.LastFailure = err.Error()
			message := err.Error()
			if !step.AllowRollback {
				message += " (not rolled back: the database migration is not reversible, " +
					"so recovery needs a human)"
			}
			setCondition(hub, v1alpha1.ConditionMigrating, metav1.ConditionFalse, "PhaseFailed", message)
			setReady(hub, metav1.ConditionFalse, "PhaseFailed", message)
			r.event(hub, corev1.EventTypeWarning, "PhaseFailed", message)
			return ctrl.Result{RequeueAfter: requeueAfterBlocked}, nil
		}

		hub.Status.Phase = step.Target
		hub.Status.LastFailure = ""
		recordRelease(hub, rel)

		// Returning to normal is what completes the sequence, so this is where the
		// revision it was carrying is finally recorded. Without it the pending
		// change would still be outstanding and the next pass would quiesce again.
		if step.Target == v1alpha1.PhaseNormal {
			hub.Status.LastAppliedRevision = desiredRevision
			hub.Status.ManifestDigest = manifestDigest
		}
		r.event(hub, corev1.EventTypeNormal, step.Reason,
			fmt.Sprintf("phase %s applied as release revision %d", step.Target, rel.Revision))
		setCondition(hub, v1alpha1.ConditionMigrating, metav1.ConditionTrue, step.Reason, step.Message)
		setReady(hub, metav1.ConditionFalse, step.Reason, step.Message)
		return ctrl.Result{RequeueAfter: requeueAfterPhase}, nil

	default:
		return ctrl.Result{}, nil
	}
}
