// Copyright Camunda Services GmbH and/or licensed to Camunda Services GmbH
// under one or more contributor license agreements. See the NOTICE file
// distributed with this work for additional information regarding copyright
// ownership. Licensed under the Apache License, Version 2.0; you may not use
// this file except in compliance with the License. You may obtain a copy of
// the License at http://www.apache.org/licenses/LICENSE-2.0

package capacity

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strconv"
	"strings"
	"sync"
	"time"
)

type PolicySource interface {
	Policy() (Policy, error)
}

type WorkloadClient interface {
	State(context.Context) (WorkloadState, error)
	SetTarget(context.Context, int) error
	Scale(context.Context, int) error
	MarkCompleted(context.Context, int) error
	PodStarted(context.Context, int) (bool, error)
}

type WorkloadState struct {
	Replicas int
	Target   int
}

type ClusterClient interface {
	Topology(context.Context) (Topology, error)
	Scale(context.Context, int, bool) (ScalePlan, error)
}

// MetricsSource runs a PromQL instant query and returns the maximum value
// across the result series, or nil when the query returned no data.
type MetricsSource interface {
	Query(context.Context, string) (*float64, error)
}

type Status struct {
	ObservedAt       time.Time          `json:"observedAt"`
	Mode             string             `json:"mode"`
	Phase            string             `json:"phase"`
	Reason           string             `json:"reason"`
	CurrentBrokers   int                `json:"currentBrokers"`
	DesiredBrokers   int                `json:"desiredBrokers"`
	WorkloadReplicas int                `json:"workloadReplicas"`
	Signals          map[string]float64 `json:"signals,omitempty"`
	LastChange       *CapacityChange    `json:"lastChange,omitempty"`
	Confidence       string             `json:"confidence"`
	LastError        string             `json:"lastError,omitempty"`
}

// CapacityChange records the most recent decision that started a broker
// change, so the evidence behind it stays visible after later reconciles.
type CapacityChange struct {
	At      time.Time          `json:"at"`
	From    int                `json:"from"`
	To      int                `json:"to"`
	Reason  string             `json:"reason"`
	Signals map[string]float64 `json:"signals,omitempty"`
}

// JSON encodes the status. It never returns an empty body: if encoding
// fails, the error is reported in lastError so /status stays diagnosable.
func (s Status) JSON() []byte {
	data, err := json.Marshal(s)
	if err != nil {
		data, _ = json.Marshal(Status{ObservedAt: s.ObservedAt, Mode: s.Mode, Phase: s.Phase, Reason: s.Reason,
			CurrentBrokers: s.CurrentBrokers, DesiredBrokers: s.DesiredBrokers, WorkloadReplicas: s.WorkloadReplicas,
			Confidence: s.Confidence, LastError: "encode status: " + err.Error()})
	}
	return data
}

type Manager struct {
	Policies PolicySource
	Workload WorkloadClient
	Cluster  ClusterClient
	Metrics  MetricsSource
	// Namespace and StatefulSet replace the $NAMESPACE and $STATEFULSET
	// placeholders in signal queries.
	Namespace   string
	StatefulSet string
	// NamespaceLabel and PodLabel name the Prometheus labels that carry the
	// pod's namespace and name; they build the $SELECTOR placeholder.
	// Empty values default to "namespace" and "pod".
	NamespaceLabel string
	PodLabel       string
	// JoinTimeout bounds how long a provisioned pod may take to become Ready
	// before the manager withdraws it; zero uses OperationWait.
	JoinTimeout   time.Duration
	joinTarget    int
	joinSince     time.Time
	Planner       *Planner
	Logger        *slog.Logger
	OperationWait time.Duration
	OperationPoll time.Duration
	mu            sync.RWMutex
	status        Status
	lastChange    *CapacityChange
}

func (m *Manager) Status() Status {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.status
}

func (m *Manager) setStatus(status Status) {
	m.mu.Lock()
	status.LastChange = m.lastChange
	m.status = status
	m.mu.Unlock()
	m.Logger.Info("capacity decision", "status", string(status.JSON()))
}

func (m *Manager) Reconcile(ctx context.Context) error {
	policy, err := m.Policies.Policy()
	if err != nil {
		return m.fail(err)
	}
	topology, err := m.Cluster.Topology(ctx)
	if err != nil {
		return m.fail(err)
	}
	workload, err := m.Workload.State(ctx)
	if err != nil {
		return m.fail(err)
	}
	current := topology.ActiveBrokerCount()
	status := Status{ObservedAt: time.Now().UTC(), Mode: policy.Mode, Phase: "observing", CurrentBrokers: current, DesiredBrokers: workload.Target, WorkloadReplicas: workload.Replicas, Confidence: "high"}
	if topology.PendingChange != nil {
		status.Phase = "blocked"
		status.Reason = fmt.Sprintf("topology change %d is %s", topology.PendingChange.ID, topology.PendingChange.Status)
		m.setStatus(status)
		return nil
	}
	if !topology.Healthy() {
		status.Phase = "blocked"
		status.Reason = "cluster topology is not healthy"
		m.setStatus(status)
		return nil
	}
	if policy.Mode != "recommend" && workload.Replicas < current {
		status.Phase = "recovering-workload"
		status.Reason = "restoring pods for current topology"
		m.setStatus(status)
		return m.Workload.Scale(ctx, current)
	}
	if policy.Mode != "recommend" && workload.Replicas > current {
		if workload.Target <= current {
			status.Phase = "contracting-workload"
			status.DesiredBrokers = current
			status.Reason = "topology reached durable target"
			m.setStatus(status)
			return m.Workload.Scale(ctx, current)
		}
		status.Phase = "joining-broker"
		status.DesiredBrokers = workload.Replicas
		status.Reason = "continuing provisioned broker join"
		m.setStatus(status)
		return m.join(ctx, workload.Replicas, status)
	}

	readings := m.readSignals(ctx, policy, current)
	decision, err := m.Planner.Decide(policy, Inputs{Current: current, PartitionReplicas: topology.PartitionReplicas(), Readings: readings}, time.Now().UTC())
	if err != nil {
		return m.fail(err)
	}
	status.DesiredBrokers = decision.Desired
	status.Reason = decision.Reason
	status.Signals = decision.Signals
	status.Confidence = decision.Confidence
	if policy.Mode == "recommend" {
		status.Phase = "recommended"
		m.setStatus(status)
		return nil
	}

	if decision.Desired == current {
		status.Phase = "stable"
		m.setStatus(status)
		if workload.Target != current {
			return m.Workload.SetTarget(ctx, current)
		}
		return m.Workload.MarkCompleted(ctx, current)
	}
	if decision.Desired != current {
		m.mu.Lock()
		m.lastChange = &CapacityChange{At: status.ObservedAt, From: current, To: decision.Desired, Reason: decision.Reason, Signals: decision.Signals}
		m.mu.Unlock()
	}
	if decision.Desired > current {
		target := current + 1
		status.Phase = "provisioning-broker"
		status.DesiredBrokers = target
		m.setStatus(status)
		if err := m.Workload.Scale(ctx, target); err != nil {
			return m.fail(err)
		}
		return nil
	}

	target := current - 1
	status.Phase = "evacuating-broker"
	status.DesiredBrokers = target
	if err := m.Workload.SetTarget(ctx, target); err != nil {
		return m.fail(err)
	}
	if _, err := m.Cluster.Scale(ctx, target, true); err != nil {
		return m.fail(fmt.Errorf("dry-run scale down: %w", err))
	}
	plan, err := m.Cluster.Scale(ctx, target, false)
	if err != nil {
		return m.fail(fmt.Errorf("apply scale down: %w", err))
	}
	m.setStatus(status)
	if err := m.waitForTopology(ctx, target, plan.ChangeID, status); err != nil {
		return m.fail(err)
	}
	if err := m.Workload.Scale(ctx, target); err != nil {
		return err
	}
	return m.Workload.MarkCompleted(ctx, target)
}

// placeholders substitutes query placeholders. $SELECTOR expands to a label
// matcher selecting the orchestration broker pods, using the configured label
// names (StatefulSet names are DNS-1123 labels, so they need no regex
// escaping). Longer names are listed first so $NAMESPACE_LABEL is not split.
func (m *Manager) placeholders(current int) *strings.Replacer {
	namespaceLabel, podLabel := m.NamespaceLabel, m.PodLabel
	if namespaceLabel == "" {
		namespaceLabel = "namespace"
	}
	if podLabel == "" {
		podLabel = "pod"
	}
	selector := fmt.Sprintf(`%s=%q,%s=~"%s-[0-9]+"`, namespaceLabel, m.Namespace, podLabel, m.StatefulSet)
	return strings.NewReplacer(
		"$NAMESPACE_LABEL", namespaceLabel,
		"$POD_LABEL", podLabel,
		"$SELECTOR", selector,
		"$BROKERS", strconv.Itoa(current),
		"$NAMESPACE", m.Namespace,
		"$STATEFULSET", m.StatefulSet,
	)
}

// readSignals queries every policy signal. A failed query is recorded as
// missing data so the planner holds instead of acting on partial evidence.
func (m *Manager) readSignals(ctx context.Context, policy Policy, current int) map[string]*float64 {
	readings := map[string]*float64{}
	if policy.Mode != "automatic" || m.Metrics == nil {
		return readings
	}
	placeholders := m.placeholders(current)
	for _, group := range [][]Signal{policy.VetoSignals, policy.ScaleUpSignals, policy.ScaleDownSignals} {
		for _, signal := range group {
			value, err := m.Metrics.Query(ctx, placeholders.Replace(signal.Query))
			if err != nil {
				m.Logger.Warn("signal query failed", "signal", signal.Name, "error", err)
				value = nil
			}
			readings[signal.Name] = value
		}
	}
	return readings
}

func (m *Manager) join(ctx context.Context, target int, status Status) error {
	started, err := m.Workload.PodStarted(ctx, target-1)
	if err != nil {
		return m.fail(err)
	}
	if !started {
		now := time.Now()
		if m.joinTarget != target || m.joinSince.IsZero() {
			m.joinTarget, m.joinSince = target, now
		}
		timeout := m.JoinTimeout
		if timeout <= 0 {
			timeout = m.OperationWait
		}
		if waited := now.Sub(m.joinSince); waited >= timeout {
			// The pod never became Ready, so its broker was never added to the
			// topology. Removing the pod is safe and stops the manager from
			// holding a half-provisioned broker forever.
			m.joinTarget, m.joinSince = 0, time.Time{}
			if err := m.Workload.Scale(ctx, target-1); err != nil {
				return m.fail(fmt.Errorf("withdraw unready broker pod: %w", err))
			}
			if m.Planner != nil {
				m.Planner.lastChange = now
			}
			return m.fail(fmt.Errorf("broker pod %d was not Ready within %s; withdrew it before joining the topology", target-1, timeout))
		}
		status.Phase = "joining-broker"
		status.Reason = fmt.Sprintf("waiting for broker pod %d to become Ready (%s)", target-1, now.Sub(m.joinSince).Round(time.Second))
		m.setStatus(status)
		return nil
	}
	m.joinTarget, m.joinSince = 0, time.Time{}
	if _, err := m.Cluster.Scale(ctx, target, true); err != nil {
		return m.fail(fmt.Errorf("dry-run scale up: %w", err))
	}
	_, err = m.Cluster.Scale(ctx, target, false)
	if err != nil {
		return m.fail(fmt.Errorf("apply scale up: %w", err))
	}
	return nil
}

func (m *Manager) waitForTopology(ctx context.Context, target int, changeID int64, status Status) error {
	started := time.Now()
	deadline := time.NewTimer(m.OperationWait)
	defer deadline.Stop()
	ticker := time.NewTicker(m.OperationPoll)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-deadline.C:
			return fmt.Errorf("timed out waiting for topology change %d", changeID)
		case <-ticker.C:
			topology, err := m.Cluster.Topology(ctx)
			// Keep /status current while the reconcile loop is blocked here.
			status.ObservedAt = time.Now().UTC()
			if err != nil {
				m.Logger.Warn("topology poll failed", "error", err)
				status.Reason = fmt.Sprintf("evacuating broker %d: topology poll failed after %s: %v", target, time.Since(started).Round(time.Second), err)
				m.setStatus(status)
				continue
			}
			progress := "waiting for change to start"
			if topology.PendingChange != nil {
				progress = fmt.Sprintf("change %d %s", topology.PendingChange.ID, topology.PendingChange.Status)
			}
			status.CurrentBrokers = topology.ActiveBrokerCount()
			status.Reason = fmt.Sprintf("evacuating broker %d: %s after %s", target, progress, time.Since(started).Round(time.Second))
			m.setStatus(status)
			if topology.PendingChange == nil && topology.LastChange != nil && topology.LastChange.ID == changeID {
				if topology.LastChange.Status != "COMPLETED" {
					return fmt.Errorf("topology change %d ended with %s", changeID, topology.LastChange.Status)
				}
				if topology.ActiveBrokerCount() == target && topology.Healthy() {
					return nil
				}
			}
		}
	}
}

func (m *Manager) fail(err error) error {
	status := m.Status()
	status.ObservedAt = time.Now().UTC()
	status.Phase = "error"
	status.LastError = err.Error()
	m.setStatus(status)
	return err
}
