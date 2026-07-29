package matrix

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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
	assert.NotContains(t, text, "global.host=example.test")
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

func TestRunStateRefusesConcurrentLock(t *testing.T) {
	store := NewRunStateStore(t.TempDir(), "run-1")
	first, err := store.Acquire()
	require.NoError(t, err)
	defer first.Close()

	_, err = NewRunStateStore(store.root, "run-1").Acquire()
	require.ErrorContains(t, err, "active in another process")
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
}

func TestResumeRejectsRedactedArguments(t *testing.T) {
	entry := Entry{Version: "8.10", Shortname: "one", Scenario: "first", Flow: "install"}
	store := NewRunStateStore(t.TempDir(), "run-1")
	_, err := store.Create([]Entry{entry}, RunOptions{ExtraHelmSets: []string{"global.password=secret"}})
	require.NoError(t, err)

	_, _, err = store.PrepareResume("")
	require.ErrorContains(t, err, "cannot reconstruct them safely")
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
	_, err := store.Create([]Entry{entry}, RunOptions{DockerUsername: "original-user", DockerPassword: "original-password"})
	require.NoError(t, err)
	_, _, err = store.PrepareResume("")
	require.ErrorContains(t, err, "does not match stored username")
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
	}
	for _, tt := range tests {
		assert.Equal(t, tt.code, ClassifyFailure(tt.err).Code)
	}
}

func TestClassifyFailureRedactsCredentialValues(t *testing.T) {
	failure := ClassifyFailure(errors.New("request failed password=hunter2 token=abc123"))
	assert.NotContains(t, failure.Message, "hunter2")
	assert.NotContains(t, failure.Message, "abc123")
	assert.Equal(t, "deployment failed; inspect the diagnostics bundle", failure.Message)
}
