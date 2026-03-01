package detector

import (
	"path/filepath"
	"sort"
	"strings"

	"github.com/abdidvp/openkraft/internal/domain"
)

// DetectedModuleResult is the concrete result from the detector adapter.
type DetectedModuleResult = domain.DetectedModule

// ModuleDetector implements domain.ModuleDetector for Go hexagonal projects.
// It supports two layout patterns:
//   - Per-feature:   internal/{feature}/{layer}/file.go
//   - Cross-cutting: internal/{layer}/{feature}/file.go
type ModuleDetector struct{}

func New() *ModuleDetector {
	return &ModuleDetector{}
}

// topLevelLayers are directory names recognized as architectural layers.
var topLevelLayers = map[string]string{
	"domain":         "domain",
	"application":    "application",
	"app":            "application",
	"core":           "application",
	"adapters":       "adapters",
	"adapter":        "adapters",
	"infrastructure": "adapters",
	"infra":          "adapters",
	"ports":          "domain",
}

// nestedLayers maps sub-module layer segments for per-feature layout.
var nestedLayers = []struct {
	pathSegment string
	layerName   string
}{
	{"adapters/http", "adapters"},
	{"adapters/repository", "adapters"},
	{"adapters/grpc", "adapters"},
	{"adapters/inbound", "adapters"},
	{"adapters/outbound", "adapters"},
	{"application", "application"},
	{"domain", "domain"},
}

func (d *ModuleDetector) Detect(scan *domain.ScanResult) ([]DetectedModuleResult, error) {
	layout := detectLayout(scan.GoFiles)
	scan.Layout = layout

	var modules []DetectedModuleResult
	switch layout {
	case domain.LayoutCrossCutting:
		modules = detectCrossCutting(scan.GoFiles)
	default:
		modules = detectPerFeature(scan.GoFiles)
	}

	sort.Slice(modules, func(i, j int) bool {
		return modules[i].Name < modules[j].Name
	})
	return modules, nil
}

// detectLayout classifies the project by inspecting second-level segments
// under internal/. If most are known layer names, it's cross-cutting.
func detectLayout(goFiles []string) domain.ArchLayout {
	secondSegments := map[string]bool{}
	for _, f := range goFiles {
		parts := strings.Split(filepath.ToSlash(f), "/")
		idx := sliceIndex(parts, "internal")
		if idx == -1 || idx+1 >= len(parts) {
			continue
		}
		secondSegments[parts[idx+1]] = true
	}

	if len(secondSegments) == 0 {
		return domain.LayoutPerFeature
	}

	layerCount := 0
	for seg := range secondSegments {
		if _, ok := topLevelLayers[seg]; ok {
			layerCount++
		}
	}

	if float64(layerCount)/float64(len(secondSegments)) >= 0.5 {
		return domain.LayoutCrossCutting
	}
	return domain.LayoutPerFeature
}

// detectPerFeature handles internal/{feature}/{layer}/file.go layouts.
func detectPerFeature(goFiles []string) []DetectedModuleResult {
	moduleMap := make(map[string]*DetectedModuleResult)
	layerSet := make(map[string]map[string]bool)

	for _, f := range goFiles {
		moduleName, layer, ok := parsePerFeaturePath(f)
		if !ok {
			continue
		}

		m, exists := moduleMap[moduleName]
		if !exists {
			m = &DetectedModuleResult{
				Name: moduleName,
				Path: filepath.Join("internal", moduleName),
			}
			moduleMap[moduleName] = m
			layerSet[moduleName] = make(map[string]bool)
		}
		m.Files = append(m.Files, f)
		if layer != "" {
			layerSet[moduleName][layer] = true
		}
	}

	modules := make([]DetectedModuleResult, 0, len(moduleMap))
	for name, m := range moduleMap {
		layers := make([]string, 0, len(layerSet[name]))
		for l := range layerSet[name] {
			layers = append(layers, l)
		}
		sort.Strings(layers)
		m.Layers = layers
		modules = append(modules, *m)
	}
	return modules
}

// parsePerFeaturePath extracts module name and layer from per-feature paths.
func parsePerFeaturePath(filePath string) (moduleName, layer string, ok bool) {
	parts := strings.Split(filepath.ToSlash(filePath), "/")
	idx := sliceIndex(parts, "internal")
	if idx == -1 || idx+1 >= len(parts) {
		return "", "", false
	}

	moduleName = parts[idx+1]
	remaining := strings.Join(parts[idx+2:], "/")

	for _, kl := range nestedLayers {
		if strings.HasPrefix(remaining, kl.pathSegment+"/") || remaining == kl.pathSegment {
			return moduleName, kl.layerName, true
		}
	}

	if len(parts) > idx+2 {
		return moduleName, "", true
	}
	return "", "", false
}

// detectCrossCutting handles internal/{layer}/{feature}/file.go layouts.
// Modules are feature sub-packages within each layer. Files directly under
// a layer (e.g., internal/domain/ports.go) belong to a root module for that layer.
func detectCrossCutting(goFiles []string) []DetectedModuleResult {
	type moduleBuilder struct {
		layers map[string]bool
		files  []string
	}
	builders := map[string]*moduleBuilder{}

	for _, f := range goFiles {
		parts := strings.Split(filepath.ToSlash(f), "/")
		idx := sliceIndex(parts, "internal")
		if idx == -1 || idx+1 >= len(parts) {
			continue
		}

		rawLayer := parts[idx+1]
		normalizedLayer, isLayer := topLevelLayers[rawLayer]
		if !isLayer {
			continue
		}

		remaining := parts[idx+2:] // everything after the layer

		featureName := resolveFeatureName(rawLayer, remaining)

		mb, ok := builders[featureName]
		if !ok {
			mb = &moduleBuilder{layers: map[string]bool{}}
			builders[featureName] = mb
		}
		mb.layers[normalizedLayer] = true
		mb.files = append(mb.files, f)
	}

	modules := make([]DetectedModuleResult, 0, len(builders))
	for name, mb := range builders {
		layers := make([]string, 0, len(mb.layers))
		for l := range mb.layers {
			layers = append(layers, l)
		}
		sort.Strings(layers)
		modules = append(modules, DetectedModuleResult{
			Name:   name,
			Path:   "internal/" + name,
			Layers: layers,
			Files:  mb.files,
		})
	}
	return modules
}

// resolveFeatureName determines the feature sub-package name from the path
// segments after the layer directory.
func resolveFeatureName(rawLayer string, remaining []string) string {
	// File directly under layer: internal/domain/ports.go → feature = "domain"
	if len(remaining) <= 1 {
		return rawLayer
	}

	// For adapters with inbound/outbound sub-dirs, go one level deeper.
	// internal/adapters/outbound/scanner/scanner.go → feature = "scanner"
	if (rawLayer == "adapters" || rawLayer == "adapter") && len(remaining) >= 3 {
		direction := remaining[0]
		if direction == "inbound" || direction == "outbound" {
			return remaining[1]
		}
	}

	// Default: first sub-directory under layer.
	// internal/domain/scoring/code_health.go → feature = "scoring"
	return remaining[0]
}

func sliceIndex(s []string, target string) int {
	for i, v := range s {
		if v == target {
			return i
		}
	}
	return -1
}
