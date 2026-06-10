// Copyright 2022 Camunda Services GmbH
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
	"testing"
)

func TestBuildOverrides86_ZeebeLinksToGateway(t *testing.T) {
	t.Setenv("ZEEBE_IMAGE_TAG", "8.7.99")
	t.Setenv("ZEEBE_GATEWAY_IMAGE_TAG", "")

	overrides := buildOverrides86()

	if overrides.Zeebe == nil {
		t.Fatal("expected Zeebe override to be set")
	}
	if overrides.ZeebeGateway == nil {
		t.Fatal("expected ZeebeGateway override to be set when ZEEBE_IMAGE_TAG is provided")
	}
	if overrides.ZeebeGateway.Image.Tag != "8.7.99" {
		t.Errorf("expected ZeebeGateway tag to be 8.7.99, got %s", overrides.ZeebeGateway.Image.Tag)
	}
}

func TestBuildOverrides86_ExplicitGatewayOverridesLink(t *testing.T) {
	t.Setenv("ZEEBE_IMAGE_TAG", "8.7.99")
	t.Setenv("ZEEBE_GATEWAY_IMAGE_TAG", "8.7.50")

	overrides := buildOverrides86()

	if overrides.ZeebeGateway == nil {
		t.Fatal("expected ZeebeGateway override to be set")
	}
	if overrides.ZeebeGateway.Image.Tag != "8.7.50" {
		t.Errorf("expected explicit gateway tag 8.7.50, got %s", overrides.ZeebeGateway.Image.Tag)
	}
}

func TestBuildOverrides86_NoZeebeNoGateway(t *testing.T) {
	t.Setenv("ZEEBE_IMAGE_TAG", "")
	t.Setenv("ZEEBE_GATEWAY_IMAGE_TAG", "")

	overrides := buildOverrides86()

	if overrides.Zeebe != nil {
		t.Error("expected Zeebe override to be nil")
	}
	if overrides.ZeebeGateway != nil {
		t.Error("expected ZeebeGateway override to be nil")
	}
}
