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

// Package registry resolves container-registry credentials and describes the
// registry deploy-camunda logs into and creates a pull secret for.
package registry

import (
	"fmt"
	"os"
	"strings"

	"scripts/camunda-core/pkg/utils"
)

// Chain is the ordered list of environment variables a credential falls back
// to when no flag value is given.
type Chain struct {
	Username []string
	Password []string
}

// Resolve returns flagUser and flagPass, each replaced by the first non-empty
// lookup of its chain when empty.
func (c Chain) Resolve(flagUser, flagPass string, lookup func(string) string) (string, string) {
	return firstNonEmpty(flagUser, c.Username, lookup), firstNonEmpty(flagPass, c.Password, lookup)
}

// ResolveEnv is Resolve against the process environment.
func (c Chain) ResolveEnv(flagUser, flagPass string) (string, string) {
	return c.Resolve(flagUser, flagPass, os.Getenv)
}

func firstNonEmpty(flagVal string, names []string, lookup func(string) string) string {
	if flagVal != "" {
		return flagVal
	}
	vals := make([]string, len(names))
	for i, n := range names {
		vals[i] = lookup(n)
	}
	return utils.FirstNonEmpty(vals...)
}

// DockerHubLogin is the credential chain for Docker Hub.
var DockerHubLogin = Chain{
	Username: []string{"DOCKERHUB_USERNAME", "TEST_DOCKER_USERNAME"},
	Password: []string{"DOCKERHUB_PASSWORD", "TEST_DOCKER_PASSWORD"},
}

// Provider is a registry deploy-camunda logs into and creates a pull secret for.
type Provider struct {
	Name       string
	Host       string
	Login      Chain
	PullSecret string
}

const harborPullName = "registry-camunda-cloud"

// Harbor is the Camunda Harbor registry.
var Harbor = Provider{
	Name: "harbor",
	Host: "registry.camunda.cloud",
	Login: Chain{
		Username: []string{"HARBOR_USERNAME", "TEST_DOCKER_USERNAME_CAMUNDA_CLOUD", "NEXUS_USERNAME"},
		Password: []string{"HARBOR_PASSWORD", "TEST_DOCKER_PASSWORD_CAMUNDA_CLOUD", "NEXUS_PASSWORD"},
	},
	PullSecret: harborPullName,
}

// Describe renders the provider's host, pull secret and credential chain.
func (p Provider) Describe() string {
	if p.Name == "" {
		return "none"
	}
	return fmt.Sprintf("%s: host=%s pull-secret=%s username-env=%s password-env=%s",
		p.Name, p.Host, p.PullSecret, strings.Join(p.Login.Username, ","), strings.Join(p.Login.Password, ","))
}
