// Copyright Camunda Services GmbH and/or licensed to Camunda Services GmbH
// under one or more contributor license agreements. See the NOTICE file
// distributed with this work for additional information regarding copyright
// ownership. Licensed under the Apache License, Version 2.0; you may not use
// this file except in compliance with the License. You may obtain a copy of
// the License at http://www.apache.org/licenses/LICENSE-2.0

package capacity

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
)

type BrokerID string

func (id *BrokerID) UnmarshalJSON(data []byte) error {
	var text string
	if len(data) > 0 && data[0] == '"' {
		if err := json.Unmarshal(data, &text); err != nil {
			return err
		}
	} else {
		var number json.Number
		if err := json.Unmarshal(data, &number); err != nil {
			return err
		}
		text = number.String()
	}
	*id = BrokerID(text)
	return nil
}

type Partition struct {
	ID       int    `json:"id"`
	State    string `json:"state"`
	Priority int    `json:"priority"`
}

type Broker struct {
	ID         BrokerID    `json:"id"`
	State      string      `json:"state"`
	Partitions []Partition `json:"partitions"`
}

type Change struct {
	ID     int64  `json:"id"`
	Status string `json:"status"`
}

type Topology struct {
	Brokers       []Broker `json:"brokers"`
	PendingChange *Change  `json:"pendingChange"`
	LastChange    *Change  `json:"lastChange"`
}

func (t Topology) PartitionCount() int {
	partitions := map[int]struct{}{}
	for _, broker := range t.Brokers {
		if broker.State == "LEFT" {
			continue
		}
		for _, partition := range broker.Partitions {
			partitions[partition.ID] = struct{}{}
		}
	}
	return len(partitions)
}

func (t Topology) PlacementBalanced() bool {
	if len(t.Brokers) < 2 {
		return true
	}
	min, max := -1, 0
	for _, broker := range t.Brokers {
		if broker.State == "LEFT" {
			continue
		}
		count := len(broker.Partitions)
		if min == -1 || count < min {
			min = count
		}
		if count > max {
			max = count
		}
	}
	return max-min <= 1
}

func (t Topology) ActiveBrokerCount() int {
	count := 0
	for _, broker := range t.Brokers {
		if broker.State != "LEFT" {
			count++
		}
	}
	return count
}

func (t Topology) Healthy() bool {
	if t.PendingChange != nil {
		return false
	}
	if t.ActiveBrokerCount() == 0 {
		return false
	}
	for _, broker := range t.Brokers {
		if broker.State == "LEFT" {
			continue
		}
		if broker.State != "ACTIVE" {
			return false
		}
		for _, partition := range broker.Partitions {
			if partition.State != "ACTIVE" {
				return false
			}
		}
	}
	return true
}

type ScalePlan struct {
	ChangeID         int64    `json:"changeId"`
	PlannedChanges   []any    `json:"plannedChanges"`
	ExpectedTopology []Broker `json:"expectedTopology"`
}

type ZeebeClient struct {
	BaseURL string
	Client  *http.Client
}

func (c ZeebeClient) Topology(ctx context.Context) (Topology, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, c.BaseURL+"/actuator/cluster", nil)
	if err != nil {
		return Topology{}, err
	}
	response, err := c.Client.Do(request)
	if err != nil {
		return Topology{}, err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return Topology{}, fmt.Errorf("cluster topology returned %s", response.Status)
	}
	var topology Topology
	if err := json.NewDecoder(response.Body).Decode(&topology); err != nil {
		return Topology{}, err
	}
	return topology, nil
}

func (c ZeebeClient) Scale(ctx context.Context, brokers int, dryRun bool) (ScalePlan, error) {
	payload, err := json.Marshal(map[string]any{"brokers": map[string]int{"count": brokers}})
	if err != nil {
		return ScalePlan{}, err
	}
	endpoint := c.BaseURL + "/actuator/cluster?dryRun=" + strconv.FormatBool(dryRun)
	request, err := http.NewRequestWithContext(ctx, http.MethodPatch, endpoint, bytes.NewReader(payload))
	if err != nil {
		return ScalePlan{}, err
	}
	request.Header.Set("Content-Type", "application/json")
	response, err := c.Client.Do(request)
	if err != nil {
		return ScalePlan{}, err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusAccepted {
		var apiError struct {
			Message string `json:"message"`
		}
		_ = json.NewDecoder(response.Body).Decode(&apiError)
		return ScalePlan{}, fmt.Errorf("cluster scale returned %s: %s", response.Status, apiError.Message)
	}
	var plan ScalePlan
	if err := json.NewDecoder(response.Body).Decode(&plan); err != nil {
		return ScalePlan{}, err
	}
	if len(plan.ExpectedTopology) != brokers {
		return ScalePlan{}, fmt.Errorf("expected topology has %d brokers, wanted %d", len(plan.ExpectedTopology), brokers)
	}
	seen := make(map[string]bool, brokers)
	for _, broker := range plan.ExpectedTopology {
		seen[string(broker.ID)] = true
	}
	for id := range brokers {
		if !seen[strconv.Itoa(id)] {
			return ScalePlan{}, fmt.Errorf("expected topology does not contain broker %d", id)
		}
	}
	return plan, nil
}
