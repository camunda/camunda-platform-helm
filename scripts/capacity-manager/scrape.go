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
	last   float64
	at     time.Time
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
	total := 0.0
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
		total += parsed
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	now := time.Now()
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.at.IsZero() || total < c.last {
		c.last = total
		c.at = now
		return nil, nil
	}
	elapsed := now.Sub(c.at).Seconds()
	rate := (total - c.last) / elapsed
	c.last = total
	c.at = now
	return &rate, nil
}
