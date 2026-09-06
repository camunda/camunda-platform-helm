// Copyright Camunda Services GmbH and/or licensed to Camunda Services GmbH
// under one or more contributor license agreements. See the NOTICE file
// distributed with this work for additional information regarding copyright
// ownership. Licensed under the Apache License, Version 2.0; you may not use
// this file except in compliance with the License. You may obtain a copy of
// the License at http://www.apache.org/licenses/LICENSE-2.0

package capacity

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestDirectMetricsClientCalculatesRate(t *testing.T) {
	value := 10
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, _ *http.Request) {
		_, _ = fmt.Fprintf(response, "test_counter_total{partition=\"1\"} %d\n", value)
	}))
	defer server.Close()
	client := &DirectMetricsClient{URL: server.URL, Client: server.Client()}
	first, err := client.Query(context.Background(), "test_counter_total")
	if err != nil || first != nil {
		t.Fatalf("unexpected first sample %v, %v", first, err)
	}
	client.at = time.Now().Add(-time.Second)
	value = 30
	second, err := client.Query(context.Background(), "test_counter_total")
	if err != nil || second == nil || *second < 19 || *second > 21 {
		t.Fatalf("unexpected rate %v, %v", second, err)
	}
}

func TestDirectMetricsClientReturnsNilWhenMetricIsAbsent(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, _ *http.Request) {
		_, _ = fmt.Fprint(response, "different_metric 10\n")
	}))
	defer server.Close()
	client := &DirectMetricsClient{URL: server.URL, Client: server.Client()}
	value, err := client.Query(context.Background(), "missing_metric")
	if err != nil || value != nil {
		t.Fatalf("unexpected value %v, %v", value, err)
	}
}

func TestDirectMetricsClientUsesMaximumPerSeriesRate(t *testing.T) {
	values := []float64{10, 100}
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, _ *http.Request) {
		_, _ = fmt.Fprintf(response, "test_counter_total{partition=\"1\"}\t%v\ntest_counter_total{partition=\"2\"} %v\n", values[0], values[1])
	}))
	defer server.Close()
	client := &DirectMetricsClient{URL: server.URL, Client: server.Client()}
	_, _ = client.Query(context.Background(), "test_counter_total")
	client.at = time.Now().Add(-time.Second)
	values = []float64{20, 130}
	value, err := client.Query(context.Background(), "test_counter_total")
	if err != nil || value == nil || *value < 29 || *value > 31 {
		t.Fatalf("unexpected maximum rate %v, %v", value, err)
	}
}

func TestDirectMetricsClientReturnsMaximumGauge(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, _ *http.Request) {
		_, _ = fmt.Fprint(response, "test_gauge{partition=\"1\"} 0.4\ntest_gauge{partition=\"2\"} 0.9\n")
	}))
	defer server.Close()
	client := &DirectMetricsClient{URL: server.URL, Client: server.Client()}
	value, err := client.Gauge(context.Background(), "test_gauge")
	if err != nil || value == nil || *value != 0.9 {
		t.Fatalf("unexpected gauge %v, %v", value, err)
	}
}
