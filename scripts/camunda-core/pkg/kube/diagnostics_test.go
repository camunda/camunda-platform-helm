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

package kube

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func useFakeKubectl(t *testing.T, script string) {
	t.Helper()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "kubectl"), []byte("#!/bin/sh\n"+script+"\n"), 0o755); err != nil {
		t.Fatalf("write fake kubectl: %v", err)
	}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
}

func TestRunKubectlTimeout(t *testing.T) {
	tests := []struct {
		name    string
		script  string
		timeout time.Duration
		wantOut string
		wantErr []string
	}{
		{
			name:    "success returns stdout",
			script:  "echo pod-a",
			timeout: 5 * time.Second,
			wantOut: "pod-a",
		},
		{
			name:    "deadline is reported as a timeout and keeps partial stdout",
			script:  "echo partial; exec sleep 10",
			timeout: time.Second,
			wantOut: "partial",
			wantErr: []string{"kubectl timed out after 1s"},
		},
		{
			name:    "failure carries stderr",
			script:  "echo 'Error from server (Forbidden): pods is forbidden' >&2; exit 1",
			timeout: 5 * time.Second,
			wantErr: []string{"exit status 1", "Error from server (Forbidden): pods is forbidden"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			useFakeKubectl(t, tt.script)

			out, err := runKubectlTimeout(context.Background(), nil, tt.timeout)
			if out != tt.wantOut {
				t.Errorf("stdout = %q, want %q", out, tt.wantOut)
			}
			if len(tt.wantErr) == 0 {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				return
			}
			if err == nil {
				t.Fatal("expected an error")
			}
			for _, want := range tt.wantErr {
				if !strings.Contains(err.Error(), want) {
					t.Errorf("error %q does not contain %q", err, want)
				}
			}
		})
	}
}
