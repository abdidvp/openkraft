package scoring

import (
	"fmt"

	"github.com/openkraft/openkraft/internal/domain"
)

// ScoreCodeHealth evaluates the 5 code smells that predict AI refactoring success.
// Weight: 0.25 (25% of overall score).
func ScoreCodeHealth(scan *domain.ScanResult, analyzed map[string]*domain.AnalyzedFile) domain.CategoryScore {
	cat := domain.CategoryScore{
		Name:   "code_health",
		Weight: 0.25,
	}

	sm1 := scoreFunctionSize(analyzed)
	sm2 := scoreFileSize(analyzed)
	sm3 := scoreNestingDepth(analyzed)
	sm4 := scoreParameterCount(analyzed)
	sm5 := scoreComplexConditionals(analyzed)

	cat.SubMetrics = []domain.SubMetric{sm1, sm2, sm3, sm4, sm5}

	total := 0
	for _, sm := range cat.SubMetrics {
		total += sm.Score
	}
	cat.Score = total

	cat.Issues = collectCodeHealthIssues(analyzed)
	return cat
}

// scoreFunctionSize (20 pts): ≤50 lines=full, 51-100=partial, >100=zero per function.
func scoreFunctionSize(analyzed map[string]*domain.AnalyzedFile) domain.SubMetric {
	sm := domain.SubMetric{Name: "function_size", Points: 20}

	total, earned := 0, 0.0
	for _, af := range analyzed {
		for _, fn := range af.Functions {
			lines := fn.LineEnd - fn.LineStart + 1
			if lines <= 0 {
				continue
			}
			total++
			switch {
			case lines <= 50:
				earned += 1.0
			case lines <= 100:
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
	sm.Detail = fmt.Sprintf("%.0f%% of %d functions within size limits", ratio*100, total)
	return sm
}

// scoreFileSize (20 pts): ≤300 lines=full, 301-500=partial, >500=zero.
func scoreFileSize(analyzed map[string]*domain.AnalyzedFile) domain.SubMetric {
	sm := domain.SubMetric{Name: "file_size", Points: 20}

	total, earned := 0, 0.0
	for _, af := range analyzed {
		if af.TotalLines <= 0 {
			continue
		}
		total++
		switch {
		case af.TotalLines <= 300:
			earned += 1.0
		case af.TotalLines <= 500:
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
	sm.Detail = fmt.Sprintf("%.0f%% of %d files within size limits", ratio*100, total)
	return sm
}

// scoreNestingDepth (20 pts): ≤3=full, 4=partial, ≥5=zero per function.
func scoreNestingDepth(analyzed map[string]*domain.AnalyzedFile) domain.SubMetric {
	sm := domain.SubMetric{Name: "nesting_depth", Points: 20}

	total, earned := 0, 0.0
	for _, af := range analyzed {
		for _, fn := range af.Functions {
			total++
			switch {
			case fn.MaxNesting <= 3:
				earned += 1.0
			case fn.MaxNesting == 4:
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
	sm.Detail = fmt.Sprintf("%.0f%% of %d functions within nesting limits", ratio*100, total)
	return sm
}

// scoreParameterCount (20 pts): ≤4=full, 5-6=partial, ≥7=zero per function.
func scoreParameterCount(analyzed map[string]*domain.AnalyzedFile) domain.SubMetric {
	sm := domain.SubMetric{Name: "parameter_count", Points: 20}

	total, earned := 0, 0.0
	for _, af := range analyzed {
		for _, fn := range af.Functions {
			total++
			paramCount := len(fn.Params)
			switch {
			case paramCount <= 4:
				earned += 1.0
			case paramCount <= 6:
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
	sm.Detail = fmt.Sprintf("%.0f%% of %d functions within parameter limits", ratio*100, total)
	return sm
}

// scoreComplexConditionals (20 pts): ≤2 &&/||=full, 3=partial, ≥4=zero.
func scoreComplexConditionals(analyzed map[string]*domain.AnalyzedFile) domain.SubMetric {
	sm := domain.SubMetric{Name: "complex_conditionals", Points: 20}

	total, earned := 0, 0.0
	for _, af := range analyzed {
		for _, fn := range af.Functions {
			total++
			switch {
			case fn.MaxCondOps <= 2:
				earned += 1.0
			case fn.MaxCondOps == 3:
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
	sm.Detail = fmt.Sprintf("%.0f%% of %d functions within conditional complexity limits", ratio*100, total)
	return sm
}

func collectCodeHealthIssues(analyzed map[string]*domain.AnalyzedFile) []domain.Issue {
	var issues []domain.Issue
	for _, af := range analyzed {
		for _, fn := range af.Functions {
			lines := fn.LineEnd - fn.LineStart + 1
			if lines > 100 {
				issues = append(issues, domain.Issue{
					Severity: domain.SeverityWarning,
					Category: "code_health",
					File:     af.Path,
					Line:     fn.LineStart,
					Message:  fmt.Sprintf("function %s is %d lines (>100)", fn.Name, lines),
				})
			}
			if fn.MaxNesting >= 5 {
				issues = append(issues, domain.Issue{
					Severity: domain.SeverityWarning,
					Category: "code_health",
					File:     af.Path,
					Line:     fn.LineStart,
					Message:  fmt.Sprintf("function %s has nesting depth %d (≥5)", fn.Name, fn.MaxNesting),
				})
			}
			if len(fn.Params) >= 7 {
				issues = append(issues, domain.Issue{
					Severity: domain.SeverityWarning,
					Category: "code_health",
					File:     af.Path,
					Line:     fn.LineStart,
					Message:  fmt.Sprintf("function %s has %d parameters (≥7)", fn.Name, len(fn.Params)),
				})
			}
		}
		if af.TotalLines > 500 {
			issues = append(issues, domain.Issue{
				Severity: domain.SeverityWarning,
				Category: "code_health",
				File:     af.Path,
				Message:  fmt.Sprintf("file has %d lines (>500)", af.TotalLines),
			})
		}
	}
	return issues
}
