// Copyright Camunda Services GmbH and/or licensed to Camunda Services GmbH
// under one or more contributor license agreements. See the NOTICE file
// distributed with this work for additional information regarding copyright
// ownership. Licensed under the Apache License, Version 2.0; you may not use
// this file except in compliance with the License. You may obtain a copy of
// the License at http://www.apache.org/licenses/LICENSE-2.0

package capacity

import (
	"bytes"
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"strings"
)

type KubernetesClient struct {
	BaseURL     string
	Namespace   string
	StatefulSet string
	TokenPath   string
	Client      *http.Client
}

const targetBrokersAnnotation = "capacity-manager.camunda.io/target-brokers"
const completedBrokersAnnotation = "capacity-manager.camunda.io/completed-brokers"

func NewInClusterKubernetesClient(namespace, statefulSet string, timeoutClient *http.Client) (KubernetesClient, error) {
	host := os.Getenv("KUBERNETES_SERVICE_HOST")
	port := os.Getenv("KUBERNETES_SERVICE_PORT_HTTPS")
	if port == "" {
		port = "443"
	}
	tokenPath := "/var/run/secrets/kubernetes.io/serviceaccount/token"
	ca, err := os.ReadFile("/var/run/secrets/kubernetes.io/serviceaccount/ca.crt")
	if err != nil {
		return KubernetesClient{}, fmt.Errorf("read Kubernetes CA: %w", err)
	}
	pool := x509.NewCertPool()
	if !pool.AppendCertsFromPEM(ca) {
		return KubernetesClient{}, fmt.Errorf("parse Kubernetes CA")
	}
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.TLSClientConfig = &tls.Config{MinVersion: tls.VersionTLS12, RootCAs: pool}
	client := *timeoutClient
	client.Transport = transport
	return KubernetesClient{
		BaseURL:     "https://" + host + ":" + port,
		Namespace:   namespace,
		StatefulSet: statefulSet,
		TokenPath:   tokenPath,
		Client:      &client,
	}, nil
}

func (c KubernetesClient) statefulSetURL() string {
	return fmt.Sprintf("%s/apis/apps/v1/namespaces/%s/statefulsets/%s", c.BaseURL, c.Namespace, c.StatefulSet)
}

func (c KubernetesClient) request(ctx context.Context, method, endpoint string, body []byte) (*http.Response, error) {
	token, err := os.ReadFile(c.TokenPath)
	if err != nil {
		return nil, fmt.Errorf("read service account token: %w", err)
	}
	request, err := http.NewRequestWithContext(ctx, method, endpoint, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	request.Header.Set("Authorization", "Bearer "+strings.TrimSpace(string(token)))
	if body != nil {
		request.Header.Set("Content-Type", "application/merge-patch+json")
	}
	return c.Client.Do(request)
}

func (c KubernetesClient) State(ctx context.Context) (WorkloadState, error) {
	response, err := c.request(ctx, http.MethodGet, c.statefulSetURL(), nil)
	if err != nil {
		return WorkloadState{}, err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return WorkloadState{}, fmt.Errorf("get StatefulSet returned %s", response.Status)
	}
	var statefulSet struct {
		Metadata struct {
			Annotations map[string]string `json:"annotations"`
		} `json:"metadata"`
		Spec struct {
			Replicas int `json:"replicas"`
		} `json:"spec"`
	}
	if err := json.NewDecoder(response.Body).Decode(&statefulSet); err != nil {
		return WorkloadState{}, err
	}
	target := statefulSet.Spec.Replicas
	if raw := statefulSet.Metadata.Annotations[targetBrokersAnnotation]; raw != "" {
		parsed, err := strconv.Atoi(raw)
		if err != nil {
			return WorkloadState{}, fmt.Errorf("parse target brokers annotation: %w", err)
		}
		target = parsed
	}
	return WorkloadState{Replicas: statefulSet.Spec.Replicas, Target: target}, nil
}

func (c KubernetesClient) SetTarget(ctx context.Context, target int) error {
	return c.patchStatefulSet(ctx, map[string]any{
		"metadata": map[string]any{"annotations": map[string]string{targetBrokersAnnotation: strconv.Itoa(target)}},
	})
}

func (c KubernetesClient) Scale(ctx context.Context, replicas int) error {
	return c.patchStatefulSet(ctx, map[string]any{
		"metadata": map[string]any{"annotations": map[string]string{targetBrokersAnnotation: strconv.Itoa(replicas)}},
		"spec":     map[string]int{"replicas": replicas},
	})
}

func (c KubernetesClient) MarkCompleted(ctx context.Context, brokers int) error {
	return c.patchStatefulSet(ctx, map[string]any{
		"metadata": map[string]any{"annotations": map[string]string{completedBrokersAnnotation: strconv.Itoa(brokers)}},
	})
}

func (c KubernetesClient) patchStatefulSet(ctx context.Context, patch map[string]any) error {
	payload, err := json.Marshal(patch)
	if err != nil {
		return err
	}
	response, err := c.request(ctx, http.MethodPatch, c.statefulSetURL(), payload)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return fmt.Errorf("patch StatefulSet returned %s", response.Status)
	}
	return nil
}

func (c KubernetesClient) PodStarted(ctx context.Context, ordinal int) (bool, error) {
	name := fmt.Sprintf("%s-%d", c.StatefulSet, ordinal)
	endpoint := fmt.Sprintf("%s/api/v1/namespaces/%s/pods/%s", c.BaseURL, c.Namespace, name)
	response, err := c.request(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return false, err
	}
	defer response.Body.Close()
	if response.StatusCode == http.StatusNotFound {
		return false, nil
	}
	if response.StatusCode != http.StatusOK {
		return false, fmt.Errorf("get pod returned %s", response.Status)
	}
	var pod struct {
		Metadata struct {
			DeletionTimestamp *string `json:"deletionTimestamp"`
		} `json:"metadata"`
		Status struct {
			Conditions []struct {
				Type   string `json:"type"`
				Status string `json:"status"`
			} `json:"conditions"`
		} `json:"status"`
	}
	if err := json.NewDecoder(response.Body).Decode(&pod); err != nil {
		return false, err
	}
	if pod.Metadata.DeletionTimestamp != nil {
		return false, nil
	}
	for _, condition := range pod.Status.Conditions {
		if condition.Type == "Ready" && condition.Status == "True" {
			return true, nil
		}
	}
	return false, nil
}
