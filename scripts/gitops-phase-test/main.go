// Copyright 2026 Camunda Services GmbH
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

const release = "gitops-phase"

type config struct {
	controller string
	repo       string
	revision   string
	chartPath  string
	namespace  string
}

type deployment struct {
	Metadata struct {
		Generation int64 `json:"generation"`
	} `json:"metadata"`
	Spec struct {
		Replicas int32 `json:"replicas"`
		Strategy struct {
			Type string `json:"type"`
		} `json:"strategy"`
		Template struct {
			Metadata struct {
				Labels map[string]string `json:"labels"`
			} `json:"metadata"`
		} `json:"template"`
	} `json:"spec"`
	Status struct {
		ObservedGeneration int64 `json:"observedGeneration"`
		Replicas           int32 `json:"replicas"`
		UpdatedReplicas    int32 `json:"updatedReplicas"`
		ReadyReplicas      int32 `json:"readyReplicas"`
		AvailableReplicas  int32 `json:"availableReplicas"`
	} `json:"status"`
}

type service struct {
	Spec struct {
		Selector map[string]string `json:"selector"`
	} `json:"spec"`
}

type endpoints struct {
	Subsets []struct {
		Addresses []json.RawMessage `json:"addresses"`
	} `json:"subsets"`
}

func main() {
	var cfg config
	flag.StringVar(&cfg.controller, "controller", "helm", "helm, argocd, or flux")
	flag.StringVar(&cfg.repo, "repo", "https://github.com/camunda/camunda-platform-helm.git", "Git repository URL")
	flag.StringVar(&cfg.revision, "revision", "main", "Git revision")
	flag.StringVar(&cfg.chartPath, "chart-path", "charts/camunda-platform-8.10", "chart path")
	flag.StringVar(&cfg.namespace, "namespace", "gitops-phase", "target namespace")
	flag.Parse()

	namespace, err := output("kubectl", "create", "namespace", cfg.namespace, "--dry-run=client", "-o", "json")
	must(err)
	mustRun(namespace, "kubectl", "apply", "-f", "-")

	for _, phase := range []string{"normal", "quiesce", "migrate", "normal"} {
		must(cfg.apply(phase))
		must(waitForPhase(cfg.namespace, phase))
		fmt.Printf("phase %s converged through %s\n", phase, cfg.controller)
	}
}

func (c config) apply(phase string) error {
	switch c.controller {
	case "helm":
		repoRoot, err := output("git", "rev-parse", "--show-toplevel")
		if err != nil {
			return err
		}
		chartPath := filepath.Join(strings.TrimSpace(string(repoRoot)), c.chartPath)
		args := []string{"upgrade", "--install", release, chartPath, "--namespace", c.namespace}
		args = append(args, helmSetArgs(phase)...)
		return run(nil, "helm", args...)
	case "argocd":
		valuesJSON, err := json.Marshal(values(phase))
		if err != nil {
			return err
		}
		manifest := map[string]any{
			"apiVersion": "argoproj.io/v1alpha1", "kind": "Application",
			"metadata": map[string]any{"name": release, "namespace": "argocd"},
			"spec": map[string]any{
				"project":     "default",
				"source":      map[string]any{"repoURL": c.repo, "targetRevision": c.revision, "path": c.chartPath, "helm": map[string]any{"values": string(valuesJSON)}},
				"destination": map[string]any{"server": "https://kubernetes.default.svc", "namespace": c.namespace},
				"syncPolicy":  map[string]any{"automated": map[string]any{"prune": true, "selfHeal": true}, "syncOptions": []string{"CreateNamespace=true"}},
			},
		}
		return applyJSON(manifest)
	case "flux":
		source := map[string]any{
			"apiVersion": "source.toolkit.fluxcd.io/v1", "kind": "GitRepository",
			"metadata": map[string]any{"name": release, "namespace": "flux-system"},
			"spec":     map[string]any{"interval": "1m", "url": c.repo, "ref": map[string]any{"commit": c.revision}},
		}
		if err := applyJSON(source); err != nil {
			return err
		}
		releaseManifest := map[string]any{
			"apiVersion": "helm.toolkit.fluxcd.io/v2", "kind": "HelmRelease",
			"metadata": map[string]any{"name": release, "namespace": "flux-system"},
			"spec": map[string]any{
				"interval": "1m", "releaseName": release, "targetNamespace": c.namespace,
				"chart":   map[string]any{"spec": map[string]any{"chart": "./" + c.chartPath, "sourceRef": map[string]any{"kind": "GitRepository", "name": release, "namespace": "flux-system"}}},
				"install": map[string]any{"disableWait": true}, "upgrade": map[string]any{"disableWait": true},
				"values": values(phase),
			},
		}
		return applyJSON(releaseManifest)
	default:
		return fmt.Errorf("unsupported controller %q", c.controller)
	}
}

func values(phase string) map[string]any {
	return map[string]any{
		"camundaHub": map[string]any{
			"enabled": true, "upgrade": map[string]any{"phase": phase},
			"restapi": map[string]any{
				"replicas": 2, "mail": map[string]any{"fromAddress": "test@example.com"},
				"pusher": map[string]any{
					"secret": map[string]any{"inlineSecret": "gitops-test-secret"},
					"client": map[string]any{"secret": map[string]any{"inlineSecret": "gitops-test-key"}},
				},
			},
		},
		"webModeler": map[string]any{
			"image": map[string]any{"tag": "1.36.1"},
			"restapi": map[string]any{
				"image":   map[string]any{"repository": "busybox"},
				"command": []string{"sh", "-c", "mkdir -p /tmp/www/health; touch /tmp/www/health/readiness /tmp/www/health/liveness; httpd -p 8081 -h /tmp/www; httpd -f -p 8091 -h /tmp/www"},
			},
			"websockets": map[string]any{
				"image":          map[string]any{"repository": "busybox"},
				"command":        []string{"sleep", "3600"},
				"readinessProbe": map[string]any{"enabled": false},
			},
		},
		"global":   map[string]any{"identity": map[string]any{"service": map[string]any{"url": "http://identity"}}},
		"identity": map[string]any{"enabled": false}, "orchestration": map[string]any{"enabled": false},
		"connectors": map[string]any{"enabled": false}, "optimize": map[string]any{"enabled": false},
	}
}

func helmSetArgs(phase string) []string {
	sets := []string{
		"camundaHub.enabled=true", "camundaHub.upgrade.phase=" + phase,
		"camundaHub.restapi.replicas=2", "camundaHub.restapi.mail.fromAddress=test@example.com",
		"camundaHub.restapi.pusher.secret.inlineSecret=gitops-test-secret", "camundaHub.restapi.pusher.client.secret.inlineSecret=gitops-test-key",
		"global.identity.service.url=http://identity", "identity.enabled=false", "orchestration.enabled=false",
		"connectors.enabled=false", "optimize.enabled=false",
		"webModeler.image.tag=1.36.1", "webModeler.restapi.image.repository=busybox", "webModeler.websockets.image.repository=busybox",
	}
	args := make([]string, 0, len(sets)*2)
	for _, value := range sets {
		args = append(args, "--set", value)
	}
	args = append(args,
		"--set-json", `webModeler.restapi.command=["sh","-c","mkdir -p /tmp/www/health; touch /tmp/www/health/readiness /tmp/www/health/liveness; httpd -p 8081 -h /tmp/www; httpd -f -p 8091 -h /tmp/www"]`,
		"--set-json", `webModeler.websockets.command=["sleep","3600"]`,
		"--set", "webModeler.websockets.readinessProbe.enabled=false",
	)
	return args
}

func waitForPhase(namespace, phase string) error {
	deadline := time.Now().Add(5 * time.Minute)
	for time.Now().Before(deadline) {
		rest, err := getDeployment(namespace, release+"-web-modeler-restapi")
		if err != nil {
			time.Sleep(5 * time.Second)
			continue
		}
		websockets, err := getDeployment(namespace, release+"-web-modeler-websockets")
		if err != nil {
			time.Sleep(5 * time.Second)
			continue
		}
		expectedRest, expectedWebsockets := int32(2), int32(1)
		if phase == "quiesce" {
			expectedRest, expectedWebsockets = 0, 0
		} else if phase == "migrate" {
			expectedRest, expectedWebsockets = 1, 0
		}
		if deploymentConverged(rest, expectedRest, phase) && deploymentConverged(websockets, expectedWebsockets, phase) && rest.Spec.Strategy.Type == "RollingUpdate" {
			if err := assertServiceSelector(namespace, release+"-web-modeler-restapi"); err != nil {
				return err
			}
			if err := assertServiceSelector(namespace, release+"-web-modeler-websockets"); err != nil {
				return err
			}
			if err := assertPhasePods(namespace, phase, expectedRest+expectedWebsockets); err != nil {
				time.Sleep(5 * time.Second)
				continue
			}
			expectEndpoints := phase == "normal"
			if err := assertEndpoints(namespace, release+"-web-modeler-restapi", expectEndpoints); err != nil {
				time.Sleep(5 * time.Second)
				continue
			}
			return assertEndpoints(namespace, release+"-web-modeler-websockets", expectEndpoints)
		}
		time.Sleep(5 * time.Second)
	}
	return fmt.Errorf("timed out waiting for phase %q", phase)
}

func deploymentConverged(value deployment, replicas int32, phase string) bool {
	return value.Spec.Replicas == replicas &&
		value.Metadata.Generation == value.Status.ObservedGeneration &&
		value.Status.Replicas == replicas &&
		value.Status.UpdatedReplicas == replicas &&
		value.Status.ReadyReplicas == replicas &&
		value.Status.AvailableReplicas == replicas &&
		value.Spec.Template.Metadata.Labels["camunda.io/upgrade-phase"] == phase
}

func assertPhasePods(namespace, phase string, expected int32) error {
	data, err := output("kubectl", "-n", namespace, "get", "pods", "-l", "camunda.io/upgrade-phase="+phase, "-o", `jsonpath={.items[*].metadata.name}`)
	if err != nil {
		return err
	}
	count := len(strings.Fields(string(data)))
	if count != int(expected) {
		return fmt.Errorf("phase %s has %d pods, expected %d", phase, count, expected)
	}
	return nil
}

func assertEndpoints(namespace, name string, expected bool) error {
	data, err := output("kubectl", "-n", namespace, "get", "endpoints", name, "-o", "json")
	if err != nil {
		return err
	}
	var result endpoints
	if err := json.Unmarshal(data, &result); err != nil {
		return err
	}
	hasAddresses := false
	for _, subset := range result.Subsets {
		hasAddresses = hasAddresses || len(subset.Addresses) > 0
	}
	if hasAddresses != expected {
		return fmt.Errorf("endpoints %s address presence is %t, expected %t", name, hasAddresses, expected)
	}
	return nil
}

func getDeployment(namespace, name string) (deployment, error) {
	data, err := output("kubectl", "-n", namespace, "get", "deployment", name, "-o", "json")
	if err != nil {
		return deployment{}, err
	}
	var result deployment
	err = json.Unmarshal(data, &result)
	return result, err
}

func assertServiceSelector(namespace, name string) error {
	data, err := output("kubectl", "-n", namespace, "get", "service", name, "-o", "json")
	if err != nil {
		return err
	}
	var result service
	if err := json.Unmarshal(data, &result); err != nil {
		return err
	}
	if result.Spec.Selector["camunda.io/upgrade-phase"] != "normal" {
		return fmt.Errorf("service %s does not select only normal pods", name)
	}
	return nil
}

func applyJSON(value any) error {
	data, err := json.Marshal(value)
	if err != nil {
		return err
	}
	return run(data, "kubectl", "apply", "-f", "-")
}

func output(name string, args ...string) ([]byte, error) {
	cmd := exec.Command(name, args...)
	cmd.Stderr = os.Stderr
	return cmd.Output()
}

func run(stdin []byte, name string, args ...string) error {
	cmd := exec.Command(name, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if stdin != nil {
		cmd.Stdin = bytes.NewReader(stdin)
	}
	return cmd.Run()
}

func mustRun(stdin []byte, name string, args ...string) {
	must(run(stdin, name, args...))
}

func must(err error) {
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
