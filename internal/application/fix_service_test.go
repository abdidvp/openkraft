package application_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/abdidvp/openkraft/internal/adapters/outbound/config"
	"github.com/abdidvp/openkraft/internal/adapters/outbound/detector"
	"github.com/abdidvp/openkraft/internal/adapters/outbound/parser"
	"github.com/abdidvp/openkraft/internal/adapters/outbound/scanner"
	"github.com/abdidvp/openkraft/internal/application"
	"github.com/abdidvp/openkraft/internal/domain"
)

func newFixService() *application.FixService {
	sc := scanner.New()
	det := detector.New()
	par := parser.New()
	cfg := config.New()
	scoreSvc := application.NewScoreService(sc, det, par, cfg)
	onboardSvc := application.NewOnboardService(sc, det, par, cfg)
	return application.NewFixService(scoreSvc, onboardSvc)
}

func TestFixPlanFixes_DryRun(t *testing.T) {
	svc := newFixService()
	plan, err := svc.PlanFixes(fixtureDir, domain.FixOptions{DryRun: true})
	require.NoError(t, err)
	assert.NotNil(t, plan)
	assert.True(t, plan.ScoreBefore > 0)
	// Dry run should not create any files on disk
}

func TestFixPlanFixes_AutoOnly(t *testing.T) {
	svc := newFixService()
	plan, err := svc.PlanFixes(fixtureDir, domain.FixOptions{DryRun: true, AutoOnly: true})
	require.NoError(t, err)
	assert.NotNil(t, plan)
	// AutoOnly should not generate drift instructions
	assert.Empty(t, plan.Instructions)
}

func TestFixInstruction_DependencyDrift(t *testing.T) {
	report := &domain.OnboardReport{
		GoldenModule:    "internal/scoring",
		ModuleBlueprint: []string{"domain", "application", "adapters"},
		Norms: domain.ProjectNorms{
			FunctionLines: 50,
			FileLines:     300,
			Parameters:    4,
			NamingStyle:   "bare",
			NamingPct:     0.92,
		},
	}

	issue := domain.Issue{
		Severity:  domain.SeverityError,
		Category:  "discoverability",
		SubMetric: "dependency_direction",
		File:      "internal/domain/user.go",
		Message:   "domain layer imports adapters/db (dependency direction violation)",
	}

	inst := application.ClassifyIssueAsInstruction(issue, "discoverability", report)
	require.NotNil(t, inst)
	assert.Equal(t, "dependency_drift", inst.Type)
	assert.Equal(t, "high", inst.Priority)
	assert.Contains(t, inst.ProjectNorm, "domain")
}

func TestFixInstruction_SizeDrift(t *testing.T) {
	report := &domain.OnboardReport{
		Norms: domain.ProjectNorms{
			FunctionLines: 50,
			FileLines:     300,
			Parameters:    4,
			NamingStyle:   "bare",
			NamingPct:     0.92,
		},
	}

	issue := domain.Issue{
		Severity:  domain.SeverityWarning,
		Category:  "code_health",
		SubMetric: "function_size",
		File:      "internal/orders/service.go",
		Line:      45,
		Message:   "function processOrder is 82 lines",
	}

	inst := application.ClassifyIssueAsInstruction(issue, "code_health", report)
	require.NotNil(t, inst)
	assert.Equal(t, "size_drift", inst.Type)
	assert.Equal(t, "medium", inst.Priority)
	assert.Contains(t, inst.ProjectNorm, "50")
}

func TestFixInstruction_StructureDrift(t *testing.T) {
	report := &domain.OnboardReport{
		GoldenModule:    "internal/scoring",
		ModuleBlueprint: []string{"domain", "application", "adapters"},
		Norms:           domain.ProjectNorms{NamingStyle: "bare", NamingPct: 0.9},
	}

	issue := domain.Issue{
		Severity: domain.SeverityWarning,
		Category: "structure",
		File:     "internal/auth/",
		Message:  "module has 2 layers but expected 4",
	}

	inst := application.ClassifyIssueAsInstruction(issue, "structure", report)
	require.NotNil(t, inst)
	assert.Equal(t, "structure_drift", inst.Type)
	assert.Equal(t, "high", inst.Priority)
	assert.Contains(t, inst.ProjectNorm, "golden")
}

func TestFixInstruction_PrioritySorting(t *testing.T) {
	svc := newFixService()
	plan, err := svc.PlanFixes("../../testdata/go-hexagonal/incomplete", domain.FixOptions{DryRun: true})
	require.NoError(t, err)

	// Instructions should be sorted by priority (high first)
	if len(plan.Instructions) >= 2 {
		for i := 1; i < len(plan.Instructions); i++ {
			assert.LessOrEqual(t, application.PriorityRank(plan.Instructions[i-1].Priority), application.PriorityRank(plan.Instructions[i].Priority),
				"instructions should be sorted by priority")
		}
	}
}

func TestFixPlanFixes_MissingCLAUDEmd(t *testing.T) {
	// The "empty" fixture has no CLAUDE.md
	svc := newFixService()
	plan, err := svc.PlanFixes("../../testdata/go-hexagonal/empty", domain.FixOptions{DryRun: true})
	require.NoError(t, err)

	// Check if CLAUDE.md fix is identified
	hasCLAUDEFix := false
	for _, fix := range plan.Applied {
		if fix.Path == "CLAUDE.md" {
			hasCLAUDEFix = true
			break
		}
	}
	assert.True(t, hasCLAUDEFix, "should identify missing CLAUDE.md as a fix")
}

func TestFixPlanFixes_CategoryFilter(t *testing.T) {
	svc := newFixService()
	plan, err := svc.PlanFixes("../../testdata/go-hexagonal/incomplete", domain.FixOptions{
		DryRun:   true,
		Category: "code_health",
	})
	require.NoError(t, err)

	// All instructions should be for code_health drift types
	for _, inst := range plan.Instructions {
		assert.True(t, inst.Type == "size_drift",
			"category filter should only return code_health drift types, got %s", inst.Type)
	}

	// Applied fixes should be empty since code_health has no safe fixes
	assert.Empty(t, plan.Applied, "code_health category should not produce safe fixes")
}
