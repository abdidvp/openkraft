package scoring

import (
	"fmt"
	"strings"

	"github.com/openkraft/openkraft/internal/domain"
)

// ScoreCompleteness evaluates how complete each module is relative to the
// most complete (proto-golden) module. Weight: 0.10 (10% of overall score).
//
// Sub-metrics (100 points total):
//   - file_completeness          (40 pts) — average file count ratio vs golden
//   - structural_completeness    (30 pts) — average layer count ratio vs golden
//   - documentation_completeness (30 pts) — proportional presence of test/error/ports files
func ScoreCompleteness(modules []domain.DetectedModule, analyzed map[string]*domain.AnalyzedFile) domain.CategoryScore {
	cat := domain.CategoryScore{
		Name:   "completeness",
		Weight: 0.10,
	}

	if len(modules) == 0 {
		cat.SubMetrics = []domain.SubMetric{
			{Name: "file_completeness", Points: 40, Detail: "no modules detected"},
			{Name: "structural_completeness", Points: 30, Detail: "no modules detected"},
			{Name: "documentation_completeness", Points: 30, Detail: "no modules detected"},
		}
		return cat
	}

	// Single module: nothing to compare, perfect score.
	if len(modules) == 1 {
		cat.SubMetrics = []domain.SubMetric{
			{Name: "file_completeness", Score: 40, Points: 40, Detail: "single module, nothing to compare"},
			{Name: "structural_completeness", Score: 30, Points: 30, Detail: "single module, nothing to compare"},
			{Name: "documentation_completeness", Score: 30, Points: 30, Detail: "single module, nothing to compare"},
		}
		cat.Score = 100
		return cat
	}

	// Find proto-golden: module with the most files + layers.
	goldenIdx := 0
	goldenRank := len(modules[0].Files) + len(modules[0].Layers)
	for i := 1; i < len(modules); i++ {
		rank := len(modules[i].Files) + len(modules[i].Layers)
		if rank > goldenRank {
			goldenRank = rank
			goldenIdx = i
		}
	}
	golden := modules[goldenIdx]

	sm1 := scoreFileCompleteness(modules, golden, goldenIdx)
	sm2 := scoreStructuralCompleteness(modules, golden, goldenIdx)
	sm3 := scoreDocumentationCompleteness(modules, golden, goldenIdx)

	cat.SubMetrics = []domain.SubMetric{sm1, sm2, sm3}

	total := 0
	for _, sm := range cat.SubMetrics {
		total += sm.Score
	}
	cat.Score = total

	cat.Issues = collectCompletenessIssues(modules, golden, goldenIdx)

	return cat
}

// scoreFileCompleteness (40 pts) — average (module_files / golden_files) across
// non-golden modules.
func scoreFileCompleteness(modules []domain.DetectedModule, golden domain.DetectedModule, goldenIdx int) domain.SubMetric {
	const maxPoints = 40

	sm := domain.SubMetric{
		Name:   "file_completeness",
		Points: maxPoints,
	}

	goldenFiles := len(golden.Files)
	if goldenFiles == 0 {
		sm.Detail = "golden module has no files"
		return sm
	}

	totalRatio := 0.0
	count := 0
	for i, m := range modules {
		if i == goldenIdx {
			continue
		}
		ratio := float64(len(m.Files)) / float64(goldenFiles)
		if ratio > 1.0 {
			ratio = 1.0
		}
		totalRatio += ratio
		count++
	}

	if count == 0 {
		sm.Detail = "no non-golden modules"
		return sm
	}

	avgRatio := totalRatio / float64(count)
	score := int(avgRatio * float64(maxPoints))
	if score > maxPoints {
		score = maxPoints
	}
	sm.Score = score
	sm.Detail = fmt.Sprintf("%.0f%% average file completeness vs golden (%s, %d files)", avgRatio*100, golden.Name, goldenFiles)
	return sm
}

// scoreStructuralCompleteness (30 pts) — average (module_layers / golden_layers)
// across non-golden modules.
func scoreStructuralCompleteness(modules []domain.DetectedModule, golden domain.DetectedModule, goldenIdx int) domain.SubMetric {
	const maxPoints = 30

	sm := domain.SubMetric{
		Name:   "structural_completeness",
		Points: maxPoints,
	}

	goldenLayers := len(golden.Layers)
	if goldenLayers == 0 {
		sm.Detail = "golden module has no layers"
		return sm
	}

	totalRatio := 0.0
	count := 0
	for i, m := range modules {
		if i == goldenIdx {
			continue
		}
		ratio := float64(len(m.Layers)) / float64(goldenLayers)
		if ratio > 1.0 {
			ratio = 1.0
		}
		totalRatio += ratio
		count++
	}

	if count == 0 {
		sm.Detail = "no non-golden modules"
		return sm
	}

	avgRatio := totalRatio / float64(count)
	score := int(avgRatio * float64(maxPoints))
	if score > maxPoints {
		score = maxPoints
	}
	sm.Score = score
	sm.Detail = fmt.Sprintf("%.0f%% average structural completeness vs golden (%s, %d layers)", avgRatio*100, golden.Name, goldenLayers)
	return sm
}

// scoreDocumentationCompleteness (30 pts) — checks whether modules have
// test files, error files, and ports files proportionally to the golden module.
func scoreDocumentationCompleteness(modules []domain.DetectedModule, golden domain.DetectedModule, goldenIdx int) domain.SubMetric {
	const maxPoints = 30

	sm := domain.SubMetric{
		Name:   "documentation_completeness",
		Points: maxPoints,
	}

	goldenTests := countFilesByPattern(golden.Files, "_test.go")
	goldenErrors := countFilesByPattern(golden.Files, "_errors.go", "errors.go")
	goldenPorts := countFilesByPattern(golden.Files, "_ports.go", "ports.go")
	goldenTotal := goldenTests + goldenErrors + goldenPorts

	if goldenTotal == 0 {
		// If the golden module has no doc files, all modules are equally
		// (in)complete — award full points since there is no gap.
		sm.Score = maxPoints
		sm.Detail = "golden module has no test/error/ports files; no gap to measure"
		return sm
	}

	totalRatio := 0.0
	count := 0
	for i, m := range modules {
		if i == goldenIdx {
			continue
		}

		modTests := countFilesByPattern(m.Files, "_test.go")
		modErrors := countFilesByPattern(m.Files, "_errors.go", "errors.go")
		modPorts := countFilesByPattern(m.Files, "_ports.go", "ports.go")
		modTotal := modTests + modErrors + modPorts

		ratio := float64(modTotal) / float64(goldenTotal)
		if ratio > 1.0 {
			ratio = 1.0
		}
		totalRatio += ratio
		count++
	}

	if count == 0 {
		sm.Detail = "no non-golden modules"
		return sm
	}

	avgRatio := totalRatio / float64(count)
	score := int(avgRatio * float64(maxPoints))
	if score > maxPoints {
		score = maxPoints
	}
	sm.Score = score
	sm.Detail = fmt.Sprintf("%.0f%% average documentation completeness vs golden (%s)", avgRatio*100, golden.Name)
	return sm
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

// collectCompletenessIssues generates issues for incomplete modules.
func collectCompletenessIssues(modules []domain.DetectedModule, golden domain.DetectedModule, goldenIdx int) []domain.Issue {
	var issues []domain.Issue

	goldenFiles := len(golden.Files)
	goldenLayers := len(golden.Layers)

	for i, m := range modules {
		if i == goldenIdx {
			continue
		}

		if goldenFiles > 0 {
			fileRatio := float64(len(m.Files)) / float64(goldenFiles)
			if fileRatio < 0.5 {
				issues = append(issues, domain.Issue{
					Severity: domain.SeverityWarning,
					Category: "completeness",
					Message:  fmt.Sprintf("module %q has only %d/%d files compared to golden module %q", m.Name, len(m.Files), goldenFiles, golden.Name),
				})
			}
		}

		if goldenLayers > 0 {
			layerRatio := float64(len(m.Layers)) / float64(goldenLayers)
			if layerRatio < 0.5 {
				issues = append(issues, domain.Issue{
					Severity: domain.SeverityWarning,
					Category: "completeness",
					Message:  fmt.Sprintf("module %q has only %d/%d layers compared to golden module %q", m.Name, len(m.Layers), goldenLayers, golden.Name),
				})
			}
		}
	}

	return issues
}
