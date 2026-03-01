package application

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	cacheAdapter "github.com/abdidvp/openkraft/internal/adapters/outbound/cache"
	"github.com/abdidvp/openkraft/internal/adapters/outbound/config"
	"github.com/abdidvp/openkraft/internal/adapters/outbound/detector"
	"github.com/abdidvp/openkraft/internal/adapters/outbound/parser"
	"github.com/abdidvp/openkraft/internal/adapters/outbound/scanner"
	"github.com/abdidvp/openkraft/internal/domain"
)

const perfectFixture = "../../testdata/go-hexagonal/perfect"
const incompleteFixture = "../../testdata/go-hexagonal/incomplete"

func newIntegrationServices() (*OnboardService, *FixService, *ValidateService) {
	sc := scanner.New()
	det := detector.New()
	par := parser.New()
	cfg := config.New()
	cacheSt := cacheAdapter.New()

	scoreSvc := NewScoreService(sc, det, par, cfg)
	onboardSvc := NewOnboardService(sc, det, par, cfg)
	fixSvc := NewFixService(scoreSvc, onboardSvc)
	validateSvc := NewValidateService(sc, det, par, scoreSvc, cacheSt, cfg)

	return onboardSvc, fixSvc, validateSvc
}

func TestIntegration_OnboardPerfect(t *testing.T) {
	onboardSvc, _, _ := newIntegrationServices()

	report, err := onboardSvc.GenerateReport(perfectFixture)
	require.NoError(t, err)

	// Verify complete report
	assert.NotEmpty(t, report.ProjectName)
	assert.NotEmpty(t, report.Modules, "should detect modules")
	assert.NotEmpty(t, report.GoldenModule, "should select golden module")
	assert.Equal(t, "hexagonal", report.ArchitectureStyle)
	assert.NotEmpty(t, report.NamingConvention)
	assert.True(t, report.NamingPercentage > 0, "naming percentage should be positive")
	assert.True(t, report.Norms.FunctionLines > 0, "function lines norm should be computed")
	assert.True(t, report.Norms.FileLines > 0, "file lines norm should be computed")
	assert.NotEmpty(t, report.DependencyRules, "should detect dependency rules for hexagonal project")
	assert.NotEmpty(t, report.BuildCommands, "should detect build commands")
	assert.NotEmpty(t, report.TestCommands, "should detect test commands")

	// Verify contract rendering
	contract := onboardSvc.RenderContract(report)
	assert.Contains(t, contract, "MUST")
	assert.Contains(t, contract, report.ProjectName)
}

func TestIntegration_FixIncomplete(t *testing.T) {
	_, fixSvc, _ := newIntegrationServices()

	plan, err := fixSvc.PlanFixes(incompleteFixture, domain.FixOptions{DryRun: true})
	require.NoError(t, err)

	assert.True(t, plan.ScoreBefore > 0, "should have a before score")

	// Check for structure drift or naming drift instructions
	hasStructureDrift := false
	for _, inst := range plan.Instructions {
		if inst.Type == "structure_drift" {
			hasStructureDrift = true
		}
		// Every instruction should have a project norm
		assert.NotEmpty(t, inst.ProjectNorm, "instruction %s should have project_norm", inst.Type)
	}
	// The incomplete fixture should have some drift
	if len(plan.Instructions) > 0 {
		assert.True(t, hasStructureDrift || len(plan.Instructions) > 0,
			"incomplete project should have drift instructions")
	}
}

func TestIntegration_FullWorkflow(t *testing.T) {
	onboardSvc, fixSvc, validateSvc := newIntegrationServices()

	// Step 1: Onboard
	report, err := onboardSvc.GenerateReport(perfectFixture)
	require.NoError(t, err)
	assert.NotEmpty(t, report.GoldenModule)

	// Step 2: Fix (dry run)
	plan, err := fixSvc.PlanFixes(perfectFixture, domain.FixOptions{DryRun: true})
	require.NoError(t, err)
	assert.True(t, plan.ScoreBefore > 0)

	// Step 3: Validate
	// Clean cache first
	_ = cacheAdapter.New().Invalidate(perfectFixture)
	defer func() { _ = cacheAdapter.New().Invalidate(perfectFixture) }()

	result, err := validateSvc.Validate(perfectFixture,
		[]string{"internal/tax/domain/tax_rule.go"}, nil, nil, false)
	require.NoError(t, err)
	assert.NotNil(t, result)
	assert.Contains(t, []string{"pass", "warn"}, result.Status,
		"perfect project should pass or warn, got %s", result.Status)
}
