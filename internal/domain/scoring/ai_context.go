package scoring

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/openkraft/openkraft/internal/domain"
)

// ScoreAIContext evaluates the quality and presence of AI context files.
// It performs file I/O to check sizes and content of context files.
// Weight: 0.10 (10% of overall score).
func ScoreAIContext(scan *domain.ScanResult) domain.CategoryScore {
	cat := domain.CategoryScore{
		Name:   "ai_context",
		Weight: 0.10,
	}

	if scan == nil {
		cat.SubMetrics = []domain.SubMetric{
			{Name: "claude_md", Points: 25},
			{Name: "cursor_rules", Points: 25},
			{Name: "agents_md", Points: 25},
			{Name: "openkraft_dir", Points: 25},
		}
		return cat
	}

	sm1 := scoreClaudeMD(scan)
	sm2 := scoreCursorRules(scan)
	sm3 := scoreAgentsMD(scan)
	sm4 := scoreOpenkraftDir(scan)

	cat.SubMetrics = []domain.SubMetric{sm1, sm2, sm3, sm4}

	total := 0
	for _, sm := range cat.SubMetrics {
		total += sm.Score
	}
	cat.Score = total

	cat.Issues = collectAIContextIssues(scan)

	return cat
}

// scoreClaudeMD (25 pts): exists (10), >500 bytes (10), has ## headers (5).
func scoreClaudeMD(scan *domain.ScanResult) domain.SubMetric {
	sm := domain.SubMetric{
		Name:   "claude_md",
		Points: 25,
	}

	if !scan.HasClaudeMD {
		sm.Detail = "CLAUDE.md not found"
		return sm
	}

	points := 10 // exists
	sm.Detail = "CLAUDE.md exists"

	data, err := os.ReadFile(filepath.Join(scan.RootPath, "CLAUDE.md"))
	if err != nil {
		sm.Score = points
		sm.Detail = fmt.Sprintf("CLAUDE.md exists but unreadable: %v", err)
		return sm
	}

	if len(data) > 500 {
		points += 10
		sm.Detail = fmt.Sprintf("CLAUDE.md exists (%d bytes)", len(data))
	} else {
		sm.Detail = fmt.Sprintf("CLAUDE.md exists but small (%d bytes, want >500)", len(data))
	}

	if hasMarkdownHeaders(string(data)) {
		points += 5
	}

	if points > sm.Points {
		points = sm.Points
	}
	sm.Score = points
	return sm
}

// scoreCursorRules (25 pts): exists (10), >200 bytes (10), has actionable content (5).
func scoreCursorRules(scan *domain.ScanResult) domain.SubMetric {
	sm := domain.SubMetric{
		Name:   "cursor_rules",
		Points: 25,
	}

	if !scan.HasCursorRules {
		sm.Detail = ".cursorrules not found"
		return sm
	}

	points := 10 // exists
	sm.Detail = ".cursorrules exists"

	data, err := os.ReadFile(filepath.Join(scan.RootPath, ".cursorrules"))
	if err != nil {
		sm.Score = points
		sm.Detail = fmt.Sprintf(".cursorrules exists but unreadable: %v", err)
		return sm
	}

	if len(data) > 200 {
		points += 10
		sm.Detail = fmt.Sprintf(".cursorrules exists (%d bytes)", len(data))
	} else {
		sm.Detail = fmt.Sprintf(".cursorrules exists but small (%d bytes, want >200)", len(data))
	}

	if hasActionableContent(string(data)) {
		points += 5
	}

	if points > sm.Points {
		points = sm.Points
	}
	sm.Score = points
	return sm
}

// scoreAgentsMD (25 pts): exists and has content.
func scoreAgentsMD(scan *domain.ScanResult) domain.SubMetric {
	sm := domain.SubMetric{
		Name:   "agents_md",
		Points: 25,
	}

	if !scan.HasAgentsMD {
		sm.Detail = "AGENTS.md not found"
		return sm
	}

	data, err := os.ReadFile(filepath.Join(scan.RootPath, "AGENTS.md"))
	if err != nil {
		sm.Detail = fmt.Sprintf("AGENTS.md exists but unreadable: %v", err)
		return sm
	}

	content := strings.TrimSpace(string(data))
	if len(content) == 0 {
		sm.Detail = "AGENTS.md exists but is empty"
		return sm
	}

	sm.Score = sm.Points
	sm.Detail = fmt.Sprintf("AGENTS.md exists with content (%d bytes)", len(data))
	return sm
}

// scoreOpenkraftDir (25 pts): exists with manifest.
func scoreOpenkraftDir(scan *domain.ScanResult) domain.SubMetric {
	sm := domain.SubMetric{
		Name:   "openkraft_dir",
		Points: 25,
	}

	if !scan.HasOpenKraftDir {
		sm.Detail = ".openkraft/ directory not found"
		return sm
	}

	// Check for any manifest file inside .openkraft/.
	dirPath := filepath.Join(scan.RootPath, ".openkraft")
	entries, err := os.ReadDir(dirPath)
	if err != nil {
		sm.Detail = fmt.Sprintf(".openkraft/ exists but unreadable: %v", err)
		return sm
	}

	if len(entries) == 0 {
		sm.Score = 10
		sm.Detail = ".openkraft/ exists but is empty"
		return sm
	}

	sm.Score = sm.Points
	sm.Detail = fmt.Sprintf(".openkraft/ exists with %d file(s)", len(entries))
	return sm
}

// hasMarkdownHeaders checks if content contains ## level headers.
func hasMarkdownHeaders(content string) bool {
	for _, line := range strings.Split(content, "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "## ") {
			return true
		}
	}
	return false
}

// hasActionableContent checks if content contains actionable instructions
// (rules, directives, imperative verbs).
func hasActionableContent(content string) bool {
	lower := strings.ToLower(content)
	actionableKeywords := []string{
		"follow", "use ", "must", "should", "always", "never",
		"rule", "require", "ensure", "avoid", "prefer",
	}
	for _, kw := range actionableKeywords {
		if strings.Contains(lower, kw) {
			return true
		}
	}
	return false
}

// collectAIContextIssues gathers issues found during AI context scoring.
func collectAIContextIssues(scan *domain.ScanResult) []domain.Issue {
	var issues []domain.Issue

	if !scan.HasClaudeMD {
		issues = append(issues, domain.Issue{
			Severity:     domain.SeverityWarning,
			Category:     "ai_context",
			Message:      "CLAUDE.md not found; add it to provide AI agents with project context",
			FixAvailable: true,
		})
	}

	if !scan.HasCursorRules {
		issues = append(issues, domain.Issue{
			Severity:     domain.SeverityInfo,
			Category:     "ai_context",
			Message:      ".cursorrules not found; add it for Cursor IDE integration",
			FixAvailable: true,
		})
	}

	if !scan.HasAgentsMD {
		issues = append(issues, domain.Issue{
			Severity:     domain.SeverityInfo,
			Category:     "ai_context",
			Message:      "AGENTS.md not found; add it to describe agent workflows",
			FixAvailable: true,
		})
	}

	if !scan.HasOpenKraftDir {
		issues = append(issues, domain.Issue{
			Severity:     domain.SeverityInfo,
			Category:     "ai_context",
			Message:      ".openkraft/ directory not found; run 'openkraft init' to create it",
			FixAvailable: true,
		})
	}

	return issues
}
