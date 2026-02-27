package scoring_test

import (
	"strings"
	"testing"

	"github.com/openkraft/openkraft/internal/domain"
	"github.com/openkraft/openkraft/internal/domain/scoring"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ---------------------------------------------------------------------------
// Test helpers — build analyzed maps from descriptive builder functions
// ---------------------------------------------------------------------------

func makeFunction(name string, lines, params, nesting, condOps int) domain.Function {
	return domain.Function{
		Name:       name,
		Exported:   name[0] >= 'A' && name[0] <= 'Z',
		LineStart:  1,
		LineEnd:    lines,
		Params:     make([]domain.Param, params),
		MaxNesting: nesting,
		MaxCondOps: condOps,
	}
}

func makeFile(path string, totalLines int, fns ...domain.Function) *domain.AnalyzedFile {
	return &domain.AnalyzedFile{
		Path:       path,
		TotalLines: totalLines,
		Functions:  fns,
	}
}

func analyzed(files ...*domain.AnalyzedFile) map[string]*domain.AnalyzedFile {
	m := make(map[string]*domain.AnalyzedFile, len(files))
	for _, f := range files {
		m[f.Path] = f
	}
	return m
}

func subMetricByName(result domain.CategoryScore, name string) *domain.SubMetric {
	for i := range result.SubMetrics {
		if result.SubMetrics[i].Name == name {
			return &result.SubMetrics[i]
		}
	}
	return nil
}

func issuesBySubMetric(issues []domain.Issue, subMetric string) []domain.Issue {
	var filtered []domain.Issue
	for _, iss := range issues {
		if iss.SubMetric == subMetric {
			filtered = append(filtered, iss)
		}
	}
	return filtered
}

// ---------------------------------------------------------------------------
// Category structure invariants
// ---------------------------------------------------------------------------

func TestScoreCodeHealth_CategoryStructure(t *testing.T) {
	tests := []struct {
		name     string
		analyzed map[string]*domain.AnalyzedFile
	}{
		{
			name:     "nil inputs",
			analyzed: nil,
		},
		{
			name:     "empty map",
			analyzed: map[string]*domain.AnalyzedFile{},
		},
		{
			name: "single well-structured file",
			analyzed: analyzed(
				makeFile("service.go", 100,
					makeFunction("CreateUser", 20, 2, 1, 0),
				),
			),
		},
	}

	expectedSubMetrics := []string{
		"function_size", "file_size", "nesting_depth",
		"parameter_count", "complex_conditionals",
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := scoring.ScoreCodeHealth(defaultProfile(), nil, tt.analyzed)

			assert.Equal(t, "code_health", result.Name)
			assert.Equal(t, 0.25, result.Weight)
			require.Len(t, result.SubMetrics, 5)

			totalPoints := 0
			for i, sm := range result.SubMetrics {
				assert.Equal(t, expectedSubMetrics[i], sm.Name)
				assert.Equal(t, 20, sm.Points, "each sub-metric allocates 20 points")
				totalPoints += sm.Points
			}
			assert.Equal(t, 100, totalPoints, "sub-metric points must sum to 100")

			assert.GreaterOrEqual(t, result.Score, 0)
			assert.LessOrEqual(t, result.Score, 100)
		})
	}
}

// ---------------------------------------------------------------------------
// P0 Bug Fix: Zero-function edge case — full credit when nothing to evaluate
// ---------------------------------------------------------------------------

func TestScoreCodeHealth_ZeroFunctionsGetFullCredit(t *testing.T) {
	tests := []struct {
		name          string
		analyzed      map[string]*domain.AnalyzedFile
		expectScore   int
		expectDetails map[string]string
	}{
		{
			name:        "empty map gives 100 — nothing to penalize",
			analyzed:    map[string]*domain.AnalyzedFile{},
			expectScore: 100,
			expectDetails: map[string]string{
				"function_size":        "no functions to evaluate",
				"file_size":            "no files to evaluate",
				"nesting_depth":        "no functions to evaluate",
				"parameter_count":      "no functions to evaluate",
				"complex_conditionals": "no functions to evaluate",
			},
		},
		{
			name:        "nil map gives 100 — nil is equivalent to empty",
			analyzed:    nil,
			expectScore: 100,
			expectDetails: map[string]string{
				"function_size":        "no functions to evaluate",
				"file_size":            "no files to evaluate",
				"nesting_depth":        "no functions to evaluate",
				"parameter_count":      "no functions to evaluate",
				"complex_conditionals": "no functions to evaluate",
			},
		},
		{
			name: "file with zero functions — function sub-metrics get full credit",
			analyzed: analyzed(
				makeFile("types.go", 50), // no functions, just type defs
			),
			expectScore: 100,
			expectDetails: map[string]string{
				"function_size":        "no functions to evaluate",
				"nesting_depth":        "no functions to evaluate",
				"parameter_count":      "no functions to evaluate",
				"complex_conditionals": "no functions to evaluate",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := scoring.ScoreCodeHealth(defaultProfile(), nil, tt.analyzed)

			assert.Equal(t, tt.expectScore, result.Score)

			for smName, expectedDetail := range tt.expectDetails {
				sm := subMetricByName(result, smName)
				require.NotNilf(t, sm, "sub-metric %s not found", smName)
				assert.Equal(t, sm.Points, sm.Score, "sub-metric %s should get full credit", smName)
				assert.Equal(t, expectedDetail, sm.Detail, "sub-metric %s detail", smName)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// P0 Bug Fix: math.Round instead of int() truncation
// ---------------------------------------------------------------------------

func TestScoreCodeHealth_RoundingBehavior(t *testing.T) {
	// Default profile: MaxFunctionLines=50, so full credit ≤50, partial 51-100, zero >100.
	// With 40 functions: 39 within limit (39.0) + 1 partial (0.5) = 39.5/40 = 0.9875.
	// int(0.9875 * 20) = int(19.75) = 19 (old truncation)
	// math.Round(0.9875 * 20) = math.Round(19.75) = 20 (new rounding)
	fns := make([]domain.Function, 0, 40)
	for i := range 39 {
		fns = append(fns, makeFunction("Good"+string(rune('A'+i%26)), 30, 2, 1, 0))
	}
	fns = append(fns, makeFunction("SlightlyLong", 70, 2, 1, 0)) // partial credit

	result := scoring.ScoreCodeHealth(defaultProfile(), nil, analyzed(
		makeFile("service.go", 200, fns...),
	))

	sm := subMetricByName(result, "function_size")
	require.NotNil(t, sm)
	// 39.5/40 = 98.75% → math.Round(19.75) = 20
	assert.Equal(t, 20, sm.Score, "98.75%% ratio should round UP to full credit, not truncate to 19")
}

func TestScoreCodeHealth_RoundingDoesNotOveraward(t *testing.T) {
	// 18 good + 2 partial = 19.0/20 = 0.95 → math.Round(19.0) = 19 (NOT 20)
	fns := make([]domain.Function, 0, 20)
	for i := range 18 {
		fns = append(fns, makeFunction("Good"+string(rune('A'+i%26)), 30, 2, 1, 0))
	}
	fns = append(fns, makeFunction("PartialA", 70, 2, 1, 0))
	fns = append(fns, makeFunction("PartialB", 70, 2, 1, 0))

	result := scoring.ScoreCodeHealth(defaultProfile(), nil, analyzed(
		makeFile("service.go", 200, fns...),
	))

	sm := subMetricByName(result, "function_size")
	require.NotNil(t, sm)
	assert.Equal(t, 19, sm.Score, "95%% ratio should yield 19, not round up to 20")
}

func TestScoreCodeHealth_RoundingLowerBoundary(t *testing.T) {
	// 9 full + 1 zero out of 10 = 9.0/10 = 0.90
	// math.Round(0.90 * 20) = math.Round(18.0) = 18
	fns := make([]domain.Function, 0, 10)
	for i := range 9 {
		fns = append(fns, makeFunction("Good"+string(rune('A'+i)), 30, 2, 1, 0))
	}
	fns = append(fns, makeFunction("Bad", 200, 2, 1, 0)) // zero credit

	result := scoring.ScoreCodeHealth(defaultProfile(), nil, analyzed(
		makeFile("service.go", 200, fns...),
	))

	sm := subMetricByName(result, "function_size")
	require.NotNil(t, sm)
	assert.Equal(t, 18, sm.Score, "90%% ratio should yield 18, not 19")
}

// ---------------------------------------------------------------------------
// P0 Bug Fix: SubMetric field populated on every issue
// ---------------------------------------------------------------------------

func TestScoreCodeHealth_AllIssuesHaveSubMetric(t *testing.T) {
	validSubMetrics := map[string]bool{
		"function_size":        true,
		"file_size":            true,
		"nesting_depth":        true,
		"parameter_count":      true,
		"complex_conditionals": true,
	}

	// Build a file that triggers ALL issue types.
	result := scoring.ScoreCodeHealth(defaultProfile(), nil, analyzed(
		makeFile("monster.go", 600,
			makeFunction("Huge", 200, 8, 6, 5),
		),
	))

	require.NotEmpty(t, result.Issues, "should generate issues for extreme violations")

	for _, issue := range result.Issues {
		assert.NotEmpty(t, issue.SubMetric,
			"issue must have SubMetric set: %s", issue.Message)
		assert.True(t, validSubMetrics[issue.SubMetric],
			"unexpected SubMetric %q on issue: %s", issue.SubMetric, issue.Message)
		assert.Equal(t, "code_health", issue.Category)
	}
}

func TestScoreCodeHealth_SubMetricMatchesIssueType(t *testing.T) {
	// Default profile thresholds: MaxFunctionLines=50, MaxNestingDepth=3,
	// MaxParameters=4, MaxFileLines=300.
	// Issue thresholds: funcSize>100, nesting≥5, params≥7, fileSize>500.
	result := scoring.ScoreCodeHealth(defaultProfile(), nil, analyzed(
		makeFile("violations.go", 600,
			// 150 lines → triggers function_size issue (>100)
			makeFunction("BigFunc", 150, 2, 1, 0),
			// nesting 6 → triggers nesting_depth issue (≥5)
			makeFunction("DeepFunc", 20, 2, 6, 0),
			// 8 params → triggers parameter_count issue (≥7)
			makeFunction("ManyParams", 20, 8, 1, 0),
		),
	))

	funcSizeIssues := issuesBySubMetric(result.Issues, "function_size")
	nestingIssues := issuesBySubMetric(result.Issues, "nesting_depth")
	paramIssues := issuesBySubMetric(result.Issues, "parameter_count")
	fileSizeIssues := issuesBySubMetric(result.Issues, "file_size")

	assert.Len(t, funcSizeIssues, 1, "expected 1 function_size issue for BigFunc")
	assert.Len(t, nestingIssues, 1, "expected 1 nesting_depth issue for DeepFunc")
	assert.Len(t, paramIssues, 1, "expected 1 parameter_count issue for ManyParams")
	assert.Len(t, fileSizeIssues, 1, "expected 1 file_size issue for 600-line file")
}

func TestScoreCodeHealth_ComplexConditionalsIssueGeneration(t *testing.T) {
	// Default: MaxConditionalOps=2, issue threshold = 2+2 = 4.
	// condOps=5 should trigger an issue.
	result := scoring.ScoreCodeHealth(defaultProfile(), nil, analyzed(
		makeFile("complex.go", 100,
			makeFunction("TooComplex", 30, 2, 1, 5),
		),
	))

	condIssues := issuesBySubMetric(result.Issues, "complex_conditionals")
	require.Len(t, condIssues, 1, "expected 1 complex_conditionals issue")
	assert.Contains(t, condIssues[0].Message, "conditional operators")
	assert.Equal(t, "complex.go", condIssues[0].File)
}

// ---------------------------------------------------------------------------
// Severity tiering: error/warning/info based on how far actual exceeds threshold
// ---------------------------------------------------------------------------

func TestScoreCodeHealth_SeverityTiering(t *testing.T) {
	// Default: MaxParameters=4, issue threshold = 4+3 = 7.
	// issueSeverity(actual, threshold):
	//   actual/threshold >= 3.0 → error
	//   actual/threshold >= 1.5 → warning
	//   else                    → info
	tests := []struct {
		name     string
		params   int
		wantSev  string
		ratioDesc string
	}{
		// 69 params / 7 threshold = 9.86x → error
		{"extreme violation (9.9x)", 69, domain.SeverityError, "9.9x"},
		// 21 params / 7 threshold = 3.0x → error
		{"3x threshold boundary", 21, domain.SeverityError, "3.0x"},
		// 14 params / 7 threshold = 2.0x → warning
		{"2x threshold", 14, domain.SeverityWarning, "2.0x"},
		// 11 params / 7 threshold = 1.57x → warning
		{"1.5x threshold", 11, domain.SeverityWarning, "1.57x"},
		// 8 params / 7 threshold = 1.14x → info
		{"just over threshold", 8, domain.SeverityInfo, "1.14x"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := scoring.ScoreCodeHealth(defaultProfile(), nil, analyzed(
				makeFile("service.go", 100,
					makeFunction("Func", 20, tt.params, 1, 0),
				),
			))

			paramIssues := issuesBySubMetric(result.Issues, "parameter_count")
			require.Len(t, paramIssues, 1, "expected 1 parameter_count issue for %d params", tt.params)
			assert.Equal(t, tt.wantSev, paramIssues[0].Severity,
				"params=%d (%s threshold) should be %s", tt.params, tt.ratioDesc, tt.wantSev)
		})
	}
}

func TestScoreCodeHealth_SeverityTieringFileSize(t *testing.T) {
	// Default: MaxFileLines=300, issue threshold = 500 (300*5/3).
	// 1500 / 500 = 3.0x → error
	// 800 / 500 = 1.6x → warning
	// 510 / 500 = 1.02x → info
	tests := []struct {
		name       string
		totalLines int
		wantSev    string
	}{
		{"3x file size → error", 1500, domain.SeverityError},
		{"1.6x file size → warning", 800, domain.SeverityWarning},
		{"just over threshold → info", 510, domain.SeverityInfo},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := scoring.ScoreCodeHealth(defaultProfile(), nil, analyzed(
				makeFile("big.go", tt.totalLines,
					makeFunction("Fn", 20, 2, 1, 0),
				),
			))

			fileIssues := issuesBySubMetric(result.Issues, "file_size")
			require.Len(t, fileIssues, 1)
			assert.Equal(t, tt.wantSev, fileIssues[0].Severity,
				"file %d lines should be %s", tt.totalLines, tt.wantSev)
		})
	}
}

func TestScoreCodeHealth_SeverityMixInSameResult(t *testing.T) {
	// One file with multiple violations at different severity levels.
	result := scoring.ScoreCodeHealth(defaultProfile(), nil, analyzed(
		makeFile("mixed.go", 1500, // file_size: 1500/500 = 3x → error
			makeFunction("Extreme", 20, 69, 1, 0), // params: 69/7 = 9.9x → error
			makeFunction("Moderate", 20, 11, 1, 0), // params: 11/7 = 1.57x → warning
			makeFunction("Slight", 20, 8, 1, 0),    // params: 8/7 = 1.14x → info
		),
	))

	severities := make(map[string]int)
	for _, iss := range result.Issues {
		severities[iss.Severity]++
	}

	assert.Greater(t, severities[domain.SeverityError], 0, "should have error-level issues")
	assert.Greater(t, severities[domain.SeverityWarning], 0, "should have warning-level issues")
	assert.Greater(t, severities[domain.SeverityInfo], 0, "should have info-level issues")
}

// ---------------------------------------------------------------------------
// Reconstruct pattern exemption
// ---------------------------------------------------------------------------

func TestScoreCodeHealth_ReconstructGetFullCreditOnParameterCount(t *testing.T) {
	// ReconstructCustomer with 69 params should get full credit on parameter_count.
	// ProcessOrder with 10 params should get zero credit.
	result := scoring.ScoreCodeHealth(defaultProfile(), nil, analyzed(
		makeFile("customer.go", 100,
			makeFunction("ReconstructCustomer", 30, 69, 1, 0),
			makeFunction("ProcessOrder", 30, 10, 1, 0),
		),
	))

	sm := subMetricByName(result, "parameter_count")
	require.NotNil(t, sm)
	// 1 full (Reconstruct) + 0 zero (ProcessOrder) = 1/2 = 50% → Round(10) = 10
	assert.Equal(t, 10, sm.Score, "Reconstruct should get full credit, ProcessOrder zero")
}

func TestScoreCodeHealth_ReconstructNoParameterCountIssue(t *testing.T) {
	result := scoring.ScoreCodeHealth(defaultProfile(), nil, analyzed(
		makeFile("domain.go", 100,
			makeFunction("ReconstructCustomer", 30, 69, 1, 0),
			makeFunction("ReconstructCredit", 30, 50, 1, 0),
		),
	))

	paramIssues := issuesBySubMetric(result.Issues, "parameter_count")
	assert.Empty(t, paramIssues, "Reconstruct functions should never produce parameter_count issues")
}

func TestScoreCodeHealth_ReconstructStillCountedForOtherSubMetrics(t *testing.T) {
	// Reconstruct is only exempt from parameter_count.
	// If it has 200 lines, it should still get zero on function_size.
	result := scoring.ScoreCodeHealth(defaultProfile(), nil, analyzed(
		makeFile("domain.go", 100,
			makeFunction("ReconstructCustomer", 200, 69, 1, 0),
		),
	))

	funcSM := subMetricByName(result, "function_size")
	paramSM := subMetricByName(result, "parameter_count")
	require.NotNil(t, funcSM)
	require.NotNil(t, paramSM)

	assert.Equal(t, 0, funcSM.Score, "Reconstruct with 200 lines still penalized on function_size")
	assert.Equal(t, 20, paramSM.Score, "Reconstruct exempt on parameter_count")
}

func TestScoreCodeHealth_NonReconstructPrefixNotExempt(t *testing.T) {
	// "Reconstructor" or "ReconstructorHelper" don't match — only "Reconstruct*"
	// Actually "Reconstructor" DOES start with "Reconstruct". Let's use "Rebuild" instead.
	result := scoring.ScoreCodeHealth(defaultProfile(), nil, analyzed(
		makeFile("mapper.go", 100,
			makeFunction("RebuildCustomer", 30, 69, 1, 0),
		),
	))

	paramIssues := issuesBySubMetric(result.Issues, "parameter_count")
	assert.NotEmpty(t, paramIssues, "RebuildCustomer is NOT exempt from parameter_count")
}

// ---------------------------------------------------------------------------
// Configurable exempt_param_patterns
// ---------------------------------------------------------------------------

func TestScoreCodeHealth_CustomExemptPattern(t *testing.T) {
	p := domain.DefaultProfile()
	p.ExemptParamPatterns = []string{"Hydrate"}

	result := scoring.ScoreCodeHealth(&p, nil, analyzed(
		makeFile("mapper.go", 100,
			makeFunction("HydrateUser", 30, 69, 1, 0),
			makeFunction("ProcessOrder", 30, 10, 1, 0),
		),
	))

	sm := subMetricByName(result, "parameter_count")
	require.NotNil(t, sm)
	// HydrateUser exempt (full credit) + ProcessOrder zero = 1/2 = 50% → 10
	assert.Equal(t, 10, sm.Score, "Hydrate pattern should exempt HydrateUser but not ProcessOrder")

	paramIssues := issuesBySubMetric(result.Issues, "parameter_count")
	for _, iss := range paramIssues {
		assert.NotContains(t, iss.Message, "HydrateUser", "HydrateUser should not produce parameter_count issues")
	}
}

func TestScoreCodeHealth_EmptyExemptPatternsNoExemptions(t *testing.T) {
	p := domain.DefaultProfile()
	p.ExemptParamPatterns = []string{} // explicitly empty — no exemptions

	result := scoring.ScoreCodeHealth(&p, nil, analyzed(
		makeFile("domain.go", 100,
			makeFunction("ReconstructCustomer", 30, 69, 1, 0),
		),
	))

	sm := subMetricByName(result, "parameter_count")
	require.NotNil(t, sm)
	assert.Equal(t, 0, sm.Score, "with empty exempt patterns, Reconstruct should NOT be exempt")

	paramIssues := issuesBySubMetric(result.Issues, "parameter_count")
	assert.NotEmpty(t, paramIssues, "ReconstructCustomer should produce issue when patterns are empty")
}

func TestScoreCodeHealth_DefaultProfileExemptsReconstruct(t *testing.T) {
	p := domain.DefaultProfile()
	assert.Contains(t, p.ExemptParamPatterns, "Reconstruct", "default profile should include Reconstruct")

	result := scoring.ScoreCodeHealth(&p, nil, analyzed(
		makeFile("domain.go", 100,
			makeFunction("ReconstructCustomer", 30, 69, 1, 0),
		),
	))

	sm := subMetricByName(result, "parameter_count")
	require.NotNil(t, sm)
	assert.Equal(t, 20, sm.Score, "default profile should exempt Reconstruct from parameter_count")
}

func TestScoreCodeHealth_MultipleExemptPatterns(t *testing.T) {
	p := domain.DefaultProfile()
	p.ExemptParamPatterns = []string{"Reconstruct", "Hydrate", "MapFrom"}

	result := scoring.ScoreCodeHealth(&p, nil, analyzed(
		makeFile("mapper.go", 100,
			makeFunction("ReconstructCustomer", 30, 69, 1, 0),
			makeFunction("HydrateOrder", 30, 20, 1, 0),
			makeFunction("MapFromDB", 30, 15, 1, 0),
			makeFunction("ProcessPayment", 30, 10, 1, 0),
		),
	))

	sm := subMetricByName(result, "parameter_count")
	require.NotNil(t, sm)
	// 3 exempt (full) + 1 zero (ProcessPayment 10 params) = 3/4 = 75% → Round(15) = 15
	assert.Equal(t, 15, sm.Score, "all three patterns should be exempt")
}

// ---------------------------------------------------------------------------
// Pattern field on issues
// ---------------------------------------------------------------------------

func TestScoreCodeHealth_PatternFieldPopulated(t *testing.T) {
	// Reconstruct → "reconstruct", New → "constructor", Test → "test", other → ""
	result := scoring.ScoreCodeHealth(defaultProfile(), nil, analyzed(
		makeFile("service.go", 100,
			makeFunction("ReconstructOrder", 200, 2, 1, 0),  // function_size issue, pattern=reconstruct
			makeFunction("NewService", 200, 2, 1, 0),        // function_size issue, pattern=constructor
			makeFunction("TestSomething", 200, 2, 1, 0),     // function_size issue, pattern=test
			makeFunction("ProcessPayment", 200, 2, 1, 0),    // function_size issue, pattern=""
		),
	))

	funcIssues := issuesBySubMetric(result.Issues, "function_size")
	require.Len(t, funcIssues, 4)

	patterns := make(map[string]string)
	for _, iss := range funcIssues {
		for _, name := range []string{"ReconstructOrder", "NewService", "TestSomething", "ProcessPayment"} {
			if strings.Contains(iss.Message, name) {
				patterns[name] = iss.Pattern
			}
		}
	}

	assert.Equal(t, "reconstruct", patterns["ReconstructOrder"])
	assert.Equal(t, "constructor", patterns["NewService"])
	assert.Equal(t, "test", patterns["TestSomething"])
	assert.Equal(t, "", patterns["ProcessPayment"])
}

func TestScoreCodeHealth_FilePatternForGeneratedPaths(t *testing.T) {
	// File in sqlc/ path should get pattern "generated" on file_size issues.
	result := scoring.ScoreCodeHealth(defaultProfile(), nil, analyzed(
		makeFile("internal/sqlc/queries.go", 800,
			makeFunction("Fn", 20, 2, 1, 0),
		),
	))

	fileIssues := issuesBySubMetric(result.Issues, "file_size")
	require.Len(t, fileIssues, 1)
	assert.Equal(t, "generated", fileIssues[0].Pattern)
}

func TestScoreCodeHealth_FilePatternForGenSuffix(t *testing.T) {
	result := scoring.ScoreCodeHealth(defaultProfile(), nil, analyzed(
		makeFile("models_gen.go", 800,
			makeFunction("Fn", 20, 2, 1, 0),
		),
	))

	fileIssues := issuesBySubMetric(result.Issues, "file_size")
	require.Len(t, fileIssues, 1)
	assert.Equal(t, "generated", fileIssues[0].Pattern)
}

// ---------------------------------------------------------------------------
// Test file relaxed thresholds
// ---------------------------------------------------------------------------

func TestScoreCodeHealth_TestFilesGetRelaxedThresholds(t *testing.T) {
	// Default: MaxFunctionLines=50. Test files get 2x = 100 for full credit.
	// A 90-line function in a test file should get full credit.
	// The same function in a source file should get partial credit.
	tests := []struct {
		name      string
		file      string
		lines     int
		subMetric string
		wantScore int
	}{
		// function_size: test threshold = 100 (50*2), source threshold = 50
		{"90-line test function gets full credit", "service_test.go", 90, "function_size", 20},
		{"90-line source function gets partial credit", "service.go", 90, "function_size", 10},

		// file_size: test threshold = 600 (300*2), source threshold = 300
		{"500-line test file gets full credit", "handler_test.go", 0, "file_size", 20},
		{"500-line source file gets partial credit", "handler.go", 0, "file_size", 10},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var af *domain.AnalyzedFile
			if tt.subMetric == "file_size" {
				af = makeFile(tt.file, 500, makeFunction("Fn", 20, 2, 1, 0))
			} else {
				af = makeFile(tt.file, 100, makeFunction("Fn", tt.lines, 2, 1, 0))
			}

			result := scoring.ScoreCodeHealth(defaultProfile(), nil, analyzed(af))

			sm := subMetricByName(result, tt.subMetric)
			require.NotNil(t, sm)
			assert.Equal(t, tt.wantScore, sm.Score, "%s: %s", tt.subMetric, tt.name)
		})
	}
}

func TestScoreCodeHealth_TestFileNestingRelaxed(t *testing.T) {
	// Default: MaxNestingDepth=3. Test files get 3+1=4 for full credit.
	// Nesting of 4 in test = full credit. In source = partial credit.
	testResult := scoring.ScoreCodeHealth(defaultProfile(), nil, analyzed(
		makeFile("handler_test.go", 100, makeFunction("TestHandler", 30, 2, 4, 0)),
	))
	srcResult := scoring.ScoreCodeHealth(defaultProfile(), nil, analyzed(
		makeFile("handler.go", 100, makeFunction("Handle", 30, 2, 4, 0)),
	))

	testSM := subMetricByName(testResult, "nesting_depth")
	srcSM := subMetricByName(srcResult, "nesting_depth")
	require.NotNil(t, testSM)
	require.NotNil(t, srcSM)

	assert.Equal(t, 20, testSM.Score, "nesting 4 in test file should get full credit")
	assert.Equal(t, 10, srcSM.Score, "nesting 4 in source file should get partial credit")
}

func TestScoreCodeHealth_TestFileIssuesUseRelaxedThresholds(t *testing.T) {
	// Default issue threshold for function_size: 50*2=100 (source), 50*4=200 (test).
	// A 150-line test function should NOT trigger an issue.
	// A 150-line source function SHOULD trigger an issue.
	testResult := scoring.ScoreCodeHealth(defaultProfile(), nil, analyzed(
		makeFile("service_test.go", 200, makeFunction("TestBigTest", 150, 2, 1, 0)),
	))
	srcResult := scoring.ScoreCodeHealth(defaultProfile(), nil, analyzed(
		makeFile("service.go", 200, makeFunction("BigFunc", 150, 2, 1, 0)),
	))

	testFuncIssues := issuesBySubMetric(testResult.Issues, "function_size")
	srcFuncIssues := issuesBySubMetric(srcResult.Issues, "function_size")

	assert.Empty(t, testFuncIssues, "150-line test function should NOT produce issue (threshold 200)")
	assert.Len(t, srcFuncIssues, 1, "150-line source function SHOULD produce issue (threshold 100)")
}

// ---------------------------------------------------------------------------
// Generated code exclusion
// ---------------------------------------------------------------------------

func makeGeneratedFile(path string, totalLines int, fns ...domain.Function) *domain.AnalyzedFile {
	af := makeFile(path, totalLines, fns...)
	af.IsGenerated = true
	return af
}

func TestScoreCodeHealth_GeneratedFilesExcludedFromScoring(t *testing.T) {
	// A generated file with terrible metrics should NOT affect the score.
	result := scoring.ScoreCodeHealth(defaultProfile(), nil, analyzed(
		makeFile("service.go", 100,
			makeFunction("Clean", 20, 2, 1, 0),
		),
		makeGeneratedFile("internal/database/sqlc/models.go", 3000,
			makeFunction("sqlcQuery", 500, 12, 8, 6),
		),
	))

	// Only service.go should be scored (1 clean function = perfect).
	assert.Equal(t, 100, result.Score, "generated file should not affect score")

	sm := subMetricByName(result, "function_size")
	require.NotNil(t, sm)
	assert.Contains(t, sm.Detail, "1 functions", "only non-generated functions counted")
}

func TestScoreCodeHealth_GeneratedFilesExcludedFromIssues(t *testing.T) {
	result := scoring.ScoreCodeHealth(defaultProfile(), nil, analyzed(
		makeGeneratedFile("sqlc/models.go", 3000,
			makeFunction("HugeGenerated", 500, 12, 8, 6),
		),
	))

	assert.Empty(t, result.Issues, "generated files should produce no issues")
}

func TestScoreCodeHealth_GeneratedFilesExcludedFromFileSize(t *testing.T) {
	// Generated file with 3000 lines should not affect file_size sub-metric.
	result := scoring.ScoreCodeHealth(defaultProfile(), nil, analyzed(
		makeFile("service.go", 100,
			makeFunction("Clean", 20, 2, 1, 0),
		),
		makeGeneratedFile("sqlc/models.go", 3000),
	))

	sm := subMetricByName(result, "file_size")
	require.NotNil(t, sm)
	assert.Equal(t, 20, sm.Score, "generated file should not penalize file_size")
}

func TestScoreCodeHealth_OnlyGeneratedFilesGetFullCredit(t *testing.T) {
	// If ALL files are generated, there's nothing to evaluate → full credit.
	result := scoring.ScoreCodeHealth(defaultProfile(), nil, analyzed(
		makeGeneratedFile("gen_a.go", 1000,
			makeFunction("GenFunc", 300, 10, 7, 5),
		),
		makeGeneratedFile("gen_b.go", 2000),
	))

	assert.Equal(t, 100, result.Score, "all-generated project should get full credit")
	assert.Empty(t, result.Issues)
}

// ---------------------------------------------------------------------------
// Scoring tiers: full credit, partial credit, zero credit
// ---------------------------------------------------------------------------

func TestScoreCodeHealth_ScoringTiers(t *testing.T) {
	// Default: MaxFunctionLines=50, MaxNestingDepth=3, MaxParameters=4, MaxConditionalOps=2.
	tests := []struct {
		name      string
		subMetric string
		fn        domain.Function
		wantTier  string // "full", "partial", "zero"
	}{
		// function_size tiers: ≤50=full, 51-100=partial, >100=zero
		{"function within limit", "function_size", makeFunction("Small", 50, 2, 1, 0), "full"},
		{"function at partial boundary", "function_size", makeFunction("Medium", 75, 2, 1, 0), "partial"},
		{"function exceeds all limits", "function_size", makeFunction("Huge", 200, 2, 1, 0), "zero"},

		// nesting tiers: ≤3=full, 4=partial, >4=zero
		{"nesting within limit", "nesting_depth", makeFunction("Shallow", 20, 2, 3, 0), "full"},
		{"nesting at partial boundary", "nesting_depth", makeFunction("SemiDeep", 20, 2, 4, 0), "partial"},
		{"nesting exceeds limit", "nesting_depth", makeFunction("Deep", 20, 2, 6, 0), "zero"},

		// parameter_count tiers: ≤4=full, 5-6=partial, >6=zero
		{"params within limit", "parameter_count", makeFunction("FewParams", 20, 4, 1, 0), "full"},
		{"params at partial boundary", "parameter_count", makeFunction("SomeParams", 20, 5, 1, 0), "partial"},
		{"params exceeds limit", "parameter_count", makeFunction("ManyParams", 20, 8, 1, 0), "zero"},

		// complex_conditionals tiers: ≤2=full, 3=partial, >3=zero
		{"conditionals within limit", "complex_conditionals", makeFunction("Simple", 20, 2, 1, 2), "full"},
		{"conditionals at partial boundary", "complex_conditionals", makeFunction("SemiComplex", 20, 2, 1, 3), "partial"},
		{"conditionals exceeds limit", "complex_conditionals", makeFunction("Complex", 20, 2, 1, 5), "zero"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := scoring.ScoreCodeHealth(defaultProfile(), nil, analyzed(
				makeFile("test.go", 100, tt.fn),
			))

			sm := subMetricByName(result, tt.subMetric)
			require.NotNilf(t, sm, "sub-metric %s not found", tt.subMetric)

			switch tt.wantTier {
			case "full":
				assert.Equal(t, 20, sm.Score, "%s should get full credit", tt.subMetric)
			case "partial":
				assert.Equal(t, 10, sm.Score, "%s should get partial credit", tt.subMetric)
			case "zero":
				assert.Equal(t, 0, sm.Score, "%s should get zero credit", tt.subMetric)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// File-size scoring tiers
// ---------------------------------------------------------------------------

func TestScoreCodeHealth_FileSizeTiers(t *testing.T) {
	// Default: MaxFileLines=300, partial limit=500 (300*5/3), issue threshold=500.
	tests := []struct {
		name       string
		totalLines int
		wantScore  int // out of 20
	}{
		{"small file", 100, 20},
		{"at limit", 300, 20},
		{"partial credit zone", 400, 10},
		{"at partial limit", 500, 10},
		{"exceeds all limits", 800, 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := scoring.ScoreCodeHealth(defaultProfile(), nil, analyzed(
				makeFile("service.go", tt.totalLines,
					makeFunction("Foo", 20, 2, 1, 0),
				),
			))

			sm := subMetricByName(result, "file_size")
			require.NotNil(t, sm)
			assert.Equal(t, tt.wantScore, sm.Score)
		})
	}
}

// ---------------------------------------------------------------------------
// Custom profile: thresholds respect profile configuration
// ---------------------------------------------------------------------------

func TestScoreCodeHealth_CustomProfileThresholds(t *testing.T) {
	p := domain.DefaultProfile()
	p.MaxFunctionLines = 100 // relaxed from default 50

	// 90-line function: within 100-line limit → full credit.
	result := scoring.ScoreCodeHealth(&p, nil, analyzed(
		makeFile("service.go", 150,
			makeFunction("BigFunc", 90, 2, 1, 0),
		),
	))

	sm := subMetricByName(result, "function_size")
	require.NotNil(t, sm)
	assert.Equal(t, 20, sm.Score, "90 lines within custom max of 100 should get full credit")
}

// ---------------------------------------------------------------------------
// Multi-file scoring: dilution and aggregate behavior
// ---------------------------------------------------------------------------

func TestScoreCodeHealth_MultiFileAggregation(t *testing.T) {
	// 9 clean functions + 1 violation = 90% → math.Round(18.0) = 18
	files := make([]*domain.AnalyzedFile, 0, 10)
	for i := range 9 {
		files = append(files, makeFile(
			"clean_"+string(rune('a'+i))+".go", 100,
			makeFunction("Clean"+string(rune('A'+i)), 30, 2, 1, 0),
		))
	}
	files = append(files, makeFile("bad.go", 100,
		makeFunction("Terrible", 200, 8, 6, 5), // all sub-metrics violated
	))

	result := scoring.ScoreCodeHealth(defaultProfile(), nil, analyzed(files...))

	// function_size: 9 full + 1 zero = 9/10 = 90% → Round(18.0) = 18
	sm := subMetricByName(result, "function_size")
	require.NotNil(t, sm)
	assert.Equal(t, 18, sm.Score)
}

// ---------------------------------------------------------------------------
// Score bounds: never negative, never exceeds 100
// ---------------------------------------------------------------------------

func TestScoreCodeHealth_ScoreBounds(t *testing.T) {
	tests := []struct {
		name     string
		analyzed map[string]*domain.AnalyzedFile
	}{
		{
			name:     "all perfect",
			analyzed: analyzed(makeFile("perfect.go", 50, makeFunction("Do", 10, 1, 0, 0))),
		},
		{
			name:     "all terrible",
			analyzed: analyzed(makeFile("terrible.go", 2000, makeFunction("Mess", 500, 20, 10, 8))),
		},
		{
			name:     "empty",
			analyzed: analyzed(),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := scoring.ScoreCodeHealth(defaultProfile(), nil, tt.analyzed)

			assert.GreaterOrEqual(t, result.Score, 0, "score must never be negative")
			assert.LessOrEqual(t, result.Score, 100, "score must never exceed 100")

			for _, sm := range result.SubMetrics {
				assert.GreaterOrEqual(t, sm.Score, 0, "sub-metric %s score must be >= 0", sm.Name)
				assert.LessOrEqual(t, sm.Score, sm.Points, "sub-metric %s score must be <= points", sm.Name)
			}
		})
	}
}
