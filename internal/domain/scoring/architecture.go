package scoring

import (
	"fmt"
	"strings"

	"github.com/openkraft/openkraft/internal/domain"
)

// ScoreArchitecture evaluates the structural quality of the codebase.
// It is pure domain logic: it receives data and returns a score with no I/O.
// Weight: 0.25 (25% of overall score).
func ScoreArchitecture(modules []domain.DetectedModule, scan *domain.ScanResult, analyzed map[string]*domain.AnalyzedFile) domain.CategoryScore {
	cat := domain.CategoryScore{
		Name:   "architecture",
		Weight: 0.25,
	}

	sm1 := scoreConsistentModuleStructure(modules)
	sm2 := scoreLayerSeparation(modules, analyzed)
	sm3 := scoreDependencyDirection(modules, analyzed)
	sm4 := scoreModuleBoundaryClarity(modules)
	sm5 := scoreArchitectureDocumentation(scan)

	cat.SubMetrics = []domain.SubMetric{sm1, sm2, sm3, sm4, sm5}

	total := 0
	for _, sm := range cat.SubMetrics {
		total += sm.Score
	}
	cat.Score = total

	// Collect issues from each sub-metric evaluation.
	cat.Issues = collectArchitectureIssues(modules, scan, analyzed)

	return cat
}

// scoreConsistentModuleStructure (30 pts) checks whether modules share the
// same subdirectory pattern (e.g., all have domain/, application/, adapters/).
func scoreConsistentModuleStructure(modules []domain.DetectedModule) domain.SubMetric {
	sm := domain.SubMetric{
		Name:   "consistent_module_structure",
		Points: 30,
	}

	if len(modules) == 0 {
		sm.Detail = "no modules detected"
		return sm
	}

	// Build a set of all layers seen across all modules.
	allLayers := make(map[string]bool)
	for _, m := range modules {
		for _, l := range m.Layers {
			allLayers[l] = true
		}
	}

	if len(allLayers) == 0 {
		sm.Detail = "no layers detected in any module"
		return sm
	}

	// For each module, check what fraction of the union-of-layers it has.
	totalCoverage := 0.0
	for _, m := range modules {
		layerSet := make(map[string]bool)
		for _, l := range m.Layers {
			layerSet[l] = true
		}
		matched := 0
		for l := range allLayers {
			if layerSet[l] {
				matched++
			}
		}
		totalCoverage += float64(matched) / float64(len(allLayers))
	}

	avgCoverage := totalCoverage / float64(len(modules))
	score := int(avgCoverage * float64(sm.Points))
	if score > sm.Points {
		score = sm.Points
	}
	sm.Score = score
	sm.Detail = fmt.Sprintf("%.0f%% average layer consistency across %d modules", avgCoverage*100, len(modules))
	return sm
}

// scoreLayerSeparation (25 pts) checks that domain packages do not import
// adapter packages.
func scoreLayerSeparation(modules []domain.DetectedModule, analyzed map[string]*domain.AnalyzedFile) domain.SubMetric {
	sm := domain.SubMetric{
		Name:   "layer_separation",
		Points: 25,
	}

	if len(modules) == 0 {
		sm.Detail = "no modules to evaluate"
		return sm
	}

	domainFiles := 0
	violations := 0

	for _, m := range modules {
		for _, f := range m.Files {
			// Check if this file is in a domain layer.
			if !isDomainFile(f) {
				continue
			}
			domainFiles++

			af, ok := analyzed[f]
			if !ok {
				continue
			}

			for _, imp := range af.Imports {
				if isAdapterImport(imp) {
					violations++
				}
			}
		}
	}

	if domainFiles == 0 {
		sm.Detail = "no domain files found"
		return sm
	}

	if violations == 0 {
		sm.Score = sm.Points
		sm.Detail = fmt.Sprintf("all %d domain files have clean imports", domainFiles)
	} else {
		// Deduct proportionally but don't go below 0.
		penalty := violations * 5
		sm.Score = sm.Points - penalty
		if sm.Score < 0 {
			sm.Score = 0
		}
		sm.Detail = fmt.Sprintf("%d domain file(s) import adapter packages", violations)
	}
	return sm
}

// scoreDependencyDirection (20 pts) checks that dependencies flow inward:
// domain <- application <- adapters. Application should import domain (good),
// adapters should import application or domain (good), but domain should not
// import application, and application should not import adapters.
func scoreDependencyDirection(modules []domain.DetectedModule, analyzed map[string]*domain.AnalyzedFile) domain.SubMetric {
	sm := domain.SubMetric{
		Name:   "dependency_direction",
		Points: 20,
	}

	if len(modules) == 0 {
		sm.Detail = "no modules to evaluate"
		return sm
	}

	totalChecked := 0
	violations := 0

	for _, m := range modules {
		for _, f := range m.Files {
			af, ok := analyzed[f]
			if !ok {
				continue
			}

			layer := fileLayer(f)
			if layer == "" {
				continue
			}
			totalChecked++

			for _, imp := range af.Imports {
				if violatesDependencyDirection(layer, imp) {
					violations++
				}
			}
		}
	}

	if totalChecked == 0 {
		sm.Detail = "no layered files to evaluate"
		return sm
	}

	if violations == 0 {
		sm.Score = sm.Points
		sm.Detail = fmt.Sprintf("all %d layered files follow correct dependency direction", totalChecked)
	} else {
		penalty := violations * 5
		sm.Score = sm.Points - penalty
		if sm.Score < 0 {
			sm.Score = 0
		}
		sm.Detail = fmt.Sprintf("%d dependency direction violation(s) found", violations)
	}
	return sm
}

// scoreModuleBoundaryClarity (15 pts) checks that modules live under internal/
// with clear separation.
func scoreModuleBoundaryClarity(modules []domain.DetectedModule) domain.SubMetric {
	sm := domain.SubMetric{
		Name:   "module_boundary_clarity",
		Points: 15,
	}

	if len(modules) == 0 {
		sm.Detail = "no modules detected"
		return sm
	}

	underInternal := 0
	for _, m := range modules {
		if strings.HasPrefix(m.Path, "internal/") || strings.HasPrefix(m.Path, "internal\\") {
			underInternal++
		}
	}

	ratio := float64(underInternal) / float64(len(modules))
	sm.Score = int(ratio * float64(sm.Points))
	if sm.Score > sm.Points {
		sm.Score = sm.Points
	}
	sm.Detail = fmt.Sprintf("%d/%d modules under internal/", underInternal, len(modules))
	return sm
}

// scoreArchitectureDocumentation (10 pts) checks for CLAUDE.md or other
// architecture documentation.
func scoreArchitectureDocumentation(scan *domain.ScanResult) domain.SubMetric {
	sm := domain.SubMetric{
		Name:   "architecture_documentation",
		Points: 10,
	}

	if scan == nil {
		sm.Detail = "no scan data"
		return sm
	}

	points := 0

	// CLAUDE.md is the primary architecture doc (6 pts).
	if scan.HasClaudeMD {
		points += 6
	}

	// Other AI context files add partial credit.
	if scan.HasCursorRules {
		points += 2
	}
	if scan.HasAgentsMD {
		points += 2
	}

	// Check for any architecture-related markdown files in AllFiles.
	for _, f := range scan.AllFiles {
		lower := strings.ToLower(f)
		if strings.Contains(lower, "architecture") && strings.HasSuffix(lower, ".md") {
			points += 2
			break
		}
	}

	if points > sm.Points {
		points = sm.Points
	}
	sm.Score = points
	sm.Detail = fmt.Sprintf("%d/%d documentation points", points, sm.Points)
	return sm
}

// --- helpers ---

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
// the inward dependency rule. Domain must not import application or adapters.
// Application must not import adapters.
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

// collectArchitectureIssues gathers issues found during scoring.
func collectArchitectureIssues(modules []domain.DetectedModule, scan *domain.ScanResult, analyzed map[string]*domain.AnalyzedFile) []domain.Issue {
	var issues []domain.Issue

	if len(modules) == 0 {
		issues = append(issues, domain.Issue{
			Severity: domain.SeverityWarning,
			Category: "architecture",
			Message:  "no modules detected; cannot evaluate architecture",
		})
		return issues
	}

	// Check domain-imports-adapter violations.
	for _, m := range modules {
		for _, f := range m.Files {
			if !isDomainFile(f) {
				continue
			}
			af, ok := analyzed[f]
			if !ok {
				continue
			}
			for _, imp := range af.Imports {
				if isAdapterImport(imp) {
					issues = append(issues, domain.Issue{
						Severity: domain.SeverityError,
						Category: "architecture",
						File:     f,
						Message:  fmt.Sprintf("domain file imports adapter package: %s", imp),
					})
				}
			}
		}
	}

	// Check for missing architecture docs.
	if scan != nil && !scan.HasClaudeMD {
		issues = append(issues, domain.Issue{
			Severity: domain.SeverityInfo,
			Category: "architecture",
			Message:  "no CLAUDE.md found; consider adding architecture documentation",
		})
	}

	return issues
}
