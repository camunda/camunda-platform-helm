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

package kube

import "time"

type ApplyManifestOptions struct {
	Retry         bool
	RetryAttempts int
	Timeout       time.Duration
}

func DefaultApplyManifestOptions() ApplyManifestOptions {
	return ApplyManifestOptions{
		Retry:         true,
		RetryAttempts: 5,
		Timeout:       30 * time.Second,
	}
}

type ExternalSecretsOptions struct {
	SkipIfCRDMissing bool
	Timeout          time.Duration
}

func DefaultExternalSecretsOptions() ExternalSecretsOptions {
	return ExternalSecretsOptions{
		SkipIfCRDMissing: true,
		Timeout:          300 * time.Second,
	}
}

type SecretOptions struct {
	RetryOnConflict bool
	FailIfExists    bool
}

func DefaultSecretOptions() SecretOptions {
	return SecretOptions{
		RetryOnConflict: true,
		FailIfExists:    false,
	}
}

type NamespaceOptions struct {
	Labels          map[string]string
	Annotations     map[string]string
	RetryOnConflict bool
}

func DefaultNamespaceOptions() NamespaceOptions {
	return NamespaceOptions{
		Labels:          make(map[string]string),
		Annotations:     make(map[string]string),
		RetryOnConflict: true,
	}
}
