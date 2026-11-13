// Copyright Camunda Services GmbH and/or licensed to Camunda Services GmbH
// under one or more contributor license agreements. See the NOTICE file
// distributed with this work for additional information regarding copyright
// ownership. Licensed under the Apache License, Version 2.0; you may not use
// this file except in compliance with the License. You may obtain a copy of
// the License at http://www.apache.org/licenses/LICENSE-2.0

package capacity

import (
	"encoding/json"
	"math"
	"strings"
	"testing"
	"time"
)

func value(v float64) *float64 { return &v }

func automaticPolicy() Policy {
	return Policy{
		Mode: "automatic", MinBrokers: 1, MaxBrokers: 5,
		ScaleUpSignals: []Signal{
			{Name: "backpressure", Query: "drop", Operator: ">=", Threshold: 0.05},
			{Name: "cpu", Query: "cpu", Operator: ">=", Threshold: 0.8},
		},
		ScaleDownSignals: []Signal{
			{Name: "idle", Query: "drop", Operator: "<=", Threshold: 0.001},
			{Name: "fits", Query: "cpu-fewer", Operator: "<=", Threshold: 0.6},
		},
		VetoSignals: []Signal{
			{Name: "exporter-backlog", Query: "backlog", Operator: ">=", Threshold: 100000},
		},
	}
}

// saturated returns readings where every scale-up signal holds.
func saturated() map[string]*float64 {
	return map[string]*float64{"backpressure": value(0.2), "cpu": value(0.95), "idle": value(0.2), "fits": value(1.4), "exporter-backlog": value(0)}
}

func quiet() map[string]*float64 {
	return map[string]*float64{"backpressure": value(0), "cpu": value(0.2), "idle": value(0), "fits": value(0.3), "exporter-backlog": value(0)}
}

func TestPlanner(t *testing.T) {
	now := time.Now().UTC()

	t.Run("uses explicit target", func(t *testing.T) {
		decision, err := (&Planner{}).Decide(Policy{Mode: "automatic", MinBrokers: 1, MaxBrokers: 5, TargetBrokers: 3}, Inputs{Current: 1}, now)
		if err != nil || decision.Desired != 3 || decision.Reason != "explicit target" {
			t.Fatalf("unexpected decision: %#v, %v", decision, err)
		}
	})

	t.Run("uses active scheduled minimum", func(t *testing.T) {
		policy := Policy{Mode: "scheduled", MinBrokers: 1, MaxBrokers: 5, ScheduledMinimum: 4, ScheduleStartsAt: now.Add(-time.Minute).Format(time.RFC3339), ScheduleEndsAt: now.Add(time.Minute).Format(time.RFC3339)}
		decision, err := (&Planner{}).Decide(policy, Inputs{Current: 1}, now)
		if err != nil || decision.Desired != 4 {
			t.Fatalf("unexpected decision: %#v, %v", decision, err)
		}
	})

	t.Run("scales up only when every scale-up signal holds", func(t *testing.T) {
		readings := saturated()
		readings["cpu"] = value(0.3)
		decision, _ := (&Planner{}).Decide(automaticPolicy(), Inputs{Current: 1, PartitionReplicas: 3, Readings: readings}, now)
		if decision.Desired != 1 {
			t.Fatalf("scaled up although cpu is low: %#v", decision)
		}
		decision, _ = (&Planner{}).Decide(automaticPolicy(), Inputs{Current: 1, PartitionReplicas: 3, Readings: saturated()}, now)
		if decision.Desired != 2 || !strings.Contains(decision.Reason, "backpressure, cpu") {
			t.Fatalf("expected scale up naming signals: %#v", decision)
		}
	})

	t.Run("does not scale beyond partition replicas", func(t *testing.T) {
		decision, _ := (&Planner{}).Decide(automaticPolicy(), Inputs{Current: 3, PartitionReplicas: 3, Readings: saturated()}, now)
		if decision.Desired != 3 || !strings.Contains(decision.Reason, "ceiling") {
			t.Fatalf("expected partition replica ceiling: %#v", decision)
		}
	})

	t.Run("veto blocks scaling", func(t *testing.T) {
		readings := saturated()
		readings["exporter-backlog"] = value(500000)
		decision, _ := (&Planner{}).Decide(automaticPolicy(), Inputs{Current: 1, PartitionReplicas: 3, Readings: readings}, now)
		if decision.Desired != 1 || decision.Reason != "vetoed by exporter-backlog" {
			t.Fatalf("expected veto: %#v", decision)
		}
	})

	t.Run("missing signal holds with low confidence", func(t *testing.T) {
		readings := saturated()
		delete(readings, "cpu")
		decision, _ := (&Planner{}).Decide(automaticPolicy(), Inputs{Current: 1, PartitionReplicas: 3, Readings: readings}, now)
		if decision.Desired != 1 || decision.Confidence != "low" {
			t.Fatalf("expected hold: %#v", decision)
		}
		readings = saturated()
		delete(readings, "exporter-backlog")
		decision, _ = (&Planner{}).Decide(automaticPolicy(), Inputs{Current: 1, PartitionReplicas: 3, Readings: readings}, now)
		if decision.Desired != 1 || decision.Confidence != "low" {
			t.Fatalf("expected hold on missing veto: %#v", decision)
		}
	})

	t.Run("missing reading resets stabilization streak", func(t *testing.T) {
		planner := &Planner{}
		policy := automaticPolicy()
		policy.ScaleUpSamples = 2
		planner.Decide(policy, Inputs{Current: 1, PartitionReplicas: 3, Readings: saturated()}, now)
		planner.Decide(policy, Inputs{Current: 1, PartitionReplicas: 3, Readings: map[string]*float64{}}, now)
		decision, _ := planner.Decide(policy, Inputs{Current: 1, PartitionReplicas: 3, Readings: saturated()}, now)
		if decision.Desired != 1 {
			t.Fatalf("gap counted toward the streak: %#v", decision)
		}
	})

	t.Run("scales down only when every scale-down signal holds", func(t *testing.T) {
		readings := quiet()
		readings["fits"] = value(0.9)
		decision, _ := (&Planner{}).Decide(automaticPolicy(), Inputs{Current: 3, PartitionReplicas: 3, Readings: readings}, now)
		if decision.Desired != 3 {
			t.Fatalf("scaled down although load would not fit: %#v", decision)
		}
		decision, _ = (&Planner{}).Decide(automaticPolicy(), Inputs{Current: 3, PartitionReplicas: 3, Readings: quiet()}, now)
		if decision.Desired != 2 {
			t.Fatalf("expected scale down: %#v", decision)
		}
	})

	t.Run("does not scale below active scheduled minimum", func(t *testing.T) {
		policy := automaticPolicy()
		policy.ScheduledMinimum = 3
		policy.ScheduleStartsAt = now.Add(-time.Minute).Format(time.RFC3339)
		policy.ScheduleEndsAt = now.Add(time.Minute).Format(time.RFC3339)
		decision, _ := (&Planner{}).Decide(policy, Inputs{Current: 3, PartitionReplicas: 6, Readings: quiet()}, now)
		if decision.Desired != 3 {
			t.Fatalf("scaled below scheduled minimum: %#v", decision)
		}
		decision, _ = (&Planner{}).Decide(policy, Inputs{Current: 4, PartitionReplicas: 6, Readings: quiet()}, now)
		if decision.Desired != 3 {
			t.Fatalf("expected scale down to floor: %#v", decision)
		}
	})

	t.Run("honours stabilization windows and cooldown", func(t *testing.T) {
		planner := &Planner{}
		policy := automaticPolicy()
		policy.ScaleUpStabilization = "1m"
		policy.Cooldown = "10m"
		in := Inputs{Current: 1, PartitionReplicas: 3, Readings: saturated()}
		if d, _ := planner.Decide(policy, in, now); d.Desired != 1 {
			t.Fatalf("scaled up before window: %#v", d)
		}
		if d, _ := planner.Decide(policy, in, now.Add(30*time.Second)); d.Desired != 1 {
			t.Fatalf("scaled up before window: %#v", d)
		}
		if d, _ := planner.Decide(policy, in, now.Add(time.Minute)); d.Desired != 2 {
			t.Fatalf("did not scale up after window: %#v", d)
		}
		in.Current = 2
		if d, _ := planner.Decide(policy, in, now.Add(5*time.Minute)); d.Desired != 2 || !strings.Contains(d.Reason, "cooling down") {
			t.Fatalf("acted during cooldown: %#v", d)
		}
	})
}

func TestPolicyValidation(t *testing.T) {
	base := Policy{Mode: "scheduled", MinBrokers: 1, MaxBrokers: 5}
	cases := map[string]func(*Policy){
		"scheduled minimum without timestamps": func(p *Policy) { p.ScheduledMinimum = 3 },
		"invalid stabilization":                func(p *Policy) { p.ScaleUpStabilization = "soon" },
		"negative stabilization":               func(p *Policy) { p.ScaleDownStabilization = "-1m" },
		"invalid cooldown":                     func(p *Policy) { p.Cooldown = "later" },
		"automatic without signals":            func(p *Policy) { p.Mode = "automatic" },
		"bad operator": func(p *Policy) {
			p.VetoSignals = []Signal{{Name: "x", Query: "q", Operator: ">"}}
		},
		"duplicate signal name": func(p *Policy) {
			p.ScaleUpSignals = []Signal{{Name: "x", Query: "q", Operator: ">="}}
			p.VetoSignals = []Signal{{Name: "x", Query: "q", Operator: ">="}}
		},
		"signal without query": func(p *Policy) {
			p.ScaleUpSignals = []Signal{{Name: "x", Operator: ">="}}
		},
	}
	for name, mutate := range cases {
		policy := base
		mutate(&policy)
		if err := policy.validate(); err == nil {
			t.Errorf("%s: expected validation error", name)
		}
	}
	valid := automaticPolicy()
	if err := valid.validate(); err != nil {
		t.Fatalf("valid automatic policy rejected: %v", err)
	}
}

// With one broker, "fits on one broker fewer" divides by zero and Prometheus
// returns +Inf. The reading must still block the scale-down and must not
// break JSON encoding of the decision.
func TestPlannerHandlesInfiniteReadings(t *testing.T) {
	readings := quiet()
	inf := math.Inf(1)
	readings["fits"] = &inf
	decision, err := (&Planner{}).Decide(automaticPolicy(), Inputs{Current: 2, PartitionReplicas: 3, Readings: readings}, time.Now())
	if err != nil || decision.Desired != 2 {
		t.Fatalf("+Inf must not satisfy a <= signal: %#v, %v", decision, err)
	}
	if _, ok := decision.Signals["fits"]; ok {
		t.Fatalf("non-finite reading was reported: %v", decision.Signals)
	}
	status := Status{Phase: "stable", Signals: decision.Signals}
	if body := status.JSON(); !json.Valid(body) || len(body) < 3 {
		t.Fatalf("status did not encode: %q", body)
	}
}

func TestStatusJSONNeverEmpty(t *testing.T) {
	body := Status{Phase: "stable", Signals: map[string]float64{"x": math.Inf(1)}}.JSON()
	var decoded Status
	if err := json.Unmarshal(body, &decoded); err != nil || decoded.Phase != "stable" || decoded.LastError == "" {
		t.Fatalf("expected a valid status reporting the encode error, got %q (%v)", body, err)
	}
}
