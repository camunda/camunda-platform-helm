// Copyright Camunda Services GmbH and/or licensed to Camunda Services GmbH
// under one or more contributor license agreements. See the NOTICE file
// distributed with this work for additional information regarding copyright
// ownership. Licensed under the Apache License, Version 2.0; you may not use
// this file except in compliance with the License. You may obtain a copy of
// the License at http://www.apache.org/licenses/LICENSE-2.0

package capacity

import (
	"context"
	"io"
	"log/slog"
	"testing"
	"time"
)

type staticPolicies struct{ policy Policy }

func (s staticPolicies) Policy() (Policy, error) { return s.policy, nil }

type fakePressure struct{ value *float64 }

func (f fakePressure) Query(context.Context, string) (*float64, error) { return f.value, nil }

type fakeWorkload struct {
	replicas int
	target   int
	started  bool
	scales   []int
}

func (f *fakeWorkload) State(context.Context) (WorkloadState, error) {
	target := f.target
	if target == 0 {
		target = f.replicas
	}
	return WorkloadState{Replicas: f.replicas, Target: target}, nil
}
func (f *fakeWorkload) SetTarget(_ context.Context, target int) error {
	f.target = target
	return nil
}
func (f *fakeWorkload) Scale(_ context.Context, replicas int) error {
	f.replicas = replicas
	f.target = replicas
	f.scales = append(f.scales, replicas)
	return nil
}
func (f *fakeWorkload) PodStarted(context.Context, int) (bool, error) { return f.started, nil }

type scaleCall struct {
	target int
	dryRun bool
}

type fakeCluster struct {
	topology Topology
	calls    []scaleCall
}

func (f *fakeCluster) Topology(context.Context) (Topology, error) { return f.topology, nil }
func (f *fakeCluster) Scale(_ context.Context, target int, dryRun bool) (ScalePlan, error) {
	f.calls = append(f.calls, scaleCall{target: target, dryRun: dryRun})
	return ScalePlan{ChangeID: 1, ExpectedTopology: make([]Broker, target)}, nil
}

func activeTopology(count int) Topology {
	brokers := make([]Broker, count)
	for i := range brokers {
		brokers[i] = Broker{State: "ACTIVE", Partitions: []Partition{{ID: 1, State: "ACTIVE"}}}
	}
	return Topology{Brokers: brokers}
}

func newTestManager(policy Policy, workload *fakeWorkload, cluster *fakeCluster) *Manager {
	return &Manager{
		Policies: staticPolicies{policy: policy}, Workload: workload, Cluster: cluster,
		Pressure: fakePressure{}, Planner: &Planner{}, Logger: slog.New(slog.NewTextHandler(io.Discard, nil)),
		OperationWait: time.Millisecond, OperationPoll: time.Millisecond,
	}
}

func TestManagerScaleUpSequence(t *testing.T) {
	policy := Policy{Mode: "scheduled", MinBrokers: 1, MaxBrokers: 3, TargetBrokers: 2}
	workload := &fakeWorkload{replicas: 1}
	cluster := &fakeCluster{topology: activeTopology(1)}
	manager := newTestManager(policy, workload, cluster)

	if err := manager.Reconcile(context.Background()); err != nil {
		t.Fatal(err)
	}
	if len(workload.scales) != 1 || workload.scales[0] != 2 || len(cluster.calls) != 0 {
		t.Fatalf("expected pod provisioning only, scales=%v calls=%v", workload.scales, cluster.calls)
	}

	workload.started = true
	if err := manager.Reconcile(context.Background()); err != nil {
		t.Fatal(err)
	}
	if len(cluster.calls) != 2 || !cluster.calls[0].dryRun || cluster.calls[1].dryRun {
		t.Fatalf("expected dry-run then apply, got %v", cluster.calls)
	}
}

func TestManagerDoesNotRemovePodBeforeTopology(t *testing.T) {
	policy := Policy{Mode: "scheduled", MinBrokers: 1, MaxBrokers: 3, TargetBrokers: 1}
	workload := &fakeWorkload{replicas: 2, started: true}
	cluster := &fakeCluster{topology: activeTopology(2)}
	manager := newTestManager(policy, workload, cluster)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := manager.Reconcile(ctx); err == nil {
		t.Fatal("expected cancelled topology wait")
	}
	if len(workload.scales) != 0 {
		t.Fatalf("pod was removed before topology completion: %v", workload.scales)
	}
	if len(cluster.calls) != 2 || !cluster.calls[0].dryRun || cluster.calls[1].dryRun {
		t.Fatalf("expected dry-run then apply, got %v", cluster.calls)
	}
}

func TestManagerBlocksDuringPendingChange(t *testing.T) {
	policy := Policy{Mode: "scheduled", MinBrokers: 1, MaxBrokers: 3, TargetBrokers: 2}
	workload := &fakeWorkload{replicas: 1}
	topology := activeTopology(1)
	topology.PendingChange = &Change{ID: 42, Status: "IN_PROGRESS"}
	cluster := &fakeCluster{topology: topology}
	manager := newTestManager(policy, workload, cluster)

	if err := manager.Reconcile(context.Background()); err != nil {
		t.Fatal(err)
	}
	if len(workload.scales) != 0 || len(cluster.calls) != 0 || manager.Status().Phase != "blocked" {
		t.Fatalf("unexpected action during pending change: scales=%v calls=%v status=%#v", workload.scales, cluster.calls, manager.Status())
	}
}

func TestManagerRecoversCompletedScaleDown(t *testing.T) {
	policy := Policy{Mode: "scheduled", MinBrokers: 1, MaxBrokers: 3, TargetBrokers: 1}
	workload := &fakeWorkload{replicas: 2, target: 1, started: true}
	cluster := &fakeCluster{topology: activeTopology(1)}
	manager := newTestManager(policy, workload, cluster)

	if err := manager.Reconcile(context.Background()); err != nil {
		t.Fatal(err)
	}
	if len(workload.scales) != 1 || workload.scales[0] != 1 || len(cluster.calls) != 0 {
		t.Fatalf("expected workload contraction only, scales=%v calls=%v", workload.scales, cluster.calls)
	}
}
