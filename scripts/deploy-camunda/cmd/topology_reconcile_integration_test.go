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
// dockerRunDetached starts a container and returns its ID. Only stdout is
// read: on a first pull, docker writes pull progress to stderr.
func dockerRunDetached(ctx context.Context, t *testing.T, args ...string) string {
	t.Helper()
	var stderr bytes.Buffer
	cmd := exec.CommandContext(ctx, "docker", append([]string{"run", "-d"}, args...)...)
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("docker run: %v: %s", err, stderr.String())
	}
	id := strings.TrimSpace(string(out))
	t.Cleanup(func() { _ = exec.Command("docker", "rm", "-f", id).Run() })
	return id
}

func TestPsqlScript_RepairsAStaleRoleOnTheSupportedImage(t *testing.T) {
	if _, err := exec.LookPath("docker"); err != nil {
		t.Skip("docker not available")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()

	id := dockerRunDetached(ctx, t, "-e", "POSTGRES_USER=app", "-e", "POSTGRES_PASSWORD=initial-pw", "-e", "POSTGRES_DB=identity", postgresImage)

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

// elasticsearchImage matches the dogfood Elasticsearch companion's image.
const elasticsearchImage = "docker.elastic.co/elasticsearch/elasticsearch:8.18.0"

// TestEsScript_RotatesTheElasticPasswordOnTheSupportedImage runs esScript, via
// docker exec, against a secured single-node Elasticsearch. The new password
// carries JSON metacharacters to cover the request body's escaping.
func TestEsScript_RotatesTheElasticPasswordOnTheSupportedImage(t *testing.T) {
	if _, err := exec.LookPath("docker"); err != nil {
		t.Skip("docker not available")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Minute)
	defer cancel()

	id := dockerRunDetached(ctx, t, "-e", "discovery.type=single-node", "-e", "xpack.security.enabled=true",
		"-e", "xpack.security.http.ssl.enabled=false", "-e", "ELASTIC_PASSWORD=initial-pw",
		"-e", "ES_JAVA_OPTS=-Xms512m -Xmx512m", elasticsearchImage)

	const rotated = `rot"at\ed-pw`
	run := func(mode string, pws ...string) error {
		cmd := exec.CommandContext(ctx, "docker", "exec", "-i", id, "sh", "-c", esScript, "sh", mode, "http://localhost:9200", "elastic")
		cmd.Stdin = bytes.NewReader(lines(pws...))
		var stderr bytes.Buffer
		cmd.Stderr = &stderr
		err := cmd.Run()
		if err != nil && exitCode(err) != loginRejected {
			t.Logf("%s stderr: %s", mode, stderr.String())
		}
		return err
	}

	// The bootstrap password is accepted before the security index exists and
	// can briefly be rejected while it is created, so wait for a stable run.
	deadline := time.Now().Add(4 * time.Minute)
	for streak := 0; streak < 5; {
		if err := run("check", "initial-pw"); err == nil {
			streak++
		} else if time.Now().After(deadline) {
			t.Fatalf("elasticsearch never stably accepted the initial password: %v", err)
		} else {
			streak = 0
		}
		time.Sleep(3 * time.Second)
	}

	if err := run("check", rotated); exitCode(err) != loginRejected {
		t.Fatalf("check with a stale password: err = %v, want exit %d", err, loginRejected)
	}
	if err := run("set", "initial-pw", rotated); err != nil {
		t.Fatalf("set: %v", err)
	}
	if err := run("check", rotated); err != nil {
		t.Fatalf("check after set: %v", err)
	}
	if err := run("check", "initial-pw"); exitCode(err) != loginRejected {
		t.Fatalf("old password still accepted: err = %v", err)
	}
}

// keycloakImage matches the dogfood Keycloak companion's image.
const keycloakImage = "quay.io/keycloak/keycloak:26.3.3"

// TestKeycloakBootstrapReset_RecoversALostAdminOnTheSupportedImage reproduces
// the lost-admin recovery against Keycloak on PostgreSQL: a second container
// runs kc.sh with bootstrapAdminArgs against the shared database, and
// kcadmScript then resets the admin as the temporary admin and deletes it.
func TestKeycloakBootstrapReset_RecoversALostAdminOnTheSupportedImage(t *testing.T) {
	if _, err := exec.LookPath("docker"); err != nil {
		t.Skip("docker not available")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()
	docker := func(args ...string) string {
		t.Helper()
		out, err := exec.CommandContext(ctx, "docker", args...).CombinedOutput()
		if err != nil {
			t.Fatalf("docker %s: %v: %s", strings.Join(args[:2], " "), err, out)
		}
		return strings.TrimSpace(string(out))
	}

	network := docker("network", "create", "reconcile-kc-"+strings.ToLower(time.Now().Format("150405")))
	t.Cleanup(func() { _ = exec.Command("docker", "network", "rm", network).Run() })
	dockerRunDetached(ctx, t, "--network", network, "--network-alias", "kcdb",
		"-e", "POSTGRES_USER=keycloak", "-e", "POSTGRES_PASSWORD=db-pw", "-e", "POSTGRES_DB=keycloak", postgresImage)
	dbEnv := []string{"-e", "KC_DB=postgres", "-e", "KC_DB_URL=jdbc:postgresql://kcdb:5432/keycloak",
		"-e", "KC_DB_USERNAME=keycloak", "-e", "KC_DB_PASSWORD=db-pw"}
	kc := dockerRunDetached(ctx, t, append(append([]string{"--network", network}, dbEnv...),
		"-e", "KC_BOOTSTRAP_ADMIN_USERNAME=admin", "-e", "KC_BOOTSTRAP_ADMIN_PASSWORD=initial-pw",
		"-e", "KC_HTTP_RELATIVE_PATH=/auth", keycloakImage, "start-dev")...)

	const url = "http://localhost:8080/auth"
	kcadm := func(stdin []string, args ...string) (string, error) {
		cmd := exec.CommandContext(ctx, "docker", append([]string{"exec", "-i", kc, "sh", "-c", kcadmScript, "sh"}, args...)...)
		cmd.Stdin = bytes.NewReader(lines(stdin...))
		var stdout, stderr bytes.Buffer
		cmd.Stdout, cmd.Stderr = &stdout, &stderr
		err := cmd.Run()
		if err != nil && exitCode(err) != loginRejected {
			t.Logf("kcadm %s stderr: %s", args[0], stderr.String())
		}
		return strings.TrimSpace(stdout.String()), err
	}

	deadline := time.Now().Add(4 * time.Minute)
	for {
		if _, err := kcadm([]string{"initial-pw"}, "check", url, "admin"); err == nil {
			break
		} else if time.Now().After(deadline) {
			t.Fatalf("keycloak never accepted the initial admin password: %v", err)
		}
		time.Sleep(5 * time.Second)
	}
	if _, err := kcadm([]string{"initial-pw"}, "check", "http://localhost:1/auth", "admin"); err == nil || exitCode(err) == loginRejected {
		t.Fatalf("an unreachable server must fail, not report a rejected password: %v", err)
	}
	if res, err := kcadm([]string{"initial-pw", "lost-pw"}, "set-password", url, "admin", "master", "admin"); err != nil || res != "set" {
		t.Fatalf("lose the admin password: res=%q err=%v", res, err)
	}
	if _, err := kcadm([]string{"initial-pw"}, "check", url, "admin"); exitCode(err) != loginRejected {
		t.Fatalf("setup: initial password still accepted: %v", err)
	}

	bootstrap := exec.CommandContext(ctx, "docker", append(append(append([]string{"run", "--rm", "--network", network}, dbEnv...),
		"-e", "KC_CACHE=local", "-e", "KC_BOOTSTRAP_USER=temp-admin", "-e", "KC_BOOTSTRAP_PW=temp-pw", keycloakImage), bootstrapAdminArgs...)...)
	if out, err := bootstrap.CombinedOutput(); err != nil {
		t.Fatalf("bootstrap-admin: %v: %s", err, out)
	}

	if res, err := kcadm([]string{"temp-pw", `rot"ated\pw`}, "set-password", url, "temp-admin", "master", "admin"); err != nil || res != "set" {
		t.Fatalf("reset the admin as the temporary admin: res=%q err=%v", res, err)
	}
	if _, err := kcadm([]string{`rot"ated\pw`}, "check", url, "admin"); err != nil {
		t.Fatalf("admin rejects the reset password: %v", err)
	}
	if res, err := kcadm([]string{`rot"ated\pw`}, "delete-user", url, "admin", "master", "temp-admin"); err != nil || res != "deleted" {
		t.Fatalf("delete the temporary admin: res=%q err=%v", res, err)
	}
	if _, err := kcadm([]string{"temp-pw"}, "check", url, "temp-admin"); exitCode(err) != loginRejected {
		t.Fatalf("temporary admin still logs in: %v", err)
	}
}
