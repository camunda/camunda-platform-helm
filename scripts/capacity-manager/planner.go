// Copyright Camunda Services GmbH and/or licensed to Camunda Services GmbH
// under one or more contributor license agreements. See the NOTICE file
// distributed with this work for additional information regarding copyright
// ownership. Licensed under the Apache License, Version 2.0; you may not use
// this file except in compliance with the License. You may obtain a copy of
// the License at http://www.apache.org/licenses/LICENSE-2.0

package capacity

import (
	"fmt"
	"strings"
	"time"
)

type Decision struct {
	Current    int
	Desired    int
	Mode       string
	Reason     string
	Signals    map[string]float64
	Confidence string
}

// Inputs is the cluster state the planner decides on.
type Inputs struct {
	// Current is the number of active brokers in the topology.
	Current int
	// PartitionReplicas is the total number of partition replicas in the
	// topology. A broker beyond this count cannot host any partition, so it
	// is the hard ceiling for useful scale-up.
	PartitionReplicas int
	// Readings holds the reduced value of every policy signal by name; a nil
	// value means the signal had no data.
	Readings map[string]*float64
}

type Planner struct {
	upSamples   int
	downSamples int
	upSince     time.Time
	downSince   time.Time
	lastChange  time.Time
}

func (p *Planner) resetUp() {
	p.upSamples = 0
	p.upSince = time.Time{}
}

func (p *Planner) resetDown() {
	p.downSamples = 0
	p.downSince = time.Time{}
}

func (p *Planner) hold(decision Decision, confidence, reason string) (Decision, error) {
	p.resetUp()
	p.resetDown()
	decision.Confidence = confidence
	decision.Reason = reason
	return decision, nil
}

// evaluate reports whether every signal holds. missing lists signals without
// data; failing lists signals with data that do not hold.
func evaluate(signals []Signal, readings map[string]*float64) (missing, failing []string) {
	for _, signal := range signals {
		value := readings[signal.Name]
		if value == nil {
			missing = append(missing, signal.Name)
			continue
		}
		if !signal.holds(*value) {
			failing = append(failing, signal.Name)
		}
	}
	return missing, failing
}

func (p *Planner) Decide(policy Policy, in Inputs, now time.Time) (Decision, error) {
	current := in.Current
	decision := Decision{Current: current, Desired: current, Mode: policy.Mode, Confidence: "high", Signals: map[string]float64{}}
	for name, value := range in.Readings {
		if value != nil {
			decision.Signals[name] = *value
		}
	}
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

	cooldown, err := parseStabilization(policy.Cooldown)
	if err != nil {
		return Decision{}, fmt.Errorf("parse cooldown: %w", err)
	}
	if !p.lastChange.IsZero() && now.Sub(p.lastChange) < cooldown {
		return p.hold(decision, "high", "cooling down after the last capacity change")
	}

	// Any missing reading means the planner cannot tell whether a change is
	// safe or useful, so it holds and restarts every stabilization streak.
	vetoMissing, vetoing := evaluateVetoes(policy.VetoSignals, in.Readings)
	if len(vetoMissing) > 0 {
		return p.hold(decision, "low", "veto signals unavailable: "+strings.Join(vetoMissing, ", "))
	}
	if len(vetoing) > 0 {
		return p.hold(decision, "high", "vetoed by "+strings.Join(vetoing, ", "))
	}
	upMissing, upFailing := evaluate(policy.ScaleUpSignals, in.Readings)
	downMissing, downFailing := evaluate(policy.ScaleDownSignals, in.Readings)
	if missing := append(upMissing, downMissing...); len(missing) > 0 {
		return p.hold(decision, "low", "signals unavailable: "+strings.Join(missing, ", "))
	}

	upWindow, err := parseStabilization(policy.ScaleUpStabilization)
	if err != nil {
		return Decision{}, fmt.Errorf("parse scaleUpStabilization: %w", err)
	}
	downWindow, err := parseStabilization(policy.ScaleDownStabilization)
	if err != nil {
		return Decision{}, fmt.Errorf("parse scaleDownStabilization: %w", err)
	}
	upRequired := max(policy.ScaleUpSamples, 1)
	downRequired := max(policy.ScaleDownSamples, 1)
	ceiling := policy.MaxBrokers
	if in.PartitionReplicas > 0 {
		ceiling = min(ceiling, in.PartitionReplicas)
	}
	// An active scheduled minimum is a floor for automatic scale-down.
	floor := max(policy.MinBrokers, scheduledMinimum)

	if len(upFailing) == 0 {
		if current >= ceiling {
			p.resetUp()
			p.resetDown()
			decision.Reason = fmt.Sprintf("scale-up signals hold but %d brokers is the ceiling (maxBrokers %d, partition replicas %d)", ceiling, policy.MaxBrokers, in.PartitionReplicas)
			return decision, nil
		}
		p.resetDown()
		p.upSamples++
		if p.upSince.IsZero() {
			p.upSince = now
		}
		if p.upSamples >= upRequired && now.Sub(p.upSince) >= upWindow {
			decision.Desired = current + 1
			decision.Reason = "scale-up signals hold: " + names(policy.ScaleUpSignals)
			p.resetUp()
			p.lastChange = now
			return decision, nil
		}
		decision.Reason = "waiting for scale-up stabilization"
		return decision, nil
	}
	if len(downFailing) == 0 && current > floor {
		p.resetUp()
		p.downSamples++
		if p.downSince.IsZero() {
			p.downSince = now
		}
		if p.downSamples >= downRequired && now.Sub(p.downSince) >= downWindow {
			decision.Desired = current - 1
			decision.Reason = "scale-down signals hold: " + names(policy.ScaleDownSignals)
			p.resetDown()
			p.lastChange = now
			return decision, nil
		}
		decision.Reason = "waiting for scale-down stabilization"
		return decision, nil
	}
	p.resetUp()
	p.resetDown()
	decision.Reason = "no scale signal holds (scale-up failing: " + strings.Join(upFailing, ", ") + ")"
	return decision, nil
}

// evaluateVetoes reports vetoes without data and vetoes that hold.
func evaluateVetoes(signals []Signal, readings map[string]*float64) (missing, holding []string) {
	for _, signal := range signals {
		value := readings[signal.Name]
		if value == nil {
			missing = append(missing, signal.Name)
			continue
		}
		if signal.holds(*value) {
			holding = append(holding, signal.Name)
		}
	}
	return missing, holding
}

func names(signals []Signal) string {
	result := make([]string, len(signals))
	for i, signal := range signals {
		result[i] = signal.Name
	}
	return strings.Join(result, ", ")
}
