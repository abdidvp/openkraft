package application_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/openkraft/openkraft/internal/adapters/outbound/config"
	"github.com/openkraft/openkraft/internal/adapters/outbound/detector"
	"github.com/openkraft/openkraft/internal/adapters/outbound/parser"
	"github.com/openkraft/openkraft/internal/adapters/outbound/scanner"
	"github.com/openkraft/openkraft/internal/application"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const fixtureDir = "../../testdata/go-hexagonal/perfect"

func TestScoreService_ScoreProject(t *testing.T) {
	svc := application.NewScoreService(
		scanner.New(),
		detector.New(),
		parser.New(),
		config.New(),
	)

	score, err := svc.ScoreProject(fixtureDir)
	require.NoError(t, err)

	assert.True(t, score.Overall > 0, "overall score should be positive")
	assert.True(t, score.Overall <= 100, "overall score should not exceed 100")
	assert.Len(t, score.Categories, 6, "should have 6 categories")
}

func TestScoreService_CategoriesHaveCorrectWeights(t *testing.T) {
	svc := application.NewScoreService(
		scanner.New(),
		detector.New(),
		parser.New(),
		config.New(),
	)

	score, err := svc.ScoreProject(fixtureDir)
	require.NoError(t, err)

	weightMap := make(map[string]float64)
	for _, c := range score.Categories {
		weightMap[c.Name] = c.Weight
	}

	assert.Equal(t, 0.25, weightMap["architecture"])
	assert.Equal(t, 0.20, weightMap["conventions"])
	assert.Equal(t, 0.20, weightMap["patterns"])
	assert.Equal(t, 0.15, weightMap["tests"])
	assert.Equal(t, 0.10, weightMap["ai_context"])
	assert.Equal(t, 0.10, weightMap["completeness"])
}

func TestScoreService_Deterministic(t *testing.T) {
	svc := application.NewScoreService(
		scanner.New(),
		detector.New(),
		parser.New(),
		config.New(),
	)

	score1, err := svc.ScoreProject(fixtureDir)
	require.NoError(t, err)

	score2, err := svc.ScoreProject(fixtureDir)
	require.NoError(t, err)

	assert.Equal(t, score1.Overall, score2.Overall, "scoring should be deterministic")
}

func TestScoreService_InvalidPath(t *testing.T) {
	svc := application.NewScoreService(
		scanner.New(),
		detector.New(),
		parser.New(),
		config.New(),
	)

	_, err := svc.ScoreProject("/nonexistent/path")
	assert.Error(t, err)
}

// --- Config-aware tests ---

func TestScoreService_CLIConfigSkipsHandlerPatterns(t *testing.T) {
	// Create a temp dir with a .openkraft.yaml for cli-tool type
	tmpDir := t.TempDir()
	cfgContent := `project_type: cli-tool
`
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, ".openkraft.yaml"), []byte(cfgContent), 0644))

	// Copy the fixture's structure isn't needed — we test against the real fixture
	// but the config comes from tmpDir. We need config loaded from the project path.
	// So let's write the config into the fixture dir temporarily.
	cfgPath := filepath.Join(fixtureDir, ".openkraft.yaml")
	require.NoError(t, os.WriteFile(cfgPath, []byte(cfgContent), 0644))
	defer os.Remove(cfgPath)

	svc := application.NewScoreService(
		scanner.New(),
		detector.New(),
		parser.New(),
		config.New(),
	)

	score, err := svc.ScoreProject(fixtureDir)
	require.NoError(t, err)

	// CLI config should skip handler_patterns and repository_patterns
	for _, cat := range score.Categories {
		if cat.Name == "patterns" {
			for _, sm := range cat.SubMetrics {
				if sm.Name == "handler_patterns" || sm.Name == "repository_patterns" {
					assert.True(t, sm.Skipped, "%s should be skipped for cli-tool", sm.Name)
				}
			}
		}
	}

	// Applied config should be present
	assert.NotNil(t, score.AppliedConfig)
	assert.Equal(t, "cli-tool", string(score.AppliedConfig.ProjectType))
}

func TestScoreService_CustomWeightsApplied(t *testing.T) {
	cfgContent := `weights:
  tests: 0.50
`
	cfgPath := filepath.Join(fixtureDir, ".openkraft.yaml")
	require.NoError(t, os.WriteFile(cfgPath, []byte(cfgContent), 0644))
	defer os.Remove(cfgPath)

	svc := application.NewScoreService(
		scanner.New(),
		detector.New(),
		parser.New(),
		config.New(),
	)

	score, err := svc.ScoreProject(fixtureDir)
	require.NoError(t, err)

	for _, cat := range score.Categories {
		if cat.Name == "tests" {
			assert.InDelta(t, 0.50, cat.Weight, 0.001, "tests weight should be overridden")
		}
	}
}

func TestScoreService_SkippedCategoryExcluded(t *testing.T) {
	cfgContent := `skip:
  categories:
    - ai_context
`
	cfgPath := filepath.Join(fixtureDir, ".openkraft.yaml")
	require.NoError(t, os.WriteFile(cfgPath, []byte(cfgContent), 0644))
	defer os.Remove(cfgPath)

	svc := application.NewScoreService(
		scanner.New(),
		detector.New(),
		parser.New(),
		config.New(),
	)

	score, err := svc.ScoreProject(fixtureDir)
	require.NoError(t, err)

	assert.Len(t, score.Categories, 5, "should have 5 categories when ai_context is skipped")
	for _, cat := range score.Categories {
		assert.NotEqual(t, "ai_context", cat.Name, "ai_context should be excluded")
	}
}

func TestScoreService_DefaultConfig_NoAppliedConfig(t *testing.T) {
	svc := application.NewScoreService(
		scanner.New(),
		detector.New(),
		parser.New(),
		config.New(),
	)

	score, err := svc.ScoreProject(fixtureDir)
	require.NoError(t, err)

	// No .openkraft.yaml in fixture = default config = nil AppliedConfig
	assert.Nil(t, score.AppliedConfig, "should not include AppliedConfig for default config")
}
