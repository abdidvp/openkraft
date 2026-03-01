package check_test

import (
	"path/filepath"
	"testing"

	"github.com/abdidvp/openkraft/internal/adapters/outbound/detector"
	"github.com/abdidvp/openkraft/internal/adapters/outbound/parser"
	"github.com/abdidvp/openkraft/internal/adapters/outbound/scanner"
	"github.com/abdidvp/openkraft/internal/domain"
	"github.com/abdidvp/openkraft/internal/domain/check"
	"github.com/abdidvp/openkraft/internal/domain/golden"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const fixtureDir = "../../../testdata/go-hexagonal/perfect"

// loadFixture scans, detects, and parses the perfect hexagonal fixture.
func loadFixture(t *testing.T) ([]domain.DetectedModule, map[string]*domain.AnalyzedFile) {
	t.Helper()

	scan, err := scanner.New().Scan(fixtureDir)
	require.NoError(t, err)

	modules, err := detector.New().Detect(scan)
	require.NoError(t, err)

	p := parser.New()
	analyzed := make(map[string]*domain.AnalyzedFile)
	for _, f := range scan.GoFiles {
		af, err := p.AnalyzeFile(filepath.Join(fixtureDir, f))
		if err != nil {
			continue
		}
		analyzed[f] = af
	}

	return modules, analyzed
}

func findModule(t *testing.T, modules []domain.DetectedModule, name string) domain.DetectedModule {
	t.Helper()
	for _, m := range modules {
		if m.Name == name {
			return m
		}
	}
	t.Fatalf("module %q not found", name)
	return domain.DetectedModule{}
}

// TestCheckModule_TaxAgainstItself checks that a golden module compared
// against its own blueprint scores ~100 (perfect or near-perfect).
func TestCheckModule_TaxAgainstItself(t *testing.T) {
	modules, analyzed := loadFixture(t)
	taxModule := findModule(t, modules, "tax")

	bp, err := golden.ExtractBlueprint(taxModule, analyzed)
	require.NoError(t, err)

	report := check.CheckModule(taxModule, bp, analyzed)

	require.NotNil(t, report)
	assert.Equal(t, "tax", report.Module)
	assert.Equal(t, "tax", report.GoldenModule)
	assert.GreaterOrEqual(t, report.Score, 90, "tax checked against itself should score >= 90, got %d", report.Score)
	assert.Empty(t, report.MissingFiles, "tax should have no missing files against its own blueprint")
	assert.Empty(t, report.MissingStructs, "tax should have no missing structs")
	assert.Empty(t, report.MissingInterfaces, "tax should have no missing interfaces")
}

// TestCheckModule_PaymentsAgainstTax checks that the incomplete payments
// module scores poorly against the tax blueprint.
func TestCheckModule_PaymentsAgainstTax(t *testing.T) {
	modules, analyzed := loadFixture(t)
	paymentsModule := findModule(t, modules, "payments")
	taxModule := findModule(t, modules, "tax")

	bp, err := golden.ExtractBlueprint(taxModule, analyzed)
	require.NoError(t, err)

	report := check.CheckModule(paymentsModule, bp, analyzed)

	require.NotNil(t, report)
	assert.Equal(t, "payments", report.Module)
	assert.Equal(t, "tax", report.GoldenModule)

	// Payments only has domain/payment.go -- it should be missing many files
	assert.Less(t, report.Score, 30, "payments should score < 30, got %d", report.Score)
	assert.NotEmpty(t, report.MissingFiles, "payments should have missing files")

	// Should report missing application layer files
	hasMissingAppFile := false
	for _, mf := range report.MissingFiles {
		if mf.Expected == "service" || mf.Expected == "ports" {
			hasMissingAppFile = true
			break
		}
	}
	assert.True(t, hasMissingAppFile, "should report missing application layer files")

	// Should report missing adapter files
	hasMissingAdapterFile := false
	for _, mf := range report.MissingFiles {
		if mf.Expected == "handler" || mf.Expected == "repository" {
			hasMissingAdapterFile = true
			break
		}
	}
	assert.True(t, hasMissingAdapterFile, "should report missing adapter files")

	// Should report missing test file
	hasMissingTestFile := false
	for _, mf := range report.MissingFiles {
		if mf.Expected == "domain_test" {
			hasMissingTestFile = true
			break
		}
	}
	assert.True(t, hasMissingTestFile, "should report missing test file")

	// Should report missing Validate method
	hasMissingValidate := false
	for _, mm := range report.MissingMethods {
		if mm.Name == "Validate" {
			hasMissingValidate = true
			break
		}
	}
	assert.True(t, hasMissingValidate, "should report missing Validate method")
}

// TestCheckModule_InventoryAgainstTax checks that inventory (mostly complete)
// scores higher than payments but may have minor differences.
func TestCheckModule_InventoryAgainstTax(t *testing.T) {
	modules, analyzed := loadFixture(t)
	inventoryModule := findModule(t, modules, "inventory")
	taxModule := findModule(t, modules, "tax")

	bp, err := golden.ExtractBlueprint(taxModule, analyzed)
	require.NoError(t, err)

	report := check.CheckModule(inventoryModule, bp, analyzed)
	require.NotNil(t, report)

	// Inventory is mostly complete -- should score much higher than payments
	assert.Greater(t, report.Score, 50, "inventory should score > 50, got %d", report.Score)
}

// TestCheckModule_ScoreMinimumIsZero ensures score never goes negative.
func TestCheckModule_ScoreMinimumIsZero(t *testing.T) {
	// A module with zero files against a full blueprint
	emptyModule := domain.DetectedModule{
		Name: "empty",
		Path: "internal/empty",
	}
	bp := &domain.Blueprint{
		Name:          "tax",
		ExtractedFrom: "internal/tax",
		Files: []domain.BlueprintFile{
			{PathPattern: "domain/{entity}.go", Type: "domain_entity", Required: true, RequiredStructs: []string{"{Entity}"}, RequiredMethods: []string{"Validate"}, RequiredFunctions: []string{"New{Entity}"}},
			{PathPattern: "domain/{entity}_test.go", Type: "domain_test", Required: true},
			{PathPattern: "domain/{entity}_errors.go", Type: "domain_errors", Required: true},
			{PathPattern: "application/{module}_service.go", Type: "service", Required: true},
			{PathPattern: "application/{module}_ports.go", Type: "ports", Required: true},
			{PathPattern: "adapters/http/{module}_handler.go", Type: "handler", Required: true},
			{PathPattern: "adapters/http/{module}_routes.go", Type: "routes", Required: true},
			{PathPattern: "adapters/repository/{entity}_repository.go", Type: "repository", Required: true},
		},
		Patterns: []string{"domain", "application", "adapters/http", "adapters/repository"},
	}
	analyzed := make(map[string]*domain.AnalyzedFile)

	report := check.CheckModule(emptyModule, bp, analyzed)
	assert.GreaterOrEqual(t, report.Score, 0, "score should never be negative")
}

// TestCheckModule_ReportHasIssues checks that issues are populated for missing items.
func TestCheckModule_ReportHasIssues(t *testing.T) {
	modules, analyzed := loadFixture(t)
	paymentsModule := findModule(t, modules, "payments")
	taxModule := findModule(t, modules, "tax")

	bp, err := golden.ExtractBlueprint(taxModule, analyzed)
	require.NoError(t, err)

	report := check.CheckModule(paymentsModule, bp, analyzed)
	assert.NotEmpty(t, report.Issues, "should have issues for missing items")
}

// TestCheckModule_PatternViolations checks pattern compliance reporting.
func TestCheckModule_PatternViolations(t *testing.T) {
	modules, analyzed := loadFixture(t)
	paymentsModule := findModule(t, modules, "payments")
	taxModule := findModule(t, modules, "tax")

	bp, err := golden.ExtractBlueprint(taxModule, analyzed)
	require.NoError(t, err)

	report := check.CheckModule(paymentsModule, bp, analyzed)

	// Payments has no Validate, no constructor, no interfaces -- should have pattern violations
	assert.NotEmpty(t, report.PatternViolations, "payments should have pattern violations")
}
