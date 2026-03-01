package domain_test

import (
	"testing"

	"github.com/abdidvp/openkraft/internal/domain"
	"github.com/stretchr/testify/assert"
)

func TestScore_Grade(t *testing.T) {
	tests := []struct {
		score int
		grade string
	}{
		{95, "A+"}, {85, "A"}, {75, "B"}, {65, "C"}, {55, "D"}, {45, "F"}, {0, "F"}, {100, "A+"},
	}
	for _, tt := range tests {
		s := domain.Score{Overall: tt.score}
		assert.Equal(t, tt.grade, s.Grade(), "score %d", tt.score)
	}
}

func TestComputeOverallScore(t *testing.T) {
	categories := []domain.CategoryScore{
		{Name: "architecture", Score: 80, Weight: 0.25},
		{Name: "conventions", Score: 60, Weight: 0.20},
		{Name: "patterns", Score: 40, Weight: 0.20},
		{Name: "tests", Score: 70, Weight: 0.15},
		{Name: "ai_context", Score: 20, Weight: 0.10},
		{Name: "completeness", Score: 50, Weight: 0.10},
	}
	score := domain.ComputeOverallScore(categories)
	assert.Equal(t, 58, score)
}

func TestComputeOverallScore_Empty(t *testing.T) {
	score := domain.ComputeOverallScore(nil)
	assert.Equal(t, 0, score)
}

func TestGradeFor(t *testing.T) {
	assert.Equal(t, "A+", domain.GradeFor(92))
	assert.Equal(t, "F", domain.GradeFor(10))
}

func TestBadgeColor(t *testing.T) {
	assert.Equal(t, "brightgreen", domain.BadgeColor(95))
	assert.Equal(t, "critical", domain.BadgeColor(30))
}
