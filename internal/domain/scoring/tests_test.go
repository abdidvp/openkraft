package scoring_test

import (
	"testing"

	"github.com/openkraft/openkraft/internal/adapters/outbound/scanner"
	"github.com/openkraft/openkraft/internal/domain"
	"github.com/openkraft/openkraft/internal/domain/scoring"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// testFixtureDir is the correct relative path from this package to the fixture.
// NOTE: fixtureDir is already declared in architecture_test.go with an incorrect
// path (4 levels up instead of 3). We use a separate variable here.
const testFixtureDir = "../../../testdata/go-hexagonal/perfect"

func TestScoreTests_PerfectFixture(t *testing.T) {
	scan, err := scanner.New().Scan(testFixtureDir)
	require.NoError(t, err)

	result := scoring.ScoreTests(scan)

	assert.Equal(t, "tests", result.Name)
	assert.Equal(t, 0.15, result.Weight)
	assert.Len(t, result.SubMetrics, 5)

	// The perfect fixture has 2 test files out of ~15 Go source files,
	// no integration tests, no test helpers, no testdata dir, no CI config.
	// Score should be moderate given limited test infrastructure.
	// 2 test files / ~13 source files = ratio ~0.15, yields ~7/25 for unit tests.
	// No integration tests, helpers, fixtures, or CI => score around 6-8.
	assert.GreaterOrEqual(t, result.Score, 1)
	assert.LessOrEqual(t, result.Score, 30)

	// Verify sub-metric names
	metricNames := make([]string, len(result.SubMetrics))
	for i, m := range result.SubMetrics {
		metricNames[i] = m.Name
	}
	assert.Contains(t, metricNames, "unit_test_presence")
	assert.Contains(t, metricNames, "integration_tests")
	assert.Contains(t, metricNames, "test_helpers")
	assert.Contains(t, metricNames, "test_fixtures")
	assert.Contains(t, metricNames, "ci_config")

	// Verify point allocations sum to 100
	totalPoints := 0
	for _, m := range result.SubMetrics {
		totalPoints += m.Points
	}
	assert.Equal(t, 100, totalPoints)
}

func TestScoreTests_NoTestFiles(t *testing.T) {
	scan := &domain.ScanResult{
		GoFiles:   []string{"main.go", "handler.go", "service.go"},
		TestFiles: []string{},
		AllFiles:  []string{"main.go", "handler.go", "service.go"},
	}

	result := scoring.ScoreTests(scan)

	assert.Equal(t, "tests", result.Name)
	assert.Equal(t, 0.15, result.Weight)
	assert.Equal(t, 0, result.Score)
	assert.Len(t, result.SubMetrics, 5)
	assert.NotEmpty(t, result.Issues)
}

func TestScoreTests_FullTestInfrastructure(t *testing.T) {
	scan := &domain.ScanResult{
		GoFiles: []string{
			"main.go", "handler.go", "service.go",
			"handler_test.go", "service_test.go", "main_test.go",
			"integration_test.go",
		},
		TestFiles: []string{
			"handler_test.go", "service_test.go", "main_test.go",
			"integration_test.go",
		},
		AllFiles: []string{
			"main.go", "handler.go", "service.go",
			"handler_test.go", "service_test.go", "main_test.go",
			"integration_test.go",
			"testdata/fixtures/input.json",
			"testdata/fixtures/expected.json",
			"test/integration/api_test.go",
			"test/e2e/smoke_test.go",
			"testutil/helpers.go",
			"Makefile",
			".github/workflows/ci.yml",
		},
		HasCIConfig: true,
	}

	result := scoring.ScoreTests(scan)

	assert.Equal(t, "tests", result.Name)
	assert.Equal(t, 100, result.Score)
	assert.Empty(t, result.Issues)
}

func TestScoreTests_PartialInfrastructure(t *testing.T) {
	// Has tests and CI but no integration tests or testdata
	scan := &domain.ScanResult{
		GoFiles: []string{
			"main.go", "handler.go",
			"handler_test.go",
		},
		TestFiles: []string{"handler_test.go"},
		AllFiles: []string{
			"main.go", "handler.go", "handler_test.go",
			".github/workflows/ci.yml",
		},
		HasCIConfig: true,
	}

	result := scoring.ScoreTests(scan)

	assert.Equal(t, "tests", result.Name)
	// Has some test coverage + CI, but no integration/helpers/fixtures
	assert.Greater(t, result.Score, 30)
	assert.Less(t, result.Score, 80)
}

func TestScoreTests_EmptyScan(t *testing.T) {
	scan := &domain.ScanResult{}

	result := scoring.ScoreTests(scan)

	assert.Equal(t, "tests", result.Name)
	assert.Equal(t, 0, result.Score)
}

func TestScoreTests_ScoreDoesNotExceed100(t *testing.T) {
	// Maximally rich test infrastructure
	scan := &domain.ScanResult{
		GoFiles:   []string{"a.go", "a_test.go", "b.go", "b_test.go"},
		TestFiles: []string{"a_test.go", "b_test.go"},
		AllFiles: []string{
			"a.go", "a_test.go", "b.go", "b_test.go",
			"test/integration/x_test.go",
			"test/e2e/y_test.go",
			"testutil/helpers.go",
			"testdata/golden/out.txt",
			"testdata/fixtures/in.json",
			".github/workflows/ci.yml",
			"Makefile",
			".gitlab-ci.yml",
		},
		HasCIConfig: true,
	}

	result := scoring.ScoreTests(scan)

	assert.LessOrEqual(t, result.Score, 100)
	for _, sm := range result.SubMetrics {
		assert.LessOrEqual(t, sm.Score, sm.Points,
			"sub-metric %s score should not exceed max points", sm.Name)
	}
}
