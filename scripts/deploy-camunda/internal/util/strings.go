// Package util provides shared utility functions for the deploy-camunda tool.
package util

import "strings"

// FirstNonEmpty returns the first non-empty string from the provided values.
// Returns an empty string if all values are empty or whitespace-only.
func FirstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}

// IsEmpty returns true if the string is empty or contains only whitespace.
func IsEmpty(s string) bool {
	return strings.TrimSpace(s) == ""
}

// IsNotEmpty returns true if the string is not empty and contains non-whitespace characters.
func IsNotEmpty(s string) bool {
	return strings.TrimSpace(s) != ""
}

