package scoring

import (
	"fmt"
	"math"
	"slices"
	"strings"

	"github.com/openkraft/openkraft/internal/domain"
)

func sortInts(s []int) { slices.Sort(s) }

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
	sm3 := scoreCognitiveComplexity(profile, analyzed)
	sm4 := scoreParameterCount(profile, analyzed)
	sm5, dupData := scoreCodeDuplication(profile, analyzed)

	cat.SubMetrics = []domain.SubMetric{sm1, sm2, sm3, sm4, sm5}

	base := 0
	for _, sm := range cat.SubMetrics {
		base += sm.Score
	}

	cat.Issues = collectCodeHealthIssues(profile, analyzed, dupData)

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

// scoreCognitiveComplexity (20 pts): continuous decay from profile.MaxCognitiveComplexity.
// Test files: threshold + 5 (additive, not 2x — CC is already additive).
// Switch-dispatch functions: exempt (earn full credit).
func scoreCognitiveComplexity(profile *domain.ScoringProfile, analyzed map[string]*domain.AnalyzedFile) domain.SubMetric {
	sm := domain.SubMetric{Name: "cognitive_complexity", Points: 20}
	maxCC := profile.MaxCognitiveComplexity

	total, earned := 0, 0.0
	for _, af := range analyzed {
		if af.IsGenerated {
			continue
		}
		effectiveMax := maxCC
		if isTestFile(af.Path) {
			effectiveMax = maxCC + 5
		}
		for _, fn := range af.Functions {
			total++
			if isSwitchDispatch(fn) {
				earned += 1.0
				continue
			}
			earned += decayCredit(fn.CognitiveComplexity, effectiveMax)
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
	sm.Detail = fmt.Sprintf("%.0f%% of %d functions within cognitive complexity limits (max %d)", ratio*100, total, maxCC)
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

/// scoreCodeDuplication (20 pts): Rabin-Karp rolling hash over NormalizedTokens.
// Detects cross-file duplication (intra-file duplicates are ignored).
// Returns a dupInfo map keyed by file path for use by collectCodeHealthIssues.

// dupInfo holds per-file duplication data computed by scoreCodeDuplication
// and consumed by collectCodeHealthIssues without mutating domain types.
type dupInfo struct {
	lines   int // estimated duplicated lines
	percent int // duplication percentage
}

func scoreCodeDuplication(profile *domain.ScoringProfile, analyzed map[string]*domain.AnalyzedFile) (domain.SubMetric, map[string]dupInfo) {
	sm := domain.SubMetric{Name: "code_duplication", Points: 20}
	windowSize := profile.MinCloneTokens
	if windowSize <= 0 {
		windowSize = 50
	}
	maxDupPercent := profile.MaxDuplicationPercent
	if maxDupPercent <= 0 {
		maxDupPercent = 5
	}

	// Collect files with enough tokens.
	type fileEntry struct {
		path   string
		af     *domain.AnalyzedFile
		tokens []int
	}
	var files []fileEntry
	for _, af := range analyzed {
		if af.IsGenerated || len(af.NormalizedTokens) < windowSize {
			continue
		}
		files = append(files, fileEntry{path: af.Path, af: af, tokens: af.NormalizedTokens})
	}

	dupMap := make(map[string]dupInfo)

	if len(files) < 2 {
		// Need at least 2 files for cross-file duplication.
		sm.Score = sm.Points
		sm.Detail = "no duplication detected"
		return sm, dupMap
	}

	// Build hash → set of file indices.
	type loc struct {
		fileIdx int
		pos     int
	}
	hashMap := make(map[uint64][]loc)

	const base uint64 = 131
	for fi, fe := range files {
		tokens := fe.tokens
		if len(tokens) < windowSize {
			continue
		}

		// Compute initial hash and basePow.
		var h uint64
		var basePow uint64 = 1
		for i := 0; i < windowSize; i++ {
			h = h*base + uint64(tokens[i]+10) // +10 to avoid negative token issues
			if i < windowSize-1 {
				basePow *= base
			}
		}
		hashMap[h] = append(hashMap[h], loc{fi, 0})

		// Roll the hash.
		for i := 1; i <= len(tokens)-windowSize; i++ {
			removed := uint64(tokens[i-1] + 10)
			added := uint64(tokens[i+windowSize-1] + 10)
			h = h*base - removed*basePow*base + added
			hashMap[h] = append(hashMap[h], loc{fi, i})
		}
	}

	// Find hashes that appear in ≥2 distinct files.
	// Track the starting positions of duplicate windows per file so we can
	// compute covered token ranges without overcounting overlaps.
	dupPositions := make(map[int][]int) // fileIdx → sorted start positions
	for _, locs := range hashMap {
		fileSet := make(map[int]bool)
		for _, l := range locs {
			fileSet[l.fileIdx] = true
		}
		if len(fileSet) < 2 {
			continue // intra-file only — skip
		}
		for _, l := range locs {
			dupPositions[l.fileIdx] = append(dupPositions[l.fileIdx], l.pos)
		}
	}

	// Estimate duplicated lines and score each file.
	total, earned := 0, 0.0
	for fi, fe := range files {
		total++
		positions := dupPositions[fi]
		if len(positions) == 0 {
			earned += 1.0
			continue
		}

		// Count unique token positions covered by duplicate windows.
		// Each window starting at pos covers tokens [pos, pos+windowSize).
		// Merge overlapping ranges to avoid overcounting.
		covered := 0
		maxEnd := 0
		// Sort positions (they may arrive out of order from hash map iteration).
		sortInts(positions)
		for _, pos := range positions {
			end := pos + windowSize
			if pos >= maxEnd {
				// Non-overlapping new range.
				covered += windowSize
			} else if end > maxEnd {
				// Partially overlapping — only count the extension.
				covered += end - maxEnd
			}
			if end > maxEnd {
				maxEnd = end
			}
		}

		// Convert covered tokens to lines (conservative: at least 1 token per line).
		tokensPerLine := float64(len(fe.tokens)) / float64(max(1, fe.af.TotalLines))
		if tokensPerLine < 1 {
			tokensPerLine = 1
		}
		dupLines := int(float64(covered) / tokensPerLine)
		if dupLines > fe.af.TotalLines {
			dupLines = fe.af.TotalLines
		}
		dupPercent := dupLines * 100 / max(1, fe.af.TotalLines)
		dupMap[fe.path] = dupInfo{lines: dupLines, percent: dupPercent}
		thresh := maxDupPercent
		if isTestFile(fe.path) {
			thresh = maxDupPercent * 2 // test files get relaxed threshold
		}
		earned += decayCredit(dupPercent, thresh)
	}

	if total == 0 {
		sm.Score = sm.Points
		sm.Detail = "no files to evaluate"
		return sm, dupMap
	}

	ratio := earned / float64(total)
	sm.Score = int(math.Round(ratio * float64(sm.Points)))
	sm.Score = min(sm.Score, sm.Points)
	sm.Detail = fmt.Sprintf("%.0f%% of %d files within duplication limits (max %d%%)", ratio*100, total, maxDupPercent)
	return sm, dupMap
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

func collectCodeHealthIssues(profile *domain.ScoringProfile, analyzed map[string]*domain.AnalyzedFile, dupData map[string]dupInfo) []domain.Issue {
	var issues []domain.Issue

	for _, af := range analyzed {
		if af.IsGenerated {
			continue
		}
		testFile := isTestFile(af.Path)

		// Compute per-file thresholds aligned with scoring boundaries.
		// Issues start where score penalties start — no silent zone.
		funcThresh := profile.MaxFunctionLines
		paramThresh := profile.MaxParameters
		ccThresh := profile.MaxCognitiveComplexity
		fileThresh := profile.MaxFileLines
		dupThresh := profile.MaxDuplicationPercent
		if dupThresh <= 0 {
			dupThresh = 5
		}
		if testFile {
			funcThresh = profile.MaxFunctionLines * 2
			paramThresh = profile.MaxParameters + 2
			ccThresh = profile.MaxCognitiveComplexity + 5
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
			if !isSwitchDispatch(fn) && fn.CognitiveComplexity > ccThresh {
				issues = append(issues, domain.Issue{
					Severity:  issueSeverity(fn.CognitiveComplexity, ccThresh),
					Category:  "code_health",
					SubMetric: "cognitive_complexity",
					File:      af.Path,
					Line:      fn.LineStart,
					Message:   fmt.Sprintf("function %s has cognitive complexity %d (>%d)", fn.Name, fn.CognitiveComplexity, ccThresh),
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
		// Code duplication issues (file-level, after function loop).
		if di, ok := dupData[af.Path]; ok && di.lines > 0 {
			fileDupThresh := dupThresh
			if isTestFile(af.Path) {
				fileDupThresh = dupThresh * 2 // test files get relaxed threshold
			}
			if di.percent > fileDupThresh {
				issues = append(issues, domain.Issue{
					Severity:  issueSeverity(di.percent, fileDupThresh),
					Category:  "code_health",
					SubMetric: "code_duplication",
					File:      af.Path,
					Message:   fmt.Sprintf("file has %d%% duplicated lines (%d lines, >%d%%)", di.percent, di.lines, fileDupThresh),
					Pattern:   filePattern(af.Path),
				})
			}
		}
	}
	return issues
}
