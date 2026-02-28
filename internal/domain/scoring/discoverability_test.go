package scoring_test

import (
	"fmt"
	"strings"
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
				{Name: "Handle", Exported: true, LineStart: 10},     // single-word → flagged
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
	// All functions have perfect multi-word names with domain-specific words.
	// With domain vocab from structs, specificity should be high enough for near-max score.
	analyzed := map[string]*domain.AnalyzedFile{
		"a.go": {
			Path:    "a.go",
			Structs: []string{"User", "Order", "Email", "Config", "Query"},
			Functions: []domain.Function{
				{Name: "CreateUser", Exported: true},
				{Name: "DeleteOrder", Exported: true},
				{Name: "ValidateEmail", Exported: true},
				{Name: "CalculateTotal", Exported: true},
				{Name: "ParseConfig", Exported: true},
				{Name: "RenderOutput", Exported: true},
				{Name: "BuildQuery", Exported: true},
				{Name: "FormatRecord", Exported: true},
			},
		},
	}

	result := scoring.ScoreDiscoverability(defaultProfile(), nil, nil, analyzed)
	naming := result.SubMetrics[0]
	assert.Equal(t, "naming_uniqueness", naming.Name)
	assert.GreaterOrEqual(t, naming.Score, 22, "well-named functions with domain vocab should score high")
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

// --- Comprehensive tests for the world-class overhaul ---

func TestSeverityPenaltyReducesDiscoverabilityScore(t *testing.T) {
	// Project with error-level issues (dependency violations) should have
	// cat.Score lower than the raw sum of sub-metric scores.
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
			Imports: []string{"internal/user/adapters/db", "internal/user/adapters/http"},
			Functions: []domain.Function{
				{Name: "CreateUser", Exported: true},
				{Name: "ValidateUser", Exported: true},
			},
		},
	}

	result := scoring.ScoreDiscoverability(defaultProfile(), modules, &domain.ScanResult{}, analyzed)

	subMetricSum := 0
	for _, sm := range result.SubMetrics {
		subMetricSum += sm.Score
	}
	assert.Less(t, result.Score, subMetricSum,
		"severity penalty should reduce score below raw sub-metric sum")
}

func TestRateBasedDependencyScoring(t *testing.T) {
	// 1 violation in 100 files should score much higher than 1 violation in 2 files.
	makeMod := func(fileCount int) ([]domain.DetectedModule, map[string]*domain.AnalyzedFile) {
		var files []string
		analyzed := make(map[string]*domain.AnalyzedFile)
		for i := 0; i < fileCount; i++ {
			path := fmt.Sprintf("internal/mod/domain/file%d.go", i)
			files = append(files, path)
			af := &domain.AnalyzedFile{Path: path, Package: "domain"}
			if i == 0 {
				af.Imports = []string{"internal/mod/adapters/db"} // single violation
			}
			analyzed[path] = af
		}
		modules := []domain.DetectedModule{{Name: "mod", Path: "internal/mod", Layers: []string{"domain"}, Files: files}}
		return modules, analyzed
	}

	mods100, an100 := makeMod(100)
	mods2, an2 := makeMod(2)

	r100 := scoring.ScoreDiscoverability(defaultProfile(), mods100, &domain.ScanResult{}, an100)
	r2 := scoring.ScoreDiscoverability(defaultProfile(), mods2, &domain.ScanResult{}, an2)

	dep100 := r100.SubMetrics[3]
	dep2 := r2.SubMetrics[3]
	assert.Greater(t, dep100.Score, dep2.Score,
		"1 violation in 100 files should score higher than 1 violation in 2 files")
}

func TestIdentifierSpecificityWithDomainVocab(t *testing.T) {
	// CreateUser with User struct should score higher than HandleData with no structs.
	analyzed := map[string]*domain.AnalyzedFile{
		"good.go": {
			Path:    "good.go",
			Package: "user",
			Structs: []string{"User"},
			Functions: []domain.Function{
				{Name: "CreateUser", Exported: true},
			},
		},
	}
	domainVocab := scoring.ExtractDomainVocabulary(analyzed)
	scoreGood := scoring.IdentifierSpecificity("CreateUser", domainVocab)
	scoreBad := scoring.IdentifierSpecificity("HandleData", domainVocab)
	assert.Greater(t, scoreGood, scoreBad,
		"CreateUser with User struct should score higher than HandleData")
}

func TestSymbolCollisionRate(t *testing.T) {
	// 3 packages all exporting "New" → collision rate > 0, issues generated.
	analyzed := map[string]*domain.AnalyzedFile{
		"a.go": {Path: "a.go", Package: "user", Functions: []domain.Function{
			{Name: "New", Exported: true},
			{Name: "CreateUser", Exported: true},
		}},
		"b.go": {Path: "b.go", Package: "order", Functions: []domain.Function{
			{Name: "New", Exported: true},
			{Name: "CreateOrder", Exported: true},
		}},
		"c.go": {Path: "c.go", Package: "payment", Functions: []domain.Function{
			{Name: "New", Exported: true},
		}},
	}

	rate := scoring.SymbolCollisionRate(analyzed)
	assert.Greater(t, rate, 0.0, "New appearing in 3 packages should produce non-zero collision rate")

	result := scoring.ScoreDiscoverability(defaultProfile(), nil, nil, analyzed)
	var collisionIssues []domain.Issue
	for _, iss := range result.Issues {
		if iss.SubMetric == "naming_uniqueness" && strings.Contains(iss.Message, "appears in") {
			collisionIssues = append(collisionIssues, iss)
		}
	}
	assert.GreaterOrEqual(t, len(collisionIssues), 1, "collision issues should be generated for 'New'")
}

func TestVaguePackageNameIssues(t *testing.T) {
	analyzed := map[string]*domain.AnalyzedFile{
		"utils/helper.go": {
			Path:    "utils/helper.go",
			Package: "utils",
			Functions: []domain.Function{
				{Name: "FormatTime", Exported: true},
			},
		},
	}

	result := scoring.ScoreDiscoverability(defaultProfile(), nil, nil, analyzed)

	var pkgIssues []domain.Issue
	for _, iss := range result.Issues {
		if strings.Contains(iss.Message, "vague name") {
			pkgIssues = append(pkgIssues, iss)
		}
	}
	require.Len(t, pkgIssues, 1)
	assert.Contains(t, pkgIssues[0].Message, "utils")
	assert.Equal(t, domain.SeverityInfo, pkgIssues[0].Severity)
}

func TestParamNameQualityIssues(t *testing.T) {
	analyzed := map[string]*domain.AnalyzedFile{
		"math.go": {
			Path:    "math.go",
			Package: "calc",
			Functions: []domain.Function{
				{Name: "Add", Exported: true, LineStart: 1, Params: []domain.Param{
					{Name: "a", Type: "int"}, {Name: "b", Type: "int"}, {Name: "c", Type: "int"},
				}},
				{Name: "Subtract", Exported: true, LineStart: 10, Params: []domain.Param{
					{Name: "ctx", Type: "context.Context"}, {Name: "name", Type: "string"},
				}},
			},
		},
	}

	result := scoring.ScoreDiscoverability(defaultProfile(), nil, nil, analyzed)

	var paramIssues []domain.Issue
	for _, iss := range result.Issues {
		if strings.Contains(iss.Message, "single-letter parameters") {
			paramIssues = append(paramIssues, iss)
		}
	}
	require.Len(t, paramIssues, 1, "only Add(a,b,c) should be flagged, not Subtract(ctx,name)")
	assert.Contains(t, paramIssues[0].Message, "Add")
}

func TestNilProfileGuard(t *testing.T) {
	// ScoreDiscoverability(nil, ...) should not panic.
	assert.NotPanics(t, func() {
		scoring.ScoreDiscoverability(nil, nil, nil, nil)
	})
}

func TestProfileFieldsUsed(t *testing.T) {
	// Custom MinNamingWordScore = 0.3 means single-word names (WCS=0.5) pass the threshold.
	p := domain.DefaultProfile()
	p.MinNamingWordScore = 0.3

	analyzed := map[string]*domain.AnalyzedFile{
		"a.go": {
			Path: "a.go",
			Functions: []domain.Function{
				{Name: "Handle", Exported: true, LineStart: 1},
			},
		},
	}

	result := scoring.ScoreDiscoverability(&p, nil, nil, analyzed)

	for _, iss := range result.Issues {
		if iss.SubMetric == "naming_uniqueness" && strings.Contains(iss.Message, "single-word") {
			t.Errorf("Handle (WCS=0.5) should not be flagged when MinNamingWordScore=0.3: %s", iss.Message)
		}
	}
}

func TestSeverityGrading(t *testing.T) {
	// Predictable structure: module missing >75% of peer layers → warning.
	// 5 layers total, module has only 1 → missing 4/5 = 80% > 75%.
	modules := []domain.DetectedModule{
		{Name: "a", Path: "internal/a", Layers: []string{"domain", "application", "adapters", "ports", "events"}},
		{Name: "b", Path: "internal/b", Layers: []string{"domain", "application", "adapters", "ports", "events"}},
		{Name: "c", Path: "internal/c", Layers: []string{"domain"}},
	}

	result := scoring.ScoreDiscoverability(defaultProfile(), modules, &domain.ScanResult{}, nil)

	var warnings []domain.Issue
	for _, iss := range result.Issues {
		if iss.SubMetric == "predictable_structure" && iss.Severity == domain.SeverityWarning {
			warnings = append(warnings, iss)
		}
	}
	assert.GreaterOrEqual(t, len(warnings), 1,
		"module missing >75%% of peer layers should produce warning-level issues")
}

func TestCollisionIssuesExemptGeneratedFiles(t *testing.T) {
	analyzed := map[string]*domain.AnalyzedFile{
		"a.go": {Path: "a.go", Package: "user", Functions: []domain.Function{
			{Name: "New", Exported: true},
		}},
		"generated.go": {Path: "generated.go", Package: "order", IsGenerated: true, Functions: []domain.Function{
			{Name: "New", Exported: true},
		}},
	}

	rate := scoring.SymbolCollisionRate(analyzed)
	assert.Equal(t, 0.0, rate, "generated files should not contribute to collision count")

	result := scoring.ScoreDiscoverability(defaultProfile(), nil, nil, analyzed)
	for _, iss := range result.Issues {
		if strings.Contains(iss.Message, "appears in") {
			t.Errorf("should not flag collisions when one file is generated: %s", iss.Message)
		}
	}
}
