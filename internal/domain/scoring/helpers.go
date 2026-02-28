package scoring

import "strings"

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
