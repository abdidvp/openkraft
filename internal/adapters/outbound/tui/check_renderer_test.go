package tui_test

import (
	"testing"

	"github.com/abdidvp/openkraft/internal/adapters/outbound/tui"
	"github.com/abdidvp/openkraft/internal/domain"
	"github.com/stretchr/testify/assert"
)

func sampleCheckReport() *domain.CheckReport {
	return &domain.CheckReport{
		Module:       "payments",
		GoldenModule: "tax",
		Score:        23,
		MissingFiles: []domain.MissingItem{
			{Name: "domain/payment_test.go", Expected: "unit tests", File: "domain/payment_test.go"},
			{Name: "application/payment_service.go", Expected: "service", File: "application/payment_service.go"},
		},
		MissingStructs: []domain.MissingItem{
			{Name: "Validate() error", File: "domain/payment.go", Description: "missing validation method"},
		},
		MissingMethods: []domain.MissingItem{
			{Name: "GetByID", File: "domain/repository.go", Expected: "repository method"},
		},
		MissingInterfaces: []domain.MissingItem{
			{Name: "PaymentRepository", File: "domain/ports.go", Expected: "port interface"},
		},
		PatternViolations: []domain.MissingItem{
			{Name: "No repository with getQuerier pattern", Description: "missing querier pattern"},
		},
	}
}

func TestRenderCheckReport_ContainsModuleName(t *testing.T) {
	output := tui.RenderCheckReport(sampleCheckReport())
	assert.Contains(t, output, "payments")
}

func TestRenderCheckReport_ContainsScore(t *testing.T) {
	output := tui.RenderCheckReport(sampleCheckReport())
	assert.Contains(t, output, "23")
	assert.Contains(t, output, "100")
}

func TestRenderCheckReport_ContainsGoldenModule(t *testing.T) {
	output := tui.RenderCheckReport(sampleCheckReport())
	assert.Contains(t, output, "tax")
}

func TestRenderCheckReport_ContainsMissingFilesSection(t *testing.T) {
	output := tui.RenderCheckReport(sampleCheckReport())
	assert.Contains(t, output, "Missing Files")
}

func TestRenderCheckReport_ContainsMissingFileNames(t *testing.T) {
	output := tui.RenderCheckReport(sampleCheckReport())
	assert.Contains(t, output, "domain/payment_test.go")
	assert.Contains(t, output, "application/payment_service.go")
}

func TestRenderCheckReport_ContainsMissingStructs(t *testing.T) {
	output := tui.RenderCheckReport(sampleCheckReport())
	assert.Contains(t, output, "Missing Structures")
	assert.Contains(t, output, "Validate() error")
}

func TestRenderCheckReport_ContainsPatternViolations(t *testing.T) {
	output := tui.RenderCheckReport(sampleCheckReport())
	assert.Contains(t, output, "Pattern Violations")
	assert.Contains(t, output, "No repository with getQuerier pattern")
}

func TestRenderCheckReport_EmptyReport(t *testing.T) {
	report := &domain.CheckReport{
		Module:       "empty",
		GoldenModule: "ref",
		Score:        100,
	}
	output := tui.RenderCheckReport(report)
	assert.Contains(t, output, "empty")
	assert.Contains(t, output, "100")
	assert.NotContains(t, output, "Missing Files")
	assert.NotContains(t, output, "Missing Structures")
	assert.NotContains(t, output, "Pattern Violations")
}

func TestRenderCheckReport_ContainsMissingMethods(t *testing.T) {
	output := tui.RenderCheckReport(sampleCheckReport())
	assert.Contains(t, output, "Missing Methods")
	assert.Contains(t, output, "GetByID")
}

func TestRenderCheckReport_ContainsMissingInterfaces(t *testing.T) {
	output := tui.RenderCheckReport(sampleCheckReport())
	assert.Contains(t, output, "Missing Interfaces")
	assert.Contains(t, output, "PaymentRepository")
}
