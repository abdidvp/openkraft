package application_test

import (
	"testing"

	"github.com/openkraft/openkraft/internal/adapters/outbound/config"
	"github.com/openkraft/openkraft/internal/adapters/outbound/detector"
	"github.com/openkraft/openkraft/internal/adapters/outbound/parser"
	"github.com/openkraft/openkraft/internal/adapters/outbound/scanner"
	"github.com/openkraft/openkraft/internal/application"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newCheckService() *application.CheckService {
	return application.NewCheckService(
		scanner.New(),
		detector.New(),
		parser.New(),
		config.New(),
	)
}

func TestCheckService_CheckModule(t *testing.T) {
	svc := newCheckService()

	report, err := svc.CheckModule(fixtureDir, "payments")
	require.NoError(t, err)

	assert.Equal(t, "payments", report.Module)
	assert.NotEmpty(t, report.GoldenModule)
	assert.True(t, report.Score >= 0 && report.Score <= 100)
}

func TestCheckService_CheckModule_NotFound(t *testing.T) {
	svc := newCheckService()

	_, err := svc.CheckModule(fixtureDir, "nonexistent")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "nonexistent")
}

func TestCheckService_CheckModule_InvalidPath(t *testing.T) {
	svc := newCheckService()

	_, err := svc.CheckModule("/nonexistent/path", "payments")
	assert.Error(t, err)
}

func TestCheckService_CheckAll(t *testing.T) {
	svc := newCheckService()

	reports, err := svc.CheckAll(fixtureDir)
	require.NoError(t, err)

	// The fixture has tax (golden), inventory, and payments.
	// CheckAll should return reports for all non-golden modules.
	assert.True(t, len(reports) >= 1, "should have at least 1 check report")

	for _, r := range reports {
		assert.NotEmpty(t, r.Module)
		assert.NotEmpty(t, r.GoldenModule)
		assert.True(t, r.Score >= 0 && r.Score <= 100)
	}
}

func TestCheckService_CheckAll_InvalidPath(t *testing.T) {
	svc := newCheckService()

	_, err := svc.CheckAll("/nonexistent/path")
	assert.Error(t, err)
}

func TestCheckService_CheckModule_PaymentsHasMissingItems(t *testing.T) {
	svc := newCheckService()

	report, err := svc.CheckModule(fixtureDir, "payments")
	require.NoError(t, err)

	// payments module is incomplete compared to the golden module (tax)
	// It should have missing files, methods, etc.
	totalMissing := len(report.MissingFiles) + len(report.MissingStructs) +
		len(report.MissingMethods) + len(report.MissingInterfaces)
	assert.True(t, totalMissing > 0, "payments should have missing items compared to golden")
}
