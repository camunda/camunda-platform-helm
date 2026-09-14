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
)

func TestPrometheusClientReturnsMaximumVectorValue(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		if request.URL.Query().Get("query") != "partition_load" {
			t.Fatalf("unexpected query %s", request.URL.RawQuery)
		}
		_, _ = fmt.Fprint(response, `{"status":"success","data":{"result":[{"value":[1,"0.4"]},{"value":[1,"0.9"]}]}}`)
	}))
	defer server.Close()
	value, err := (PrometheusClient{BaseURL: server.URL, Client: server.Client()}).Query(context.Background(), "partition_load")
	if err != nil || value == nil || *value != 0.9 {
		t.Fatalf("unexpected value %v, %v", value, err)
	}
}
