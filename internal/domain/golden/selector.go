package golden

import (
	"errors"
	"strings"

	"github.com/openkraft/openkraft/internal/domain"
)

// GoldenModule represents a module ranked by its golden score.
// The golden score is a weighted composite of five dimensions that
// measure how "complete" and well-structured a module is, making it
// the best candidate to serve as a reference template.
type GoldenModule struct {
	Module         domain.DetectedModule
	Score          float64
	ScoreBreakdown map[string]float64
}

// Weights for each scoring dimension.
const (
	weightFileCompleteness  = 0.30
	weightStructuralDepth   = 0.25
	weightTestCoverage      = 0.20
	weightPatternCompliance = 0.15
	weightDocumentation     = 0.10
)

// SelectGolden ranks the given modules by golden score and returns the
// top-ranked module. It returns an error if no modules are provided.
func SelectGolden(modules []domain.DetectedModule, analyzed map[string]*domain.AnalyzedFile) (*GoldenModule, error) {
	if len(modules) == 0 {
		return nil, errors.New("no modules provided")
	}

	var best *GoldenModule
	for _, m := range modules {
		gm := ScoreModule(m, modules, analyzed)
		if best == nil || gm.Score > best.Score {
			gm := gm // capture loop variable
			best = &gm
		}
	}

	return best, nil
}

// ScoreModule computes the golden score for a single module.
// It is exported so callers can rank all modules individually.
func ScoreModule(m domain.DetectedModule, allModules []domain.DetectedModule, analyzed map[string]*domain.AnalyzedFile) GoldenModule {
	breakdown := make(map[string]float64)

	breakdown["file_completeness"] = scoreFileCompleteness(m, allModules)
	breakdown["structural_depth"] = scoreStructuralDepth(m)
	breakdown["test_coverage"] = scoreTestCoverage(m)
	breakdown["pattern_compliance"] = scorePatternCompliance(m, analyzed)
	breakdown["documentation"] = scoreDocumentation(m, analyzed)

	total := breakdown["file_completeness"]*weightFileCompleteness +
		breakdown["structural_depth"]*weightStructuralDepth +
		breakdown["test_coverage"]*weightTestCoverage +
		breakdown["pattern_compliance"]*weightPatternCompliance +
		breakdown["documentation"]*weightDocumentation

	return GoldenModule{
		Module:         m,
		Score:          total,
		ScoreBreakdown: breakdown,
	}
}

// scoreFileCompleteness normalizes the file count relative to the module with
// the most files. A module with the most files scores 1.0.
func scoreFileCompleteness(m domain.DetectedModule, allModules []domain.DetectedModule) float64 {
	maxFiles := 0
	for _, mod := range allModules {
		if len(mod.Files) > maxFiles {
			maxFiles = len(mod.Files)
		}
	}
	if maxFiles == 0 {
		return 0
	}
	return float64(len(m.Files)) / float64(maxFiles)
}

// scoreStructuralDepth normalizes the layer count. The canonical hexagonal
// architecture has 4 layers (domain, application, adapters/http, adapters/repository).
// A module with 4+ layers scores 1.0.
func scoreStructuralDepth(m domain.DetectedModule) float64 {
	const maxLayers = 4
	n := len(m.Layers)
	if n >= maxLayers {
		return 1.0
	}
	return float64(n) / float64(maxLayers)
}

// scoreTestCoverage computes the ratio of test files to non-test source files.
// A ratio of 1:1 or better scores 1.0.
func scoreTestCoverage(m domain.DetectedModule) float64 {
	testFiles := 0
	sourceFiles := 0
	for _, f := range m.Files {
		if isTestFile(f) {
			testFiles++
		} else {
			sourceFiles++
		}
	}
	if sourceFiles == 0 {
		return 0
	}
	ratio := float64(testFiles) / float64(sourceFiles)
	if ratio > 1.0 {
		ratio = 1.0
	}
	return ratio
}

// scorePatternCompliance checks whether the module follows expected patterns:
// - Has a Validate method (0.40)
// - Has constructor functions (New...) (0.30)
// - Has interfaces (0.30)
func scorePatternCompliance(m domain.DetectedModule, analyzed map[string]*domain.AnalyzedFile) float64 {
	hasValidate := false
	hasConstructor := false
	hasInterface := false

	for _, f := range m.Files {
		af, ok := analyzed[f]
		if !ok {
			continue
		}

		for _, fn := range af.Functions {
			if fn.Name == "Validate" && fn.Receiver != "" {
				hasValidate = true
			}
			if strings.HasPrefix(fn.Name, "New") && fn.Receiver == "" {
				hasConstructor = true
			}
		}

		if len(af.Interfaces) > 0 {
			hasInterface = true
		}
	}

	score := 0.0
	if hasValidate {
		score += 0.40
	}
	if hasConstructor {
		score += 0.30
	}
	if hasInterface {
		score += 0.30
	}
	return score
}

// scoreDocumentation checks for documentation-related files:
// - Has error definition files (0.50)
// - Has test files (0.50)
func scoreDocumentation(m domain.DetectedModule, analyzed map[string]*domain.AnalyzedFile) float64 {
	hasErrorFile := false
	hasTestFile := false

	for _, f := range m.Files {
		lower := strings.ToLower(f)
		if strings.Contains(lower, "error") && !isTestFile(f) {
			hasErrorFile = true
		}
		if isTestFile(f) {
			hasTestFile = true
		}
	}

	score := 0.0
	if hasErrorFile {
		score += 0.50
	}
	if hasTestFile {
		score += 0.50
	}
	return score
}

// isTestFile returns true if the file path ends with _test.go.
func isTestFile(path string) bool {
	return strings.HasSuffix(path, "_test.go")
}
