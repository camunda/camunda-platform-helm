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

package utils

import (
	"fmt"
	"os"
	"os/exec"
)

func CheckDependencies() (bool, []string) {
	var missing []string

	if _, err := exec.LookPath("yq"); err != nil {
		missing = append(missing, "yq")
	}

	if _, err := exec.LookPath("jq"); err != nil {
		missing = append(missing, "jq")
	}

	return len(missing) == 0, missing
}

func DetectRipgrep() bool {
	_, err := exec.LookPath("rg")
	return err == nil
}

func ValidateFile(file string) error {
	info, err := os.Stat(file)
	if err != nil {
		return fmt.Errorf("file %s not found: %w", file, err)
	}
	if info.IsDir() {
		return fmt.Errorf("%s is a directory, not a file", file)
	}
	return nil
}

func ValidateDirectory(dir string) error {
	info, err := os.Stat(dir)
	if err != nil {
		return fmt.Errorf("directory %s not found: %w", dir, err)
	}
	if !info.IsDir() {
		return fmt.Errorf("%s is not a directory", dir)
	}
	return nil
}
