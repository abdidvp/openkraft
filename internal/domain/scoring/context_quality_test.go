package scoring_test

import (
	"testing"

	"github.com/abdidvp/openkraft/internal/domain"
	"github.com/abdidvp/openkraft/internal/domain/scoring"
	"github.com/stretchr/testify/assert"
)

func TestScoreContextQuality_NilInputs(t *testing.T) {
	result := scoring.ScoreContextQuality(defaultProfile(), nil, nil)

	assert.Equal(t, "context_quality", result.Name)
	assert.Equal(t, 0.15, result.Weight)
	assert.Len(t, result.SubMetrics, 4)
	assert.GreaterOrEqual(t, result.Score, 0)
	assert.LessOrEqual(t, result.Score, 100)
}

func TestScoreContextQuality_EmptyInputs(t *testing.T) {
	scan := &domain.ScanResult{}
	analyzed := map[string]*domain.AnalyzedFile{}

	result := scoring.ScoreContextQuality(defaultProfile(), scan, analyzed)

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

	result := scoring.ScoreContextQuality(defaultProfile(), scan, analyzed)

	assert.Equal(t, "context_quality", result.Name)
	assert.Equal(t, 0.15, result.Weight)
	assert.Len(t, result.SubMetrics, 4)

	pkgDoc := result.SubMetrics[1]
	assert.Equal(t, "package_documentation", pkgDoc.Name)
	assert.Equal(t, pkgDoc.Points, pkgDoc.Score)
}

func TestScoreContextQuality_SubMetricPointsSum(t *testing.T) {
	result := scoring.ScoreContextQuality(defaultProfile(), nil, nil)

	totalPoints := 0
	for _, sm := range result.SubMetrics {
		totalPoints += sm.Points
	}
	assert.Equal(t, 100, totalPoints, "sub-metric points should sum to 100")
}

func TestScoreContextQuality_SubMetricNames(t *testing.T) {
	result := scoring.ScoreContextQuality(defaultProfile(), nil, nil)

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

	result := scoring.ScoreContextQuality(defaultProfile(), scan, nil)

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

	result := scoring.ScoreContextQuality(defaultProfile(), scan, nil)

	aiContext := result.SubMetrics[0]
	assert.Equal(t, "ai_context_files", aiContext.Name)
	// Default profile: CLAUDE.md=10, AGENTS.md=8, .cursorrules=7, copilot=5 → total=30 pts
	// CLAUDE.md: min_size=500, size=1000 → 5 (half) + 5 (size met) = 10
	// AGENTS.md: no min_size → full 8
	// .cursorrules: min_size=200, size=500 → 3 (half, 7/2=3) + 4 (size met) = 7
	// copilot: no min_size → full 5
	assert.Equal(t, 30, aiContext.Score)
}

func TestScoreContextQuality_AIContextFilesPartialCredit(t *testing.T) {
	scan := &domain.ScanResult{
		HasClaudeMD:     true,
		HasCursorRules:  true,
		HasAgentsMD:     true,
		ClaudeMDSize:    100, // <500
		AgentsMDSize:    50,
		CursorRulesSize: 50, // <200
	}

	result := scoring.ScoreContextQuality(defaultProfile(), scan, nil)

	aiContext := result.SubMetrics[0]
	assert.Equal(t, "ai_context_files", aiContext.Name)
	// CLAUDE.md: min_size=500, size=100 → 5 (half only, size not met)
	// AGENTS.md: no min_size → full 8
	// .cursorrules: min_size=200, size=50 → 3 (half only, size not met)
	// Total: 5 + 8 + 3 = 16
	assert.Equal(t, 16, aiContext.Score)
}

func TestScoreContextQuality_CustomContextFiles(t *testing.T) {
	p := domain.DefaultProfile()
	p.ContextFiles = []domain.ContextFileSpec{
		{Name: "CLAUDE.md", Points: 20, MinSize: 100},
	}

	scan := &domain.ScanResult{
		HasClaudeMD:  true,
		ClaudeMDSize: 500,
	}

	result := scoring.ScoreContextQuality(&p, scan, nil)

	aiContext := result.SubMetrics[0]
	assert.Equal(t, "ai_context_files", aiContext.Name)
	assert.Equal(t, 20, aiContext.Points, "points should match profile spec")
	// 20 pts, min_size=100, size=500 → 10 (half) + 10 (size met) = 20
	assert.Equal(t, 20, aiContext.Score)
}
