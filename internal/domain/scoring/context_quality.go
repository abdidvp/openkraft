package scoring

import (
	"fmt"
	"strings"

	"github.com/openkraft/openkraft/internal/domain"
)

// ScoreContextQuality evaluates the quality of AI context files.
// Weight: 0.15 (15% of overall score).
func ScoreContextQuality(scan *domain.ScanResult, analyzed map[string]*domain.AnalyzedFile) domain.CategoryScore {
	cat := domain.CategoryScore{
		Name:   "context_quality",
		Weight: 0.15,
	}

	sm1 := scoreAIContextFiles(scan)
	sm2 := scorePackageDocumentation(analyzed)
	sm3 := scoreArchitectureDocs(scan)
	sm4 := scoreCanonicalExamples(scan)

	cat.SubMetrics = []domain.SubMetric{sm1, sm2, sm3, sm4}

	total := 0
	for _, sm := range cat.SubMetrics {
		total += sm.Score
	}
	cat.Score = total

	cat.Issues = collectContextQualityIssues(scan)
	return cat
}

// scoreAIContextFiles (30 pts): CLAUDE.md, AGENTS.md, .cursorrules, copilot-instructions.md.
// Enhanced from ai_context.go.
func scoreAIContextFiles(scan *domain.ScanResult) domain.SubMetric {
	sm := domain.SubMetric{Name: "ai_context_files", Points: 30}

	if scan == nil {
		sm.Detail = "no scan data"
		return sm
	}

	points := 0
	found := []string{}

	// CLAUDE.md (10 pts: 5 exists + 5 size>500)
	if scan.HasClaudeMD {
		points += 5
		found = append(found, "CLAUDE.md")
		if scan.ClaudeMDSize > 500 {
			points += 5
		}
	}

	// AGENTS.md (8 pts: 4 exists + 4 non-empty)
	if scan.HasAgentsMD {
		points += 4
		found = append(found, "AGENTS.md")
		if scan.AgentsMDSize > 0 {
			points += 4
		}
	}

	// .cursorrules (7 pts: 3 exists + 4 size>200)
	if scan.HasCursorRules {
		points += 3
		found = append(found, ".cursorrules")
		if scan.CursorRulesSize > 200 {
			points += 4
		}
	}

	// copilot-instructions.md (5 pts)
	if scan.HasCopilotInstructions {
		points += 5
		found = append(found, "copilot-instructions.md")
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

// scoreCanonicalExamples (25 pts): example_test.go files + CLAUDE.md pattern references.
func scoreCanonicalExamples(scan *domain.ScanResult) domain.SubMetric {
	sm := domain.SubMetric{Name: "canonical_examples", Points: 25}

	if scan == nil {
		sm.Detail = "no scan data"
		return sm
	}

	points := 0
	found := []string{}

	// example_test.go files (15 pts)
	exampleCount := 0
	for _, f := range scan.AllFiles {
		if strings.HasSuffix(f, "example_test.go") || strings.Contains(f, "_example_test.go") {
			exampleCount++
		}
	}
	if exampleCount > 0 {
		if exampleCount >= 3 {
			points += 15
		} else {
			points += exampleCount * 5
		}
		found = append(found, fmt.Sprintf("%d example_test.go files", exampleCount))
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
