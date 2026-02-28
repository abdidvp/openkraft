package scoring_test

import (
	"testing"

	"github.com/openkraft/openkraft/internal/domain"
	"github.com/openkraft/openkraft/internal/domain/scoring"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestScoreDiscoverability_NilInputs(t *testing.T) {
	result := scoring.ScoreDiscoverability(defaultProfile(), nil, nil, nil)

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

	result := scoring.ScoreDiscoverability(defaultProfile(), modules, scan, analyzed)

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

	result := scoring.ScoreDiscoverability(defaultProfile(), modules, scan, analyzed)

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
	result := scoring.ScoreDiscoverability(defaultProfile(), nil, nil, nil)

	totalPoints := 0
	for _, sm := range result.SubMetrics {
		totalPoints += sm.Points
	}
	assert.Equal(t, 100, totalPoints, "sub-metric points should sum to 100")
}

func TestScoreDiscoverability_SharedLayersDifferentFiles(t *testing.T) {
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

	result := scoring.ScoreDiscoverability(defaultProfile(), modules, &domain.ScanResult{}, nil)

	predictable := result.SubMetrics[2]
	assert.Equal(t, "predictable_structure", predictable.Name)
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

	result := scoring.ScoreDiscoverability(defaultProfile(), modules, &domain.ScanResult{}, nil)

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

	result := scoring.ScoreDiscoverability(defaultProfile(), modules, &domain.ScanResult{}, analyzed)

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
	result := scoring.ScoreDiscoverability(defaultProfile(), nil, scan, nil)
	naming := result.SubMetrics[1]
	assert.Equal(t, "file_naming_conventions", naming.Name)
	assert.GreaterOrEqual(t, naming.Score, 22, "all-bare naming = 100%% consistent")
}

func TestScoreDiscoverability_MixedNamingReducesScore(t *testing.T) {
	scan := &domain.ScanResult{
		GoFiles: []string{"scanner.go", "detector.go", "user_handler.go", "tax_service.go", "parser.go", "config.go"},
	}
	result := scoring.ScoreDiscoverability(defaultProfile(), nil, scan, nil)
	naming := result.SubMetrics[1]
	assert.Less(t, naming.Score, 22, "mixed naming lowers score")
	assert.Greater(t, naming.Score, 10, "majority still consistent")
}

func TestScoreDiscoverability_IncomparableModulesGetFullCredit(t *testing.T) {
	modules := []domain.DetectedModule{
		{Name: "scoring", Layers: []string{"domain"}, Files: []string{"internal/domain/scoring/code_health.go"}},
		{Name: "scanner", Layers: []string{"adapters"}, Files: []string{"internal/adapters/outbound/scanner/scanner.go"}},
	}
	result := scoring.ScoreDiscoverability(defaultProfile(), modules, &domain.ScanResult{}, nil)
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

	result := scoring.ScoreDiscoverability(defaultProfile(), modules, &domain.ScanResult{}, analyzed)

	depDirection := result.SubMetrics[3]
	assert.Equal(t, "dependency_direction", depDirection.Name)
	assert.Less(t, depDirection.Score, depDirection.Points)
}

func TestScoreDiscoverability_ForcedBareNaming(t *testing.T) {
	p := domain.DefaultProfile()
	p.NamingConvention = "bare"

	scan := &domain.ScanResult{
		GoFiles: []string{"user_handler.go", "tax_service.go", "order_repo.go", "scanner.go"},
	}
	result := scoring.ScoreDiscoverability(&p, nil, scan, nil)
	naming := result.SubMetrics[1]
	// With forced "bare", only scanner.go matches. 1/4 = 25% → ~6 pts.
	assert.Less(t, naming.Score, 10, "forced bare should penalize suffixed files")
}

// --- Bug fix regression tests ---

func TestScoreDiscoverability_SuffixJaccardNotContaminatedByBareNames(t *testing.T) {
	// Two modules with same role suffixes (_model, _service) but different bare files.
	// Before the fix, bare names (credit, errors) contaminated the Jaccard set,
	// destroying similarity. After the fix, only recognized suffixes are compared.
	modules := []domain.DetectedModule{
		{
			Name:   "user",
			Path:   "internal/user",
			Layers: []string{"domain", "application"},
			Files: []string{
				"internal/user/domain/user_model.go",
				"internal/user/domain/credit.go",
				"internal/user/application/user_service.go",
			},
		},
		{
			Name:   "order",
			Path:   "internal/order",
			Layers: []string{"domain", "application"},
			Files: []string{
				"internal/order/domain/order_model.go",
				"internal/order/domain/errors.go",
				"internal/order/application/order_service.go",
			},
		},
	}
	scan := &domain.ScanResult{
		GoFiles: []string{
			"internal/user/domain/user_model.go",
			"internal/user/domain/credit.go",
			"internal/user/application/user_service.go",
			"internal/order/domain/order_model.go",
			"internal/order/domain/errors.go",
			"internal/order/application/order_service.go",
		},
	}

	result := scoring.ScoreDiscoverability(defaultProfile(), modules, scan, nil)
	predictable := result.SubMetrics[2]
	assert.Equal(t, "predictable_structure", predictable.Name)
	assert.GreaterOrEqual(t, predictable.Score, 22,
		"same role suffixes should produce high Jaccard despite different bare filenames")
}

func TestScoreDiscoverability_NamingUniquenessIssues(t *testing.T) {
	analyzed := map[string]*domain.AnalyzedFile{
		"handler.go": {
			Path: "handler.go",
			Functions: []domain.Function{
				{Name: "Handle", Exported: true, LineStart: 10},   // single-word → flagged
				{Name: "CreateUser", Exported: true, LineStart: 20}, // multi-word → not flagged
				{Name: "helper", Exported: false, LineStart: 30},    // unexported → skipped
			},
		},
	}

	result := scoring.ScoreDiscoverability(defaultProfile(), nil, nil, analyzed)

	var namingIssues []domain.Issue
	for _, iss := range result.Issues {
		if iss.SubMetric == "naming_uniqueness" {
			namingIssues = append(namingIssues, iss)
		}
	}
	require.Len(t, namingIssues, 1, "only single-word exported 'Handle' should be flagged")
	assert.Contains(t, namingIssues[0].Message, "Handle")
	assert.Equal(t, domain.SeverityInfo, namingIssues[0].Severity)
}

func TestScoreDiscoverability_MethodsWithReceiverExempt(t *testing.T) {
	// Single-word methods with a receiver get context from the type name
	// (e.g., (*User).String()). This works for any interface in any project.
	analyzed := map[string]*domain.AnalyzedFile{
		"types.go": {
			Path: "types.go",
			Functions: []domain.Function{
				{Name: "String", Exported: true, Receiver: "*User", LineStart: 5},
				{Name: "Error", Exported: true, Receiver: "*AppError", LineStart: 10},
				{Name: "Close", Exported: true, Receiver: "*Conn", LineStart: 15},
				{Name: "Execute", Exported: true, Receiver: "*Command", LineStart: 20},
			},
		},
	}

	result := scoring.ScoreDiscoverability(defaultProfile(), nil, nil, analyzed)

	for _, iss := range result.Issues {
		if iss.SubMetric == "naming_uniqueness" {
			t.Errorf("method with receiver should be exempt: %s", iss.Message)
		}
	}
}

func TestScoreDiscoverability_FileNamingConventionIssues(t *testing.T) {
	// 7 bare + 1 suffixed → dominant is bare at 87.5%, suffixed file should be flagged.
	scan := &domain.ScanResult{
		GoFiles: []string{
			"scanner.go", "detector.go", "parser.go", "renderer.go",
			"config.go", "model.go", "ports.go",
			"user_handler.go", // violator
		},
	}

	result := scoring.ScoreDiscoverability(defaultProfile(), nil, scan, nil)

	var fileIssues []domain.Issue
	for _, iss := range result.Issues {
		if iss.SubMetric == "file_naming_conventions" {
			fileIssues = append(fileIssues, iss)
		}
	}
	require.Len(t, fileIssues, 1)
	assert.Contains(t, fileIssues[0].Message, "user_handler.go")
	assert.Equal(t, domain.SeverityInfo, fileIssues[0].Severity)
}

func TestScoreDiscoverability_PredictableStructureIssues(t *testing.T) {
	// 3 modules: user and order have domain+application+adapters, payment only has domain.
	// Payment should get flagged for missing application and adapters.
	modules := []domain.DetectedModule{
		{Name: "user", Path: "internal/user", Layers: []string{"domain", "application", "adapters"},
			Files: []string{"internal/user/domain/model.go"}},
		{Name: "order", Path: "internal/order", Layers: []string{"domain", "application", "adapters"},
			Files: []string{"internal/order/domain/model.go"}},
		{Name: "payment", Path: "internal/payment", Layers: []string{"domain"},
			Files: []string{"internal/payment/domain/model.go"}},
	}

	result := scoring.ScoreDiscoverability(defaultProfile(), modules, &domain.ScanResult{}, nil)

	var structIssues []domain.Issue
	for _, iss := range result.Issues {
		if iss.SubMetric == "predictable_structure" {
			structIssues = append(structIssues, iss)
		}
	}
	assert.GreaterOrEqual(t, len(structIssues), 2,
		"payment module should be flagged for missing application and adapters layers")
	for _, iss := range structIssues {
		assert.Contains(t, iss.Message, "payment")
		assert.Equal(t, domain.SeverityInfo, iss.Severity)
	}
}

func TestScoreDiscoverability_RoundingNotTruncating(t *testing.T) {
	// All functions have perfect multi-word names → composite should round to exactly 25.
	analyzed := map[string]*domain.AnalyzedFile{
		"a.go": {
			Path: "a.go",
			Functions: []domain.Function{
				{Name: "CreateUser", Exported: true},
				{Name: "DeleteOrder", Exported: true},
				{Name: "ValidateEmail", Exported: true},
				{Name: "CalculateTotal", Exported: true},
				{Name: "ParseConfig", Exported: true},
				{Name: "RenderOutput", Exported: true},
				{Name: "BuildQuery", Exported: true},
				{Name: "HandleRequest", Exported: true},
			},
		},
	}

	result := scoring.ScoreDiscoverability(defaultProfile(), nil, nil, analyzed)
	naming := result.SubMetrics[0]
	assert.Equal(t, "naming_uniqueness", naming.Name)
	assert.Equal(t, 25, naming.Score, "perfect naming should yield exactly 25 points, not 24 from truncation")
}

func TestScoreDiscoverability_SkipsGeneratedFiles(t *testing.T) {
	analyzed := map[string]*domain.AnalyzedFile{
		"generated.go": {
			Path:        "generated.go",
			IsGenerated: true,
			Functions: []domain.Function{
				{Name: "Handle", Exported: true, LineStart: 5},
				{Name: "Run", Exported: true, LineStart: 10},
			},
		},
	}

	result := scoring.ScoreDiscoverability(defaultProfile(), nil, nil, analyzed)

	for _, iss := range result.Issues {
		if iss.SubMetric == "naming_uniqueness" {
			t.Errorf("generated file should not produce naming issues: %s", iss.Message)
		}
	}
}
