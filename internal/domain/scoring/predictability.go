package scoring

import (
	"fmt"
	"path/filepath"
	"strings"
	"unicode"

	"github.com/openkraft/openkraft/internal/domain"
)

// ScorePredictability evaluates code standardization for AI value.
// Weight: 0.10 (10% of overall score).
func ScorePredictability(profile *domain.ScoringProfile, modules []domain.DetectedModule, scan *domain.ScanResult, analyzed map[string]*domain.AnalyzedFile) domain.CategoryScore {
	cat := domain.CategoryScore{
		Name:   "predictability",
		Weight: 0.10,
	}

	sm1 := scoreSelfDescribingNames(analyzed)
	sm2 := scoreExplicitDependencies(profile, analyzed)
	sm3 := scoreErrorMessageQuality(analyzed)
	sm4 := scoreConsistentPatterns(modules, analyzed)

	cat.SubMetrics = []domain.SubMetric{sm1, sm2, sm3, sm4}

	total := 0
	for _, sm := range cat.SubMetrics {
		total += sm.Score
	}
	cat.Score = total

	cat.Issues = collectPredictabilityIssues(analyzed)
	return cat
}

// scoreSelfDescribingNames (25 pts): exported functions with verb+noun via CamelCase split.
func scoreSelfDescribingNames(analyzed map[string]*domain.AnalyzedFile) domain.SubMetric {
	sm := domain.SubMetric{Name: "self_describing_names", Points: 25}

	total := 0
	verbNoun := 0

	for _, af := range analyzed {
		for _, fn := range af.Functions {
			if !fn.Exported {
				continue
			}
			total++
			if hasVerbNounPattern(fn.Name) {
				verbNoun++
			}
		}
	}

	if total == 0 {
		sm.Detail = "no exported functions found"
		return sm
	}

	ratio := float64(verbNoun) / float64(total)
	sm.Score = int(ratio * float64(sm.Points))
	if sm.Score > sm.Points {
		sm.Score = sm.Points
	}
	sm.Detail = fmt.Sprintf("%d/%d exported functions follow verb+noun naming", verbNoun, total)
	return sm
}

// scoreExplicitDependencies (25 pts): count mutable package-level vars + init() functions.
// Uses profile.MaxGlobalVarPenalty as per-violation penalty.
func scoreExplicitDependencies(profile *domain.ScoringProfile, analyzed map[string]*domain.AnalyzedFile) domain.SubMetric {
	sm := domain.SubMetric{Name: "explicit_dependencies", Points: 25}

	totalFiles := 0
	mutableState := 0

	for _, af := range analyzed {
		if strings.HasSuffix(af.Path, "_test.go") {
			continue
		}
		totalFiles++
		for _, gv := range af.GlobalVars {
			// Only penalize exported vars that aren't sentinel errors (Err* prefix).
			// Unexported vars are implementation details, not cross-package dependencies.
			if len(gv) > 0 && unicode.IsUpper(rune(gv[0])) && !strings.HasPrefix(gv, "Err") {
				mutableState++
			}
		}
		mutableState += af.InitFunctions
	}

	if totalFiles == 0 {
		sm.Detail = "no source files found"
		return sm
	}

	if mutableState == 0 {
		sm.Score = sm.Points
		sm.Detail = "no mutable package-level state or init() functions"
	} else {
		penalty := mutableState * profile.MaxGlobalVarPenalty
		sm.Score = sm.Points - penalty
		if sm.Score < 0 {
			sm.Score = 0
		}
		sm.Detail = fmt.Sprintf("%d mutable package-level vars/init() functions found", mutableState)
	}
	return sm
}

// scoreErrorMessageQuality (25 pts): composite — wrapping ratio 40% + context richness 30%
// + convention compliance 20% + sentinel presence 10%.
func scoreErrorMessageQuality(analyzed map[string]*domain.AnalyzedFile) domain.SubMetric {
	sm := domain.SubMetric{Name: "error_message_quality", Points: 25}

	var totalErrors, wrapped, withContext int
	hasSentinels := false

	for _, af := range analyzed {
		for _, ec := range af.ErrorCalls {
			totalErrors++
			if ec.HasWrap {
				wrapped++
			}
			if ec.HasContext {
				withContext++
			}
		}
		// Check for Err-prefixed vars (sentinel errors).
		for _, gv := range af.GlobalVars {
			if strings.HasPrefix(gv, "Err") {
				hasSentinels = true
			}
		}
	}

	if totalErrors == 0 {
		sm.Score = 0
		sm.Detail = "no error handling found"
		return sm
	}

	wrapRatio := float64(wrapped) / float64(totalErrors)
	contextRatio := float64(withContext) / float64(totalErrors)

	// Convention compliance: errors.New for simple, fmt.Errorf for wrapping.
	conventionCompliance := 0.5 // Base score for having error handling at all.
	if wrapRatio > 0.5 {
		conventionCompliance = 1.0
	} else if wrapRatio > 0.2 {
		conventionCompliance = 0.7
	}

	sentinelScore := 0.0
	if hasSentinels {
		sentinelScore = 1.0
	}

	composite := wrapRatio*0.4 + contextRatio*0.3 + conventionCompliance*0.2 + sentinelScore*0.1
	sm.Score = int(composite * float64(sm.Points))
	if sm.Score > sm.Points {
		sm.Score = sm.Points
	}
	sm.Detail = fmt.Sprintf("wrap=%.0f%%, context=%.0f%%, %d total errors",
		wrapRatio*100, contextRatio*100, totalErrors)
	return sm
}

// scoreConsistentPatterns (25 pts): group functions by role (file suffix), normalize
// signatures, measure modal consistency.
func scoreConsistentPatterns(_ []domain.DetectedModule, analyzed map[string]*domain.AnalyzedFile) domain.SubMetric {
	sm := domain.SubMetric{Name: "consistent_patterns", Points: 25}

	type signature struct {
		paramCount  int
		returnCount int
		hasContext  bool
		hasError    bool
	}

	// Group functions by file role suffix.
	roleSignatures := make(map[string][]signature)

	for _, af := range analyzed {
		if strings.HasSuffix(af.Path, "_test.go") {
			continue
		}
		base := filepath.Base(af.Path)
		name := strings.TrimSuffix(base, ".go")
		role := ""
		if idx := strings.LastIndex(name, "_"); idx >= 0 {
			role = name[idx:]
		}
		if role == "" {
			continue
		}

		for _, fn := range af.Functions {
			if fn.Receiver == "" || !fn.Exported {
				continue
			}
			sig := signature{
				paramCount:  len(fn.Params),
				returnCount: len(fn.Returns),
			}
			for _, p := range fn.Params {
				if p.Type == "context.Context" {
					sig.hasContext = true
				}
			}
			for _, r := range fn.Returns {
				if r == "error" {
					sig.hasError = true
				}
			}
			roleSignatures[role] = append(roleSignatures[role], sig)
		}
	}

	if len(roleSignatures) == 0 {
		sm.Detail = "no role-based function groups found"
		sm.Score = int(0.5 * float64(sm.Points)) // Partial credit
		return sm
	}

	// For each role, check consistency of context/error patterns.
	totalRoles := 0
	consistentRoles := 0
	for _, sigs := range roleSignatures {
		if len(sigs) < 2 {
			continue
		}
		totalRoles++

		// Check if context and error patterns are consistent.
		contextCount := 0
		errorCount := 0
		for _, s := range sigs {
			if s.hasContext {
				contextCount++
			}
			if s.hasError {
				errorCount++
			}
		}
		contextRatio := float64(contextCount) / float64(len(sigs))
		errorRatio := float64(errorCount) / float64(len(sigs))

		// Consistent if all-or-nothing (ratio >0.8 or <0.2).
		contextConsistent := contextRatio >= 0.8 || contextRatio <= 0.2
		errorConsistent := errorRatio >= 0.8 || errorRatio <= 0.2
		if contextConsistent && errorConsistent {
			consistentRoles++
		}
	}

	if totalRoles == 0 {
		sm.Score = int(0.5 * float64(sm.Points))
		sm.Detail = "not enough role groups for consistency analysis"
		return sm
	}

	ratio := float64(consistentRoles) / float64(totalRoles)
	sm.Score = int(ratio * float64(sm.Points))
	if sm.Score > sm.Points {
		sm.Score = sm.Points
	}
	sm.Detail = fmt.Sprintf("%d/%d role groups have consistent patterns", consistentRoles, totalRoles)
	return sm
}

func collectPredictabilityIssues(analyzed map[string]*domain.AnalyzedFile) []domain.Issue {
	var issues []domain.Issue

	totalErrors := 0
	for _, af := range analyzed {
		if !strings.HasSuffix(af.Path, "_test.go") {
			totalErrors += len(af.ErrorCalls)
		}
	}
	if totalErrors == 0 && len(analyzed) > 0 {
		issues = append(issues, domain.Issue{
			Severity: domain.SeverityInfo,
			Category: "predictability",
			Message:  "no error handling found across all source files",
		})
	}

	for _, af := range analyzed {
		if strings.HasSuffix(af.Path, "_test.go") {
			continue
		}
		if len(af.GlobalVars) > 3 {
			issues = append(issues, domain.Issue{
				Severity: domain.SeverityWarning,
				Category: "predictability",
				File:     af.Path,
				Message:  fmt.Sprintf("file has %d package-level variables (prefer explicit injection)", len(af.GlobalVars)),
			})
		}
		if af.InitFunctions > 0 {
			issues = append(issues, domain.Issue{
				Severity: domain.SeverityInfo,
				Category: "predictability",
				File:     af.Path,
				Message:  fmt.Sprintf("file has %d init() function(s) (prefer explicit initialization)", af.InitFunctions),
			})
		}
	}

	return issues
}
