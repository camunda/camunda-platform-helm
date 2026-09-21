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

package cmd

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"scripts/deploy-camunda/matrix"
)

func TestCIE2EPartitionsWritesSM89JSON(t *testing.T) {
	repoRoot, err := filepath.Abs("../../..")
	if err != nil {
		t.Fatal(err)
	}
	command := newCIE2EPartitionsCommand()
	var output bytes.Buffer
	command.SetOut(&output)
	command.SetArgs([]string{"--repo-root", repoRoot, "--version", "8.9"})
	if err := command.Execute(); err != nil {
		t.Fatalf("execute e2e-partitions: %v", err)
	}

	var partitions []matrix.E2EPartition
	if err := json.Unmarshal(output.Bytes(), &partitions); err != nil {
		t.Fatalf("decode output: %v", err)
	}
	if len(partitions) != 9 {
		t.Fatalf("partitions len = %d, want 9 active partitions", len(partitions))
	}
	wantIDs := []string{"standard-v2", "tasklist-v1", "rba", "mt", "document", "license", "mcp", "migration", "opensearch"}
	for i, want := range wantIDs {
		if partitions[i].ID != want {
			t.Errorf("partition %d id = %q, want %q", i, partitions[i].ID, want)
		}
		if !partitions[i].QA || !partitions[i].ImageTags {
			t.Errorf("partition %q must resolve a QA image-tags scenario", partitions[i].ID)
		}
	}
	if partitions[1].PlaywrightProject != "full-suite-v1" {
		t.Errorf("tasklist-v1 project = %q", partitions[1].PlaywrightProject)
	}
	if partitions[6].Features != "mcp" || !partitions[6].MCPGatewayEnabled {
		t.Errorf("mcp partition = %+v", partitions[6])
	}
	if partitions[7].Flow != "upgrade-minor" || !partitions[7].IsMigration || partitions[7].FilePattern != "migration-path-user-flows.spec.ts" {
		t.Errorf("migration partition = %+v", partitions[7])
	}
}

func TestSM89PartitionWorkflowWiresMigrationPreparation(t *testing.T) {
	repoRoot, err := filepath.Abs("../../..")
	if err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(repoRoot, ".github/workflows/test-sm-8-9-e2e-partitions.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"pre-upgrade-migration: ${{ matrix.partition.is_migration }}",
		"e2e-file-pattern: ${{ matrix.partition.file_pattern }}",
	} {
		if !bytes.Contains(data, []byte(want)) {
			t.Errorf("workflow does not contain %q", want)
		}
	}
	if bytes.Contains(data, []byte("blocked-reason")) || !bytes.Contains(data, []byte("Nine active partitions")) {
		t.Error("workflow does not report all nine runnable partitions")
	}

	template, err := os.ReadFile(filepath.Join(repoRoot, ".github/workflows/test-integration-template.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	runner, err := os.ReadFile(filepath.Join(repoRoot, ".github/workflows/test-integration-runner.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"pre-upgrade-migration:",
		"Run migration smoke tests on previous version",
		"playwright-project: migration-smoke",
		"file-pattern: smoke-tests.spec.ts",
		"is-migration: \"true\"",
		"deploy-camunda ci migration-data",
		"PRE_UPGRADE_ARGS=(--upgrade-phase upgrade)",
	} {
		if !bytes.Contains(template, []byte(want)) && !bytes.Contains(runner, []byte(want)) {
			t.Errorf("reusable migration workflow does not contain %q", want)
		}
	}
}

func TestCIE2EPartitionsRejectsUnknownVersion(t *testing.T) {
	repoRoot, err := filepath.Abs("../../..")
	if err != nil {
		t.Fatal(err)
	}
	command := newCIE2EPartitionsCommand()
	command.SilenceErrors = true
	command.SilenceUsage = true
	command.SetArgs([]string{"--repo-root", repoRoot, "--version", "8.8"})
	if err := command.Execute(); err == nil {
		t.Fatal("expected missing 8.8 partition registry to fail")
	}
}
