package domain_test

import (
	"encoding/json"
	"testing"

	"github.com/abdidvp/openkraft/internal/domain"
	"github.com/stretchr/testify/assert"
)

func TestValidationResult_JSONRoundTrip(t *testing.T) {
	original := domain.ValidationResult{
		Status:       "drift_detected",
		FilesChecked: []string{"handler.go", "service.go"},
		DriftIssues: []domain.DriftIssue{
			{File: "handler.go", Line: 10, Severity: "warning", Message: "function too long", Category: "code_health", DriftType: "norm_violation"},
		},
		ScoreImpact: domain.ScoreImpact{
			Overall:    -5,
			Categories: map[string]int{"code_health": -3, "structure": -2},
		},
		Suggestions: []string{"split function into smaller units"},
	}

	data, err := json.Marshal(original)
	assert.NoError(t, err)

	var decoded domain.ValidationResult
	err = json.Unmarshal(data, &decoded)
	assert.NoError(t, err)

	assert.Equal(t, original.Status, decoded.Status)
	assert.Equal(t, original.FilesChecked, decoded.FilesChecked)
	assert.Len(t, decoded.DriftIssues, 1)
	assert.Equal(t, original.DriftIssues[0].File, decoded.DriftIssues[0].File)
	assert.Equal(t, original.DriftIssues[0].Line, decoded.DriftIssues[0].Line)
	assert.Equal(t, original.DriftIssues[0].Severity, decoded.DriftIssues[0].Severity)
	assert.Equal(t, original.DriftIssues[0].Message, decoded.DriftIssues[0].Message)
	assert.Equal(t, original.DriftIssues[0].Category, decoded.DriftIssues[0].Category)
	assert.Equal(t, original.DriftIssues[0].DriftType, decoded.DriftIssues[0].DriftType)
	assert.Equal(t, original.ScoreImpact.Overall, decoded.ScoreImpact.Overall)
	assert.Equal(t, original.ScoreImpact.Categories, decoded.ScoreImpact.Categories)
	assert.Equal(t, original.Suggestions, decoded.Suggestions)
}
