package scoring

import (
	"fmt"
	"math"
	"strings"

	"github.com/openkraft/openkraft/internal/domain"
)

// decayK controls how gradually credit decays past the threshold.
// With k=4, credit reaches zero at threshold*5 (5x threshold).
// Calibrated alongside severityPenaltyScale=120 to produce industry-aligned
// scores (88-98 for well-maintained OSS projects).
const decayK = 4

func isTestFile(path string) bool {
	return strings.HasSuffix(path, "_test.go")
}

// decayCredit returns a continuous credit in [0,1] using linear decay.
// At or below threshold: 1.0. Beyond threshold: linearly decays to 0.0
// at threshold*(decayK+1).
func decayCredit(value, threshold int) float64 {
	if value <= threshold {
		return 1.0
	}
	credit := 1.0 - float64(value-threshold)/float64(threshold*decayK)
	return max(0.0, credit)
}

// severityPenaltyScale converts the debt ratio (severity_weight / funcCount)
// into a point deduction. Calibrated so that a 6% debt ratio yields ~7
// points of penalty, aligning with SonarQube's SQALE model where well-
// maintained OSS projects receive B grades (79-82 range).
const severityPenaltyScale = 120.0

// ScoreCodeHealth evaluates the 5 code smells that predict AI refactoring success.
// Weight: 0.25 (25% of overall score).
//
// The score is computed as a hybrid of two signals:
//  1. Ratio-based sub-metrics (0–100): continuous decay credit per function/file.
//  2. Severity-weighted penalty: deducts points based on issue density and severity.
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

	base := 0
	for _, sm := range cat.SubMetrics {
		base += sm.Score
	}

	cat.Issues = collectCodeHealthIssues(profile, analyzed)

	// Count non-generated functions for normalization.
	funcCount := 0
	for _, af := range analyzed {
		if af.IsGenerated {
			continue
		}
		funcCount += len(af.Functions)
	}

	penalty := severityPenalty(cat.Issues, funcCount)
	cat.Score = max(0, base-penalty)

	return cat
}

// severityPenalty computes a point deduction based on the debt ratio
// (severity_weight / funcCount). This rate-based approach ensures that
// codebases of different sizes are compared fairly — same violation rate
// produces the same penalty regardless of codebase size.
//
// An error floor guarantees at least 1 point deduction when any error-level
// issue exists, so critical violations never go unnoticed.
func severityPenalty(issues []domain.Issue, funcCount int) int {
	if len(issues) == 0 || funcCount == 0 {
		return 0
	}

	var weight float64
	var hasError bool
	for _, iss := range issues {
		switch iss.Severity {
		case domain.SeverityError:
			weight += 3.0
			hasError = true
		case domain.SeverityWarning:
			weight += 1.0
		case domain.SeverityInfo:
			weight += 0.2
		}
	}

	debtRatio := weight / float64(funcCount)
	penalty := int(math.Round(debtRatio * severityPenaltyScale))

	// Floor: at least 1 point if any error-level issue exists.
	if hasError && penalty < 1 {
		penalty = 1
	}

	return penalty
}

// isTemplateFunc reports whether a function is dominated by string literals,
// indicating it's a template holder (e.g., shell completion scripts) rather
// than logic. Uses the configurable StringLiteralThreshold from the profile.
func isTemplateFunc(fn domain.Function, profile *domain.ScoringProfile) bool {
	threshold := profile.StringLiteralThreshold
	if threshold <= 0 {
		threshold = 0.8
	}
	return fn.StringLiteralRatio > threshold
}

// templateMultiplier returns the configured size multiplier for template
// functions, defaulting to 5 if unset.
func templateMultiplier(profile *domain.ScoringProfile) int {
	if profile.TemplateFuncSizeMultiplier > 0 {
		return profile.TemplateFuncSizeMultiplier
	}
	return 5
}

// isDataHeavyTest reports whether a function in a test file is a table-driven
// test dominated by data declarations. These functions are long (300-2000+ lines)
// but structurally simple — at most a for-range + t.Run + assertion nesting pattern.
// MaxNesting <= 2 accommodates the standard Go table-test pattern:
//
//	for _, tt := range tests {
//	    t.Run(tt.name, func(t *testing.T) {  // nesting 1
//	        if condition {                     // nesting 2
//
// They receive the template multiplier instead of the normal 2x test multiplier.
func isDataHeavyTest(fn domain.Function, isTest bool) bool {
	return isTest && fn.MaxNesting <= 2 && fn.MaxCondOps <= 1
}

// isSwitchDispatch reports whether a function is dominated by a single switch
// statement with many structurally-identical case arms. These functions (e.g.,
// zap's Any(), ollama's String()) have zero cognitive complexity — each case
// is independent and trivially understood — but are flagged for function_size.
func isSwitchDispatch(fn domain.Function) bool {
	return fn.MaxCaseArms >= 10 && fn.AvgCaseLines <= 3.0
}

// scoreFunctionSize (20 pts): continuous decay from profile.MaxFunctionLines.
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
		isTest := isTestFile(af.Path)
		for _, fn := range af.Functions {
			lines := fn.LineEnd - fn.LineStart + 1
			if lines <= 0 {
				continue
			}
			total++
			fnMax := effectiveMax
			if isTemplateFunc(fn, profile) {
				fnMax = effectiveMax * templateMultiplier(profile)
			} else if isDataHeavyTest(fn, isTest) {
				fnMax = maxLines * templateMultiplier(profile)
			} else if isSwitchDispatch(fn) {
				fnMax = maxLines * templateMultiplier(profile)
			}
			earned += decayCredit(lines, fnMax)
		}
	}
	if total == 0 {
		sm.Score = sm.Points
		sm.Detail = "no functions to evaluate"
		return sm
	}

	ratio := earned / float64(total)
	sm.Score = int(math.Round(ratio * float64(sm.Points)))
	sm.Score = min(sm.Score, sm.Points)
	sm.Detail = fmt.Sprintf("%.0f%% of %d functions within size limits (max %d lines)", ratio*100, total, maxLines)
	return sm
}

// scoreFileSize (20 pts): continuous decay from profile.MaxFileLines.
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
		total++
		earned += decayCredit(af.TotalLines, effectiveMax)
	}
	if total == 0 {
		sm.Score = sm.Points
		sm.Detail = "no files to evaluate"
		return sm
	}

	ratio := earned / float64(total)
	sm.Score = int(math.Round(ratio * float64(sm.Points)))
	sm.Score = min(sm.Score, sm.Points)
	sm.Detail = fmt.Sprintf("%.0f%% of %d files within size limits (max %d lines)", ratio*100, total, maxLines)
	return sm
}

// scoreNestingDepth (20 pts): continuous decay from profile.MaxNestingDepth.
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
			earned += decayCredit(fn.MaxNesting, effectiveMax)
		}
	}
	if total == 0 {
		sm.Score = sm.Points
		sm.Detail = "no functions to evaluate"
		return sm
	}

	ratio := earned / float64(total)
	sm.Score = int(math.Round(ratio * float64(sm.Points)))
	sm.Score = min(sm.Score, sm.Points)
	sm.Detail = fmt.Sprintf("%.0f%% of %d functions within nesting limits (max %d)", ratio*100, total, maxDepth)
	return sm
}

// scoreParameterCount (20 pts): continuous decay from profile.MaxParameters.
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
		if af.HasCGoImport {
			effectiveMax = max(effectiveMax, profile.CGoParamThreshold)
		}
		for _, fn := range af.Functions {
			total++
			if isExemptFromParams(fn.Name, profile.ExemptParamPatterns) {
				earned += 1.0
				continue
			}
			earned += decayCredit(len(fn.Params), effectiveMax)
		}
	}
	if total == 0 {
		sm.Score = sm.Points
		sm.Detail = "no functions to evaluate"
		return sm
	}

	ratio := earned / float64(total)
	sm.Score = int(math.Round(ratio * float64(sm.Points)))
	sm.Score = min(sm.Score, sm.Points)
	sm.Detail = fmt.Sprintf("%.0f%% of %d functions within parameter limits (max %d)", ratio*100, total, maxParams)
	return sm
}

// scoreComplexConditionals (20 pts): continuous decay from profile.MaxConditionalOps.
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
			earned += decayCredit(fn.MaxCondOps, effectiveMax)
		}
	}
	if total == 0 {
		sm.Score = sm.Points
		sm.Detail = "no functions to evaluate"
		return sm
	}

	ratio := earned / float64(total)
	sm.Score = int(math.Round(ratio * float64(sm.Points)))
	sm.Score = min(sm.Score, sm.Points)
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

		// Compute per-file thresholds aligned with scoring boundaries.
		// Issues start where score penalties start — no silent zone.
		funcThresh := profile.MaxFunctionLines
		nestThresh := profile.MaxNestingDepth
		paramThresh := profile.MaxParameters
		condThresh := profile.MaxConditionalOps
		fileThresh := profile.MaxFileLines
		if testFile {
			funcThresh = profile.MaxFunctionLines * 2
			nestThresh = profile.MaxNestingDepth + 1
			paramThresh = profile.MaxParameters + 2
			condThresh = profile.MaxConditionalOps + 1
			fileThresh = profile.MaxFileLines * 2
		}
		if af.HasCGoImport {
			paramThresh = max(paramThresh, profile.CGoParamThreshold)
		}

		for _, fn := range af.Functions {
			pat := funcPattern(fn.Name)
			lines := fn.LineEnd - fn.LineStart + 1

			// Template functions (dominated by string literals) get a relaxed size threshold.
			// Data-heavy tests (low complexity table-driven tests) get the same relaxation.
			// Switch-dispatch functions (many simple case arms) get the same relaxation.
			fnFuncThresh := funcThresh
			if isTemplateFunc(fn, profile) {
				fnFuncThresh = funcThresh * templateMultiplier(profile)
			} else if isDataHeavyTest(fn, testFile) {
				fnFuncThresh = profile.MaxFunctionLines * templateMultiplier(profile)
			} else if isSwitchDispatch(fn) {
				fnFuncThresh = profile.MaxFunctionLines * templateMultiplier(profile)
			}
			if lines > fnFuncThresh {
				issues = append(issues, domain.Issue{
					Severity:  issueSeverity(lines, fnFuncThresh),
					Category:  "code_health",
					SubMetric: "function_size",
					File:      af.Path,
					Line:      fn.LineStart,
					Message:   fmt.Sprintf("function %s is %d lines (>%d)", fn.Name, lines, fnFuncThresh),
					Pattern:   pat,
				})
			}
			if fn.MaxNesting > nestThresh {
				issues = append(issues, domain.Issue{
					Severity:  issueSeverity(fn.MaxNesting, nestThresh),
					Category:  "code_health",
					SubMetric: "nesting_depth",
					File:      af.Path,
					Line:      fn.LineStart,
					Message:   fmt.Sprintf("function %s has nesting depth %d (>%d)", fn.Name, fn.MaxNesting, nestThresh),
					Pattern:   pat,
				})
			}
			if len(fn.Params) > paramThresh && !isExemptFromParams(fn.Name, profile.ExemptParamPatterns) {
				issues = append(issues, domain.Issue{
					Severity:  issueSeverity(len(fn.Params), paramThresh),
					Category:  "code_health",
					SubMetric: "parameter_count",
					File:      af.Path,
					Line:      fn.LineStart,
					Message:   fmt.Sprintf("function %s has %d parameters (>%d)", fn.Name, len(fn.Params), paramThresh),
					Pattern:   pat,
				})
			}
			if fn.MaxCondOps > condThresh {
				issues = append(issues, domain.Issue{
					Severity:  issueSeverity(fn.MaxCondOps, condThresh),
					Category:  "code_health",
					SubMetric: "complex_conditionals",
					File:      af.Path,
					Line:      fn.LineStart,
					Message:   fmt.Sprintf("function %s has %d conditional operators (>%d)", fn.Name, fn.MaxCondOps, condThresh),
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
