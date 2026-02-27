package application

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"

	"github.com/openkraft/openkraft/internal/domain"
)

// FixService orchestrates the fix pipeline:
// score → onboard → identify safe fixes → generate drift corrections.
type FixService struct {
	scoreService   *ScoreService
	onboardService *OnboardService
}

func NewFixService(score *ScoreService, onboard *OnboardService) *FixService {
	return &FixService{scoreService: score, onboardService: onboard}
}

func (s *FixService) PlanFixes(projectPath string, opts domain.FixOptions) (*domain.FixPlan, error) {
	// 1. Score the project
	score, err := s.scoreService.ScoreProject(projectPath)
	if err != nil {
		return nil, fmt.Errorf("scoring project: %w", err)
	}

	// 2. Generate onboard report (establishes norms)
	report, err := s.onboardService.GenerateReport(projectPath)
	if err != nil {
		return nil, fmt.Errorf("generating report: %w", err)
	}

	plan := &domain.FixPlan{ScoreBefore: score.Overall}

	// 3. Identify safe fixes (file creation only)
	if !opts.DryRun {
		plan.Applied = s.applySafeFixes(projectPath, score, report, opts)
	} else {
		plan.Applied = s.identifySafeFixes(projectPath, score, opts)
	}

	// 4. Generate drift correction instructions
	if !opts.AutoOnly {
		plan.Instructions = s.generateDriftCorrections(score, report, opts)
	}

	// 5. If not dry run, verify compilation and compute after score
	if !opts.DryRun && len(plan.Applied) > 0 {
		// Verify compilation
		cmd := exec.Command("go", "build", "./...")
		cmd.Dir = projectPath
		if err := cmd.Run(); err != nil {
			// Rollback: delete created files
			for _, fix := range plan.Applied {
				if fix.Type == "create_file" {
					os.Remove(filepath.Join(projectPath, fix.Path))
				}
			}
			return nil, fmt.Errorf("compilation check failed after fixes: %w", err)
		}

		// Re-score
		afterScore, err := s.scoreService.ScoreProject(projectPath)
		if err == nil {
			plan.ScoreAfter = afterScore.Overall
		}
	} else {
		plan.ScoreAfter = score.Overall
	}

	return plan, nil
}

func (s *FixService) identifySafeFixes(projectPath string, score *domain.Score, opts domain.FixOptions) []domain.AppliedFix {
	var fixes []domain.AppliedFix

	if opts.Category != "" && opts.Category != "context_quality" && opts.Category != "verifiability" {
		return fixes
	}

	// 1. Missing CLAUDE.md
	claudePath := filepath.Join(projectPath, "CLAUDE.md")
	if _, err := os.Stat(claudePath); os.IsNotExist(err) {
		if opts.Category == "" || opts.Category == "context_quality" {
			fixes = append(fixes, domain.AppliedFix{
				Type:        "create_file",
				Path:        "CLAUDE.md",
				Description: "Generated consistency contract from codebase analysis",
			})
		}
	}

	// 2. Missing test stubs
	if opts.Category == "" || opts.Category == "verifiability" {
		fixes = append(fixes, s.identifyMissingTestStubs(projectPath, score)...)
	}

	return fixes
}

func (s *FixService) applySafeFixes(projectPath string, score *domain.Score, report *domain.OnboardReport, opts domain.FixOptions) []domain.AppliedFix {
	fixes := s.identifySafeFixes(projectPath, score, opts)
	var applied []domain.AppliedFix

	for _, fix := range fixes {
		absPath := filepath.Join(projectPath, fix.Path)
		switch {
		case fix.Path == "CLAUDE.md":
			contract := s.onboardService.RenderContract(report)
			if err := os.WriteFile(absPath, []byte(contract), 0644); err == nil {
				applied = append(applied, fix)
			}
		case strings.HasSuffix(fix.Path, "_test.go"):
			// Determine package name from directory
			dir := filepath.Dir(absPath)
			pkgName := filepath.Base(dir)
			content := fmt.Sprintf("package %s_test\n\nimport \"testing\"\n\nfunc TestPlaceholder(t *testing.T) {\n\tt.Skip(\"placeholder test\")\n}\n", pkgName)
			if err := os.MkdirAll(dir, 0755); err == nil {
				if err := os.WriteFile(absPath, []byte(content), 0644); err == nil {
					applied = append(applied, fix)
				}
			}
		}
	}

	return applied
}

func (s *FixService) identifyMissingTestStubs(projectPath string, score *domain.Score) []domain.AppliedFix {
	var fixes []domain.AppliedFix

	// Track which directories already have test stubs identified
	packagesWithTests := make(map[string]bool)

	// Walk through issues to find packages without tests
	for _, cat := range score.Categories {
		if cat.Name != "verifiability" {
			continue
		}
		for _, issue := range cat.Issues {
			if strings.Contains(issue.Message, "no test file") || strings.Contains(issue.Message, "test") {
				dir := filepath.Dir(issue.File)
				if dir != "" && !packagesWithTests[dir] {
					testFile := filepath.Join(dir, filepath.Base(dir)+"_test.go")
					// Check if test file already exists
					absTest := filepath.Join(projectPath, testFile)
					if _, err := os.Stat(absTest); os.IsNotExist(err) {
						fixes = append(fixes, domain.AppliedFix{
							Type:        "create_file",
							Path:        testFile,
							Description: fmt.Sprintf("Test stub for package %s", filepath.Base(dir)),
						})
						packagesWithTests[dir] = true
					}
				}
			}
		}
	}

	return fixes
}

func (s *FixService) generateDriftCorrections(score *domain.Score, report *domain.OnboardReport, opts domain.FixOptions) []domain.Instruction {
	var instructions []domain.Instruction

	for _, cat := range score.Categories {
		if opts.Category != "" && cat.Name != opts.Category {
			continue
		}

		for _, issue := range cat.Issues {
			inst := ClassifyIssueAsInstruction(issue, cat.Name, report)
			if inst != nil {
				instructions = append(instructions, *inst)
			}
		}
	}

	// Sort by priority: high first, then medium, then low
	sort.Slice(instructions, func(i, j int) bool {
		return PriorityRank(instructions[i].Priority) < PriorityRank(instructions[j].Priority)
	})

	return instructions
}

// ClassifyIssueAsInstruction maps a scoring issue to a drift correction instruction.
// Returns nil if the issue does not map to a known drift type.
func ClassifyIssueAsInstruction(issue domain.Issue, category string, report *domain.OnboardReport) *domain.Instruction {
	inst := &domain.Instruction{
		File:    issue.File,
		Line:    issue.Line,
		Message: issue.Message,
	}

	switch {
	case category == "discoverability" && (issue.SubMetric == "file_naming_conventions" || strings.Contains(issue.Message, "naming")):
		inst.Type = "naming_drift"
		inst.Priority = "medium"
		inst.ProjectNorm = fmt.Sprintf("%s naming (%.0f%% of files)", report.Norms.NamingStyle, report.Norms.NamingPct*100)

	case category == "discoverability" && (issue.SubMetric == "dependency_direction" || strings.Contains(issue.Message, "dependency") || strings.Contains(issue.Message, "imports")):
		inst.Type = "dependency_drift"
		inst.Priority = "high"
		inst.ProjectNorm = "domain/ has zero adapter imports"

	case category == "structure":
		inst.Type = "structure_drift"
		inst.Priority = "high"
		if report.GoldenModule != "" {
			inst.ProjectNorm = fmt.Sprintf("%d layers per module (golden: %s)", len(report.ModuleBlueprint), report.GoldenModule)
		} else {
			inst.ProjectNorm = "consistent module structure"
		}

	case category == "code_health" && (issue.SubMetric == "function_size" || strings.Contains(issue.Message, "function") || strings.Contains(issue.Message, "lines")):
		inst.Type = "size_drift"
		inst.Priority = "medium"
		inst.ProjectNorm = fmt.Sprintf("p90 function length: %d lines", report.Norms.FunctionLines)

	case category == "code_health" && (issue.SubMetric == "file_size" || strings.Contains(issue.Message, "file")):
		inst.Type = "size_drift"
		inst.Priority = "medium"
		inst.ProjectNorm = fmt.Sprintf("p90 file length: %d lines", report.Norms.FileLines)

	case category == "code_health" && (issue.SubMetric == "parameter_count" || strings.Contains(issue.Message, "param")):
		inst.Type = "size_drift"
		inst.Priority = "medium"
		inst.ProjectNorm = fmt.Sprintf("p90 parameter count: %d", report.Norms.Parameters)

	case category == "context_quality" && strings.Contains(issue.Message, "doc"):
		inst.Type = "missing_docs"
		inst.Priority = "low"
		inst.ProjectNorm = "see internal/domain/ for doc style"

	default:
		// Skip issues that don't map to drift types
		return nil
	}

	return inst
}

// PriorityRank returns a numeric rank for sorting priorities (lower is higher priority).
func PriorityRank(p string) int {
	switch p {
	case "high":
		return 0
	case "medium":
		return 1
	case "low":
		return 2
	default:
		return 3
	}
}
