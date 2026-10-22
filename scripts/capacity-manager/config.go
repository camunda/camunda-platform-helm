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
	Mode                   string                 `json:"mode"`
	MinBrokers             int                    `json:"minBrokers"`
	MaxBrokers             int                    `json:"maxBrokers"`
	TargetBrokers          int                    `json:"targetBrokers,omitempty"`
	ScheduledMinimum       int                    `json:"scheduledMinimumBrokers,omitempty"`
	ScheduleStartsAt       string                 `json:"scheduleStartsAt,omitempty"`
	ScheduleEndsAt         string                 `json:"scheduleEndsAt,omitempty"`
	ScaleUpSignals         []Signal               `json:"scaleUpSignals,omitempty"`
	ScaleDownSignals       []Signal               `json:"scaleDownSignals,omitempty"`
	VetoSignals            []Signal               `json:"vetoSignals,omitempty"`
	ScaleUpSamples         int                    `json:"scaleUpSamples,omitempty"`
	ScaleDownSamples       int                    `json:"scaleDownSamples,omitempty"`
	ScaleUpStabilization   string                 `json:"scaleUpStabilization,omitempty"`
	ScaleDownStabilization string                 `json:"scaleDownStabilization,omitempty"`
	Cooldown               string                 `json:"cooldown,omitempty"`
	PartitionAdvisor       PartitionAdvisorPolicy `json:"partitionAdvisor,omitempty"`
}

// PartitionAdvisorPolicy configures the recommendation-only partition
// advisor. LoadQuery is a PromQL query for the busiest partition's load,
// in the same unit as TargetLoad.
type PartitionAdvisorPolicy struct {
	Enabled                  bool    `json:"enabled"`
	MaxRecommendedPartitions int     `json:"maxRecommendedPartitions"`
	TargetLoad               float64 `json:"targetLoad"`
	LoadQuery                string  `json:"loadQuery"`
	CeilingSamples           int     `json:"ceilingSamples"`
}

// Signal is a named PromQL condition. The query result is reduced to the
// maximum across all returned series, so "<=" means every series is at or
// below the threshold and ">=" means at least one series reached it. The
// placeholder $BROKERS is replaced with the current active broker count.
type Signal struct {
	Name      string  `json:"name"`
	Query     string  `json:"query"`
	Operator  string  `json:"operator"`
	Threshold float64 `json:"threshold"`
}

func (s Signal) holds(value float64) bool {
	if s.Operator == "<=" {
		return value <= s.Threshold
	}
	return value >= s.Threshold
}

func validateSignals(role string, signals []Signal, seen map[string]bool) error {
	for i, signal := range signals {
		if signal.Name == "" || signal.Query == "" {
			return fmt.Errorf("%s[%d] requires name and query", role, i)
		}
		if seen[signal.Name] {
			return fmt.Errorf("signal name %q is used more than once", signal.Name)
		}
		seen[signal.Name] = true
		if signal.Operator != ">=" && signal.Operator != "<=" {
			return fmt.Errorf("%s %q operator must be >= or <=", role, signal.Name)
		}
		if math.IsNaN(signal.Threshold) || math.IsInf(signal.Threshold, 0) {
			return fmt.Errorf("%s %q threshold must be finite", role, signal.Name)
		}
	}
	return nil
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
		if p.PartitionAdvisor.LoadQuery == "" {
			return fmt.Errorf("partitionAdvisor.loadQuery is required")
		}
		if p.PartitionAdvisor.CeilingSamples < 1 {
			return fmt.Errorf("partitionAdvisor.ceilingSamples must be at least 1")
		}
	}
	if p.ScaleUpSamples < 0 || p.ScaleDownSamples < 0 {
		return fmt.Errorf("scale sample counts cannot be negative")
	}
	seen := map[string]bool{}
	if err := validateSignals("scaleUpSignals", p.ScaleUpSignals, seen); err != nil {
		return err
	}
	if err := validateSignals("scaleDownSignals", p.ScaleDownSignals, seen); err != nil {
		return err
	}
	if err := validateSignals("vetoSignals", p.VetoSignals, seen); err != nil {
		return err
	}
	if p.Mode == "automatic" && (len(p.ScaleUpSignals) == 0 || len(p.ScaleDownSignals) == 0) {
		return fmt.Errorf("automatic mode requires scaleUpSignals and scaleDownSignals")
	}
	if _, err := parseStabilization(p.Cooldown); err != nil {
		return fmt.Errorf("parse cooldown: %w", err)
	}
	if (p.ScheduleStartsAt == "") != (p.ScheduleEndsAt == "") {
		return fmt.Errorf("scheduleStartsAt and scheduleEndsAt must be configured together")
	}
	if p.ScheduledMinimum != 0 && p.ScheduleStartsAt == "" {
		return fmt.Errorf("scheduledMinimumBrokers requires scheduleStartsAt and scheduleEndsAt")
	}
	if _, err := parseStabilization(p.ScaleUpStabilization); err != nil {
		return fmt.Errorf("parse scaleUpStabilization: %w", err)
	}
	if _, err := parseStabilization(p.ScaleDownStabilization); err != nil {
		return fmt.Errorf("parse scaleDownStabilization: %w", err)
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

// parseStabilization parses an optional Go duration; empty means no window.
func parseStabilization(value string) (time.Duration, error) {
	if value == "" {
		return 0, nil
	}
	d, err := time.ParseDuration(value)
	if err != nil {
		return 0, err
	}
	if d < 0 {
		return 0, fmt.Errorf("duration cannot be negative")
	}
	return d, nil
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
