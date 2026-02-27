package application_test

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/openkraft/openkraft/internal/adapters/outbound/config"
	"github.com/openkraft/openkraft/internal/adapters/outbound/detector"
	"github.com/openkraft/openkraft/internal/adapters/outbound/parser"
	"github.com/openkraft/openkraft/internal/adapters/outbound/scanner"
	"github.com/openkraft/openkraft/internal/application"
	"github.com/openkraft/openkraft/internal/domain"
)

func newOnboardService() *application.OnboardService {
	return application.NewOnboardService(
		scanner.New(),
		detector.New(),
		parser.New(),
		config.New(),
	)
}

func TestOnboard_PerfectProject(t *testing.T) {
	svc := newOnboardService()
	report, err := svc.GenerateReport(fixtureDir)
	require.NoError(t, err)

	assert.NotEmpty(t, report.ProjectName)
	assert.NotEmpty(t, report.Modules)
	assert.NotEmpty(t, report.GoldenModule)
	assert.NotEmpty(t, report.NamingConvention)
	assert.True(t, report.NamingPercentage > 0)
	assert.True(t, report.Norms.FunctionLines > 0)
	assert.NotEmpty(t, report.DependencyRules)
	assert.Equal(t, "hexagonal", report.ArchitectureStyle)
}

func TestOnboard_EmptyProject(t *testing.T) {
	svc := newOnboardService()
	report, err := svc.GenerateReport("../../testdata/go-hexagonal/empty")
	require.NoError(t, err)

	// Should handle gracefully (no golden module, minimal data)
	assert.NotEmpty(t, report.ProjectName)
	assert.Empty(t, report.GoldenModule) // no modules to select from
}

func TestRenderContract_Populated(t *testing.T) {
	svc := newOnboardService()
	report := &domain.OnboardReport{
		ProjectName:       "testproject",
		ProjectType:       "api",
		ArchitectureStyle: "hexagonal",
		LayoutStyle:       domain.LayoutPerFeature,
		NamingConvention:  "bare",
		NamingPercentage:  0.92,
		GoldenModule:      "internal/scoring",
		ModuleBlueprint:   []string{"domain", "application", "adapters"},
		Modules: []domain.DetectedModule{
			{Name: "scoring", Path: "internal/scoring", Layers: []string{"domain", "application"}},
		},
		DependencyRules: []domain.DependencyRule{
			{Source: "domain", Forbids: "adapters", Reason: "domain must not import adapters"},
		},
		Interfaces: []domain.InterfaceMapping{
			{Interface: "ScoreHistory", Implementation: "JSONHistory", Package: "history"},
		},
		BuildCommands: []string{"go build ./..."},
		TestCommands:  []string{"go test ./..."},
		Norms: domain.ProjectNorms{
			FunctionLines: 50,
			FileLines:     300,
			Parameters:    4,
		},
	}

	contract := svc.RenderContract(report)

	// Must contain prescriptive language
	assert.True(t, strings.Contains(contract, "MUST") || strings.Contains(contract, "ALWAYS") || strings.Contains(contract, "NEVER"),
		"contract should contain prescriptive language")

	// Must be under 200 lines
	lines := strings.Count(contract, "\n") + 1
	assert.LessOrEqual(t, lines, 200, "contract should be under 200 lines")

	// Verify key sections present
	assert.Contains(t, contract, "testproject")
	assert.Contains(t, contract, "hexagonal")
	assert.Contains(t, contract, "internal/scoring")
}

func TestRenderContract_EmptyModules(t *testing.T) {
	svc := newOnboardService()
	report := &domain.OnboardReport{
		ProjectName:       "testproject",
		ProjectType:       "api",
		ArchitectureStyle: "flat",
		NamingConvention:  "bare",
		NamingPercentage:  1.0,
		Norms: domain.ProjectNorms{
			FunctionLines: 30,
			FileLines:     200,
			Parameters:    3,
		},
	}

	contract := svc.RenderContract(report)
	// Module section should be omitted
	assert.NotContains(t, contract, "| Module |")
}

func TestRenderJSON_RoundTrip(t *testing.T) {
	svc := newOnboardService()
	report := &domain.OnboardReport{
		ProjectName:       "testproject",
		ProjectType:       "api",
		ArchitectureStyle: "hexagonal",
		NamingConvention:  "bare",
		NamingPercentage:  0.9,
		Norms: domain.ProjectNorms{
			FunctionLines: 50,
			FileLines:     300,
			Parameters:    4,
		},
	}

	data, err := svc.RenderJSON(report)
	require.NoError(t, err)

	var decoded domain.OnboardReport
	err = json.Unmarshal(data, &decoded)
	require.NoError(t, err)
	assert.Equal(t, report.ProjectName, decoded.ProjectName)
	assert.Equal(t, report.NamingConvention, decoded.NamingConvention)
}
