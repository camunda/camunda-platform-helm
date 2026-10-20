// Copyright Camunda Services GmbH and/or licensed to Camunda Services GmbH
// under one or more contributor license agreements. See the NOTICE file
// distributed with this work for additional information regarding copyright
// ownership. Licensed under the Apache License, Version 2.0; you may not use
// this file except in compliance with the License. You may obtain a copy of
// the License at http://www.apache.org/licenses/LICENSE-2.0

package main

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"

	capacity "scripts/capacity-manager"
)

func main() {
	namespace := os.Getenv("TEST_NAMESPACE")
	release := os.Getenv("RELEASE_NAME")
	if namespace == "" || release == "" {
		fatalf("TEST_NAMESPACE and RELEASE_NAME are required")
	}
	beforeTopology, err := getTopology(namespace, release)
	if err != nil {
		fatalf("get initial topology: %v", err)
	}
	beforeReplicas, err := getReplicas(namespace, release)
	if err != nil {
		fatalf("get initial replicas: %v", err)
	}
	beforeChange := changeID(beforeTopology.LastChange)
	deadline := time.Now().Add(10 * time.Minute)
	var recommendation capacity.PartitionRecommendation
	for time.Now().Before(deadline) {
		status, err := getStatus(namespace, release)
		if err == nil && status.PartitionAdvisor.CeilingDetected && status.PartitionAdvisor.Recommended > status.PartitionAdvisor.Current {
			recommendation = status.PartitionAdvisor
			break
		}
		time.Sleep(5 * time.Second)
	}
	if !recommendation.CeilingDetected {
		fatalf("timed out waiting for partition recommendation")
	}
	afterTopology, err := getTopology(namespace, release)
	if err != nil {
		fatalf("get final topology: %v", err)
	}
	afterReplicas, err := getReplicas(namespace, release)
	if err != nil {
		fatalf("get final replicas: %v", err)
	}
	if beforeReplicas != afterReplicas || beforeTopology.ActiveBrokerCount() != afterTopology.ActiveBrokerCount() || beforeTopology.PartitionCount() != afterTopology.PartitionCount() || beforeChange != changeID(afterTopology.LastChange) || afterTopology.PendingChange != nil {
		fatalf("advisor mutated cluster: replicas %d->%d brokers %d->%d partitions %d->%d change %d->%d", beforeReplicas, afterReplicas, beforeTopology.ActiveBrokerCount(), afterTopology.ActiveBrokerCount(), beforeTopology.PartitionCount(), afterTopology.PartitionCount(), beforeChange, changeID(afterTopology.LastChange))
	}
	fmt.Printf("partition advisor recommended %d -> %d with %s confidence and did not mutate the cluster\n", recommendation.Current, recommendation.Recommended, recommendation.Confidence)
}

func getStatus(namespace, release string) (capacity.Status, error) {
	path := fmt.Sprintf("/api/v1/namespaces/%s/services/http:%s-capacity-manager:8080/proxy/status", namespace, release)
	output, err := kubectl("get", "--raw", path)
	if err != nil {
		return capacity.Status{}, err
	}
	var status capacity.Status
	if err := json.Unmarshal(output, &status); err != nil {
		return capacity.Status{}, err
	}
	return status, nil
}

func getTopology(namespace, release string) (capacity.Topology, error) {
	path := fmt.Sprintf("/api/v1/namespaces/%s/services/http:%s-zeebe-gateway:9600/proxy/orchestration/actuator/cluster", namespace, release)
	output, err := kubectl("get", "--raw", path)
	if err != nil {
		return capacity.Topology{}, err
	}
	var topology capacity.Topology
	if err := json.Unmarshal(output, &topology); err != nil {
		return capacity.Topology{}, err
	}
	return topology, nil
}

func getReplicas(namespace, release string) (int, error) {
	output, err := kubectl("get", "statefulset", release+"-zeebe", "-n", namespace, "-o", "jsonpath={.spec.replicas}")
	if err != nil {
		return 0, err
	}
	return strconv.Atoi(strings.TrimSpace(string(output)))
}

func changeID(change *capacity.Change) int64 {
	if change == nil {
		return 0
	}
	return change.ID
}

func kubectl(args ...string) ([]byte, error) {
	if context := os.Getenv("KUBE_CONTEXT"); context != "" {
		args = append([]string{"--context", context}, args...)
	}
	output, err := exec.Command("kubectl", args...).CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("kubectl %s: %w: %s", strings.Join(args, " "), err, output)
	}
	return output, nil
}

func fatalf(format string, args ...any) {
	fmt.Fprintf(os.Stderr, format+"\n", args...)
	os.Exit(1)
}
