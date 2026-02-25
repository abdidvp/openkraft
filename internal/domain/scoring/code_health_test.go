package scoring_test

import (
	"testing"

	"github.com/openkraft/openkraft/internal/domain"
	"github.com/openkraft/openkraft/internal/domain/scoring"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestScoreCodeHealth_NilInputs(t *testing.T) {
	result := scoring.ScoreCodeHealth(nil, nil)

	assert.Equal(t, "code_health", result.Name)
	assert.Equal(t, 0.25, result.Weight)
	assert.Len(t, result.SubMetrics, 5)
	assert.GreaterOrEqual(t, result.Score, 0)
	assert.LessOrEqual(t, result.Score, 100)
}

func TestScoreCodeHealth_EmptyInputs(t *testing.T) {
	scan := &domain.ScanResult{}
	analyzed := map[string]*domain.AnalyzedFile{}

	result := scoring.ScoreCodeHealth(scan, analyzed)

	assert.Equal(t, "code_health", result.Name)
	assert.Equal(t, 0.25, result.Weight)
	assert.Len(t, result.SubMetrics, 5)
	assert.Equal(t, 0, result.Score)
}

func TestScoreCodeHealth_WellStructuredCode(t *testing.T) {
	scan := &domain.ScanResult{
		GoFiles: []string{"service.go"},
	}
	analyzed := map[string]*domain.AnalyzedFile{
		"service.go": {
			Path:       "service.go",
			Package:    "app",
			TotalLines: 100,
			Functions: []domain.Function{
				{
					Name:       "CreateUser",
					Exported:   true,
					LineStart:  10,
					LineEnd:    30,
					Params:     []domain.Param{{Name: "ctx", Type: "context.Context"}, {Name: "name", Type: "string"}},
					MaxNesting: 2,
					MaxCondOps: 1,
				},
				{
					Name:       "DeleteUser",
					Exported:   true,
					LineStart:  35,
					LineEnd:    55,
					Params:     []domain.Param{{Name: "ctx", Type: "context.Context"}, {Name: "id", Type: "string"}},
					MaxNesting: 1,
					MaxCondOps: 0,
				},
			},
		},
	}

	result := scoring.ScoreCodeHealth(scan, analyzed)

	assert.Equal(t, "code_health", result.Name)
	assert.Equal(t, 0.25, result.Weight)
	assert.Len(t, result.SubMetrics, 5)
	assert.Greater(t, result.Score, 0)
	assert.LessOrEqual(t, result.Score, 100)

	// Verify sub-metric names.
	expectedNames := []string{
		"function_size", "file_size", "nesting_depth",
		"parameter_count", "complex_conditionals",
	}
	for i, name := range expectedNames {
		assert.Equal(t, name, result.SubMetrics[i].Name)
	}
}

func TestScoreCodeHealth_SubMetricPointsSum(t *testing.T) {
	totalPoints := 0
	analyzed := map[string]*domain.AnalyzedFile{
		"a.go": {
			Path:       "a.go",
			TotalLines: 50,
			Functions: []domain.Function{
				{Name: "Foo", LineStart: 1, LineEnd: 10, MaxNesting: 1, MaxCondOps: 0},
			},
		},
	}

	result := scoring.ScoreCodeHealth(nil, analyzed)

	for _, sm := range result.SubMetrics {
		totalPoints += sm.Points
	}
	assert.Equal(t, 100, totalPoints, "sub-metric points should sum to 100")
}

func TestScoreCodeHealth_LargeFunctionsReduceScore(t *testing.T) {
	analyzed := map[string]*domain.AnalyzedFile{
		"big.go": {
			Path:       "big.go",
			TotalLines: 600,
			Functions: []domain.Function{
				{Name: "Huge", LineStart: 1, LineEnd: 200, Params: make([]domain.Param, 8), MaxNesting: 6, MaxCondOps: 5},
			},
		},
	}

	result := scoring.ScoreCodeHealth(nil, analyzed)

	// With a 200-line function, deep nesting, many params, and complex conditionals,
	// scores for those sub-metrics should be zero or low.
	require.Len(t, result.SubMetrics, 5)

	// function_size should be 0 (>100 lines).
	assert.Equal(t, 0, result.SubMetrics[0].Score)
	// nesting_depth should be 0 (>=5).
	assert.Equal(t, 0, result.SubMetrics[2].Score)
	// parameter_count should be 0 (>=7).
	assert.Equal(t, 0, result.SubMetrics[3].Score)
	// complex_conditionals should be 0 (>=4).
	assert.Equal(t, 0, result.SubMetrics[4].Score)

	// Issues should be generated for oversized function, nesting, and params.
	assert.NotEmpty(t, result.Issues)
}
