package application

import (
	"fmt"
	"path/filepath"

	"github.com/openkraft/openkraft/internal/domain"
	"github.com/openkraft/openkraft/internal/domain/check"
	"github.com/openkraft/openkraft/internal/domain/golden"
)

// CheckService orchestrates the check pipeline:
// scan -> detect modules -> analyze AST -> select golden -> extract blueprint -> check.
type CheckService struct {
	scanner  domain.ProjectScanner
	detector domain.ModuleDetector
	analyzer domain.CodeAnalyzer
}

func NewCheckService(
	scanner domain.ProjectScanner,
	detector domain.ModuleDetector,
	analyzer domain.CodeAnalyzer,
) *CheckService {
	return &CheckService{
		scanner:  scanner,
		detector: detector,
		analyzer: analyzer,
	}
}

// scanAndAnalyze performs the common scan/detect/analyze steps shared by
// CheckModule and CheckAll.
func (s *CheckService) scanAndAnalyze(projectPath string) ([]domain.DetectedModule, map[string]*domain.AnalyzedFile, error) {
	// 1. Scan filesystem
	scan, err := s.scanner.Scan(projectPath)
	if err != nil {
		return nil, nil, fmt.Errorf("scanning project: %w", err)
	}

	// 2. Detect modules
	modules, err := s.detector.Detect(scan)
	if err != nil {
		return nil, nil, fmt.Errorf("detecting modules: %w", err)
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

	return modules, analyzed, nil
}

// CheckModule checks a single module by name against the golden module's blueprint.
func (s *CheckService) CheckModule(projectPath, moduleName string) (*domain.CheckReport, error) {
	modules, analyzed, err := s.scanAndAnalyze(projectPath)
	if err != nil {
		return nil, err
	}

	// Select golden module
	goldenMod, err := golden.SelectGolden(modules, analyzed)
	if err != nil {
		return nil, fmt.Errorf("selecting golden module: %w", err)
	}

	// Extract blueprint from golden
	blueprint, err := golden.ExtractBlueprint(goldenMod.Module, analyzed)
	if err != nil {
		return nil, fmt.Errorf("extracting blueprint: %w", err)
	}

	// Find target module by name
	var target *domain.DetectedModule
	for i := range modules {
		if modules[i].Name == moduleName {
			target = &modules[i]
			break
		}
	}
	if target == nil {
		return nil, fmt.Errorf("module %q not found in project", moduleName)
	}

	// Run check
	report := check.CheckModule(*target, blueprint, analyzed)
	return report, nil
}

// CheckAll checks all non-golden modules against the golden module's blueprint.
func (s *CheckService) CheckAll(projectPath string) ([]*domain.CheckReport, error) {
	modules, analyzed, err := s.scanAndAnalyze(projectPath)
	if err != nil {
		return nil, err
	}

	// Select golden module
	goldenMod, err := golden.SelectGolden(modules, analyzed)
	if err != nil {
		return nil, fmt.Errorf("selecting golden module: %w", err)
	}

	// Extract blueprint from golden
	blueprint, err := golden.ExtractBlueprint(goldenMod.Module, analyzed)
	if err != nil {
		return nil, fmt.Errorf("extracting blueprint: %w", err)
	}

	// Check all non-golden modules
	var reports []*domain.CheckReport
	for _, m := range modules {
		if m.Name == goldenMod.Module.Name {
			continue // skip the golden module itself
		}
		report := check.CheckModule(m, blueprint, analyzed)
		reports = append(reports, report)
	}

	return reports, nil
}
