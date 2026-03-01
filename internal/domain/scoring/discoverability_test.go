package scoring_test

import (
	"fmt"
	"strings"
	"testing"

	"github.com/abdidvp/openkraft/internal/domain"
	"github.com/abdidvp/openkraft/internal/domain/scoring"
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
	// Empty inputs: no functions, no files, no modules.
	// predictable_structure and dependency_direction give full credit (nothing to penalize).
	// naming_uniqueness and file_naming_conventions give 0 (no data).
	assert.Equal(t, 50, result.Score, "empty project: 0+0+25+25 = 50")
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
	// With forced "bare", scanner.go and order_repo.go match bare (since _repo isn't a known suffix).
	// user_handler.go and tax_service.go are recognized suffixed files. 2/4 = 50% → ~13 pts.
	assert.Less(t, naming.Score, 15, "forced bare should penalize suffixed files")
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

// --- Audit bug regression tests (2026-02-28) ---

func TestScoreDiscoverability_NonHexagonalProjectGetsFullDependencyCredit(t *testing.T) {
	// Bug 1: Flat projects with no layered files were getting 0/25 on dependency_direction.
	// They should get full credit — no layers means no violations.
	t.Run("modules_with_no_layers", func(t *testing.T) {
		modules := []domain.DetectedModule{
			{
				Name:   "server",
				Path:   "pkg/server",
				Layers: []string{},
				Files:  []string{"pkg/server/server.go", "pkg/server/routes.go"},
			},
		}
		analyzed := map[string]*domain.AnalyzedFile{
			"pkg/server/server.go": {
				Path:    "pkg/server/server.go",
				Package: "server",
				Imports: []string{"net/http", "pkg/server/middleware"},
				Functions: []domain.Function{
					{Name: "NewServer", Exported: true},
				},
			},
			"pkg/server/routes.go": {
				Path:    "pkg/server/routes.go",
				Package: "server",
				Imports: []string{"net/http"},
				Functions: []domain.Function{
					{Name: "SetupRoutes", Exported: true},
				},
			},
		}

		result := scoring.ScoreDiscoverability(defaultProfile(), modules, &domain.ScanResult{}, analyzed)

		depDirection := result.SubMetrics[3]
		assert.Equal(t, "dependency_direction", depDirection.Name)
		assert.Equal(t, 25, depDirection.Score,
			"flat project with no layers should get full 25/25 dependency direction credit")
	})

	t.Run("zero_modules", func(t *testing.T) {
		// Projects like chi where no modules are detected at all.
		analyzed := map[string]*domain.AnalyzedFile{
			"router.go": {
				Path:    "router.go",
				Package: "chi",
				Functions: []domain.Function{
					{Name: "NewRouter", Exported: true},
				},
			},
		}

		result := scoring.ScoreDiscoverability(defaultProfile(), nil, &domain.ScanResult{}, analyzed)

		depDirection := result.SubMetrics[3]
		assert.Equal(t, "dependency_direction", depDirection.Name)
		assert.Equal(t, 25, depDirection.Score,
			"project with zero modules should get full 25/25 dependency direction credit")

		predictable := result.SubMetrics[2]
		assert.Equal(t, "predictable_structure", predictable.Name)
		assert.Equal(t, 25, predictable.Score,
			"project with zero modules should get full 25/25 predictable structure credit")
	})
}

func TestScoreDiscoverability_LayerAliasesRecognized(t *testing.T) {
	// Bug 7: helpers.go used hardcoded layer names, ignoring profile.LayerAliases.
	// Projects using /app/ or /infra/ instead of /application/ or /adapters/ should
	// have violations detected correctly.
	modules := []domain.DetectedModule{
		{
			Name:   "user",
			Path:   "internal/user",
			Layers: []string{"domain", "app", "infra"},
			Files: []string{
				"internal/user/domain/model.go",
				"internal/user/app/service.go",
				"internal/user/infra/repo.go",
			},
		},
	}
	analyzed := map[string]*domain.AnalyzedFile{
		"internal/user/domain/model.go": {
			Path:    "internal/user/domain/model.go",
			Package: "domain",
			Imports: []string{"internal/user/infra/repo"}, // domain → adapters(infra) = violation
		},
		"internal/user/app/service.go": {
			Path:    "internal/user/app/service.go",
			Package: "app",
			Imports: []string{"internal/user/domain/model"},
		},
		"internal/user/infra/repo.go": {
			Path:    "internal/user/infra/repo.go",
			Package: "infra",
			Imports: []string{"database/sql"},
		},
	}

	result := scoring.ScoreDiscoverability(defaultProfile(), modules, &domain.ScanResult{}, analyzed)

	depDirection := result.SubMetrics[3]
	assert.Equal(t, "dependency_direction", depDirection.Name)
	assert.Less(t, depDirection.Score, depDirection.Points,
		"domain importing infra should be detected as violation via alias resolution")

	var depIssues []domain.Issue
	for _, iss := range result.Issues {
		if iss.SubMetric == "dependency_direction" {
			depIssues = append(depIssues, iss)
		}
	}
	assert.GreaterOrEqual(t, len(depIssues), 1, "should flag domain→infra violation")
}

func TestScoreDiscoverability_MethodsWithReceiverNotCountedAsCollisions(t *testing.T) {
	// Bug 3: Methods like (*User).String() and (*Order).String() were counted as
	// collisions despite being fully qualified by receiver type.
	analyzed := map[string]*domain.AnalyzedFile{
		"user.go": {
			Path:    "user.go",
			Package: "user",
			Functions: []domain.Function{
				{Name: "String", Exported: true, Receiver: "*User"},
				{Name: "Error", Exported: true, Receiver: "*UserError"},
			},
		},
		"order.go": {
			Path:    "order.go",
			Package: "order",
			Functions: []domain.Function{
				{Name: "String", Exported: true, Receiver: "*Order"},
				{Name: "Error", Exported: true, Receiver: "*OrderError"},
			},
		},
	}

	rate := scoring.SymbolCollisionRate(analyzed)
	assert.Equal(t, 0.0, rate, "methods with receivers should not count as collisions")
}

func TestScoreDiscoverability_TopLevelFunctionsStillCollide(t *testing.T) {
	// Bug 3 counterpart: top-level functions without receivers should still collide.
	analyzed := map[string]*domain.AnalyzedFile{
		"user.go": {
			Path:    "user.go",
			Package: "user",
			Functions: []domain.Function{
				{Name: "New", Exported: true},
			},
		},
		"order.go": {
			Path:    "order.go",
			Package: "order",
			Functions: []domain.Function{
				{Name: "New", Exported: true},
			},
		},
	}

	rate := scoring.SymbolCollisionRate(analyzed)
	assert.Greater(t, rate, 0.0, "top-level New() in 2 packages should produce collision")
}

func TestScoreDiscoverability_CompoundNamesNotTreatedAsSuffixed(t *testing.T) {
	// Bug 2: content_type.go was treated as suffixed because it contains "_".
	// It should be treated as bare since "_type" is not in ExpectedFileSuffixes.
	scan := &domain.ScanResult{
		GoFiles: []string{
			"content_type.go", "error_code.go", "route_group.go",
			"scanner.go", "parser.go", "config.go",
		},
	}

	result := scoring.ScoreDiscoverability(defaultProfile(), nil, scan, nil)
	naming := result.SubMetrics[1]
	assert.Equal(t, "file_naming_conventions", naming.Name)
	// All files should be classified as bare → 100% consistency.
	assert.GreaterOrEqual(t, naming.Score, 22,
		"compound names like content_type.go should be treated as bare")
}

func TestScoreDiscoverability_BuildTagsNotTreatedAsSuffixed(t *testing.T) {
	// Bug 2: server_darwin.go was treated as suffixed because it contains "_".
	// Platform build tags should be stripped before checking.
	scan := &domain.ScanResult{
		GoFiles: []string{
			"server.go", "server_darwin.go", "server_linux.go",
			"client.go", "client_windows.go",
		},
	}

	result := scoring.ScoreDiscoverability(defaultProfile(), nil, scan, nil)
	naming := result.SubMetrics[1]
	assert.Equal(t, "file_naming_conventions", naming.Name)
	assert.GreaterOrEqual(t, naming.Score, 22,
		"platform build tag files should be treated as bare")
}

func TestWordCountScore_VerboseNotWorseThanSingleWord(t *testing.T) {
	// Bug 5: 6+ word names scored 0.3, worse than single-word names (0.5).
	// Both should score equally since verbose names are not worse than cryptic ones.
	singleWord := scoring.WordCountScore("Handle")
	verbose := scoring.WordCountScore("CreateUserFromExternalSourceWithValidation") // 6+ words

	assert.Equal(t, singleWord, verbose,
		"6+ word names should not score worse than single-word names")
	assert.Equal(t, 0.5, verbose)
}

func TestScoreDiscoverability_VerboseNameIssueMessage(t *testing.T) {
	// Bug 5: The issue message said "single-word" even for 6+ word names.
	analyzed := map[string]*domain.AnalyzedFile{
		"handler.go": {
			Path:    "handler.go",
			Package: "handler",
			Functions: []domain.Function{
				{Name: "A", Exported: true, LineStart: 1}, // 1 word → "single-word" message
			},
		},
	}

	p := domain.DefaultProfile()
	p.MinNamingWordScore = 0.9 // Set high enough that single-word names are flagged
	result := scoring.ScoreDiscoverability(&p, nil, nil, analyzed)

	for _, iss := range result.Issues {
		if iss.SubMetric == "naming_uniqueness" && strings.Contains(iss.Message, `"A"`) {
			assert.Contains(t, iss.Message, "single-word",
				"1-word name should say 'single-word'")
		}
	}
}

func TestScoreDiscoverability_TestFilesExcludedFromCollisions(t *testing.T) {
	// Bug 4: Test files contributed to collision counts, inflating false positives.
	analyzed := map[string]*domain.AnalyzedFile{
		"user.go": {
			Path:    "user.go",
			Package: "user",
			Functions: []domain.Function{
				{Name: "New", Exported: true},
			},
		},
		"user_test.go": {
			Path:    "user_test.go",
			Package: "user_test",
			Functions: []domain.Function{
				{Name: "New", Exported: true}, // test helper
			},
		},
		"order.go": {
			Path:    "order.go",
			Package: "order",
			Functions: []domain.Function{
				{Name: "Create", Exported: true},
			},
		},
		"order_test.go": {
			Path:    "order_test.go",
			Package: "order_test",
			Functions: []domain.Function{
				{Name: "New", Exported: true}, // test helper
			},
		},
	}

	rate := scoring.SymbolCollisionRate(analyzed)
	assert.Equal(t, 0.0, rate,
		"test file functions should not contribute to collision rate")

	result := scoring.ScoreDiscoverability(defaultProfile(), nil, nil, analyzed)
	for _, iss := range result.Issues {
		if strings.Contains(iss.Message, "appears in") && strings.Contains(iss.Message, "New") {
			t.Errorf("test files should not produce collision issues: %s", iss.Message)
		}
	}
}

func TestScoreDiscoverability_MethodParamsNotFlagged(t *testing.T) {
	// Bug 6: Methods implementing interfaces (e.g., ServeHTTP(w, r)) were flagged
	// for single-letter params. Methods get context from receiver type.
	analyzed := map[string]*domain.AnalyzedFile{
		"handler.go": {
			Path:    "handler.go",
			Package: "handler",
			Functions: []domain.Function{
				{Name: "ServeHTTP", Exported: true, Receiver: "*Handler", LineStart: 10,
					Params: []domain.Param{{Name: "w", Type: "http.ResponseWriter"}, {Name: "r", Type: "*http.Request"}}},
				{Name: "Less", Exported: true, Receiver: "*Sorter", LineStart: 20,
					Params: []domain.Param{{Name: "i", Type: "int"}, {Name: "j", Type: "int"}}},
				{Name: "Swap", Exported: true, Receiver: "*Sorter", LineStart: 30,
					Params: []domain.Param{{Name: "i", Type: "int"}, {Name: "j", Type: "int"}}},
			},
		},
	}

	result := scoring.ScoreDiscoverability(defaultProfile(), nil, nil, analyzed)

	for _, iss := range result.Issues {
		if strings.Contains(iss.Message, "single-letter parameters") {
			t.Errorf("method with receiver should not be flagged for param names: %s", iss.Message)
		}
	}
}

func TestScoreDiscoverability_TopLevelFuncParamsStillFlagged(t *testing.T) {
	// Bug 6 counterpart: top-level functions with all single-letter params should still be flagged.
	analyzed := map[string]*domain.AnalyzedFile{
		"math.go": {
			Path:    "math.go",
			Package: "math",
			Functions: []domain.Function{
				{Name: "Add", Exported: true, LineStart: 1,
					Params: []domain.Param{{Name: "s", Type: "int"}, {Name: "t", Type: "int"}}},
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
	require.Len(t, paramIssues, 1, "top-level Add(s, t) should still be flagged")
	assert.Contains(t, paramIssues[0].Message, "Add")
}

// --- Import graph integration tests ---

func TestScoreDiscoverability_ImportGraphComposite(t *testing.T) {
	mod := "github.com/example/app"
	scan := &domain.ScanResult{
		ModulePath: mod,
		GoFiles: []string{
			"domain/model.go",
			"application/service.go",
			"adapters/handler.go",
		},
	}
	modules := []domain.DetectedModule{
		{
			Name:   "app",
			Path:   ".",
			Layers: []string{"domain", "application", "adapters"},
			Files:  scan.GoFiles,
		},
	}
	analyzed := map[string]*domain.AnalyzedFile{
		"domain/model.go": {
			Path: "domain/model.go", Package: "domain",
			Interfaces: []string{"Repository"},
			Structs:    []string{"User"},
			Functions:  []domain.Function{{Name: "CreateUser", Exported: true}},
		},
		"application/service.go": {
			Path: "application/service.go", Package: "application",
			Imports: []string{mod + "/domain"},
			Structs: []string{"UserService"},
			Functions: []domain.Function{{Name: "NewUserService", Exported: true}},
		},
		"adapters/handler.go": {
			Path: "adapters/handler.go", Package: "adapters",
			Imports: []string{mod + "/application", mod + "/domain"},
			Structs: []string{"Handler"},
			Functions: []domain.Function{{Name: "NewHandler", Exported: true}},
		},
	}

	result := scoring.ScoreDiscoverability(defaultProfile(), modules, scan, analyzed)
	depDirection := result.SubMetrics[3]
	assert.Equal(t, "dependency_direction", depDirection.Name)
	// Clean architecture with no cycles → should score well.
	assert.GreaterOrEqual(t, depDirection.Score, 20)
	assert.Contains(t, depDirection.Detail, "graph:")
}

func TestScoreDiscoverability_NoModulePathGetsFullGraphCredit(t *testing.T) {
	modules := []domain.DetectedModule{
		{
			Name:   "mod",
			Path:   "internal/mod",
			Layers: []string{"domain"},
			Files:  []string{"internal/mod/domain/model.go"},
		},
	}
	analyzed := map[string]*domain.AnalyzedFile{
		"internal/mod/domain/model.go": {
			Path: "internal/mod/domain/model.go", Package: "domain",
			Functions: []domain.Function{{Name: "CreateUser", Exported: true}},
		},
	}
	scan := &domain.ScanResult{} // No ModulePath

	result := scoring.ScoreDiscoverability(defaultProfile(), modules, scan, analyzed)
	depDirection := result.SubMetrics[3]
	assert.Equal(t, "dependency_direction", depDirection.Name)
	// No module path → graph gets full credit, only layer violations matter.
	// No violations → full 25 points.
	assert.Equal(t, 25, depDirection.Score)
}

func TestScoreDiscoverability_CycleDetectedInIssues(t *testing.T) {
	mod := "github.com/example/cyclic"
	scan := &domain.ScanResult{
		ModulePath: mod,
		GoFiles:    []string{"a/a.go", "b/b.go"},
	}
	analyzed := map[string]*domain.AnalyzedFile{
		"a/a.go": {
			Path: "a/a.go", Package: "a",
			Imports: []string{mod + "/b"},
			Structs: []string{"A"},
			Functions: []domain.Function{{Name: "NewA", Exported: true}},
		},
		"b/b.go": {
			Path: "b/b.go", Package: "b",
			Imports: []string{mod + "/a"},
			Structs: []string{"B"},
			Functions: []domain.Function{{Name: "NewB", Exported: true}},
		},
	}

	result := scoring.ScoreDiscoverability(defaultProfile(), nil, scan, analyzed)

	var cycleIssues []domain.Issue
	for _, iss := range result.Issues {
		if iss.Pattern == "import-cycle" {
			cycleIssues = append(cycleIssues, iss)
		}
	}
	require.GreaterOrEqual(t, len(cycleIssues), 1, "should detect import cycle")
	assert.Equal(t, domain.SeverityError, cycleIssues[0].Severity)
	assert.Contains(t, cycleIssues[0].Message, "import cycle")
}

func TestScoreDiscoverability_CouplingOutlierInIssues(t *testing.T) {
	// All leaf packages import at least 1 internal pkg so median Ce ≥ 1,
	// making the outlier detection meaningful (Approach A: no signal = no penalty).
	mod := "github.com/example/coupled"
	scan := &domain.ScanResult{
		ModulePath: mod,
		GoFiles:    []string{"god/god.go", "a/a.go", "b/b.go", "c/c.go", "d/d.go", "e/e.go"},
	}
	analyzed := map[string]*domain.AnalyzedFile{
		"god/god.go": {
			Path: "god/god.go", Package: "god",
			Imports: []string{mod + "/a", mod + "/b", mod + "/c", mod + "/d", mod + "/e"},
			Structs:   []string{"God"},
			Functions: []domain.Function{{Name: "NewGod", Exported: true}},
		},
		"a/a.go": {Path: "a/a.go", Package: "a", Imports: []string{mod + "/b"},
			Structs: []string{"A"}, Functions: []domain.Function{{Name: "NewA", Exported: true}}},
		"b/b.go": {Path: "b/b.go", Package: "b", Imports: []string{mod + "/c"},
			Structs: []string{"B"}, Functions: []domain.Function{{Name: "NewB", Exported: true}}},
		"c/c.go": {Path: "c/c.go", Package: "c", Imports: []string{mod + "/d"},
			Structs: []string{"C"}, Functions: []domain.Function{{Name: "NewC", Exported: true}}},
		"d/d.go": {Path: "d/d.go", Package: "d", Imports: []string{mod + "/e"},
			Structs: []string{"D"}, Functions: []domain.Function{{Name: "NewD", Exported: true}}},
		"e/e.go": {Path: "e/e.go", Package: "e",
			Structs: []string{"E"}, Functions: []domain.Function{{Name: "NewE", Exported: true}}},
	}

	result := scoring.ScoreDiscoverability(defaultProfile(), nil, scan, analyzed)

	var couplingIssues []domain.Issue
	for _, iss := range result.Issues {
		if iss.Pattern == "coupling-outlier" {
			couplingIssues = append(couplingIssues, iss)
		}
	}
	require.GreaterOrEqual(t, len(couplingIssues), 1, "should detect coupling outlier")
	assert.Equal(t, domain.SeverityWarning, couplingIssues[0].Severity)
	assert.Contains(t, couplingIssues[0].Message, "god")
}

func TestScoreDiscoverability_SinglePackageProjectFullCredit(t *testing.T) {
	mod := "github.com/example/simple"
	scan := &domain.ScanResult{
		ModulePath: mod,
		GoFiles:    []string{"main.go", "config.go"},
	}
	analyzed := map[string]*domain.AnalyzedFile{
		"main.go": {
			Path: "main.go", Package: "main",
			Structs:   []string{"App"},
			Functions: []domain.Function{{Name: "NewApp", Exported: true}},
		},
		"config.go": {
			Path: "config.go", Package: "main",
			Structs:   []string{"Config"},
			Functions: []domain.Function{{Name: "LoadConfig", Exported: true}},
		},
	}

	result := scoring.ScoreDiscoverability(defaultProfile(), nil, scan, analyzed)
	depDirection := result.SubMetrics[3]
	assert.Equal(t, "dependency_direction", depDirection.Name)
	assert.Equal(t, 25, depDirection.Score, "single-package project should get full credit")
}
