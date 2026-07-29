// Copyright Camunda Services GmbH and/or licensed to Camunda Services GmbH
// under one or more contributor license agreements. See the NOTICE file
// distributed with this work for additional information regarding copyright
// ownership. Licensed under the Apache License, Version 2.0; you may not use
// this file except in compliance with the License. You may obtain a copy of
// the License at http://www.apache.org/licenses/LICENSE-2.0

package capacity

import (
	"bufio"
	"context"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"
)

type DirectMetricsClient struct {
	URL    string
	Client *http.Client
	mu     sync.Mutex
	last   map[string]float64
	at     time.Time
}

func (c *DirectMetricsClient) Gauge(ctx context.Context, metric string) (*float64, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, c.URL, nil)
	if err != nil {
		return nil, err
	}
	response, err := c.Client.Do(request)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("metrics endpoint returned %s", response.Status)
	}
	var maximum *float64
	scanner := bufio.NewScanner(response.Body)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		name, value, ok := strings.Cut(line, " ")
		if !ok || (name != metric && !strings.HasPrefix(name, metric+"{")) {
			continue
		}
		parsed, err := strconv.ParseFloat(strings.TrimSpace(value), 64)
		if err != nil {
			return nil, fmt.Errorf("parse metric %s: %w", metric, err)
		}
		if maximum == nil || parsed > *maximum {
			copy := parsed
			maximum = &copy
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return maximum, nil
}

func (c *DirectMetricsClient) Query(ctx context.Context, metric string) (*float64, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, c.URL, nil)
	if err != nil {
		return nil, err
	}
	response, err := c.Client.Do(request)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("metrics endpoint returned %s", response.Status)
	}
	series := map[string]float64{}
	scanner := bufio.NewScanner(response.Body)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 2 || (fields[0] != metric && !strings.HasPrefix(fields[0], metric+"{")) {
			continue
		}
		parsed, err := strconv.ParseFloat(fields[1], 64)
		if err != nil {
			return nil, fmt.Errorf("parse metric %s: %w", metric, err)
		}
		series[fields[0]] = parsed
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	if len(series) == 0 {
		return nil, nil
	}
	now := time.Now()
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.at.IsZero() {
		c.last = series
		c.at = now
		return nil, nil
	}
	elapsed := now.Sub(c.at).Seconds()
	maximum := 0.0
	hasRate := false
	for name, current := range series {
		previous, ok := c.last[name]
		if !ok || current < previous {
			continue
		}
		rate := (current - previous) / elapsed
		if !hasRate || rate > maximum {
			maximum = rate
			hasRate = true
		}
	}
	c.last = series
	c.at = now
	if !hasRate {
		return nil, nil
	}
	return &maximum, nil
}
