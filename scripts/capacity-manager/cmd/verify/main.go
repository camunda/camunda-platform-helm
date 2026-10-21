// Copyright Camunda Services GmbH and/or licensed to Camunda Services GmbH
// under one or more contributor license agreements. See the NOTICE file
// distributed with this work for additional information regarding copyright
// ownership. Licensed under the Apache License, Version 2.0; you may not use
// this file except in compliance with the License. You may obtain a copy of
// the License at http://www.apache.org/licenses/LICENSE-2.0

// Command verify drives the capacity-manager end-to-end scenario against a
// live cluster and fails on the first unmet acceptance criterion. Each phase
// prints a PASS line with its measured evidence, and a background sampler
// prints CALIBRATION lines with every signal reading so a run doubles as
// threshold-tuning data.
//
// Environment:
//
//	TEST_NAMESPACE, RELEASE_NAME  required
//	KUBE_CONTEXT                  optional kubectl/helm context
//	CONTEXT_PATH                  orchestration context path (default /orchestration)
//	CHART_DIR                     chart used for Helm upgrade phases (default <repo>/charts/camunda-platform-8.11)
//	ES_STATEFULSET                Elasticsearch StatefulSet (default elasticsearch-master)
//	CAPACITY_VERIFY_PHASES        comma-separated phases to run (default: all, in order)
package main

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"sort"
	"strconv"
	"strings"
	"time"

	capacity "scripts/capacity-manager"
)

type env struct {
	namespace, release, contextPath, chartDir, esStatefulSet string
}

var phases = []string{
	"scale-up",          // AC1, AC7 (restart during scale-up)
	"relief",            // AC2
	"upgrade-keeps",     // AC8: helm upgrade while the manager owns replicas
	"handback-blocked",  // AC8: Helm cannot take replicas back at 2 brokers
	"prometheus-outage", // AC6: missing data holds
	"exporter-veto",     // AC6: exporter backlog blocks scale-down
	"scale-down",        // AC3, AC4, AC7 (restart during evacuation)
	"handback",          // AC8: contracted cluster returns to Helm and disables
}

func main() {
	e := env{
		namespace:     os.Getenv("TEST_NAMESPACE"),
		release:       os.Getenv("RELEASE_NAME"),
		contextPath:   envOr("CONTEXT_PATH", "/orchestration"),
		chartDir:      os.Getenv("CHART_DIR"),
		esStatefulSet: envOr("ES_STATEFULSET", "elasticsearch-master"),
	}
	if e.namespace == "" || e.release == "" {
		fatalf("TEST_NAMESPACE and RELEASE_NAME are required")
	}
	if e.chartDir == "" {
		root, err := exec.Command("git", "rev-parse", "--show-toplevel").Output()
		if err != nil {
			fatalf("locate repository root: %v", err)
		}
		e.chartDir = filepath.Join(strings.TrimSpace(string(root)), "charts", "camunda-platform-8.11")
	}
	selected := phases
	if raw := os.Getenv("CAPACITY_VERIFY_PHASES"); raw != "" {
		selected = strings.Split(raw, ",")
	}

	stop := make(chan struct{})
	go calibrate(e, stop)
	defer close(stop)

	start := time.Now()
	var up capacity.Status
	for _, phase := range selected {
		begin := time.Now()
		var err error
		switch strings.TrimSpace(phase) {
		case "scale-up":
			up, err = scaleUp(e)
		case "relief":
			err = relief(e, up)
		case "upgrade-keeps":
			err = upgradeKeeps(e)
		case "handback-blocked":
			err = handbackBlocked(e)
		case "prometheus-outage":
			err = prometheusOutage(e)
		case "exporter-veto":
			err = exporterVeto(e)
		case "scale-down":
			err = scaleDown(e)
		case "handback":
			err = handback(e)
		default:
			err = fmt.Errorf("unknown phase %q (known: %s)", phase, strings.Join(phases, ","))
		}
		if err != nil {
			fatalf("FAIL %s after %s: %v", phase, time.Since(begin).Round(time.Second), err)
		}
		fmt.Printf("PASS %s in %s\n", phase, time.Since(begin).Round(time.Second))
	}
	fmt.Printf("capacity manager verified %s in %s\n", strings.Join(selected, ","), time.Since(start).Round(time.Second))
}

// scaleUp waits for the load to trigger 1 -> 2, restarting the manager while
// the new broker is joining, and requires the decision to cite saturation.
func scaleUp(e env) (capacity.Status, error) {
	restarted := false
	status, err := waitFor(e, 25*time.Minute, func(s capacity.Status, brokers, replicas int) bool {
		if !restarted && (s.Phase == "joining-broker" || s.Phase == "provisioning-broker" || replicas > brokers) {
			fmt.Printf("restarting manager during scale-up (phase %s, brokers %d, replicas %d)\n", s.Phase, brokers, replicas)
			restartManager(e)
			restarted = true
		}
		return brokers == 2 && replicas == 2 && s.Phase == "stable"
	})
	if err != nil {
		return status, err
	}
	if !restarted {
		return status, fmt.Errorf("never observed the join in progress, so the restart-during-scale-up check did not run")
	}
	if err := requireChange(status.LastChange, 1, 2, "backpressure", "broker-cpu"); err != nil {
		return status, err
	}
	fmt.Printf("scale-up evidence: %q signals=%v\n", status.LastChange.Reason, sortedSignals(status.LastChange.Signals))
	return status, nil
}

// relief requires the second broker to lower the drop ratio that triggered it.
func relief(e env, up capacity.Status) error {
	if up.LastChange == nil {
		var err error
		if up, err = getStatus(e); err != nil || up.LastChange == nil {
			return fmt.Errorf("no scale-up evidence to compare against")
		}
	}
	trigger := up.LastChange.Signals["backpressure"]
	_, err := waitFor(e, 10*time.Minute, func(s capacity.Status, _, _ int) bool {
		value, ok := s.Signals["backpressure"]
		return ok && value < trigger
	})
	return err
}

// upgradeKeeps runs helm upgrade with ownership kept and requires the live
// replica count to survive it.
func upgradeKeeps(e env) error {
	before, err := getReplicas(e)
	if err != nil {
		return err
	}
	if out, err := helm(e, "upgrade", e.release, e.chartDir, "--reuse-values"); err != nil {
		return fmt.Errorf("helm upgrade with ownership kept: %v: %s", err, tail(out))
	}
	time.Sleep(20 * time.Second)
	after, err := getReplicas(e)
	if err != nil {
		return err
	}
	if after != before {
		return fmt.Errorf("helm upgrade changed replicas %d -> %d while the manager owned them", before, after)
	}
	fmt.Printf("helm upgrade kept %d replicas\n", after)
	return nil
}

// handbackBlocked requires the chart to refuse giving replicas back to Helm
// while the manager holds the cluster at two brokers.
func handbackBlocked(e env) error {
	out, err := helm(e, "upgrade", e.release, e.chartDir, "--reuse-values", "--dry-run=server",
		"--set", "capacityManager.replicaOwnership=")
	if err == nil {
		return fmt.Errorf("handback at two brokers rendered; the fail-closed guard did not fire")
	}
	if !strings.Contains(string(out), "Helm cannot take back the orchestration replica count") {
		return fmt.Errorf("handback failed for an unexpected reason: %s", tail(out))
	}
	fmt.Println("handback at two brokers refused by the chart guard")
	return nil
}

// prometheusOutage stops Prometheus and requires the manager to hold with low
// confidence and no capacity change, then restores it.
func prometheusOutage(e env) error {
	name := e.release + "-capacity-prometheus"
	if _, err := kubectl(e, "scale", "deployment", name, "--replicas=0"); err != nil {
		return err
	}
	defer func() {
		_, _ = kubectl(e, "scale", "deployment", name, "--replicas=1")
		_, _ = kubectl(e, "rollout", "status", "deployment", name, "--timeout=5m")
	}()
	if _, err := kubectl(e, "wait", "--for=delete", "pod", "-l", "app.kubernetes.io/component=capacity-prometheus", "--timeout=2m"); err != nil {
		return err
	}
	brokersBefore, _ := getBrokerCount(e)
	held := 0
	_, err := waitFor(e, 5*time.Minute, func(s capacity.Status, brokers, _ int) bool {
		if brokers != brokersBefore {
			return false
		}
		if s.Confidence == "low" && strings.Contains(s.Reason, "unavailable") {
			held++
		} else {
			held = 0
		}
		return held >= 3
	})
	if err != nil {
		return err
	}
	if after, _ := getBrokerCount(e); after != brokersBefore {
		return fmt.Errorf("capacity changed %d -> %d without metrics", brokersBefore, after)
	}
	fmt.Printf("held at %d brokers with low confidence while Prometheus was down\n", brokersBefore)
	return nil
}

// exporterVeto makes Elasticsearch refuse writes (it stays up, so brokers
// keep processing and the exporter falls behind), removes the load, and
// requires the exporter-backlog veto to hold two brokers until writes are
// allowed again. Stopping Elasticsearch instead is not a lag test on 8.11:
// orchestration pods cannot become Ready without their secondary store, so
// processing stops and the backlog never grows.
func exporterVeto(e env) error {
	block := func(enabled bool) error {
		value := "null"
		if enabled {
			value = "true"
		}
		_, err := kubectl(e, "exec", e.esStatefulSet+"-0", "--", "curl", "-fsS", "-XPUT", "localhost:9200/_all/_settings",
			"-H", "Content-Type: application/json", "-d", `{"index.blocks.write":`+value+`}`)
		return err
	}
	if err := block(true); err != nil {
		return fmt.Errorf("block Elasticsearch writes: %w", err)
	}
	restored := false
	restore := func() {
		if !restored {
			if err := block(false); err != nil {
				fmt.Printf("unblock Elasticsearch writes: %v\n", err)
			}
			restored = true
		}
	}
	defer restore()
	if _, err := waitFor(e, 15*time.Minute, func(s capacity.Status, _, _ int) bool {
		return strings.Contains(s.Reason, "vetoed by exporter-backlog")
	}); err != nil {
		return fmt.Errorf("exporter backlog never vetoed: %w", err)
	}
	stopLoad(e)
	vetoed := 0
	if _, err := waitFor(e, 5*time.Minute, func(s capacity.Status, brokers, _ int) bool {
		if brokers != 2 {
			vetoed = -1000
			return true
		}
		if strings.Contains(s.Reason, "vetoed by exporter-backlog") {
			vetoed++
		}
		return vetoed >= 6
	}); err != nil || vetoed < 0 {
		return fmt.Errorf("scale-down was not held by the exporter-backlog veto (vetoed=%d, err=%v)", vetoed, err)
	}
	fmt.Println("exporter-backlog veto held two brokers after load stopped")
	restore()
	return nil
}

// scaleDown waits for idle 2 -> 1, restarting the manager mid-evacuation, and
// requires the decision to cite the scale-down signals.
func scaleDown(e env) error {
	stopLoad(e)
	restarted := false
	status, err := waitFor(e, 25*time.Minute, func(s capacity.Status, brokers, replicas int) bool {
		// Restart either while the topology change runs or in the window
		// after it completed but before the pod was removed; both must
		// resume to a contracted, consistent cluster.
		if !restarted && (s.Phase == "evacuating-broker" || (brokers == 1 && replicas == 2)) {
			fmt.Printf("restarting manager during scale-down (phase %s, brokers %d, replicas %d): %s\n", s.Phase, brokers, replicas, s.Reason)
			restartManager(e)
			restarted = true
		}
		return brokers == 1 && replicas == 1 && s.Phase == "stable"
	})
	if err != nil {
		return err
	}
	if !restarted {
		return fmt.Errorf("never observed the evacuation in progress, so the restart-during-scale-down check did not run")
	}
	return requireChange(status.LastChange, 2, 1, "no-backpressure", "fits-on-fewer-brokers")
}

// handback returns replica ownership to Helm and disables the component once
// the cluster is contracted, and requires Helm to own replicas=1 afterwards.
func handback(e env) error {
	target, completed, err := annotations(e)
	if err != nil {
		return err
	}
	if target != "1" || completed != "1" {
		return fmt.Errorf("cluster not contracted for handback: target=%s completed=%s", target, completed)
	}
	if out, err := helm(e, "upgrade", e.release, e.chartDir, "--reuse-values",
		"--set", "capacityManager.replicaOwnership=", "--set", "capacityManager.enabled=false"); err != nil {
		return fmt.Errorf("handback upgrade: %v: %s", err, tail(out))
	}
	replicas, err := getReplicas(e)
	if err != nil {
		return err
	}
	if replicas != 1 {
		return fmt.Errorf("Helm-owned replicas = %d, want 1", replicas)
	}
	if _, err := kubectl(e, "get", "deployment", e.release+"-capacity-manager"); err == nil {
		return fmt.Errorf("capacity manager still deployed after disable")
	}
	fmt.Println("ownership returned to Helm at one broker and the component was disabled")
	return nil
}

// waitFor polls broker count, replicas and manager status until done returns
// true, failing immediately if pods are ever fewer than active brokers.
func waitFor(e env, timeout time.Duration, done func(capacity.Status, int, int) bool) (capacity.Status, error) {
	deadline := time.Now().Add(timeout)
	var last capacity.Status
	for time.Now().Before(deadline) {
		status, err := getStatus(e)
		brokers, berr := getBrokerCount(e)
		replicas, rerr := getReplicas(e)
		if err == nil && berr == nil && rerr == nil {
			last = status
			if replicas < brokers {
				return status, fmt.Errorf("SAFETY: StatefulSet has %d pods for %d active brokers; status %s", replicas, brokers, status.JSON())
			}
			if done(status, brokers, replicas) {
				return status, nil
			}
		}
		time.Sleep(2 * time.Second)
	}
	return last, fmt.Errorf("timed out after %s; last status %s", timeout, last.JSON())
}

func requireChange(change *capacity.CapacityChange, from, to int, signals ...string) error {
	if change == nil {
		return fmt.Errorf("status has no lastChange")
	}
	if change.From != from || change.To != to {
		return fmt.Errorf("last change was %d -> %d, want %d -> %d", change.From, change.To, from, to)
	}
	for _, signal := range signals {
		if !strings.Contains(change.Reason, signal) {
			return fmt.Errorf("reason %q does not name signal %q", change.Reason, signal)
		}
		if _, ok := change.Signals[signal]; !ok {
			return fmt.Errorf("signal %q has no recorded reading: %v", signal, change.Signals)
		}
	}
	return nil
}

// calibrate prints every signal reading every 15 seconds.
func calibrate(e env, stop <-chan struct{}) {
	ticker := time.NewTicker(15 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-stop:
			return
		case <-ticker.C:
			status, err := getStatus(e)
			if err != nil {
				continue
			}
			brokers, _ := getBrokerCount(e)
			fmt.Printf("CALIBRATION t=%s brokers=%d phase=%s confidence=%s %s reason=%q\n",
				time.Now().UTC().Format(time.RFC3339), brokers, status.Phase, status.Confidence, sortedSignals(status.Signals), status.Reason)
		}
	}
}

func sortedSignals(signals map[string]float64) string {
	names := make([]string, 0, len(signals))
	for name := range signals {
		names = append(names, name)
	}
	sort.Strings(names)
	parts := make([]string, 0, len(names))
	for _, name := range names {
		parts = append(parts, fmt.Sprintf("%s=%.4g", name, signals[name]))
	}
	return strings.Join(parts, " ")
}

func restartManager(e env) {
	if _, err := kubectl(e, "delete", "pod", "-l", "app.kubernetes.io/component=capacity-manager", "--wait=false"); err != nil {
		fmt.Printf("restart manager: %v\n", err)
	}
}

func stopLoad(e env) {
	_, _ = kubectl(e, "delete", "deployment", e.release+"-capacity-load", "--ignore-not-found", "--wait=true", "--timeout=2m")
}

func annotations(e env) (string, string, error) {
	out, err := kubectl(e, "get", "statefulset", e.release+"-zeebe", "-o", "json")
	if err != nil {
		return "", "", err
	}
	var sts struct {
		Metadata struct {
			Annotations map[string]string `json:"annotations"`
		} `json:"metadata"`
	}
	if err := json.Unmarshal(out, &sts); err != nil {
		return "", "", err
	}
	a := sts.Metadata.Annotations
	return a["capacity-manager.camunda.io/target-brokers"], a["capacity-manager.camunda.io/completed-brokers"], nil
}

func getReplicas(e env) (int, error) {
	output, err := kubectl(e, "get", "statefulset", e.release+"-zeebe", "-o", "jsonpath={.spec.replicas}")
	if err != nil {
		return 0, err
	}
	return strconv.Atoi(strings.TrimSpace(string(output)))
}

func getBrokerCount(e env) (int, error) {
	path := fmt.Sprintf("/api/v1/namespaces/%s/services/http:%s-zeebe-gateway:9600/proxy%s/actuator/cluster", e.namespace, e.release, e.contextPath)
	output, err := kubectl(e, "get", "--raw", path)
	if err != nil {
		return 0, err
	}
	var topology capacity.Topology
	if err := json.Unmarshal(output, &topology); err != nil {
		return 0, err
	}
	return topology.ActiveBrokerCount(), nil
}

func getStatus(e env) (capacity.Status, error) {
	path := fmt.Sprintf("/api/v1/namespaces/%s/services/http:%s-capacity-manager:8080/proxy/status", e.namespace, e.release)
	output, err := kubectl(e, "get", "--raw", path)
	if err != nil {
		return capacity.Status{}, err
	}
	var status capacity.Status
	if err := json.Unmarshal(output, &status); err != nil {
		return capacity.Status{}, err
	}
	return status, nil
}

func kubectl(e env, args ...string) ([]byte, error) {
	// Namespace goes before the arguments so it never lands after "--"
	// (kubectl exec passes everything after "--" to the container).
	if !slices.Contains(args, "--raw") {
		args = append([]string{"-n", e.namespace}, args...)
	}
	if context := os.Getenv("KUBE_CONTEXT"); context != "" {
		args = append([]string{"--context", context}, args...)
	}
	output, err := exec.Command("kubectl", args...).CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("kubectl %s: %w: %s", strings.Join(args, " "), err, tail(output))
	}
	return output, nil
}

func helm(e env, args ...string) ([]byte, error) {
	if _, err := os.Stat(filepath.Join(e.chartDir, "charts")); err != nil {
		if out, err := exec.Command("helm", "dependency", "build", e.chartDir).CombinedOutput(); err != nil {
			return out, fmt.Errorf("helm dependency build: %w", err)
		}
	}
	args = append(args, "-n", e.namespace)
	if context := os.Getenv("KUBE_CONTEXT"); context != "" {
		args = append(args, "--kube-context", context)
	}
	return exec.Command("helm", args...).CombinedOutput()
}

func tail(out []byte) string {
	s := strings.TrimSpace(string(out))
	if len(s) > 600 {
		s = s[len(s)-600:]
	}
	return s
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func fatalf(format string, args ...any) {
	fmt.Fprintf(os.Stderr, format+"\n", args...)
	os.Exit(1)
}
