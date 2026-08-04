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
	"encoding/base64"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

const (
	fixtureUser      = "upgrade-user"
	fixtureWorkspace = "upgrade-workspace"
	containerFolder  = "upgrade-container"
	nestedProject    = "upgrade-nested-project"
	nestedIDP        = "upgrade-nested-idp"
	rootProject      = "upgrade-root-project"
	looseFolder      = "upgrade-loose-folder"
	looseFile        = "upgrade-loose-file"
	migrationUser    = "66666666-6666-6666-6666-666666666666"
)

type options struct {
	namespace string
	release   string
	chartPath string
}

func main() {
	var opts options
	flag.StringVar(&opts.namespace, "namespace", "", "Kubernetes namespace")
	flag.StringVar(&opts.release, "release", "integration", "Helm release name")
	flag.StringVar(&opts.chartPath, "chart-path", "", "8.10 chart path")
	flag.Parse()

	if opts.namespace == "" || flag.NArg() != 1 {
		fail(fmt.Errorf("usage: hub-migration-integration [flags] seed|verify-and-activate"))
	}

	var err error
	switch flag.Arg(0) {
	case "seed":
		err = opts.seed()
	case "verify-and-activate":
		err = opts.verifyAndActivate()
	default:
		err = fmt.Errorf("unknown command %q", flag.Arg(0))
	}
	fail(err)
}

func (o options) seed() error {
	password, err := secretValue(o.namespace, "integration-test-credentials", "webmodeler-postgresql-admin-password")
	if err != nil {
		return err
	}
	target := "statefulset/" + o.release + "-postgresql-web-modeler"
	if err := psql(o.namespace, target, "postgres", "web-modeler", password, seedSQL()); err != nil {
		return fmt.Errorf("seed 8.9 Web Modeler fixture: %w", err)
	}
	fmt.Println("seeded representative 8.9 Hub migration fixture")
	return nil
}

func (o options) verifyAndActivate() error {
	username := os.Getenv("RDBMS_POSTGRESQL_USERNAME")
	password := os.Getenv("RDBMS_POSTGRESQL_PASSWORD")
	if username == "" || password == "" {
		return fmt.Errorf("RDBMS_POSTGRESQL_USERNAME and RDBMS_POSTGRESQL_PASSWORD are required")
	}
	target := "deployment/postgresql"
	result, err := psqlOutput(o.namespace, target, username, "webmodeler", password,
		"SELECT to_regclass('public.hub_projects') IS NOT NULL;")
	if err != nil {
		return fmt.Errorf("detect migrated Hub schema: %w", err)
	}
	verification := transitionalVerificationSQL()
	if strings.TrimSpace(string(result)) == "t" {
		verification = finalVerificationSQL()
	}
	if err := psql(o.namespace, target, username, "webmodeler", password, verification); err != nil {
		return fmt.Errorf("verify 8.10 Hub migration fixture: %w", err)
	}
	if o.chartPath == "" {
		return fmt.Errorf("--chart-path is required for activation")
	}
	if err := run(nil, "helm", "upgrade", o.release, o.chartPath,
		"--namespace", o.namespace, "--reuse-values",
		"--set", "camundaHub.upgrade.phase=normal", "--wait", "--timeout", "15m"); err != nil {
		return fmt.Errorf("activate normal Hub phase: %w", err)
	}
	fmt.Println("verified migrated data and activated normal Hub phase")
	return nil
}

func seedSQL() string {
	return fmt.Sprintf(`
INSERT INTO users (id, iam_id, name, username, created, updated)
VALUES ('%[1]s', 'iam-%[1]s', 'Upgrade User', '%[1]s', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP);

INSERT INTO projects (id, organization_id, name, created, created_by, updated, updated_by)
VALUES ('%[2]s', 'upgrade-org', 'Upgrade Workspace', CURRENT_TIMESTAMP, '%[1]s', CURRENT_TIMESTAMP, '%[1]s');

INSERT INTO folders (id, name, project_id, parent_id, created, created_by, updated, updated_by) VALUES
('%[3]s', 'Container', '%[2]s', NULL, CURRENT_TIMESTAMP, '%[1]s', CURRENT_TIMESTAMP, '%[1]s'),
('%[4]s', 'Nested Project', '%[2]s', '%[3]s', CURRENT_TIMESTAMP, '%[1]s', CURRENT_TIMESTAMP, '%[1]s'),
('%[5]s', 'Root Project', '%[2]s', NULL, CURRENT_TIMESTAMP, '%[1]s', CURRENT_TIMESTAMP, '%[1]s'),
('%[6]s', 'Loose Folder', '%[2]s', NULL, CURRENT_TIMESTAMP, '%[1]s', CURRENT_TIMESTAMP, '%[1]s');

INSERT INTO process_applications (id) VALUES ('%[4]s'), ('%[5]s');

INSERT INTO files (id, name, project_id, folder_id, content, revision, process_id, type, created, created_by, updated, updated_by)
VALUES ('%[7]s', 'Loose File', '%[2]s', NULL, 'content', 1, 'upgrade-process', 'BPMN', CURRENT_TIMESTAMP, '%[1]s', CURRENT_TIMESTAMP, '%[1]s');

INSERT INTO idp_applications (id, name, project_id, folder_id, cluster_id, created, created_by, updated, updated_by)
VALUES ('%[8]s', 'Nested IDP', '%[2]s', '%[3]s', 'cluster-1', CURRENT_TIMESTAMP, '%[1]s', CURRENT_TIMESTAMP, '%[1]s');
`, fixtureUser, fixtureWorkspace, containerFolder, nestedProject, rootProject, looseFolder, looseFile, nestedIDP)
}

func transitionalVerificationSQL() string {
	return fmt.Sprintf(`
DO $$
DECLARE
  catch_all_id text;
BEGIN
  SELECT f.id INTO STRICT catch_all_id
  FROM folders f
  JOIN process_applications pa ON pa.id = f.id
  WHERE f.project_id = '%[1]s' AND f.created_by = '%[9]s';

  IF NOT EXISTS (SELECT 1 FROM folders WHERE id = '%[2]s' AND parent_id IS NULL AND ws_migration_original_parent_id = '%[3]s') THEN
    RAISE EXCEPTION 'nested project was not lifted with provenance';
  END IF;
  IF NOT EXISTS (SELECT 1 FROM idp_applications WHERE id = '%[4]s' AND folder_id IS NULL AND ws_migration_original_folder_id = '%[3]s') THEN
    RAISE EXCEPTION 'nested IDP application was not lifted with provenance';
  END IF;
  IF NOT EXISTS (SELECT 1 FROM files WHERE id = '%[5]s' AND folder_id = catch_all_id AND ws_migration_original_folder_id = 'ROOT') THEN
    RAISE EXCEPTION 'loose file was not moved into catch-all with provenance';
  END IF;
  IF NOT EXISTS (SELECT 1 FROM folders WHERE id = '%[6]s' AND parent_id = catch_all_id AND ws_migration_original_parent_id = 'ROOT') THEN
    RAISE EXCEPTION 'loose folder was not moved into catch-all with provenance';
  END IF;
  IF NOT EXISTS (SELECT 1 FROM folders WHERE id = '%[7]s' AND parent_id = catch_all_id AND ws_migration_original_parent_id = 'ROOT') THEN
    RAISE EXCEPTION 'container emptied by lift was not moved into catch-all';
  END IF;
  IF NOT EXISTS (SELECT 1 FROM folders WHERE id = '%[8]s' AND parent_id IS NULL AND ws_migration_original_parent_id IS NULL) THEN
    RAISE EXCEPTION 'root project was unexpectedly changed';
  END IF;
END $$;
`, fixtureWorkspace, nestedProject, containerFolder, nestedIDP, looseFile, looseFolder, containerFolder, rootProject, migrationUser)
}

func finalVerificationSQL() string {
	return fmt.Sprintf(`
DO $$
DECLARE
  catch_all_id text;
BEGIN
  SELECT id INTO STRICT catch_all_id
  FROM hub_projects
  WHERE project_id = '%[1]s' AND created_by = '%[9]s';

  IF NOT EXISTS (SELECT 1 FROM hub_projects WHERE id = '%[2]s' AND ws_migration_original_parent_id = '%[3]s') THEN
    RAISE EXCEPTION 'nested project was not extracted with provenance';
  END IF;
  IF NOT EXISTS (SELECT 1 FROM idp_applications WHERE id = '%[4]s' AND folder_id IS NULL AND ws_migration_original_folder_id = '%[3]s') THEN
    RAISE EXCEPTION 'nested IDP application was not lifted with provenance';
  END IF;
  IF NOT EXISTS (SELECT 1 FROM files WHERE id = '%[5]s' AND hub_project_id = catch_all_id AND folder_id IS NULL AND ws_migration_original_folder_id = 'ROOT') THEN
    RAISE EXCEPTION 'loose file was not moved into catch-all project root';
  END IF;
  IF NOT EXISTS (SELECT 1 FROM folders WHERE id = '%[6]s' AND hub_project_id = catch_all_id AND parent_id IS NULL AND ws_migration_original_parent_id = 'ROOT') THEN
    RAISE EXCEPTION 'loose folder was not moved into catch-all project root';
  END IF;
  IF NOT EXISTS (SELECT 1 FROM folders WHERE id = '%[7]s' AND hub_project_id = catch_all_id AND parent_id IS NULL AND ws_migration_original_parent_id = 'ROOT') THEN
    RAISE EXCEPTION 'container emptied by lift was not moved into catch-all project root';
  END IF;
  IF NOT EXISTS (SELECT 1 FROM hub_projects WHERE id = '%[8]s' AND ws_migration_original_parent_id IS NULL) THEN
    RAISE EXCEPTION 'root project was unexpectedly changed';
  END IF;
END $$;
`, fixtureWorkspace, nestedProject, containerFolder, nestedIDP, looseFile, looseFolder, containerFolder, rootProject, migrationUser)
}

func secretValue(namespace, name, key string) (string, error) {
	data, err := output("kubectl", "-n", namespace, "get", "secret", name, "-o", "json")
	if err != nil {
		return "", err
	}
	var secret struct {
		Data map[string]string `json:"data"`
	}
	if err := json.Unmarshal(data, &secret); err != nil {
		return "", err
	}
	decoded, err := base64.StdEncoding.DecodeString(secret.Data[key])
	return string(decoded), err
}

func psql(namespace, target, username, database, password, sql string) error {
	return run([]byte(sql), "kubectl", "-n", namespace, "exec", "-i", target, "--",
		"env", "PGPASSWORD="+password, "psql", "--set", "ON_ERROR_STOP=1",
		"--username", username, "--dbname", database)
}

func psqlOutput(namespace, target, username, database, password, sql string) ([]byte, error) {
	return output("kubectl", "-n", namespace, "exec", target, "--",
		"env", "PGPASSWORD="+password, "psql", "--set", "ON_ERROR_STOP=1",
		"--tuples-only", "--no-align", "--username", username, "--dbname", database,
		"--command", sql)
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

func fail(err error) {
	if err != nil {
		fmt.Fprintln(os.Stderr, strings.TrimSpace(err.Error()))
		os.Exit(1)
	}
}
