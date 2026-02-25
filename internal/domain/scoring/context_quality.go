package scoring

import (
	"fmt"
	"strings"

	"github.com/openkraft/openkraft/internal/domain"
)

// ScoreContextQuality evaluates the quality of AI context files.
// Weight: 0.15 (15% of overall score).
func ScoreContextQuality(profile *domain.ScoringProfile, scan *domain.ScanResult, analyzed map[string]*domain.AnalyzedFile) domain.CategoryScore {
	cat := domain.CategoryScore{
		Name:   "context_quality",
		Weight: 0.15,
	}

	sm1 := scoreAIContextFiles(profile, scan)
	sm2 := scorePackageDocumentation(analyzed)
	sm3 := scoreArchitectureDocs(scan)
	sm4 := scoreCanonicalExamples(scan, analyzed)

	cat.SubMetrics = []domain.SubMetric{sm1, sm2, sm3, sm4}

	total := 0
	for _, sm := range cat.SubMetrics {
		total += sm.Score
	}
	cat.Score = total

	cat.Issues = collectContextQualityIssues(scan)
	return cat
}

// scoreAIContextFiles: uses profile.ContextFiles to determine which files to check
// and their point values. Total points are the sum of all configured file points.
func scoreAIContextFiles(profile *domain.ScoringProfile, scan *domain.ScanResult) domain.SubMetric {
	totalPossible := 0
	for _, cf := range profile.ContextFiles {
		totalPossible += cf.Points
	}
	if totalPossible == 0 {
		totalPossible = 30
	}
	sm := domain.SubMetric{Name: "ai_context_files", Points: totalPossible}

	if scan == nil {
		sm.Detail = "no scan data"
		return sm
	}

	points := 0
	found := []string{}

	for _, cf := range profile.ContextFiles {
		exists, size := contextFileStatus(cf.Name, scan)
		if !exists {
			continue
		}
		found = append(found, cf.Name)
		if cf.MinSize > 0 {
			// Half points for existence, full for meeting size threshold.
			halfPts := cf.Points / 2
			if halfPts == 0 {
				halfPts = 1
			}
			points += halfPts
			if size >= cf.MinSize {
				points += cf.Points - halfPts
			}
		} else {
			points += cf.Points
		}
	}

	if points > sm.Points {
		points = sm.Points
	}
	sm.Score = points
	if len(found) > 0 {
		sm.Detail = fmt.Sprintf("found: %s (%d pts)", strings.Join(found, ", "), points)
	} else {
		sm.Detail = "no AI context files found"
	}
	return sm
}

// contextFileStatus maps a context file name to its scan status.
func contextFileStatus(name string, scan *domain.ScanResult) (exists bool, size int) {
	switch {
	case name == "CLAUDE.md":
		return scan.HasClaudeMD, scan.ClaudeMDSize
	case name == "AGENTS.md":
		return scan.HasAgentsMD, scan.AgentsMDSize
	case name == ".cursorrules":
		return scan.HasCursorRules, scan.CursorRulesSize
	case strings.Contains(name, "copilot-instructions"):
		return scan.HasCopilotInstructions, 0
	default:
		// Unknown context file — check AllFiles for presence.
		for _, f := range scan.AllFiles {
			if f == name || strings.HasSuffix(f, "/"+name) {
				return true, 0
			}
		}
		return false, 0
	}
}

// scorePackageDocumentation (25 pts): ratio of packages with // Package ... doc comment.
func scorePackageDocumentation(analyzed map[string]*domain.AnalyzedFile) domain.SubMetric {
	sm := domain.SubMetric{Name: "package_documentation", Points: 25}

	// Deduplicate packages — one file with doc is enough per package.
	packages := make(map[string]bool)   // package name → seen
	documented := make(map[string]bool) // package name → has doc

	for _, af := range analyzed {
		if strings.HasSuffix(af.Path, "_test.go") {
			continue
		}
		pkg := af.Package
		packages[pkg] = true
		if af.PackageDoc {
			documented[pkg] = true
		}
	}

	if len(packages) == 0 {
		sm.Detail = "no packages found"
		return sm
	}

	ratio := float64(len(documented)) / float64(len(packages))
	sm.Score = int(ratio * float64(sm.Points))
	if sm.Score > sm.Points {
		sm.Score = sm.Points
	}
	sm.Detail = fmt.Sprintf("%d/%d packages have documentation comments", len(documented), len(packages))
	return sm
}

// scoreArchitectureDocs (20 pts): README.md >500 bytes (8), docs/ dir (7), ADR files (5).
func scoreArchitectureDocs(scan *domain.ScanResult) domain.SubMetric {
	sm := domain.SubMetric{Name: "architecture_docs", Points: 20}

	if scan == nil {
		sm.Detail = "no scan data"
		return sm
	}

	points := 0
	found := []string{}

	// README.md >500 bytes (8 pts)
	if scan.ReadmeSize > 500 {
		points += 8
		found = append(found, "README.md")
	} else if scan.ReadmeSize > 0 {
		points += 4
		found = append(found, "README.md (small)")
	}

	// docs/ directory (7 pts)
	for _, f := range scan.AllFiles {
		if strings.HasPrefix(f, "docs/") || strings.HasPrefix(f, "doc/") {
			points += 7
			found = append(found, "docs/")
			break
		}
	}

	// ADR files (5 pts)
	for _, f := range scan.AllFiles {
		lower := strings.ToLower(f)
		if strings.Contains(lower, "adr") && strings.HasSuffix(lower, ".md") {
			points += 5
			found = append(found, "ADR files")
			break
		}
	}

	if points > sm.Points {
		points = sm.Points
	}
	sm.Score = points
	if len(found) > 0 {
		sm.Detail = fmt.Sprintf("found: %s", strings.Join(found, ", "))
	} else {
		sm.Detail = "no architecture documentation found"
	}
	return sm
}

// scoreCanonicalExamples (25 pts): example_test.go files + Example* functions + CLAUDE.md references.
func scoreCanonicalExamples(scan *domain.ScanResult, analyzed map[string]*domain.AnalyzedFile) domain.SubMetric {
	sm := domain.SubMetric{Name: "canonical_examples", Points: 25}

	if scan == nil {
		sm.Detail = "no scan data"
		return sm
	}

	points := 0
	found := []string{}

	// example_test.go files (10 pts)
	exampleFileCount := 0
	for _, f := range scan.AllFiles {
		if strings.HasSuffix(f, "example_test.go") || strings.Contains(f, "_example_test.go") {
			exampleFileCount++
		}
	}
	if exampleFileCount > 0 {
		if exampleFileCount >= 3 {
			points += 10
		} else {
			points += exampleFileCount * 4
		}
		found = append(found, fmt.Sprintf("%d example_test.go files", exampleFileCount))
	}

	// Example* test functions in any _test.go file (5 pts)
	exampleFuncCount := 0
	for _, af := range analyzed {
		if !strings.HasSuffix(af.Path, "_test.go") {
			continue
		}
		for _, fn := range af.Functions {
			if strings.HasPrefix(fn.Name, "Example") {
				exampleFuncCount++
			}
		}
	}
	if exampleFuncCount > 0 {
		if exampleFuncCount >= 5 {
			points += 5
		} else {
			points += exampleFuncCount
		}
		found = append(found, fmt.Sprintf("%d Example* functions", exampleFuncCount))
	}

	// CLAUDE.md pattern references: 'see `path`' (10 pts)
	if scan.HasClaudeMD && scan.ClaudeMDContent != "" {
		content := scan.ClaudeMDContent
		if strings.Contains(content, "see `") || strings.Contains(content, "See `") ||
			strings.Contains(content, "example") || strings.Contains(content, "Example") {
			points += 10
			found = append(found, "CLAUDE.md references")
		}
	}

	if points > sm.Points {
		points = sm.Points
	}
	sm.Score = points
	if len(found) > 0 {
		sm.Detail = fmt.Sprintf("found: %s", strings.Join(found, ", "))
	} else {
		sm.Detail = "no canonical examples found"
	}
	return sm
}

func collectContextQualityIssues(scan *domain.ScanResult) []domain.Issue {
	var issues []domain.Issue

	if scan == nil {
		return issues
	}

	if !scan.HasClaudeMD {
		issues = append(issues, domain.Issue{
			Severity:     domain.SeverityWarning,
			Category:     "context_quality",
			Message:      "CLAUDE.md not found; add it to provide AI agents with project context",
			FixAvailable: true,
		})
	}

	if !scan.HasCursorRules {
		issues = append(issues, domain.Issue{
			Severity:     domain.SeverityInfo,
			Category:     "context_quality",
			Message:      ".cursorrules not found; add it for Cursor IDE integration",
			FixAvailable: true,
		})
	}

	if !scan.HasAgentsMD {
		issues = append(issues, domain.Issue{
			Severity:     domain.SeverityInfo,
			Category:     "context_quality",
			Message:      "AGENTS.md not found; add it to describe agent workflows",
			FixAvailable: true,
		})
	}

	return issues
}
