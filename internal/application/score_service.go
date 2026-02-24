package application

import (
	"fmt"
	"path/filepath"
	"time"

	"github.com/openkraft/openkraft/internal/domain"
	"github.com/openkraft/openkraft/internal/domain/scoring"
)

// ScoreService orchestrates the scoring pipeline:
// scan → detect modules → analyze AST → run scorers → weighted average.
type ScoreService struct {
	scanner  domain.ProjectScanner
	detector domain.ModuleDetector
	analyzer domain.CodeAnalyzer
}

func NewScoreService(
	scanner domain.ProjectScanner,
	detector domain.ModuleDetector,
	analyzer domain.CodeAnalyzer,
) *ScoreService {
	return &ScoreService{
		scanner:  scanner,
		detector: detector,
		analyzer: analyzer,
	}
}

func (s *ScoreService) ScoreProject(projectPath string) (*domain.Score, error) {
	// 1. Scan filesystem
	scan, err := s.scanner.Scan(projectPath)
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

	// 4. Run scorers
	categories := []domain.CategoryScore{
		scoring.ScoreArchitecture(modules, scan, analyzed),
		scoring.ScoreTests(scan),
	}

	// 5. Compute overall
	overall := domain.ComputeOverallScore(categories)

	return &domain.Score{
		Overall:    overall,
		Categories: categories,
		Timestamp:  time.Now(),
	}, nil
}
