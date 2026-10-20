// Copyright Camunda Services GmbH and/or licensed to Camunda Services GmbH
// under one or more contributor license agreements. See the NOTICE file
// distributed with this work for additional information regarding copyright
// ownership. Licensed under the Apache License, Version 2.0; you may not use
// this file except in compliance with the License. You may obtain a copy of
// the License at http://www.apache.org/licenses/LICENSE-2.0

package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"mime/multipart"
	"net/http"
	"os/signal"
	"sync"
	"sync/atomic"
	"syscall"
	"time"
)

const process = `<?xml version="1.0" encoding="UTF-8"?>
<bpmn:definitions xmlns:bpmn="http://www.omg.org/spec/BPMN/20100524/MODEL" targetNamespace="http://bpmn.io/schema/bpmn">
  <bpmn:process id="capacity-load" isExecutable="true">
    <bpmn:startEvent id="start"><bpmn:outgoing>flow</bpmn:outgoing></bpmn:startEvent>
    <bpmn:endEvent id="end"><bpmn:incoming>flow</bpmn:incoming></bpmn:endEvent>
    <bpmn:sequenceFlow id="flow" sourceRef="start" targetRef="end" />
  </bpmn:process>
</bpmn:definitions>`

func main() {
	var baseURL, username, password string
	var rate, maxRate, step, workers int
	var stepEvery time.Duration
	flag.StringVar(&baseURL, "url", "", "Camunda v2 REST base URL")
	flag.StringVar(&username, "username", "demo", "basic-auth username")
	flag.StringVar(&password, "password", "demo", "basic-auth password")
	flag.IntVar(&rate, "rate", 100, "initial process instances per second")
	flag.IntVar(&maxRate, "max-rate", 0, "ramp the rate up to this value (0 keeps the initial rate)")
	flag.IntVar(&step, "ramp-step", 50, "process instances per second added at each ramp step")
	flag.DurationVar(&stepEvery, "ramp-every", 30*time.Second, "interval between ramp steps")
	flag.IntVar(&workers, "workers", 32, "concurrent request workers")
	flag.Parse()
	if baseURL == "" || rate < 1 || workers < 1 || step < 1 || stepEvery <= 0 {
		log.Fatal("url, a positive rate, workers, ramp-step and ramp-every are required")
	}
	maxRate = max(maxRate, rate)
	client := &http.Client{Timeout: 10 * time.Second, Transport: &http.Transport{MaxIdleConnsPerHost: workers}}
	if err := deploy(client, baseURL, username, password); err != nil {
		log.Fatalf("deploy process: %v", err)
	}
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	var stats counters
	jobs := make(chan struct{}, workers)
	var wg sync.WaitGroup
	for range workers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for range jobs {
				stats.record(start(client, baseURL, username, password))
			}
		}()
	}
	go report(ctx, &stats, func() int { return int(currentRate.Load()) })

	currentRate.Store(int64(rate))
	ramp := time.NewTicker(stepEvery)
	defer ramp.Stop()
	ticker := time.NewTicker(time.Second / time.Duration(rate))
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			close(jobs)
			wg.Wait()
			return
		case <-ramp.C:
			if saturated.Load() {
				continue
			}
			if next := min(int(currentRate.Load())+step, maxRate); next != int(currentRate.Load()) {
				currentRate.Store(int64(next))
				ticker.Reset(time.Second / time.Duration(next))
			}
		case <-ticker.C:
			select {
			case jobs <- struct{}{}:
			default:
				// Every worker is busy: the cluster is slower than the offered
				// rate, which is itself a sign of saturation.
				stats.skipped.Add(1)
			}
		}
	}
}

var currentRate atomic.Int64

// saturated stops the ramp once the cluster rejects requests, so the load
// holds just past the point where one broker can no longer keep up.
var saturated atomic.Bool

// counters tracks request outcomes; rejected counts backpressure responses.
type counters struct {
	ok, rejected, failed, skipped atomic.Int64
}

func (c *counters) record(err error) {
	switch {
	case err == nil:
		c.ok.Add(1)
	case errors.Is(err, errBackpressure):
		c.rejected.Add(1)
	default:
		c.failed.Add(1)
		log.Printf("start process: %v", err)
	}
}

func report(ctx context.Context, stats *counters, rate func() int) {
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			ok, rejected := stats.ok.Swap(0), stats.rejected.Swap(0)
			if total := ok + rejected; total > 0 && float64(rejected)/float64(total) >= 0.01 && !saturated.Load() {
				saturated.Store(true)
				log.Printf("backpressure observed, holding rate at %d/s", rate())
			}
			log.Printf("rate=%d/s ok=%d rejected=%d failed=%d skipped=%d", rate(),
				ok, rejected, stats.failed.Swap(0), stats.skipped.Swap(0))
		}
	}
}

var errBackpressure = errors.New("rejected by backpressure")

func deploy(client *http.Client, baseURL, username, password string) error {
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	part, err := writer.CreateFormFile("resources", "capacity-load.bpmn")
	if err != nil {
		return err
	}
	if _, err := io.WriteString(part, process); err != nil {
		return err
	}
	if err := writer.Close(); err != nil {
		return err
	}
	request, err := http.NewRequest(http.MethodPost, baseURL+"/deployments", &body)
	if err != nil {
		return err
	}
	request.Header.Set("Content-Type", writer.FormDataContentType())
	return execute(client, request, username, password)
}

func start(client *http.Client, baseURL, username, password string) error {
	payload, _ := json.Marshal(map[string]any{"processDefinitionId": "capacity-load", "variables": map[string]any{}})
	request, err := http.NewRequest(http.MethodPost, baseURL+"/process-instances", bytes.NewReader(payload))
	if err != nil {
		return err
	}
	request.Header.Set("Content-Type", "application/json")
	return execute(client, request, username, password)
}

func execute(client *http.Client, request *http.Request, username, password string) error {
	request.SetBasicAuth(username, password)
	response, err := client.Do(request)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	if response.StatusCode == http.StatusServiceUnavailable || response.StatusCode == http.StatusTooManyRequests {
		_, _ = io.Copy(io.Discard, response.Body)
		return fmt.Errorf("%s: %w", response.Status, errBackpressure)
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		message, _ := io.ReadAll(io.LimitReader(response.Body, 1024))
		return fmt.Errorf("%s: %s", response.Status, message)
	}
	_, _ = io.Copy(io.Discard, response.Body)
	return nil
}
