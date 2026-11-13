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
	"os"
	"strings"
	"testing"
	"time"
)

type staticPolicies struct{ policy Policy }

func (s staticPolicies) Policy() (Policy, error) { return s.policy, nil }

type fakeMetrics struct{ values map[string]float64 }

func (f fakeMetrics) Query(_ context.Context, query string) (*float64, error) {
	if value, ok := f.values[query]; ok {
		return &value, nil
	}
	return nil, nil
}

type fakeWorkload struct {
	recorded  *CapacityChange
	replicas  int
	target    int
	started   bool
	scales    []int
	completed []int
}

func (f *fakeWorkload) State(context.Context) (WorkloadState, error) {
	target := f.target
	if target == 0 {
		target = f.replicas
	}
	return WorkloadState{Replicas: f.replicas, Target: target, LastChange: f.recorded}, nil
}
func (f *fakeWorkload) RecordChange(_ context.Context, change CapacityChange) error {
	f.recorded = &change
	return nil
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
func (f *fakeWorkload) MarkCompleted(_ context.Context, brokers int) error {
	f.completed = append(f.completed, brokers)
	return nil
}
func (f *fakeWorkload) PodStarted(context.Context, int) (bool, error) { return f.started, nil }

type scaleCall struct {
	target int
	dryRun bool
}

type fakeCluster struct {
	topology   Topology
	topologies []Topology
	calls      []scaleCall
}

func (f *fakeCluster) Topology(context.Context) (Topology, error) {
	if len(f.topologies) > 0 {
		topology := f.topologies[0]
		f.topologies = f.topologies[1:]
		return topology, nil
	}
	return f.topology, nil
}
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
		Metrics: fakeMetrics{}, Planner: &Planner{}, Logger: slog.New(slog.NewTextHandler(io.Discard, nil)),
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

func TestManagerDoesNotRemovePodAfterFailedTopologyChange(t *testing.T) {
	policy := Policy{Mode: "scheduled", MinBrokers: 1, MaxBrokers: 3, TargetBrokers: 1}
	workload := &fakeWorkload{replicas: 2, started: true}
	failed := activeTopology(2)
	failed.LastChange = &Change{ID: 1, Status: "FAILED"}
	cluster := &fakeCluster{
		topology:   failed,
		topologies: []Topology{activeTopology(2), failed},
	}
	manager := newTestManager(policy, workload, cluster)

	if err := manager.Reconcile(context.Background()); err == nil {
		t.Fatal("expected failed topology change")
	}
	if len(workload.scales) != 0 {
		t.Fatalf("pod was removed after failed topology change: %v", workload.scales)
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

func TestManagerQueriesSignalsWithBrokerCount(t *testing.T) {
	policy := automaticPolicy()
	policy.ScaleDownSignals[1].Query = `sum(cpu{namespace="$NAMESPACE",pod=~"$STATEFULSET-[0-9]+"}) / ($BROKERS - 1)`
	workload := &fakeWorkload{replicas: 2}
	contracted := activeTopology(1)
	contracted.LastChange = &Change{ID: 1, Status: "COMPLETED"}
	cluster := &fakeCluster{topology: contracted, topologies: []Topology{activeTopology(2)}}
	manager := newTestManager(policy, workload, cluster)
	manager.OperationWait = time.Second
	manager.Namespace, manager.StatefulSet = "ns", "zeebe"
	manager.Metrics = fakeMetrics{values: map[string]float64{
		"drop": 0, "cpu": 0.2, "backlog": 0, `sum(cpu{namespace="ns",pod=~"zeebe-[0-9]+"}) / (2 - 1)`: 0.3,
	}}
	if err := manager.Reconcile(context.Background()); err != nil {
		t.Fatal(err)
	}
	if change := manager.Status().LastChange; change == nil || change.From != 2 || change.To != 1 || change.Signals["fits"] != 0.3 {
		t.Fatalf("expected last change evidence, got %#v", change)
	}
	if got := manager.Status().Signals["fits"]; got != 0.3 {
		t.Fatalf("expected $BROKERS substitution, signals=%v", manager.Status().Signals)
	}
	if len(cluster.calls) != 2 || cluster.calls[1].target != 1 {
		t.Fatalf("expected scale down to 1, calls=%v", cluster.calls)
	}
}

func TestManagerSelectorUsesConfiguredLabels(t *testing.T) {
	manager := &Manager{Namespace: "ns", StatefulSet: "zeebe", NamespaceLabel: "kubernetes_namespace", PodLabel: "kubernetes_pod_name"}
	got := manager.placeholders(3).Replace(`up{$SELECTOR} / $BROKERS by ($POD_LABEL, $NAMESPACE_LABEL)`)
	want := `up{kubernetes_namespace="ns",kubernetes_pod_name=~"zeebe-[0-9]+"} / 3 by (kubernetes_pod_name, kubernetes_namespace)`
	if got != want {
		t.Fatalf("got %s\nwant %s", got, want)
	}
	if got := (&Manager{Namespace: "ns", StatefulSet: "zeebe"}).placeholders(1).Replace("$SELECTOR"); got != `namespace="ns",pod=~"zeebe-[0-9]+"` {
		t.Fatalf("default selector %s", got)
	}
}

func TestManagerWithdrawsBrokerPodThatNeverBecomesReady(t *testing.T) {
	policy := Policy{Mode: "scheduled", MinBrokers: 1, MaxBrokers: 3, TargetBrokers: 2}
	workload := &fakeWorkload{replicas: 2, target: 2, started: false}
	cluster := &fakeCluster{topology: activeTopology(1)}
	manager := newTestManager(policy, workload, cluster)
	manager.JoinTimeout = time.Hour

	if err := manager.Reconcile(context.Background()); err != nil {
		t.Fatal(err)
	}
	if status := manager.Status(); status.Phase != "joining-broker" || !strings.Contains(status.Reason, "waiting for broker pod 1") {
		t.Fatalf("expected waiting join status, got %#v", status)
	}
	if len(workload.scales) != 0 || len(cluster.calls) != 0 {
		t.Fatalf("acted before readiness: scales=%v calls=%v", workload.scales, cluster.calls)
	}

	manager.joinSince = time.Now().Add(-2 * time.Hour)
	if err := manager.Reconcile(context.Background()); err == nil || !strings.Contains(err.Error(), "not Ready") {
		t.Fatalf("expected join timeout, got %v", err)
	}
	if len(workload.scales) != 1 || workload.scales[0] != 1 || len(cluster.calls) != 0 {
		t.Fatalf("expected unready pod withdrawn without topology change, scales=%v calls=%v", workload.scales, cluster.calls)
	}
}

func TestManagerReportsProgressWhileEvacuating(t *testing.T) {
	policy := Policy{Mode: "scheduled", MinBrokers: 1, MaxBrokers: 3, TargetBrokers: 1}
	workload := &fakeWorkload{replicas: 2, started: true}
	inProgress := activeTopology(2)
	inProgress.PendingChange = &Change{ID: 1, Status: "IN_PROGRESS"}
	done := activeTopology(1)
	done.LastChange = &Change{ID: 1, Status: "COMPLETED"}
	cluster := &fakeCluster{topology: done, topologies: []Topology{activeTopology(2), inProgress, inProgress}}
	manager := newTestManager(policy, workload, cluster)
	manager.OperationWait = time.Second
	var phases []string
	manager.Logger = slog.New(slog.NewTextHandler(&reasonRecorder{reasons: &phases}, nil))

	if err := manager.Reconcile(context.Background()); err != nil {
		t.Fatal(err)
	}
	found := false
	for _, line := range phases {
		if strings.Contains(line, "change 1 IN_PROGRESS") {
			found = true
		}
	}
	if !found {
		t.Fatalf("no in-progress status published during evacuation: %v", phases)
	}
	if len(workload.scales) != 1 || workload.scales[0] != 1 {
		t.Fatalf("expected contraction after completion, scales=%v", workload.scales)
	}
}

type reasonRecorder struct{ reasons *[]string }

func (r *reasonRecorder) Write(p []byte) (int, error) {
	*r.reasons = append(*r.reasons, string(p))
	return len(p), nil
}

func TestManagerRestoresLastChangeAfterRestart(t *testing.T) {
	policy := automaticPolicy()
	workload := &fakeWorkload{replicas: 1}
	cluster := &fakeCluster{topology: activeTopology(1)}
	cluster.topology.Brokers[0].Partitions = []Partition{{ID: 1, State: "ACTIVE"}, {ID: 2, State: "ACTIVE"}}
	manager := newTestManager(policy, workload, cluster)
	manager.Metrics = fakeMetrics{values: map[string]float64{"drop": 0.2, "cpu": 0.95, "cpu-fewer": 1.4, "backlog": 0}}
	if err := manager.Reconcile(context.Background()); err != nil {
		t.Fatal(err)
	}
	if workload.recorded == nil || workload.recorded.From != 1 || workload.recorded.To != 2 {
		t.Fatalf("scale-up evidence not recorded: %#v", workload.recorded)
	}

	restarted := newTestManager(policy, workload, cluster)
	if err := restarted.Reconcile(context.Background()); err != nil {
		t.Fatal(err)
	}
	change := restarted.Status().LastChange
	if change == nil || change.To != 2 || change.Signals["backpressure"] != 0.2 {
		t.Fatalf("restarted manager lost the last change evidence: %#v", change)
	}
}

// $(NAME) survives envsubst-style tooling (os.Expand leaves it untouched)
// and must expand exactly like $NAME.
func TestManagerParenthesisedPlaceholders(t *testing.T) {
	manager := &Manager{Namespace: "ns", StatefulSet: "zeebe"}
	got := manager.placeholders(3).Replace(`x{$(SELECTOR)} / ($(BROKERS) - 1) by ($(POD_LABEL)) $(NAMESPACE)/$(STATEFULSET)`)
	want := `x{namespace="ns",pod=~"zeebe-[0-9]+"} / (3 - 1) by (pod) ns/zeebe`
	if got != want {
		t.Fatalf("got %s\nwant %s", got, want)
	}
	if expanded := os.Expand(`$(SELECTOR) $(BROKERS)`, func(string) string { return "SUBSTITUTED" }); expanded != `$(SELECTOR) $(BROKERS)` {
		t.Fatalf("os.Expand changed the parenthesised form: %s", expanded)
	}
}
