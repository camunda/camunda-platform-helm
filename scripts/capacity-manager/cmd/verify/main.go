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
	if err := waitFor(namespace, release, 2, 10*time.Minute); err != nil {
		fatalf("load-triggered scale up verification: %v", err)
	}
	if _, err := kubectl("delete", "deployment", release+"-capacity-load", "-n", namespace, "--ignore-not-found", "--wait=true", "--timeout=2m"); err != nil {
		fatalf("stop load generator: %v", err)
	}
	if err := waitFor(namespace, release, 1, 10*time.Minute); err != nil {
		fatalf("idle-triggered scale down verification: %v", err)
	}
	fmt.Println("capacity manager verified load-triggered 1 -> 2 and idle-triggered safe 2 -> 1 broker scaling")
}

func waitFor(namespace, release string, target int, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		status, err := getStatus(namespace, release)
		if err != nil {
			time.Sleep(5 * time.Second)
			continue
		}
		replicas, err := getReplicas(namespace, release)
		if err != nil {
			time.Sleep(5 * time.Second)
			continue
		}
		brokers, err := getBrokerCount(namespace, release)
		if err != nil {
			time.Sleep(5 * time.Second)
			continue
		}
		if target == 1 && replicas == 1 && brokers > 1 {
			return fmt.Errorf("StatefulSet contracted before topology evacuation: replicas=%d brokers=%d status=%#v", replicas, brokers, status)
		}
		if brokers == target && replicas == target && status.Phase == "stable" {
			return nil
		}
		time.Sleep(5 * time.Second)
	}
	return fmt.Errorf("timed out waiting for %d brokers", target)
}

func getReplicas(namespace, release string) (int, error) {
	output, err := kubectl("get", "statefulset", release+"-zeebe", "-n", namespace, "-o", "jsonpath={.spec.replicas}")
	if err != nil {
		return 0, err
	}
	return strconv.Atoi(strings.TrimSpace(string(output)))
}

func getBrokerCount(namespace, release string) (int, error) {
	path := fmt.Sprintf("/api/v1/namespaces/%s/services/http:%s-zeebe-gateway:9600/proxy/orchestration/actuator/cluster", namespace, release)
	output, err := kubectl("get", "--raw", path)
	if err != nil {
		return 0, err
	}
	var topology capacity.Topology
	if err := json.Unmarshal(output, &topology); err != nil {
		return 0, err
	}
	return topology.ActiveBrokerCount(), nil
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

func kubectl(args ...string) ([]byte, error) {
	if context := os.Getenv("KUBE_CONTEXT"); context != "" {
		args = append([]string{"--context", context}, args...)
	}
	command := exec.Command("kubectl", args...)
	output, err := command.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("kubectl %s: %w: %s", strings.Join(args, " "), err, output)
	}
	return output, nil
}

func fatalf(format string, args ...any) {
	fmt.Fprintf(os.Stderr, format+"\n", args...)
	os.Exit(1)
}
