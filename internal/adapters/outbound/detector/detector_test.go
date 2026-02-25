package detector_test

import (
	"testing"

	"github.com/openkraft/openkraft/internal/adapters/outbound/detector"
	"github.com/openkraft/openkraft/internal/adapters/outbound/scanner"
	"github.com/openkraft/openkraft/internal/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const fixtureDir = "../../../../testdata/go-hexagonal/perfect"

func scanFixture(t *testing.T) *scanner.FileScanner {
	t.Helper()
	return scanner.New()
}

// --- Per-Feature Layout (fixture) ---

func TestModuleDetector_PerFeature_FindsAllModules(t *testing.T) {
	s := scanFixture(t)
	scanResult, err := s.Scan(fixtureDir)
	require.NoError(t, err)

	d := detector.New()
	modules, err := d.Detect(scanResult)
	require.NoError(t, err)

	assert.Equal(t, domain.LayoutPerFeature, scanResult.Layout)

	names := make([]string, len(modules))
	for i, m := range modules {
		names[i] = m.Name
	}
	assert.Contains(t, names, "tax")
	assert.Contains(t, names, "inventory")
	assert.Contains(t, names, "payments")
	assert.Len(t, modules, 3)
}

func TestModuleDetector_PerFeature_TaxHasAllLayers(t *testing.T) {
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
	assert.Contains(t, tax.Layers, "adapters")
}

func TestModuleDetector_PerFeature_PaymentsOnlyHasDomain(t *testing.T) {
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

func TestModuleDetector_PerFeature_FilesAssigned(t *testing.T) {
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

// --- Cross-Cutting Layout ---

func TestModuleDetector_CrossCutting_DetectsLayout(t *testing.T) {
	scan := &domain.ScanResult{
		GoFiles: []string{
			"internal/domain/ports.go",
			"internal/domain/model.go",
			"internal/domain/scoring/code_health.go",
			"internal/domain/scoring/structure.go",
			"internal/domain/check/checker.go",
			"internal/application/score_service.go",
			"internal/application/check_service.go",
			"internal/adapters/outbound/scanner/scanner.go",
			"internal/adapters/outbound/parser/go_parser.go",
			"internal/adapters/inbound/cli/root.go",
			"internal/adapters/inbound/cli/score.go",
		},
	}

	d := detector.New()
	modules, err := d.Detect(scan)
	require.NoError(t, err)

	assert.Equal(t, domain.LayoutCrossCutting, scan.Layout)

	names := map[string]bool{}
	for _, m := range modules {
		names[m.Name] = true
	}
	assert.True(t, names["scoring"], "should detect 'scoring' as a module")
	assert.True(t, names["check"], "should detect 'check' as a module")
	assert.True(t, names["scanner"], "should detect 'scanner' as a module")
	assert.True(t, names["cli"], "should detect 'cli' as a module")
	assert.True(t, names["domain"], "root domain files should form a module")
	assert.True(t, names["application"], "root application files should form a module")
}

func TestModuleDetector_CrossCutting_ModuleHasCorrectLayer(t *testing.T) {
	scan := &domain.ScanResult{
		GoFiles: []string{
			"internal/domain/scoring/code_health.go",
			"internal/adapters/outbound/scanner/scanner.go",
		},
	}

	d := detector.New()
	modules, err := d.Detect(scan)
	require.NoError(t, err)

	for _, m := range modules {
		if m.Name == "scoring" {
			assert.Contains(t, m.Layers, "domain")
		}
		if m.Name == "scanner" {
			assert.Contains(t, m.Layers, "adapters")
		}
	}
}
