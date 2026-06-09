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
	"encoding/json"
	"fmt"
)

// UpsertImages drops any existing entry for chartVersion and appends a fresh
// {chart_version, chart_images[, chart_enterprise_images]} object.
// chart_enterprise_images is included only when enterpriseImages is non-empty
// (pass nil to omit it). Entries for other versions are preserved verbatim
// (including any extra fields), only re-indented. existing is the current file
// content; empty input is treated as an empty array. Output is 2-space-indented,
// no HTML escaping, trailing newline.
func UpsertImages(existing []byte, chartVersion string, images, enterpriseImages []string) ([]byte, error) {
	entries, err := decodeEntries(existing)
	if err != nil {
		return nil, err
	}

	kept := entries[:0]
	for _, raw := range entries {
		ver, err := entryVersion(raw)
		if err != nil {
			return nil, err
		}
		if ver != chartVersion {
			kept = append(kept, raw)
		}
	}

	if images == nil {
		images = []string{}
	}
	entry := struct {
		ChartVersion          string   `json:"chart_version"`
		ChartImages           []string `json:"chart_images"`
		ChartEnterpriseImages []string `json:"chart_enterprise_images,omitempty"`
	}{ChartVersion: chartVersion, ChartImages: images}
	if len(enterpriseImages) > 0 {
		entry.ChartEnterpriseImages = enterpriseImages
	}
	newEntry, err := json.Marshal(entry)
	if err != nil {
		return nil, fmt.Errorf("marshal version-matrix entry: %w", err)
	}
	kept = append(kept, json.RawMessage(newEntry))

	return encodeEntries(kept)
}

// decodeEntries parses the file content into raw per-entry messages. Empty or
// whitespace-only input is treated as an empty array.
func decodeEntries(existing []byte) ([]json.RawMessage, error) {
	if len(bytes.TrimSpace(existing)) == 0 {
		return []json.RawMessage{}, nil
	}
	var entries []json.RawMessage
	if err := json.Unmarshal(existing, &entries); err != nil {
		return nil, fmt.Errorf("parse version-matrix: %w", err)
	}
	return entries, nil
}

// entryVersion extracts the chart_version of a raw entry without disturbing its
// other fields.
func entryVersion(raw json.RawMessage) (string, error) {
	var probe struct {
		ChartVersion string `json:"chart_version"`
	}
	if err := json.Unmarshal(raw, &probe); err != nil {
		return "", fmt.Errorf("parse version-matrix entry: %w", err)
	}
	return probe.ChartVersion, nil
}

// encodeEntries renders the entries with 2-space indent, no HTML escaping
// (<, >, & stay literal), and a trailing newline.
func encodeEntries(entries []json.RawMessage) ([]byte, error) {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")
	if err := enc.Encode(entries); err != nil {
		return nil, fmt.Errorf("encode version-matrix: %w", err)
	}
	return buf.Bytes(), nil
}
