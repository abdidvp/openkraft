package domain_test

import (
	"encoding/json"
	"testing"

	"github.com/openkraft/openkraft/internal/domain"
	"github.com/stretchr/testify/assert"
)

func TestOnboardReport_JSONRoundTrip(t *testing.T) {
	original := domain.OnboardReport{
		ProjectName:       "myproject",
		ProjectType:       "service",
		ArchitectureStyle: "hexagonal",
		LayoutStyle:       domain.LayoutPerFeature,
		Modules: []domain.DetectedModule{
			{Name: "payments", Path: "internal/payments", Layers: []string{"domain", "handler"}, Files: []string{"handler.go"}},
		},
		NamingConvention: "snake_case",
		NamingPercentage: 85.5,
		GoldenModule:     "payments",
		ModuleBlueprint:  []string{"handler.go", "service.go", "repository.go"},
		BuildCommands:    []string{"go build ./..."},
		TestCommands:     []string{"go test ./..."},
		DependencyRules: []domain.DependencyRule{
			{Source: "domain", Forbids: "handler", Reason: "domain must not depend on handler"},
		},
		Interfaces: []domain.InterfaceMapping{
			{Interface: "Repository", Implementation: "PostgresRepo", Package: "infra"},
		},
		Norms: domain.ProjectNorms{
			FunctionLines: 30,
			FileLines:     200,
			Parameters:    4,
			NamingStyle:   "camelCase",
			NamingPct:     92.0,
		},
	}

	data, err := json.Marshal(original)
	assert.NoError(t, err)

	var decoded domain.OnboardReport
	err = json.Unmarshal(data, &decoded)
	assert.NoError(t, err)

	assert.Equal(t, original.ProjectName, decoded.ProjectName)
	assert.Equal(t, original.ProjectType, decoded.ProjectType)
	assert.Equal(t, original.ArchitectureStyle, decoded.ArchitectureStyle)
	assert.Equal(t, original.LayoutStyle, decoded.LayoutStyle)
	assert.Len(t, decoded.Modules, 1)
	assert.Equal(t, original.Modules[0].Name, decoded.Modules[0].Name)
	assert.Equal(t, original.NamingConvention, decoded.NamingConvention)
	assert.InDelta(t, original.NamingPercentage, decoded.NamingPercentage, 0.001)
	assert.Equal(t, original.GoldenModule, decoded.GoldenModule)
	assert.Equal(t, original.ModuleBlueprint, decoded.ModuleBlueprint)
	assert.Equal(t, original.BuildCommands, decoded.BuildCommands)
	assert.Equal(t, original.TestCommands, decoded.TestCommands)
	assert.Len(t, decoded.DependencyRules, 1)
	assert.Equal(t, original.DependencyRules[0].Source, decoded.DependencyRules[0].Source)
	assert.Len(t, decoded.Interfaces, 1)
	assert.Equal(t, original.Interfaces[0].Interface, decoded.Interfaces[0].Interface)
	assert.Equal(t, original.Norms.FunctionLines, decoded.Norms.FunctionLines)
}
