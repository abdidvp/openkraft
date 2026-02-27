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
		af.Path = f // store relative path for clean issue reporting
		analyzed[f] = af
	}

	// 4. Build scoring profile from config defaults + user overrides
	profile := BuildProfile(cfg)

	// 5-7. Score with pre-loaded data
	result := s.ScoreWithData(cfg, profile, scan, modules, analyzed)

	// Attach config to output if non-default
	var appliedCfg *domain.ProjectConfig
	if cfg.ProjectType != "" || len(cfg.Weights) > 0 || len(cfg.Skip.Categories) > 0 || len(cfg.Skip.SubMetrics) > 0 {
		appliedCfg = &cfg
	}
	result.AppliedConfig = appliedCfg

	return result, nil
}

// ScoreWithData runs all 6 scorers with pre-loaded data. No disk I/O.
func (s *ScoreService) ScoreWithData(
	cfg domain.ProjectConfig,
	profile domain.ScoringProfile,
	scan *domain.ScanResult,
	modules []domain.DetectedModule,
	analyzed map[string]*domain.AnalyzedFile,
) *domain.Score {
	categories := []domain.CategoryScore{
		scoring.ScoreCodeHealth(&profile, scan, analyzed),
		scoring.ScoreDiscoverability(&profile, modules, scan, analyzed),
		scoring.ScoreStructure(&profile, modules, scan, analyzed),
		scoring.ScoreVerifiability(&profile, scan, analyzed),
		scoring.ScoreContextQuality(&profile, scan, analyzed),
		scoring.ScorePredictability(&profile, modules, scan, analyzed),
	}

	categories = applyConfig(categories, cfg)
	overall := domain.ComputeOverallScore(categories)

	return &domain.Score{
		Overall:    overall,
		Categories: categories,
		Timestamp:  time.Now(),
	}
}

// BuildProfile constructs a ScoringProfile from config defaults and user overrides.
func BuildProfile(cfg domain.ProjectConfig) domain.ScoringProfile {
	base := domain.DefaultProfileForType(cfg.ProjectType)
	if cfg.Profile == nil {
		return base
	}
	p := cfg.Profile

	if len(p.ExpectedLayers) > 0 {
		base.ExpectedLayers = p.ExpectedLayers
	}
	if len(p.ExpectedDirs) > 0 {
		base.ExpectedDirs = p.ExpectedDirs
	}
	if len(p.LayerAliases) > 0 {
		base.LayerAliases = p.LayerAliases
	}
	if len(p.ExpectedFileSuffixes) > 0 {
		base.ExpectedFileSuffixes = p.ExpectedFileSuffixes
	}
	if p.NamingConvention != "" {
		base.NamingConvention = p.NamingConvention
	}
	if p.MaxFunctionLines != nil {
		base.MaxFunctionLines = *p.MaxFunctionLines
	}
	if p.MaxFileLines != nil {
		base.MaxFileLines = *p.MaxFileLines
	}
	if p.MaxNestingDepth != nil {
		base.MaxNestingDepth = *p.MaxNestingDepth
	}
	if p.MaxParameters != nil {
		base.MaxParameters = *p.MaxParameters
	}
	if p.MaxConditionalOps != nil {
		base.MaxConditionalOps = *p.MaxConditionalOps
	}
	if len(p.ExemptParamPatterns) > 0 {
		base.ExemptParamPatterns = p.ExemptParamPatterns
	}
	if len(p.ContextFiles) > 0 {
		base.ContextFiles = p.ContextFiles
	}
	if p.MinTestRatio != nil {
		base.MinTestRatio = *p.MinTestRatio
	}
	if p.MaxGlobalVarPenalty != nil {
		base.MaxGlobalVarPenalty = *p.MaxGlobalVarPenalty
	}

	return base
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
			cat.SubMetrics[i].Score = 0
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

	// Remove issues associated with skipped sub-metrics
	if hasSkipped {
		var filtered []domain.Issue
		for _, issue := range cat.Issues {
			if issue.SubMetric == "" || !cfg.IsSkippedSubMetric(issue.SubMetric) {
				filtered = append(filtered, issue)
			}
		}
		cat.Issues = filtered
	}

	return cat
}
