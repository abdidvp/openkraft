package scoring_test

import (
	"strings"
	"testing"

	"github.com/abdidvp/openkraft/internal/domain"
	"github.com/abdidvp/openkraft/internal/domain/scoring"
	"github.com/stretchr/testify/assert"
)

func TestScorePredictability_NilInputs(t *testing.T) {
	result := scoring.ScorePredictability(defaultProfile(), nil, nil, nil)

	assert.Equal(t, "predictability", result.Name)
	assert.Equal(t, 0.10, result.Weight)
	assert.Len(t, result.SubMetrics, 4)
	assert.GreaterOrEqual(t, result.Score, 0)
	assert.LessOrEqual(t, result.Score, 100)
}

func TestScorePredictability_EmptyInputs(t *testing.T) {
	modules := []domain.DetectedModule{}
	scan := &domain.ScanResult{}
	analyzed := map[string]*domain.AnalyzedFile{}

	result := scoring.ScorePredictability(defaultProfile(), modules, scan, analyzed)

	assert.Equal(t, "predictability", result.Name)
	assert.Equal(t, 0.10, result.Weight)
	assert.Len(t, result.SubMetrics, 4)
}

func TestScorePredictability_CleanCode(t *testing.T) {
	modules := []domain.DetectedModule{
		{
			Name: "user",
			Path: "internal/user",
			Files: []string{
				"internal/user/user_service.go",
				"internal/user/user_handler.go",
			},
		},
	}
	scan := &domain.ScanResult{}
	analyzed := map[string]*domain.AnalyzedFile{
		"internal/user/user_service.go": {
			Path:    "internal/user/user_service.go",
			Package: "user",
			Functions: []domain.Function{
				{
					Name:     "CreateUser",
					Exported: true,
					Receiver: "UserService",
					Params:   []domain.Param{{Name: "ctx", Type: "context.Context"}, {Name: "req", Type: "CreateRequest"}},
					Returns:  []string{"*User", "error"},
				},
				{
					Name:     "DeleteUser",
					Exported: true,
					Receiver: "UserService",
					Params:   []domain.Param{{Name: "ctx", Type: "context.Context"}, {Name: "id", Type: "string"}},
					Returns:  []string{"error"},
				},
			},
			ErrorCalls: []domain.ErrorCall{
				{Type: "fmt.Errorf", HasWrap: true, HasContext: true},
				{Type: "fmt.Errorf", HasWrap: true, HasContext: true},
			},
			GlobalVars: []string{"ErrNotFound"},
		},
	}

	result := scoring.ScorePredictability(defaultProfile(), modules, scan, analyzed)

	assert.Equal(t, "predictability", result.Name)
	assert.Equal(t, 0.10, result.Weight)
	assert.Len(t, result.SubMetrics, 4)
	assert.Greater(t, result.Score, 0)
	assert.LessOrEqual(t, result.Score, 100)

	expectedNames := []string{
		"self_describing_names", "explicit_dependencies",
		"error_message_quality", "consistent_patterns",
	}
	for i, name := range expectedNames {
		assert.Equal(t, name, result.SubMetrics[i].Name)
	}
}

func TestScorePredictability_SubMetricPointsSum(t *testing.T) {
	result := scoring.ScorePredictability(defaultProfile(), nil, nil, nil)

	totalPoints := 0
	for _, sm := range result.SubMetrics {
		totalPoints += sm.Points
	}
	assert.Equal(t, 100, totalPoints, "sub-metric points should sum to 100")
}

func TestScorePredictability_NoErrorCallsGivesZero(t *testing.T) {
	analyzed := map[string]*domain.AnalyzedFile{
		"service.go": {
			Path:       "service.go",
			Package:    "app",
			Functions:  []domain.Function{{Name: "DoWork", Exported: true}},
			ErrorCalls: nil,
		},
	}

	result := scoring.ScorePredictability(defaultProfile(), nil, nil, analyzed)

	errQuality := result.SubMetrics[2]
	assert.Equal(t, "error_message_quality", errQuality.Name)
	assert.Equal(t, 0, errQuality.Score, "no error calls should give zero points")
	assert.Contains(t, errQuality.Detail, "no error handling found")

	found := false
	for _, issue := range result.Issues {
		if issue.Category == "predictability" && strings.Contains(issue.Message, "no error handling") {
			found = true
			assert.Equal(t, domain.SeverityInfo, issue.Severity)
		}
	}
	assert.True(t, found, "expected info issue about no error handling")
}

func TestScorePredictability_MutableStateReducesScore(t *testing.T) {
	analyzed := map[string]*domain.AnalyzedFile{
		"config.go": {
			Path:          "config.go",
			Package:       "app",
			GlobalVars:    []string{"DB", "Logger", "Config", "Cache", "Mutex"},
			InitFunctions: 2,
			Functions:     []domain.Function{{Name: "Setup", Exported: true}},
		},
	}

	result := scoring.ScorePredictability(defaultProfile(), nil, nil, analyzed)

	// explicit_dependencies: 5 exported vars + 2 inits = 7 * 3 (default penalty) = 21 penalty.
	// 25 - 21 = 4.
	explDeps := result.SubMetrics[1]
	assert.Equal(t, "explicit_dependencies", explDeps.Name)
	assert.Equal(t, 4, explDeps.Score)
}

func TestScorePredictability_UnexportedVarsNotPenalized(t *testing.T) {
	analyzed := map[string]*domain.AnalyzedFile{
		"renderer.go": {
			Path:    "renderer.go",
			Package: "tui",
			GlobalVars: []string{
				"accent", "fg", "dim", "faint", "success", "danger",
				"headerStyle", "boxStyle", "dimStyle", "faintStyle",
			},
		},
	}

	result := scoring.ScorePredictability(defaultProfile(), nil, nil, analyzed)

	explDeps := result.SubMetrics[1]
	assert.Equal(t, "explicit_dependencies", explDeps.Name)
	assert.Equal(t, explDeps.Points, explDeps.Score,
		"unexported vars should not reduce score")
}

func TestScorePredictability_SentinelErrorsNotPenalized(t *testing.T) {
	analyzed := map[string]*domain.AnalyzedFile{
		"errors.go": {
			Path:    "errors.go",
			Package: "domain",
			GlobalVars: []string{
				"ErrNotFound",
				"ErrUnauthorized",
				"DB",
			},
		},
	}

	result := scoring.ScorePredictability(defaultProfile(), nil, nil, analyzed)

	explDeps := result.SubMetrics[1]
	assert.Equal(t, "explicit_dependencies", explDeps.Name)
	// Only DB = 1 penalized, 1 * 3 = 3 penalty. 25 - 3 = 22.
	assert.Equal(t, 22, explDeps.Score)
}

func TestScorePredictability_ExcessGlobalVarsGeneratesIssue(t *testing.T) {
	analyzed := map[string]*domain.AnalyzedFile{
		"globals.go": {
			Path:       "globals.go",
			Package:    "app",
			GlobalVars: []string{"a", "b", "c", "d"},
		},
	}

	result := scoring.ScorePredictability(defaultProfile(), nil, nil, analyzed)

	found := false
	for _, issue := range result.Issues {
		if issue.Category == "predictability" && issue.File == "globals.go" {
			found = true
		}
	}
	assert.True(t, found, "expected a predictability issue for excessive global variables")
}

func TestScorePredictability_InitFunctionGeneratesIssue(t *testing.T) {
	analyzed := map[string]*domain.AnalyzedFile{
		"setup.go": {
			Path:          "setup.go",
			Package:       "app",
			InitFunctions: 1,
		},
	}

	result := scoring.ScorePredictability(defaultProfile(), nil, nil, analyzed)

	found := false
	for _, issue := range result.Issues {
		if issue.Category == "predictability" && issue.File == "setup.go" {
			found = true
		}
	}
	assert.True(t, found, "expected a predictability issue for init() function")
}

func TestScorePredictability_CustomGlobalVarPenalty(t *testing.T) {
	p := domain.DefaultProfile()
	p.MaxGlobalVarPenalty = 5 // Harsher penalty

	analyzed := map[string]*domain.AnalyzedFile{
		"config.go": {
			Path:       "config.go",
			Package:    "app",
			GlobalVars: []string{"DB", "Logger"},
			Functions:  []domain.Function{{Name: "Setup", Exported: true}},
		},
	}

	result := scoring.ScorePredictability(&p, nil, nil, analyzed)

	explDeps := result.SubMetrics[1]
	assert.Equal(t, "explicit_dependencies", explDeps.Name)
	// 2 exported vars * 5 penalty = 10. 25 - 10 = 15.
	assert.Equal(t, 15, explDeps.Score)
}
