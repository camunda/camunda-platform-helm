// Package util provides shared utility functions for the deploy-camunda tool.
package util

import (
	"crypto/rand"
	"math/big"
	"strings"
)

// RandomSuffixLength is the length of random suffixes generated for unique identifiers.
const RandomSuffixLength = 8

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

// generateRandomSuffix creates a random string of RandomSuffixLength characters.
func GenerateRandomSuffix() string {
	const chars = "abcdefghijklmnopqrstuvwxyz0123456789"
	result := make([]byte, RandomSuffixLength)
	for i := range result {
		num, _ := rand.Int(rand.Reader, big.NewInt(int64(len(chars))))
		result[i] = chars[num.Int64()]
	}
	return string(result)
}