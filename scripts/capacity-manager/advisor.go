// Copyright Camunda Services GmbH and/or licensed to Camunda Services GmbH
// under one or more contributor license agreements. See the NOTICE file
// distributed with this work for additional information regarding copyright
// ownership. Licensed under the Apache License, Version 2.0; you may not use
// this file except in compliance with the License. You may obtain a copy of
// the License at http://www.apache.org/licenses/LICENSE-2.0

package capacity

import "math"

type PartitionRecommendation struct {
	Enabled         bool     `json:"enabled"`
	Current         int      `json:"current"`
	Recommended     int      `json:"recommended"`
	CeilingDetected bool     `json:"ceilingDetected"`
	PeakLoad        float64  `json:"peakLoad,omitempty"`
	Confidence      string   `json:"confidence"`
	Reason          string   `json:"reason"`
	Blockers        []string `json:"blockers,omitempty"`
	Irreversible    bool     `json:"irreversible"`
}

type PartitionAdvisor struct {
	highSamples int
}

func (a *PartitionAdvisor) Advise(policy PartitionAdvisorPolicy, topology Topology, peakLoad *float64) PartitionRecommendation {
	current := topology.PartitionCount()
	recommendation := PartitionRecommendation{
		Enabled: policy.Enabled, Current: current, Recommended: current,
		Confidence: "low", Reason: "advisor disabled", Irreversible: true,
	}
	if !policy.Enabled {
		return recommendation
	}
	if topology.PendingChange != nil || !topology.Healthy() {
		recommendation.Reason = "topology is not stable"
		recommendation.Blockers = []string{"topology-unhealthy"}
		a.highSamples = 0
		return recommendation
	}
	if !topology.PlacementBalanced() {
		recommendation.Reason = "partition placement is not balanced"
		recommendation.Blockers = []string{"placement-unbalanced"}
		a.highSamples = 0
		return recommendation
	}
	if peakLoad == nil {
		recommendation.Reason = "partition load is unavailable"
		recommendation.Blockers = []string{"partition-load-missing"}
		a.highSamples = 0
		return recommendation
	}
	recommendation.PeakLoad = *peakLoad
	if *peakLoad < policy.TargetLoad {
		recommendation.Confidence = "high"
		recommendation.Reason = "partition load has headroom"
		a.highSamples = 0
		return recommendation
	}
	a.highSamples++
	if a.highSamples < policy.CeilingSamples {
		recommendation.Confidence = "medium"
		recommendation.Reason = "collecting sustained partition saturation evidence"
		return recommendation
	}
	required := int(math.Ceil(float64(current) * *peakLoad / policy.TargetLoad))
	increment := max(1, int(math.Ceil(float64(current)*0.25)))
	recommended := min(current+increment, required, policy.MaxRecommendedPartitions)
	if recommended <= current {
		recommendation.Confidence = "high"
		recommendation.Reason = "partition recommendation is limited by configured maximum"
		recommendation.Blockers = []string{"maximum-partitions-reached"}
		return recommendation
	}
	recommendation.Recommended = recommended
	recommendation.CeilingDetected = true
	recommendation.Confidence = "high"
	recommendation.Reason = "sustained partition saturation with stable topology"
	return recommendation
}
