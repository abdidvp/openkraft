package detector

import (
	"path/filepath"
	"sort"
	"strings"

	"github.com/openkraft/openkraft/internal/domain"
)

// DetectedModuleResult is the concrete result from the detector adapter.
type DetectedModuleResult = domain.DetectedModule

// ModuleDetector implements domain.ModuleDetector for Go hexagonal projects.
type ModuleDetector struct{}

func New() *ModuleDetector {
	return &ModuleDetector{}
}

// knownLayers maps directory segments under a module to layer names.
var knownLayers = []struct {
	pathSegment string
	layerName   string
}{
	{"adapters/http", "adapters/http"},
	{"adapters/repository", "adapters/repository"},
	{"adapters/grpc", "adapters/grpc"},
	{"application", "application"},
	{"domain", "domain"},
}

func (d *ModuleDetector) Detect(scan *domain.ScanResult) ([]DetectedModuleResult, error) {
	moduleMap := make(map[string]*DetectedModuleResult)
	layerSet := make(map[string]map[string]bool)

	for _, f := range scan.GoFiles {
		moduleName, layer, ok := parseModulePath(f)
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

	sort.Slice(modules, func(i, j int) bool {
		return modules[i].Name < modules[j].Name
	})

	return modules, nil
}

// parseModulePath extracts module name and layer from a Go file path.
// Expected pattern: internal/{module}/{layer}/.../*.go
func parseModulePath(filePath string) (moduleName, layer string, ok bool) {
	parts := strings.Split(filepath.ToSlash(filePath), "/")

	// Find "internal" segment
	internalIdx := -1
	for i, p := range parts {
		if p == "internal" {
			internalIdx = i
			break
		}
	}
	if internalIdx == -1 || internalIdx+1 >= len(parts) {
		return "", "", false
	}

	moduleName = parts[internalIdx+1]
	remaining := strings.Join(parts[internalIdx+2:], "/")

	for _, kl := range knownLayers {
		if strings.HasPrefix(remaining, kl.pathSegment+"/") || remaining == kl.pathSegment {
			return moduleName, kl.layerName, true
		}
	}

	// File directly under module dir (no recognized layer)
	if len(parts) > internalIdx+2 {
		return moduleName, "", true
	}

	return "", "", false
}
