package detector_test

import (
	"testing"

	"github.com/openkraft/openkraft/internal/adapters/outbound/detector"
	"github.com/openkraft/openkraft/internal/adapters/outbound/scanner"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const fixtureDir = "../../../../testdata/go-hexagonal/perfect"

func scanFixture(t *testing.T) *scanner.FileScanner {
	t.Helper()
	return scanner.New()
}

func TestModuleDetector_FindsAllModules(t *testing.T) {
	s := scanFixture(t)
	scanResult, err := s.Scan(fixtureDir)
	require.NoError(t, err)

	d := detector.New()
	modules, err := d.Detect(scanResult)
	require.NoError(t, err)

	names := make([]string, len(modules))
	for i, m := range modules {
		names[i] = m.Name
	}
	assert.Contains(t, names, "tax")
	assert.Contains(t, names, "inventory")
	assert.Contains(t, names, "payments")
	assert.Len(t, modules, 3)
}

func TestModuleDetector_TaxHasAllLayers(t *testing.T) {
	s := scanFixture(t)
	scanResult, err := s.Scan(fixtureDir)
	require.NoError(t, err)

	d := detector.New()
	modules, err := d.Detect(scanResult)
	require.NoError(t, err)

	var tax *detector.DetectedModuleResult
	for i := range modules {
		if modules[i].Name == "tax" {
			tax = &modules[i]
			break
		}
	}
	require.NotNil(t, tax, "tax module should be found")
	assert.Contains(t, tax.Layers, "domain")
	assert.Contains(t, tax.Layers, "application")
	assert.Contains(t, tax.Layers, "adapters/http")
	assert.Contains(t, tax.Layers, "adapters/repository")
}

func TestModuleDetector_PaymentsOnlyHasDomain(t *testing.T) {
	s := scanFixture(t)
	scanResult, err := s.Scan(fixtureDir)
	require.NoError(t, err)

	d := detector.New()
	modules, err := d.Detect(scanResult)
	require.NoError(t, err)

	var payments *detector.DetectedModuleResult
	for i := range modules {
		if modules[i].Name == "payments" {
			payments = &modules[i]
			break
		}
	}
	require.NotNil(t, payments, "payments module should be found")
	assert.Equal(t, []string{"domain"}, payments.Layers)
}

func TestModuleDetector_FilesAssigned(t *testing.T) {
	s := scanFixture(t)
	scanResult, err := s.Scan(fixtureDir)
	require.NoError(t, err)

	d := detector.New()
	modules, err := d.Detect(scanResult)
	require.NoError(t, err)

	for _, m := range modules {
		assert.True(t, len(m.Files) > 0, "module %s should have files", m.Name)
	}
}
