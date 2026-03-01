package tui_test

import (
	"testing"

	"github.com/abdidvp/openkraft/internal/adapters/outbound/tui"
	"github.com/abdidvp/openkraft/internal/domain"
	"github.com/stretchr/testify/assert"
)

func sampleScore() *domain.Score {
	return &domain.Score{
		Overall: 67,
		Categories: []domain.CategoryScore{
			{
				Name: "code_health", Score: 80, Weight: 0.25,
				SubMetrics: []domain.SubMetric{
					{Name: "function_size", Score: 20, Points: 20, Detail: "all functions small"},
					{Name: "cognitive_complexity", Score: 5, Points: 20, Detail: "3 complex functions"},
				},
				Issues: []domain.Issue{
					{Severity: "error", Category: "code_health", File: "internal/domain/foo.go", Message: "function too long"},
				},
			},
			{
				Name: "verifiability", Score: 45, Weight: 0.15,
				SubMetrics: []domain.SubMetric{
					{Name: "test_presence", Score: 25, Points: 25, Detail: "good coverage"},
					{Name: "test_naming", Score: 0, Points: 25, Detail: "none found"},
					{Name: "interface_contracts", Score: 0, Points: 25, Skipped: true},
				},
				Issues: []domain.Issue{
					{Severity: "warning", Category: "verifiability", Message: "missing test naming conventions"},
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
	assert.Contains(t, output, "code_health")
	assert.Contains(t, output, "verifiability")
}

func TestRenderScore_ContainsGrade(t *testing.T) {
	output := tui.RenderScore(sampleScore())
	assert.Contains(t, output, "C")
}

func TestRenderScore_ContainsSubMetrics(t *testing.T) {
	output := tui.RenderScore(sampleScore())
	assert.Contains(t, output, "function_size")
	assert.Contains(t, output, "cognitive_complexity")
	assert.Contains(t, output, "test_presence")
	assert.Contains(t, output, "test_naming")
}

func TestRenderScore_ShowsSubMetricDetails(t *testing.T) {
	output := tui.RenderScore(sampleScore())
	assert.Contains(t, output, "all functions small")
	assert.Contains(t, output, "3 complex functions")
	assert.Contains(t, output, "none found")
}

func TestRenderScore_ShowsSkippedSubMetrics(t *testing.T) {
	output := tui.RenderScore(sampleScore())
	assert.Contains(t, output, "interface_contracts")
	assert.Contains(t, output, "skipped")
}

func TestRenderScore_ShowsIssues(t *testing.T) {
	output := tui.RenderScore(sampleScore())
	assert.Contains(t, output, "function too long")
	assert.Contains(t, output, "missing test naming conventions")
	assert.Contains(t, output, "Issues")
}

func TestRenderScore_ShowsIssueSeverityTags(t *testing.T) {
	output := tui.RenderScore(sampleScore())
	assert.Contains(t, output, "error")
	assert.Contains(t, output, "warn")
}

func TestRenderScore_ShowsIssueFile(t *testing.T) {
	output := tui.RenderScore(sampleScore())
	assert.Contains(t, output, "internal/domain/foo.go")
}

func TestRenderScore_ErrorsBeforeWarnings(t *testing.T) {
	output := tui.RenderScore(sampleScore())
	errorIdx := indexOf(output, "function too long")
	warnIdx := indexOf(output, "missing test naming conventions")
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
