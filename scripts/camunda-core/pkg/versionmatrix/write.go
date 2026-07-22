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

// UpsertImages replaces the image sets of chartVersion's entry, creating the
// entry when absent. Release-time facts already recorded on the entry
// (release_date, helm_cli, release_tag) are preserved — an image re-derivation
// must never erase a stamped release fact. chart_enterprise_images is included
// only when enterpriseImages is non-empty (pass nil to omit it). Entries for
// other versions are preserved verbatim (including any extra fields), only
// re-indented. existing is the current file content; empty input is treated as
// an empty array. Output is 2-space-indented, no HTML escaping, trailing
// newline.
func UpsertImages(existing []byte, chartVersion string, images, enterpriseImages []string) ([]byte, error) {
	entry, _, err := FindEntry(existing, chartVersion)
	if err != nil {
		return nil, err
	}
	entry.ChartVersion = chartVersion
	if images == nil {
		images = []string{}
	}
	entry.ChartImages = images
	entry.ChartEnterpriseImages = nil
	if len(enterpriseImages) > 0 {
		entry.ChartEnterpriseImages = enterpriseImages
	}
	return UpsertEntry(existing, entry)
}

// FindEntry returns chartVersion's entry parsed from the file content, and
// whether it exists. A missing entry yields a zero ChartEntry and ok=false.
func FindEntry(existing []byte, chartVersion string) (ChartEntry, bool, error) {
	entries, err := decodeEntries(existing)
	if err != nil {
		return ChartEntry{}, false, err
	}
	for _, raw := range entries {
		ver, err := entryVersion(raw)
		if err != nil {
			return ChartEntry{}, false, err
		}
		if ver != chartVersion {
			continue
		}
		var entry ChartEntry
		if err := json.Unmarshal(raw, &entry); err != nil {
			return ChartEntry{}, false, fmt.Errorf("parse version-matrix entry %s: %w", chartVersion, err)
		}
		return entry, true, nil
	}
	return ChartEntry{}, false, nil
}

// UpsertEntry drops any existing entry for entry.ChartVersion and appends the
// given entry, freshly marshaled. Entries for other versions are preserved
// verbatim (including any extra fields), only re-indented.
func UpsertEntry(existing []byte, entry ChartEntry) ([]byte, error) {
	if entry.ChartVersion == "" {
		return nil, fmt.Errorf("upsert version-matrix entry: chart_version is empty")
	}
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
		if ver != entry.ChartVersion {
			kept = append(kept, raw)
		}
	}

	if entry.ChartImages == nil {
		entry.ChartImages = []string{}
	}
	newEntry, err := json.Marshal(entry)
	if err != nil {
		return nil, fmt.Errorf("marshal version-matrix entry: %w", err)
	}
	kept = append(kept, json.RawMessage(newEntry))

	return encodeEntries(kept)
}

// EncodeEntries renders a full entry list with the file's canonical encoding
// (2-space indent, no HTML escaping, trailing newline). Used by whole-file
// rewrites such as the historical backfill.
func EncodeEntries(entries []ChartEntry) ([]byte, error) {
	raw := make([]json.RawMessage, len(entries))
	for i, e := range entries {
		if e.ChartImages == nil {
			e.ChartImages = []string{}
		}
		m, err := json.Marshal(e)
		if err != nil {
			return nil, fmt.Errorf("marshal version-matrix entry: %w", err)
		}
		raw[i] = m
	}
	return encodeEntries(raw)
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
