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

type PressureSource interface {
	Query(context.Context, string) (*float64, error)
}

type GaugeSource interface {
	Gauge(context.Context, string) (*float64, error)
	Query(context.Context, string) (*float64, error)
}

type Status struct {
	ObservedAt       time.Time               `json:"observedAt"`
	Mode             string                  `json:"mode"`
	Phase            string                  `json:"phase"`
	Reason           string                  `json:"reason"`
	CurrentBrokers   int                     `json:"currentBrokers"`
	DesiredBrokers   int                     `json:"desiredBrokers"`
	WorkloadReplicas int                     `json:"workloadReplicas"`
	Pressure         float64                 `json:"pressure,omitempty"`
	Confidence       string                  `json:"confidence"`
	LastError        string                  `json:"lastError,omitempty"`
	PartitionAdvisor PartitionRecommendation `json:"partitionAdvisor"`
}

func (s Status) JSON() []byte {
	data, _ := json.Marshal(s)
	return data
}

type Manager struct {
	Policies       PolicySource
	Workload       WorkloadClient
	Cluster        ClusterClient
	Pressure       PressureSource
	AdvisorMetrics GaugeSource
	Planner        *Planner
	Advisor        *PartitionAdvisor
	Logger         *slog.Logger
	OperationWait  time.Duration
	OperationPoll  time.Duration
	mu             sync.RWMutex
	status         Status
}

func (m *Manager) Status() Status {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.status
}

func (m *Manager) setStatus(status Status) {
	m.mu.Lock()
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
	var partitionLoad *float64
	if policy.PartitionAdvisor.Enabled && m.AdvisorMetrics != nil {
		if policy.PartitionAdvisor.LoadMetricType == "counter-rate" {
			partitionLoad, err = m.AdvisorMetrics.Query(ctx, policy.PartitionAdvisor.LoadMetric)
		} else {
			partitionLoad, err = m.AdvisorMetrics.Gauge(ctx, policy.PartitionAdvisor.LoadMetric)
		}
		if err != nil {
			m.Logger.Warn("partition advisor metric query failed", "error", err)
			partitionLoad = nil
		}
	}
	if m.Advisor != nil {
		status.PartitionAdvisor = m.Advisor.Advise(policy.PartitionAdvisor, topology, partitionLoad)
	}
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
	if policy.Mode == "recommend" {
		status.Phase = "recommended"
		status.Reason = "broker autoscaling disabled"
		status.DesiredBrokers = current
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
		return m.join(ctx, workload.Replicas)
	}

	pressure, err := m.Pressure.Query(ctx, policy.PressureQuery)
	if err != nil {
		m.Logger.Warn("pressure query failed", "error", err)
		pressure = nil
	}
	decision, err := m.Planner.Decide(policy, current, pressure, time.Now().UTC())
	if err != nil {
		return m.fail(err)
	}
	status.DesiredBrokers = decision.Desired
	status.Reason = decision.Reason
	status.Pressure = decision.Pressure
	status.Confidence = decision.Confidence
	if decision.Desired == current {
		status.Phase = "stable"
		m.setStatus(status)
		if workload.Target != current {
			return m.Workload.SetTarget(ctx, current)
		}
		return m.Workload.MarkCompleted(ctx, current)
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
	if err := m.waitForTopology(ctx, target, plan.ChangeID); err != nil {
		return m.fail(err)
	}
	if err := m.Workload.Scale(ctx, target); err != nil {
		return err
	}
	return m.Workload.MarkCompleted(ctx, target)
}

func (m *Manager) join(ctx context.Context, target int) error {
	started, err := m.Workload.PodStarted(ctx, target-1)
	if err != nil {
		return m.fail(err)
	}
	if !started {
		return nil
	}
	if _, err := m.Cluster.Scale(ctx, target, true); err != nil {
		return m.fail(fmt.Errorf("dry-run scale up: %w", err))
	}
	_, err = m.Cluster.Scale(ctx, target, false)
	if err != nil {
		return m.fail(fmt.Errorf("apply scale up: %w", err))
	}
	return nil
}

func (m *Manager) waitForTopology(ctx context.Context, target int, changeID int64) error {
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
			if err != nil {
				m.Logger.Warn("topology poll failed", "error", err)
				continue
			}
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
