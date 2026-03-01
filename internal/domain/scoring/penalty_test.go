package scoring

import (
	"testing"

	"github.com/abdidvp/openkraft/internal/domain"
	"github.com/stretchr/testify/assert"
)

func TestDecayCredit_AtThreshold(t *testing.T) {
	assert.Equal(t, 1.0, decayCredit(50, 50))
}

func TestDecayCredit_BelowThreshold(t *testing.T) {
	assert.Equal(t, 1.0, decayCredit(30, 50))
}

func TestDecayCredit_AboveThreshold(t *testing.T) {
	credit := decayCredit(100, 50)
	assert.Greater(t, credit, 0.0)
	assert.Less(t, credit, 1.0)
}

func TestDecayCredit_AtFiveXThreshold(t *testing.T) {
	// At threshold*(decayK+1) = 50*5 = 250, credit should be 0.
	assert.Equal(t, 0.0, decayCredit(250, 50))
}

func TestDecayCredit_BeyondFiveX(t *testing.T) {
	assert.Equal(t, 0.0, decayCredit(300, 50))
}

func TestSeverityPenalty_NoIssues(t *testing.T) {
	assert.Equal(t, 0, severityPenalty(nil, 100))
}

func TestSeverityPenalty_ZeroFuncCount(t *testing.T) {
	issues := []domain.Issue{{Severity: domain.SeverityError}}
	assert.Equal(t, 0, severityPenalty(issues, 0))
}

func TestSeverityPenalty_ErrorFloor(t *testing.T) {
	// Single error in a large codebase: floor guarantees >= 1.
	issues := []domain.Issue{{Severity: domain.SeverityError}}
	p := severityPenalty(issues, 1000)
	assert.GreaterOrEqual(t, p, 1)
}

func TestSeverityPenalty_InfoLowWeight(t *testing.T) {
	issues := []domain.Issue{{Severity: domain.SeverityInfo}}
	p := severityPenalty(issues, 100)
	// 0.2/100 * 120 = 0.24 → rounds to 0
	assert.Equal(t, 0, p)
}

func TestIssueSeverity_Error(t *testing.T) {
	assert.Equal(t, domain.SeverityError, issueSeverity(150, 50))
}

func TestIssueSeverity_Warning(t *testing.T) {
	assert.Equal(t, domain.SeverityWarning, issueSeverity(80, 50))
}

func TestIssueSeverity_Info(t *testing.T) {
	assert.Equal(t, domain.SeverityInfo, issueSeverity(60, 50))
}

func TestIssueSeverity_ZeroThreshold(t *testing.T) {
	assert.Equal(t, domain.SeverityWarning, issueSeverity(10, 0))
}
