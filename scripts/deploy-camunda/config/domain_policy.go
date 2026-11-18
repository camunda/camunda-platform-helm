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

package config

import (
	"fmt"
	"slices"
	"strings"
)

// DomainPolicy is the set of ingress base domains a deploy may use.
// Entries are matched literally.
type DomainPolicy struct {
	Allowed []string
}

// DefaultDomainPolicy allows DefaultIngressBaseDomains.
func DefaultDomainPolicy() DomainPolicy {
	return DomainPolicy{Allowed: DefaultIngressBaseDomains}
}

// Domains returns the allowed base domains in declaration order.
func (p DomainPolicy) Domains() []string {
	return p.Allowed
}

// Allows reports whether domain is one of the allowed base domains.
func (p DomainPolicy) Allows(domain string) bool {
	return slices.Contains(p.Allowed, domain)
}

// Validate returns the error for --<flag> when domain is not allowed.
func (p DomainPolicy) Validate(flag, domain string) error {
	if p.Allows(domain) {
		return nil
	}
	return fmt.Errorf("--%s must be one of: %s", flag, strings.Join(p.Allowed, ", "))
}

// RequiredError is the error for --ingress-subdomain given without --ingress-base-domain.
func (p DomainPolicy) RequiredError() error {
	return fmt.Errorf("--ingress-base-domain is required when using --ingress-subdomain; valid values: %s", strings.Join(p.Allowed, ", "))
}

// PruneSuffixes returns ".<domain>" for each allowed domain: the host suffixes
// that identify redirect URIs a CI cleanup may remove.
func (p DomainPolicy) PruneSuffixes() []string {
	suffixes := make([]string, len(p.Allowed))
	for i, d := range p.Allowed {
		suffixes[i] = "." + d
	}
	return suffixes
}
