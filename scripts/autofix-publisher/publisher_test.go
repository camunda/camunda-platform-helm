// Copyright Camunda Services GmbH
// SPDX-License-Identifier: Apache-2.0

package publisher

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"go.yaml.in/yaml/v3"
)

type step struct {
	Name string            `yaml:"name"`
	Uses string            `yaml:"uses"`
	Run  string            `yaml:"run"`
	If   string            `yaml:"if"`
	Env  map[string]string `yaml:"env"`
	With map[string]string `yaml:"with"`
}

type job struct {
	Steps       []step            `yaml:"steps"`
	Permissions map[string]string `yaml:"permissions"`
	Env         map[string]string `yaml:"env"`
	Needs       string            `yaml:"needs"`
	If          string            `yaml:"if"`
	Uses        string            `yaml:"uses"`
	With        map[string]string `yaml:"with"`
}

type workflow struct {
	Jobs map[string]job `yaml:"jobs"`
}

func loadWorkflow(test *testing.T, name string) workflow {
	test.Helper()
	content, err := os.ReadFile(filepath.Join("..", "..", ".github", "workflows", name))
	if err != nil {
		test.Fatal(err)
	}
	var result workflow
	if err := yaml.Unmarshal(content, &result); err != nil {
		test.Fatal(err)
	}
	return result
}

func namedStep(test *testing.T, owner job, name string) step {
	test.Helper()
	for _, candidate := range owner.Steps {
		if candidate.Name == name {
			return candidate
		}
	}
	test.Fatalf("step %q not found", name)
	return step{}
}

type fixture struct {
	test        *testing.T
	root        string
	producer    string
	remote      string
	runner      string
	base        string
	tip         string
	environment []string
	publisher   job
}

func commandOutput(test *testing.T, directory string, environment []string, executable string, arguments ...string) (string, error) {
	test.Helper()
	command := exec.Command(executable, arguments...)
	command.Dir = directory
	command.Env = environment
	output, err := command.CombinedOutput()
	return string(output), err
}

func (fixture *fixture) git(directory string, arguments ...string) string {
	fixture.test.Helper()
	output, err := commandOutput(fixture.test, directory, fixture.environment, "git", arguments...)
	if err != nil {
		fixture.test.Fatalf("git %q: %v; output=%q", arguments, err, output)
	}
	return strings.TrimSpace(output)
}

func writeFile(test *testing.T, filename, content string, mode os.FileMode) {
	test.Helper()
	if err := os.MkdirAll(filepath.Dir(filename), 0700); err != nil {
		test.Fatal(err)
	}
	if err := os.WriteFile(filename, []byte(content), mode); err != nil {
		test.Fatal(err)
	}
}

func newFixture(test *testing.T) *fixture {
	test.Helper()
	root := test.TempDir()
	result := &fixture{
		test: test, root: root, producer: filepath.Join(root, "producer"),
		remote: filepath.Join(root, "remote.git"), runner: filepath.Join(root, "runner"),
		publisher:   loadWorkflow(test, "commit-generated-template.yaml").Jobs["push"],
		environment: []string{"PATH=" + os.Getenv("PATH"), "HOME=" + root, "GIT_CONFIG_NOSYSTEM=1", "GIT_CONFIG_GLOBAL=/dev/null", "GIT_NO_REPLACE_OBJECTS=1", "GIT_TERMINAL_PROMPT=0"},
	}
	result.git(root, "init", "--bare", result.remote)
	result.git(root, "init", "--initial-branch=canary", result.producer)
	result.git(result.producer, "config", "user.name", "Publisher test")
	result.git(result.producer, "config", "user.email", "publisher@example.invalid")
	writeFile(test, filepath.Join(result.producer, "charts/camunda-platform-8.10/README.md"), "original\n", 0644)
	result.git(result.producer, "add", ".")
	result.git(result.producer, "commit", "-m", "base")
	result.base = result.git(result.producer, "rev-parse", "HEAD")
	result.git(result.producer, "push", result.remote, "HEAD:refs/heads/canary")
	if err := os.MkdirAll(filepath.Join(result.runner, "generated"), 0700); err != nil {
		test.Fatal(err)
	}
	return result
}

func (fixture *fixture) bundle() {
	fixture.test.Helper()
	fixture.git(fixture.producer, "add", "--all")
	fixture.git(fixture.producer, "commit", "--allow-empty", "-m", "generated")
	fixture.tip = fixture.git(fixture.producer, "rev-parse", "HEAD")
	fixture.git(fixture.producer, "bundle", "create", filepath.Join(fixture.runner, "generated/changes.bundle"), "HEAD", "^"+fixture.base)
}

func (fixture *fixture) execute(name string, modules bool) (string, error) {
	fixture.test.Helper()
	script := namedStep(fixture.test, fixture.publisher, name)
	moduleFlag := "false"
	if modules {
		moduleFlag = "true"
	}
	environment := append(append([]string{}, fixture.environment...),
		"RUNNER_TEMP="+fixture.runner, "BASE_SHA="+fixture.base, "TARGET_REF=canary", "TARGET_URL="+fixture.remote,
		"ALLOWED_PATHS="+script.Env["ALLOWED_PATHS"], "UPDATE_MODULES="+moduleFlag, "GH_APP_TOKEN=dummy-not-a-secret")
	return commandOutput(fixture.test, fixture.root, environment, "/bin/bash", "--noprofile", "--norc", "-e", "-o", "pipefail", "-c", script.Run)
}

func TestGeneratedCommitPolicy(test *testing.T) {
	for _, scenario := range []struct {
		name    string
		file    string
		mode    os.FileMode
		modules bool
		valid   bool
	}{
		{name: "readme", file: "charts/camunda-platform-8.10/README.md", valid: true},
		{name: "schema", file: "charts/camunda-platform-8.10/values.schema.json", valid: true},
		{name: "golden", file: "charts/camunda-platform-8.10/test/unit/orchestration/golden/configmap.golden.yaml", valid: true},
		{name: "registry", file: "charts/camunda-platform-8.9/test/ci/registry-snapshot.yaml", valid: true},
		{name: "lock", file: "charts/camunda-platform-8.10/Chart.lock", valid: true},
		{name: "renovate-modules", file: "scripts/deploy-camunda/go.mod", modules: true, valid: true},
		{name: "renovate-chart-sums", file: "charts/camunda-platform-8.9/go.sum", modules: true, valid: true},
		{name: "chart-modules-denied", file: "scripts/deploy-camunda/go.mod"},
		{name: "workflow", file: ".github/workflows/changed.yaml"},
		{name: "script", file: "scripts/deploy-camunda/main.go", modules: true},
		{name: "chart-template", file: "charts/camunda-platform-8.10/templates/configmap.yaml"},
		{name: "executable", file: "charts/camunda-platform-8.10/README.md", mode: 0755},
		{name: "newline", file: "charts/camunda-platform-8.10/test/unit/orchestration/golden/bad\nfile.yaml"},
	} {
		test.Run(scenario.name, func(test *testing.T) {
			fixture := newFixture(test)
			mode := scenario.mode
			if mode == 0 {
				mode = 0644
			}
			filename := filepath.Join(fixture.producer, scenario.file)
			writeFile(test, filename, "generated\n", mode)
			if err := os.Chmod(filename, mode); err != nil {
				test.Fatal(err)
			}
			fixture.bundle()
			output, err := fixture.execute("Validate generated commit", scenario.modules)
			if (err == nil) != scenario.valid {
				test.Fatalf("valid=%t error=%v output=%q", scenario.valid, err, output)
			}
			if scenario.valid {
				output, err = fixture.execute("Push generated commit", scenario.modules)
				if err != nil {
					test.Fatalf("push: %v output=%q", err, output)
				}
				if actual := fixture.git(fixture.remote, "rev-parse", "refs/heads/canary"); actual != fixture.tip {
					test.Fatalf("remote %s != generated %s", actual, fixture.tip)
				}
			}
		})
	}
}

func TestProducerStateDoesNotCrossBoundary(test *testing.T) {
	fixture := newFixture(test)
	marker := filepath.Join(fixture.root, "credential-observed")
	observer := "#!/bin/sh\nif [ -n \"${GH_APP_TOKEN-}\" ]; then printf observed > \"" + marker + "\"; fi\n"
	writeFile(test, filepath.Join(fixture.producer, ".git/hooks/pre-push"), observer, 0755)
	writeFile(test, filepath.Join(fixture.producer, ".git/injected-bin/git"), observer, 0755)
	writeFile(test, filepath.Join(fixture.producer, ".git/startup.sh"), observer, 0644)
	writeFile(test, filepath.Join(fixture.producer, ".git/github-env"), "BASH_ENV="+filepath.Join(fixture.producer, ".git/startup.sh")+"\n", 0644)
	writeFile(test, filepath.Join(fixture.producer, ".git/github-path"), filepath.Join(fixture.producer, ".git/injected-bin")+"\n", 0644)
	fixture.git(fixture.producer, "config", "core.hooksPath", filepath.Join(fixture.producer, ".git/hooks"))
	fixture.git(fixture.producer, "config", "credential.helper", "!"+filepath.Join(fixture.producer, ".git/injected-bin/git"))
	writeFile(test, filepath.Join(fixture.producer, "charts/camunda-platform-8.10/README.md"), "generated\n", 0644)
	fixture.bundle()
	for _, operation := range []string{"Validate generated commit", "Push generated commit"} {
		if output, err := fixture.execute(operation, false); err != nil {
			test.Fatalf("%s: %v output=%q", operation, err, output)
		}
	}
	if _, err := os.Stat(marker); !os.IsNotExist(err) {
		test.Fatalf("producer code observed publisher credential: %v", err)
	}
	if fixture.git(fixture.remote, "rev-parse", "refs/heads/canary") != fixture.tip {
		test.Fatal("isolated publisher did not push the commit")
	}
	configuration, err := os.ReadFile(filepath.Join(fixture.runner, "publish.git/config"))
	if err != nil {
		test.Fatal(err)
	}
	for _, forbidden := range []string{"dummy-not-a-secret", "hooksPath", "credential", "injected-bin"} {
		if strings.Contains(string(configuration), forbidden) {
			test.Fatalf("publisher persisted %s", forbidden)
		}
	}
}

func TestBinaryAndDeletion(test *testing.T) {
	for _, operation := range []string{"binary", "delete"} {
		test.Run(operation, func(test *testing.T) {
			fixture := newFixture(test)
			filename := filepath.Join(fixture.producer, "charts/camunda-platform-8.10/README.md")
			if operation == "binary" {
				writeFile(test, filename, "generated\x00binary\n", 0644)
			} else if err := os.Remove(filename); err != nil {
				test.Fatal(err)
			}
			fixture.bundle()
			for _, name := range []string{"Validate generated commit", "Push generated commit"} {
				if output, err := fixture.execute(name, false); err != nil {
					test.Fatalf("%s: %v output=%q", name, err, output)
				}
			}
			if fixture.git(fixture.remote, "rev-parse", "refs/heads/canary^{tree}") != fixture.git(fixture.producer, "rev-parse", "HEAD^{tree}") {
				test.Fatal("publisher changed the generated tree")
			}
		})
	}
}

func TestRejectInvalidBundles(test *testing.T) {
	for _, scenario := range []string{"extra-commit", "empty-commit", "symlink", "malformed", "oversized", "wrong-base", "merge"} {
		test.Run(scenario, func(test *testing.T) {
			fixture := newFixture(test)
			filename := filepath.Join(fixture.producer, "charts/camunda-platform-8.10/README.md")
			if scenario != "empty-commit" {
				writeFile(test, filename, "generated\n", 0644)
			}
			if scenario == "extra-commit" {
				fixture.git(fixture.producer, "commit", "--allow-empty", "-m", "extra")
			}
			if scenario == "symlink" {
				if err := os.Remove(filename); err != nil {
					test.Fatal(err)
				}
				if err := os.Symlink("../../.git/config", filename); err != nil {
					test.Fatal(err)
				}
			}
			fixture.bundle()
			bundle := filepath.Join(fixture.runner, "generated/changes.bundle")
			switch scenario {
			case "malformed":
				writeFile(test, bundle, "not a bundle\n", 0644)
			case "oversized":
				if err := os.Truncate(bundle, 52428801); err != nil {
					test.Fatal(err)
				}
			case "wrong-base":
				fixture.base = strings.Repeat("a", 40)
			case "merge":
				tree := fixture.git(fixture.producer, "rev-parse", "HEAD^{tree}")
				merge := fixture.git(fixture.producer, "commit-tree", tree, "-p", fixture.base, "-p", fixture.tip, "-m", "merge")
				fixture.git(fixture.producer, "update-ref", "HEAD", merge)
				fixture.git(fixture.producer, "bundle", "create", bundle, "HEAD", "^"+fixture.base)
			}
			if output, err := fixture.execute("Validate generated commit", false); err == nil {
				test.Fatalf("accepted invalid bundle: %q", output)
			}
		})
	}
}

func TestRejectMovedBranch(test *testing.T) {
	for _, moment := range []string{"before-validation", "after-validation", "deleted"} {
		test.Run(moment, func(test *testing.T) {
			fixture := newFixture(test)
			writeFile(test, filepath.Join(fixture.producer, "charts/camunda-platform-8.10/README.md"), "generated\n", 0644)
			fixture.bundle()
			if moment != "before-validation" {
				if output, err := fixture.execute("Validate generated commit", false); err != nil {
					test.Fatalf("validate: %v output=%q", err, output)
				}
			}
			fixture.git(fixture.producer, "checkout", "--detach", fixture.base)
			fixture.git(fixture.producer, "commit", "--allow-empty", "-m", "concurrent update")
			concurrent := fixture.git(fixture.producer, "rev-parse", "HEAD")
			fixture.git(fixture.producer, "push", fixture.remote, "HEAD:refs/heads/canary")
			if moment == "deleted" {
				fixture.git(fixture.remote, "update-ref", "-d", "refs/heads/canary")
			}
			operation := "Push generated commit"
			if moment == "before-validation" {
				operation = "Validate generated commit"
			}
			if output, err := fixture.execute(operation, false); err == nil {
				test.Fatalf("accepted moved branch: %q", output)
			}
			if moment != "deleted" && fixture.git(fixture.remote, "rev-parse", "refs/heads/canary") != concurrent {
				test.Fatal("concurrent commit was overwritten")
			}
		})
	}
}

func TestCredentialIsolationContract(test *testing.T) {
	for filename, producerName := range map[string]string{"chart-chores.yaml": "chores", "renovate-post-upgrade.yaml": "run"} {
		test.Run(filename, func(test *testing.T) {
			workflow := loadWorkflow(test, filename)
			producer := workflow.Jobs[producerName]
			if len(producer.Permissions) != 1 || producer.Permissions["contents"] != "read" {
				test.Fatal("producer must have only contents:read")
			}
			for _, current := range producer.Steps {
				content, err := yaml.Marshal(current)
				if err != nil {
					test.Fatal(err)
				}
				for _, forbidden := range []string{"secrets.", "vault-action", "github-app-token", "GH_APP_TOKEN"} {
					if strings.Contains(string(content), forbidden) {
						test.Fatalf("producer references %s", forbidden)
					}
				}
				if strings.HasPrefix(current.Uses, "actions/checkout@") && current.With["persist-credentials"] != "false" {
					test.Fatal("checkout persists credentials")
				}
				if strings.HasPrefix(current.Uses, "EndBug/add-and-commit@") && (current.With["fetch"] != "false" || current.With["push"] != "false") {
					test.Fatal("commit action has network writes enabled")
				}
			}
			publisher := workflow.Jobs["push"]
			if publisher.Needs != producerName || !strings.Contains(publisher.If, ".outputs.committed == 'true'") {
				test.Fatal("publisher must require a producer commit")
			}
			if publisher.Uses != "./.github/workflows/commit-generated-template.yaml" {
				test.Fatal("publisher must use the isolated workflow")
			}
		})
	}
	provider := loadWorkflow(test, "commit-generated-template.yaml").Jobs["push"]
	validated := false
	for _, current := range provider.Steps {
		if current.Name == "Validate generated commit" {
			validated = true
		}
		if strings.Contains(current.Uses, "vault-action") && !validated {
			test.Fatal("credentials acquired before validation")
		}
		if current.Uses != "" && !strings.HasPrefix(current.Uses, "actions/download-artifact@") && !strings.HasPrefix(current.Uses, "hashicorp/vault-action@") && !strings.HasPrefix(current.Uses, "tibdex/github-app-token@") {
			test.Fatalf("unapproved publisher action %s", current.Uses)
		}
		for _, forbidden := range []string{"GITHUB_ENV", "GITHUB_PATH", "go run", "make ", "npm ", "git checkout", "git switch"} {
			if strings.Contains(current.Run, forbidden) {
				test.Fatalf("publisher can execute repository code or persist environment: %s", forbidden)
			}
		}
	}
}
