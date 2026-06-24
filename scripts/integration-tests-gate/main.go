// Copyright 2025 Camunda Services GmbH
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package main

import (
	"fmt"
	"os"
	"time"
)

func mustEnv(key string) string {
	v := os.Getenv(key)
	if v == "" {
		fmt.Fprintf(os.Stderr, "error: required env var %s is not set\n", key)
		os.Exit(2)
	}
	return v
}

func main() {
	repo := mustEnv("GH_REPO")
	workflow := mustEnv("MATRIX_WORKFLOW")
	event := mustEnv("EVENT_NAME")
	prHead := os.Getenv("PR_HEAD_SHA")
	mgHead := os.Getenv("MG_HEAD_SHA")

	if override := os.Getenv("OVERRIDE_SHA"); override != "" {
		overrideEvent := os.Getenv("OVERRIDE_EVENT")
		if overrideEvent == "" {
			overrideEvent = "pull_request"
		}
		event = overrideEvent
		prHead = override
		mgHead = override
	}

	gate := &Gate{
		Client:               newGHCLI(repo, 60*time.Second),
		Workflow:             workflow,
		DiscoveryTries:       120,
		DiscoveryInterval:    10 * time.Second,
		PollInterval:         60 * time.Second,
		MaxConsecutiveErrors: 10,
		RegistrationTries:    120,
		RegistrationInterval: 5 * time.Second,
		RunAttemptTries:      5,
		RunAttemptBackoff:    5 * time.Second,
		RerunTries:           3,
		RerunBackoff:         10 * time.Second,
		Sleep:                time.Sleep,
		Logf: func(format string, args ...any) {
			fmt.Fprintf(os.Stderr, format+"\n", args...)
		},
	}

	if err := gate.Run(event, prHead, mgHead); err != nil {
		fmt.Fprintf(os.Stderr, "::error::%v\n", err)
		os.Exit(1)
	}
}
