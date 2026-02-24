package golden_test

import (
	"path/filepath"
	"testing"

	"github.com/openkraft/openkraft/internal/adapters/outbound/detector"
	"github.com/openkraft/openkraft/internal/adapters/outbound/parser"
	"github.com/openkraft/openkraft/internal/adapters/outbound/scanner"
	"github.com/openkraft/openkraft/internal/domain"
	"github.com/openkraft/openkraft/internal/domain/golden"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const fixtureDir = "../../../testdata/go-hexagonal/perfect"

// loadFixture scans, detects, and parses the perfect hexagonal fixture
// returning modules and analyzed files. Shared by selector and extractor tests.
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

func TestSelectGolden_TaxRanksFirst(t *testing.T) {
	modules, analyzed := loadFixture(t)

	result, err := golden.SelectGolden(modules, analyzed)
	require.NoError(t, err)
	require.NotNil(t, result)

	assert.Equal(t, "tax", result.Module.Name,
		"tax module should rank #1 (most files, most layers, has tests, Validate, constructors, errors)")
	assert.Greater(t, result.Score, 0.5,
		"golden module should have a meaningful score")
}

func TestSelectGolden_PaymentsRanksLast(t *testing.T) {
	modules, analyzed := loadFixture(t)

	result, err := golden.SelectGolden(modules, analyzed)
	require.NoError(t, err)
	require.NotNil(t, result)

	// Payments should NOT be the golden module.
	assert.NotEqual(t, "payments", result.Module.Name,
		"payments (1 file, 1 layer, no tests) should not be golden")
}

func TestSelectGolden_ScoreBreakdown(t *testing.T) {
	modules, analyzed := loadFixture(t)

	result, err := golden.SelectGolden(modules, analyzed)
	require.NoError(t, err)

	// The breakdown should contain all five scoring dimensions.
	expectedKeys := []string{
		"file_completeness",
		"structural_depth",
		"test_coverage",
		"pattern_compliance",
		"documentation",
	}
	for _, key := range expectedKeys {
		_, ok := result.ScoreBreakdown[key]
		assert.True(t, ok, "breakdown should contain %q", key)
	}
}

func TestSelectGolden_ScoreBetweenZeroAndOne(t *testing.T) {
	modules, analyzed := loadFixture(t)

	result, err := golden.SelectGolden(modules, analyzed)
	require.NoError(t, err)

	assert.GreaterOrEqual(t, result.Score, 0.0)
	assert.LessOrEqual(t, result.Score, 1.0)

	for key, val := range result.ScoreBreakdown {
		assert.GreaterOrEqual(t, val, 0.0, "breakdown %s should be >= 0", key)
		assert.LessOrEqual(t, val, 1.0, "breakdown %s should be <= 1", key)
	}
}

func TestSelectGolden_ErrorOnEmptyModules(t *testing.T) {
	analyzed := make(map[string]*domain.AnalyzedFile)

	result, err := golden.SelectGolden(nil, analyzed)
	assert.Error(t, err)
	assert.Nil(t, result)

	result, err = golden.SelectGolden([]domain.DetectedModule{}, analyzed)
	assert.Error(t, err)
	assert.Nil(t, result)
}

func TestSelectGolden_TaxScoresHigherThanInventory(t *testing.T) {
	modules, analyzed := loadFixture(t)

	// Tax has 8 files vs inventory's 7, plus an extra error file and routes file.
	// Both have 4 layers, tests, Validate, constructors, interfaces, errors.
	// Tax should edge out inventory due to file completeness.
	var taxScore, inventoryScore, paymentsScore float64
	for _, m := range modules {
		gm := golden.ScoreModule(m, modules, analyzed)
		switch m.Name {
		case "tax":
			taxScore = gm.Score
		case "inventory":
			inventoryScore = gm.Score
		case "payments":
			paymentsScore = gm.Score
		}
	}

	assert.Greater(t, taxScore, inventoryScore,
		"tax (8 files) should score higher than inventory (7 files)")
	assert.Greater(t, inventoryScore, paymentsScore,
		"inventory should score higher than payments (1 file, 1 layer)")
}
