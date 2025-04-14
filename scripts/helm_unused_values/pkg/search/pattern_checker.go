package search

import (
	"fmt"
	"strings"
)

// searchForDirectUsageOfKeyAcrossAllTemplates searches for a key in the template files
func (f *Finder) searchForDirectUsageOfKeyAcrossAllTemplates(key string) (bool, []string) {
	// Escape dots in the key for regex pattern
	escapedKey := strings.ReplaceAll(key, ".", "\\.")
	pattern := fmt.Sprintf("\\.Values\\.%s", escapedKey)

	if f.Debug {
		fmt.Println("SearchKeyInTemplates debug: Key:", key)
		fmt.Println("SearchKeyInTemplates debug: Search pattern:", pattern)
		fmt.Println("SearchKeyInTemplates debug: Templates directory:", f.TemplatesDir)
		fmt.Println("SearchKeyInTemplates debug: Escaped key:", escapedKey)
	}

	// Search for the pattern in template files
	matches := f.searchFiles(pattern, f.TemplatesDir)

	if f.Debug {
		fmt.Printf("SearchKeyInTemplates debug: Key '%s' found: %v, matches: %d\n",
			key, len(matches) > 0, len(matches))
	}

	return len(matches) > 0, matches
}

// adjustRegexPatterns customizes the regex pattern based on pattern name and key
func (f *Finder) adjustRegexPatterns(patternName string, key string) string {
	var regexPattern string
	switch {
	case strings.Contains(patternName, "with_context"):
		regexPattern = f.Registry.Regexes[patternName] + strings.ReplaceAll(key, ".", "\\.") + `\s+}`
	case strings.Contains(patternName, "toyaml"):
		regexPattern = f.Registry.Regexes[patternName] + strings.ReplaceAll(key, ".", "\\.") + `\s+\|\s+nindent`
	case strings.Contains(patternName, "imageByParams"):
		regexPattern = f.Registry.Regexes[patternName] + strings.ReplaceAll(key, ".", "\\.") + `\s*`
	case strings.Contains(patternName, "subChartImagePullSecrets"):
		regexPattern = f.Registry.Regexes[patternName] + strings.ReplaceAll(key, ".", "\\.") + "\\)\\)\\s+}"
	case strings.Contains(patternName, "security_context"):
		regexPattern = f.Registry.Regexes[patternName] + strings.ReplaceAll(key, ".", `\.`) + `.*"context"`
	case strings.Contains(patternName, "include_context"):
		regexPattern = f.Registry.Regexes[patternName] + strings.ReplaceAll(key, ".", `\.`)
	default:
		panic("Unknown pattern name: " + patternName)
	}
	return `"` + regexPattern + `"` // escaping so terminal support is better
}

// adjustKeysForHelpers transforms keys based on known helper patterns
func (f *Finder) adjustKeysForHelpers(patternName string, key string) string {
	var localKey string
	if strings.Contains(key, "identityKeycloak.postgresql") ||
		strings.Contains(key, "identityKeycloak.resources") ||
		strings.Contains(key, "identityKeycloak.containerSecurityContext") ||
		strings.Contains(key, "identityKeycloak.podSecurityContext") ||
		strings.Contains(key, "identityKeycloak.ingress"){
		localKey = strings.ReplaceAll(key, "identityKeycloak.", "identity.")
	} else if strings.Contains(key, "zeebe-gateway") {
		localKey = strings.ReplaceAll(key, "zeebe-gateway.", "zeebeGateway.")
	} else if strings.Contains(key, "serviceAccount.name") {
		localKey = strings.ReplaceAll(key, "serviceAccount.name", "serviceAccountName")
	} else {
		localKey = key
	}
	return localKey
}

// isKeyUsedWithPattern checks if a key is used with a specific pattern
func (f *Finder) isKeyUsedWithPattern(key, patternName string) (bool, string, []string) {
	if f.Debug {
		fmt.Println("IsKeyUsedWithPattern debug: Key:", key)
	}
	if patternName == "imageByParams" {
		if !strings.Contains(key, "image") {
			return false, "", nil
		}
	}
	if patternName == "include_context" {
		if !strings.Contains(key, "name") {
			return false, "", nil
		}
	}
	if patternName == "security_context" {
		if !strings.Contains(key, "SecurityContext") {
			return false, "", nil
		}
	}
	localKey := f.adjustKeysForHelpers(patternName, key)
	if patternName == "imageByParams" {
		if !strings.Contains(localKey, "image") {
			return false, "", nil
		}
	}
	parts := strings.Split(localKey, ".")
	regexPattern := f.adjustRegexPatterns(patternName, localKey)
	matches := f.searchFiles(regexPattern, f.TemplatesDir)

	for len(parts) > 1 && len(matches) == 0 {
		parts = parts[:len(parts)-1]
		regexPattern := f.adjustRegexPatterns(patternName, strings.Join(parts, "."))
		matches = f.searchFiles(regexPattern, f.TemplatesDir)

		if f.Debug {
			fmt.Println("IsKeyUsedWithPattern debug: Key:", key)
			fmt.Println("IsKeyUsedWithPattern debug: Local Key:", localKey)
			fmt.Println("IsKeyUsedWithPattern debug: PatternName:", patternName)
			fmt.Println("IsKeyUsedWithPattern debug: Parts:", parts)
			fmt.Println("IsKeyUsedWithPattern debug: Trying pattern:", regexPattern)
			fmt.Println("IsKeyUsedWithPattern debug: Matches:", matches)
		}
		if len(matches) > 0 { // Fixed: Consistent with loop condition
			break
		}
	}

	return len(matches) != 0, patternName, matches
}

