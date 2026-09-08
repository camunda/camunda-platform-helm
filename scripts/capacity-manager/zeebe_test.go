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
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestZeebeClient(t *testing.T) {
	t.Run("decodes live topology shape", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
			if request.URL.Path != "/orchestration/actuator/cluster" {
				t.Fatalf("unexpected path %s", request.URL.Path)
			}
			_, _ = io.WriteString(response, `{"version":2,"brokers":[{"id":0,"state":"ACTIVE","partitions":[{"id":1,"state":"ACTIVE","priority":1}]}]}`)
		}))
		defer server.Close()

		topology, err := (ZeebeClient{BaseURL: server.URL + "/orchestration", Client: server.Client()}).Topology(context.Background())
		if err != nil || topology.ActiveBrokerCount() != 1 || !topology.Healthy() {
			t.Fatalf("unexpected topology %#v, %v", topology, err)
		}
	})

	t.Run("dry runs broker count", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
			if request.Method != http.MethodPatch || request.URL.Query().Get("dryRun") != "true" {
				t.Fatalf("unexpected request %s %s", request.Method, request.URL.String())
			}
			body, _ := io.ReadAll(request.Body)
			if strings.TrimSpace(string(body)) != `{"brokers":{"count":2}}` {
				t.Fatalf("unexpected body %s", body)
			}
			response.WriteHeader(http.StatusAccepted)
			_, _ = fmt.Fprint(response, `{"changeId":7,"plannedChanges":[{"operation":"BROKER_ADD"}],"expectedTopology":[{"id":0},{"id":1}]}`)
		}))
		defer server.Close()

		plan, err := (ZeebeClient{BaseURL: server.URL, Client: server.Client()}).Scale(context.Background(), 2, true)
		if err != nil || plan.ChangeID != 7 || len(plan.ExpectedTopology) != 2 {
			t.Fatalf("unexpected plan %#v, %v", plan, err)
		}
	})
}

func TestTopologyPartitionCountUsesUniqueIds(t *testing.T) {
	topology := Topology{Brokers: []Broker{
		{State: "ACTIVE", Partitions: []Partition{{ID: 1, State: "ACTIVE"}, {ID: 2, State: "ACTIVE"}}},
		{State: "ACTIVE", Partitions: []Partition{{ID: 1, State: "ACTIVE"}, {ID: 2, State: "ACTIVE"}}},
	}}
	if topology.PartitionCount() != 2 {
		t.Fatalf("unexpected partition count %d", topology.PartitionCount())
	}
}
