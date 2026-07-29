package matrix

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"scripts/camunda-core/pkg/versionmatrix"
	"scripts/deploy-camunda/credentials"
	"scripts/prepare-helm-values/pkg/env"
	"scripts/prepare-helm-values/pkg/values"
)

const RunStateSchema = "camunda.matrix-run/v1"

var resumeCredentialStore credentials.Store = credentials.KeyringStore{}

type RunStatus string

const (
	RunPending     RunStatus = "pending"
	RunRunning     RunStatus = "running"
	RunPassed      RunStatus = "passed"
	RunFailed      RunStatus = "failed"
	RunInterrupted RunStatus = "interrupted"
	RunCleaned     RunStatus = "cleaned"
)

type Failure struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

type ReplayManifest struct {
	Command []string `json:"command"`
}

type StoredRunOptions struct {
	Cleanup                     bool              `json:"cleanup,omitempty"`
	StopOnFailure               bool              `json:"stopOnFailure,omitempty"`
	KubeContexts                map[string]string `json:"kubeContexts,omitempty"`
	KubeContext                 string            `json:"kubeContext,omitempty"`
	NamespacePrefix             string            `json:"namespacePrefix,omitempty"`
	Platform                    string            `json:"platform,omitempty"`
	MaxParallel                 int               `json:"maxParallel,omitempty"`
	TestE2E                     bool              `json:"testE2E,omitempty"`
	TestAll                     bool              `json:"testAll,omitempty"`
	RepoRoot                    string            `json:"repoRoot"`
	EnvFiles                    map[string]string `json:"envFiles,omitempty"`
	EnvFile                     string            `json:"envFile,omitempty"`
	KeycloakHost                string            `json:"keycloakHost,omitempty"`
	KeycloakProtocol            string            `json:"keycloakProtocol,omitempty"`
	IngressBaseDomains          map[string]string `json:"ingressBaseDomains,omitempty"`
	IngressBaseDomain           string            `json:"ingressBaseDomain,omitempty"`
	LogLevel                    string            `json:"logLevel,omitempty"`
	SkipDependencyUpdate        bool              `json:"skipDependencyUpdate,omitempty"`
	VaultBackedSecrets          map[string]bool   `json:"vaultBackedSecrets,omitempty"`
	UseVaultBackedSecrets       bool              `json:"useVaultBackedSecrets,omitempty"`
	DeleteNamespaceFirst        bool              `json:"deleteNamespaceFirst,omitempty"`
	UpgradeFromVersion          string            `json:"upgradeFromVersion,omitempty"`
	HelmTimeout                 int               `json:"helmTimeout,omitempty"`
	EnsureDockerRegistry        bool              `json:"ensureDockerRegistry,omitempty"`
	DockerUsername              string            `json:"dockerUsername,omitempty"`
	RequiresDockerPassword      bool              `json:"requiresDockerPassword,omitempty"`
	EnsureDockerHub             bool              `json:"ensureDockerHub,omitempty"`
	DockerHubUsername           string            `json:"dockerHubUsername,omitempty"`
	RequiresDockerHubPassword   bool              `json:"requiresDockerHubPassword,omitempty"`
	UseLatest                   bool              `json:"useLatest,omitempty"`
	UseQA                       bool              `json:"useQA,omitempty"`
	ExtraHelmArgs               []string          `json:"extraHelmArgs,omitempty"`
	ExtraHelmSets               []string          `json:"extraHelmSets,omitempty"`
	ExtraValues                 []string          `json:"extraValues,omitempty"`
	NamespaceOverride           string            `json:"namespaceOverride,omitempty"`
	ChartRef                    string            `json:"chartRef,omitempty"`
	ChartRefVersion             string            `json:"chartRefVersion,omitempty"`
	ForceImageOverrides         bool              `json:"forceImageOverrides,omitempty"`
	WaitIngressReady            bool              `json:"waitIngressReady,omitempty"`
	IngressReadyTimeoutMinutes  int               `json:"ingressReadyTimeoutMinutes,omitempty"`
	GeneratePostgresCredentials bool              `json:"generatePostgresCredentials,omitempty"`
	ImportDockerAuth            bool              `json:"importDockerAuth,omitempty"`
	DockerConfigPath            string            `json:"dockerConfigPath,omitempty"`
}

func StoreRunOptions(opts RunOptions) StoredRunOptions {
	abs := func(path string) string {
		if path == "" {
			return ""
		}
		if value, err := filepath.Abs(path); err == nil {
			return value
		}
		return path
	}
	envFiles := make(map[string]string, len(opts.EnvFiles))
	for version, path := range opts.EnvFiles {
		envFiles[version] = abs(path)
	}
	extraValues := make([]string, len(opts.ExtraValues))
	for i, path := range opts.ExtraValues {
		extraValues[i] = abs(path)
	}
	chartRef := opts.ChartRef
	if strings.HasSuffix(chartRef, ".tgz") {
		chartRef = abs(chartRef)
	}
	return StoredRunOptions{
		Cleanup:       opts.Cleanup,
		StopOnFailure: opts.StopOnFailure, KubeContexts: opts.KubeContexts, KubeContext: opts.KubeContext,
		NamespacePrefix: opts.NamespacePrefix, Platform: opts.Platform, MaxParallel: opts.MaxParallel,
		TestE2E: opts.TestE2E, TestAll: opts.TestAll, RepoRoot: abs(opts.RepoRoot), EnvFiles: envFiles,
		EnvFile: abs(opts.EnvFile), KeycloakHost: opts.KeycloakHost, KeycloakProtocol: opts.KeycloakProtocol,
		IngressBaseDomains: opts.IngressBaseDomains, IngressBaseDomain: opts.IngressBaseDomain,
		LogLevel: opts.LogLevel, SkipDependencyUpdate: opts.SkipDependencyUpdate,
		VaultBackedSecrets: opts.VaultBackedSecrets, UseVaultBackedSecrets: opts.UseVaultBackedSecrets,
		DeleteNamespaceFirst: opts.DeleteNamespaceFirst, UpgradeFromVersion: opts.UpgradeFromVersion,
		HelmTimeout: opts.HelmTimeout, EnsureDockerRegistry: opts.EnsureDockerRegistry,
		DockerUsername: opts.DockerUsername, RequiresDockerPassword: opts.EnsureDockerRegistry,
		EnsureDockerHub: opts.EnsureDockerHub, DockerHubUsername: opts.DockerHubUsername,
		RequiresDockerHubPassword: opts.EnsureDockerHub, UseLatest: opts.UseLatest, UseQA: opts.UseQA,
		ExtraHelmArgs: redactStoredArgs(opts.ExtraHelmArgs), ExtraHelmSets: redactStoredSets(opts.ExtraHelmSets),
		ExtraValues: extraValues, NamespaceOverride: opts.NamespaceOverride, ChartRef: chartRef,
		ChartRefVersion: opts.ChartRefVersion, ForceImageOverrides: opts.ForceImageOverrides,
		WaitIngressReady: opts.WaitIngressReady, IngressReadyTimeoutMinutes: opts.IngressReadyTimeoutMinutes,
		GeneratePostgresCredentials: opts.GeneratePostgresCredentials,
		ImportDockerAuth:            opts.ImportDockerAuth, DockerConfigPath: abs(opts.DockerConfigPath),
	}
}

func (s StoredRunOptions) RunOptions() RunOptions {
	return RunOptions{
		Cleanup:       s.Cleanup,
		StopOnFailure: s.StopOnFailure, KubeContexts: s.KubeContexts, KubeContext: s.KubeContext,
		NamespacePrefix: s.NamespacePrefix, Platform: s.Platform, MaxParallel: s.MaxParallel,
		TestE2E: s.TestE2E, TestAll: s.TestAll, RepoRoot: s.RepoRoot, EnvFiles: s.EnvFiles,
		EnvFile: s.EnvFile, KeycloakHost: s.KeycloakHost, KeycloakProtocol: s.KeycloakProtocol,
		IngressBaseDomains: s.IngressBaseDomains, IngressBaseDomain: s.IngressBaseDomain,
		LogLevel: s.LogLevel, SkipDependencyUpdate: s.SkipDependencyUpdate,
		VaultBackedSecrets: s.VaultBackedSecrets, UseVaultBackedSecrets: s.UseVaultBackedSecrets,
		DeleteNamespaceFirst: s.DeleteNamespaceFirst, UpgradeFromVersion: s.UpgradeFromVersion,
		HelmTimeout: s.HelmTimeout, EnsureDockerRegistry: s.EnsureDockerRegistry,
		DockerUsername: s.DockerUsername, EnsureDockerHub: s.EnsureDockerHub,
		DockerHubUsername: s.DockerHubUsername, UseLatest: s.UseLatest, UseQA: s.UseQA,
		ExtraHelmArgs: removeRedactedArgs(s.ExtraHelmArgs), ExtraHelmSets: removeRedactedArgs(s.ExtraHelmSets),
		ExtraValues: s.ExtraValues, NamespaceOverride: s.NamespaceOverride, ChartRef: s.ChartRef,
		ChartRefVersion: s.ChartRefVersion, ForceImageOverrides: s.ForceImageOverrides,
		WaitIngressReady: s.WaitIngressReady, IngressReadyTimeoutMinutes: s.IngressReadyTimeoutMinutes,
		GeneratePostgresCredentials: s.GeneratePostgresCredentials,
		ImportDockerAuth:            s.ImportDockerAuth, DockerConfigPath: s.DockerConfigPath,
	}
}

type EntryRunState struct {
	ID               string         `json:"id"`
	Entry            Entry          `json:"entry"`
	Status           RunStatus      `json:"status"`
	Phase            string         `json:"phase,omitempty"`
	Namespace        string         `json:"namespace"`
	KubeContext      string         `json:"kubeContext,omitempty"`
	Attempts         int            `json:"attempts"`
	StartedAt        *time.Time     `json:"startedAt,omitempty"`
	FinishedAt       *time.Time     `json:"finishedAt,omitempty"`
	Failure          *Failure       `json:"failure,omitempty"`
	Diagnostics      string         `json:"diagnostics,omitempty"`
	Replay           ReplayManifest `json:"replay"`
	Cleaned          bool           `json:"cleaned,omitempty"`
	EntraObjectID    string         `json:"entraObjectId,omitempty"`
	Auth0ClientIDs   []string       `json:"auth0ClientIds,omitempty"`
	EntraDirectoryID string         `json:"entraDirectoryId,omitempty"`
	Auth0Domain      string         `json:"auth0Domain,omitempty"`
}

func (s *RunStateStore) RecordExternalResources(entry Entry, entraObjectID, entraDirectoryID, auth0Domain string, auth0ClientIDs []string) error {
	return s.update(entry, func(_ *MatrixRunState, item *EntryRunState) RunEvent {
		if entraObjectID != "" {
			item.EntraObjectID = entraObjectID
		}
		if entraDirectoryID != "" {
			item.EntraDirectoryID = entraDirectoryID
		}
		if auth0Domain != "" {
			item.Auth0Domain = auth0Domain
		}
		if len(auth0ClientIDs) > 0 {
			item.Auth0ClientIDs = append([]string(nil), auth0ClientIDs...)
		}
		return RunEvent{Time: time.Now().UTC(), RunID: s.runID, EntryID: item.ID, Status: item.Status, Phase: "external-resources"}
	})
}

type MatrixRunState struct {
	Schema    string           `json:"schema"`
	ID        string           `json:"id"`
	Status    RunStatus        `json:"status"`
	CreatedAt time.Time        `json:"createdAt"`
	UpdatedAt time.Time        `json:"updatedAt"`
	Options   StoredRunOptions `json:"options"`
	Entries   []*EntryRunState `json:"entries"`
}

type RunEvent struct {
	Time    time.Time `json:"time"`
	RunID   string    `json:"runId"`
	EntryID string    `json:"entryId,omitempty"`
	Status  RunStatus `json:"status,omitempty"`
	Phase   string    `json:"phase,omitempty"`
	Code    string    `json:"code,omitempty"`
}

type runSecrets struct {
	PostgresUsername string   `json:"postgresUsername,omitempty"`
	PostgresPassword string   `json:"postgresPassword,omitempty"`
	ExtraHelmArgs    []string `json:"extraHelmArgs,omitempty"`
	ExtraHelmSets    []string `json:"extraHelmSets,omitempty"`
}

type RunStateStore struct {
	root  string
	runID string
	mu    sync.Mutex
}

type RunLock struct {
	path string
}

type lockOwner struct {
	Hostname string `json:"hostname"`
	PID      int    `json:"pid"`
}

func NewRunStateStore(root, runID string) *RunStateStore {
	return &RunStateStore{root: root, runID: runID}
}

func ValidateRunID(runID string) error {
	if runID == "" || filepath.IsAbs(runID) || filepath.Base(runID) != runID || runID == "." || runID == ".." {
		return fmt.Errorf("invalid matrix run ID %q", runID)
	}
	return nil
}

func NewRunID(now time.Time) string {
	return now.UTC().Format("20060102T150405.000000000Z")
}

func (s *RunStateStore) RunDir() string { return filepath.Join(s.root, s.runID) }

func (s *RunStateStore) RunID() string { return s.runID }

func (s *RunStateStore) RecoverStaleLock() error {
	path := filepath.Join(s.RunDir(), "run.lock")
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("read matrix run lock: %w", err)
	}
	var owner lockOwner
	if err := json.Unmarshal(data, &owner); err != nil {
		return fmt.Errorf("decode matrix run lock: %w", err)
	}
	hostname, _ := os.Hostname()
	if owner.Hostname != hostname {
		return fmt.Errorf("refusing to remove lock created on host %q", owner.Hostname)
	}
	return os.Remove(path)
}

func (s *RunStateStore) Acquire() (*RunLock, error) {
	if err := os.MkdirAll(s.RunDir(), 0o700); err != nil {
		return nil, fmt.Errorf("create matrix run directory: %w", err)
	}
	path := filepath.Join(s.RunDir(), "run.lock")
	file, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	if err != nil {
		ownerData, readErr := os.ReadFile(path)
		var owner lockOwner
		parseErr := json.Unmarshal(ownerData, &owner)
		hostname, _ := os.Hostname()
		if readErr == nil && parseErr == nil && owner.Hostname == hostname && !processAlive(owner.PID) {
			if removeErr := os.Remove(path); removeErr == nil {
				file, err = os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
			}
		}
		if err != nil {
			return nil, fmt.Errorf("matrix run %q is active in another process", s.runID)
		}
	}
	hostname, _ := os.Hostname()
	ownerData, _ := json.Marshal(lockOwner{Hostname: hostname, PID: os.Getpid()})
	if _, err := file.Write(append(ownerData, '\n')); err != nil {
		file.Close()
		_ = os.Remove(path)
		return nil, err
	}
	if err := file.Close(); err != nil {
		_ = os.Remove(path)
		return nil, err
	}
	return &RunLock{path: path}, nil
}

func (l *RunLock) Close() error {
	if l == nil || l.path == "" {
		return nil
	}
	return os.Remove(l.path)
}

func (s *RunStateStore) Create(entries []Entry, opts RunOptions) (*MatrixRunState, error) {
	now := time.Now().UTC()
	state := &MatrixRunState{Schema: RunStateSchema, ID: s.runID, Status: RunPending, CreatedAt: now, UpdatedAt: now, Options: StoreRunOptions(opts)}
	replayOpts := state.Options.RunOptions()
	for _, entry := range entries {
		entry = normalizeEntryPaths(entry, replayOpts.RepoRoot)
		namespace := ResolveNamespace(opts, entry)
		state.Entries = append(state.Entries, &EntryRunState{
			ID: EntryID(entry), Entry: entry, Status: RunPending, Namespace: namespace,
			KubeContext: ResolveKubeContext(opts, entry), Replay: ReplayManifest{Command: ReplayCommand(entry, replayOpts)},
		})
	}
	if err := os.MkdirAll(s.RunDir(), 0o700); err != nil {
		return nil, fmt.Errorf("create matrix run directory: %w", err)
	}
	if err := os.Chmod(s.RunDir(), 0o700); err != nil {
		return nil, fmt.Errorf("secure matrix run directory: %w", err)
	}
	if opts.GeneratedPostgresPassword != "" || len(opts.ExtraHelmArgs) > 0 || len(opts.ExtraHelmSets) > 0 {
		data, err := json.Marshal(runSecrets{PostgresUsername: opts.GeneratedPostgresUsername, PostgresPassword: opts.GeneratedPostgresPassword, ExtraHelmArgs: opts.ExtraHelmArgs, ExtraHelmSets: opts.ExtraHelmSets})
		if err != nil {
			return nil, err
		}
		if err := atomicWriteFile(filepath.Join(s.RunDir(), "run-secrets.json"), append(data, '\n'), 0o600); err != nil {
			return nil, fmt.Errorf("write matrix run secrets: %w", err)
		}
	}
	if err := s.write(state); err != nil {
		return nil, err
	}
	return state, s.appendEvent(RunEvent{Time: now, RunID: s.runID, Status: RunPending})
}

func (s *RunStateStore) Load() (*MatrixRunState, error) {
	data, err := os.ReadFile(filepath.Join(s.RunDir(), "matrix-state.json"))
	if err != nil {
		return nil, fmt.Errorf("read matrix run %q: %w", s.runID, err)
	}
	var state MatrixRunState
	if err := json.Unmarshal(data, &state); err != nil {
		return nil, fmt.Errorf("decode matrix run %q: %w", s.runID, err)
	}
	if state.Schema != RunStateSchema {
		return nil, fmt.Errorf("unsupported matrix run schema %q", state.Schema)
	}
	return &state, nil
}

func (s *RunStateStore) Start(entry Entry, namespace string) error {
	return s.update(entry, func(state *MatrixRunState, item *EntryRunState) RunEvent {
		now := time.Now().UTC()
		state.Status = RunRunning
		item.Status, item.Namespace, item.StartedAt, item.FinishedAt = RunRunning, namespace, &now, nil
		item.Attempts++
		item.Failure = nil
		return RunEvent{Time: now, RunID: s.runID, EntryID: item.ID, Status: item.Status}
	})
}

func (s *RunStateStore) Phase(entry Entry, phase string) error {
	return s.update(entry, func(_ *MatrixRunState, item *EntryRunState) RunEvent {
		item.Phase = phase
		return RunEvent{Time: time.Now().UTC(), RunID: s.runID, EntryID: item.ID, Status: item.Status, Phase: phase}
	})
}

func (s *RunStateStore) Complete(entry Entry, result RunResult) error {
	return s.update(entry, func(state *MatrixRunState, item *EntryRunState) RunEvent {
		now := time.Now().UTC()
		item.FinishedAt, item.Diagnostics, item.KubeContext = &now, result.Diagnostics, result.KubeContext
		if result.Error == nil {
			item.Status, item.Phase, item.Failure = RunPassed, "complete", nil
		} else {
			item.Status = RunFailed
			item.Failure = ClassifyFailure(result.Error)
		}
		state.Status = aggregateRunStatus(state.Entries)
		code := ""
		if item.Failure != nil {
			code = item.Failure.Code
		}
		return RunEvent{Time: now, RunID: s.runID, EntryID: item.ID, Status: item.Status, Phase: item.Phase, Code: code}
	})
}

func (s *RunStateStore) MarkInterrupted() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	state, err := s.Load()
	if err != nil {
		return err
	}
	changed := false
	for _, item := range state.Entries {
		if item.Status == RunRunning {
			item.Status = RunInterrupted
			item.Failure = &Failure{Code: "RUN_INTERRUPTED", Message: "matrix process ended before the entry completed"}
			changed = true
		}
	}
	if !changed {
		return nil
	}
	state.Status, state.UpdatedAt = RunInterrupted, time.Now().UTC()
	if err := s.write(state); err != nil {
		return err
	}
	return s.appendEvent(RunEvent{Time: state.UpdatedAt, RunID: s.runID, Status: RunInterrupted, Code: "RUN_INTERRUPTED"})
}

func (s *RunStateStore) PrepareResume(entryID string) ([]Entry, RunOptions, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	state, err := s.Load()
	if err != nil {
		return nil, RunOptions{}, err
	}
	var entries []Entry
	for _, item := range state.Entries {
		if entryID != "" && item.ID != entryID {
			continue
		}
		if item.Cleaned || item.Status == RunPassed || item.Status == RunCleaned {
			continue
		}
		if item.Attempts > 0 && versionmatrix.IsTwoStepUpgradeFlow(item.Entry.Flow) {
			return nil, RunOptions{}, fmt.Errorf("entry %q is a partially executed two-step upgrade and cannot be resumed safely; replay it in a clean namespace", item.ID)
		}
		item.Status, item.Phase, item.Failure, item.Cleaned = RunPending, "", nil, false
		entries = append(entries, normalizeEntryPaths(item.Entry, state.Options.RepoRoot))
	}
	if entryID != "" && len(entries) == 0 {
		return nil, RunOptions{}, fmt.Errorf("entry %q is not resumable or does not exist", entryID)
	}
	if len(entries) == 0 {
		return nil, RunOptions{}, errors.New("matrix run has no incomplete entries")
	}
	harborPassword, hubPassword := "", ""
	var credentialErr error
	if harborPassword == "" {
		harborPassword, credentialErr = resumeCredentialPassword(state.Options.DockerUsername, state.Options.RequiresDockerPassword, [][2]string{
			{"HARBOR_USERNAME", "HARBOR_PASSWORD"},
			{"TEST_DOCKER_USERNAME_CAMUNDA_CLOUD", "TEST_DOCKER_PASSWORD_CAMUNDA_CLOUD"},
			{"NEXUS_USERNAME", "NEXUS_PASSWORD"},
		})
	}
	if credentialErr != nil {
		return nil, RunOptions{}, fmt.Errorf("restore Harbor credentials: %w", credentialErr)
	}
	if harborPassword == "" {
		harborPassword, credentialErr = resumeCredentialPasswordFromFiles(state.Options, entries, state.Options.DockerUsername, [][2]string{
			{"HARBOR_USERNAME", "HARBOR_PASSWORD"},
			{"TEST_DOCKER_USERNAME_CAMUNDA_CLOUD", "TEST_DOCKER_PASSWORD_CAMUNDA_CLOUD"},
			{"NEXUS_USERNAME", "NEXUS_PASSWORD"},
		})
	}
	if credentialErr != nil {
		return nil, RunOptions{}, fmt.Errorf("restore Harbor credentials: %w", credentialErr)
	}
	if hubPassword == "" {
		hubPassword, credentialErr = resumeCredentialPassword(state.Options.DockerHubUsername, state.Options.RequiresDockerHubPassword, [][2]string{
			{"DOCKERHUB_USERNAME", "DOCKERHUB_PASSWORD"},
			{"TEST_DOCKER_USERNAME", "TEST_DOCKER_PASSWORD"},
		})
	}
	if hubPassword == "" {
		hubPassword, credentialErr = resumeCredentialPasswordFromFiles(state.Options, entries, state.Options.DockerHubUsername, [][2]string{
			{"DOCKERHUB_USERNAME", "DOCKERHUB_PASSWORD"},
			{"TEST_DOCKER_USERNAME", "TEST_DOCKER_PASSWORD"},
		})
	}
	if credentialErr != nil {
		return nil, RunOptions{}, fmt.Errorf("restore Docker Hub credentials: %w", credentialErr)
	}
	if harborPassword == "" && state.Options.RequiresDockerPassword {
		credential, found, keyringErr := credentials.GetOptional(resumeCredentialStore, credentials.HarborRegistry)
		if keyringErr != nil {
			return nil, RunOptions{}, keyringErr
		}
		if keyringErr == nil && found && credential.Username == state.Options.DockerUsername {
			harborPassword = credential.Password
		}
	}
	if hubPassword == "" && state.Options.RequiresDockerHubPassword {
		credential, found, keyringErr := credentials.GetOptional(resumeCredentialStore, credentials.DockerHubRegistry)
		if keyringErr != nil {
			return nil, RunOptions{}, keyringErr
		}
		if keyringErr == nil && found && credential.Username == state.Options.DockerHubUsername {
			hubPassword = credential.Password
		}
	}
	if state.Options.ImportDockerAuth && (harborPassword == "" || hubPassword == "") {
		auths, importErr := ImportPlaintextDockerAuth(state.Options.DockerConfigPath, "registry.camunda.cloud", "docker.io")
		if importErr != nil {
			return nil, RunOptions{}, fmt.Errorf("restore Docker config credentials: %w", importErr)
		}
		if auth, ok := auths["registry.camunda.cloud"]; harborPassword == "" && ok && auth.Username == state.Options.DockerUsername {
			harborPassword = auth.Password
		}
		if auth, ok := auths["docker.io"]; hubPassword == "" && ok && auth.Username == state.Options.DockerHubUsername {
			hubPassword = auth.Password
		}
	}
	if state.Options.RequiresDockerPassword && harborPassword == "" {
		return nil, RunOptions{}, errors.New("matching Harbor credentials are required to resume")
	}
	if state.Options.RequiresDockerHubPassword && hubPassword == "" {
		return nil, RunOptions{}, errors.New("matching Docker Hub credentials are required to resume")
	}
	opts := state.Options.RunOptions()
	opts.DeleteNamespaceFirst = false
	opts.DockerPassword, opts.DockerHubPassword = harborPassword, hubPassword
	secretData, err := os.ReadFile(filepath.Join(s.RunDir(), "run-secrets.json"))
	if err == nil {
		var secrets runSecrets
		if err := json.Unmarshal(secretData, &secrets); err != nil {
			return nil, RunOptions{}, fmt.Errorf("decode matrix run secrets: %w", err)
		}
		opts.GeneratedPostgresUsername = secrets.PostgresUsername
		opts.GeneratedPostgresPassword = secrets.PostgresPassword
		opts.ExtraHelmArgs = secrets.ExtraHelmArgs
		opts.ExtraHelmSets = secrets.ExtraHelmSets
	} else if !errors.Is(err, os.ErrNotExist) {
		return nil, RunOptions{}, fmt.Errorf("read matrix run secrets: %w", err)
	}
	state.Status, state.UpdatedAt = RunPending, time.Now().UTC()
	if err := s.write(state); err != nil {
		return nil, RunOptions{}, err
	}
	return entries, opts, nil
}

func normalizeEntryPaths(entry Entry, repoRoot string) Entry {
	if entry.ChartPath != "" && !filepath.IsAbs(entry.ChartPath) {
		candidate := entry.ChartPath
		if repoRoot != "" && !strings.HasPrefix(filepath.Clean(candidate), filepath.Clean(repoRoot)+string(filepath.Separator)) {
			if relative, err := filepath.Rel(filepath.Dir(filepath.Join(repoRoot, "scripts", "deploy-camunda")), candidate); err == nil && !strings.HasPrefix(relative, "..") {
				candidate = filepath.Join(repoRoot, relative)
			}
		}
		if absolute, err := filepath.Abs(candidate); err == nil {
			entry.ChartPath = absolute
		}
	}
	for i := range entry.Dependencies {
		if entry.Dependencies[i].ValuesFile != "" && !filepath.IsAbs(entry.Dependencies[i].ValuesFile) {
			entry.Dependencies[i].ValuesFile = filepath.Join(repoRoot, entry.Dependencies[i].ValuesFile)
		}
		if entry.Dependencies[i].Chart != "" && !filepath.IsAbs(entry.Dependencies[i].Chart) {
			candidate := filepath.Join(repoRoot, entry.Dependencies[i].Chart)
			if info, err := os.Stat(candidate); err == nil && info.IsDir() {
				entry.Dependencies[i].Chart = candidate
			}
		}
	}
	return entry
}

func (s *RunStateStore) MarkCleaned(entryID string) error {
	return s.updateID(entryID, func(state *MatrixRunState, item *EntryRunState) RunEvent {
		item.Cleaned = true
		item.Phase = "cleaned"
		if item.Status != RunFailed && item.Status != RunInterrupted {
			item.Status = RunCleaned
		}
		state.Status = aggregateRunStatus(state.Entries)
		return RunEvent{Time: time.Now().UTC(), RunID: s.runID, EntryID: item.ID, Status: item.Status, Phase: "cleanup"}
	})
}

func (s *RunStateStore) update(entry Entry, fn func(*MatrixRunState, *EntryRunState) RunEvent) error {
	return s.updateID(EntryID(entry), fn)
}

func (s *RunStateStore) updateID(id string, fn func(*MatrixRunState, *EntryRunState) RunEvent) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	state, err := s.Load()
	if err != nil {
		return err
	}
	for _, item := range state.Entries {
		if item.ID != id {
			continue
		}
		event := fn(state, item)
		state.UpdatedAt = time.Now().UTC()
		if err := s.appendEvent(event); err != nil {
			return err
		}
		return s.write(state)
	}
	return fmt.Errorf("matrix entry %q not found in run %q", id, s.runID)
}

func (s *RunStateStore) write(state *MatrixRunState) error {
	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return fmt.Errorf("encode matrix run state: %w", err)
	}
	data = append(data, '\n')
	return atomicWriteFile(filepath.Join(s.RunDir(), "matrix-state.json"), data, 0o600)
}

func (s *RunStateStore) appendEvent(event RunEvent) error {
	data, err := json.Marshal(event)
	if err != nil {
		return err
	}
	f, err := os.OpenFile(filepath.Join(s.RunDir(), "events.ndjson"), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		return fmt.Errorf("open matrix run events: %w", err)
	}
	defer f.Close()
	if err := f.Chmod(0o600); err != nil {
		return err
	}
	if _, err := f.Write(append(data, '\n')); err != nil {
		return fmt.Errorf("append matrix run event: %w", err)
	}
	return f.Sync()
}

func atomicWriteFile(path string, data []byte, mode os.FileMode) error {
	dir := filepath.Dir(path)
	tmp, err := os.CreateTemp(dir, ".state-*")
	if err != nil {
		return fmt.Errorf("create temporary state file: %w", err)
	}
	tmpPath := tmp.Name()
	defer os.Remove(tmpPath)
	if err := tmp.Chmod(mode); err != nil {
		tmp.Close()
		return err
	}
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Rename(tmpPath, path); err != nil {
		return fmt.Errorf("replace state file: %w", err)
	}
	d, err := os.Open(dir)
	if err != nil {
		return fmt.Errorf("open state directory: %w", err)
	}
	defer d.Close()
	if err := d.Sync(); err != nil {
		return fmt.Errorf("sync state directory: %w", err)
	}
	return nil
}

func EntryID(entry Entry) string { return entryID(entry) }

func ReplayCommand(entry Entry, opts RunOptions) []string {
	args := []string{"deploy-camunda", "matrix", "run", "--versions", entry.Version, "--shortname-filter", entry.Shortname, "--shortname-exact", "--flow-filter", entry.Flow}
	if entry.Platform != "" {
		args = append(args, "--platform", entry.Platform)
	}
	if opts.RepoRoot != "" {
		args = append(args, "--repo-root", opts.RepoRoot)
	}
	if path := opts.EnvFiles[entry.Version]; path != "" {
		args = append(args, "--env-file-"+entry.Version, path)
	} else if opts.EnvFile != "" {
		args = append(args, "--env-file", opts.EnvFile)
	}
	if opts.NamespacePrefix != "" {
		args = append(args, "--namespace-prefix", opts.NamespacePrefix)
	}
	if opts.KubeContext != "" {
		args = append(args, "--kube-context", opts.KubeContext)
	}
	if value := opts.KubeContexts[entry.Platform]; value != "" {
		args = append(args, "--kube-context-"+entry.Platform, value)
	}
	if opts.IngressBaseDomain != "" {
		args = append(args, "--ingress-base-domain", opts.IngressBaseDomain)
	}
	if value := opts.IngressBaseDomains[entry.Platform]; value != "" {
		args = append(args, "--ingress-base-domain-"+entry.Platform, value)
	}
	if opts.NamespaceOverride != "" {
		args = append(args, "--namespace-override", opts.NamespaceOverride)
	}
	if opts.ChartRef != "" {
		args = append(args, "--chart-ref", opts.ChartRef)
	}
	if opts.ChartRefVersion != "" {
		args = append(args, "--chart-version", opts.ChartRefVersion)
	}
	if opts.UpgradeFromVersion != "" {
		args = append(args, "--upgrade-from-version", opts.UpgradeFromVersion)
	}
	if opts.UseLatest {
		args = append(args, "--use-latest")
	}
	if opts.UseQA {
		args = append(args, "--use-qa")
	}
	if opts.ForceImageOverrides {
		args = append(args, "--force-image-overrides")
	}
	if opts.HelmTimeout > 0 {
		args = append(args, "--timeout", strconv.Itoa(opts.HelmTimeout))
	}
	if opts.WaitIngressReady {
		args = append(args, "--wait-ingress-ready", "--ingress-ready-timeout", strconv.Itoa(opts.IngressReadyTimeoutMinutes))
	}
	if opts.SkipDependencyUpdate {
		args = append(args, "--skip-dependency-update")
	}
	for _, value := range redactStoredArgs(opts.ExtraHelmArgs) {
		args = append(args, "--extra-helm-arg", value)
	}
	for _, value := range redactStoredSets(opts.ExtraHelmSets) {
		args = append(args, "--extra-helm-set", value)
	}
	for _, value := range opts.ExtraValues {
		args = append(args, "--extra-values", value)
	}
	if opts.TestE2E {
		args = append(args, "--test-e2e")
	}
	if opts.TestAll {
		args = append(args, "--test-all")
	}
	if opts.EnsureDockerRegistry {
		args = append(args, "--ensure-docker-registry")
	}
	if opts.EnsureDockerHub {
		args = append(args, "--ensure-docker-hub")
	}
	if opts.Cleanup {
		args = append(args, "--cleanup")
	}
	if !opts.GeneratePostgresCredentials {
		args = append(args, "--generate-postgres-credentials=false")
	}
	if opts.ImportDockerAuth {
		args = append(args, "--import-docker-auth")
		if opts.DockerConfigPath != "" {
			args = append(args, "--docker-config", opts.DockerConfigPath)
		}
	}
	return args
}

func ResolveKubeContext(opts RunOptions, entry Entry) string {
	return resolveKubeContext(opts, resolvePlatform(opts, entry))
}

func ClassifyFailure(err error) *Failure {
	if err == nil {
		return nil
	}
	lower := strings.ToLower(err.Error())
	code := "DEPLOY_UNKNOWN"
	switch {
	case errors.Is(err, os.ErrNotExist), strings.Contains(lower, "no such file"):
		code = "INPUT_NOT_FOUND"
	case strings.Contains(lower, "docker") || strings.Contains(lower, "registry") || strings.Contains(lower, "imagepull"):
		code = "REGISTRY_FAILURE"
	case strings.Contains(lower, "unauthorized") || strings.Contains(lower, "forbidden") || strings.Contains(lower, "credential"):
		code = "AUTH_FAILURE"
	case strings.Contains(lower, "context canceled") || strings.Contains(lower, "cancelled"):
		code = "RUN_INTERRUPTED"
	case strings.Contains(lower, "helm") || strings.Contains(lower, "upgrade") || strings.Contains(lower, "release"):
		code = "HELM_FAILURE"
	case strings.Contains(lower, "test") || strings.Contains(lower, "playwright") || strings.Contains(lower, "e2e"):
		code = "TEST_FAILURE"
	case strings.Contains(lower, "ingress") || strings.Contains(lower, "dns"):
		code = "INGRESS_FAILURE"
	case strings.Contains(lower, "namespace") || strings.Contains(lower, "kubernetes") || strings.Contains(lower, "kubectl"):
		code = "KUBERNETES_FAILURE"
	case strings.Contains(lower, "kube context") || strings.Contains(lower, "kubeconfig") || strings.Contains(lower, "cluster connectivity"):
		code = "KUBERNETES_FAILURE"
	}
	return &Failure{Code: code, Message: failureMessage(code)}
}

func failureMessage(code string) string {
	messages := map[string]string{
		"INPUT_NOT_FOUND":    "required deployment input was not found",
		"REGISTRY_FAILURE":   "container registry operation failed",
		"AUTH_FAILURE":       "authentication or authorization failed",
		"RUN_INTERRUPTED":    "matrix entry was interrupted",
		"HELM_FAILURE":       "Helm deployment failed",
		"TEST_FAILURE":       "deployment test failed",
		"INGRESS_FAILURE":    "ingress readiness failed",
		"KUBERNETES_FAILURE": "Kubernetes operation failed",
		"DEPLOY_UNKNOWN":     "deployment failed; inspect the diagnostics bundle",
	}
	return messages[code]
}

func redactStoredArgs(args []string) []string {
	out := make([]string, len(args))
	for i, arg := range args {
		out[i] = arg
		key := strings.TrimLeft(arg, "-")
		if idx := strings.LastIndex(key, "="); idx >= 0 {
			key = key[:idx]
		}
		if values.IsSecretName(key) || !safeStoredHelmKey(key) {
			out[i] = "<redacted>"
		}
	}
	return out
}

func safeStoredHelmKey(key string) bool {
	for _, prefix := range []string{"global.host", "orchestration.upgrade.allowPreReleaseImages", "feature.enabled", "atomic"} {
		if key == prefix || strings.HasSuffix(key, prefix) {
			return true
		}
	}
	return false
}

func redactStoredSets(values []string) []string { return redactStoredArgs(values) }

func removeRedactedArgs(values []string) []string {
	var out []string
	for _, value := range values {
		if value != "<redacted>" {
			out = append(out, value)
		}
	}
	return out
}

func hasRedactedArgs(values []string) bool {
	for _, value := range values {
		if value == "<redacted>" {
			return true
		}
	}
	return false
}

func resumeCredentialPassword(storedUsername string, required bool, pairs [][2]string) (string, error) {
	if !required {
		return "", nil
	}
	for _, pair := range pairs {
		username, password := os.Getenv(pair[0]), os.Getenv(pair[1])
		if username == "" && password == "" {
			continue
		}
		if username == "" || password == "" {
			return "", fmt.Errorf("%s/%s must both be set", pair[0], pair[1])
		}
		if storedUsername != "" && username != storedUsername {
			return "", fmt.Errorf("%s username %q does not match stored username %q", pair[0], username, storedUsername)
		}
		return password, nil
	}
	return "", nil
}

func resumeCredentialPasswordFromFiles(options StoredRunOptions, entries []Entry, storedUsername string, pairs [][2]string) (string, error) {
	paths := map[string]struct{}{}
	path := options.EnvFile
	if path == "" {
		path = ".env"
	}
	paths[path] = struct{}{}
	for _, entry := range entries {
		if versionPath := options.EnvFiles[entry.Version]; versionPath != "" {
			paths[versionPath] = struct{}{}
		}
	}
	password := ""
	for path := range paths {
		values, err := env.ReadFile(path)
		if err != nil {
			continue
		}
		for _, pair := range pairs {
			if values[pair[0]] == storedUsername && values[pair[1]] != "" {
				if password != "" && password != values[pair[1]] {
					return "", errors.New("selected env files contain different registry passwords")
				}
				password = values[pair[1]]
			}
		}
	}
	return password, nil
}

func aggregateRunStatus(entries []*EntryRunState) RunStatus {
	allTerminal, anyFailed, allCleaned := true, false, len(entries) > 0
	for _, item := range entries {
		switch item.Status {
		case RunFailed, RunInterrupted:
			anyFailed = true
		case RunPending, RunRunning:
			allTerminal = false
		}
		if !item.Cleaned && item.Status != RunCleaned {
			allCleaned = false
		}
	}
	if anyFailed {
		return RunFailed
	}
	if allCleaned {
		return RunCleaned
	}
	if allTerminal {
		return RunPassed
	}
	return RunRunning
}

func ListRunIDs(root string) ([]string, error) {
	items, err := os.ReadDir(root)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var ids []string
	for _, item := range items {
		if item.IsDir() {
			ids = append(ids, item.Name())
		}
	}
	sort.Sort(sort.Reverse(sort.StringSlice(ids)))
	return ids, nil
}
