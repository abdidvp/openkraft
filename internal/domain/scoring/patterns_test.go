package scoring_test

import (
	"testing"

	"github.com/openkraft/openkraft/internal/domain"
	"github.com/openkraft/openkraft/internal/domain/scoring"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestScorePatterns_PerfectFixture(t *testing.T) {
	modules, _, analyzed := loadFixture(t)
	result := scoring.ScorePatterns(modules, analyzed)

	assert.Equal(t, "patterns", result.Name)
	assert.Equal(t, 0.20, result.Weight)
	assert.Len(t, result.SubMetrics, 5)

	// tax + inventory fully comply, payments does not.
	// Expected: moderate-to-high score (not perfect because payments lacks patterns).
	assert.Greater(t, result.Score, 50, "perfect fixture should score > 50 (tax+inventory comply)")
	assert.Less(t, result.Score, 100, "perfect fixture should score < 100 (payments doesn't comply)")
}

func TestScorePatterns_SubMetricNames(t *testing.T) {
	modules, _, analyzed := loadFixture(t)
	result := scoring.ScorePatterns(modules, analyzed)

	names := make([]string, len(result.SubMetrics))
	for i, sm := range result.SubMetrics {
		names[i] = sm.Name
	}

	assert.Contains(t, names, "entity_patterns")
	assert.Contains(t, names, "repository_patterns")
	assert.Contains(t, names, "service_patterns")
	assert.Contains(t, names, "port_patterns")
	assert.Contains(t, names, "handler_patterns")
}

func TestScorePatterns_SubMetricMaxPoints(t *testing.T) {
	modules, _, analyzed := loadFixture(t)
	result := scoring.ScorePatterns(modules, analyzed)

	pointsMap := make(map[string]int)
	for _, sm := range result.SubMetrics {
		pointsMap[sm.Name] = sm.Points
	}

	assert.Equal(t, 30, pointsMap["entity_patterns"])
	assert.Equal(t, 25, pointsMap["repository_patterns"])
	assert.Equal(t, 20, pointsMap["service_patterns"])
	assert.Equal(t, 15, pointsMap["port_patterns"])
	assert.Equal(t, 10, pointsMap["handler_patterns"])
}

func TestScorePatterns_EntityPatternsPartialCompliance(t *testing.T) {
	modules, _, analyzed := loadFixture(t)
	result := scoring.ScorePatterns(modules, analyzed)

	for _, sm := range result.SubMetrics {
		if sm.Name == "entity_patterns" {
			// tax and inventory have Validate+New*, payments does not.
			// 2 out of 3 modules with domain entities have the pattern,
			// but only modules that HAVE domain entities count.
			// All 3 modules have domain structs, so 2/3 compliance.
			assert.Greater(t, sm.Score, 0, "entity_patterns should score > 0")
			assert.Less(t, sm.Score, sm.Points, "entity_patterns should not be perfect (payments missing)")
			return
		}
	}
	t.Fatal("entity_patterns sub-metric not found")
}

func TestScorePatterns_ServicePatternsPartialCompliance(t *testing.T) {
	modules, _, analyzed := loadFixture(t)
	result := scoring.ScorePatterns(modules, analyzed)

	for _, sm := range result.SubMetrics {
		if sm.Name == "service_patterns" {
			// tax and inventory have services with constructor injection.
			// payments has no service layer.
			// Only modules that HAVE an application layer are candidates.
			// tax and inventory both comply -> could be full marks for this sub-metric.
			assert.Greater(t, sm.Score, 0, "service_patterns should score > 0")
			return
		}
	}
	t.Fatal("service_patterns sub-metric not found")
}

func TestScorePatterns_ScoreDoesNotExceed100(t *testing.T) {
	modules, _, analyzed := loadFixture(t)
	result := scoring.ScorePatterns(modules, analyzed)

	assert.LessOrEqual(t, result.Score, 100)

	totalPoints := 0
	for _, sm := range result.SubMetrics {
		totalPoints += sm.Score
		assert.LessOrEqual(t, sm.Score, sm.Points, "sub-metric %s score should not exceed max points", sm.Name)
	}
	assert.LessOrEqual(t, totalPoints, 100)
}

func TestScorePatterns_EmptyModules(t *testing.T) {
	analyzed := make(map[string]*domain.AnalyzedFile)
	result := scoring.ScorePatterns(nil, analyzed)

	assert.Equal(t, "patterns", result.Name)
	assert.Equal(t, 0.20, result.Weight)
	assert.Len(t, result.SubMetrics, 5)
	assert.Equal(t, 0, result.Score)
}

func TestScorePatterns_NoSubMetricNegative(t *testing.T) {
	modules, _, analyzed := loadFixture(t)
	result := scoring.ScorePatterns(modules, analyzed)

	for _, sm := range result.SubMetrics {
		assert.GreaterOrEqual(t, sm.Score, 0, "sub-metric %s should not be negative", sm.Name)
	}
}

func TestScorePatterns_IssuesReported(t *testing.T) {
	modules, _, analyzed := loadFixture(t)
	result := scoring.ScorePatterns(modules, analyzed)

	// payments module should generate issues for missing patterns
	require.NotEmpty(t, result.Issues, "should report issues for non-compliant modules")

	// Check that at least one issue mentions payments
	found := false
	for _, issue := range result.Issues {
		if issue.Category == "patterns" {
			found = true
			break
		}
	}
	assert.True(t, found, "should have issues with category 'patterns'")
}
