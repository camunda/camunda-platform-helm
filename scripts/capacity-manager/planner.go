// Copyright Camunda Services GmbH and/or licensed to Camunda Services GmbH
// under one or more contributor license agreements. See the NOTICE file
// distributed with this work for additional information regarding copyright
// ownership. Licensed under the Apache License, Version 2.0; you may not use
// this file except in compliance with the License. You may obtain a copy of
// the License at http://www.apache.org/licenses/LICENSE-2.0

package capacity

import (
	"fmt"
	"time"
)

type Decision struct {
	Current    int
	Desired    int
	Mode       string
	Reason     string
	Pressure   float64
	Confidence string
}

type Planner struct {
	upSamples   int
	downSamples int
}

func (p *Planner) Decide(policy Policy, current int, pressure *float64, now time.Time) (Decision, error) {
	decision := Decision{Current: current, Desired: current, Mode: policy.Mode, Confidence: "high"}
	if policy.TargetBrokers > 0 {
		decision.Desired = policy.TargetBrokers
		decision.Reason = "explicit target"
		return decision, nil
	}

	scheduledMinimum, err := policy.scheduledMinimum(now)
	if err != nil {
		return Decision{}, err
	}
	if scheduledMinimum > current {
		decision.Desired = scheduledMinimum
		decision.Reason = "scheduled minimum"
		return decision, nil
	}

	if policy.Mode != "automatic" {
		decision.Reason = "no automatic action"
		return decision, nil
	}
	if pressure == nil {
		decision.Confidence = "low"
		decision.Reason = "pressure unavailable"
		return decision, nil
	}
	decision.Pressure = *pressure
	upRequired := max(policy.ScaleUpSamples, 1)
	downRequired := max(policy.ScaleDownSamples, 1)
	if *pressure >= policy.ScaleUpThreshold && current < policy.MaxBrokers {
		p.upSamples++
		p.downSamples = 0
		if p.upSamples >= upRequired {
			decision.Desired = current + 1
			decision.Reason = fmt.Sprintf("pressure %.3f reached scale-up threshold %.3f", *pressure, policy.ScaleUpThreshold)
			p.upSamples = 0
			return decision, nil
		}
		decision.Reason = "waiting for scale-up stabilization"
		return decision, nil
	}
	if *pressure <= policy.ScaleDownThreshold && current > policy.MinBrokers {
		p.downSamples++
		p.upSamples = 0
		if p.downSamples >= downRequired {
			decision.Desired = current - 1
			decision.Reason = fmt.Sprintf("pressure %.3f reached scale-down threshold %.3f", *pressure, policy.ScaleDownThreshold)
			p.downSamples = 0
			return decision, nil
		}
		decision.Reason = "waiting for scale-down stabilization"
		return decision, nil
	}
	p.upSamples = 0
	p.downSamples = 0
	decision.Reason = "pressure within target range"
	return decision, nil
}
