package domain_test

import (
	"encoding/json"
	"testing"

	"github.com/abdidvp/openkraft/internal/domain"
	"github.com/stretchr/testify/assert"
)

func TestFixPlan_JSONRoundTrip(t *testing.T) {
	original := domain.FixPlan{
		Applied: []domain.AppliedFix{
			{Type: "rename", Path: "internal/handler.go", Description: "renamed function to follow convention"},
		},
		Instructions: []domain.Instruction{
			{Type: "manual", File: "internal/service.go", Line: 42, Message: "extract method", Priority: "high", ProjectNorm: "function_lines"},
		},
		ScoreBefore: 65,
		ScoreAfter:  78,
	}

	data, err := json.Marshal(original)
	assert.NoError(t, err)

	var decoded domain.FixPlan
	err = json.Unmarshal(data, &decoded)
	assert.NoError(t, err)

	assert.Equal(t, original.ScoreBefore, decoded.ScoreBefore)
	assert.Equal(t, original.ScoreAfter, decoded.ScoreAfter)
	assert.Len(t, decoded.Applied, 1)
	assert.Equal(t, original.Applied[0].Type, decoded.Applied[0].Type)
	assert.Equal(t, original.Applied[0].Path, decoded.Applied[0].Path)
	assert.Equal(t, original.Applied[0].Description, decoded.Applied[0].Description)
	assert.Len(t, decoded.Instructions, 1)
	assert.Equal(t, original.Instructions[0].Type, decoded.Instructions[0].Type)
	assert.Equal(t, original.Instructions[0].File, decoded.Instructions[0].File)
	assert.Equal(t, original.Instructions[0].Line, decoded.Instructions[0].Line)
	assert.Equal(t, original.Instructions[0].Message, decoded.Instructions[0].Message)
	assert.Equal(t, original.Instructions[0].Priority, decoded.Instructions[0].Priority)
	assert.Equal(t, original.Instructions[0].ProjectNorm, decoded.Instructions[0].ProjectNorm)
}
