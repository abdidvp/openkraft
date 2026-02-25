package scoring_test

import (
	"testing"

	"github.com/openkraft/openkraft/internal/domain"
	"github.com/openkraft/openkraft/internal/domain/scoring"
	"github.com/stretchr/testify/assert"
)

func TestScoreVerifiability_NilInputs(t *testing.T) {
	// ScanResult is dereferenced directly, so pass an empty one to avoid panic.
	result := scoring.ScoreVerifiability(&domain.ScanResult{}, nil)

	assert.Equal(t, "verifiability", result.Name)
	assert.Equal(t, 0.15, result.Weight)
	assert.Len(t, result.SubMetrics, 4)
	assert.GreaterOrEqual(t, result.Score, 0)
	assert.LessOrEqual(t, result.Score, 100)
}

func TestScoreVerifiability_EmptyInputs(t *testing.T) {
	scan := &domain.ScanResult{}
	analyzed := map[string]*domain.AnalyzedFile{}

	result := scoring.ScoreVerifiability(scan, analyzed)

	assert.Equal(t, "verifiability", result.Name)
	assert.Equal(t, 0.15, result.Weight)
	assert.Len(t, result.SubMetrics, 4)
}

func TestScoreVerifiability_WellTestedProject(t *testing.T) {
	scan := &domain.ScanResult{
		GoFiles: []string{
			"service.go",
			"handler.go",
			"service_test.go",
			"handler_test.go",
		},
		TestFiles: []string{
			"service_test.go",
			"handler_test.go",
		},
		AllFiles: []string{
			"service.go",
			"handler.go",
			"service_test.go",
			"handler_test.go",
			"go.sum",
			"Makefile",
			".golangci.yml",
		},
		HasCIConfig: true,
	}
	analyzed := map[string]*domain.AnalyzedFile{
		"service_test.go": {
			Path:    "service_test.go",
			Package: "app_test",
			Functions: []domain.Function{
				{Name: "TestCreateUser_Success", Exported: true},
				{Name: "TestCreateUser_InvalidInput", Exported: true},
			},
		},
		"handler_test.go": {
			Path:    "handler_test.go",
			Package: "app_test",
			Functions: []domain.Function{
				{Name: "TestHandleRequest_OK", Exported: true},
			},
		},
		"service.go": {
			Path:    "service.go",
			Package: "app",
			Functions: []domain.Function{
				{Name: "CreateUser", Exported: true, Params: []domain.Param{{Name: "name", Type: "string"}}},
			},
		},
	}

	result := scoring.ScoreVerifiability(scan, analyzed)

	assert.Equal(t, "verifiability", result.Name)
	assert.Equal(t, 0.15, result.Weight)
	assert.Len(t, result.SubMetrics, 4)
	assert.Greater(t, result.Score, 0)
	assert.LessOrEqual(t, result.Score, 100)

	expectedNames := []string{
		"test_presence", "test_naming",
		"build_reproducibility", "type_safety_signals",
	}
	for i, name := range expectedNames {
		assert.Equal(t, name, result.SubMetrics[i].Name)
	}
}

func TestScoreVerifiability_SubMetricPointsSum(t *testing.T) {
	result := scoring.ScoreVerifiability(&domain.ScanResult{}, nil)

	totalPoints := 0
	for _, sm := range result.SubMetrics {
		totalPoints += sm.Points
	}
	assert.Equal(t, 100, totalPoints, "sub-metric points should sum to 100")
}

func TestScoreVerifiability_NoTestsGeneratesIssue(t *testing.T) {
	scan := &domain.ScanResult{
		GoFiles: []string{"main.go"},
	}

	result := scoring.ScoreVerifiability(scan, nil)

	// test_presence should be zero, triggering an issue.
	assert.Equal(t, 0, result.SubMetrics[0].Score)
	assert.NotEmpty(t, result.Issues)
}

func TestScoreVerifiability_BuildReproducibilitySignals(t *testing.T) {
	scan := &domain.ScanResult{
		GoFiles: []string{"main.go"},
		AllFiles: []string{
			"main.go",
			"go.sum",
			"Makefile",
		},
		HasCIConfig: true,
	}

	result := scoring.ScoreVerifiability(scan, nil)

	buildRepro := result.SubMetrics[2]
	assert.Equal(t, "build_reproducibility", buildRepro.Name)
	assert.Equal(t, 25, buildRepro.Score) // 10 + 8 + 7 = 25
}
