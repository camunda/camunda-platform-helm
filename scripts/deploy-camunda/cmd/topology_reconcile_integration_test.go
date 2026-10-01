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

//go:build integration

package cmd

import (
	"bytes"
	"context"
	"os/exec"
	"strings"
	"testing"
	"time"
)

// postgresImage matches internal-postgresql's and internal-keycloak-26's database image.
const postgresImage = "postgres:16-alpine"

// TestPsqlScript_RepairsAStaleRoleOnTheSupportedImage runs psqlScript, via
// docker exec, against a PostgreSQL initialised with one password: the new
// password must be rejected, set, and then accepted.
func TestPsqlScript_RepairsAStaleRoleOnTheSupportedImage(t *testing.T) {
	if _, err := exec.LookPath("docker"); err != nil {
		t.Skip("docker not available")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()

	out, err := exec.CommandContext(ctx, "docker", "run", "-d", "--rm",
		"-e", "POSTGRES_USER=app", "-e", "POSTGRES_PASSWORD=initial-pw", "-e", "POSTGRES_DB=identity",
		postgresImage).CombinedOutput()
	if err != nil {
		t.Fatalf("docker run: %v: %s", err, out)
	}
	id := strings.TrimSpace(string(out))
	t.Cleanup(func() { _ = exec.Command("docker", "rm", "-f", id).Run() })

	run := func(mode, pw string) error {
		cmd := exec.CommandContext(ctx, "docker", "exec", "-i", id, "sh", "-c", psqlScript, "sh", mode, "app", "identity")
		cmd.Stdin = bytes.NewReader(lines(pw))
		var stderr bytes.Buffer
		cmd.Stderr = &stderr
		err := cmd.Run()
		if err != nil && exitCode(err) != loginRejected {
			t.Logf("%s stderr: %s", mode, stderr.String())
		}
		return err
	}

	deadline := time.Now().Add(90 * time.Second)
	for {
		if err := run("check", "initial-pw"); err == nil {
			break
		} else if time.Now().After(deadline) {
			t.Fatalf("postgres never accepted the initial password: %v", err)
		}
		time.Sleep(2 * time.Second)
	}

	if err := run("check", "rotated-pw"); exitCode(err) != loginRejected {
		t.Fatalf("check with a stale password: err = %v, want exit %d", err, loginRejected)
	}
	if err := run("set", "rotated-pw"); err != nil {
		t.Fatalf("set: %v", err)
	}
	if err := run("check", "rotated-pw"); err != nil {
		t.Fatalf("check after set: %v", err)
	}
	if err := run("check", "initial-pw"); exitCode(err) != loginRejected {
		t.Fatalf("old password still accepted: err = %v", err)
	}
}
