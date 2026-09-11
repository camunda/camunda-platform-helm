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
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"scripts/deploy-camunda/deploy"
)

func enterpriseRepo(t *testing.T, chartsWithOverlay ...string) string {
	t.Helper()
	root := t.TempDir()
	for _, chart := range []string{"camunda-platform-8.7", "camunda-platform-8.8", "camunda-platform-8.10"} {
		dir := filepath.Join(root, "charts", chart)
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatalf("mkdir %s: %v", dir, err)
		}
	}
	for _, chart := range chartsWithOverlay {
		path := filepath.Join(root, "charts", chart, "values-enterprise.yaml")
		if err := os.WriteFile(path, []byte("{}\n"), 0o644); err != nil {
			t.Fatalf("write %s: %v", path, err)
		}
	}
	return root
}

func stubAudit(t *testing.T, fn func(valuesFile string) ([]deploy.EnterpriseImageResult, error)) {
	t.Helper()
	orig := auditEnterpriseImages
	auditEnterpriseImages = func(_ context.Context, valuesFile, _ string) ([]deploy.EnterpriseImageResult, error) {
		return fn(valuesFile)
	}
	t.Cleanup(func() { auditEnterpriseImages = orig })
}

func stubLookPath(t *testing.T, err error) {
	t.Helper()
	orig := lookPath
	lookPath = func(file string) (string, error) { return "/usr/bin/" + file, err }
	t.Cleanup(func() { lookPath = orig })
}

func runCheckEnterpriseImages(t *testing.T, args ...string) (string, error) {
	t.Helper()
	stubLookPath(t, nil)
	root := NewRootCommand()
	root.AddCommand(newCheckEnterpriseImagesCommand())
	var buf bytes.Buffer
	root.SetOut(&buf)
	root.SetErr(&buf)
	root.SetArgs(append([]string{"check-enterprise-images"}, args...))
	err := root.Execute()
	return buf.String(), err
}

func TestCheckEnterpriseImagesCommand(t *testing.T) {
	t.Run("every chart passing exits zero", func(t *testing.T) {
		repo := enterpriseRepo(t, "camunda-platform-8.7", "camunda-platform-8.8")
		stubAudit(t, func(string) ([]deploy.EnterpriseImageResult, error) {
			return []deploy.EnterpriseImageResult{{Ref: "reg/img:1", OK: true}}, nil
		})

		out, err := runCheckEnterpriseImages(t, "--repo-root", repo)
		if err != nil {
			t.Fatalf("expected success, got %v\n%s", err, out)
		}
		for _, want := range []string{
			"✓ camunda-platform-8.7 validation passed",
			"✓ camunda-platform-8.8 validation passed",
			"Skipping camunda-platform-8.10: no values-enterprise.yaml",
		} {
			if !strings.Contains(out, want) {
				t.Errorf("output missing %q:\n%s", want, out)
			}
		}
	})

	t.Run("one failing chart still reports the passing ones and exits non-zero", func(t *testing.T) {
		repo := enterpriseRepo(t, "camunda-platform-8.7", "camunda-platform-8.8")
		stubAudit(t, func(valuesFile string) ([]deploy.EnterpriseImageResult, error) {
			if strings.Contains(valuesFile, "8.7") {
				return []deploy.EnterpriseImageResult{
					{Ref: "reg/broken:1", ChildDenied: true, Detail: "reg/broken:1 on linux/amd64: child manifest sha256:deadbeef not pullable"},
					{Ref: "reg/other:2", Detail: "reg/other:2 on linux/amd64: could not be verified"},
					{Ref: "reg/fine:3", OK: true},
				}, nil
			}
			return []deploy.EnterpriseImageResult{{Ref: "reg/img:1", OK: true}}, nil
		})

		out, err := runCheckEnterpriseImages(t, "--repo-root", repo)
		if err == nil {
			t.Fatalf("a broken image must fail the run:\n%s", out)
		}
		if !strings.Contains(out, "6804") {
			t.Errorf("an image-level failure should point at the tracking issue:\n%s", out)
		}
		for _, want := range []string{
			"✗ reg/broken:1 on linux/amd64: child manifest sha256:deadbeef not pullable",
			"✗ reg/other:2 on linux/amd64: could not be verified",
			"  ✓ reg/fine:3",
			"✗ camunda-platform-8.7 validation failed (2 of 3 images)",
			"✓ camunda-platform-8.8 validation passed",
		} {
			if !strings.Contains(out, want) {
				t.Errorf("output missing %q:\n%s", want, out)
			}
		}
	})

	t.Run("a chart without the overlay is skipped without a registry call", func(t *testing.T) {
		repo := enterpriseRepo(t)
		stubAudit(t, func(valuesFile string) ([]deploy.EnterpriseImageResult, error) {
			t.Errorf("unexpected audit of %s", valuesFile)
			return nil, nil
		})

		out, err := runCheckEnterpriseImages(t, "--repo-root", repo, "--chart-version", "8.7")
		if err != nil {
			t.Fatalf("expected success, got %v\n%s", err, out)
		}
		if !strings.Contains(out, "Skipping camunda-platform-8.7") {
			t.Errorf("output should record the skip:\n%s", out)
		}
	})

	t.Run("an empty --chart-version is not a filter", func(t *testing.T) {
		repo := enterpriseRepo(t, "camunda-platform-8.7", "camunda-platform-8.8")
		var audited []string
		stubAudit(t, func(valuesFile string) ([]deploy.EnterpriseImageResult, error) {
			audited = append(audited, valuesFile)
			return []deploy.EnterpriseImageResult{{Ref: "reg/img:1", OK: true}}, nil
		})

		if out, err := runCheckEnterpriseImages(t, "--repo-root", repo, "--chart-version", ""); err != nil {
			t.Fatalf("expected success, got %v\n%s", err, out)
		}
		if len(audited) != 2 {
			t.Fatalf("an empty filter must check every chart, audited %v", audited)
		}
	})

	t.Run("a missing docker binary is one clear error, not one per image", func(t *testing.T) {
		repo := enterpriseRepo(t, "camunda-platform-8.7")
		stubAudit(t, func(valuesFile string) ([]deploy.EnterpriseImageResult, error) {
			t.Errorf("unexpected audit of %s without docker", valuesFile)
			return nil, nil
		})
		root := NewRootCommand()
		root.AddCommand(newCheckEnterpriseImagesCommand())
		var buf bytes.Buffer
		root.SetOut(&buf)
		root.SetErr(&buf)
		root.SetArgs([]string{"check-enterprise-images", "--repo-root", repo})
		stubLookPath(t, errors.New(`exec: "docker": executable file not found in $PATH`))

		err := root.Execute()
		if err == nil {
			t.Fatal("a missing docker binary must fail the run")
		}
		if !strings.Contains(err.Error(), "docker is required") {
			t.Errorf("error should name the missing tool, got %v", err)
		}
	})

	t.Run("a chart whose overlay cannot be loaded fails without stopping the others", func(t *testing.T) {
		repo := enterpriseRepo(t, "camunda-platform-8.7", "camunda-platform-8.8")
		stubAudit(t, func(valuesFile string) ([]deploy.EnterpriseImageResult, error) {
			if strings.Contains(valuesFile, "8.7") {
				return nil, errors.New("yaml: line 3: found unexpected end of stream")
			}
			return []deploy.EnterpriseImageResult{{Ref: "reg/img:1", OK: true}}, nil
		})

		out, err := runCheckEnterpriseImages(t, "--repo-root", repo)
		if err == nil {
			t.Fatalf("an unloadable overlay must not report validation passed:\n%s", out)
		}
		for _, want := range []string{
			"found unexpected end of stream",
			"✗ camunda-platform-8.7 validation failed",
			"✓ camunda-platform-8.8 validation passed",
		} {
			if !strings.Contains(out, want) {
				t.Errorf("output missing %q:\n%s", want, out)
			}
		}
		if strings.Contains(out, "camunda-platform-8.7 validation passed") {
			t.Errorf("a load failure must never read as a pass:\n%s", out)
		}
		if strings.Contains(out, "6804") {
			t.Errorf("a load failure is not the registry defect and must not cite it:\n%s", out)
		}
	})

	t.Run("an image the index never advertised does not cite the registry defect", func(t *testing.T) {
		repo := enterpriseRepo(t, "camunda-platform-8.7")
		stubAudit(t, func(string) ([]deploy.EnterpriseImageResult, error) {
			return []deploy.EnterpriseImageResult{
				{Ref: "reg/img:1", Detail: "reg/img:1: the index advertises no linux/s390x child; it carries linux/amd64"},
			}, nil
		})

		out, err := runCheckEnterpriseImages(t, "--repo-root", repo, "--platform", "linux/s390x")
		if err == nil {
			t.Fatalf("an unmatched platform must fail:\n%s", out)
		}
		if strings.Contains(out, "6804") {
			t.Errorf("an index that never advertised the platform is not #6804:\n%s", out)
		}
	})

	t.Run("a malformed platform is rejected before any chart is read", func(t *testing.T) {
		repo := enterpriseRepo(t, "camunda-platform-8.7")
		stubAudit(t, func(valuesFile string) ([]deploy.EnterpriseImageResult, error) {
			t.Errorf("unexpected audit of %s", valuesFile)
			return nil, nil
		})
		if _, err := runCheckEnterpriseImages(t, "--repo-root", repo, "--platform", "linux"); err == nil {
			t.Fatal("a malformed --platform must fail the run")
		}
	})

	t.Run("positional arguments are rejected", func(t *testing.T) {
		repo := enterpriseRepo(t, "camunda-platform-8.7")
		if _, err := runCheckEnterpriseImages(t, "--repo-root", repo, "8.7"); err == nil {
			t.Fatal("a stray positional must not be silently ignored")
		}
	})
}

func TestEnterpriseChartDirs(t *testing.T) {
	repo := enterpriseRepo(t, "camunda-platform-8.7")

	t.Run("comma-separated and repeated flags are equivalent", func(t *testing.T) {
		want := []string{"camunda-platform-8.7", "camunda-platform-8.9"}
		for _, versions := range [][]string{{"8.7,8.9"}, {"8.7", "8.9"}, {" 8.7 ", "", "8.9"}} {
			got, err := enterpriseChartDirs(repo, splitSliceFlag(versions))
			if err != nil {
				t.Fatalf("%v: %v", versions, err)
			}
			if strings.Join(got, ",") != strings.Join(want, ",") {
				t.Errorf("%v -> %v, want %v", versions, got, want)
			}
		}
	})

	t.Run("auto-discovery is sorted and covers every chart directory", func(t *testing.T) {
		got, err := enterpriseChartDirs(repo, nil)
		if err != nil {
			t.Fatalf("discover: %v", err)
		}
		want := "camunda-platform-8.10,camunda-platform-8.7,camunda-platform-8.8"
		if strings.Join(got, ",") != want {
			t.Errorf("got %v, want %s", got, want)
		}
	})

	t.Run("discovering nothing is an error, not a silent pass", func(t *testing.T) {
		if _, err := enterpriseChartDirs(t.TempDir(), nil); err == nil {
			t.Fatal("zero discovered charts must not report success")
		}
	})
}

func splitSliceFlag(values []string) []string {
	var out []string
	for _, v := range values {
		out = append(out, strings.Split(v, ",")...)
	}
	return out
}
