package scoring

import (
	"fmt"
	"math"
	"strings"

	"github.com/openkraft/openkraft/internal/domain"
)

func isTestFile(path string) bool {
	return strings.HasSuffix(path, "_test.go")
}

// ScoreCodeHealth evaluates the 5 code smells that predict AI refactoring success.
// Weight: 0.25 (25% of overall score).
func ScoreCodeHealth(profile *domain.ScoringProfile, scan *domain.ScanResult, analyzed map[string]*domain.AnalyzedFile) domain.CategoryScore {
	if profile == nil {
		p := domain.DefaultProfile()
		profile = &p
	}

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
		if af.IsGenerated {
			continue
		}
		effectiveMax := maxLines
		if isTestFile(af.Path) {
			effectiveMax = maxLines * 2
		}
		for _, fn := range af.Functions {
			lines := fn.LineEnd - fn.LineStart + 1
			if lines <= 0 {
				continue
			}
			total++
			switch {
			case lines <= effectiveMax:
				earned += 1.0
			case lines <= effectiveMax*2:
				earned += 0.5
			}
		}
	}
	if total == 0 {
		sm.Score = sm.Points
		sm.Detail = "no functions to evaluate"
		return sm
	}

	ratio := earned / float64(total)
	sm.Score = int(math.Round(ratio * float64(sm.Points)))
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

	total, earned := 0, 0.0
	for _, af := range analyzed {
		if af.IsGenerated || af.TotalLines <= 0 {
			continue
		}
		effectiveMax := maxLines
		if isTestFile(af.Path) {
			effectiveMax = maxLines * 2
		}
		partialLimit := effectiveMax * 5 / 3
		total++
		switch {
		case af.TotalLines <= effectiveMax:
			earned += 1.0
		case af.TotalLines <= partialLimit:
			earned += 0.5
		}
	}
	if total == 0 {
		sm.Score = sm.Points
		sm.Detail = "no files to evaluate"
		return sm
	}

	ratio := earned / float64(total)
	sm.Score = int(math.Round(ratio * float64(sm.Points)))
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
		if af.IsGenerated {
			continue
		}
		effectiveMax := maxDepth
		if isTestFile(af.Path) {
			effectiveMax = maxDepth + 1
		}
		for _, fn := range af.Functions {
			total++
			switch {
			case fn.MaxNesting <= effectiveMax:
				earned += 1.0
			case fn.MaxNesting == effectiveMax+1:
				earned += 0.5
			}
		}
	}
	if total == 0 {
		sm.Score = sm.Points
		sm.Detail = "no functions to evaluate"
		return sm
	}

	ratio := earned / float64(total)
	sm.Score = int(math.Round(ratio * float64(sm.Points)))
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
		if af.IsGenerated {
			continue
		}
		effectiveMax := maxParams
		if isTestFile(af.Path) {
			effectiveMax = maxParams + 2
		}
		for _, fn := range af.Functions {
			total++
			if isExemptFromParams(fn.Name, profile.ExemptParamPatterns) {
				earned += 1.0 // exempt pattern: skip parameter limits
				continue
			}
			paramCount := len(fn.Params)
			switch {
			case paramCount <= effectiveMax:
				earned += 1.0
			case paramCount <= effectiveMax+2:
				earned += 0.5
			}
		}
	}
	if total == 0 {
		sm.Score = sm.Points
		sm.Detail = "no functions to evaluate"
		return sm
	}

	ratio := earned / float64(total)
	sm.Score = int(math.Round(ratio * float64(sm.Points)))
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
		if af.IsGenerated {
			continue
		}
		effectiveMax := maxOps
		if isTestFile(af.Path) {
			effectiveMax = maxOps + 1
		}
		for _, fn := range af.Functions {
			total++
			switch {
			case fn.MaxCondOps <= effectiveMax:
				earned += 1.0
			case fn.MaxCondOps == effectiveMax+1:
				earned += 0.5
			}
		}
	}
	if total == 0 {
		sm.Score = sm.Points
		sm.Detail = "no functions to evaluate"
		return sm
	}

	ratio := earned / float64(total)
	sm.Score = int(math.Round(ratio * float64(sm.Points)))
	if sm.Score > sm.Points {
		sm.Score = sm.Points
	}
	sm.Detail = fmt.Sprintf("%.0f%% of %d functions within conditional complexity limits (max %d ops)", ratio*100, total, maxOps)
	return sm
}

// isExemptFromParams reports whether the function name matches any of the
// configured exempt prefixes for parameter count scoring.
func isExemptFromParams(name string, patterns []string) bool {
	for _, p := range patterns {
		if strings.HasPrefix(name, p) {
			return true
		}
	}
	return false
}

// issueSeverity returns a severity level based on how far the actual value
// exceeds the threshold. ≥3x = error, ≥1.5x = warning, else info.
func issueSeverity(actual, threshold int) string {
	if threshold <= 0 {
		return domain.SeverityWarning
	}
	ratio := float64(actual) / float64(threshold)
	switch {
	case ratio >= 3.0:
		return domain.SeverityError
	case ratio >= 1.5:
		return domain.SeverityWarning
	default:
		return domain.SeverityInfo
	}
}

// funcPattern classifies a function name into a pattern for issue grouping.
func funcPattern(name string) string {
	switch {
	case strings.HasPrefix(name, "Reconstruct"):
		return "reconstruct"
	case strings.HasPrefix(name, "New"):
		return "constructor"
	case strings.HasPrefix(name, "Test"):
		return "test"
	default:
		return ""
	}
}

// filePattern classifies a file path into a pattern for issue grouping.
func filePattern(path string) string {
	if strings.Contains(path, "sqlc/") || strings.HasSuffix(path, "_gen.go") {
		return "generated"
	}
	return ""
}

func collectCodeHealthIssues(profile *domain.ScoringProfile, analyzed map[string]*domain.AnalyzedFile) []domain.Issue {
	var issues []domain.Issue

	for _, af := range analyzed {
		if af.IsGenerated {
			continue
		}
		testFile := isTestFile(af.Path)

		// Compute per-file thresholds (relaxed for test files).
		funcThresh := profile.MaxFunctionLines * 2
		nestThresh := profile.MaxNestingDepth + 2
		paramThresh := profile.MaxParameters + 3
		condThresh := profile.MaxConditionalOps + 2
		fileThresh := profile.MaxFileLines * 5 / 3
		if testFile {
			funcThresh = profile.MaxFunctionLines * 4
			nestThresh = profile.MaxNestingDepth + 3
			paramThresh = profile.MaxParameters + 5
			condThresh = profile.MaxConditionalOps + 3
			fileThresh = profile.MaxFileLines * 10 / 3
		}

		for _, fn := range af.Functions {
			pat := funcPattern(fn.Name)
			lines := fn.LineEnd - fn.LineStart + 1
			if lines > funcThresh {
				issues = append(issues, domain.Issue{
					Severity:  issueSeverity(lines, funcThresh),
					Category:  "code_health",
					SubMetric: "function_size",
					File:      af.Path,
					Line:      fn.LineStart,
					Message:   fmt.Sprintf("function %s is %d lines (>%d)", fn.Name, lines, funcThresh),
					Pattern:   pat,
				})
			}
			if fn.MaxNesting >= nestThresh {
				issues = append(issues, domain.Issue{
					Severity:  issueSeverity(fn.MaxNesting, nestThresh),
					Category:  "code_health",
					SubMetric: "nesting_depth",
					File:      af.Path,
					Line:      fn.LineStart,
					Message:   fmt.Sprintf("function %s has nesting depth %d (≥%d)", fn.Name, fn.MaxNesting, nestThresh),
					Pattern:   pat,
				})
			}
			if len(fn.Params) >= paramThresh && !isExemptFromParams(fn.Name, profile.ExemptParamPatterns) {
				issues = append(issues, domain.Issue{
					Severity:  issueSeverity(len(fn.Params), paramThresh),
					Category:  "code_health",
					SubMetric: "parameter_count",
					File:      af.Path,
					Line:      fn.LineStart,
					Message:   fmt.Sprintf("function %s has %d parameters (≥%d)", fn.Name, len(fn.Params), paramThresh),
					Pattern:   pat,
				})
			}
			if fn.MaxCondOps >= condThresh {
				issues = append(issues, domain.Issue{
					Severity:  issueSeverity(fn.MaxCondOps, condThresh),
					Category:  "code_health",
					SubMetric: "complex_conditionals",
					File:      af.Path,
					Line:      fn.LineStart,
					Message:   fmt.Sprintf("function %s has %d conditional operators (≥%d)", fn.Name, fn.MaxCondOps, condThresh),
					Pattern:   pat,
				})
			}
		}
		if af.TotalLines > fileThresh {
			issues = append(issues, domain.Issue{
				Severity:  issueSeverity(af.TotalLines, fileThresh),
				Category:  "code_health",
				SubMetric: "file_size",
				File:      af.Path,
				Message:   fmt.Sprintf("file has %d lines (>%d)", af.TotalLines, fileThresh),
				Pattern:   filePattern(af.Path),
			})
		}
	}
	return issues
}
