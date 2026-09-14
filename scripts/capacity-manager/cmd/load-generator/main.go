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
	"flag"
	"fmt"
	"io"
	"log"
	"mime/multipart"
	"net/http"
	"os/signal"
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
	var rate int
	flag.StringVar(&baseURL, "url", "", "Camunda v2 REST base URL")
	flag.StringVar(&username, "username", "demo", "basic-auth username")
	flag.StringVar(&password, "password", "demo", "basic-auth password")
	flag.IntVar(&rate, "rate", 100, "process instances per second")
	flag.Parse()
	if baseURL == "" || rate < 1 {
		log.Fatal("url and a positive rate are required")
	}
	client := &http.Client{Timeout: 10 * time.Second}
	if err := deploy(client, baseURL, username, password); err != nil {
		log.Fatalf("deploy process: %v", err)
	}
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	interval := time.Second / time.Duration(rate)
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if err := start(client, baseURL, username, password); err != nil {
				log.Printf("start process: %v", err)
			}
		}
	}
}

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
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		message, _ := io.ReadAll(io.LimitReader(response.Body, 1024))
		return fmt.Errorf("%s: %s", response.Status, message)
	}
	_, _ = io.Copy(io.Discard, response.Body)
	return nil
}
