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
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/spf13/cobra"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"scripts/deploy-camunda/deploy"
	"scripts/deploy-camunda/matrix"
)

// kubectlFieldManager matches camunda-core/pkg/kube's server-side apply owner,
// so the deploy's own apply of the credentials-manifest never conflicts.
const kubectlFieldManager = "camunda-platform-helm"

// loginRejected is the exit code both exec scripts use for a refused password.
const loginRejected = 3

// kcadmScript runs in the Keycloak container. Positional args: mode, server
// URL, login user, then realm and user for set-password and delete-user.
// stdin: the login password, then for set-password the new password.
const kcadmScript = `set -eu
mode=$1 url=$2 login=$3
K=/opt/keycloak/bin/kcadm.sh
cfg=$(mktemp)
trap 'rm -f "$cfg"' EXIT
IFS= read -r lp
"$K" config credentials --config "$cfg" --server "$url" --realm master --user "$login" --password "$lp" >/dev/null 2>&1 || exit 3
[ "$mode" = check ] && exit 0
realm=$4 user=$5
id=$("$K" get users --config "$cfg" -r "$realm" -q username="$user" -q exact=true --fields id --format csv --noquotes)
if [ -z "$id" ]; then echo absent; exit 0; fi
case $mode in
set-password)
  IFS= read -r np
  "$K" set-password --config "$cfg" -r "$realm" --userid "$id" --new-password "$np"
  echo set ;;
delete-user)
  "$K" delete "users/$id" --config "$cfg" -r "$realm"
  echo deleted ;;
esac
`

// psqlScript runs in a PostgreSQL container. Positional args: mode, role,
// database. stdin: the password. check logs in over TCP to the pod's own IP:
// the image's pg_hba.conf trusts the local socket and loopback, so only a
// non-loopback address verifies the password. set uses the trusted socket.
const psqlScript = `set -eu
mode=$1 role=$2 db=$3
IFS= read -r pw
case $mode in
check)
  ip=$(hostname -i | cut -d' ' -f1)
  case $ip in ''|127.*|::1) echo "no non-loopback pod IP" >&2; exit 1 ;; esac
  PGPASSWORD="$pw" psql -h "$ip" -U "$role" -d "$db" -tAc 'select 1' >/dev/null 2>&1 || exit 3 ;;
set)
  export pw
  psql -v ON_ERROR_STOP=1 -q -h /var/run/postgresql -U "$role" -d "$db" -v role="$role" <<'SQL'
\getenv pw pw
ALTER ROLE :"role" PASSWORD :'pw';
SQL
  ;;
esac
`

// esScript runs in an Elasticsearch container. Positional args: mode, URL,
// user. stdin: the login password, then for set the new password.
const esScript = `set -eu
mode=$1 url=$2 user=$3
IFS= read -r lp
auth=$(printf '%s:%s' "$user" "$lp" | base64 | tr -d '\n')
code=$(curl -s -o /dev/null -w '%{http_code}' -H "Authorization: Basic $auth" "$url/_security/_authenticate")
case $code in 200) ;; 401) exit 3 ;; *) echo "authenticate returned HTTP $code" >&2; exit 1 ;; esac
[ "$mode" = check ] && exit 0
IFS= read -r np
body=$(printf '%s' "$np" | sed 's/\\/\\\\/g; s/"/\\"/g')
code=$(printf '{"password":"%s"}' "$body" | curl -s -o /dev/null -w '%{http_code}' -X POST -H "Authorization: Basic $auth" -H 'Content-Type: application/json' --data-binary @- "$url/_security/user/$user/_password")
[ "$code" = 200 ] || { echo "change password returned HTTP $code" >&2; exit 1; }
`

type kubectlFunc func(ctx context.Context, stdin []byte, args ...string) ([]byte, error)

type kubectlError struct {
	args   []string
	stderr string
	err    error
}

func (e *kubectlError) Error() string {
	return fmt.Sprintf("kubectl %s: %v: %s", strings.Join(e.args, " "), e.err, strings.TrimSpace(e.stderr))
}

func (e *kubectlError) Unwrap() error { return e.err }

func newKubectl(kubeContext string) kubectlFunc {
	return func(ctx context.Context, stdin []byte, args ...string) ([]byte, error) {
		full := args
		if kubeContext != "" {
			full = append([]string{"--context", kubeContext}, args...)
		}
		cmd := exec.CommandContext(ctx, "kubectl", full...)
		if stdin != nil {
			cmd.Stdin = bytes.NewReader(stdin)
		}
		var stdout, stderr bytes.Buffer
		cmd.Stdout, cmd.Stderr = &stdout, &stderr
		if err := cmd.Run(); err != nil {
			shown := args
			if len(shown) > 5 {
				shown = shown[:5]
			}
			return stdout.Bytes(), &kubectlError{args: shown, stderr: stderr.String(), err: err}
		}
		return stdout.Bytes(), nil
	}
}

func exitCode(err error) int {
	var coded interface{ ExitCode() int }
	if errors.As(err, &coded) {
		return coded.ExitCode()
	}
	return -1
}

func lines(values ...string) []byte {
	var b bytes.Buffer
	for _, v := range values {
		b.WriteString(v)
		b.WriteByte('\n')
	}
	return b.Bytes()
}

type credentialReconciler struct {
	kubectl  kubectlFunc
	previous map[string]string
	postgres []matrix.PostgresCredentialStore
	out      io.Writer
	poll     time.Duration
	timeout  time.Duration
	now      func() time.Time
	generate func() (string, error)
}

type reconcileInput struct {
	manifest   []byte
	src        credentialSource
	sourceNS   string
	namespaces []string
	hubNS      string
	stores     matrix.CredentialStores
}

func (r *credentialReconciler) secretData(ctx context.Context, ns, name string) (map[string]string, error) {
	raw, err := r.kubectl(ctx, nil, "get", "secret", name, "-n", ns, "--ignore-not-found", "-o", "json")
	if err != nil {
		return nil, err
	}
	if len(bytes.TrimSpace(raw)) == 0 {
		return nil, nil
	}
	var s struct {
		Data map[string]string `json:"data"`
	}
	if err := json.Unmarshal(raw, &s); err != nil {
		return nil, fmt.Errorf("decode secret %s/%s: %w", ns, name, err)
	}
	out := make(map[string]string, len(s.Data))
	for k, v := range s.Data {
		d, err := base64.StdEncoding.DecodeString(v)
		if err != nil {
			return nil, fmt.Errorf("decode secret %s/%s key %s: %w", ns, name, k, err)
		}
		out[k] = string(d)
	}
	return out, nil
}

func (r *credentialReconciler) exists(ctx context.Context, ns string, kindName ...string) (bool, error) {
	args := append([]string{"get"}, kindName...)
	if ns != "" {
		args = append(args, "-n", ns)
	}
	raw, err := r.kubectl(ctx, nil, append(args, "--ignore-not-found", "-o", "name")...)
	if err != nil {
		return false, err
	}
	return len(bytes.TrimSpace(raw)) > 0, nil
}

// reconcile sets every credential store to the value the credentials-manifest
// now hands the environment: it first syncs each namespace's ExternalSecrets
// to the source, then re-keys Keycloak (while it still reaches its database)
// and finally the PostgreSQL roles.
func (r *credentialReconciler) reconcile(ctx context.Context, in reconcileInput) error {
	source, err := r.secretData(ctx, in.sourceNS, in.src.Name)
	if err != nil {
		return err
	}
	if source == nil {
		return fmt.Errorf("source secret %s/%s does not exist; run ensure-credentials first", in.sourceNS, in.src.Name)
	}
	previousName := in.stores.Secret + "-previous"
	previous, err := r.secretData(ctx, in.hubNS, previousName)
	if err != nil {
		return err
	}
	if previous == nil {
		if previous, err = r.secretData(ctx, in.hubNS, in.stores.Secret); err != nil {
			return err
		}
		if len(previous) > 0 {
			if err := r.saveSecret(ctx, in.hubNS, previousName, previous); err != nil {
				return err
			}
		}
	}
	hubExists := false
	for _, ns := range in.namespaces {
		ok, err := r.exists(ctx, "", "namespace", ns)
		if err != nil {
			return err
		}
		if !ok {
			continue
		}
		if ns == in.hubNS {
			hubExists = true
		}
		if err := r.syncExternalSecrets(ctx, ns, in.manifest, in.src, source); err != nil {
			return err
		}
	}
	if !hubExists {
		fmt.Fprintf(r.out, "%s does not exist; no credential store to reconcile\n", in.hubNS)
		return nil
	}
	current, err := r.secretData(ctx, in.hubNS, in.stores.Secret)
	if err != nil {
		return err
	}
	r.previous = previous
	r.postgres = in.stores.Postgres
	value := func(key string) (string, error) {
		if current[key] == "" {
			return "", fmt.Errorf("secret %s/%s has no %s", in.hubNS, in.stores.Secret, key)
		}
		return current[key], nil
	}
	if k := in.stores.Keycloak; k != nil {
		if err := r.reconcileKeycloak(ctx, in.hubNS, *k, value, previous); err != nil {
			return err
		}
	}
	if e := in.stores.Elasticsearch; e != nil {
		if err := r.reconcileElasticsearch(ctx, in.hubNS, *e, value, previous); err != nil {
			return err
		}
	}
	for _, pg := range in.stores.Postgres {
		pw, err := value(pg.SecretKey)
		if err != nil {
			return err
		}
		if err := r.reconcilePostgres(ctx, in.hubNS, pg, pw); err != nil {
			return err
		}
	}
	if _, err := r.kubectl(ctx, nil, "delete", "secret", previousName, "-n", in.hubNS, "--ignore-not-found"); err != nil {
		return err
	}
	return nil
}

// saveSecret creates an Opaque Secret holding data. It is the pre-sync
// snapshot reconcile falls back to until a run completes, so a retry after a
// partial failure still has the values the stores were last set to.
func (r *credentialReconciler) saveSecret(ctx context.Context, ns, name string, data map[string]string) error {
	secret := corev1.Secret{
		TypeMeta:   metav1.TypeMeta{APIVersion: "v1", Kind: "Secret"},
		ObjectMeta: metav1.ObjectMeta{Name: name, Labels: map[string]string{"app.kubernetes.io/managed-by": "deploy-camunda"}},
		Type:       corev1.SecretTypeOpaque,
		StringData: data,
	}
	manifest, err := json.Marshal(secret)
	if err != nil {
		return err
	}
	_, err = r.kubectl(ctx, manifest, "create", "-n", ns, "-f", "-")
	return err
}

func (r *credentialReconciler) syncExternalSecrets(ctx context.Context, ns string, manifest []byte, src credentialSource, source map[string]string) error {
	if _, err := r.kubectl(ctx, manifest, "apply", "--server-side", "--force-conflicts", "--field-manager="+kubectlFieldManager, "-n", ns, "-f", "-"); err != nil {
		return err
	}
	stamp := fmt.Sprintf("force-sync=%d", r.now().Unix())
	for _, es := range src.ExternalSecrets {
		if _, err := r.kubectl(ctx, nil, "annotate", "externalsecret", es, "-n", ns, stamp, "--overwrite"); err != nil {
			return err
		}
	}
	targets := make([]string, 0, len(src.Targets))
	for t := range src.Targets {
		targets = append(targets, t)
	}
	sort.Strings(targets)
	deadline := r.now().Add(r.timeout)
	for {
		var stale []string
		for _, t := range targets {
			live, err := r.secretData(ctx, ns, t)
			if err != nil {
				return err
			}
			for key, prop := range src.Targets[t] {
				if live[key] != source[prop] {
					stale = append(stale, t+":"+key)
				}
			}
		}
		if len(stale) == 0 {
			fmt.Fprintf(r.out, "%s: credentials synced from source\n", ns)
			return nil
		}
		if !r.now().Before(deadline) {
			sort.Strings(stale)
			return fmt.Errorf("%s: ExternalSecrets did not sync %v from the source within %s", ns, stale, r.timeout)
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(r.poll):
		}
	}
}

func (r *credentialReconciler) kcadm(ctx context.Context, ns string, k matrix.KeycloakCredentialStore, stdin []byte, args ...string) (string, error) {
	full := append([]string{"exec", "-i", "-n", ns, "deployment/" + k.Deployment, "-c", k.Container, "--", "sh", "-c", kcadmScript, "sh"}, args...)
	out, err := r.kubectl(ctx, stdin, full...)
	return strings.TrimSpace(string(out)), err
}

func (r *credentialReconciler) kcLogin(ctx context.Context, ns string, k matrix.KeycloakCredentialStore, user, pw string) (bool, error) {
	_, err := r.kcadm(ctx, ns, k, lines(pw), "check", k.URL, user)
	switch {
	case err == nil:
		return true, nil
	case exitCode(err) == loginRejected:
		return false, nil
	default:
		return false, err
	}
}

func (r *credentialReconciler) kcSetPassword(ctx context.Context, ns string, k matrix.KeycloakCredentialStore, loginUser, loginPw, realm, user, pw string) (string, error) {
	out, err := r.kcadm(ctx, ns, k, lines(loginPw, pw), "set-password", k.URL, loginUser, realm, user)
	if exitCode(err) == loginRejected {
		return "", fmt.Errorf("keycloak rejected %s's password while setting %s/%s", loginUser, realm, user)
	}
	return out, err
}

func (r *credentialReconciler) reconcileKeycloak(ctx context.Context, ns string, k matrix.KeycloakCredentialStore, value func(string) (string, error), previous map[string]string) error {
	ok, err := r.exists(ctx, ns, "deployment", k.Deployment)
	if err != nil || !ok {
		if ok || err == nil {
			fmt.Fprintf(r.out, "%s: deployment/%s absent; skipping Keycloak\n", ns, k.Deployment)
		}
		return err
	}
	adminPw, err := value(k.AdminSecretKey)
	if err != nil {
		return err
	}
	current, err := r.kcLogin(ctx, ns, k, k.AdminUser, adminPw)
	if err != nil {
		return err
	}
	switch {
	case current:
		fmt.Fprintf(r.out, "%s: keycloak %s password current\n", ns, k.AdminUser)
	default:
		prev := previous[k.AdminSecretKey]
		usePrev := false
		if prev != "" && prev != adminPw {
			if usePrev, err = r.kcLogin(ctx, ns, k, k.AdminUser, prev); err != nil {
				return err
			}
		}
		if usePrev {
			if _, err := r.kcSetPassword(ctx, ns, k, k.AdminUser, prev, "master", k.AdminUser, adminPw); err != nil {
				return err
			}
			fmt.Fprintf(r.out, "%s: keycloak %s password rotated from its previous value\n", ns, k.AdminUser)
		} else {
			if err := r.resetKeycloakAdmin(ctx, ns, k, adminPw); err != nil {
				return err
			}
			fmt.Fprintf(r.out, "%s: keycloak %s password reset through a temporary bootstrap admin\n", ns, k.AdminUser)
		}
		if ok, err := r.kcLogin(ctx, ns, k, k.AdminUser, adminPw); err != nil || !ok {
			if err == nil {
				err = fmt.Errorf("%s: keycloak still rejects %s's password after the reset", ns, k.AdminUser)
			}
			return err
		}
	}
	for _, u := range k.Users {
		pw, err := value(u.SecretKey)
		if err != nil {
			return err
		}
		res, err := r.kcSetPassword(ctx, ns, k, k.AdminUser, adminPw, u.Realm, u.Username, pw)
		if err != nil {
			return err
		}
		if res == "absent" {
			fmt.Fprintf(r.out, "%s: keycloak user %s/%s absent; skipped\n", ns, u.Realm, u.Username)
			continue
		}
		fmt.Fprintf(r.out, "%s: keycloak user %s/%s password set\n", ns, u.Realm, u.Username)
	}
	return nil
}

// resetKeycloakAdmin runs `kc.sh bootstrap-admin user` in a separate pod built
// from the Keycloak Deployment's image and database settings, logs in as the
// temporary admin it creates to set adminPw, then deletes it. The live
// container's memory limit leaves no room for the second JVM.
func (r *credentialReconciler) resetKeycloakAdmin(ctx context.Context, ns string, k matrix.KeycloakCredentialStore, adminPw string) (retErr error) {
	raw, err := r.kubectl(ctx, nil, "get", "deployment", k.Deployment, "-n", ns, "-o", "json")
	if err != nil {
		return err
	}
	var d appsv1.Deployment
	if err := json.Unmarshal(raw, &d); err != nil {
		return fmt.Errorf("decode deployment/%s: %w", k.Deployment, err)
	}
	var src *corev1.Container
	for i := range d.Spec.Template.Spec.Containers {
		if d.Spec.Template.Spec.Containers[i].Name == k.Container {
			src = &d.Spec.Template.Spec.Containers[i]
		}
	}
	if src == nil {
		return fmt.Errorf("deployment/%s has no container %q", k.Deployment, k.Container)
	}
	suffix, err := r.generate()
	if err != nil {
		return err
	}
	tempPw, err := r.generate()
	if err != nil {
		return err
	}
	tempUser := "deploy-camunda-reconcile-" + strings.ToLower(suffix[:8])
	name := k.Deployment + "-reconcile-bootstrap"

	dbPw, err := r.acceptedDatabasePassword(ctx, ns, src.Env)
	if err != nil {
		return err
	}
	env := []corev1.EnvVar{{Name: "KC_CACHE", Value: "local"}}
	for _, e := range src.Env {
		if strings.HasPrefix(e.Name, "KC_DB") && (e.Name != "KC_DB_PASSWORD" || dbPw == "") {
			env = append(env, e)
		}
	}
	if dbPw != "" {
		env = append(env, corev1.EnvVar{Name: "KC_DB_PASSWORD", ValueFrom: &corev1.EnvVarSource{SecretKeyRef: &corev1.SecretKeySelector{
			LocalObjectReference: corev1.LocalObjectReference{Name: name}, Key: "db-password",
		}}})
	}
	for _, e := range []struct{ name, key string }{{"KC_BOOTSTRAP_USER", "username"}, {"KC_BOOTSTRAP_PW", "password"}} {
		env = append(env, corev1.EnvVar{Name: e.name, ValueFrom: &corev1.EnvVarSource{SecretKeyRef: &corev1.SecretKeySelector{
			LocalObjectReference: corev1.LocalObjectReference{Name: name}, Key: e.key,
		}}})
	}
	labels := map[string]string{"app.kubernetes.io/managed-by": "deploy-camunda", "app.kubernetes.io/component": "keycloak-reconcile"}
	secret := corev1.Secret{
		TypeMeta:   metav1.TypeMeta{APIVersion: "v1", Kind: "Secret"},
		ObjectMeta: metav1.ObjectMeta{Name: name, Labels: labels},
		StringData: map[string]string{"username": tempUser, "password": tempPw, "db-password": dbPw},
	}
	pod := corev1.Pod{
		TypeMeta:   metav1.TypeMeta{APIVersion: "v1", Kind: "Pod"},
		ObjectMeta: metav1.ObjectMeta{Name: name, Labels: labels},
		Spec: corev1.PodSpec{
			RestartPolicy:    corev1.RestartPolicyNever,
			ImagePullSecrets: d.Spec.Template.Spec.ImagePullSecrets,
			SecurityContext:  d.Spec.Template.Spec.SecurityContext,
			Containers: []corev1.Container{{
				Name:            "bootstrap-admin",
				Image:           src.Image,
				Args:            []string{"bootstrap-admin", "user", "--username:env", "KC_BOOTSTRAP_USER", "--password:env", "KC_BOOTSTRAP_PW", "--no-prompt"},
				Env:             env,
				SecurityContext: src.SecurityContext,
				Resources: corev1.ResourceRequirements{
					Requests: corev1.ResourceList{corev1.ResourceMemory: resource.MustParse("512Mi"), corev1.ResourceCPU: resource.MustParse("200m")},
					Limits:   corev1.ResourceList{corev1.ResourceMemory: resource.MustParse("1Gi")},
				},
			}},
		},
	}
	cleanup := func(c context.Context) error {
		_, err := r.kubectl(c, nil, "delete", "pod/"+name, "secret/"+name, "-n", ns, "--ignore-not-found", "--wait=true")
		return err
	}
	if err := cleanup(ctx); err != nil {
		return err
	}
	defer func() {
		c, cancel := context.WithTimeout(context.WithoutCancel(ctx), 2*time.Minute)
		defer cancel()
		if err := r.removeTempAdmin(c, ns, k, tempUser, tempPw, adminPw); err != nil {
			retErr = errors.Join(retErr, err)
		}
		if err := cleanup(c); err != nil {
			retErr = errors.Join(retErr, fmt.Errorf("%s: delete bootstrap pod and secret %s: %w", ns, name, err))
		}
	}()
	for _, obj := range []any{secret, pod} {
		manifest, err := json.Marshal(obj)
		if err != nil {
			return err
		}
		if _, err := r.kubectl(ctx, manifest, "create", "-n", ns, "-f", "-"); err != nil {
			return err
		}
	}
	deadline := r.now().Add(r.timeout)
	for {
		phase, err := r.kubectl(ctx, nil, "get", "pod", name, "-n", ns, "-o", "jsonpath={.status.phase}")
		if err != nil {
			return err
		}
		switch strings.TrimSpace(string(phase)) {
		case string(corev1.PodSucceeded):
			_, err := r.kcSetPassword(ctx, ns, k, tempUser, tempPw, "master", k.AdminUser, adminPw)
			return err
		case string(corev1.PodFailed):
			logs, _ := r.kubectl(ctx, nil, "logs", "pod/"+name, "-n", ns, "--tail=20")
			return fmt.Errorf("%s: bootstrap-admin pod failed:\n%s", ns, logs)
		}
		if !r.now().Before(deadline) {
			return fmt.Errorf("%s: bootstrap-admin pod did not finish within %s", ns, r.timeout)
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(r.poll):
		}
	}
}

func (r *credentialReconciler) es(ctx context.Context, ns string, e matrix.ElasticsearchCredential, stdin []byte, mode string) error {
	_, err := r.kubectl(ctx, stdin, "exec", "-i", "-n", ns, "statefulset/"+e.StatefulSet, "-c", e.Container, "--", "sh", "-c", esScript, "sh", mode, e.URL, e.User)
	return err
}

func (r *credentialReconciler) esLogin(ctx context.Context, ns string, e matrix.ElasticsearchCredential, pw string) (bool, error) {
	err := r.es(ctx, ns, e, lines(pw), "check")
	switch {
	case err == nil:
		return true, nil
	case exitCode(err) == loginRejected:
		return false, nil
	default:
		return false, err
	}
}

// reconcileElasticsearch sets the native user's password through the security
// API, authenticating with the namespace's previous value. Without a working
// previous value there is no non-interactive way in, so it fails with the
// manual step instead.
func (r *credentialReconciler) reconcileElasticsearch(ctx context.Context, ns string, e matrix.ElasticsearchCredential, value func(string) (string, error), previous map[string]string) error {
	ok, err := r.exists(ctx, ns, "statefulset", e.StatefulSet)
	if err != nil {
		return err
	}
	if !ok {
		fmt.Fprintf(r.out, "%s: statefulset/%s absent; skipping Elasticsearch\n", ns, e.StatefulSet)
		return nil
	}
	pw, err := value(e.SecretKey)
	if err != nil {
		return err
	}
	if current, err := r.esLogin(ctx, ns, e, pw); err != nil || current {
		if current {
			fmt.Fprintf(r.out, "%s: elasticsearch %s password current\n", ns, e.User)
		}
		return err
	}
	prev := previous[e.SecretKey]
	usable := false
	if prev != "" && prev != pw {
		if usable, err = r.esLogin(ctx, ns, e, prev); err != nil {
			return err
		}
	}
	if !usable {
		return fmt.Errorf("%s: elasticsearch rejects both the current and the previous %s password; reset it in %s-0 with "+
			"`bin/elasticsearch-reset-password -u %s -i` to the value of %s, then re-run", ns, e.User, e.StatefulSet, e.User, e.SecretKey)
	}
	if err := r.es(ctx, ns, e, lines(prev, pw), "set"); err != nil {
		return err
	}
	if ok, err := r.esLogin(ctx, ns, e, pw); err != nil || !ok {
		if err == nil {
			err = fmt.Errorf("%s: elasticsearch still rejects %s's password after the change", ns, e.User)
		}
		return err
	}
	fmt.Fprintf(r.out, "%s: elasticsearch %s password rotated from its previous value\n", ns, e.User)
	return nil
}

// removeTempAdmin deletes the bootstrap admin, logging in as the permanent
// admin or, if that password was never set, as the temporary admin itself. A
// temporary admin that no longer exists, or never got created, is not an error.
func (r *credentialReconciler) removeTempAdmin(ctx context.Context, ns string, k matrix.KeycloakCredentialStore, tempUser, tempPw, adminPw string) error {
	var last error
	for _, login := range []struct{ user, pw string }{{k.AdminUser, adminPw}, {tempUser, tempPw}} {
		res, err := r.kcadm(ctx, ns, k, lines(login.pw), "delete-user", k.URL, login.user, "master", tempUser)
		switch {
		case err == nil && (res == "deleted" || res == "absent"):
			return nil
		case err == nil:
			last = fmt.Errorf("unexpected kcadm output %q", res)
		case exitCode(err) == loginRejected:
			continue
		default:
			last = err
		}
	}
	if last == nil {
		return nil
	}
	return fmt.Errorf("%s: temporary keycloak admin %s may remain in the master realm; delete it: %w", ns, tempUser, last)
}

// acceptedDatabasePassword returns the value of Keycloak's KC_DB_PASSWORD
// secret key that its PostgreSQL store accepts right now: the synced value, or
// the previous one while the role is not yet rotated. It returns "" when no
// PostgreSQL store matches that key, leaving the env reference unchanged.
func (r *credentialReconciler) acceptedDatabasePassword(ctx context.Context, ns string, env []corev1.EnvVar) (string, error) {
	var key, secret string
	for _, e := range env {
		if e.Name == "KC_DB_PASSWORD" && e.ValueFrom != nil && e.ValueFrom.SecretKeyRef != nil {
			key, secret = e.ValueFrom.SecretKeyRef.Key, e.ValueFrom.SecretKeyRef.Name
		}
	}
	for _, pg := range r.postgres {
		if key == "" || pg.SecretKey != key {
			continue
		}
		current, err := r.secretData(ctx, ns, secret)
		if err != nil {
			return "", err
		}
		for _, candidate := range []string{current[key], r.previous[key]} {
			if candidate == "" {
				continue
			}
			err := r.psql(ctx, ns, pg, "check", candidate)
			if err == nil {
				return candidate, nil
			}
			if exitCode(err) != loginRejected {
				return "", err
			}
		}
		return "", fmt.Errorf("%s: postgres %s/%s accepts neither the current nor the previous %s; the bootstrap admin cannot reach the database", ns, pg.StatefulSet, pg.User, key)
	}
	return "", nil
}

func (r *credentialReconciler) psql(ctx context.Context, ns string, pg matrix.PostgresCredentialStore, mode, pw string) error {
	_, err := r.kubectl(ctx, lines(pw), "exec", "-i", "-n", ns, "statefulset/"+pg.StatefulSet, "--", "sh", "-c", psqlScript, "sh", mode, pg.User, pg.Database)
	return err
}

func (r *credentialReconciler) reconcilePostgres(ctx context.Context, ns string, pg matrix.PostgresCredentialStore, pw string) error {
	ok, err := r.exists(ctx, ns, "statefulset", pg.StatefulSet)
	if err != nil {
		return err
	}
	if !ok {
		fmt.Fprintf(r.out, "%s: statefulset/%s absent; skipping role %s\n", ns, pg.StatefulSet, pg.User)
		return nil
	}
	err = r.psql(ctx, ns, pg, "check", pw)
	if err == nil {
		fmt.Fprintf(r.out, "%s: postgres %s/%s password current\n", ns, pg.StatefulSet, pg.User)
		return nil
	}
	if exitCode(err) != loginRejected {
		return err
	}
	if err := r.psql(ctx, ns, pg, "set", pw); err != nil {
		return err
	}
	if err := r.psql(ctx, ns, pg, "check", pw); err != nil {
		return fmt.Errorf("%s: postgres %s/%s still rejects the password after ALTER ROLE: %w", ns, pg.StatefulSet, pg.User, err)
	}
	fmt.Fprintf(r.out, "%s: postgres %s/%s password set\n", ns, pg.StatefulSet, pg.User)
	return nil
}

// credentialsChecksum is a short digest of every property the manifest reads,
// stable across runs until a value changes.
func credentialsChecksum(src credentialSource, source map[string]string) (string, error) {
	h := sha256.New()
	for _, p := range src.Properties {
		v, ok := source[p]
		if !ok || v == "" {
			return "", fmt.Errorf("source secret %s has no %s; run ensure-credentials first", src.Name, p)
		}
		fmt.Fprintf(h, "%d:%s=%d:%s\n", len(p), p, len(v), v)
	}
	return hex.EncodeToString(h.Sum(nil))[:16], nil
}

type credentialsCommandFlags struct {
	topologyLifecycleFlags
	secretNamespace string
}

func (f *credentialsCommandFlags) bind(cmd *cobra.Command) {
	f.topologyLifecycleFlags.bind(cmd)
	cmd.Flags().StringVar(&f.secretNamespace, "secret-namespace", "distribution-team", "namespace the ClusterSecretStore reads source secrets from")
}

func (f *credentialsCommandFlags) load() (*matrix.Topology, []byte, credentialSource, error) {
	topology, err := loadScenarioTopology(f.repoRoot, f.version, f.scenario)
	if err != nil {
		return nil, nil, credentialSource{}, err
	}
	if topology.CredentialsManifest == "" {
		return nil, nil, credentialSource{}, fmt.Errorf("scenario %q declares no credentials-manifest", f.scenario)
	}
	raw, err := os.ReadFile(filepath.Join(f.repoRoot, topology.CredentialsManifest))
	if err != nil {
		return nil, nil, credentialSource{}, fmt.Errorf("read credentials manifest: %w", err)
	}
	manifest, err := matrix.RenderCredentialsManifest(raw, f.base)
	if err != nil {
		return nil, nil, credentialSource{}, err
	}
	src, err := parseCredentialSource(manifest)
	return topology, manifest, src, err
}

func newTopologyReconcileCredentialsCommand() *cobra.Command {
	var (
		f       credentialsCommandFlags
		timeout time.Duration
	)
	cmd := &cobra.Command{
		Use:   "reconcile-credentials",
		Short: "Set a deployed topology's credential stores to the credentials-manifest's current values",
		Long: `Applies the topology's credentials-manifest to every existing release namespace
and waits for its ExternalSecrets to sync from the source Secret. Then, in the
namespace named by the topology's credential-stores, it sets every store that
keeps the password it was initialised with to the synced value:

- the Keycloak master-realm admin: logs in with the synced value, else with the
  namespace's previous value, else creates a temporary admin with
  kc.sh bootstrap-admin in a separate pod and deletes it afterwards;
- each listed Keycloak realm user that exists;
- the Elasticsearch native user, through the security API with its previous
  value;
- each PostgreSQL role, through ALTER ROLE over the pod's local socket.

Stores already current are left alone and no value is ever printed. Run it
before the deploy; the deploy's credentials checksum restarts the consumers.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			topology, manifest, src, err := f.load()
			if err != nil {
				return err
			}
			if topology.CredentialStores == nil {
				fmt.Fprintf(cmd.OutOrStdout(), "scenario %q declares no credential-stores; nothing to reconcile\n", f.scenario)
				return nil
			}
			releases, err := f.releases()
			if err != nil {
				return err
			}
			namespaces := make([]string, 0, len(releases))
			for _, rel := range releases {
				namespaces = append(namespaces, rel.Namespace)
			}
			hubNS, err := deploy.DeriveReleaseNamespace(f.base, topology.CredentialStores.NamespaceSuffix)
			if err != nil {
				return err
			}
			r := &credentialReconciler{
				kubectl:  newKubectl(f.kubeContext),
				out:      cmd.OutOrStdout(),
				poll:     5 * time.Second,
				timeout:  timeout,
				now:      time.Now,
				generate: generateCredential,
			}
			return r.reconcile(cmd.Context(), reconcileInput{
				manifest:   manifest,
				src:        src,
				sourceNS:   f.secretNamespace,
				namespaces: namespaces,
				hubNS:      hubNS,
				stores:     *topology.CredentialStores,
			})
		},
	}
	f.bind(cmd)
	cmd.Flags().DurationVar(&timeout, "timeout", 5*time.Minute, "how long to wait for an ExternalSecret sync or the bootstrap-admin pod")
	return cmd
}

func newTopologyCredentialsChecksumCommand() *cobra.Command {
	var f credentialsCommandFlags
	cmd := &cobra.Command{
		Use:   "credentials-checksum",
		Short: "Print a digest of the credentials-manifest's source values, for pod annotations that restart consumers on rotation",
		RunE: func(cmd *cobra.Command, args []string) error {
			_, _, src, err := f.load()
			if err != nil {
				return err
			}
			r := &credentialReconciler{kubectl: newKubectl(f.kubeContext)}
			source, err := r.secretData(cmd.Context(), f.secretNamespace, src.Name)
			if err != nil {
				return err
			}
			sum, err := credentialsChecksum(src, source)
			if err != nil {
				return err
			}
			fmt.Fprintln(cmd.OutOrStdout(), sum)
			return nil
		},
	}
	f.bind(cmd)
	return cmd
}
