package scoring_test

import (
	"testing"

	"github.com/openkraft/openkraft/internal/domain"
	"github.com/openkraft/openkraft/internal/domain/scoring"
	"github.com/stretchr/testify/assert"
)

func TestScoreDiscoverability_NilInputs(t *testing.T) {
	result := scoring.ScoreDiscoverability(nil, nil, nil)

	assert.Equal(t, "discoverability", result.Name)
	assert.Equal(t, 0.20, result.Weight)
	assert.Len(t, result.SubMetrics, 4)
	assert.GreaterOrEqual(t, result.Score, 0)
	assert.LessOrEqual(t, result.Score, 100)
}

func TestScoreDiscoverability_EmptyInputs(t *testing.T) {
	modules := []domain.DetectedModule{}
	scan := &domain.ScanResult{}
	analyzed := map[string]*domain.AnalyzedFile{}

	result := scoring.ScoreDiscoverability(modules, scan, analyzed)

	assert.Equal(t, "discoverability", result.Name)
	assert.Equal(t, 0.20, result.Weight)
	assert.Len(t, result.SubMetrics, 4)
	assert.Equal(t, 0, result.Score)
}

func TestScoreDiscoverability_WellStructuredProject(t *testing.T) {
	modules := []domain.DetectedModule{
		{
			Name:   "user",
			Path:   "internal/user",
			Layers: []string{"domain", "application", "adapters"},
			Files: []string{
				"internal/user/domain/user_model.go",
				"internal/user/application/user_service.go",
				"internal/user/adapters/user_handler.go",
			},
		},
		{
			Name:   "order",
			Path:   "internal/order",
			Layers: []string{"domain", "application", "adapters"},
			Files: []string{
				"internal/order/domain/order_model.go",
				"internal/order/application/order_service.go",
				"internal/order/adapters/order_handler.go",
			},
		},
	}
	scan := &domain.ScanResult{
		GoFiles: []string{
			"internal/user/domain/user_model.go",
			"internal/user/application/user_service.go",
			"internal/user/adapters/user_handler.go",
			"internal/order/domain/order_model.go",
			"internal/order/application/order_service.go",
			"internal/order/adapters/order_handler.go",
		},
	}
	analyzed := map[string]*domain.AnalyzedFile{
		"internal/user/domain/user_model.go": {
			Path:    "internal/user/domain/user_model.go",
			Package: "domain",
			Functions: []domain.Function{
				{Name: "CreateUser", Exported: true},
				{Name: "ValidateEmail", Exported: true},
			},
		},
		"internal/order/domain/order_model.go": {
			Path:    "internal/order/domain/order_model.go",
			Package: "domain",
			Functions: []domain.Function{
				{Name: "CreateOrder", Exported: true},
				{Name: "CalculateTotal", Exported: true},
			},
		},
	}

	result := scoring.ScoreDiscoverability(modules, scan, analyzed)

	assert.Equal(t, "discoverability", result.Name)
	assert.Equal(t, 0.20, result.Weight)
	assert.Len(t, result.SubMetrics, 4)
	assert.Greater(t, result.Score, 0)
	assert.LessOrEqual(t, result.Score, 100)

	expectedNames := []string{
		"naming_uniqueness", "file_naming_conventions",
		"predictable_structure", "dependency_direction",
	}
	for i, name := range expectedNames {
		assert.Equal(t, name, result.SubMetrics[i].Name)
	}
}

func TestScoreDiscoverability_SubMetricPointsSum(t *testing.T) {
	result := scoring.ScoreDiscoverability(nil, nil, nil)

	totalPoints := 0
	for _, sm := range result.SubMetrics {
		totalPoints += sm.Points
	}
	assert.Equal(t, 100, totalPoints, "sub-metric points should sum to 100")
}

func TestScoreDiscoverability_SharedLayersDifferentFiles(t *testing.T) {
	// Modules share the same layers but have different domain files.
	// Should score high on predictable_structure.
	modules := []domain.DetectedModule{
		{
			Name:   "user",
			Path:   "internal/user",
			Layers: []string{"domain", "application", "adapters"},
			Files: []string{
				"internal/user/domain/user.go",
				"internal/user/application/user_service.go",
				"internal/user/adapters/user_handler.go",
			},
		},
		{
			Name:   "order",
			Path:   "internal/order",
			Layers: []string{"domain", "application", "adapters"},
			Files: []string{
				"internal/order/domain/order.go",
				"internal/order/application/order_service.go",
				"internal/order/adapters/order_handler.go",
			},
		},
	}

	result := scoring.ScoreDiscoverability(modules, &domain.ScanResult{}, nil)

	predictable := result.SubMetrics[2]
	assert.Equal(t, "predictable_structure", predictable.Name)
	// Same layers, same suffixes, same file counts → high score.
	assert.GreaterOrEqual(t, predictable.Score, 20, "modules with identical layers should score high")
}

func TestScoreDiscoverability_PredictableStructureWithoutSuffixes(t *testing.T) {
	modules := []domain.DetectedModule{
		{
			Name:   "user",
			Path:   "internal/user",
			Layers: []string{"domain", "application"},
			Files: []string{
				"internal/user/domain/model.go",
				"internal/user/application/service.go",
			},
		},
		{
			Name:   "order",
			Path:   "internal/order",
			Layers: []string{"domain", "application"},
			Files: []string{
				"internal/order/domain/model.go",
				"internal/order/application/service.go",
			},
		},
	}

	result := scoring.ScoreDiscoverability(modules, &domain.ScanResult{}, nil)

	predictable := result.SubMetrics[2]
	assert.Equal(t, "predictable_structure", predictable.Name)
	assert.GreaterOrEqual(t, predictable.Score, 20,
		"modules with matching filenames should score high even without suffixes")
}

func TestScoreDiscoverability_TestFileImportsNotCounted(t *testing.T) {
	modules := []domain.DetectedModule{
		{
			Name:   "user",
			Path:   "internal/user",
			Layers: []string{"domain"},
			Files: []string{
				"internal/user/domain/model.go",
				"internal/user/domain/model_test.go",
			},
		},
	}
	analyzed := map[string]*domain.AnalyzedFile{
		"internal/user/domain/model.go": {
			Path:    "internal/user/domain/model.go",
			Package: "domain",
			Imports: []string{"fmt"},
		},
		"internal/user/domain/model_test.go": {
			Path:    "internal/user/domain/model_test.go",
			Package: "domain_test",
			Imports: []string{"internal/user/adapters/db"},
		},
	}

	result := scoring.ScoreDiscoverability(modules, &domain.ScanResult{}, analyzed)

	depDirection := result.SubMetrics[3]
	assert.Equal(t, "dependency_direction", depDirection.Name)
	assert.Equal(t, depDirection.Points, depDirection.Score,
		"test file imports should not count as violations")

	for _, issue := range result.Issues {
		assert.NotContains(t, issue.File, "_test.go",
			"test files should not generate dependency violations")
	}
}

func TestScoreDiscoverability_BareNamingConsistency(t *testing.T) {
	scan := &domain.ScanResult{
		GoFiles: []string{"scanner.go", "detector.go", "parser.go", "renderer.go", "config.go", "model.go", "ports.go", "helpers.go"},
	}
	result := scoring.ScoreDiscoverability(nil, scan, nil)
	naming := result.SubMetrics[1]
	assert.Equal(t, "file_naming_conventions", naming.Name)
	assert.GreaterOrEqual(t, naming.Score, 22, "all-bare naming = 100%% consistent")
}

func TestScoreDiscoverability_MixedNamingReducesScore(t *testing.T) {
	scan := &domain.ScanResult{
		GoFiles: []string{"scanner.go", "detector.go", "user_handler.go", "tax_service.go", "parser.go", "config.go"},
	}
	result := scoring.ScoreDiscoverability(nil, scan, nil)
	naming := result.SubMetrics[1]
	assert.Less(t, naming.Score, 22, "mixed naming lowers score")
	assert.Greater(t, naming.Score, 10, "majority still consistent")
}

func TestScoreDiscoverability_IncomparableModulesGetFullCredit(t *testing.T) {
	modules := []domain.DetectedModule{
		{Name: "scoring", Layers: []string{"domain"}, Files: []string{"internal/domain/scoring/code_health.go"}},
		{Name: "scanner", Layers: []string{"adapters"}, Files: []string{"internal/adapters/outbound/scanner/scanner.go"}},
	}
	result := scoring.ScoreDiscoverability(modules, &domain.ScanResult{}, nil)
	predictable := result.SubMetrics[2]
	assert.Equal(t, "predictable_structure", predictable.Name)
	assert.Equal(t, 25, predictable.Score, "no comparable pairs = full credit")
}

func TestScoreDiscoverability_DependencyViolation(t *testing.T) {
	modules := []domain.DetectedModule{
		{
			Name:   "user",
			Path:   "internal/user",
			Layers: []string{"domain"},
			Files:  []string{"internal/user/domain/model.go"},
		},
	}
	analyzed := map[string]*domain.AnalyzedFile{
		"internal/user/domain/model.go": {
			Path:    "internal/user/domain/model.go",
			Package: "domain",
			Imports: []string{"internal/user/adapters/db"},
		},
	}

	result := scoring.ScoreDiscoverability(modules, &domain.ScanResult{}, analyzed)

	// dependency_direction sub-metric should have reduced score due to domain importing adapters.
	depDirection := result.SubMetrics[3]
	assert.Equal(t, "dependency_direction", depDirection.Name)
	assert.Less(t, depDirection.Score, depDirection.Points)
}
