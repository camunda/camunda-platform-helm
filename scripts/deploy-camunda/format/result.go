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

package format

import (
	"encoding/json"
	"fmt"
	"io"
)

const (
	ResultAPIVersion = "deploy-camunda/v1"
	ResultKind       = "Result"

	StatusOK   = "ok"
	StatusFail = "fail"

	OutputText = "text"
	OutputJSON = "json"
)

// Result is the versioned JSON envelope shared by commands that support --format json.
type Result struct {
	APIVersion string         `json:"apiVersion"`
	Kind       string         `json:"kind"`
	Status     string         `json:"status"`
	Slots      []Slot         `json:"slots"`
	Outputs    map[string]any `json:"outputs"`
	Errors     []Error        `json:"errors"`
}

// Slot reports the preflight state of one provider-backed slot.
type Slot struct {
	Slot      string    `json:"slot"`
	Provider  string    `json:"provider"`
	Preflight Preflight `json:"preflight"`
}

// Preflight is the outcome of a slot provider's preflight.
type Preflight struct {
	OK          bool   `json:"ok"`
	Code        string `json:"code"`
	Message     string `json:"message"`
	Remediation string `json:"remediation"`
}

// Error is one failure reported in a Result.
type Error struct {
	Code        string `json:"code"`
	Message     string `json:"message"`
	Remediation string `json:"remediation"`
}

// NewResult returns an envelope with non-nil collections so they encode as [] and {}.
func NewResult() Result {
	return Result{
		APIVersion: ResultAPIVersion,
		Kind:       ResultKind,
		Status:     StatusOK,
		Slots:      []Slot{},
		Outputs:    map[string]any{},
		Errors:     []Error{},
	}
}

// ValidateOutput rejects unsupported --format values.
func ValidateOutput(f string) error {
	if f == OutputText || f == OutputJSON {
		return nil
	}
	return fmt.Errorf("unsupported --format %q (want %q or %q)", f, OutputText, OutputJSON)
}

// WriteJSON encodes r as indented JSON followed by a newline.
func WriteJSON(w io.Writer, r Result) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(r)
}
