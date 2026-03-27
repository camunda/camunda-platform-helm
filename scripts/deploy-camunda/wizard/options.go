package wizard

import (
	"os/exec"
	"scripts/deploy-camunda/config"
	"strings"

	"scripts/camunda-core/pkg/scenarios"
)

// DataSource abstracts external lookups (kubectl, git, filesystem) for testability.
type DataSource interface {
	KubeContexts() ([]string, error)
	DetectRepoRoot() (string, error)
	ListScenarios(scenarioPath string) ([]string, error)
}

// LiveDataSource implements DataSource using real system calls.
type LiveDataSource struct{}

func (LiveDataSource) KubeContexts() ([]string, error) {
	out, err := exec.Command("kubectl", "config", "get-contexts", "-o", "name").Output()
	if err != nil {
		return nil, err
	}
	var contexts []string
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		ctx := strings.TrimSpace(line)
		if ctx != "" {
			contexts = append(contexts, ctx)
		}
	}
	return contexts, nil
}

func (LiveDataSource) DetectRepoRoot() (string, error) {
	return config.DetectRepoRoot()
}

func (LiveDataSource) ListScenarios(scenarioPath string) ([]string, error) {
	return scenarios.List(scenarioPath)
}

// MockDataSource provides canned data for tests.
type MockDataSource struct {
	Contexts      []string
	RepoRoot      string
	RepoRootErr   error
	Scenarios     []string
	ScenariosErr  error
}

func (m MockDataSource) KubeContexts() ([]string, error) {
	return m.Contexts, nil
}

func (m MockDataSource) DetectRepoRoot() (string, error) {
	return m.RepoRoot, m.RepoRootErr
}

func (m MockDataSource) ListScenarios(scenarioPath string) ([]string, error) {
	return m.Scenarios, m.ScenariosErr
}
