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

// Package provider defines the pluggable provider contract used to assemble a
// Camunda deployment from independent slots (credentials, registry, domain,
// tls, persistence, nodePools). Each slot is filled by exactly one named
// provider chosen from a process-wide registry.
//
// Contract for every Provider:
//   - Preflight is read-only and fast. It inspects the environment and reports
//     Findings; it must never mutate the cluster or local state.
//   - Values is pure. It derives a ValuesLayer from the Env alone, without side
//     effects, so it can be called repeatedly (for example to render offline).
//   - Prepare mutates the cluster (creates secrets, namespaces, fixtures, ...)
//     and may be a no-op for providers that only contribute values.
//   - Describe returns a JSON-serialisable summary for diagnostics.
package provider

import (
	"context"
	"slices"
)

// Slot identifies one pluggable concern of a deployment.
type Slot string

// Known slots.
const (
	SlotCredentials Slot = "credentials"
	SlotRegistry    Slot = "registry"
	SlotDomain      Slot = "domain"
	SlotTLS         Slot = "tls"
	SlotPersistence Slot = "persistence"
	SlotNodePools   Slot = "nodePools"
)

// Slots returns the fixed order in which slots are resolved and their values
// layers are applied. A fresh slice is returned on each call.
func Slots() []Slot {
	return []Slot{
		SlotCredentials,
		SlotRegistry,
		SlotDomain,
		SlotTLS,
		SlotPersistence,
		SlotNodePools,
	}
}

// Valid reports whether s is one of the known slots.
func (s Slot) Valid() bool {
	return slices.Contains(Slots(), s)
}

// Finding is a single Preflight result. Code is a stable machine-readable
// identifier; Remediation tells the user how to fix a failing check.
type Finding struct {
	OK          bool
	Code        string
	Message     string
	Remediation string
}

// Env is the minimal input shared with providers. Settings carries
// provider-specific options keyed by name.
type Env struct {
	Namespace   string
	KubeContext string
	Settings    map[string]string
}

// ValuesLayer is a provider's contribution to the Helm values. Files are
// applied in order, followed by Values.
type ValuesLayer struct {
	Files  []string
	Values map[string]any
}

// Provider fills a single Slot. See the package documentation for the
// semantics of each method.
type Provider interface {
	Slot() Slot
	Name() string
	Preflight(ctx context.Context, env Env) []Finding
	Values(ctx context.Context, env Env) (ValuesLayer, error)
	Prepare(ctx context.Context, env Env) error
	Describe() map[string]any
}

// Factory constructs a new Provider instance.
type Factory func() Provider
