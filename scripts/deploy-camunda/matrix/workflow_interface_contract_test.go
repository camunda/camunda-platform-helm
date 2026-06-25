// Copyright 2025 Camunda Services GmbH
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package matrix

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"testing"

	"gopkg.in/yaml.v3"
)

// integrationTemplatePath is the reusable workflow whose workflow_call interface
// is a public contract consumed by other repositories.
const integrationTemplatePath = ".github/workflows/test-integration-template.yaml"

// consumerContractFixture records how known downstream repos call the template.
const consumerContractFixture = "testdata/integration_template_consumers.yaml"

// templateInput is the contract-relevant shape of one workflow_call input.
type templateInput struct {
	required   bool
	typ        string
	hasDefault bool
}

// consumerContract is the parsed fixture: how each known caller invokes the
// reusable workflow.
type consumerContract struct {
	Consumers []struct {
		Repo     string   `yaml:"repo"`
		Workflow string   `yaml:"workflow"`
		Secrets  string   `yaml:"secrets"`
		Inputs   []string `yaml:"inputs"`
	} `yaml:"consumers"`
	ExpectedInputTypes map[string]string `yaml:"expectedInputTypes"`
}

// TestIntegrationTemplateConsumerContract guards the workflow_call interface of
// test-integration-template.yaml against changes that would break 3rd-party
// callers (camunda/camunda, camunda/connectors, camunda/camunda-hub). A reusable
// workflow's inputs are a public API: GitHub fails a caller's run if it passes an
// input the workflow no longer declares, if a required-without-default input is
// not supplied, or on a type mismatch. None of that surfaces in this repo's own
// CI — it only fails downstream — so this test brings the breakage forward.
//
// The known callers and the inputs they pass are recorded in the fixture
// (testdata/integration_template_consumers.yaml); refresh it when a caller
// changes how it invokes the workflow. All known callers use `secrets: inherit`,
// so secret-level changes are not contract-breaking and are not modeled here.
func TestIntegrationTemplateConsumerContract(t *testing.T) {
	repoRoot := findRepoRoot(t)

	inputs := loadTemplateInputs(t, filepath.Join(repoRoot, integrationTemplatePath))
	contract := loadConsumerContract(t, consumerContractFixture)

	if len(contract.Consumers) == 0 {
		t.Fatal("consumer contract fixture lists no consumers")
	}

	for _, msg := range checkConsumerContract(inputs, contract) {
		t.Error(msg)
	}
}

// checkConsumerContract returns one message per contract violation. It is pure
// (no I/O) so the breaking-change detection can be unit-tested directly.
func checkConsumerContract(inputs map[string]templateInput, contract consumerContract) []string {
	var violations []string

	// (A) Every input a caller passes must still be declared by the template.
	for _, c := range contract.Consumers {
		for _, in := range c.Inputs {
			if _, ok := inputs[in]; !ok {
				violations = append(violations, fmt.Sprintf(
					"[%s %s] passes input %q which test-integration-template.yaml no longer declares — "+
						"this breaks the caller's next run with an \"unexpected input\" error. Keep a "+
						"backward-compatible input (a deprecated no-op is fine) or coordinate the caller change first.",
					c.Repo, c.Workflow, in))
			}
		}
	}

	// (B) A required-without-default input must be passed by EVERY known caller.
	// Catches both new required inputs and optional -> required flips: any caller
	// that does not pass it would fail at dispatch.
	for _, meta := range sortedInputs(inputs) {
		if !meta.required || meta.hasDefault {
			continue
		}
		for _, c := range contract.Consumers {
			if !slices.Contains(c.Inputs, meta.name) {
				violations = append(violations, fmt.Sprintf(
					"template input %q is required with no default, but [%s %s] does not pass it — "+
						"the caller's next run fails. Give the input a default, or keep it optional.",
					meta.name, c.Repo, c.Workflow))
			}
		}
	}

	// (C) The type of an input callers pass must not change.
	for name, want := range contract.ExpectedInputTypes {
		meta, ok := inputs[name]
		if !ok {
			continue // already reported by (A)
		}
		if meta.typ != want {
			violations = append(violations, fmt.Sprintf(
				"template input %q changed type from %q to %q — callers passing it may break. "+
					"Revert the type, or update every caller and this fixture deliberately.",
				name, want, meta.typ))
		}
	}

	return violations
}

// inputWithName pairs an input name with its metadata for deterministic ordering.
type inputWithName struct {
	name string
	templateInput
}

// sortedInputs yields inputs in name order so violation output is stable.
func sortedInputs(inputs map[string]templateInput) []inputWithName {
	names := make([]string, 0, len(inputs))
	for n := range inputs {
		names = append(names, n)
	}
	sort.Strings(names)
	out := make([]inputWithName, 0, len(names))
	for _, n := range names {
		out = append(out, inputWithName{name: n, templateInput: inputs[n]})
	}
	return out
}

// loadTemplateInputs parses on.workflow_call.inputs from a workflow file using
// node navigation, which is robust to YAML resolving the bare `on` key as a
// boolean (the node's literal Value is still "on" either way).
func loadTemplateInputs(t *testing.T, path string) map[string]templateInput {
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	var root yaml.Node
	if err := yaml.Unmarshal(data, &root); err != nil {
		t.Fatalf("parse %s: %v", path, err)
	}

	on := mappingValue(&root, "on")
	if on == nil {
		t.Fatalf("%s: no top-level `on:` mapping", path)
	}
	workflowCall := mappingValue(on, "workflow_call")
	if workflowCall == nil {
		t.Fatalf("%s: `on.workflow_call` is missing — the workflow is no longer reusable", path)
	}
	inputsNode := mappingValue(workflowCall, "inputs")
	if inputsNode == nil || inputsNode.Kind != yaml.MappingNode {
		t.Fatalf("%s: `on.workflow_call.inputs` is missing or not a mapping", path)
	}

	out := make(map[string]templateInput)
	for i := 0; i+1 < len(inputsNode.Content); i += 2 {
		name := inputsNode.Content[i].Value
		spec := inputsNode.Content[i+1]
		ti := templateInput{}
		if r := mappingValue(spec, "required"); r != nil {
			ti.required = r.Value == "true"
		}
		if ty := mappingValue(spec, "type"); ty != nil {
			ti.typ = ty.Value
		}
		if d := mappingValue(spec, "default"); d != nil {
			ti.hasDefault = true
		}
		out[name] = ti
	}
	return out
}

// mappingValue returns the value node for key in a mapping node, unwrapping a
// document node. Matching on the literal Value tolerates differing YAML tags.
func mappingValue(node *yaml.Node, key string) *yaml.Node {
	if node == nil {
		return nil
	}
	if node.Kind == yaml.DocumentNode && len(node.Content) > 0 {
		node = node.Content[0]
	}
	if node.Kind != yaml.MappingNode {
		return nil
	}
	for i := 0; i+1 < len(node.Content); i += 2 {
		if node.Content[i].Value == key {
			return node.Content[i+1]
		}
	}
	return nil
}

func loadConsumerContract(t *testing.T, path string) consumerContract {
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	var c consumerContract
	if err := yaml.Unmarshal(data, &c); err != nil {
		t.Fatalf("parse %s: %v", path, err)
	}
	return c
}

// --- unit tests for the pure checker -------------------------------------

func makeContract() consumerContract {
	c := consumerContract{ExpectedInputTypes: map[string]string{
		"identifier": "string", "flag": "boolean",
	}}
	c.Consumers = append(c.Consumers, struct {
		Repo     string   `yaml:"repo"`
		Workflow string   `yaml:"workflow"`
		Secrets  string   `yaml:"secrets"`
		Inputs   []string `yaml:"inputs"`
	}{Repo: "camunda/x", Workflow: ".github/workflows/w.yml", Secrets: "inherit",
		Inputs: []string{"identifier", "flag"}})
	return c
}

func TestCheckConsumerContract_HealthyPasses(t *testing.T) {
	inputs := map[string]templateInput{
		"identifier": {required: true, typ: "string", hasDefault: false},
		"flag":       {required: false, typ: "boolean", hasDefault: true},
		"extra":      {required: false, typ: "string", hasDefault: true}, // additive optional: safe
	}
	if v := checkConsumerContract(inputs, makeContract()); len(v) != 0 {
		t.Errorf("expected no violations, got %v", v)
	}
}

func TestCheckConsumerContract_RemovedInputBreaks(t *testing.T) {
	inputs := map[string]templateInput{
		"identifier": {required: true, typ: "string"},
		// "flag" removed
	}
	if v := checkConsumerContract(inputs, makeContract()); len(v) == 0 {
		t.Error("removing an input a caller passes must be flagged")
	}
}

func TestCheckConsumerContract_NewRequiredInputBreaks(t *testing.T) {
	inputs := map[string]templateInput{
		"identifier": {required: true, typ: "string"},
		"flag":       {required: false, typ: "boolean", hasDefault: true},
		"newReq":     {required: true, typ: "string", hasDefault: false}, // no caller passes it
	}
	if v := checkConsumerContract(inputs, makeContract()); len(v) == 0 {
		t.Error("a required-without-default input no caller passes must be flagged")
	}
}

func TestCheckConsumerContract_TypeChangeBreaks(t *testing.T) {
	inputs := map[string]templateInput{
		"identifier": {required: true, typ: "string"},
		"flag":       {required: false, typ: "string", hasDefault: true}, // was boolean
	}
	if v := checkConsumerContract(inputs, makeContract()); len(v) == 0 {
		t.Error("changing the type of a consumed input must be flagged")
	}
}
