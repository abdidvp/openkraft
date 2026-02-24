package application_test

import (
	"testing"

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
	)

	score, err := svc.ScoreProject(fixtureDir)
	require.NoError(t, err)

	assert.True(t, score.Overall > 0, "overall score should be positive")
	assert.True(t, score.Overall <= 100, "overall score should not exceed 100")
	assert.Len(t, score.Categories, 2, "should have 2 categories (architecture + tests)")
}

func TestScoreService_CategoriesHaveCorrectWeights(t *testing.T) {
	svc := application.NewScoreService(
		scanner.New(),
		detector.New(),
		parser.New(),
	)

	score, err := svc.ScoreProject(fixtureDir)
	require.NoError(t, err)

	weightMap := make(map[string]float64)
	for _, c := range score.Categories {
		weightMap[c.Name] = c.Weight
	}

	assert.Equal(t, 0.25, weightMap["architecture"])
	assert.Equal(t, 0.15, weightMap["tests"])
}

func TestScoreService_Deterministic(t *testing.T) {
	svc := application.NewScoreService(
		scanner.New(),
		detector.New(),
		parser.New(),
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
	)

	_, err := svc.ScoreProject("/nonexistent/path")
	assert.Error(t, err)
}
