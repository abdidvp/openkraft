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
			{
				Name: "architecture", Score: 80, Weight: 0.25,
				SubMetrics: []domain.SubMetric{
					{Name: "layer_separation", Score: 20, Points: 25, Detail: "clean layers"},
					{Name: "dependency_direction", Score: 5, Points: 20, Detail: "3 violations"},
				},
				Issues: []domain.Issue{
					{Severity: "error", Category: "architecture", File: "internal/domain/foo_test.go", Message: "domain imports adapter"},
				},
			},
			{
				Name: "tests", Score: 45, Weight: 0.15,
				SubMetrics: []domain.SubMetric{
					{Name: "unit_test_presence", Score: 25, Points: 25, Detail: "good coverage"},
					{Name: "test_helpers", Score: 0, Points: 15, Detail: "none found"},
					{Name: "handler_patterns", Score: 0, Points: 10, Skipped: true},
				},
				Issues: []domain.Issue{
					{Severity: "warning", Category: "tests", Message: "missing test helpers"},
				},
			},
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

func TestRenderScore_ContainsSubMetrics(t *testing.T) {
	output := tui.RenderScore(sampleScore())
	assert.Contains(t, output, "layer_separation")
	assert.Contains(t, output, "dependency_direction")
	assert.Contains(t, output, "unit_test_presence")
	assert.Contains(t, output, "test_helpers")
}

func TestRenderScore_ShowsSubMetricDetails(t *testing.T) {
	output := tui.RenderScore(sampleScore())
	assert.Contains(t, output, "clean layers")
	assert.Contains(t, output, "3 violations")
	assert.Contains(t, output, "none found")
}

func TestRenderScore_ShowsSkippedSubMetrics(t *testing.T) {
	output := tui.RenderScore(sampleScore())
	assert.Contains(t, output, "handler_patterns")
	assert.Contains(t, output, "skipped")
}

func TestRenderScore_ShowsIssues(t *testing.T) {
	output := tui.RenderScore(sampleScore())
	assert.Contains(t, output, "domain imports adapter")
	assert.Contains(t, output, "missing test helpers")
	assert.Contains(t, output, "Issues")
}

func TestRenderScore_ShowsIssueSeverityTags(t *testing.T) {
	output := tui.RenderScore(sampleScore())
	assert.Contains(t, output, "error")
	assert.Contains(t, output, "warn")
}

func TestRenderScore_ShowsIssueFile(t *testing.T) {
	output := tui.RenderScore(sampleScore())
	assert.Contains(t, output, "internal/domain/foo_test.go")
}

func TestRenderScore_ErrorsBeforeWarnings(t *testing.T) {
	output := tui.RenderScore(sampleScore())
	errorIdx := indexOf(output, "domain imports adapter")
	warnIdx := indexOf(output, "missing test helpers")
	assert.True(t, errorIdx < warnIdx, "errors should appear before warnings")
}

func TestRenderScore_ProgressBars(t *testing.T) {
	output := tui.RenderScore(sampleScore())
	assert.Contains(t, output, "█")
}

func TestRenderScore_StatusIndicators(t *testing.T) {
	output := tui.RenderScore(sampleScore())
	assert.Contains(t, output, "●", "should use ● indicators for sub-metrics")
	assert.Contains(t, output, "○", "should use ○ for skipped sub-metrics")
}

func TestRenderScore_IssueSummaryCount(t *testing.T) {
	output := tui.RenderScore(sampleScore())
	assert.Contains(t, output, "1 errors")
	assert.Contains(t, output, "1 warnings")
}

func indexOf(s, substr string) int {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return i
		}
	}
	return -1
}
