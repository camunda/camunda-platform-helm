// Copyright Camunda Services GmbH and/or licensed to Camunda Services GmbH
// under one or more contributor license agreements. See the NOTICE file
// distributed with this work for additional information regarding copyright
// ownership. Licensed under the Apache License, Version 2.0; you may not use
// this file except in compliance with the License. You may obtain a copy of
// the License at http://www.apache.org/licenses/LICENSE-2.0

package capacity

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
)

type PrometheusClient struct {
	BaseURL string
	Client  *http.Client
}

func (c PrometheusClient) Query(ctx context.Context, query string) (*float64, error) {
	if c.BaseURL == "" || query == "" {
		return nil, nil
	}
	endpoint, err := url.Parse(c.BaseURL + "/api/v1/query")
	if err != nil {
		return nil, err
	}
	values := endpoint.Query()
	values.Set("query", query)
	endpoint.RawQuery = values.Encode()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint.String(), nil)
	if err != nil {
		return nil, err
	}
	res, err := c.Client.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("prometheus returned %s", res.Status)
	}
	var response struct {
		Status string `json:"status"`
		Data   struct {
			Result []struct {
				Value []json.RawMessage `json:"value"`
			} `json:"result"`
		} `json:"data"`
	}
	if err := json.NewDecoder(res.Body).Decode(&response); err != nil {
		return nil, err
	}
	if response.Status != "success" || len(response.Data.Result) == 0 {
		return nil, nil
	}
	var maximum *float64
	for _, result := range response.Data.Result {
		if len(result.Value) < 2 {
			continue
		}
		var raw string
		if err := json.Unmarshal(result.Value[1], &raw); err != nil {
			return nil, err
		}
		value, err := strconv.ParseFloat(raw, 64)
		if err != nil {
			return nil, err
		}
		if maximum == nil || value > *maximum {
			copy := value
			maximum = &copy
		}
	}
	return maximum, nil
}
