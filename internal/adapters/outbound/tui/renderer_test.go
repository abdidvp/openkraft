package tui_test

import (
	"testing"

	"github.com/openkraft/openkraft/internal/adapters/outbound/tui"
	"github.com/openkraft/openkraft/internal/domain"
	"github.com/stretchr/testify/assert"
)

func sampleScore() *domain.Score {
	return &domain.Score{
		Overall: 67,
		Categories: []domain.CategoryScore{
			{Name: "architecture", Score: 80, Weight: 0.25},
			{Name: "tests", Score: 45, Weight: 0.15},
		},
	}
}

func TestRenderScore_ContainsOverall(t *testing.T) {
	output := tui.RenderScore(sampleScore())
	assert.Contains(t, output, "67")
	assert.Contains(t, output, "100")
}

func TestRenderScore_ContainsCategoryNames(t *testing.T) {
	output := tui.RenderScore(sampleScore())
	assert.Contains(t, output, "architecture")
	assert.Contains(t, output, "tests")
}

func TestRenderScore_ContainsGrade(t *testing.T) {
	output := tui.RenderScore(sampleScore())
	assert.Contains(t, output, "C")
}

func TestRenderScore_NonEmpty(t *testing.T) {
	output := tui.RenderScore(sampleScore())
	assert.True(t, len(output) > 50, "rendered output should be substantial")
}

func TestRenderScore_ProgressBars(t *testing.T) {
	output := tui.RenderScore(sampleScore())
	assert.Contains(t, output, "█")
}
