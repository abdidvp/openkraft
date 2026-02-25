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

const fixtureBase = "../../../testdata/go-hexagonal"

type fixtureData struct {
	scan     *domain.ScanResult
	modules  []domain.DetectedModule
	analyzed map[string]*domain.AnalyzedFile
}

func loadFixture(t *testing.T, name string) fixtureData {
	t.Helper()
	path := filepath.Join(fixtureBase, name)

	s := scanner.New()
	scan, err := s.Scan(path)
	require.NoError(t, err)

	d := &detector.ModuleDetector{}
	modules, err := d.Detect(scan)
	require.NoError(t, err)

	p := &parser.GoParser{}
	analyzed := make(map[string]*domain.AnalyzedFile)
	for _, f := range scan.GoFiles {
		absPath := filepath.Join(scan.RootPath, f)
		af, err := p.AnalyzeFile(absPath)
		if err != nil {
			continue
		}
		analyzed[f] = af
	}

	return fixtureData{scan: scan, modules: modules, analyzed: analyzed}
}

type scorerFunc func(fixtureData) domain.CategoryScore

var scorers = map[string]scorerFunc{
	"code_health": func(fd fixtureData) domain.CategoryScore {
		return scoring.ScoreCodeHealth(fd.scan, fd.analyzed)
	},
	"discoverability": func(fd fixtureData) domain.CategoryScore {
		return scoring.ScoreDiscoverability(fd.modules, fd.scan, fd.analyzed)
	},
	"structure": func(fd fixtureData) domain.CategoryScore {
		return scoring.ScoreStructure(fd.modules, fd.scan, fd.analyzed)
	},
	"verifiability": func(fd fixtureData) domain.CategoryScore {
		return scoring.ScoreVerifiability(fd.scan, fd.analyzed)
	},
	"context_quality": func(fd fixtureData) domain.CategoryScore {
		return scoring.ScoreContextQuality(fd.scan, fd.analyzed)
	},
	"predictability": func(fd fixtureData) domain.CategoryScore {
		return scoring.ScorePredictability(fd.modules, fd.scan, fd.analyzed)
	},
}

func TestIntegration_ScoresWithinBounds(t *testing.T) {
	for _, fixture := range []string{"perfect", "incomplete", "empty"} {
		t.Run(fixture, func(t *testing.T) {
			fd := loadFixture(t, fixture)

			for name, fn := range scorers {
				t.Run(name, func(t *testing.T) {
					cat := fn(fd)

					assert.GreaterOrEqual(t, cat.Score, 0, "score should be >= 0")
					assert.LessOrEqual(t, cat.Score, 100, "score should be <= 100")

					// Sub-metric points must sum to 100.
					totalPoints := 0
					for _, sm := range cat.SubMetrics {
						totalPoints += sm.Points
						assert.GreaterOrEqual(t, sm.Score, 0, "sub-metric %s score >= 0", sm.Name)
						assert.LessOrEqual(t, sm.Score, sm.Points, "sub-metric %s score <= points", sm.Name)
					}
					assert.Equal(t, 100, totalPoints, "sub-metric points should sum to 100")
				})
			}
		})
	}
}

func TestIntegration_PerfectBeatIncompleteOnMostCategories(t *testing.T) {
	perfect := loadFixture(t, "perfect")
	incomplete := loadFixture(t, "incomplete")

	winsNeeded := 4
	wins := 0

	for name, fn := range scorers {
		pScore := fn(perfect).Score
		iScore := fn(incomplete).Score
		t.Logf("%s: perfect=%d incomplete=%d", name, pScore, iScore)
		if pScore >= iScore {
			wins++
		}
	}

	assert.GreaterOrEqual(t, wins, winsNeeded,
		"perfect fixture should beat incomplete on at least %d of 6 categories", winsNeeded)
}
