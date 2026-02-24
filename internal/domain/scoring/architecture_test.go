package scoring_test

import (
	"path/filepath"
	"testing"

	"github.com/openkraft/openkraft/internal/adapters/outbound/detector"
	"github.com/openkraft/openkraft/internal/adapters/outbound/parser"
	"github.com/openkraft/openkraft/internal/adapters/outbound/scanner"
	"github.com/openkraft/openkraft/internal/domain"
	"github.com/openkraft/openkraft/internal/domain/scoring"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const archFixtureDir = "../../../testdata/go-hexagonal/perfect"

// helper loads scan result, detected modules, and analyzed files from the fixture.
func loadFixture(t *testing.T) ([]domain.DetectedModule, *domain.ScanResult, map[string]*domain.AnalyzedFile) {
	t.Helper()

	scan, err := scanner.New().Scan(archFixtureDir)
	require.NoError(t, err)

	modules, err := detector.New().Detect(scan)
	require.NoError(t, err)

	p := parser.New()
	analyzed := make(map[string]*domain.AnalyzedFile)
	for _, f := range scan.GoFiles {
		af, err := p.AnalyzeFile(filepath.Join(archFixtureDir, f))
		if err != nil {
			continue
		}
		analyzed[f] = af
	}

	return modules, scan, analyzed
}

func TestScoreArchitecture_PerfectFixture(t *testing.T) {
	modules, scan, analyzed := loadFixture(t)
	result := scoring.ScoreArchitecture(modules, scan, analyzed)

	assert.Equal(t, "architecture", result.Name)
	assert.Equal(t, 0.25, result.Weight)
	assert.Len(t, result.SubMetrics, 5)
	assert.Greater(t, result.Score, 70, "perfect fixture should score > 70")
}

func TestScoreArchitecture_SubMetricNames(t *testing.T) {
	modules, scan, analyzed := loadFixture(t)
	result := scoring.ScoreArchitecture(modules, scan, analyzed)

	names := make([]string, len(result.SubMetrics))
	for i, sm := range result.SubMetrics {
		names[i] = sm.Name
	}

	assert.Contains(t, names, "consistent_module_structure")
	assert.Contains(t, names, "layer_separation")
	assert.Contains(t, names, "dependency_direction")
	assert.Contains(t, names, "module_boundary_clarity")
	assert.Contains(t, names, "architecture_documentation")
}

func TestScoreArchitecture_SubMetricMaxPoints(t *testing.T) {
	modules, scan, analyzed := loadFixture(t)
	result := scoring.ScoreArchitecture(modules, scan, analyzed)

	pointsMap := make(map[string]int)
	for _, sm := range result.SubMetrics {
		pointsMap[sm.Name] = sm.Points
	}

	assert.Equal(t, 30, pointsMap["consistent_module_structure"])
	assert.Equal(t, 25, pointsMap["layer_separation"])
	assert.Equal(t, 20, pointsMap["dependency_direction"])
	assert.Equal(t, 15, pointsMap["module_boundary_clarity"])
	assert.Equal(t, 10, pointsMap["architecture_documentation"])
}

func TestScoreArchitecture_ArchitectureDocumentation(t *testing.T) {
	modules, scan, analyzed := loadFixture(t)
	// Perfect fixture has CLAUDE.md
	result := scoring.ScoreArchitecture(modules, scan, analyzed)

	for _, sm := range result.SubMetrics {
		if sm.Name == "architecture_documentation" {
			assert.GreaterOrEqual(t, sm.Score, 8, "perfect fixture has CLAUDE.md + .cursorrules, should score well")
			return
		}
	}
	t.Fatal("architecture_documentation sub-metric not found")
}

func TestScoreArchitecture_EmptyModules(t *testing.T) {
	scan := &domain.ScanResult{}
	analyzed := make(map[string]*domain.AnalyzedFile)

	result := scoring.ScoreArchitecture(nil, scan, analyzed)

	assert.Equal(t, "architecture", result.Name)
	assert.Equal(t, 0.25, result.Weight)
	assert.Len(t, result.SubMetrics, 5)
	// With no modules, score should be very low (only docs can contribute)
	assert.LessOrEqual(t, result.Score, 10)
}

func TestScoreArchitecture_ModuleBoundaryClarity(t *testing.T) {
	modules, scan, analyzed := loadFixture(t)
	result := scoring.ScoreArchitecture(modules, scan, analyzed)

	for _, sm := range result.SubMetrics {
		if sm.Name == "module_boundary_clarity" {
			assert.Greater(t, sm.Score, 0, "perfect fixture modules are under internal/")
			return
		}
	}
	t.Fatal("module_boundary_clarity sub-metric not found")
}

func TestScoreArchitecture_ScoreDoesNotExceed100(t *testing.T) {
	modules, scan, analyzed := loadFixture(t)
	result := scoring.ScoreArchitecture(modules, scan, analyzed)

	assert.LessOrEqual(t, result.Score, 100)

	totalPoints := 0
	for _, sm := range result.SubMetrics {
		totalPoints += sm.Score
		assert.LessOrEqual(t, sm.Score, sm.Points, "sub-metric %s score should not exceed max points", sm.Name)
	}
	assert.LessOrEqual(t, totalPoints, 100)
}
