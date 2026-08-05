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
	"strings"
	"testing"
)

func TestSeedAndVerificationShareStableFixtureIDs(t *testing.T) {
	for _, id := range []string{fixtureWorkspace, containerFolder, nestedProject, nestedIDP, rootProject, looseFolder, looseFile} {
		if !strings.Contains(seedSQL(), id) {
			t.Fatalf("seed SQL does not contain fixture ID %q", id)
		}
		if !strings.Contains(transitionalVerificationSQL(), id) || !strings.Contains(finalVerificationSQL(), id) {
			t.Fatalf("verification SQL does not contain fixture ID %q", id)
		}
	}
}

func TestVerificationChecksMigrationProvenance(t *testing.T) {
	sql := transitionalVerificationSQL() + finalVerificationSQL()
	for _, expected := range []string{"ws_migration_original_parent_id", "ws_migration_original_folder_id", migrationUser, "ROOT"} {
		if !strings.Contains(sql, expected) {
			t.Fatalf("verification SQL does not check %q", expected)
		}
	}
}
