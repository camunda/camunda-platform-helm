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

package versionmatrix

import (
	"bytes"
	"testing"
)

func TestUpsertImagesEmpty(t *testing.T) {
	got, err := UpsertImages([]byte("[]\n"), "13.4.0", []string{
		"docker.io/camunda/camunda:8.10.0",
		"docker.io/camunda/identity:8.10.0",
	}, nil)
	if err != nil {
		t.Fatalf("UpsertImages: %v", err)
	}
	want := `[
  {
    "chart_version": "13.4.0",
    "chart_images": [
      "docker.io/camunda/camunda:8.10.0",
      "docker.io/camunda/identity:8.10.0"
    ]
  }
]
`
	if string(got) != want {
		t.Errorf("UpsertImages empty:\n got: %q\nwant: %q", got, want)
	}
}

func TestUpsertImagesEmptyStringInput(t *testing.T) {
	// `test -f file || echo '[]'` means an absent/empty file is treated as [].
	got, err := UpsertImages([]byte(""), "13.4.0", []string{"busybox:1.36"}, nil)
	if err != nil {
		t.Fatalf("UpsertImages: %v", err)
	}
	want := `[
  {
    "chart_version": "13.4.0",
    "chart_images": [
      "busybox:1.36"
    ]
  }
]
`
	if string(got) != want {
		t.Errorf("UpsertImages empty-string:\n got: %q\nwant: %q", got, want)
	}
}

func TestUpsertImagesPreservesSiblingEnterprise(t *testing.T) {
	// A DIFFERENT version entry that carries chart_enterprise_images must survive
	// byte-for-byte (re-indented), while the target version is appended.
	existing := `[
  {
    "chart_version": "13.3.0",
    "chart_images": [
      "docker.io/camunda/camunda:8.10.0"
    ],
    "chart_enterprise_images": [
      "registry.camunda.cloud/camunda/camunda:8.10.0"
    ]
  }
]
`
	got, err := UpsertImages([]byte(existing), "13.4.0", []string{"docker.io/camunda/identity:8.10.1"}, nil)
	if err != nil {
		t.Fatalf("UpsertImages: %v", err)
	}
	want := `[
  {
    "chart_version": "13.3.0",
    "chart_images": [
      "docker.io/camunda/camunda:8.10.0"
    ],
    "chart_enterprise_images": [
      "registry.camunda.cloud/camunda/camunda:8.10.0"
    ]
  },
  {
    "chart_version": "13.4.0",
    "chart_images": [
      "docker.io/camunda/identity:8.10.1"
    ]
  }
]
`
	if string(got) != want {
		t.Errorf("UpsertImages preserve-sibling:\n got: %q\nwant: %q", got, want)
	}
}

func TestUpsertImagesReplacesSameVersion(t *testing.T) {
	// The bash drops the whole matching entry (losing its chart_enterprise_images)
	// then appends a fresh {chart_version, chart_images}. Replicate that exactly.
	existing := `[
  {
    "chart_version": "13.4.0",
    "chart_images": [
      "docker.io/camunda/camunda:OLD"
    ],
    "chart_enterprise_images": [
      "registry.camunda.cloud/camunda/camunda:OLD"
    ]
  }
]
`
	got, err := UpsertImages([]byte(existing), "13.4.0", []string{"docker.io/camunda/camunda:NEW"}, nil)
	if err != nil {
		t.Fatalf("UpsertImages: %v", err)
	}
	want := `[
  {
    "chart_version": "13.4.0",
    "chart_images": [
      "docker.io/camunda/camunda:NEW"
    ]
  }
]
`
	if string(got) != want {
		t.Errorf("UpsertImages replace-same-version:\n got: %q\nwant: %q", got, want)
	}
}

func TestUpsertImagesNilIsEmptyArray(t *testing.T) {
	got, err := UpsertImages([]byte("[]"), "1.0.0", nil, nil)
	if err != nil {
		t.Fatalf("UpsertImages: %v", err)
	}
	want := `[
  {
    "chart_version": "1.0.0",
    "chart_images": []
  }
]
`
	if string(got) != want {
		t.Errorf("UpsertImages nil-images:\n got: %q\nwant: %q", got, want)
	}
}

func TestUpsertImagesWritesEnterprise(t *testing.T) {
	got, err := UpsertImages([]byte("[]"), "13.4.0",
		[]string{"docker.io/camunda/camunda:8.10.0"},
		[]string{"registry.camunda.cloud/camunda/camunda:8.10.0"})
	if err != nil {
		t.Fatalf("UpsertImages: %v", err)
	}
	want := `[
  {
    "chart_version": "13.4.0",
    "chart_images": [
      "docker.io/camunda/camunda:8.10.0"
    ],
    "chart_enterprise_images": [
      "registry.camunda.cloud/camunda/camunda:8.10.0"
    ]
  }
]
`
	if string(got) != want {
		t.Errorf("UpsertImages enterprise:\n got: %q\nwant: %q", got, want)
	}
}

func TestUpsertImagesPreservesReleaseFacts(t *testing.T) {
	// An image re-derivation (e.g. a repeated RC promotion) must not erase
	// release-time facts already recorded on the entry.
	existing := `[
  {
    "chart_version": "13.4.0",
    "chart_images": ["docker.io/camunda/camunda:8.10.0"],
    "release_date": "2026-07-08",
    "helm_cli": "3.20.2",
    "release_tag": "camunda-platform-8.8-13.4.0"
  }
]
`
	got, err := UpsertImages([]byte(existing), "13.4.0",
		[]string{"docker.io/camunda/camunda:8.10.1"}, nil)
	if err != nil {
		t.Fatalf("UpsertImages: %v", err)
	}
	for _, want := range []string{
		`"release_date": "2026-07-08"`,
		`"helm_cli": "3.20.2"`,
		`"release_tag": "camunda-platform-8.8-13.4.0"`,
		`"docker.io/camunda/camunda:8.10.1"`,
	} {
		if !bytes.Contains(got, []byte(want)) {
			t.Errorf("UpsertImages dropped %q:\n%s", want, got)
		}
	}
	if bytes.Contains(got, []byte("8.10.0\"")) {
		t.Errorf("UpsertImages kept stale image list:\n%s", got)
	}
}

func TestUpsertEntryAndFindEntry(t *testing.T) {
	entry := ChartEntry{
		ChartVersion: "14.7.0",
		ChartImages:  []string{"docker.io/camunda/camunda:8.9.12"},
		ReleaseDate:  "2026-07-17",
		HelmCLI:      "3.20.2,4.2.3",
		ReleaseTag:   "camunda-platform-8.9-14.7.0",
	}
	out, err := UpsertEntry([]byte("[]"), entry)
	if err != nil {
		t.Fatalf("UpsertEntry: %v", err)
	}
	got, ok, err := FindEntry(out, "14.7.0")
	if err != nil || !ok {
		t.Fatalf("FindEntry: ok=%v err=%v", ok, err)
	}
	if got.ReleaseDate != "2026-07-17" || got.HelmCLI != "3.20.2,4.2.3" || got.ReleaseTag != entry.ReleaseTag {
		t.Errorf("FindEntry roundtrip mismatch: %+v", got)
	}
	if _, ok, _ := FindEntry(out, "0.0.1"); ok {
		t.Errorf("FindEntry: unexpected hit for absent version")
	}
	if _, err := UpsertEntry([]byte("[]"), ChartEntry{}); err == nil {
		t.Errorf("UpsertEntry: want error for empty chart_version")
	}
}

func TestEncodeEntries(t *testing.T) {
	out, err := EncodeEntries([]ChartEntry{
		{ChartVersion: "1.0.0", ReleaseDate: "2026-01-01"},
	})
	if err != nil {
		t.Fatalf("EncodeEntries: %v", err)
	}
	want := `[
  {
    "chart_version": "1.0.0",
    "chart_images": [],
    "release_date": "2026-01-01"
  }
]
`
	if string(out) != want {
		t.Errorf("EncodeEntries:\n got: %q\nwant: %q", out, want)
	}
}
