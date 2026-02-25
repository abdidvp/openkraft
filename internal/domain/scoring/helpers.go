package scoring

import "strings"

// isDomainFile checks if a file path is in a domain layer.
func isDomainFile(path string) bool {
	normalized := strings.ReplaceAll(path, "\\", "/")
	return strings.Contains(normalized, "/domain/")
}

// isAdapterImport checks if an import path refers to an adapter package.
func isAdapterImport(importPath string) bool {
	return strings.Contains(importPath, "/adapters/") || strings.Contains(importPath, "/adapter/")
}

// fileLayer returns the architectural layer of a file: "domain", "application", or "adapters".
func fileLayer(path string) string {
	normalized := strings.ReplaceAll(path, "\\", "/")
	switch {
	case strings.Contains(normalized, "/domain/"):
		return "domain"
	case strings.Contains(normalized, "/application/"):
		return "application"
	case strings.Contains(normalized, "/adapters/"):
		return "adapters"
	default:
		return ""
	}
}

// violatesDependencyDirection checks if an import from a given layer breaks
// the inward dependency rule.
func violatesDependencyDirection(layer, importPath string) bool {
	switch layer {
	case "domain":
		return strings.Contains(importPath, "/application/") || isAdapterImport(importPath)
	case "application":
		return isAdapterImport(importPath)
	default:
		return false
	}
}

// containsAny checks if s contains any of the given substrings.
func containsAny(s string, subs ...string) bool {
	for _, sub := range subs {
		if strings.Contains(s, sub) {
			return true
		}
	}
	return false
}

// containsString checks if a slice contains a string.
func containsString(slice []string, s string) bool {
	for _, item := range slice {
		if item == s {
			return true
		}
	}
	return false
}

// countFilesByPattern counts files whose path ends with any of the given suffixes.
func countFilesByPattern(files []string, suffixes ...string) int {
	count := 0
	for _, f := range files {
		lower := strings.ToLower(f)
		for _, s := range suffixes {
			if strings.HasSuffix(lower, s) {
				count++
				break
			}
		}
	}
	return count
}
