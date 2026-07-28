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
