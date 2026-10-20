// Copyright Camunda Services GmbH and/or licensed to Camunda Services GmbH
// under one or more contributor license agreements. See the NOTICE file
// distributed with this work for additional information regarding copyright
// ownership. Licensed under the Apache License, Version 2.0; you may not use
// this file except in compliance with the License. You may obtain a copy of
// the License at http://www.apache.org/licenses/LICENSE-2.0

package capacity

import (
	"testing"
	"time"
)

func TestPlanner(t *testing.T) {
	t.Run("uses explicit target", func(t *testing.T) {
		planner := &Planner{}
		decision, err := planner.Decide(Policy{Mode: "automatic", MinBrokers: 1, MaxBrokers: 5, TargetBrokers: 3}, 1, nil, time.Now())
		if err != nil || decision.Desired != 3 || decision.Reason != "explicit target" {
			t.Fatalf("unexpected decision: %#v, %v", decision, err)
		}
	})

	t.Run("uses active scheduled minimum", func(t *testing.T) {
		now := time.Now().UTC()
		planner := &Planner{}
		policy := Policy{Mode: "scheduled", MinBrokers: 1, MaxBrokers: 5, ScheduledMinimum: 4, ScheduleStartsAt: now.Add(-time.Minute).Format(time.RFC3339), ScheduleEndsAt: now.Add(time.Minute).Format(time.RFC3339)}
		decision, err := planner.Decide(policy, 1, nil, now)
		if err != nil || decision.Desired != 4 {
			t.Fatalf("unexpected decision: %#v, %v", decision, err)
		}
	})

	t.Run("stabilizes automatic scale up", func(t *testing.T) {
		planner := &Planner{}
		pressure := 0.9
		policy := Policy{Mode: "automatic", MinBrokers: 1, MaxBrokers: 3, ScaleUpThreshold: 0.8, ScaleDownThreshold: 0.2, ScaleUpSamples: 2}
		first, _ := planner.Decide(policy, 1, &pressure, time.Now())
		second, _ := planner.Decide(policy, 1, &pressure, time.Now())
		if first.Desired != 1 || second.Desired != 2 {
			t.Fatalf("unexpected decisions: %#v, %#v", first, second)
		}
	})

	t.Run("does not scale down without pressure", func(t *testing.T) {
		planner := &Planner{}
		decision, err := planner.Decide(Policy{Mode: "automatic", MinBrokers: 1, MaxBrokers: 3}, 2, nil, time.Now())
		if err != nil || decision.Desired != 2 || decision.Confidence != "low" {
			t.Fatalf("unexpected decision: %#v, %v", decision, err)
		}
	})
	t.Run("does not scale below active scheduled minimum", func(t *testing.T) {
		now := time.Now().UTC()
		planner := &Planner{}
		pressure := 0.0
		policy := Policy{Mode: "automatic", MinBrokers: 1, MaxBrokers: 5, ScaleUpThreshold: 0.8, ScaleDownThreshold: 0.2, ScheduledMinimum: 3, ScheduleStartsAt: now.Add(-time.Minute).Format(time.RFC3339), ScheduleEndsAt: now.Add(time.Minute).Format(time.RFC3339)}
		decision, err := planner.Decide(policy, 3, &pressure, now)
		if err != nil || decision.Desired != 3 {
			t.Fatalf("unexpected decision: %#v, %v", decision, err)
		}
		decision, err = planner.Decide(policy, 4, &pressure, now)
		if err != nil || decision.Desired != 3 {
			t.Fatalf("expected scale down to floor: %#v, %v", decision, err)
		}
	})

	t.Run("honours stabilization windows", func(t *testing.T) {
		start := time.Now().UTC()
		planner := &Planner{}
		high, low := 0.9, 0.1
		policy := Policy{Mode: "automatic", MinBrokers: 1, MaxBrokers: 3, ScaleUpThreshold: 0.8, ScaleDownThreshold: 0.2, ScaleUpStabilization: "1m", ScaleDownStabilization: "5m"}
		if d, _ := planner.Decide(policy, 1, &high, start); d.Desired != 1 {
			t.Fatalf("scaled up before window: %#v", d)
		}
		if d, _ := planner.Decide(policy, 1, &high, start.Add(30*time.Second)); d.Desired != 1 {
			t.Fatalf("scaled up before window: %#v", d)
		}
		if d, _ := planner.Decide(policy, 1, &high, start.Add(time.Minute)); d.Desired != 2 {
			t.Fatalf("did not scale up after window: %#v", d)
		}
		if d, _ := planner.Decide(policy, 2, &low, start.Add(2*time.Minute)); d.Desired != 2 {
			t.Fatalf("scaled down before window: %#v", d)
		}
		if d, _ := planner.Decide(policy, 2, &low, start.Add(7*time.Minute)); d.Desired != 1 {
			t.Fatalf("did not scale down after window: %#v", d)
		}
	})
}

func TestPolicyValidation(t *testing.T) {
	base := Policy{Mode: "scheduled", MinBrokers: 1, MaxBrokers: 5}
	missing := base
	missing.ScheduledMinimum = 3
	if err := missing.validate(); err == nil {
		t.Fatal("expected scheduled minimum without timestamps to be rejected")
	}
	bad := base
	bad.ScaleUpStabilization = "soon"
	if err := bad.validate(); err == nil {
		t.Fatal("expected invalid stabilization to be rejected")
	}
	negative := base
	negative.ScaleDownStabilization = "-1m"
	if err := negative.validate(); err == nil {
		t.Fatal("expected negative stabilization to be rejected")
	}
}
