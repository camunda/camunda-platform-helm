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
	upSince     time.Time
	downSince   time.Time
}

func (p *Planner) resetUp() {
	p.upSamples = 0
	p.upSince = time.Time{}
}

func (p *Planner) resetDown() {
	p.downSamples = 0
	p.downSince = time.Time{}
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
	upWindow, err := parseStabilization(policy.ScaleUpStabilization)
	if err != nil {
		return Decision{}, fmt.Errorf("parse scaleUpStabilization: %w", err)
	}
	downWindow, err := parseStabilization(policy.ScaleDownStabilization)
	if err != nil {
		return Decision{}, fmt.Errorf("parse scaleDownStabilization: %w", err)
	}
	// An active scheduled minimum is a floor for automatic scale-down.
	downFloor := max(policy.MinBrokers, scheduledMinimum)
	if *pressure >= policy.ScaleUpThreshold && current < policy.MaxBrokers {
		p.resetDown()
		p.upSamples++
		if p.upSince.IsZero() {
			p.upSince = now
		}
		if p.upSamples >= upRequired && now.Sub(p.upSince) >= upWindow {
			decision.Desired = current + 1
			decision.Reason = fmt.Sprintf("pressure %.3f reached scale-up threshold %.3f", *pressure, policy.ScaleUpThreshold)
			p.resetUp()
			return decision, nil
		}
		decision.Reason = "waiting for scale-up stabilization"
		return decision, nil
	}
	if *pressure <= policy.ScaleDownThreshold && current > downFloor {
		p.resetUp()
		p.downSamples++
		if p.downSince.IsZero() {
			p.downSince = now
		}
		if p.downSamples >= downRequired && now.Sub(p.downSince) >= downWindow {
			decision.Desired = current - 1
			decision.Reason = fmt.Sprintf("pressure %.3f reached scale-down threshold %.3f", *pressure, policy.ScaleDownThreshold)
			p.resetDown()
			return decision, nil
		}
		decision.Reason = "waiting for scale-down stabilization"
		return decision, nil
	}
	p.resetUp()
	p.resetDown()
	decision.Reason = "pressure within target range"
	return decision, nil
}
