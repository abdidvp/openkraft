package scoring

import (
	"strings"

	"github.com/abdidvp/openkraft/internal/domain"
)

// buildLayerMap constructs a map from directory name to canonical layer name,
// using both canonical names and profile aliases.
func buildLayerMap(profile *domain.ScoringProfile) map[string]string {
	m := map[string]string{
		"domain":      "domain",
		"application": "application",
		"adapters":    "adapters",
	}
	if profile != nil {
		for alias, canonical := range profile.LayerAliases {
			m[alias] = canonical
		}
	}
	return m
}

// fileLayer returns the architectural layer of a file: "domain", "application", or "adapters".
func fileLayer(path string, profile *domain.ScoringProfile) string {
	normalized := strings.ReplaceAll(path, "\\", "/")
	layers := buildLayerMap(profile)
	for name, canonical := range layers {
		if strings.Contains(normalized, "/"+name+"/") {
			return canonical
		}
	}
	return ""
}

// importLayer returns the architectural layer of an import path.
func importLayer(importPath string, profile *domain.ScoringProfile) string {
	layers := buildLayerMap(profile)
	for name, canonical := range layers {
		if strings.Contains(importPath, "/"+name+"/") || strings.HasSuffix(importPath, "/"+name) {
			return canonical
		}
	}
	return "unknown"
}

// violatesDependencyDirection checks if an import from a given layer breaks
// the inward dependency rule.
func violatesDependencyDirection(layer, importPath string, profile *domain.ScoringProfile) bool {
	impLayer := importLayer(importPath, profile)
	switch layer {
	case "domain":
		return impLayer == "application" || impLayer == "adapters"
	case "application":
		return impLayer == "adapters"
	default:
		return false
	}
}
