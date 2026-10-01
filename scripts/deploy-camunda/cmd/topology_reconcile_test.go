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
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"

	"scripts/deploy-camunda/matrix"
)

const reconcileManifest = `
apiVersion: external-secrets.io/v1
kind: ExternalSecret
metadata:
  name: creds-es
spec:
  target:
    name: creds
  data:
    - secretKey: kc-admin
      remoteRef: {key: src, property: kc-admin}
    - secretKey: kc-db
      remoteRef: {key: src, property: kc-db}
    - secretKey: app-db
      remoteRef: {key: src, property: app-db}
    - secretKey: demo
      remoteRef: {key: src, property: demo}
    - secretKey: es
      remoteRef: {key: src, property: es}
`

type exitErr int

func (e exitErr) Error() string { return fmt.Sprintf("exit status %d", int(e)) }
func (e exitErr) ExitCode() int { return int(e) }

// fakeCluster models the few kubectl behaviours reconcile relies on:
// ExternalSecret sync on annotate, Keycloak logins and users, PostgreSQL role
// passwords, and a bootstrap-admin pod that creates its temporary admin.
type fakeCluster struct {
	namespaces map[string]bool
	secrets    map[string]map[string]string
	resources  map[string]bool
	esoSync    bool
	src        credentialSource
	sourceRef  string
	kc         map[string]string
	pg         map[string]string
	pods       map[string]string
	bootNames  []string
	bootDBPw   string
	failES     bool
	failPG     bool
	es         string
	sets       int
	bootstraps int
	applies    int
}

func newFakeCluster(t *testing.T) *fakeCluster {
	src, err := parseCredentialSource([]byte(reconcileManifest))
	if err != nil {
		t.Fatal(err)
	}
	return &fakeCluster{
		namespaces: map[string]bool{"env-hub": true, "env-plain": true},
		secrets: map[string]map[string]string{
			"distribution-team/src": {"kc-admin": "new-admin", "kc-db": "new-kcdb", "app-db": "new-appdb", "demo": "new-demo", "es": "new-es"},
			"env-hub/creds":         {"kc-admin": "old-admin", "kc-db": "old-kcdb", "app-db": "old-appdb", "demo": "old-demo", "es": "old-es"},
			"env-plain/creds":       {"kc-admin": "old-admin", "kc-db": "old-kcdb", "app-db": "old-appdb", "demo": "old-demo", "es": "old-es"},
		},
		resources: map[string]bool{
			"env-hub/deployment/keycloak":             true,
			"env-hub/statefulset/keycloak-postgresql": true,
			"env-hub/statefulset/postgresql":          true,
			"env-hub/statefulset/elasticsearch":       true,
		},
		esoSync:   true,
		src:       src,
		sourceRef: "distribution-team/src",
		kc:        map[string]string{"master/admin": "old-admin", "camunda-platform/demo": "old-demo"},
		pg:        map[string]string{"env-hub/keycloak-postgresql/keycloak": "old-kcdb", "env-hub/postgresql/app": "old-appdb"},
		pods:      map[string]string{},
		es:        "old-es",
	}
}

func flagValue(args []string, flag string) string {
	for i, a := range args {
		if a == flag && i+1 < len(args) {
			return args[i+1]
		}
	}
	return ""
}

// scriptArgs returns the positional args after `sh -c <script> sh`.
func scriptArgs(args []string) []string {
	for i := 0; i+3 < len(args); i++ {
		if args[i] == "sh" && args[i+1] == "-c" {
			return args[i+4:]
		}
	}
	return nil
}

func (f *fakeCluster) kubectl(_ context.Context, stdin []byte, args ...string) ([]byte, error) {
	ns := flagValue(args, "-n")
	in := strings.Split(strings.TrimSuffix(string(stdin), "\n"), "\n")
	switch args[0] {
	case "get":
		switch args[1] {
		case "namespace":
			if f.namespaces[args[2]] {
				return []byte("namespace/" + args[2]), nil
			}
			return nil, nil
		case "secret":
			data, ok := f.secrets[ns+"/"+args[2]]
			if !ok {
				return nil, nil
			}
			enc := map[string]string{}
			for k, v := range data {
				enc[k] = base64.StdEncoding.EncodeToString([]byte(v))
			}
			return json.Marshal(map[string]any{"data": enc})
		case "pod":
			return []byte(f.pods[ns+"/"+args[2]]), nil
		case "deployment", "statefulset":
			if !f.resources[ns+"/"+args[1]+"/"+args[2]] {
				return nil, nil
			}
			if flagValue(args, "-o") == "json" {
				return []byte(`{"spec":{"template":{"spec":{"containers":[{"name":"keycloak","image":"kc:26","env":[{"name":"KC_DB_URL","value":"jdbc:x"},{"name":"KC_DB_PASSWORD","valueFrom":{"secretKeyRef":{"name":"creds","key":"kc-db"}}},{"name":"KC_HOSTNAME","value":"h"}]}]}}}}`), nil
			}
			return []byte(args[1] + "/" + args[2]), nil
		}
	case "apply":
		var obj struct {
			Kind     string                `json:"kind"`
			Metadata struct{ Name string } `json:"metadata"`
			Data     map[string]string     `json:"stringData"`
		}
		if json.Unmarshal(stdin, &obj) == nil && obj.Kind == "Secret" {
			f.secrets[ns+"/"+obj.Metadata.Name] = obj.Data
			return nil, nil
		}
		f.applies++
		return nil, nil
	case "annotate":
		if f.esoSync {
			for target, keys := range f.src.Targets {
				live := f.secrets[ns+"/"+target]
				for key, prop := range keys {
					live[key] = f.secrets[f.sourceRef][prop]
				}
			}
		}
		return nil, nil
	case "create":
		var obj struct {
			Kind     string                `json:"kind"`
			Metadata struct{ Name string } `json:"metadata"`
			Data     map[string]string     `json:"stringData"`
			Spec     corev1.PodSpec        `json:"spec"`
		}
		if err := json.Unmarshal(stdin, &obj); err != nil {
			return nil, err
		}
		if obj.Kind == "Secret" {
			f.secrets[ns+"/"+obj.Metadata.Name] = obj.Data
			return nil, nil
		}
		f.bootstraps++
		f.bootNames = append(f.bootNames, obj.Metadata.Name)
		tmp := f.secrets[ns+"/"+obj.Metadata.Name]
		for _, e := range obj.Spec.Containers[0].Env {
			if e.Name == "KC_DB_PASSWORD" && e.ValueFrom != nil && e.ValueFrom.SecretKeyRef != nil {
				f.bootDBPw = f.secrets[ns+"/"+e.ValueFrom.SecretKeyRef.Name][e.ValueFrom.SecretKeyRef.Key]
			}
		}
		if f.bootDBPw != f.pg[ns+"/keycloak-postgresql/keycloak"] {
			f.pods[ns+"/"+obj.Metadata.Name] = "Failed"
			return nil, nil
		}
		f.kc["master/"+tmp["username"]] = tmp["password"]
		f.pods[ns+"/"+obj.Metadata.Name] = "Succeeded"
		return nil, nil
	case "delete":
		if args[1] == "secret" {
			delete(f.secrets, ns+"/"+args[2])
			return nil, nil
		}
		for _, a := range args[1:] {
			if name, ok := strings.CutPrefix(a, "pod/"); ok {
				delete(f.pods, ns+"/"+name)
			}
			if name, ok := strings.CutPrefix(a, "secret/"); ok {
				delete(f.secrets, ns+"/"+name)
			}
		}
		return nil, nil
	case "logs":
		return nil, nil
	case "exec":
		pos := scriptArgs(args)
		target := args[4]
		if target == "statefulset/elasticsearch" {
			if f.es != in[0] {
				return nil, exitErr(loginRejected)
			}
			if pos[0] == "set" {
				if f.failES {
					return nil, fmt.Errorf("injected elasticsearch failure")
				}
				f.sets++
				f.es = in[1]
			}
			return nil, nil
		}
		if strings.HasPrefix(target, "deployment/") {
			mode, login := pos[0], pos[2]
			if pw, ok := f.kc["master/"+login]; !ok || pw != in[0] {
				return nil, exitErr(loginRejected)
			}
			if mode == "check" {
				return nil, nil
			}
			key := pos[3] + "/" + pos[4]
			if _, ok := f.kc[key]; !ok {
				return []byte("absent\n"), nil
			}
			if mode == "delete-user" {
				delete(f.kc, key)
				return []byte("deleted\n"), nil
			}
			f.sets++
			f.kc[key] = in[1]
			return []byte("set\n"), nil
		}
		key := ns + "/" + strings.TrimPrefix(target, "statefulset/") + "/" + pos[1]
		if pos[0] == "check" {
			if f.pg[key] != in[0] {
				return nil, exitErr(loginRejected)
			}
			return nil, nil
		}
		if f.failPG {
			return nil, fmt.Errorf("injected postgres failure")
		}
		f.sets++
		f.pg[key] = in[0]
		return nil, nil
	}
	return nil, fmt.Errorf("unexpected kubectl %v", args)
}

func testStores() matrix.CredentialStores {
	return matrix.CredentialStores{
		NamespaceSuffix: "hub",
		Secret:          "creds",
		Postgres: []matrix.PostgresCredentialStore{
			{StatefulSet: "keycloak-postgresql", User: "keycloak", Database: "keycloak", SecretKey: "kc-db"},
			{StatefulSet: "postgresql", User: "app", Database: "identity", SecretKey: "app-db"},
		},
		Keycloak: &matrix.KeycloakCredentialStore{
			Deployment: "keycloak", Container: "keycloak", URL: "http://localhost:8080/auth",
			AdminUser: "admin", AdminSecretKey: "kc-admin",
			Users: []matrix.KeycloakUserCredential{{Realm: "camunda-platform", Username: "demo", SecretKey: "demo"}},
		},
		Elasticsearch: &matrix.ElasticsearchCredential{
			StatefulSet: "elasticsearch", Container: "elasticsearch", URL: "http://localhost:9200", User: "elastic", SecretKey: "es",
		},
	}
}

func runReconcile(t *testing.T, f *fakeCluster) (string, error) {
	t.Helper()
	var out bytes.Buffer
	clock := time.Unix(1_790_000_000, 0)
	r := &credentialReconciler{
		kubectl:  f.kubectl,
		out:      &out,
		poll:     time.Millisecond,
		timeout:  time.Minute,
		now:      func() time.Time { clock = clock.Add(10 * time.Second); return clock },
		generate: generateCredential,
	}
	err := r.reconcile(context.Background(), reconcileInput{
		manifest:   []byte(reconcileManifest),
		src:        f.src,
		sourceNS:   "distribution-team",
		namespaces: []string{"env-hub", "env-plain", "env-mt"},
		hubNS:      "env-hub",
		stores:     testStores(),
	})
	return out.String(), err
}

func assertNoValues(t *testing.T, out string) {
	t.Helper()
	for _, v := range []string{"old-admin", "new-admin", "old-kcdb", "new-kcdb", "old-appdb", "new-appdb", "old-demo", "new-demo", "old-es", "new-es"} {
		if strings.Contains(out, v) {
			t.Fatalf("output leaks credential %q:\n%s", v, out)
		}
	}
}

func TestReconcileCredentials_RotatesEveryStoreToTheSource(t *testing.T) {
	f := newFakeCluster(t)
	out, err := runReconcile(t, f)
	if err != nil {
		t.Fatalf("reconcile: %v\n%s", err, out)
	}
	want := map[string]string{"master/admin": "new-admin", "camunda-platform/demo": "new-demo"}
	for k, v := range want {
		if f.kc[k] != v {
			t.Errorf("keycloak %s not rotated", k)
		}
	}
	if f.pg["env-hub/keycloak-postgresql/keycloak"] != "new-kcdb" || f.pg["env-hub/postgresql/app"] != "new-appdb" {
		t.Errorf("postgres roles not rotated: %v", f.pg)
	}
	if f.es != "new-es" {
		t.Error("elasticsearch password not rotated")
	}
	if f.secrets["env-plain/creds"]["kc-admin"] != "new-admin" {
		t.Error("non-hub namespace was not synced")
	}
	if f.applies != 2 {
		t.Errorf("applies = %d, want one per existing namespace (2)", f.applies)
	}
	if f.bootstraps != 0 {
		t.Errorf("bootstraps = %d, want 0 when the previous admin password still works", f.bootstraps)
	}
	if !strings.Contains(out, "rotated from its previous value") {
		t.Errorf("output = %q", out)
	}
	assertNoValues(t, out)
}

func TestReconcileCredentials_ResetsAdminThroughBootstrapWhenNoKnownPasswordWorks(t *testing.T) {
	f := newFakeCluster(t)
	f.kc["master/admin"] = "unknown"
	out, err := runReconcile(t, f)
	if err != nil {
		t.Fatalf("reconcile: %v\n%s", err, out)
	}
	if f.bootstraps != 1 || f.kc["master/admin"] != "new-admin" {
		t.Fatalf("bootstraps=%d admin rotated=%v", f.bootstraps, f.kc["master/admin"] == "new-admin")
	}
	for k := range f.kc {
		if strings.Contains(k, "deploy-camunda-reconcile-") {
			t.Errorf("temporary admin %s was left behind", k)
		}
	}
	if len(f.pods) != 0 {
		t.Errorf("bootstrap pod left behind: %v", f.pods)
	}
	for k := range f.secrets {
		if strings.HasPrefix(k, "env-hub/keycloak-reconcile-") {
			t.Errorf("bootstrap secret %s left behind", k)
		}
	}
	assertNoValues(t, out)
}

func TestReconcileCredentials_RemovesTemporaryAdminWhenTheResetFails(t *testing.T) {
	f := newFakeCluster(t)
	f.kc["master/admin"] = "unknown"
	api := f.kubectl
	f2 := func(ctx context.Context, stdin []byte, args ...string) ([]byte, error) {
		if pos := scriptArgs(args); len(pos) > 2 && pos[0] == "set-password" && strings.HasPrefix(pos[2], "deploy-camunda-reconcile-") {
			return nil, fmt.Errorf("injected set-password failure")
		}
		return api(ctx, stdin, args...)
	}
	var out bytes.Buffer
	clock := time.Unix(1_790_000_000, 0)
	r := &credentialReconciler{kubectl: f2, out: &out, poll: time.Millisecond, timeout: time.Minute,
		now: func() time.Time { clock = clock.Add(10 * time.Second); return clock }, generate: generateCredential}
	err := r.reconcile(context.Background(), reconcileInput{manifest: []byte(reconcileManifest), src: f.src, sourceNS: "distribution-team",
		namespaces: []string{"env-hub"}, hubNS: "env-hub", stores: testStores()})
	if err == nil || !strings.Contains(err.Error(), "injected") {
		t.Fatalf("err = %v, want the injected failure", err)
	}
	for k := range f.kc {
		if strings.Contains(k, "deploy-camunda-reconcile-") {
			t.Errorf("temporary admin %s left behind after a failed reset", k)
		}
	}
	if len(f.pods) != 0 {
		t.Errorf("bootstrap pod left behind: %v", f.pods)
	}
}

func TestReconcileCredentials_ElasticsearchWithoutAWorkingPasswordFailsWithTheManualStep(t *testing.T) {
	f := newFakeCluster(t)
	f.es = "unknown"
	_, err := runReconcile(t, f)
	if err == nil || !strings.Contains(err.Error(), "elasticsearch-reset-password") || strings.Contains(err.Error(), "new-es") {
		t.Fatalf("err = %v", err)
	}
}

func TestReconcileCredentials_BootstrapUsesTheDatabasePasswordPostgresStillAccepts(t *testing.T) {
	f := newFakeCluster(t)
	f.kc["master/admin"] = "unknown"
	out, err := runReconcile(t, f)
	if err != nil {
		t.Fatalf("reconcile: %v\n%s", err, out)
	}
	if f.bootDBPw != "old-kcdb" {
		t.Errorf("bootstrap pod got a database password Postgres did not accept (match old=%v)", f.bootDBPw == "old-kcdb")
	}
	if f.pg["env-hub/keycloak-postgresql/keycloak"] != "new-kcdb" {
		t.Error("keycloak role not rotated after the bootstrap")
	}
}

func TestReconcileCredentials_RetryAfterAPartialFailureKeepsThePreviousValues(t *testing.T) {
	f := newFakeCluster(t)
	f.failES = true
	if _, err := runReconcile(t, f); err == nil || !strings.Contains(err.Error(), "injected elasticsearch failure") {
		t.Fatalf("first run err = %v, want the injected failure", err)
	}
	if f.secrets["env-hub/creds"]["es"] != "new-es" || f.es != "old-es" {
		t.Fatal("setup: the first run should have synced the namespace but left elasticsearch on the old value")
	}
	if f.secrets["env-hub/creds-previous"]["g0.es"] != "old-es" {
		t.Fatal("the pre-sync snapshot was not kept after the failure")
	}
	f.failES = false
	out, err := runReconcile(t, f)
	if err != nil {
		t.Fatalf("retry: %v\n%s", err, out)
	}
	if f.es != "new-es" {
		t.Error("retry did not rotate elasticsearch from the snapshot")
	}
	if _, kept := f.secrets["env-hub/creds-previous"]; kept {
		t.Error("snapshot must be removed after a successful run")
	}
	assertNoValues(t, out)
}

func TestReconcileCredentials_RetryAfterTheSourceRotatesAgainUsesTheValueAStoreWasLeftOn(t *testing.T) {
	f := newFakeCluster(t)
	f.failPG = true
	if _, err := runReconcile(t, f); err == nil || !strings.Contains(err.Error(), "injected postgres failure") {
		t.Fatalf("first run err = %v", err)
	}
	if f.es != "new-es" || f.kc["master/admin"] != "new-admin" {
		t.Fatal("setup: keycloak and elasticsearch should have moved to the first rotation's values")
	}
	src := f.secrets["distribution-team/src"]
	src["es"], src["kc-admin"], src["kc-db"] = "third-es", "third-admin", "third-kcdb"
	f.failPG = false
	out, err := runReconcile(t, f)
	if err != nil {
		t.Fatalf("retry: %v\n%s", err, out)
	}
	if f.es != "third-es" || f.kc["master/admin"] != "third-admin" || f.pg["env-hub/keycloak-postgresql/keycloak"] != "third-kcdb" {
		t.Errorf("retry did not reach the second rotation: es=%v admin=%v", f.es == "third-es", f.kc["master/admin"] == "third-admin")
	}
	if f.bootstraps != 0 {
		t.Errorf("bootstraps = %d, want 0: the admin's intermediate value was known", f.bootstraps)
	}
	for _, v := range []string{"third-es", "third-admin", "third-kcdb"} {
		if strings.Contains(out, v) {
			t.Fatalf("output leaks %q", v)
		}
	}
}

func TestReconcileCredentials_RepeatedFailedRotationsKeepEveryIntermediateValue(t *testing.T) {
	f := newFakeCluster(t)
	src := f.secrets["distribution-team/src"]

	f.failPG = true
	if _, err := runReconcile(t, f); err == nil {
		t.Fatal("A->B: want the injected postgres failure")
	}
	if f.es != "new-es" {
		t.Fatal("setup: elasticsearch should be on B")
	}

	src["es"], src["kc-admin"] = "c-es", "c-admin"
	f.failES = true
	if _, err := runReconcile(t, f); err == nil {
		t.Fatal("B->C: want the injected elasticsearch failure")
	}
	if f.es != "new-es" {
		t.Fatal("setup: elasticsearch should still be on B")
	}

	src["es"], src["kc-admin"] = "d-es", "d-admin"
	f.failES, f.failPG = false, false
	out, err := runReconcile(t, f)
	if err != nil {
		t.Fatalf("D retry: %v\n%s", err, out)
	}
	if f.es != "d-es" || f.kc["master/admin"] != "d-admin" {
		t.Errorf("D retry did not reach D: es=%v admin=%v", f.es == "d-es", f.kc["master/admin"] == "d-admin")
	}
	if _, kept := f.secrets["env-hub/creds-previous"]; kept {
		t.Error("snapshot must be removed after a successful run")
	}
}

func TestReconcileCredentials_FailsWhenTheSourceChangesDuringTheRun(t *testing.T) {
	f := newFakeCluster(t)
	api := f.kubectl
	execs := 0
	f2 := func(ctx context.Context, stdin []byte, args ...string) ([]byte, error) {
		if args[0] == "exec" {
			execs++
			if execs == 1 {
				f.secrets["distribution-team/src"]["es"] = "concurrent-es"
			}
		}
		return api(ctx, stdin, args...)
	}
	var out bytes.Buffer
	clock := time.Unix(1_790_000_000, 0)
	var sum string
	r := &credentialReconciler{kubectl: f2, out: &out, poll: time.Millisecond, timeout: time.Minute,
		now: func() time.Time { clock = clock.Add(10 * time.Second); return clock }, generate: generateCredential}
	err := r.reconcile(context.Background(), reconcileInput{checksum: &sum, manifest: []byte(reconcileManifest), src: f.src,
		sourceNS: "distribution-team", namespaces: []string{"env-hub"}, hubNS: "env-hub", stores: testStores()})
	if err == nil || !strings.Contains(err.Error(), "changed during reconcile (es)") || strings.Contains(err.Error(), "concurrent-es") {
		t.Fatalf("err = %v", err)
	}
	if _, kept := f.secrets["env-hub/creds-previous"]; !kept {
		t.Error("the snapshot must survive a run that did not finish")
	}
}

func TestGenerations_RoundTripAndDeduplicate(t *testing.T) {
	gens := []map[string]string{{"a": "1", "b": "2"}, {"a": "3", "b": "4"}}
	back := splitGenerations(joinGenerations(gens))
	if len(back) != 2 || back[0]["a"] != "1" || back[1]["b"] != "4" {
		t.Fatalf("round trip = %v", back)
	}
	if !containsGeneration(back, map[string]string{"a": "3", "b": "4"}) || containsGeneration(back, map[string]string{"a": "3"}) {
		t.Error("containsGeneration mismatch")
	}
	if got := splitGenerations(map[string]string{"legacy": "x", "gx.y": "z"}); len(got) != 0 {
		t.Errorf("non-generation keys must be ignored, got %v", got)
	}
}

func TestReconcileCredentials_LeavesCurrentStoresAlone(t *testing.T) {
	f := newFakeCluster(t)
	for k, v := range f.secrets["distribution-team/src"] {
		f.secrets["env-hub/creds"][k] = v
	}
	f.kc["master/admin"] = "new-admin"
	f.pg["env-hub/keycloak-postgresql/keycloak"] = "new-kcdb"
	f.pg["env-hub/postgresql/app"] = "new-appdb"
	f.es = "new-es"
	out, err := runReconcile(t, f)
	if err != nil {
		t.Fatalf("reconcile: %v\n%s", err, out)
	}
	if f.sets != 1 {
		t.Errorf("sets = %d, want only the managed user's idempotent set", f.sets)
	}
	if f.bootstraps != 0 {
		t.Error("bootstrap must not run when the admin password is current")
	}
}

func TestReconcileCredentials_FreshEnvironmentIsANoOp(t *testing.T) {
	f := newFakeCluster(t)
	f.namespaces = map[string]bool{}
	out, err := runReconcile(t, f)
	if err != nil {
		t.Fatal(err)
	}
	if f.applies != 0 || f.sets != 0 || !strings.Contains(out, "no credential store to reconcile") {
		t.Errorf("applies=%d sets=%d out=%q", f.applies, f.sets, out)
	}
}

func TestReconcileCredentials_FailsWithoutSource(t *testing.T) {
	f := newFakeCluster(t)
	delete(f.secrets, "distribution-team/src")
	if _, err := runReconcile(t, f); err == nil || !strings.Contains(err.Error(), "run ensure-credentials first") {
		t.Fatalf("err = %v", err)
	}
}

func TestReconcileCredentials_FailsBeforeTouchingStoresWhenExternalSecretsDoNotSync(t *testing.T) {
	f := newFakeCluster(t)
	f.esoSync = false
	_, err := runReconcile(t, f)
	if err == nil || !strings.Contains(err.Error(), "did not sync") || !strings.Contains(err.Error(), "creds:kc-admin") {
		t.Fatalf("err = %v", err)
	}
	if strings.Contains(err.Error(), "old-") || strings.Contains(err.Error(), "new-") {
		t.Fatalf("error leaks a value: %v", err)
	}
	if f.sets != 0 || f.kc["master/admin"] != "old-admin" {
		t.Error("stores must not change when the sync fails")
	}
}

func TestReconcileCredentials_SkipsAbsentStores(t *testing.T) {
	f := newFakeCluster(t)
	delete(f.kc, "camunda-platform/demo")
	delete(f.resources, "env-hub/statefulset/postgresql")
	out, err := runReconcile(t, f)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "camunda-platform/demo absent") || !strings.Contains(out, "statefulset/postgresql absent") {
		t.Errorf("output = %q", out)
	}
}

func TestPsqlScript_ChecksOverANonLoopbackAddress(t *testing.T) {
	check := psqlScript[strings.Index(psqlScript, "check)"):strings.Index(psqlScript, "set)")]
	if strings.Contains(check, "127.0.0.1") || strings.Contains(check, "localhost") || strings.Contains(check, "/var/run/postgresql") {
		t.Fatalf("check must not connect over the socket or loopback, which pg_hba.conf trusts without a password:\n%s", check)
	}
	if !strings.Contains(check, "hostname -i") {
		t.Fatalf("check must connect to the pod IP:\n%s", check)
	}
}

func TestReconcileCredentials_ChecksumMatchesTheSourceItReconciledAgainst(t *testing.T) {
	f := newFakeCluster(t)
	want, err := credentialsChecksum(f.src, f.secrets["distribution-team/src"])
	if err != nil {
		t.Fatal(err)
	}
	var got string
	var out bytes.Buffer
	clock := time.Unix(1_790_000_000, 0)
	r := &credentialReconciler{kubectl: f.kubectl, out: &out, poll: time.Millisecond, timeout: time.Minute,
		now: func() time.Time { clock = clock.Add(10 * time.Second); return clock }, generate: generateCredential}
	if err := r.reconcile(context.Background(), reconcileInput{checksum: &got, manifest: []byte(reconcileManifest), src: f.src,
		sourceNS: "distribution-team", namespaces: []string{"env-hub"}, hubNS: "env-hub", stores: testStores()}); err != nil {
		t.Fatal(err)
	}
	if got != want {
		t.Errorf("checksum = %q, want %q", got, want)
	}
}

func TestReconcileCredentials_EachBootstrapUsesItsOwnResourceNames(t *testing.T) {
	f := newFakeCluster(t)
	for i := 0; i < 2; i++ {
		f.kc["master/admin"] = "unknown"
		if out, err := runReconcile(t, f); err != nil {
			t.Fatalf("run %d: %v\n%s", i, err, out)
		}
	}
	if len(f.bootNames) != 2 || f.bootNames[0] == f.bootNames[1] {
		t.Fatalf("bootstrap names = %v, want two distinct names", f.bootNames)
	}
	for _, n := range f.bootNames {
		if len(n) > 63 || strings.ToLower(n) != n {
			t.Errorf("%q is not a valid pod name", n)
		}
	}
}

func TestReconcileCredentials_RejectsAControlCharacterWithoutPrintingIt(t *testing.T) {
	f := newFakeCluster(t)
	f.secrets["distribution-team/src"]["kc-admin"] = "line-one\nline-two"
	_, err := runReconcile(t, f)
	if err == nil || !strings.Contains(err.Error(), "property kc-admin contains a control character") {
		t.Fatalf("err = %v", err)
	}
	if strings.Contains(err.Error(), "line-one") {
		t.Fatal("error leaks the value")
	}
	if f.kc["master/admin"] != "old-admin" || f.sets != 0 || f.applies != 0 {
		t.Error("nothing may be synced or changed when a source value cannot be transported")
	}
}

func TestPriorValues_SkipsUntransportableAndDuplicateValues(t *testing.T) {
	r := &credentialReconciler{prior: []map[string]string{{"k": "a"}, {"k": "bad\tvalue"}, {"k": "a"}, {"k": "b"}, nil}}
	if got := strings.Join(r.priorValues("k", "b"), ","); got != "a" {
		t.Errorf("priorValues = %q, want a", got)
	}
}

func TestCredentialsChecksum(t *testing.T) {
	src := credentialSource{Name: "src", Properties: []string{"a", "b"}}
	one, err := credentialsChecksum(src, map[string]string{"a": "1", "b": "2", "unused": "x"})
	if err != nil {
		t.Fatal(err)
	}
	same, _ := credentialsChecksum(src, map[string]string{"a": "1", "b": "2"})
	other, _ := credentialsChecksum(src, map[string]string{"a": "1", "b": "3"})
	if one != same || one == other || len(one) != 16 {
		t.Errorf("one=%s same=%s other=%s", one, same, other)
	}
	if _, err := credentialsChecksum(src, map[string]string{"a": "1"}); err == nil {
		t.Error("want an error for a missing property")
	}
}

func TestDogfoodCredentialStores_KeysExistInTheManifest(t *testing.T) {
	repoRoot := filepath.Join("..", "..", "..")
	topology, err := loadScenarioTopology(repoRoot, "8.10", "dogfood")
	if err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(filepath.Join(repoRoot, topology.CredentialsManifest))
	if err != nil {
		t.Fatal(err)
	}
	manifest, err := matrix.RenderCredentialsManifest(raw, "dogfood")
	if err != nil {
		t.Fatal(err)
	}
	src, err := parseCredentialSource(manifest)
	if err != nil {
		t.Fatal(err)
	}
	s := topology.CredentialStores
	if s == nil {
		t.Fatal("dogfood declares no credential-stores")
	}
	keys := src.Targets[s.Secret]
	if keys == nil {
		t.Fatalf("manifest writes no secret %q", s.Secret)
	}
	want := []string{s.Keycloak.AdminSecretKey}
	for _, pg := range s.Postgres {
		want = append(want, pg.SecretKey)
	}
	for _, u := range s.Keycloak.Users {
		want = append(want, u.SecretKey)
	}
	if s.Elasticsearch != nil {
		want = append(want, s.Elasticsearch.SecretKey)
	}
	for _, k := range want {
		if keys[k] == "" {
			t.Errorf("credential-stores key %q is not written by the manifest", k)
		}
	}
}
