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

//go:build unix

package e2erun

import (
	"os/exec"
	"syscall"
	"time"
)

const processGroupGrace = 15 * time.Second

// setProcessGroup starts cmd in a new process group. Cancelling the context
// sends SIGTERM to the whole group, then SIGKILL after a grace period.
func setProcessGroup(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	cmd.Cancel = func() error {
		pgid := cmd.Process.Pid
		_ = syscall.Kill(-pgid, syscall.SIGTERM)
		time.AfterFunc(processGroupGrace, func() { _ = syscall.Kill(-pgid, syscall.SIGKILL) })
		return nil
	}
}

// killProcessGroup stops processes the script left behind, such as a browser
// that outlived Playwright, so they cannot write into the next leg's reports.
func killProcessGroup(cmd *exec.Cmd) {
	if cmd.Process != nil {
		_ = syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
	}
}
