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

package executil

import (
	"context"
	"strings"
	"testing"
	"time"
)

const longLineBytes = 200000

var longLineShell = `head -c 200000 /dev/zero | tr "\0" A; echo; head -c 200000 /dev/zero | tr "\0" A 1>&2; echo 1>&2`

func TestRunCommand_longLines(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	err := RunCommand(ctx, "sh", []string{"-c", longLineShell}, nil, "")
	if err != nil {
		t.Fatalf("RunCommand returned error: %v", err)
	}
	if ctx.Err() != nil {
		t.Fatalf("RunCommand did not finish before context deadline: %v", ctx.Err())
	}
}

func TestRunCommandCapture_longLines(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	out, err := RunCommandCapture(ctx, "sh", []string{"-c", longLineShell}, nil, "")
	if err != nil {
		t.Fatalf("RunCommandCapture returned error: %v", err)
	}
	if ctx.Err() != nil {
		t.Fatalf("RunCommandCapture did not finish before context deadline: %v", ctx.Err())
	}

	line := strings.TrimRight(string(out), "\n")
	if len(line) != longLineBytes {
		t.Fatalf("stdout line length = %d, want %d", len(line), longLineBytes)
	}
}

func TestRunCommandCaptureStderr_longLines(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	stderr, err := RunCommandCaptureStderr(ctx, "sh", []string{"-c", longLineShell}, nil, "")
	if err != nil {
		t.Fatalf("RunCommandCaptureStderr returned error: %v", err)
	}
	if ctx.Err() != nil {
		t.Fatalf("RunCommandCaptureStderr did not finish before context deadline: %v", ctx.Err())
	}

	lines := strings.Split(strings.TrimRight(stderr, "\n"), "\n")
	if len(lines) != 1 {
		t.Fatalf("stderr line count = %d, want 1", len(lines))
	}
	if len(lines[0]) != longLineBytes {
		t.Fatalf("stderr line length = %d, want %d", len(lines[0]), longLineBytes)
	}
}
