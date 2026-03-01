package scoring

import (
	"math"

	"github.com/abdidvp/openkraft/internal/domain"
)

// decayK controls how gradually credit decays past the threshold.
// With k=4, credit reaches zero at threshold*5 (5x threshold).
// Calibrated alongside severityPenaltyScale=120 to produce industry-aligned
// scores (88-98 for well-maintained OSS projects).
const decayK = 4

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
