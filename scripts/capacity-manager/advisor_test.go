// Copyright Camunda Services GmbH and/or licensed to Camunda Services GmbH
// under one or more contributor license agreements. See the NOTICE file
// distributed with this work for additional information regarding copyright
// ownership. Licensed under the Apache License, Version 2.0; you may not use
// this file except in compliance with the License. You may obtain a copy of
// the License at http://www.apache.org/licenses/LICENSE-2.0

package capacity

import "testing"

func TestPartitionAdvisor(t *testing.T) {
	policy := PartitionAdvisorPolicy{Enabled: true, MaxRecommendedPartitions: 8, TargetLoad: 0.7, LoadMetric: "load", LoadMetricType: "gauge", CeilingSamples: 3}
	topology := activeTopology(1)
	load := 0.95
	advisor := &PartitionAdvisor{}

	for sample := 1; sample <= 2; sample++ {
		recommendation := advisor.Advise(policy, topology, &load)
		if recommendation.CeilingDetected || recommendation.Recommended != 1 {
			t.Fatalf("recommended too early on sample %d: %#v", sample, recommendation)
		}
	}
	recommendation := advisor.Advise(policy, topology, &load)
	if !recommendation.CeilingDetected || recommendation.Recommended != 2 || recommendation.Confidence != "high" {
		t.Fatalf("unexpected recommendation: %#v", recommendation)
	}
}

func TestPartitionAdvisorBlocksUnstableTopology(t *testing.T) {
	policy := PartitionAdvisorPolicy{Enabled: true, MaxRecommendedPartitions: 8, TargetLoad: 0.7, LoadMetric: "load", LoadMetricType: "gauge", CeilingSamples: 1}
	topology := activeTopology(1)
	topology.PendingChange = &Change{ID: 1, Status: "IN_PROGRESS"}
	load := 0.95
	recommendation := (&PartitionAdvisor{}).Advise(policy, topology, &load)
	if recommendation.CeilingDetected || len(recommendation.Blockers) != 1 || recommendation.Blockers[0] != "topology-unhealthy" {
		t.Fatalf("unexpected recommendation: %#v", recommendation)
	}
}
