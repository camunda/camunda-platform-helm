// Copyright Camunda Services GmbH and/or licensed to Camunda Services GmbH
// under one or more contributor license agreements. See the NOTICE file
// distributed with this work for additional information regarding copyright
// ownership. Licensed under the Apache License, Version 2.0; you may not use
// this file except in compliance with the License. You may obtain a copy of
// the License at http://www.apache.org/licenses/LICENSE-2.0

package capacity

import (
	"encoding/json"
	"fmt"
	"math"
	"os"
	"time"
)

type Policy struct {
	BrokerAutoscalingEnabled bool                   `json:"brokerAutoscalingEnabled"`
	Mode                     string                 `json:"mode"`
	MinBrokers               int                    `json:"minBrokers"`
	MaxBrokers               int                    `json:"maxBrokers"`
	TargetBrokers            int                    `json:"targetBrokers,omitempty"`
	ScheduledMinimum         int                    `json:"scheduledMinimumBrokers,omitempty"`
	ScheduleStartsAt         string                 `json:"scheduleStartsAt,omitempty"`
	ScheduleEndsAt           string                 `json:"scheduleEndsAt,omitempty"`
	PressureQuery            string                 `json:"pressureQuery,omitempty"`
	ScaleUpThreshold         float64                `json:"scaleUpThreshold,omitempty"`
	ScaleDownThreshold       float64                `json:"scaleDownThreshold,omitempty"`
	ScaleUpSamples           int                    `json:"scaleUpSamples,omitempty"`
	ScaleDownSamples         int                    `json:"scaleDownSamples,omitempty"`
	ScaleUpStabilization     string                 `json:"scaleUpStabilization,omitempty"`
	ScaleDownStabilization   string                 `json:"scaleDownStabilization,omitempty"`
	PartitionAdvisor         PartitionAdvisorPolicy `json:"partitionAdvisor,omitempty"`
}

type PartitionAdvisorPolicy struct {
	Enabled                  bool    `json:"enabled"`
	MaxRecommendedPartitions int     `json:"maxRecommendedPartitions"`
	TargetLoad               float64 `json:"targetLoad"`
	LoadMetric               string  `json:"loadMetric"`
	LoadMetricType           string  `json:"loadMetricType"`
	CeilingSamples           int     `json:"ceilingSamples"`
}

func (p Policy) validate() error {
	if p.MinBrokers < 1 {
		return fmt.Errorf("minBrokers must be at least 1")
	}
	if p.MaxBrokers < p.MinBrokers {
		return fmt.Errorf("maxBrokers must be greater than or equal to minBrokers")
	}
	if p.TargetBrokers != 0 && (p.TargetBrokers < p.MinBrokers || p.TargetBrokers > p.MaxBrokers) {
		return fmt.Errorf("targetBrokers must be between minBrokers and maxBrokers")
	}
	if p.ScheduledMinimum != 0 && (p.ScheduledMinimum < p.MinBrokers || p.ScheduledMinimum > p.MaxBrokers) {
		return fmt.Errorf("scheduledMinimumBrokers must be between minBrokers and maxBrokers")
	}
	if p.Mode != "recommend" && p.Mode != "scheduled" && p.Mode != "automatic" {
		return fmt.Errorf("mode must be recommend, scheduled, or automatic")
	}
	if p.PartitionAdvisor.Enabled {
		if p.PartitionAdvisor.MaxRecommendedPartitions < 1 {
			return fmt.Errorf("partitionAdvisor.maxRecommendedPartitions must be at least 1")
		}
		if p.PartitionAdvisor.TargetLoad <= 0 {
			return fmt.Errorf("partitionAdvisor.targetLoad must be greater than 0")
		}
		if p.PartitionAdvisor.LoadMetric == "" {
			return fmt.Errorf("partitionAdvisor.loadMetric is required")
		}
		if p.PartitionAdvisor.LoadMetricType != "gauge" && p.PartitionAdvisor.LoadMetricType != "counter-rate" {
			return fmt.Errorf("partitionAdvisor.loadMetricType must be gauge or counter-rate")
		}
		if p.PartitionAdvisor.CeilingSamples < 1 {
			return fmt.Errorf("partitionAdvisor.ceilingSamples must be at least 1")
		}
	}
	if p.ScaleUpSamples < 0 || p.ScaleDownSamples < 0 {
		return fmt.Errorf("scale sample counts cannot be negative")
	}
	if math.IsNaN(p.ScaleUpThreshold) || math.IsNaN(p.ScaleDownThreshold) || math.IsInf(p.ScaleUpThreshold, 0) || math.IsInf(p.ScaleDownThreshold, 0) {
		return fmt.Errorf("scale thresholds must be finite")
	}
	if p.Mode == "automatic" && p.ScaleDownThreshold >= p.ScaleUpThreshold {
		return fmt.Errorf("scaleDownThreshold must be lower than scaleUpThreshold")
	}
	if (p.ScheduleStartsAt == "") != (p.ScheduleEndsAt == "") {
		return fmt.Errorf("scheduleStartsAt and scheduleEndsAt must be configured together")
	}
	if p.ScheduleStartsAt != "" {
		start, err := time.Parse(time.RFC3339, p.ScheduleStartsAt)
		if err != nil {
			return fmt.Errorf("parse scheduleStartsAt: %w", err)
		}
		end, err := time.Parse(time.RFC3339, p.ScheduleEndsAt)
		if err != nil {
			return fmt.Errorf("parse scheduleEndsAt: %w", err)
		}
		if !end.After(start) {
			return fmt.Errorf("scheduleEndsAt must be after scheduleStartsAt")
		}
	}
	return nil
}

func (p Policy) scheduledMinimum(now time.Time) (int, error) {
	if p.ScheduledMinimum == 0 {
		return 0, nil
	}
	start, err := time.Parse(time.RFC3339, p.ScheduleStartsAt)
	if err != nil {
		return 0, fmt.Errorf("parse scheduleStartsAt: %w", err)
	}
	end, err := time.Parse(time.RFC3339, p.ScheduleEndsAt)
	if err != nil {
		return 0, fmt.Errorf("parse scheduleEndsAt: %w", err)
	}
	if !now.Before(start) && now.Before(end) {
		return p.ScheduledMinimum, nil
	}
	return 0, nil
}

type FilePolicySource struct {
	Path string
}

func (s FilePolicySource) Policy() (Policy, error) {
	data, err := os.ReadFile(s.Path)
	if err != nil {
		return Policy{}, fmt.Errorf("read policy: %w", err)
	}
	var policy Policy
	if err := json.Unmarshal(data, &policy); err != nil {
		return Policy{}, fmt.Errorf("decode policy: %w", err)
	}
	if err := policy.validate(); err != nil {
		return Policy{}, err
	}
	return policy, nil
}
