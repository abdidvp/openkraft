package scoring_test

import (
	"testing"

	"github.com/openkraft/openkraft/internal/domain"
	"github.com/openkraft/openkraft/internal/domain/scoring"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const completenessFixtureDir = "../../../testdata/go-hexagonal/perfect"

func TestScoreCompleteness_PerfectFixture(t *testing.T) {
	modules, _, analyzed := loadFixture(t)

	// Fixture modules (sorted by detector): inventory, payments, tax
	// tax: 8 files, 4 layers (adapters/http, adapters/repository, application, domain)
	// inventory: 7 files, 4 layers (adapters/http, adapters/repository, application, domain)
	// payments: 1 file, 1 layer (domain)
	// Proto-golden = tax (most files + layers = 8+4=12)
	// payments drags score down significantly => expect 40-70 overall.
	require.GreaterOrEqual(t, len(modules), 3, "fixture should have at least 3 modules")

	result := scoring.ScoreCompleteness(modules, analyzed)

	assert.Equal(t, "completeness", result.Name)
	assert.Equal(t, 0.10, result.Weight)
	assert.Len(t, result.SubMetrics, 3)

	// Payments (1 file, 1 layer) drags score down significantly.
	assert.GreaterOrEqual(t, result.Score, 40, "score should be >= 40")
	assert.LessOrEqual(t, result.Score, 70, "score should be <= 70")
}

func TestScoreCompleteness_SubMetricNames(t *testing.T) {
	modules, _, analyzed := loadFixture(t)
	result := scoring.ScoreCompleteness(modules, analyzed)

	names := make([]string, len(result.SubMetrics))
	for i, sm := range result.SubMetrics {
		names[i] = sm.Name
	}

	assert.Contains(t, names, "file_completeness")
	assert.Contains(t, names, "structural_completeness")
	assert.Contains(t, names, "documentation_completeness")
}

func TestScoreCompleteness_SubMetricMaxPoints(t *testing.T) {
	modules, _, analyzed := loadFixture(t)
	result := scoring.ScoreCompleteness(modules, analyzed)

	pointsMap := make(map[string]int)
	for _, sm := range result.SubMetrics {
		pointsMap[sm.Name] = sm.Points
	}

	assert.Equal(t, 40, pointsMap["file_completeness"])
	assert.Equal(t, 30, pointsMap["structural_completeness"])
	assert.Equal(t, 30, pointsMap["documentation_completeness"])
}

func TestScoreCompleteness_SingleModule(t *testing.T) {
	modules := []domain.DetectedModule{
		{Name: "orders", Path: "internal/orders", Layers: []string{"domain", "application"}, Files: []string{"a.go", "b.go"}},
	}
	analyzed := make(map[string]*domain.AnalyzedFile)

	result := scoring.ScoreCompleteness(modules, analyzed)

	assert.Equal(t, 100, result.Score, "single module should score 100 (nothing to compare)")
}

func TestScoreCompleteness_EmptyModules(t *testing.T) {
	analyzed := make(map[string]*domain.AnalyzedFile)

	result := scoring.ScoreCompleteness(nil, analyzed)

	assert.Equal(t, "completeness", result.Name)
	assert.Equal(t, 0.10, result.Weight)
	assert.Equal(t, 0, result.Score)
}

func TestScoreCompleteness_TwoEqualModules(t *testing.T) {
	modules := []domain.DetectedModule{
		{Name: "a", Path: "internal/a", Layers: []string{"domain", "application"}, Files: []string{"a1.go", "a2.go", "a3.go"}},
		{Name: "b", Path: "internal/b", Layers: []string{"domain", "application"}, Files: []string{"b1.go", "b2.go", "b3.go"}},
	}
	analyzed := make(map[string]*domain.AnalyzedFile)

	result := scoring.ScoreCompleteness(modules, analyzed)

	// Equal modules: non-golden has same files and layers as golden => 100%
	assert.GreaterOrEqual(t, result.Score, 90, "two equal modules should score near 100")
}

func TestScoreCompleteness_ScoreDoesNotExceed100(t *testing.T) {
	modules, _, analyzed := loadFixture(t)
	result := scoring.ScoreCompleteness(modules, analyzed)

	assert.LessOrEqual(t, result.Score, 100)

	totalPoints := 0
	for _, sm := range result.SubMetrics {
		totalPoints += sm.Score
		assert.LessOrEqual(t, sm.Score, sm.Points, "sub-metric %s score should not exceed max points", sm.Name)
	}
	assert.LessOrEqual(t, totalPoints, 100)
}

func TestScoreCompleteness_PointsSumTo100(t *testing.T) {
	modules, _, analyzed := loadFixture(t)
	result := scoring.ScoreCompleteness(modules, analyzed)

	totalPoints := 0
	for _, sm := range result.SubMetrics {
		totalPoints += sm.Points
	}
	assert.Equal(t, 100, totalPoints)
}
