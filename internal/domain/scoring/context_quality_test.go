package scoring_test

import (
	"testing"

	"github.com/openkraft/openkraft/internal/domain"
	"github.com/openkraft/openkraft/internal/domain/scoring"
	"github.com/stretchr/testify/assert"
)

func TestScoreContextQuality_NilInputs(t *testing.T) {
	result := scoring.ScoreContextQuality(nil, nil)

	assert.Equal(t, "context_quality", result.Name)
	assert.Equal(t, 0.15, result.Weight)
	assert.Len(t, result.SubMetrics, 4)
	assert.GreaterOrEqual(t, result.Score, 0)
	assert.LessOrEqual(t, result.Score, 100)
}

func TestScoreContextQuality_EmptyInputs(t *testing.T) {
	scan := &domain.ScanResult{}
	analyzed := map[string]*domain.AnalyzedFile{}

	result := scoring.ScoreContextQuality(scan, analyzed)

	assert.Equal(t, "context_quality", result.Name)
	assert.Equal(t, 0.15, result.Weight)
	assert.Len(t, result.SubMetrics, 4)
	assert.Equal(t, 0, result.Score)
}

func TestScoreContextQuality_WithDocumentedPackages(t *testing.T) {
	scan := &domain.ScanResult{}
	analyzed := map[string]*domain.AnalyzedFile{
		"service.go": {
			Path:       "service.go",
			Package:    "app",
			PackageDoc: true,
		},
		"handler.go": {
			Path:       "handler.go",
			Package:    "http",
			PackageDoc: true,
		},
	}

	result := scoring.ScoreContextQuality(scan, analyzed)

	assert.Equal(t, "context_quality", result.Name)
	assert.Equal(t, 0.15, result.Weight)
	assert.Len(t, result.SubMetrics, 4)

	// package_documentation should score well since all packages are documented.
	pkgDoc := result.SubMetrics[1]
	assert.Equal(t, "package_documentation", pkgDoc.Name)
	assert.Equal(t, pkgDoc.Points, pkgDoc.Score)
}

func TestScoreContextQuality_SubMetricPointsSum(t *testing.T) {
	result := scoring.ScoreContextQuality(nil, nil)

	totalPoints := 0
	for _, sm := range result.SubMetrics {
		totalPoints += sm.Points
	}
	assert.Equal(t, 100, totalPoints, "sub-metric points should sum to 100")
}

func TestScoreContextQuality_SubMetricNames(t *testing.T) {
	result := scoring.ScoreContextQuality(nil, nil)

	expectedNames := []string{
		"ai_context_files", "package_documentation",
		"architecture_docs", "canonical_examples",
	}
	for i, name := range expectedNames {
		assert.Equal(t, name, result.SubMetrics[i].Name)
	}
}

func TestScoreContextQuality_MissingContextFilesGeneratesIssues(t *testing.T) {
	scan := &domain.ScanResult{
		HasClaudeMD:    false,
		HasCursorRules: false,
		HasAgentsMD:    false,
	}

	result := scoring.ScoreContextQuality(scan, nil)

	// Should generate issues for missing CLAUDE.md, .cursorrules, AGENTS.md.
	assert.GreaterOrEqual(t, len(result.Issues), 3)

	categories := make(map[string]bool)
	for _, issue := range result.Issues {
		categories[issue.Category] = true
	}
	assert.True(t, categories["context_quality"])
}

func TestScoreContextQuality_AIContextFilesFlags(t *testing.T) {
	scan := &domain.ScanResult{
		HasClaudeMD:            true,
		HasCursorRules:         true,
		HasAgentsMD:            true,
		HasCopilotInstructions: true,
		ClaudeMDSize:           1000,
		ClaudeMDContent:        "see `internal/domain` for example patterns",
		AgentsMDSize:           200,
		CursorRulesSize:        500,
	}

	result := scoring.ScoreContextQuality(scan, nil)

	aiContext := result.SubMetrics[0]
	assert.Equal(t, "ai_context_files", aiContext.Name)
	// 10 (CLAUDE.md) + 8 (AGENTS.md) + 7 (.cursorrules) + 5 (copilot) = 30.
	assert.Equal(t, 30, aiContext.Score)
}

func TestScoreContextQuality_AIContextFilesPartialCredit(t *testing.T) {
	// Small files: existence points only, no size bonus.
	scan := &domain.ScanResult{
		HasClaudeMD:     true,
		HasCursorRules:  true,
		HasAgentsMD:     true,
		ClaudeMDSize:    100, // <500
		AgentsMDSize:    50,
		CursorRulesSize: 50, // <200
	}

	result := scoring.ScoreContextQuality(scan, nil)

	aiContext := result.SubMetrics[0]
	assert.Equal(t, "ai_context_files", aiContext.Name)
	// 5 (CLAUDE.md exists) + 4+4 (AGENTS.md exists+non-empty) + 3 (.cursorrules exists) = 16.
	assert.Equal(t, 16, aiContext.Score)
}
