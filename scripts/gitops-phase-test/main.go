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
}

type service struct {
	Spec struct {
		Selector map[string]string `json:"selector"`
	} `json:"spec"`
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
		args := []string{"upgrade", "--install", release, c.chartPath, "--namespace", c.namespace}
		args = append(args, helmSetArgs(phase)...)
		return run(nil, "helm", args...)
	case "argocd":
		manifest := map[string]any{
			"apiVersion": "argoproj.io/v1alpha1", "kind": "Application",
			"metadata": map[string]any{"name": release, "namespace": "argocd"},
			"spec": map[string]any{
				"project":     "default",
				"source":      map[string]any{"repoURL": c.repo, "targetRevision": c.revision, "path": c.chartPath, "helm": map[string]any{"parameters": helmParameters(phase)}},
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
		"global":   map[string]any{"identity": map[string]any{"service": map[string]any{"url": "http://identity"}}},
		"identity": map[string]any{"enabled": false}, "orchestration": map[string]any{"enabled": false},
		"connectors": map[string]any{"enabled": false}, "optimize": map[string]any{"enabled": false},
	}
}

func helmParameters(phase string) []map[string]string {
	params := []map[string]string{}
	for _, arg := range helmSetArgs(phase) {
		if !strings.Contains(arg, "=") {
			continue
		}
		parts := strings.SplitN(arg, "=", 2)
		params = append(params, map[string]string{"name": parts[0], "value": parts[1]})
	}
	return params
}

func helmSetArgs(phase string) []string {
	sets := []string{
		"camundaHub.enabled=true", "camundaHub.upgrade.phase=" + phase,
		"camundaHub.restapi.replicas=2", "camundaHub.restapi.mail.fromAddress=test@example.com",
		"camundaHub.restapi.pusher.secret.inlineSecret=gitops-test-secret", "camundaHub.restapi.pusher.client.secret.inlineSecret=gitops-test-key",
		"global.identity.service.url=http://identity", "identity.enabled=false", "orchestration.enabled=false",
		"connectors.enabled=false", "optimize.enabled=false",
	}
	args := make([]string, 0, len(sets)*2)
	for _, value := range sets {
		args = append(args, "--set", value)
	}
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
		if rest.Spec.Replicas == expectedRest && websockets.Spec.Replicas == expectedWebsockets && rest.Spec.Strategy.Type == "RollingUpdate" && rest.Spec.Template.Metadata.Labels["camunda.io/upgrade-phase"] == phase && websockets.Spec.Template.Metadata.Labels["camunda.io/upgrade-phase"] == phase {
			if err := assertServiceSelector(namespace, release+"-web-modeler-restapi"); err != nil {
				return err
			}
			return assertServiceSelector(namespace, release+"-web-modeler-websockets")
		}
		time.Sleep(5 * time.Second)
	}
	return fmt.Errorf("timed out waiting for phase %q", phase)
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
