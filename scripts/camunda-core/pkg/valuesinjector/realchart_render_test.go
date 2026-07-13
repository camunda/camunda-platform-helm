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

package valuesinjector

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestRealChart810RendersHubImage(t *testing.T) {
	chartDir := filepath.Join("..", "..", "..", "..", "charts", "camunda-platform-8.10")

	if _, err := os.Stat(filepath.Join(chartDir, "values.yaml")); err != nil {
		t.Skipf("chart values.yaml absent at %s", filepath.Join(chartDir, "values.yaml"))
	}
	if _, err := exec.LookPath("helm"); err != nil {
		t.Skip("helm not installed")
	}
	tgzs, err := filepath.Glob(filepath.Join(chartDir, "charts", "*.tgz"))
	if err != nil {
		t.Fatalf("glob chart dependencies: %v", err)
	}
	if len(tgzs) == 0 {
		t.Skip("chart subchart dependencies not vendored (run make helm.dependency-update)")
	}

	raw, err := os.ReadFile(filepath.Join(chartDir, "values.yaml"))
	if err != nil {
		t.Fatalf("read values.yaml: %v", err)
	}

	const sentinel = "hub-render-sentinel"
	injected, err := MergeImageTags810(string(raw), &ValuesYAML810{
		WebModeler: &ComponentImage{Image: ImageTag{Tag: sentinel}},
	})
	if err != nil {
		t.Fatalf("inject webModeler tag: %v", err)
	}

	tmp := filepath.Join(t.TempDir(), "values.yaml")
	if err := os.WriteFile(tmp, []byte(injected), 0o644); err != nil {
		t.Fatalf("write injected values: %v", err)
	}

	cmd := exec.Command("helm", "template", "hubtest", chartDir,
		"-f", tmp,
		"--set", "webModeler.enabled=true",
		"--set", "identity.enabled=true",
		"--set", "orchestration.data.secondaryStorage.type=elasticsearch",
		"--set", "webModeler.restapi.mail.fromAddress=noreply@example.com")
	stdout, err := cmd.Output()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			t.Fatalf("helm template failed: %v\n%s", err, string(exitErr.Stderr))
		}
		t.Fatalf("helm template failed: %v", err)
	}

	out := string(stdout)
	if !strings.Contains(out, "camunda/hub:"+sentinel) {
		t.Errorf("rendered output missing camunda/hub:%s — injected webModeler.image.tag did not flow to the Camunda Hub restapi image", sentinel)
	}
	if !strings.Contains(out, "camunda/hub-websockets:"+sentinel) {
		t.Errorf("rendered output missing camunda/hub-websockets:%s — injected webModeler.image.tag did not flow to the Camunda Hub websockets image", sentinel)
	}
}
