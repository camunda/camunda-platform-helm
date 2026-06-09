package versionmatrix

import (
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
