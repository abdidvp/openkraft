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

	assert.Equal(t, 0.25, weightMap["code_health"])
	assert.Equal(t, 0.20, weightMap["discoverability"])
	assert.Equal(t, 0.15, weightMap["structure"])
	assert.Equal(t, 0.15, weightMap["verifiability"])
	assert.Equal(t, 0.15, weightMap["context_quality"])
	assert.Equal(t, 0.10, weightMap["predictability"])
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

func TestScoreService_CLIConfigSkipsSubMetrics(t *testing.T) {
	cfgContent := `project_type: cli-tool
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

	// CLI config should skip interface_contracts and module_completeness
	for _, cat := range score.Categories {
		if cat.Name == "structure" {
			for _, sm := range cat.SubMetrics {
				if sm.Name == "interface_contracts" || sm.Name == "module_completeness" {
					assert.True(t, sm.Skipped, "%s should be skipped for cli-tool", sm.Name)
				}
			}
		}
	}

	assert.NotNil(t, score.AppliedConfig)
	assert.Equal(t, "cli-tool", string(score.AppliedConfig.ProjectType))
}

func TestScoreService_CustomWeightsApplied(t *testing.T) {
	cfgContent := `weights:
  verifiability: 0.50
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
		if cat.Name == "verifiability" {
			assert.InDelta(t, 0.50, cat.Weight, 0.001, "verifiability weight should be overridden")
		}
	}
}

func TestScoreService_SkippedCategoryExcluded(t *testing.T) {
	cfgContent := `skip:
  categories:
    - context_quality
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

	assert.Len(t, score.Categories, 5, "should have 5 categories when context_quality is skipped")
	for _, cat := range score.Categories {
		assert.NotEqual(t, "context_quality", cat.Name, "context_quality should be excluded")
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

	assert.Nil(t, score.AppliedConfig, "should not include AppliedConfig for default config")
}
