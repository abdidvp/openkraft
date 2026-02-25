package scoring

import (
	"fmt"

	"github.com/openkraft/openkraft/internal/domain"
)

// ScoreCodeHealth evaluates the 5 code smells that predict AI refactoring success.
// Weight: 0.25 (25% of overall score).
func ScoreCodeHealth(profile *domain.ScoringProfile, scan *domain.ScanResult, analyzed map[string]*domain.AnalyzedFile) domain.CategoryScore {
	cat := domain.CategoryScore{
		Name:   "code_health",
		Weight: 0.25,
	}

	sm1 := scoreFunctionSize(profile, analyzed)
	sm2 := scoreFileSize(profile, analyzed)
	sm3 := scoreNestingDepth(profile, analyzed)
	sm4 := scoreParameterCount(profile, analyzed)
	sm5 := scoreComplexConditionals(profile, analyzed)

	cat.SubMetrics = []domain.SubMetric{sm1, sm2, sm3, sm4, sm5}

	total := 0
	for _, sm := range cat.SubMetrics {
		total += sm.Score
	}
	cat.Score = total

	cat.Issues = collectCodeHealthIssues(profile, analyzed)
	return cat
}

// scoreFunctionSize (20 pts): uses profile.MaxFunctionLines as threshold.
// ≤max=full, max+1 to max*2=partial, >max*2=zero per function.
func scoreFunctionSize(profile *domain.ScoringProfile, analyzed map[string]*domain.AnalyzedFile) domain.SubMetric {
	sm := domain.SubMetric{Name: "function_size", Points: 20}
	maxLines := profile.MaxFunctionLines

	total, earned := 0, 0.0
	for _, af := range analyzed {
		for _, fn := range af.Functions {
			lines := fn.LineEnd - fn.LineStart + 1
			if lines <= 0 {
				continue
			}
			total++
			switch {
			case lines <= maxLines:
				earned += 1.0
			case lines <= maxLines*2:
				earned += 0.5
			}
		}
	}
	if total == 0 {
		sm.Detail = "no functions found"
		return sm
	}

	ratio := earned / float64(total)
	sm.Score = int(ratio * float64(sm.Points))
	if sm.Score > sm.Points {
		sm.Score = sm.Points
	}
	sm.Detail = fmt.Sprintf("%.0f%% of %d functions within size limits (max %d lines)", ratio*100, total, maxLines)
	return sm
}

// scoreFileSize (20 pts): uses profile.MaxFileLines as threshold.
// ≤max=full, max+1 to max*5/3=partial, >max*5/3=zero.
func scoreFileSize(profile *domain.ScoringProfile, analyzed map[string]*domain.AnalyzedFile) domain.SubMetric {
	sm := domain.SubMetric{Name: "file_size", Points: 20}
	maxLines := profile.MaxFileLines
	partialLimit := maxLines * 5 / 3

	total, earned := 0, 0.0
	for _, af := range analyzed {
		if af.TotalLines <= 0 {
			continue
		}
		total++
		switch {
		case af.TotalLines <= maxLines:
			earned += 1.0
		case af.TotalLines <= partialLimit:
			earned += 0.5
		}
	}
	if total == 0 {
		sm.Detail = "no files with line counts"
		return sm
	}

	ratio := earned / float64(total)
	sm.Score = int(ratio * float64(sm.Points))
	if sm.Score > sm.Points {
		sm.Score = sm.Points
	}
	sm.Detail = fmt.Sprintf("%.0f%% of %d files within size limits (max %d lines)", ratio*100, total, maxLines)
	return sm
}

// scoreNestingDepth (20 pts): uses profile.MaxNestingDepth as threshold.
// ≤max=full, max+1=partial, >max+1=zero per function.
func scoreNestingDepth(profile *domain.ScoringProfile, analyzed map[string]*domain.AnalyzedFile) domain.SubMetric {
	sm := domain.SubMetric{Name: "nesting_depth", Points: 20}
	maxDepth := profile.MaxNestingDepth

	total, earned := 0, 0.0
	for _, af := range analyzed {
		for _, fn := range af.Functions {
			total++
			switch {
			case fn.MaxNesting <= maxDepth:
				earned += 1.0
			case fn.MaxNesting == maxDepth+1:
				earned += 0.5
			}
		}
	}
	if total == 0 {
		sm.Detail = "no functions found"
		return sm
	}

	ratio := earned / float64(total)
	sm.Score = int(ratio * float64(sm.Points))
	if sm.Score > sm.Points {
		sm.Score = sm.Points
	}
	sm.Detail = fmt.Sprintf("%.0f%% of %d functions within nesting limits (max %d)", ratio*100, total, maxDepth)
	return sm
}

// scoreParameterCount (20 pts): uses profile.MaxParameters as threshold.
// ≤max=full, max+1 to max+2=partial, >max+2=zero per function.
func scoreParameterCount(profile *domain.ScoringProfile, analyzed map[string]*domain.AnalyzedFile) domain.SubMetric {
	sm := domain.SubMetric{Name: "parameter_count", Points: 20}
	maxParams := profile.MaxParameters

	total, earned := 0, 0.0
	for _, af := range analyzed {
		for _, fn := range af.Functions {
			total++
			paramCount := len(fn.Params)
			switch {
			case paramCount <= maxParams:
				earned += 1.0
			case paramCount <= maxParams+2:
				earned += 0.5
			}
		}
	}
	if total == 0 {
		sm.Detail = "no functions found"
		return sm
	}

	ratio := earned / float64(total)
	sm.Score = int(ratio * float64(sm.Points))
	if sm.Score > sm.Points {
		sm.Score = sm.Points
	}
	sm.Detail = fmt.Sprintf("%.0f%% of %d functions within parameter limits (max %d)", ratio*100, total, maxParams)
	return sm
}

// scoreComplexConditionals (20 pts): uses profile.MaxConditionalOps as threshold.
// ≤max=full, max+1=partial, >max+1=zero.
func scoreComplexConditionals(profile *domain.ScoringProfile, analyzed map[string]*domain.AnalyzedFile) domain.SubMetric {
	sm := domain.SubMetric{Name: "complex_conditionals", Points: 20}
	maxOps := profile.MaxConditionalOps

	total, earned := 0, 0.0
	for _, af := range analyzed {
		for _, fn := range af.Functions {
			total++
			switch {
			case fn.MaxCondOps <= maxOps:
				earned += 1.0
			case fn.MaxCondOps == maxOps+1:
				earned += 0.5
			}
		}
	}
	if total == 0 {
		sm.Detail = "no functions found"
		return sm
	}

	ratio := earned / float64(total)
	sm.Score = int(ratio * float64(sm.Points))
	if sm.Score > sm.Points {
		sm.Score = sm.Points
	}
	sm.Detail = fmt.Sprintf("%.0f%% of %d functions within conditional complexity limits (max %d ops)", ratio*100, total, maxOps)
	return sm
}

func collectCodeHealthIssues(profile *domain.ScoringProfile, analyzed map[string]*domain.AnalyzedFile) []domain.Issue {
	var issues []domain.Issue
	funcIssueThreshold := profile.MaxFunctionLines * 2
	nestIssueThreshold := profile.MaxNestingDepth + 2
	paramIssueThreshold := profile.MaxParameters + 3
	fileIssueThreshold := profile.MaxFileLines * 5 / 3

	for _, af := range analyzed {
		for _, fn := range af.Functions {
			lines := fn.LineEnd - fn.LineStart + 1
			if lines > funcIssueThreshold {
				issues = append(issues, domain.Issue{
					Severity: domain.SeverityWarning,
					Category: "code_health",
					File:     af.Path,
					Line:     fn.LineStart,
					Message:  fmt.Sprintf("function %s is %d lines (>%d)", fn.Name, lines, funcIssueThreshold),
				})
			}
			if fn.MaxNesting >= nestIssueThreshold {
				issues = append(issues, domain.Issue{
					Severity: domain.SeverityWarning,
					Category: "code_health",
					File:     af.Path,
					Line:     fn.LineStart,
					Message:  fmt.Sprintf("function %s has nesting depth %d (≥%d)", fn.Name, fn.MaxNesting, nestIssueThreshold),
				})
			}
			if len(fn.Params) >= paramIssueThreshold {
				issues = append(issues, domain.Issue{
					Severity: domain.SeverityWarning,
					Category: "code_health",
					File:     af.Path,
					Line:     fn.LineStart,
					Message:  fmt.Sprintf("function %s has %d parameters (≥%d)", fn.Name, len(fn.Params), paramIssueThreshold),
				})
			}
		}
		if af.TotalLines > fileIssueThreshold {
			issues = append(issues, domain.Issue{
				Severity: domain.SeverityWarning,
				Category: "code_health",
				File:     af.Path,
				Message:  fmt.Sprintf("file has %d lines (>%d)", af.TotalLines, fileIssueThreshold),
			})
		}
	}
	return issues
}
