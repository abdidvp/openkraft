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
		"function_size", "file_size", "cognitive_complexity",
		"parameter_count", "code_duplication",
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
				"cognitive_complexity": "no functions to evaluate",
				"parameter_count":      "no functions to evaluate",
				"code_duplication":     "no duplication detected",
			},
		},
		{
			name:        "nil map gives 100 — nil is equivalent to empty",
			analyzed:    nil,
			expectScore: 100,
			expectDetails: map[string]string{
				"function_size":        "no functions to evaluate",
				"file_size":            "no files to evaluate",
				"cognitive_complexity": "no functions to evaluate",
				"parameter_count":      "no functions to evaluate",
				"code_duplication":     "no duplication detected",
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
				"cognitive_complexity": "no functions to evaluate",
				"parameter_count":      "no functions to evaluate",
				"code_duplication":     "no duplication detected",
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
	// Default profile: MaxFunctionLines=50, continuous decay with k=4.
	// 39 within limit (1.0 each) + 1 at 70 lines: decay(70,50,k=4)=0.9
	// earned = 39.0 + 0.9 = 39.9/40 = 0.9975 → round(19.95) = 20
	fns := make([]domain.Function, 0, 40)
	for i := range 39 {
		fns = append(fns, makeFunction("Good"+string(rune('A'+i%26)), 30, 2, 1, 0))
	}
	fns = append(fns, makeFunction("SlightlyLong", 70, 2, 1, 0)) // decay credit 0.8

	result := scoring.ScoreCodeHealth(defaultProfile(), nil, analyzed(
		makeFile("service.go", 200, fns...),
	))

	sm := subMetricByName(result, "function_size")
	require.NotNil(t, sm)
	assert.Equal(t, 20, sm.Score, "99.5%% ratio should round UP to full credit")
}

func TestScoreCodeHealth_RoundingDoesNotOveraward(t *testing.T) {
	// 18 good(30 lines) + 2 at 250 lines. decay(250,50,k=4) = 0.0
	// earned = 18.0/20 = 0.9 → round(18.0) = 18
	fns := make([]domain.Function, 0, 20)
	for i := range 18 {
		fns = append(fns, makeFunction("Good"+string(rune('A'+i%26)), 30, 2, 1, 0))
	}
	fns = append(fns, makeFunction("PartialA", 250, 2, 1, 0))
	fns = append(fns, makeFunction("PartialB", 250, 2, 1, 0))

	result := scoring.ScoreCodeHealth(defaultProfile(), nil, analyzed(
		makeFile("service.go", 200, fns...),
	))

	sm := subMetricByName(result, "function_size")
	require.NotNil(t, sm)
	assert.Equal(t, 18, sm.Score, "90%% ratio should yield 18")
}

func TestScoreCodeHealth_RoundingLowerBoundary(t *testing.T) {
	// 9 full + 1 at 250 lines. decay(250,50,k=4) = 0.0
	// earned = 9.0/10 = 0.9 → round(18.0) = 18
	fns := make([]domain.Function, 0, 10)
	for i := range 9 {
		fns = append(fns, makeFunction("Good"+string(rune('A'+i)), 30, 2, 1, 0))
	}
	fns = append(fns, makeFunction("Bad", 250, 2, 1, 0)) // decay credit 0.0 (5x threshold)

	result := scoring.ScoreCodeHealth(defaultProfile(), nil, analyzed(
		makeFile("service.go", 200, fns...),
	))

	sm := subMetricByName(result, "function_size")
	require.NotNil(t, sm)
	assert.Equal(t, 18, sm.Score, "90%% ratio should yield 18")
}

// ---------------------------------------------------------------------------
// P0 Bug Fix: SubMetric field populated on every issue
// ---------------------------------------------------------------------------

func TestScoreCodeHealth_AllIssuesHaveSubMetric(t *testing.T) {
	validSubMetrics := map[string]bool{
		"function_size":        true,
		"file_size":            true,
		"cognitive_complexity": true,
		"parameter_count":      true,
		"code_duplication":     true,
	}

	// Build a file that triggers function_size, cognitive_complexity, parameter_count, file_size issues.
	monsterFn := makeFunction("Huge", 200, 8, 6, 5)
	monsterFn.CognitiveComplexity = 50
	result := scoring.ScoreCodeHealth(defaultProfile(), nil, analyzed(
		makeFile("monster.go", 600, monsterFn),
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
	// Default profile thresholds: MaxFunctionLines=50, MaxCognitiveComplexity=25,
	// MaxParameters=4, MaxFileLines=300.
	ccFunc := makeFunction("ComplexFunc", 20, 2, 1, 0)
	ccFunc.CognitiveComplexity = 30

	result := scoring.ScoreCodeHealth(defaultProfile(), nil, analyzed(
		makeFile("violations.go", 600,
			// 150 lines → triggers function_size issue (>50)
			makeFunction("BigFunc", 150, 2, 1, 0),
			// CC=30 → triggers cognitive_complexity issue (>25)
			ccFunc,
			// 8 params → triggers parameter_count issue (>4)
			makeFunction("ManyParams", 20, 8, 1, 0),
		),
	))

	funcSizeIssues := issuesBySubMetric(result.Issues, "function_size")
	ccIssues := issuesBySubMetric(result.Issues, "cognitive_complexity")
	paramIssues := issuesBySubMetric(result.Issues, "parameter_count")
	fileSizeIssues := issuesBySubMetric(result.Issues, "file_size")

	assert.Len(t, funcSizeIssues, 1, "expected 1 function_size issue for BigFunc")
	assert.Len(t, ccIssues, 1, "expected 1 cognitive_complexity issue for ComplexFunc")
	assert.Len(t, paramIssues, 1, "expected 1 parameter_count issue for ManyParams")
	assert.Len(t, fileSizeIssues, 1, "expected 1 file_size issue for 600-line file")
}

func TestScoreCodeHealth_CognitiveComplexityIssueGeneration(t *testing.T) {
	// Default: MaxCognitiveComplexity=25, issue threshold = 25.
	// CC=30 should trigger an issue (30 > 25).
	fn := makeFunction("TooComplex", 30, 2, 1, 0)
	fn.CognitiveComplexity = 30
	result := scoring.ScoreCodeHealth(defaultProfile(), nil, analyzed(
		makeFile("complex.go", 100, fn),
	))

	ccIssues := issuesBySubMetric(result.Issues, "cognitive_complexity")
	require.Len(t, ccIssues, 1, "expected 1 cognitive_complexity issue")
	assert.Contains(t, ccIssues[0].Message, "cognitive complexity")
	assert.Equal(t, "complex.go", ccIssues[0].File)
}

// ---------------------------------------------------------------------------
// Severity tiering: error/warning/info based on how far actual exceeds threshold
// ---------------------------------------------------------------------------

func TestScoreCodeHealth_SeverityTiering(t *testing.T) {
	// Default: MaxParameters=4, issue threshold = 4 (aligned with scoring).
	// issueSeverity(actual, threshold):
	//   actual/threshold >= 3.0 → error
	//   actual/threshold >= 1.5 → warning
	//   else                    → info
	tests := []struct {
		name      string
		params    int
		wantSev   string
		ratioDesc string
	}{
		// 69 params / 4 threshold = 17.25x → error
		{"extreme violation (17x)", 69, domain.SeverityError, "17.25x"},
		// 12 params / 4 threshold = 3.0x → error
		{"3x threshold boundary", 12, domain.SeverityError, "3.0x"},
		// 8 params / 4 threshold = 2.0x → warning
		{"2x threshold", 8, domain.SeverityWarning, "2.0x"},
		// 6 params / 4 threshold = 1.5x → warning
		{"1.5x threshold", 6, domain.SeverityWarning, "1.5x"},
		// 5 params / 4 threshold = 1.25x → info
		{"just over threshold", 5, domain.SeverityInfo, "1.25x"},
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
	// Default: MaxFileLines=300, issue threshold = 300 (aligned with scoring).
	// 900 / 300 = 3.0x → error
	// 500 / 300 = 1.67x → warning
	// 310 / 300 = 1.03x → info
	tests := []struct {
		name       string
		totalLines int
		wantSev    string
	}{
		{"3x file size → error", 900, domain.SeverityError},
		{"1.67x file size → warning", 500, domain.SeverityWarning},
		{"just over threshold → info", 310, domain.SeverityInfo},
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
		makeFile("mixed.go", 900, // file_size: 900/300 = 3x → error
			makeFunction("Extreme", 20, 69, 1, 0), // params: 69/4 = 17.25x → error
			makeFunction("Moderate", 20, 8, 1, 0), // params: 8/4 = 2.0x → warning
			makeFunction("Slight", 20, 5, 1, 0),   // params: 5/4 = 1.25x → info
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
	// Reconstruct: 1.0 (exempt). ProcessOrder: decay(10, 4, k=4) = 1-6/16 = 0.625
	// earned = 1.625/2 = 0.8125 → Round(16.25) = 16
	assert.Equal(t, 16, sm.Score, "Reconstruct should get full credit, ProcessOrder partial via decay")
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
	// If it has 300 lines, it should still get zero on function_size.
	result := scoring.ScoreCodeHealth(defaultProfile(), nil, analyzed(
		makeFile("domain.go", 100,
			makeFunction("ReconstructCustomer", 300, 69, 1, 0),
		),
	))

	funcSM := subMetricByName(result, "function_size")
	paramSM := subMetricByName(result, "parameter_count")
	require.NotNil(t, funcSM)
	require.NotNil(t, paramSM)

	// decay(300, 50, k=4) = 0.0 (300 > 250 = 5x threshold) → score 0
	assert.Equal(t, 0, funcSM.Score, "Reconstruct with 300 lines still penalized on function_size")
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
	// HydrateUser exempt (1.0) + ProcessOrder decay(10,4,k=4)=0.625 = 1.625/2 = 0.8125 → 16
	assert.Equal(t, 16, sm.Score, "Hydrate pattern should exempt HydrateUser but not ProcessOrder")

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
	// 3 exempt (1.0 each) + ProcessPayment decay(10,4,k=4)=0.625 = 3.625/4 = 0.90625 → Round(18.125) = 18
	assert.Equal(t, 18, sm.Score, "all three patterns should be exempt")
}

// ---------------------------------------------------------------------------
// Pattern field on issues
// ---------------------------------------------------------------------------

func TestScoreCodeHealth_PatternFieldPopulated(t *testing.T) {
	// Reconstruct → "reconstruct", New → "constructor", Test → "test", other → ""
	result := scoring.ScoreCodeHealth(defaultProfile(), nil, analyzed(
		makeFile("service.go", 100,
			makeFunction("ReconstructOrder", 200, 2, 1, 0), // function_size issue, pattern=reconstruct
			makeFunction("NewService", 200, 2, 1, 0),       // function_size issue, pattern=constructor
			makeFunction("TestSomething", 200, 2, 1, 0),    // function_size issue, pattern=test
			makeFunction("ProcessPayment", 200, 2, 1, 0),   // function_size issue, pattern=""
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
		// 90-line source: decay(90,50,k=4) = 1-40/200 = 0.8 → round(16) = 16
		{"90-line source function gets decay credit", "service.go", 90, "function_size", 16},

		// file_size: test threshold = 600 (300*2), source threshold = 300
		{"500-line test file gets full credit", "handler_test.go", 0, "file_size", 20},
		// 500-line source: decay(500,300,k=4) = 1-200/1200 = 0.833 → round(16.67) = 17
		{"500-line source file gets decay credit", "handler.go", 0, "file_size", 17},
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

func TestScoreCodeHealth_TestFileCCRelaxed(t *testing.T) {
	// Default: MaxCognitiveComplexity=25. Test files get 25+5=30 for full credit.
	// CC=28 in test = full credit. In source = partial credit.
	testFn := makeFunctionCC("TestHandler", 30, 2, 1, 0, 28)
	srcFn := makeFunctionCC("Handle", 30, 2, 1, 0, 28)

	testResult := scoring.ScoreCodeHealth(defaultProfile(), nil, analyzed(
		makeFile("handler_test.go", 100, testFn),
	))
	srcResult := scoring.ScoreCodeHealth(defaultProfile(), nil, analyzed(
		makeFile("handler.go", 100, srcFn),
	))

	testSM := subMetricByName(testResult, "cognitive_complexity")
	srcSM := subMetricByName(srcResult, "cognitive_complexity")
	require.NotNil(t, testSM)
	require.NotNil(t, srcSM)

	assert.Equal(t, 20, testSM.Score, "CC 28 in test file (threshold 30) should get full credit")
	// decay(28, 25, k=4) = 1 - 3/100 = 0.97 → round(19.4) = 19
	assert.Equal(t, 19, srcSM.Score, "CC 28 in source file should get decay credit")
}

func TestScoreCodeHealth_TestFileIssuesUseRelaxedThresholds(t *testing.T) {
	// Default issue threshold for function_size: 50 (source), 100 (test).
	// A 90-line test function should NOT trigger an issue (90 ≤ 100).
	// A 90-line source function SHOULD trigger an issue (90 > 50).
	testResult := scoring.ScoreCodeHealth(defaultProfile(), nil, analyzed(
		makeFile("service_test.go", 200, makeFunction("TestBigTest", 90, 2, 1, 0)),
	))
	srcResult := scoring.ScoreCodeHealth(defaultProfile(), nil, analyzed(
		makeFile("service.go", 200, makeFunction("BigFunc", 90, 2, 1, 0)),
	))

	testFuncIssues := issuesBySubMetric(testResult.Issues, "function_size")
	srcFuncIssues := issuesBySubMetric(srcResult.Issues, "function_size")

	assert.Empty(t, testFuncIssues, "90-line test function should NOT produce issue (threshold 100)")
	assert.Len(t, srcFuncIssues, 1, "90-line source function SHOULD produce issue (threshold 50)")
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

func makeFunctionCC(name string, lines, params, nesting, condOps, cogCC int) domain.Function {
	fn := makeFunction(name, lines, params, nesting, condOps)
	fn.CognitiveComplexity = cogCC
	return fn
}

func TestScoreCodeHealth_ContinuousDecay(t *testing.T) {
	// Default: MaxFunctionLines=50, MaxCognitiveComplexity=15, MaxParameters=4.
	// With k=4, zero credit at 5x threshold.
	tests := []struct {
		name      string
		subMetric string
		fn        domain.Function
		wantScore int
	}{
		// function_size: threshold=50, k=4, zero at 250
		{"function within limit", "function_size", makeFunction("Small", 50, 2, 1, 0), 20},
		// decay(75,50,k=4) = 1 - 25/200 = 0.875 → round(17.5) = 18
		{"function slightly over", "function_size", makeFunction("Medium", 75, 2, 1, 0), 18},
		// decay(100,50,k=4) = 1 - 50/200 = 0.75 → round(15.0) = 15
		{"function at 2x threshold", "function_size", makeFunction("Big", 100, 2, 1, 0), 15},
		// decay(250,50,k=4) = 0.0 → 0
		{"function at zero boundary", "function_size", makeFunction("Extreme", 250, 2, 1, 0), 0},

		// cognitive_complexity: threshold=25, k=4, zero at 125
		{"CC within limit", "cognitive_complexity", makeFunctionCC("Low", 20, 2, 1, 0, 25), 20},
		// decay(35,25,k=4) = 1 - 10/100 = 0.9 → round(18.0) = 18
		{"CC slightly over", "cognitive_complexity", makeFunctionCC("Medium", 20, 2, 1, 0, 35), 18},
		// decay(50,25,k=4) = 1 - 25/100 = 0.75 → round(15.0) = 15
		{"CC well over", "cognitive_complexity", makeFunctionCC("High", 20, 2, 1, 0, 50), 15},
		// decay(125,25,k=4) = 0.0 → 0
		{"CC at zero boundary", "cognitive_complexity", makeFunctionCC("Extreme", 20, 2, 1, 0, 125), 0},

		// parameter_count: threshold=4, k=4, zero at 20
		{"params within limit", "parameter_count", makeFunction("FewParams", 20, 4, 1, 0), 20},
		// decay(5,4,k=4) = 1 - 1/16 = 0.9375 → round(18.75) = 19
		{"params slightly over", "parameter_count", makeFunction("SomeParams", 20, 5, 1, 0), 19},
		// decay(8,4,k=4) = 1 - 4/16 = 0.75 → round(15.0) = 15
		{"params well over", "parameter_count", makeFunction("ManyParams", 20, 8, 1, 0), 15},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := scoring.ScoreCodeHealth(defaultProfile(), nil, analyzed(
				makeFile("test.go", 100, tt.fn),
			))

			sm := subMetricByName(result, tt.subMetric)
			require.NotNilf(t, sm, "sub-metric %s not found", tt.subMetric)
			assert.Equal(t, tt.wantScore, sm.Score, "%s decay score", tt.subMetric)
		})
	}
}

// ---------------------------------------------------------------------------
// File-size scoring tiers
// ---------------------------------------------------------------------------

func TestScoreCodeHealth_FileSizeDecay(t *testing.T) {
	// Default: MaxFileLines=300, k=4, zero at 1500.
	tests := []struct {
		name       string
		totalLines int
		wantScore  int // out of 20
	}{
		{"small file", 100, 20},
		{"at limit", 300, 20},
		// decay(400,300,k=4) = 1 - 100/1200 = 0.917 → round(18.33) = 18
		{"slightly over", 400, 18},
		// decay(500,300,k=4) = 1 - 200/1200 = 0.833 → round(16.67) = 17
		{"moderately over", 500, 17},
		// decay(800,300,k=4) = 1 - 500/1200 = 0.583 → round(11.67) = 12
		{"well over", 800, 12},
		// decay(1500,300,k=4) = 1 - 1200/1200 = 0.0 → 0
		{"at zero boundary", 1500, 0},
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
	// 9 clean functions + 1 with 300 lines.
	// decay(300,50,k=4) = 0.0 (>5x threshold). earned = 9.0/10 = 0.9 → round(18.0) = 18
	files := make([]*domain.AnalyzedFile, 0, 10)
	for i := range 9 {
		files = append(files, makeFile(
			"clean_"+string(rune('a'+i))+".go", 100,
			makeFunction("Clean"+string(rune('A'+i)), 30, 2, 1, 0),
		))
	}
	files = append(files, makeFile("bad.go", 100,
		makeFunction("Terrible", 300, 8, 6, 5), // all sub-metrics violated
	))

	result := scoring.ScoreCodeHealth(defaultProfile(), nil, analyzed(files...))

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

// ---------------------------------------------------------------------------
// Fix: Issues reported at scoring boundary (no silent zone)
// ---------------------------------------------------------------------------

func TestScoreCodeHealth_IssuesReportedAtScoringBoundary(t *testing.T) {
	// Default: MaxFunctionLines=50. A 51-line function loses score AND generates an issue.
	// A 50-line function gets full credit and no issue.
	tests := []struct {
		name       string
		lines      int
		wantIssues int
	}{
		{"at threshold (50 lines) — no issue", 50, 0},
		{"just over threshold (51 lines) — info issue", 51, 1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := scoring.ScoreCodeHealth(defaultProfile(), nil, analyzed(
				makeFile("service.go", 100,
					makeFunction("Fn", tt.lines, 2, 1, 0),
				),
			))

			funcIssues := issuesBySubMetric(result.Issues, "function_size")
			assert.Len(t, funcIssues, tt.wantIssues)
			if tt.wantIssues > 0 {
				assert.Equal(t, domain.SeverityInfo, funcIssues[0].Severity,
					"51-line function should be info severity (just over threshold)")
			}
		})
	}
}

// ---------------------------------------------------------------------------
// Fix: Outlier penalty — extreme functions subtract points
// ---------------------------------------------------------------------------

func TestScoreCodeHealth_ExtremeOutliersGetZeroCredit(t *testing.T) {
	// With k=4, functions at ≥5x threshold get exactly 0.0 credit.
	// Default: MaxFunctionLines=50, zero at 250.
	// 9 clean + 1 at 300 lines. decay(300,50,k=4) = 0.0
	// earned = 9.0/10 = 0.9 → round(18.0) = 18
	fns := make([]domain.Function, 0, 10)
	for i := range 9 {
		fns = append(fns, makeFunction("Good"+string(rune('A'+i)), 30, 2, 1, 0))
	}
	fns = append(fns, makeFunction("Extreme", 300, 2, 1, 0))

	result := scoring.ScoreCodeHealth(defaultProfile(), nil, analyzed(
		makeFile("service.go", 400, fns...),
	))

	sm := subMetricByName(result, "function_size")
	require.NotNil(t, sm)
	assert.Equal(t, 18, sm.Score, "extreme outlier gets zero credit via decay")
}

func TestScoreCodeHealth_ExtremeFileGetZeroCredit(t *testing.T) {
	// Default: MaxFileLines=300, k=4, zero at 1500.
	// 2 files: 1 clean (200) + 1 at 1600. decay(1600,300,k=4) = 0.0
	// earned = 1.0/2 = 0.5 → round(10.0) = 10
	result := scoring.ScoreCodeHealth(defaultProfile(), nil, analyzed(
		makeFile("clean.go", 200, makeFunction("A", 20, 2, 1, 0)),
		makeFile("huge.go", 1600, makeFunction("B", 20, 2, 1, 0)),
	))

	sm := subMetricByName(result, "file_size")
	require.NotNil(t, sm)
	assert.Equal(t, 10, sm.Score, "extreme file gets zero credit via decay")
}

func TestScoreCodeHealth_AllExtremeOutliersGetZero(t *testing.T) {
	// All functions beyond 5x threshold (≥250 lines) → all get 0.0 credit → score = 0.
	result := scoring.ScoreCodeHealth(defaultProfile(), nil, analyzed(
		makeFile("terrible.go", 100,
			makeFunction("A", 300, 2, 1, 0),
			makeFunction("B", 400, 2, 1, 0),
			makeFunction("C", 500, 2, 1, 0),
		),
	))

	sm := subMetricByName(result, "function_size")
	require.NotNil(t, sm)
	assert.Equal(t, 0, sm.Score, "all extreme outliers yield zero score")
}

// ---------------------------------------------------------------------------
// Severity-weighted penalty: hybrid model deducts points based on issue density
// ---------------------------------------------------------------------------

func TestScoreCodeHealth_SeverityPenaltyApplied(t *testing.T) {
	// A codebase with many violations should have its score reduced by the penalty.
	// 10 functions: 5 clean + 5 with 200 lines (=error severity: 200/50=4x > 3x).
	// Base: function_size decay(200,50,k=2)=0.0 for 5, 1.0 for 5 → 50% → 10/20
	// Other sub-metrics: all 10 within limits → 80/80
	// Base = 10 + 80 = 90
	// Issues: 5 function_size errors (200/50=4x ≥ 3x → error)
	// severity_weight = 5*3 = 15
	// debtRatio = 15 / 10 = 1.5
	// penalty = round(1.5 * 120) = 180 → clamped by max(0, 90-180) = 0
	// But that's too harsh for a test. Let's use fewer violations.
	//
	// Instead: 20 functions, 2 with 200 lines (error severity).
	// Base: function_size: 18/20 = 0.9 → 18. Other 4 sub-metrics: 20 each = 80.
	// Base = 98.
	// Issues: 2 function_size errors.
	// severity_weight = 2*3 = 6
	// debtRatio = 6 / 20 = 0.3
	// penalty = round(0.3 * 120) = 36 → max(0, 98-36) = 62
	// Still too harsh for a simple test. Let's verify with 100 functions.
	//
	// 100 functions: 95 clean + 5 with 200 lines.
	// Base: function_size: decay(200,50,k=4)=0.25 per bad fn.
	// earned = (95 + 5*0.25)/100 = 96.25/100 = 0.9625 → round(19.25) = 19. Others: 80.
	// Base = 99.
	// Issues: 5 errors (200/50=4x ≥ 3x). weight = 15. debtRatio = 15/100 = 0.15.
	// penalty = round(0.15 * 120) = round(18) = 18.
	// Score = max(0, 99-18) = 81.
	fns := make([]domain.Function, 0, 100)
	for i := range 95 {
		fns = append(fns, makeFunction("Good"+string(rune('A'+i%26)), 30, 2, 1, 0))
	}
	for i := range 5 {
		fns = append(fns, makeFunction("Bad"+string(rune('A'+i)), 200, 2, 1, 0))
	}

	result := scoring.ScoreCodeHealth(defaultProfile(), nil, analyzed(
		makeFile("service.go", 100, fns...),
	))

	// Base sub-metric score
	base := 0
	for _, sm := range result.SubMetrics {
		base += sm.Score
	}
	assert.Equal(t, 99, base, "base sub-metric total before penalty")
	assert.Less(t, result.Score, base, "penalty should reduce score below base")
	assert.Equal(t, 81, result.Score, "score after rate-based severity penalty")
}

func TestScoreCodeHealth_NoPenaltyWhenNoIssues(t *testing.T) {
	// A perfectly clean codebase should have zero penalty.
	result := scoring.ScoreCodeHealth(defaultProfile(), nil, analyzed(
		makeFile("clean.go", 100,
			makeFunction("Do", 20, 2, 1, 0),
		),
	))

	base := 0
	for _, sm := range result.SubMetrics {
		base += sm.Score
	}
	assert.Equal(t, base, result.Score, "no issues = no penalty, score equals base")
	assert.Equal(t, 100, result.Score)
}

func TestScoreCodeHealth_PenaltyNeverExceedsBase(t *testing.T) {
	// Even with extreme violations, score should be clamped at 0.
	result := scoring.ScoreCodeHealth(defaultProfile(), nil, analyzed(
		makeFile("terrible.go", 2000,
			makeFunction("A", 500, 20, 10, 8),
			makeFunction("B", 400, 15, 9, 7),
			makeFunction("C", 300, 12, 8, 6),
		),
	))

	assert.GreaterOrEqual(t, result.Score, 0, "score must never be negative")
}

func TestScoreCodeHealth_PenaltyScalesWithSeverity(t *testing.T) {
	// More severe issues should produce a larger penalty.
	// Both cases have 50 functions to keep the rate-based penalty realistic.
	// Case 1: 49 clean + 1 error (200 lines). debtRatio = 3/50 = 0.06 → penalty 7
	fns1 := make([]domain.Function, 0, 50)
	for i := range 49 {
		fns1 = append(fns1, makeFunction("Good"+string(rune('A'+i%26)), 30, 2, 1, 0))
	}
	fns1 = append(fns1, makeFunction("Bad", 200, 2, 1, 0))
	result1 := scoring.ScoreCodeHealth(defaultProfile(), nil, analyzed(
		makeFile("mild.go", 100, fns1...),
	))

	// Case 2: 45 clean + 5 errors. debtRatio = 15/50 = 0.3 → penalty 36
	fns2 := make([]domain.Function, 0, 50)
	for i := range 45 {
		fns2 = append(fns2, makeFunction("Good"+string(rune('A'+i%26)), 30, 2, 1, 0))
	}
	for i := range 5 {
		fns2 = append(fns2, makeFunction("Bad"+string(rune('A'+i)), 200, 2, 1, 0))
	}
	result2 := scoring.ScoreCodeHealth(defaultProfile(), nil, analyzed(
		makeFile("severe.go", 100, fns2...),
	))

	assert.Greater(t, result1.Score, result2.Score,
		"more severe violations should produce lower score")
}

func TestScoreCodeHealth_PenaltyScaleCalibration(t *testing.T) {
	// Verify that a 6% debt ratio produces ~7 points of penalty.
	// 50 functions: 49 clean + 1 error (200 lines, 200/50=4x ≥ 3x → error).
	// weight = 3. debtRatio = 3/50 = 0.06. penalty = round(0.06 * 120) = 7.
	fns := make([]domain.Function, 0, 50)
	for i := range 49 {
		fns = append(fns, makeFunction("Good"+string(rune('A'+i%26)), 30, 2, 1, 0))
	}
	fns = append(fns, makeFunction("Bad", 200, 2, 1, 0))

	result := scoring.ScoreCodeHealth(defaultProfile(), nil, analyzed(
		makeFile("service.go", 100, fns...),
	))

	base := 0
	for _, sm := range result.SubMetrics {
		base += sm.Score
	}
	penalty := base - result.Score
	assert.Equal(t, 7, penalty, "6%% debt ratio should produce 7 points of penalty with scale=120")
}

func TestScoreCodeHealth_NoSilentZone(t *testing.T) {
	// Every function that loses score must have a corresponding issue.
	// Functions at 51-100 lines lose score (partial credit) and should now generate issues.
	result := scoring.ScoreCodeHealth(defaultProfile(), nil, analyzed(
		makeFile("mixed.go", 200,
			makeFunction("Clean", 50, 2, 1, 0),   // full credit, no issue
			makeFunction("Partial", 75, 2, 1, 0), // partial credit, should have issue
			makeFunction("Zero", 150, 2, 1, 0),   // zero credit, should have issue
		),
	))

	funcIssues := issuesBySubMetric(result.Issues, "function_size")
	// Partial (75 > 50) and Zero (150 > 50) should both generate issues.
	assert.Len(t, funcIssues, 2, "both penalized functions should generate issues")

	// Verify the clean function doesn't generate an issue.
	for _, iss := range funcIssues {
		assert.NotContains(t, iss.Message, "Clean",
			"function at exactly threshold should not generate issue")
	}
}

// ---------------------------------------------------------------------------
// Template function scoring: string-literal-dominated functions get relaxed thresholds
// ---------------------------------------------------------------------------

func makeTemplateFunction(name string, lines int, ratio float64) domain.Function {
	return domain.Function{
		Name:               name,
		Exported:           name[0] >= 'A' && name[0] <= 'Z',
		LineStart:          1,
		LineEnd:            lines,
		Params:             make([]domain.Param, 0),
		MaxNesting:         0,
		MaxCondOps:         0,
		StringLiteralRatio: ratio,
	}
}

func TestScoreCodeHealth_TemplateFunctionGetsFullCredit(t *testing.T) {
	// Default: MaxFunctionLines=50, threshold*5=250.
	// A 200-line template function (ratio 0.9 > 0.8) should get full credit.
	result := scoring.ScoreCodeHealth(defaultProfile(), nil, analyzed(
		makeFile("completions.go", 300,
			makeTemplateFunction("BashCompletion", 200, 0.9),
		),
	))

	sm := subMetricByName(result, "function_size")
	require.NotNil(t, sm)
	assert.Equal(t, 20, sm.Score, "template function within relaxed limit should get full credit")
}

func TestScoreCodeHealth_TemplateFunctionNoIssue(t *testing.T) {
	// A 200-line template function should NOT produce a function_size issue.
	result := scoring.ScoreCodeHealth(defaultProfile(), nil, analyzed(
		makeFile("completions.go", 300,
			makeTemplateFunction("BashCompletion", 200, 0.9),
		),
	))

	funcIssues := issuesBySubMetric(result.Issues, "function_size")
	assert.Empty(t, funcIssues, "template function under relaxed threshold should produce no issues")
}

func TestScoreCodeHealth_NormalFunctionStillPenalized(t *testing.T) {
	// A 300-line normal function (ratio 0.0) should still be penalized.
	// decay(300, 50, k=4) = 0.0 (past 5x threshold).
	result := scoring.ScoreCodeHealth(defaultProfile(), nil, analyzed(
		makeFile("handler.go", 400,
			makeFunction("BigHandler", 300, 2, 1, 0),
		),
	))

	sm := subMetricByName(result, "function_size")
	require.NotNil(t, sm)
	assert.Equal(t, 0, sm.Score, "normal 300-line function should get zero credit")
}

func TestScoreCodeHealth_TemplateFunctionPenalizedAtExtremeSize(t *testing.T) {
	// Default: effectiveMax for template = 50*5 = 250, zero at 250*(4+1) = 1250.
	// A 1300-line template function should get zero credit.
	// decay(1300, 250, k=4) = 1 - 1050/1000 = -0.05 → clamped to 0.0.
	result := scoring.ScoreCodeHealth(defaultProfile(), nil, analyzed(
		makeFile("completions.go", 1400,
			makeTemplateFunction("MassiveTemplate", 1300, 0.95),
		),
	))

	sm := subMetricByName(result, "function_size")
	require.NotNil(t, sm)
	assert.Equal(t, 0, sm.Score, "even template functions get penalized at extreme sizes")
}

func TestScoreCodeHealth_TemplateFunctionCustomThreshold(t *testing.T) {
	// Custom profile: StringLiteralThreshold=0.5, TemplateFuncSizeMultiplier=3.
	// A function with ratio 0.6 (> 0.5) is treated as template.
	// effectiveMax = 50*3 = 150. A 140-line template function → full credit.
	p := domain.DefaultProfile()
	p.StringLiteralThreshold = 0.5
	p.TemplateFuncSizeMultiplier = 3

	result := scoring.ScoreCodeHealth(&p, nil, analyzed(
		makeFile("config.go", 200,
			makeTemplateFunction("EmbeddedConfig", 140, 0.6),
		),
	))

	sm := subMetricByName(result, "function_size")
	require.NotNil(t, sm)
	assert.Equal(t, 20, sm.Score, "custom template threshold should be respected")
}

func TestScoreCodeHealth_TemplateFunctionBelowThresholdNotRelaxed(t *testing.T) {
	// A function with ratio 0.7 (below default 0.8 threshold) is NOT a template.
	// 300-line function with 0.7 ratio → normal scoring → zero credit.
	// decay(300, 50, k=4) = 1 - 250/200 = -0.25 → clamped to 0.0.
	result := scoring.ScoreCodeHealth(defaultProfile(), nil, analyzed(
		makeFile("mixed.go", 400,
			makeTemplateFunction("MixedFunc", 300, 0.7),
		),
	))

	sm := subMetricByName(result, "function_size")
	require.NotNil(t, sm)
	assert.Equal(t, 0, sm.Score, "function below StringLiteralThreshold should not get relaxed scoring")
}

// ---------------------------------------------------------------------------
// Data-heavy test detection: table-driven tests get template multiplier
// ---------------------------------------------------------------------------

func makeDataHeavyTestFunc(name string, lines int) domain.Function {
	return domain.Function{
		Name:       name,
		Exported:   name[0] >= 'A' && name[0] <= 'Z',
		LineStart:  1,
		LineEnd:    lines,
		Params:     make([]domain.Param, 0),
		MaxNesting: 2, // standard pattern: for range { t.Run { if
		MaxCondOps: 0, // no conditional operators
	}
}

func TestScoreCodeHealth_DataHeavyTestGetRelaxedThreshold(t *testing.T) {
	// Default: MaxFunctionLines=50, templateMultiplier=5 → threshold=250.
	// A 200-line low-complexity test function should get full credit.
	result := scoring.ScoreCodeHealth(defaultProfile(), nil, analyzed(
		makeFile("handler_test.go", 300,
			makeDataHeavyTestFunc("TestHandlerCases", 200),
		),
	))

	sm := subMetricByName(result, "function_size")
	require.NotNil(t, sm)
	assert.Equal(t, 20, sm.Score, "data-heavy test within relaxed limit (250) should get full credit")
}

func TestScoreCodeHealth_DataHeavyTestNoIssue(t *testing.T) {
	// Same 200-line data-heavy test → no function_size issue.
	result := scoring.ScoreCodeHealth(defaultProfile(), nil, analyzed(
		makeFile("handler_test.go", 300,
			makeDataHeavyTestFunc("TestHandlerCases", 200),
		),
	))

	funcIssues := issuesBySubMetric(result.Issues, "function_size")
	assert.Empty(t, funcIssues, "data-heavy test under relaxed threshold should produce no issues")
}

func TestScoreCodeHealth_ComplexTestNotRelaxed(t *testing.T) {
	// A 200-line test with MaxNesting=3 is NOT data-heavy → uses normal 2x (threshold=100).
	// decay(200, 100, k=4) = 1 - 100/400 = 0.75 → round(15) = 15
	result := scoring.ScoreCodeHealth(defaultProfile(), nil, analyzed(
		makeFile("handler_test.go", 300,
			makeFunction("TestComplexHandler", 200, 0, 3, 2),
		),
	))

	sm := subMetricByName(result, "function_size")
	require.NotNil(t, sm)
	assert.Equal(t, 15, sm.Score, "complex test should use normal 2x threshold, not data-heavy relaxation")
}

func TestScoreCodeHealth_DataHeavyTestNesting1StillRelaxed(t *testing.T) {
	// A test with MaxNesting=1 (simple for range + t.Run, no if) still qualifies as data-heavy.
	result := scoring.ScoreCodeHealth(defaultProfile(), nil, analyzed(
		makeFile("handler_test.go", 300,
			domain.Function{
				Name:       "TestSimpleTable",
				Exported:   true,
				LineStart:  1,
				LineEnd:    200,
				Params:     make([]domain.Param, 0),
				MaxNesting: 1,
				MaxCondOps: 0,
			},
		),
	))

	sm := subMetricByName(result, "function_size")
	require.NotNil(t, sm)
	assert.Equal(t, 20, sm.Score, "nesting=1 test should still qualify as data-heavy (threshold 250)")
}

func TestScoreCodeHealth_DataHeavyTestNesting3NotRelaxed(t *testing.T) {
	// A test with MaxNesting=3 does NOT qualify as data-heavy → uses normal 2x (threshold=100).
	// decay(200, 100, k=4) = 1 - 100/400 = 0.75 → round(15) = 15
	result := scoring.ScoreCodeHealth(defaultProfile(), nil, analyzed(
		makeFile("handler_test.go", 300,
			domain.Function{
				Name:       "TestDeeplyNested",
				Exported:   true,
				LineStart:  1,
				LineEnd:    200,
				Params:     make([]domain.Param, 0),
				MaxNesting: 3,
				MaxCondOps: 0,
			},
		),
	))

	sm := subMetricByName(result, "function_size")
	require.NotNil(t, sm)
	assert.Equal(t, 15, sm.Score, "nesting=3 test should NOT qualify as data-heavy, uses normal 2x threshold")
}

func TestScoreCodeHealth_DataHeavyTestPenalizedAtExtremeSize(t *testing.T) {
	// Default: threshold=250, zero at 250*(4+1)=1250.
	// A 1300-line data-heavy test → zero credit.
	// decay(1300, 250, k=4) = 1 - 1050/1000 = -0.05 → clamped to 0.0.
	result := scoring.ScoreCodeHealth(defaultProfile(), nil, analyzed(
		makeFile("handler_test.go", 1400,
			makeDataHeavyTestFunc("TestMassiveTable", 1300),
		),
	))

	sm := subMetricByName(result, "function_size")
	require.NotNil(t, sm)
	assert.Equal(t, 0, sm.Score, "even data-heavy tests get penalized at extreme sizes")
}

func TestScoreCodeHealth_DataHeavyTestIssueDowngradedSeverity(t *testing.T) {
	// A 300-line data-heavy test → threshold=250. 300/250=1.2 < 1.5x → info severity.
	result := scoring.ScoreCodeHealth(defaultProfile(), nil, analyzed(
		makeFile("handler_test.go", 400,
			makeDataHeavyTestFunc("TestLargeTable", 300),
		),
	))

	funcIssues := issuesBySubMetric(result.Issues, "function_size")
	require.Len(t, funcIssues, 1)
	assert.Equal(t, domain.SeverityInfo, funcIssues[0].Severity,
		"300/250=1.2x should be info severity, not error")
}

// ---------------------------------------------------------------------------
// CGo/FFI binding parameter exemption: files with import "C" get relaxed param thresholds
// ---------------------------------------------------------------------------

func makeCGoFile(path string, totalLines int, fns ...domain.Function) *domain.AnalyzedFile {
	return &domain.AnalyzedFile{
		Path:         path,
		TotalLines:   totalLines,
		Functions:    fns,
		HasCGoImport: true,
	}
}

func TestScoreCodeHealth_CGoFileRelaxedParameterCount(t *testing.T) {
	// Default: MaxParameters=4, CGoParamThreshold=12.
	// A CGo file with 10 params should get full credit (10 <= 12).
	result := scoring.ScoreCodeHealth(defaultProfile(), nil, analyzed(
		makeCGoFile("gpu.go", 200,
			makeFunction("GpuInit", 30, 10, 1, 0),
		),
	))

	sm := subMetricByName(result, "parameter_count")
	require.NotNil(t, sm)
	assert.Equal(t, 20, sm.Score, "CGo file with 10 params should get full credit (threshold 12)")
}

func TestScoreCodeHealth_CGoFileNoParameterIssue(t *testing.T) {
	// A CGo file with 10 params should NOT produce a parameter_count issue.
	result := scoring.ScoreCodeHealth(defaultProfile(), nil, analyzed(
		makeCGoFile("gpu.go", 200,
			makeFunction("GpuInit", 30, 10, 1, 0),
		),
	))

	paramIssues := issuesBySubMetric(result.Issues, "parameter_count")
	assert.Empty(t, paramIssues, "CGo file with params within CGo threshold should produce no issues")
}

func TestScoreCodeHealth_CGoFileStillPenalizedBeyondThreshold(t *testing.T) {
	// Default: CGoParamThreshold=12. A function with 15 params → penalized.
	// 15 > 12 → issue generated at info severity (15/12=1.25 < 1.5x).
	result := scoring.ScoreCodeHealth(defaultProfile(), nil, analyzed(
		makeCGoFile("gpu.go", 200,
			makeFunction("GpuMegaInit", 30, 15, 1, 0),
		),
	))

	paramIssues := issuesBySubMetric(result.Issues, "parameter_count")
	require.Len(t, paramIssues, 1)
	assert.Equal(t, domain.SeverityInfo, paramIssues[0].Severity,
		"15/12=1.25x should be info severity")
}

func TestScoreCodeHealth_CGoFileOtherMetricsUnaffected(t *testing.T) {
	// CGo exemption only applies to parameter_count. A 200-line function
	// in a CGo file should still be penalized for function_size.
	// decay(200, 50, k=4) = 1 - 150/200 = 0.25
	result := scoring.ScoreCodeHealth(defaultProfile(), nil, analyzed(
		makeCGoFile("gpu.go", 300,
			makeFunction("GpuBigFunc", 200, 2, 1, 0),
		),
	))

	funcIssues := issuesBySubMetric(result.Issues, "function_size")
	assert.NotEmpty(t, funcIssues, "CGo exemption should not affect function_size scoring")
}

func TestScoreCodeHealth_NonCGoFileNotRelaxed(t *testing.T) {
	// A normal file with 10 params should be penalized (10 > 4 default).
	// decay(10, 4, k=4) = 1 - 6/16 = 0.625 → round(12.5) = 13
	result := scoring.ScoreCodeHealth(defaultProfile(), nil, analyzed(
		makeFile("handler.go", 200,
			makeFunction("BigHandler", 30, 10, 1, 0),
		),
	))

	sm := subMetricByName(result, "parameter_count")
	require.NotNil(t, sm)
	assert.Less(t, sm.Score, 20, "non-CGo file with 10 params should not get full credit")
}

// ---------------------------------------------------------------------------
// Switch-dispatch detection: type-switch functions get template multiplier
// ---------------------------------------------------------------------------

func makeSwitchDispatchFunc(name string, lines, caseArms int, avgCaseLines float64) domain.Function {
	return domain.Function{
		Name:         name,
		Exported:     name[0] >= 'A' && name[0] <= 'Z',
		LineStart:    1,
		LineEnd:      lines,
		Params:       make([]domain.Param, 0),
		MaxNesting:   1,
		MaxCondOps:   0,
		MaxCaseArms:  caseArms,
		AvgCaseLines: avgCaseLines,
	}
}

func TestScoreCodeHealth_SwitchDispatchGetRelaxedThreshold(t *testing.T) {
	// Default: MaxFunctionLines=50, templateMultiplier=5 → threshold=250.
	// A 130-line switch-dispatch function with 40 cases, avg 1.5 lines → full credit.
	result := scoring.ScoreCodeHealth(defaultProfile(), nil, analyzed(
		makeFile("field.go", 200,
			makeSwitchDispatchFunc("Any", 130, 40, 1.5),
		),
	))

	sm := subMetricByName(result, "function_size")
	require.NotNil(t, sm)
	assert.Equal(t, 20, sm.Score, "switch-dispatch function within relaxed limit (250) should get full credit")
}

func TestScoreCodeHealth_SwitchDispatchNoIssue(t *testing.T) {
	// Same 130-line switch-dispatch function → no function_size issue.
	result := scoring.ScoreCodeHealth(defaultProfile(), nil, analyzed(
		makeFile("field.go", 200,
			makeSwitchDispatchFunc("Any", 130, 40, 1.5),
		),
	))

	funcIssues := issuesBySubMetric(result.Issues, "function_size")
	assert.Empty(t, funcIssues, "switch-dispatch function under relaxed threshold should produce no issues")
}

func TestScoreCodeHealth_SwitchDispatchStillPenalizedAtExtremeSize(t *testing.T) {
	// Default: threshold=250, zero at 250*(4+1)=1250.
	// A 1300-line switch-dispatch → zero credit.
	result := scoring.ScoreCodeHealth(defaultProfile(), nil, analyzed(
		makeFile("field.go", 1400,
			makeSwitchDispatchFunc("MegaSwitch", 1300, 500, 2.0),
		),
	))

	sm := subMetricByName(result, "function_size")
	require.NotNil(t, sm)
	assert.Equal(t, 0, sm.Score, "even switch-dispatch functions get penalized at extreme sizes")
}

func TestScoreCodeHealth_FewCasesNotRelaxed(t *testing.T) {
	// A 130-line function with only 5 cases → NOT switch-dispatch, normal threshold (50).
	// decay(130, 50, k=4) = 1 - 80/200 = 0.6 → round(12) = 12
	result := scoring.ScoreCodeHealth(defaultProfile(), nil, analyzed(
		makeFile("handler.go", 200,
			makeSwitchDispatchFunc("Handle", 130, 5, 1.5),
		),
	))

	sm := subMetricByName(result, "function_size")
	require.NotNil(t, sm)
	assert.Equal(t, 12, sm.Score, "few cases should NOT qualify as switch-dispatch, uses normal threshold")
}

func TestScoreCodeHealth_ComplexCasesNotRelaxed(t *testing.T) {
	// A 130-line function with 40 cases but avg 8 lines per case → NOT switch-dispatch.
	// decay(130, 50, k=4) = 1 - 80/200 = 0.6 → round(12) = 12
	result := scoring.ScoreCodeHealth(defaultProfile(), nil, analyzed(
		makeFile("handler.go", 200,
			makeSwitchDispatchFunc("Process", 130, 40, 8.0),
		),
	))

	sm := subMetricByName(result, "function_size")
	require.NotNil(t, sm)
	assert.Equal(t, 12, sm.Score, "complex cases should NOT qualify as switch-dispatch, uses normal threshold")
}

// ---------------------------------------------------------------------------
// Cognitive complexity scoring
// ---------------------------------------------------------------------------

func TestScoreCodeHealth_CognitiveComplexityFullCredit(t *testing.T) {
	// CC=0 and CC=25 (at threshold) should both get full credit.
	tests := []struct {
		name string
		cc   int
	}{
		{"CC=0", 0},
		{"CC=25 (at threshold)", 25},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fn := makeFunctionCC("Fn", 20, 2, 1, 0, tt.cc)
			result := scoring.ScoreCodeHealth(defaultProfile(), nil, analyzed(
				makeFile("service.go", 100, fn),
			))
			sm := subMetricByName(result, "cognitive_complexity")
			require.NotNil(t, sm)
			assert.Equal(t, 20, sm.Score)
		})
	}
}

func TestScoreCodeHealth_CognitiveComplexitySeverityTiers(t *testing.T) {
	// MaxCognitiveComplexity=25. Severity thresholds:
	// info: > 25 (1.0x), warning: >= 1.5x = 38, error: >= 3x = 75
	tests := []struct {
		name    string
		cc      int
		wantSev string
	}{
		{"just over → info", 26, domain.SeverityInfo},
		{"1.5x → warning", 38, domain.SeverityWarning},
		{"3x → error", 75, domain.SeverityError},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fn := makeFunctionCC("Fn", 20, 2, 1, 0, tt.cc)
			result := scoring.ScoreCodeHealth(defaultProfile(), nil, analyzed(
				makeFile("service.go", 100, fn),
			))
			ccIssues := issuesBySubMetric(result.Issues, "cognitive_complexity")
			require.Len(t, ccIssues, 1)
			assert.Equal(t, tt.wantSev, ccIssues[0].Severity)
		})
	}
}

func TestScoreCodeHealth_CognitiveComplexitySwitchDispatchExempt(t *testing.T) {
	// Switch dispatch functions with high CC should get full credit.
	fn := makeSwitchDispatchFunc("Any", 130, 40, 1.5)
	fn.CognitiveComplexity = 50 // high CC but exempt
	result := scoring.ScoreCodeHealth(defaultProfile(), nil, analyzed(
		makeFile("field.go", 200, fn),
	))

	sm := subMetricByName(result, "cognitive_complexity")
	require.NotNil(t, sm)
	assert.Equal(t, 20, sm.Score, "switch dispatch should be exempt from CC scoring")

	ccIssues := issuesBySubMetric(result.Issues, "cognitive_complexity")
	assert.Empty(t, ccIssues, "switch dispatch should not produce CC issues")
}

// ---------------------------------------------------------------------------
// Code duplication scoring
// ---------------------------------------------------------------------------

func makeFileWithTokens(path string, totalLines int, tokens []int, fns ...domain.Function) *domain.AnalyzedFile {
	af := makeFile(path, totalLines, fns...)
	af.NormalizedTokens = tokens
	return af
}

func TestScoreCodeHealth_CodeDuplicationNoTokens(t *testing.T) {
	// Files without tokens → full credit on code_duplication.
	result := scoring.ScoreCodeHealth(defaultProfile(), nil, analyzed(
		makeFile("a.go", 100, makeFunction("A", 20, 2, 1, 0)),
		makeFile("b.go", 100, makeFunction("B", 20, 2, 1, 0)),
	))

	sm := subMetricByName(result, "code_duplication")
	require.NotNil(t, sm)
	assert.Equal(t, 20, sm.Score, "files without tokens should get full credit")
}

func TestScoreCodeHealth_CodeDuplicationNoMatch(t *testing.T) {
	// Two files with completely different token sequences → no duplication.
	tokensA := make([]int, 100)
	tokensB := make([]int, 100)
	for i := range tokensA {
		tokensA[i] = i
		tokensB[i] = i + 1000
	}
	result := scoring.ScoreCodeHealth(defaultProfile(), nil, analyzed(
		makeFileWithTokens("a.go", 100, tokensA, makeFunction("A", 20, 2, 1, 0)),
		makeFileWithTokens("b.go", 100, tokensB, makeFunction("B", 20, 2, 1, 0)),
	))

	sm := subMetricByName(result, "code_duplication")
	require.NotNil(t, sm)
	assert.Equal(t, 20, sm.Score, "files with different tokens should get full credit")
}

func TestScoreCodeHealth_CodeDuplicationFullMatch(t *testing.T) {
	// Two files with identical token sequences → duplication detected.
	tokens := make([]int, 100)
	for i := range tokens {
		tokens[i] = i % 10
	}
	result := scoring.ScoreCodeHealth(defaultProfile(), nil, analyzed(
		makeFileWithTokens("a.go", 100, tokens, makeFunction("A", 20, 2, 1, 0)),
		makeFileWithTokens("b.go", 100, tokens, makeFunction("B", 20, 2, 1, 0)),
	))

	sm := subMetricByName(result, "code_duplication")
	require.NotNil(t, sm)
	assert.Less(t, sm.Score, 20, "identical tokens across files should be penalized")
}

func TestScoreCodeHealth_CodeDuplicationIntraFileIgnored(t *testing.T) {
	// A single file with repeated windows should NOT be flagged (intra-file dupes ignored).
	tokens := make([]int, 200)
	for i := range tokens {
		tokens[i] = i % 10 // creates repeated windows within the same file
	}
	result := scoring.ScoreCodeHealth(defaultProfile(), nil, analyzed(
		makeFileWithTokens("a.go", 200, tokens, makeFunction("A", 20, 2, 1, 0)),
	))

	sm := subMetricByName(result, "code_duplication")
	require.NotNil(t, sm)
	assert.Equal(t, 20, sm.Score, "single file should not be penalized for intra-file duplication")
}

func TestScoreCodeHealth_CodeDuplicationGeneratedExcluded(t *testing.T) {
	// Generated files with identical tokens should not affect duplication scoring.
	tokens := make([]int, 100)
	for i := range tokens {
		tokens[i] = i % 10
	}
	genFile := makeFileWithTokens("gen.go", 100, tokens, makeFunction("A", 20, 2, 1, 0))
	genFile.IsGenerated = true

	result := scoring.ScoreCodeHealth(defaultProfile(), nil, analyzed(
		makeFileWithTokens("a.go", 100, tokens, makeFunction("B", 20, 2, 1, 0)),
		genFile,
	))

	sm := subMetricByName(result, "code_duplication")
	require.NotNil(t, sm)
	assert.Equal(t, 20, sm.Score, "generated file duplication should not affect score")
}

func TestScoreCodeHealth_CodeDuplicationIssueGeneration(t *testing.T) {
	// Two files with identical tokens → duplication issue should be generated.
	tokens := make([]int, 100)
	for i := range tokens {
		tokens[i] = i % 10
	}
	result := scoring.ScoreCodeHealth(defaultProfile(), nil, analyzed(
		makeFileWithTokens("a.go", 100, tokens, makeFunction("A", 20, 2, 1, 0)),
		makeFileWithTokens("b.go", 100, tokens, makeFunction("B", 20, 2, 1, 0)),
	))

	dupIssues := issuesBySubMetric(result.Issues, "code_duplication")
	// The files should have duplication detected and issues generated for files above threshold.
	for _, iss := range dupIssues {
		assert.Equal(t, "code_health", iss.Category)
		assert.Equal(t, "code_duplication", iss.SubMetric)
	}
}

func TestScoreCodeHealth_CodeDuplicationOverlappingWindows(t *testing.T) {
	// Two files share a 100-token sequence. The duplication scorer should merge
	// overlapping windows correctly: e.g., windows starting at 0, 1, 2, ... 25
	// all overlap and should cover tokens [0, 100), not 26×75 = 1950 tokens.
	tokens := make([]int, 150)
	for i := range tokens {
		tokens[i] = i % 20
	}
	// File B shares the first 100 tokens with file A but diverges after.
	tokensB := make([]int, 150)
	copy(tokensB, tokens[:100])
	for i := 100; i < 150; i++ {
		tokensB[i] = 99 + i // unique values so no match
	}

	result := scoring.ScoreCodeHealth(defaultProfile(), nil, analyzed(
		makeFileWithTokens("a.go", 150, tokens, makeFunction("A", 20, 2, 1, 0)),
		makeFileWithTokens("b.go", 150, tokensB, makeFunction("B", 20, 2, 1, 0)),
	))

	sm := subMetricByName(result, "code_duplication")
	require.NotNil(t, sm)
	// With correct merging, duplication should be partial (100/150 tokens ≈ 67%).
	// Without merging, it would be massively overcounted.
	// The score should be penalized but not zero.
	assert.Greater(t, sm.Score, 0, "overlapping windows should not over-penalize to zero")
	assert.Less(t, sm.Score, 20, "partial duplication should still be penalized")
}

func TestScoreCodeHealth_CodeDuplicationTestFileRelaxed(t *testing.T) {
	// Test files get 2x the duplication threshold. A test file at 20% duplication
	// should not be penalized when MaxDuplicationPercent=15 (threshold becomes 30%).
	tokens := make([]int, 100)
	for i := range tokens {
		tokens[i] = i % 10
	}
	result := scoring.ScoreCodeHealth(defaultProfile(), nil, analyzed(
		makeFileWithTokens("service.go", 100, tokens, makeFunction("A", 20, 2, 1, 0)),
		makeFileWithTokens("service_test.go", 100, tokens, makeFunction("B", 20, 2, 1, 0)),
	))

	// Both files have identical tokens (100% duplication), but test file uses 2x threshold.
	dupIssues := issuesBySubMetric(result.Issues, "code_duplication")
	testIssues := 0
	srcIssues := 0
	for _, iss := range dupIssues {
		if strings.HasSuffix(iss.File, "_test.go") {
			testIssues++
		} else {
			srcIssues++
		}
	}
	assert.Equal(t, 1, srcIssues, "source file should have duplication issue")
	// Test file also has 100% duplication which exceeds even the 2x threshold (30%),
	// so it should still generate an issue, but at a lower severity.
	assert.Equal(t, 1, testIssues, "test file should also have duplication issue (100% > 30%)")
}
