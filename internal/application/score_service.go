package application

import (
	"fmt"
	"math"
	"path/filepath"
	"time"

	"github.com/openkraft/openkraft/internal/domain"
	"github.com/openkraft/openkraft/internal/domain/scoring"
)

// ScoreService orchestrates the scoring pipeline:
// scan → detect modules → analyze AST → run scorers → apply config → weighted average.
type ScoreService struct {
	scanner      domain.ProjectScanner
	detector     domain.ModuleDetector
	analyzer     domain.CodeAnalyzer
	configLoader domain.ConfigLoader
}

func NewScoreService(
	scanner domain.ProjectScanner,
	detector domain.ModuleDetector,
	analyzer domain.CodeAnalyzer,
	configLoader domain.ConfigLoader,
) *ScoreService {
	return &ScoreService{
		scanner:      scanner,
		detector:     detector,
		analyzer:     analyzer,
		configLoader: configLoader,
	}
}

func (s *ScoreService) ScoreProject(projectPath string) (*domain.Score, error) {
	// 0. Load config
	cfg, err := s.configLoader.Load(projectPath)
	if err != nil {
		return nil, fmt.Errorf("loading config: %w", err)
	}

	// 1. Scan filesystem (pass exclude paths from config)
	scan, err := s.scanner.Scan(projectPath, cfg.ExcludePaths...)
	if err != nil {
		return nil, fmt.Errorf("scanning project: %w", err)
	}

	// 2. Detect modules
	modules, err := s.detector.Detect(scan)
	if err != nil {
		return nil, fmt.Errorf("detecting modules: %w", err)
	}

	// 3. Analyze Go files via AST
	analyzed := make(map[string]*domain.AnalyzedFile)
	for _, f := range scan.GoFiles {
		absPath := filepath.Join(scan.RootPath, f)
		af, err := s.analyzer.AnalyzeFile(absPath)
		if err != nil {
			continue // skip files that can't be parsed
		}
		analyzed[f] = af
	}

	// 4. Run all 6 scorers
	categories := []domain.CategoryScore{
		scoring.ScoreCodeHealth(scan, analyzed),
		scoring.ScoreDiscoverability(modules, scan, analyzed),
		scoring.ScoreStructure(modules, scan, analyzed),
		scoring.ScoreVerifiability(scan, analyzed),
		scoring.ScoreContextQuality(scan, analyzed),
		scoring.ScorePredictability(modules, scan, analyzed),
	}

	// 5. Apply config: skip categories, filter sub-metrics, override weights
	categories = applyConfig(categories, cfg)

	// 6. Compute overall
	overall := domain.ComputeOverallScore(categories)

	// Attach config to output if non-default
	var appliedCfg *domain.ProjectConfig
	if cfg.ProjectType != "" || len(cfg.Weights) > 0 || len(cfg.Skip.Categories) > 0 || len(cfg.Skip.SubMetrics) > 0 {
		appliedCfg = &cfg
	}

	return &domain.Score{
		Overall:       overall,
		Categories:    categories,
		Timestamp:     time.Now(),
		AppliedConfig: appliedCfg,
	}, nil
}

// applyConfig filters and adjusts category scores based on project config.
func applyConfig(categories []domain.CategoryScore, cfg domain.ProjectConfig) []domain.CategoryScore {
	var result []domain.CategoryScore

	for _, cat := range categories {
		// Skip entire categories
		if cfg.IsSkippedCategory(cat.Name) {
			continue
		}

		// Override weight
		cat.Weight = cfg.EffectiveWeight(cat.Name, cat.Weight)

		// Filter skipped sub-metrics and recalculate category score
		cat = filterSubMetrics(cat, cfg)

		result = append(result, cat)
	}

	return result
}

// filterSubMetrics marks skipped sub-metrics and recalculates the category score
// based only on remaining (non-skipped) sub-metrics.
func filterSubMetrics(cat domain.CategoryScore, cfg domain.ProjectConfig) domain.CategoryScore {
	var totalPoints, earnedPoints int
	var hasSkipped bool

	for i, sm := range cat.SubMetrics {
		if cfg.IsSkippedSubMetric(sm.Name) {
			cat.SubMetrics[i].Skipped = true
			hasSkipped = true
			continue
		}
		totalPoints += sm.Points
		earnedPoints += sm.Score
	}

	// Recalculate category score if sub-metrics were skipped
	if hasSkipped && totalPoints > 0 {
		cat.Score = int(math.Round(float64(earnedPoints) / float64(totalPoints) * 100))
	}

	return cat
}
