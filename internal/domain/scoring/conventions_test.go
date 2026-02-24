package scoring_test

import (
	"testing"

	"github.com/openkraft/openkraft/internal/domain"
	"github.com/openkraft/openkraft/internal/domain/scoring"
	"github.com/stretchr/testify/assert"
)

func TestScoreConventions_PerfectFixture(t *testing.T) {
	_, scan, analyzed := loadFixture(t)
	result := scoring.ScoreConventions(scan, analyzed)

	assert.Equal(t, "conventions", result.Name)
	assert.Equal(t, 0.20, result.Weight)
	assert.Len(t, result.SubMetrics, 5)
	assert.Greater(t, result.Score, 60, "perfect fixture should score > 60")
}

func TestScoreConventions_SubMetricNames(t *testing.T) {
	_, scan, analyzed := loadFixture(t)
	result := scoring.ScoreConventions(scan, analyzed)

	names := make([]string, len(result.SubMetrics))
	for i, sm := range result.SubMetrics {
		names[i] = sm.Name
	}

	assert.Contains(t, names, "naming_consistency")
	assert.Contains(t, names, "error_handling")
	assert.Contains(t, names, "import_ordering")
	assert.Contains(t, names, "file_organization")
	assert.Contains(t, names, "code_style")
}

func TestScoreConventions_SubMetricMaxPoints(t *testing.T) {
	_, scan, analyzed := loadFixture(t)
	result := scoring.ScoreConventions(scan, analyzed)

	pointsMap := make(map[string]int)
	for _, sm := range result.SubMetrics {
		pointsMap[sm.Name] = sm.Points
	}

	assert.Equal(t, 30, pointsMap["naming_consistency"])
	assert.Equal(t, 25, pointsMap["error_handling"])
	assert.Equal(t, 15, pointsMap["import_ordering"])
	assert.Equal(t, 15, pointsMap["file_organization"])
	assert.Equal(t, 15, pointsMap["code_style"])
}

func TestScoreConventions_ScoreDoesNotExceed100(t *testing.T) {
	_, scan, analyzed := loadFixture(t)
	result := scoring.ScoreConventions(scan, analyzed)

	assert.LessOrEqual(t, result.Score, 100)

	totalPoints := 0
	for _, sm := range result.SubMetrics {
		totalPoints += sm.Score
		assert.LessOrEqual(t, sm.Score, sm.Points, "sub-metric %s score should not exceed max points", sm.Name)
	}
	assert.LessOrEqual(t, totalPoints, 100)
}

func TestScoreConventions_NamingConsistency(t *testing.T) {
	_, scan, analyzed := loadFixture(t)
	result := scoring.ScoreConventions(scan, analyzed)

	for _, sm := range result.SubMetrics {
		if sm.Name == "naming_consistency" {
			assert.Greater(t, sm.Score, 0, "perfect fixture uses snake_case filenames and PascalCase structs")
			return
		}
	}
	t.Fatal("naming_consistency sub-metric not found")
}

func TestScoreConventions_ErrorHandling(t *testing.T) {
	_, scan, analyzed := loadFixture(t)
	result := scoring.ScoreConventions(scan, analyzed)

	for _, sm := range result.SubMetrics {
		if sm.Name == "error_handling" {
			assert.Greater(t, sm.Score, 0, "perfect fixture uses Err prefix error variables")
			return
		}
	}
	t.Fatal("error_handling sub-metric not found")
}

func TestScoreConventions_CodeStyle(t *testing.T) {
	_, scan, analyzed := loadFixture(t)
	result := scoring.ScoreConventions(scan, analyzed)

	for _, sm := range result.SubMetrics {
		if sm.Name == "code_style" {
			assert.Greater(t, sm.Score, 0, "perfect fixture uses New{Type} constructors")
			return
		}
	}
	t.Fatal("code_style sub-metric not found")
}

func TestScoreConventions_EmptyInput(t *testing.T) {
	scan := &domain.ScanResult{}
	analyzed := make(map[string]*domain.AnalyzedFile)

	result := scoring.ScoreConventions(scan, analyzed)

	assert.Equal(t, "conventions", result.Name)
	assert.Equal(t, 0.20, result.Weight)
	assert.Len(t, result.SubMetrics, 5)
	assert.LessOrEqual(t, result.Score, 10, "empty project should score very low")
}
