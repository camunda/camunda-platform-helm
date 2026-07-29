// Copyright Camunda Services GmbH and/or licensed to Camunda Services GmbH
// under one or more contributor license agreements. See the NOTICE file
// distributed with this work for additional information regarding copyright
// ownership. Licensed under the Apache License, Version 2.0; you may not use
// this file except in compliance with the License. You may obtain a copy of
// the License at http://www.apache.org/licenses/LICENSE-2.0

package main

import (
	"context"
	"flag"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	capacity "scripts/capacity-manager"
)

func main() {
	var policyPath, namespace, statefulSet, zeebeURL, prometheusURL, listenAddress string
	var interval, operationWait time.Duration
	flag.StringVar(&policyPath, "policy", "/etc/capacity-manager/policy.json", "path to the capacity policy")
	flag.StringVar(&namespace, "namespace", os.Getenv("POD_NAMESPACE"), "Kubernetes namespace")
	flag.StringVar(&statefulSet, "statefulset", "", "orchestration StatefulSet name")
	flag.StringVar(&zeebeURL, "zeebe-url", "", "orchestration management URL")
	flag.StringVar(&prometheusURL, "prometheus-url", "", "Prometheus base URL")
	flag.StringVar(&listenAddress, "listen", ":8080", "health and status listen address")
	flag.DurationVar(&interval, "interval", 30*time.Second, "reconciliation interval")
	flag.DurationVar(&operationWait, "operation-timeout", 20*time.Minute, "topology operation timeout")
	flag.Parse()

	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	if interval <= 0 || operationWait <= 0 {
		logger.Error("interval and operation-timeout must be positive")
		os.Exit(2)
	}
	if namespace == "" || statefulSet == "" || zeebeURL == "" {
		logger.Error("namespace, statefulset and zeebe-url are required")
		os.Exit(2)
	}
	httpClient := &http.Client{Timeout: 15 * time.Second}
	kubernetes, err := capacity.NewInClusterKubernetesClient(namespace, statefulSet, httpClient)
	if err != nil {
		logger.Error("initialize Kubernetes client", "error", err)
		os.Exit(1)
	}
	var pressure capacity.PressureSource = capacity.PrometheusClient{BaseURL: prometheusURL, Client: httpClient}
	var advisorMetrics capacity.GaugeSource = capacity.PrometheusClient{BaseURL: prometheusURL, Client: httpClient}
	if prometheusURL != "" && strings.HasSuffix(prometheusURL, "/actuator/prometheus") {
		direct := &capacity.DirectMetricsClient{URL: prometheusURL, Client: httpClient}
		pressure = direct
		advisorMetrics = direct
	}
	manager := &capacity.Manager{
		Policies:       capacity.FilePolicySource{Path: policyPath},
		Workload:       kubernetes,
		Cluster:        capacity.ZeebeClient{BaseURL: zeebeURL, Client: httpClient},
		Pressure:       pressure,
		AdvisorMetrics: advisorMetrics,
		Planner:        &capacity.Planner{},
		Advisor:        &capacity.PartitionAdvisor{},
		Logger:         logger,
		OperationWait:  operationWait,
		OperationPoll:  5 * time.Second,
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", func(response http.ResponseWriter, _ *http.Request) {
		response.WriteHeader(http.StatusOK)
		_, _ = response.Write([]byte("ok"))
	})
	mux.HandleFunc("/status", func(response http.ResponseWriter, _ *http.Request) {
		response.Header().Set("Content-Type", "application/json")
		_, _ = response.Write(manager.Status().JSON())
	})
	mux.HandleFunc("/metrics", func(response http.ResponseWriter, _ *http.Request) {
		status := manager.Status()
		response.Header().Set("Content-Type", "text/plain; version=0.0.4")
		phase := strings.NewReplacer("\\", "\\\\", "\"", "\\\"").Replace(status.Phase)
		ceiling := "0"
		if status.PartitionAdvisor.CeilingDetected {
			ceiling = "1"
		}
		_, _ = response.Write([]byte(
			"# TYPE camunda_capacity_manager_current_brokers gauge\n" +
				"camunda_capacity_manager_current_brokers " + strconv.Itoa(status.CurrentBrokers) + "\n" +
				"# TYPE camunda_capacity_manager_desired_brokers gauge\n" +
				"camunda_capacity_manager_desired_brokers " + strconv.Itoa(status.DesiredBrokers) + "\n" +
				"# TYPE camunda_capacity_manager_workload_replicas gauge\n" +
				"camunda_capacity_manager_workload_replicas " + strconv.Itoa(status.WorkloadReplicas) + "\n" +
				"# TYPE camunda_capacity_manager_pressure gauge\n" +
				"camunda_capacity_manager_pressure " + strconv.FormatFloat(status.Pressure, 'g', -1, 64) + "\n" +
				"# TYPE camunda_capacity_manager_phase gauge\n" +
				"camunda_capacity_manager_phase{phase=\"" + phase + "\"} 1\n" +
				"# TYPE camunda_capacity_manager_current_partitions gauge\n" +
				"camunda_capacity_manager_current_partitions " + strconv.Itoa(status.PartitionAdvisor.Current) + "\n" +
				"# TYPE camunda_capacity_manager_recommended_partitions gauge\n" +
				"camunda_capacity_manager_recommended_partitions " + strconv.Itoa(status.PartitionAdvisor.Recommended) + "\n" +
				"# TYPE camunda_capacity_manager_partition_ceiling_detected gauge\n" +
				"camunda_capacity_manager_partition_ceiling_detected " + ceiling + "\n"))
	})
	server := &http.Server{Addr: listenAddress, Handler: mux, ReadHeaderTimeout: 5 * time.Second}
	go func() {
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Error("serve status endpoint", "error", err)
		}
	}()

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	run(ctx, interval, manager, logger)
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := server.Shutdown(shutdownCtx); err != nil {
		logger.Warn("shutdown status endpoint", "error", err)
	}
}

func run(ctx context.Context, interval time.Duration, manager *capacity.Manager, logger *slog.Logger) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		if err := manager.Reconcile(ctx); err != nil {
			logger.Error("reconcile failed", "error", err)
		}
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}
