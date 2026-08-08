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
}
