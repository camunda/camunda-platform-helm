package matrix

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"scripts/deploy-camunda/auth0"
	"scripts/deploy-camunda/credentials"
)

func TestRunStateLifecycleAndResume(t *testing.T) {
	root := t.TempDir()
	entries := []Entry{
		{Version: "8.10", Shortname: "one", Scenario: "first", Flow: "install", Platform: "gke"},
		{Version: "8.10", Shortname: "two", Scenario: "second", Flow: "install", Platform: "gke"},
	}
	opts := RunOptions{RepoRoot: "/repo", NamespacePrefix: "test", KubeContext: "cluster"}
	store := NewRunStateStore(root, "run-1")

	_, err := store.Create(entries, opts)
	require.NoError(t, err)
	require.NoError(t, store.Start(entries[0], "test-810-one"))
	require.NoError(t, store.Phase(entries[0], "deploying"))
	require.NoError(t, store.Complete(entries[0], RunResult{Entry: entries[0], Namespace: "test-810-one", KubeContext: "cluster"}))
	require.NoError(t, store.Start(entries[1], "test-810-two"))
	require.NoError(t, store.MarkInterrupted())

	state, err := store.Load()
	require.NoError(t, err)
	assert.Equal(t, RunPassed, state.Entries[0].Status)
	assert.Equal(t, "complete", state.Entries[0].Phase)
	assert.Equal(t, RunInterrupted, state.Entries[1].Status)
	assert.Equal(t, "RUN_INTERRUPTED", state.Entries[1].Failure.Code)

	resumed, restored, err := store.PrepareResume("")
	require.NoError(t, err)
	require.Len(t, resumed, 1)
	assert.Equal(t, "two", resumed[0].Shortname)
	assert.Equal(t, "/repo", restored.RepoRoot)
	assert.Equal(t, "cluster", restored.KubeContext)

	info, err := os.Stat(filepath.Join(root, "run-1", "matrix-state.json"))
	require.NoError(t, err)
	assert.Equal(t, os.FileMode(0o600), info.Mode().Perm())
}

func TestRunStateRedactsSecrets(t *testing.T) {
	entry := Entry{Version: "8.10", Shortname: "one", Scenario: "first", Flow: "install"}
	opts := RunOptions{
		DockerPassword: "registry-password",
		ExtraHelmSets:  []string{"global.password=super-secret", "global.host=example.test"},
		ExtraHelmArgs:  []string{"--set=identity.secret.inlineSecret=super-secret"},
	}
	store := NewRunStateStore(t.TempDir(), "run-1")
	_, err := store.Create([]Entry{entry}, opts)
	require.NoError(t, err)

	data, err := os.ReadFile(filepath.Join(store.RunDir(), "matrix-state.json"))
	require.NoError(t, err)
	text := string(data)
	assert.NotContains(t, text, "registry-password")
	assert.NotContains(t, text, "super-secret")
	assert.Contains(t, text, "global.host=example.test")
}

func TestRunStatePreservesBenignHelmOverride(t *testing.T) {
	stored := StoreRunOptions(RunOptions{ExtraHelmSets: []string{"feature.enabled=true"}, ExtraHelmArgs: []string{"--atomic"}})
	assert.Equal(t, []string{"feature.enabled=true"}, stored.ExtraHelmSets)
	assert.Equal(t, []string{"--atomic"}, stored.ExtraHelmArgs)
}

func TestRunStateRedactsUnknownHelmOverride(t *testing.T) {
	stored := StoreRunOptions(RunOptions{ExtraHelmSets: []string{"auth=secret-value"}})
	assert.Equal(t, []string{"<redacted>"}, stored.ExtraHelmSets)
}

func TestRunStateRequiresOnlyEnabledRegistryCredentials(t *testing.T) {
	stored := StoreRunOptions(RunOptions{DockerUsername: "incidental", DockerPassword: "incidental-password"})
	assert.False(t, stored.RequiresDockerPassword)
	stored = StoreRunOptions(RunOptions{EnsureDockerRegistry: true})
	assert.True(t, stored.RequiresDockerPassword)
}

func TestReplayCommandIncludesExecutionTarget(t *testing.T) {
	entry := Entry{Version: "8.10", Shortname: "one", Scenario: "first", Flow: "install", Platform: "gke"}
	command := ReplayCommand(entry, RunOptions{
		RepoRoot: "/repo", NamespacePrefix: "test", KubeContexts: map[string]string{"gke": "cluster"},
		IngressBaseDomains: map[string]string{"gke": "example.test"}, ChartRef: "/tmp/chart.tgz", TestE2E: true,
	})
	joined := strings.Join(command, " ")
	assert.Contains(t, joined, "--kube-context-gke cluster")
	assert.Contains(t, joined, "--ingress-base-domain-gke example.test")
	assert.Contains(t, joined, "--chart-ref /tmp/chart.tgz")
	assert.Contains(t, joined, "--test-e2e")
}

func TestReplayCommandIncludesUpgradeSource(t *testing.T) {
	entry := Entry{Version: "8.10", Shortname: "one", Flow: "upgrade-minor"}
	command := strings.Join(ReplayCommand(entry, RunOptions{UpgradeFromVersion: "13.5.0"}), " ")
	assert.Contains(t, command, "--upgrade-from-version 13.5.0")
}

func TestValidateRunIDRejectsTraversal(t *testing.T) {
	for _, id := range []string{"", "..", "../other", "/tmp/run", "a/b"} {
		assert.Error(t, ValidateRunID(id), id)
	}
	assert.NoError(t, ValidateRunID("20260729T120000Z"))
}

func TestRunStateRefusesConcurrentLock(t *testing.T) {
	store := NewRunStateStore(t.TempDir(), "run-1")
	first, err := store.Acquire()
	require.NoError(t, err)
	defer first.Close()

	_, err = NewRunStateStore(store.root, "run-1").Acquire()
	require.ErrorContains(t, err, "active in another process")
}

func TestRunStateReclaimsStaleLock(t *testing.T) {
	store := NewRunStateStore(t.TempDir(), "run-1")
	require.NoError(t, os.MkdirAll(store.RunDir(), 0o700))
	hostname, _ := os.Hostname()
	data, _ := json.Marshal(lockOwner{Hostname: hostname, PID: 999999999})
	require.NoError(t, os.WriteFile(filepath.Join(store.RunDir(), "run.lock"), append(data, '\n'), 0o600))
	lock, err := store.Acquire()
	require.NoError(t, err)
	require.NoError(t, lock.Close())
}

func TestRunStateDoesNotReclaimForeignHostLock(t *testing.T) {
	store := NewRunStateStore(t.TempDir(), "run-1")
	require.NoError(t, os.MkdirAll(store.RunDir(), 0o700))
	data, _ := json.Marshal(lockOwner{Hostname: "other-host", PID: 999999999})
	require.NoError(t, os.WriteFile(filepath.Join(store.RunDir(), "run.lock"), append(data, '\n'), 0o600))
	_, err := store.Acquire()
	require.ErrorContains(t, err, "active in another process")
}

func TestRecoverStaleLockRejectsLiveProcess(t *testing.T) {
	store := NewRunStateStore(t.TempDir(), "run-1")
	require.NoError(t, os.MkdirAll(store.RunDir(), 0o700))
	hostname, _ := os.Hostname()
	data, _ := json.Marshal(lockOwner{Hostname: hostname, PID: os.Getpid(), ProcessIdentity: processIdentity(os.Getpid())})
	require.NoError(t, os.WriteFile(filepath.Join(store.RunDir(), "run.lock"), append(data, '\n'), 0o600))
	require.ErrorContains(t, store.RecoverStaleLock(), "live process")
}

func TestCleaningFailedEntryPreservesFailedRun(t *testing.T) {
	entry := Entry{Version: "8.10", Shortname: "one", Scenario: "first", Flow: "install"}
	store := NewRunStateStore(t.TempDir(), "run-1")
	_, err := store.Create([]Entry{entry}, RunOptions{})
	require.NoError(t, err)
	require.NoError(t, store.Start(entry, "namespace"))
	require.NoError(t, store.Complete(entry, RunResult{Entry: entry, Error: errors.New("helm failed")}))
	require.NoError(t, store.MarkCleaned(EntryID(entry)))

	state, err := store.Load()
	require.NoError(t, err)
	assert.Equal(t, RunFailed, state.Status)
	assert.Equal(t, RunFailed, state.Entries[0].Status)
	assert.True(t, state.Entries[0].Cleaned)
	assert.Equal(t, "cleaned", state.Entries[0].Phase)
}

func TestResumeRestoresRedactedArgumentsFromRunSecrets(t *testing.T) {
	entry := Entry{Version: "8.10", Shortname: "one", Scenario: "first", Flow: "install"}
	store := NewRunStateStore(t.TempDir(), "run-1")
	_, err := store.Create([]Entry{entry}, RunOptions{ExtraHelmSets: []string{"global.password=secret"}})
	require.NoError(t, err)

	_, opts, err := store.PrepareResume("")
	require.NoError(t, err)
	assert.Equal(t, []string{"global.password=secret"}, opts.ExtraHelmSets)
	state, err := store.Load()
	require.NoError(t, err)
	assert.Equal(t, []string{"<redacted>"}, state.Options.ExtraHelmSets)
}

func TestRunStatePersistsGeneratedPostgresCredentialsSeparately(t *testing.T) {
	entry := Entry{Version: "8.10", Shortname: "one", Scenario: "first", Flow: "install"}
	store := NewRunStateStore(t.TempDir(), "run-1")
	_, err := store.Create([]Entry{entry}, RunOptions{
		GeneratePostgresCredentials: true,
		GeneratedPostgresUsername:   "camunda",
		GeneratedPostgresPassword:   "generated-password",
	})
	require.NoError(t, err)

	stateData, err := os.ReadFile(filepath.Join(store.RunDir(), "matrix-state.json"))
	require.NoError(t, err)
	assert.NotContains(t, string(stateData), "generated-password")
	secretInfo, err := os.Stat(filepath.Join(store.RunDir(), "run-secrets.json"))
	require.NoError(t, err)
	assert.Equal(t, os.FileMode(0o600), secretInfo.Mode().Perm())

	_, opts, err := store.PrepareResume("")
	require.NoError(t, err)
	assert.Equal(t, "generated-password", opts.GeneratedPostgresPassword)
}

func TestPrepareResumeRejectsMismatchedDockerCredentials(t *testing.T) {
	t.Setenv("HARBOR_USERNAME", "different-user")
	t.Setenv("HARBOR_PASSWORD", "password")
	t.Setenv("TEST_DOCKER_USERNAME_CAMUNDA_CLOUD", "")
	t.Setenv("TEST_DOCKER_PASSWORD_CAMUNDA_CLOUD", "")
	t.Setenv("NEXUS_USERNAME", "")
	t.Setenv("NEXUS_PASSWORD", "")
	entry := Entry{Version: "8.10", Shortname: "one", Scenario: "first", Flow: "install"}
	store := NewRunStateStore(t.TempDir(), "run-1")
	_, err := store.Create([]Entry{entry}, RunOptions{DockerUsername: "original-user", DockerPassword: "original-password", EnsureDockerRegistry: true})
	require.NoError(t, err)
	_, _, err = store.PrepareResume("")
	require.ErrorContains(t, err, "does not match stored username")
}

func TestPrepareResumeRestoresImportedDockerCredentials(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "config.json")
	auth := base64.StdEncoding.EncodeToString([]byte("imported-user:imported-password"))
	require.NoError(t, os.WriteFile(configPath, []byte(`{"auths":{"registry.camunda.cloud":{"auth":"`+auth+`"}}}`), 0o600))
	entry := Entry{Version: "8.10", Shortname: "one", Scenario: "first", Flow: "install"}
	store := NewRunStateStore(t.TempDir(), "run-1")
	_, err := store.Create([]Entry{entry}, RunOptions{
		DockerUsername: "imported-user", DockerPassword: "imported-password",
		ImportDockerAuth: true, DockerConfigPath: configPath,
	})
	require.NoError(t, err)
	_, opts, err := store.PrepareResume("")
	require.NoError(t, err)
	assert.Equal(t, "imported-password", opts.DockerPassword)
}

type testCredentialStore struct {
	values map[string]credentials.Credential
}

func (s testCredentialStore) Get(registry string) (credentials.Credential, bool, error) {
	value, ok := s.values[registry]
	return value, ok, nil
}
func (testCredentialStore) Set(string, credentials.Credential) error { return nil }
func (testCredentialStore) Delete(string) error                      { return nil }

func TestPrepareResumeRestoresKeyringCredential(t *testing.T) {
	entry := Entry{Version: "8.10", Shortname: "one", Scenario: "first", Flow: "install"}
	store := NewRunStateStore(t.TempDir(), "run-1")
	_, err := store.Create([]Entry{entry}, RunOptions{DockerUsername: "robot", DockerPassword: "original", EnsureDockerRegistry: true})
	require.NoError(t, err)
	original := resumeCredentialStore
	resumeCredentialStore = testCredentialStore{values: map[string]credentials.Credential{credentials.HarborRegistry: {Username: "robot", Password: "stored-token"}}}
	t.Cleanup(func() { resumeCredentialStore = original })
	_, opts, err := store.PrepareResume("")
	require.NoError(t, err)
	assert.Equal(t, "stored-token", opts.DockerPassword)
}

func TestClassifyFailure(t *testing.T) {
	tests := []struct {
		err  error
		code string
	}{
		{errors.New("helm upgrade failed"), "HELM_FAILURE"},
		{errors.New("docker registry unauthorized"), "REGISTRY_FAILURE"},
		{errors.New("Playwright e2e test failed"), "TEST_FAILURE"},
		{errors.New("context canceled"), "RUN_INTERRUPTED"},
		{errors.New("cluster connectivity check failed: kube context missing"), "KUBERNETES_FAILURE"},
	}
	for _, tt := range tests {
		assert.Equal(t, tt.code, ClassifyFailure(tt.err).Code)
	}
}

func TestCreatePersistsAbsoluteEntryChartPath(t *testing.T) {
	entry := Entry{Version: "8.10", Shortname: "one", Scenario: "first", Flow: "install", ChartPath: "../../charts/example"}
	store := NewRunStateStore(t.TempDir(), "run-1")
	_, err := store.Create([]Entry{entry}, RunOptions{})
	require.NoError(t, err)
	state, err := store.Load()
	require.NoError(t, err)
	assert.True(t, filepath.IsAbs(state.Entries[0].Entry.ChartPath), state.Entries[0].Entry.ChartPath)
	replay := strings.Join(state.Entries[0].Replay.Command, " ")
	assert.NotContains(t, replay, "--repo-root ../..")
}

func TestCreatePersistsAbsoluteLocalDependencyPaths(t *testing.T) {
	repoRoot := t.TempDir()
	chartPath := filepath.Join(repoRoot, "charts", "local")
	require.NoError(t, os.MkdirAll(chartPath, 0o755))
	entry := Entry{Version: "8.10", Shortname: "one", Scenario: "first", Flow: "install", Dependencies: []ChartDependency{{Chart: "charts/local", ValuesFile: "values/local.yaml"}}}
	store := NewRunStateStore(t.TempDir(), "run-1")
	_, err := store.Create([]Entry{entry}, RunOptions{RepoRoot: repoRoot})
	require.NoError(t, err)
	state, err := store.Load()
	require.NoError(t, err)
	assert.Equal(t, chartPath, state.Entries[0].Entry.Dependencies[0].Chart)
	assert.Equal(t, filepath.Join(repoRoot, "values/local.yaml"), state.Entries[0].Entry.Dependencies[0].ValuesFile)
}

func TestClassifyFailureRedactsCredentialValues(t *testing.T) {
	failure := ClassifyFailure(errors.New("request failed password=hunter2 token=abc123"))
	assert.NotContains(t, failure.Message, "hunter2")
	assert.NotContains(t, failure.Message, "abc123")
	assert.Equal(t, "deployment failed; inspect the diagnostics bundle", failure.Message)
}

func TestApplyCleanupResultPreservesFailureAndNotCleaned(t *testing.T) {
	result := RunResult{Error: errors.New("deployment failed")}
	cleaned := applyCleanupResult(&result, func() error { return errors.New("namespace deletion failed") })
	assert.False(t, cleaned)
	assert.ErrorContains(t, result.Error, "deployment failed")
	assert.ErrorContains(t, result.Error, "cleanup matrix entry: namespace deletion failed")
}

func TestCleanupEntryUsesCheckpointedAuth0IDsAndStopsOnFailure(t *testing.T) {
	entry := Entry{Version: "8.10", Shortname: "auth0", Scenario: "auth0", Flow: "install", Identity: "auth0"}
	store := NewRunStateStore(t.TempDir(), "run-1")
	_, err := store.Create([]Entry{entry}, RunOptions{})
	require.NoError(t, err)
	require.NoError(t, store.RecordExternalResources(entry, "", "", "https://tenant.example", []string{"id-A"}))
	original := cleanupAuth0IDs
	called := false
	cleanupAuth0IDs = func(context.Context, auth0.Options, []string) error {
		called = true
		return errors.New("provider failure")
	}
	t.Cleanup(func() { cleanupAuth0IDs = original })
	result := RunResult{Entry: entry, Namespace: "namespace", auth0Opts: &auth0.Options{Namespace: "namespace"}}
	err = cleanupEntry(context.Background(), result, RunOptions{StateStore: store})
	require.ErrorContains(t, err, "provider failure")
	assert.True(t, called)
}
