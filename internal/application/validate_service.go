package application

import (
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"

	"github.com/abdidvp/openkraft/internal/domain"
)

// ValidateService validates file changes against a cached baseline score,
// detecting drift and computing score impact incrementally.
type ValidateService struct {
	scanner      domain.ProjectScanner
	detector     domain.ModuleDetector
	analyzer     domain.CodeAnalyzer
	scoreService *ScoreService
	cache        domain.CacheStore
	configLoader domain.ConfigLoader
}

// NewValidateService creates a new ValidateService with all required dependencies.
func NewValidateService(
	scanner domain.ProjectScanner,
	detector domain.ModuleDetector,
	analyzer domain.CodeAnalyzer,
	scoreService *ScoreService,
	cache domain.CacheStore,
	configLoader domain.ConfigLoader,
) *ValidateService {
	return &ValidateService{
		scanner: scanner, detector: detector, analyzer: analyzer,
		scoreService: scoreService, cache: cache, configLoader: configLoader,
	}
}

// Validate checks changed/added/deleted files against the cached baseline.
// It returns drift issues, score impact, and a pass/warn/fail status.
func (s *ValidateService) Validate(projectPath string, changed, added, deleted []string, strict bool) (*domain.ValidationResult, error) {
	// 1. Load config
	cfg, err := s.configLoader.Load(projectPath)
	if err != nil {
		return nil, fmt.Errorf("loading config: %w", err)
	}

	// 2. Compute hashes
	goModHash := fileHash(filepath.Join(projectPath, "go.mod"))
	configHash := fileHash(filepath.Join(projectPath, ".openkraft.yaml"))

	// 3. Load cache
	cached, err := s.cache.Load(projectPath)
	if err != nil || cached == nil || cached.IsInvalidated(goModHash, configHash) {
		cached, err = s.createCache(projectPath, cfg, goModHash, configHash)
		if err != nil {
			return nil, fmt.Errorf("creating cache: %w", err)
		}
	}

	// 4. Apply file changes
	for _, f := range deleted {
		delete(cached.AnalyzedFiles, f)
		cached.ScanResult.RemoveFile(f)
	}

	for _, f := range added {
		cached.ScanResult.AddFile(f)
		absPath := filepath.Join(projectPath, f)
		af, err := s.analyzer.AnalyzeFile(absPath)
		if err != nil {
			continue
		}
		af.Path = f
		if cached.AnalyzedFiles == nil {
			cached.AnalyzedFiles = make(map[string]*domain.AnalyzedFile)
		}
		cached.AnalyzedFiles[f] = af
	}

	for _, f := range changed {
		absPath := filepath.Join(projectPath, f)
		af, err := s.analyzer.AnalyzeFile(absPath)
		if err != nil {
			continue
		}
		af.Path = f
		cached.AnalyzedFiles[f] = af
	}

	// 5. Re-detect modules
	modules, err := s.detector.Detect(cached.ScanResult)
	if err != nil {
		return nil, fmt.Errorf("detecting modules: %w", err)
	}

	// 6. Build profile and score
	profile := BuildProfile(cfg)
	newScore := s.scoreService.ScoreWithData(cfg, profile, cached.ScanResult, modules, cached.AnalyzedFiles)

	// 7. Compute norms for drift context
	norms := domain.ComputeNorms(cached.AnalyzedFiles)

	// 8. Classify drift issues
	driftIssues := classifyDrift(cached.BaselineScore, newScore, norms)

	// 9. Compute score impact
	impact := domain.ScoreImpact{
		Categories: make(map[string]int),
	}
	if cached.BaselineScore != nil {
		impact.Overall = newScore.Overall - cached.BaselineScore.Overall
		baselineCats := make(map[string]int)
		for _, c := range cached.BaselineScore.Categories {
			baselineCats[c.Name] = c.Score
		}
		for _, c := range newScore.Categories {
			if base, ok := baselineCats[c.Name]; ok {
				impact.Categories[c.Name] = c.Score - base
			}
		}
	}

	// 10. Determine status
	status := "pass"
	for _, d := range driftIssues {
		if d.Severity == "error" {
			status = "fail"
			break
		}
		if d.Severity == "warning" && status != "fail" {
			status = "warn"
		}
	}
	if strict && status == "warn" {
		status = "fail"
	}

	// Collect all checked files
	allChecked := make([]string, 0, len(changed)+len(added))
	allChecked = append(allChecked, changed...)
	allChecked = append(allChecked, added...)

	// 11. Save updated cache
	cached.BaselineScore = newScore
	cached.Modules = modules
	_ = s.cache.Save(cached)

	return &domain.ValidationResult{
		Status:       status,
		FilesChecked: allChecked,
		DriftIssues:  driftIssues,
		ScoreImpact:  impact,
	}, nil
}

func (s *ValidateService) createCache(projectPath string, cfg domain.ProjectConfig, goModHash, configHash string) (*domain.ProjectCache, error) {
	scan, err := s.scanner.Scan(projectPath, cfg.ExcludePaths...)
	if err != nil {
		return nil, err
	}

	modules, err := s.detector.Detect(scan)
	if err != nil {
		return nil, err
	}

	analyzed := make(map[string]*domain.AnalyzedFile)
	for _, f := range scan.GoFiles {
		absPath := filepath.Join(scan.RootPath, f)
		af, err := s.analyzer.AnalyzeFile(absPath)
		if err != nil {
			continue
		}
		af.Path = f
		analyzed[f] = af
	}

	profile := BuildProfile(cfg)
	score := s.scoreService.ScoreWithData(cfg, profile, scan, modules, analyzed)

	cache := &domain.ProjectCache{
		ProjectPath:   projectPath,
		ConfigHash:    configHash,
		GoModHash:     goModHash,
		ScanResult:    scan,
		AnalyzedFiles: analyzed,
		Modules:       modules,
		BaselineScore: score,
	}

	_ = s.cache.Save(cache)
	return cache, nil
}

func fileHash(path string) string {
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	h := sha256.Sum256(data)
	return fmt.Sprintf("%x", h)
}

// classifyDrift compares baseline and current scores, returning new drift issues.
// Only issues that are new (not present in baseline) are reported.
func classifyDrift(baseline, current *domain.Score, norms domain.ProjectNorms) []domain.DriftIssue {
	if baseline == nil {
		return nil
	}

	// Build set of baseline issues for deduplication
	baselineIssues := make(map[string]bool)
	for _, cat := range baseline.Categories {
		for _, issue := range cat.Issues {
			key := fmt.Sprintf("%s:%s:%s:%d", issue.Category, issue.File, issue.Message, issue.Line)
			baselineIssues[key] = true
		}
	}

	var drifts []domain.DriftIssue
	for _, cat := range current.Categories {
		for _, issue := range cat.Issues {
			key := fmt.Sprintf("%s:%s:%s:%d", issue.Category, issue.File, issue.Message, issue.Line)
			if baselineIssues[key] {
				continue // not new drift
			}

			driftType := mapIssueToDriftType(cat.Name, issue)
			if driftType == "" {
				continue // skip non-drift categories
			}

			msg := issue.Message
			if driftType == "size_drift" && norms.FunctionLines > 0 {
				msg = fmt.Sprintf("%s (project p90: %d lines)", issue.Message, norms.FunctionLines)
			}

			drifts = append(drifts, domain.DriftIssue{
				File:      issue.File,
				Line:      issue.Line,
				Severity:  issue.Severity,
				Message:   msg,
				Category:  cat.Name,
				DriftType: driftType,
			})
		}
	}

	return drifts
}

// mapIssueToDriftType maps a scoring category and issue to a drift classification.
func mapIssueToDriftType(category string, issue domain.Issue) string {
	switch category {
	case "discoverability":
		if issue.SubMetric == "file_naming_conventions" {
			return "naming_drift"
		}
		if issue.SubMetric == "dependency_direction" {
			return "dependency_drift"
		}
		return "naming_drift" // default for discoverability
	case "structure":
		return "structure_drift"
	case "code_health":
		return "size_drift"
	default:
		return "" // verifiability, context_quality, predictability are not drift
	}
}
