package scoring

import (
	"fmt"
	"strings"

	"github.com/openkraft/openkraft/internal/domain"
)

// ScoreTests evaluates the test infrastructure quality of a project.
// Weight: 0.15 (15% of overall score).
//
// Sub-metrics (100 points total):
//   - unit_test_presence (25 pts): ratio of test files to source files
//   - integration_tests  (25 pts): presence of integration/e2e test dirs or files
//   - test_helpers       (15 pts): test helper functions, testutil dirs
//   - test_fixtures      (15 pts): testdata directory, fixture files
//   - ci_config          (20 pts): CI/CD configuration for running tests
func ScoreTests(scan *domain.ScanResult) domain.CategoryScore {
	cat := domain.CategoryScore{
		Name:   "tests",
		Weight: 0.15,
	}

	cat.SubMetrics = []domain.SubMetric{
		scoreUnitTestPresence(scan),
		scoreIntegrationTests(scan),
		scoreTestHelpers(scan),
		scoreTestFixtures(scan),
		scoreCIConfig(scan),
	}

	total := 0
	for _, m := range cat.SubMetrics {
		total += m.Score
	}
	cat.Score = total

	// Generate issues
	cat.Issues = generateTestIssues(scan, cat.SubMetrics)

	return cat
}

// scoreUnitTestPresence awards up to 25 points based on the ratio of test files
// to non-test Go source files.
func scoreUnitTestPresence(scan *domain.ScanResult) domain.SubMetric {
	const maxPoints = 25

	sourceCount := len(scan.GoFiles) - len(scan.TestFiles)
	testCount := len(scan.TestFiles)

	if sourceCount <= 0 {
		return domain.SubMetric{
			Name:   "unit_test_presence",
			Score:  0,
			Points: maxPoints,
			Detail: "no Go source files found",
		}
	}

	ratio := float64(testCount) / float64(sourceCount)
	// A ratio of 0.5 or above earns full marks (one test file per two source files is good).
	score := int(ratio / 0.5 * float64(maxPoints))
	if score > maxPoints {
		score = maxPoints
	}

	return domain.SubMetric{
		Name:   "unit_test_presence",
		Score:  score,
		Points: maxPoints,
		Detail: fmt.Sprintf("%d test files for %d source files (ratio %.2f)", testCount, sourceCount, ratio),
	}
}

// scoreIntegrationTests awards up to 25 points for presence of integration
// or end-to-end test directories and files.
func scoreIntegrationTests(scan *domain.ScanResult) domain.SubMetric {
	const maxPoints = 25

	score := 0
	found := []string{}

	for _, f := range scan.AllFiles {
		lower := strings.ToLower(f)

		// Check for integration/e2e directory patterns
		if containsAny(lower, "test/integration", "tests/integration", "integration_test", "e2e", "test/e2e", "tests/e2e") {
			if !containsString(found, "integration/e2e directory") {
				found = append(found, "integration/e2e directory")
				score += 15
			}
		}

		// Check for integration test files (files with "integration" in name)
		if strings.HasSuffix(lower, "_test.go") && strings.Contains(lower, "integration") {
			if !containsString(found, "integration test files") {
				found = append(found, "integration test files")
				score += 10
			}
		}
	}

	if score > maxPoints {
		score = maxPoints
	}

	detail := "no integration or e2e tests found"
	if len(found) > 0 {
		detail = fmt.Sprintf("found: %s", strings.Join(found, ", "))
	}

	return domain.SubMetric{
		Name:   "integration_tests",
		Score:  score,
		Points: maxPoints,
		Detail: detail,
	}
}

// scoreTestHelpers awards up to 15 points for test helper infrastructure.
func scoreTestHelpers(scan *domain.ScanResult) domain.SubMetric {
	const maxPoints = 15

	score := 0
	found := []string{}

	for _, f := range scan.AllFiles {
		lower := strings.ToLower(f)

		// testutil or testhelper directories
		if containsAny(lower, "testutil/", "testhelper/", "testhelpers/", "testing/") {
			if !containsString(found, "test helper directory") {
				found = append(found, "test helper directory")
				score += 10
			}
		}

		// Helper files (e.g. helpers_test.go, test_helpers.go)
		if containsAny(lower, "helper", "mock", "fake", "stub") && strings.HasSuffix(lower, ".go") {
			if !containsString(found, "helper/mock files") {
				found = append(found, "helper/mock files")
				score += 5
			}
		}
	}

	if score > maxPoints {
		score = maxPoints
	}

	detail := "no test helpers found"
	if len(found) > 0 {
		detail = fmt.Sprintf("found: %s", strings.Join(found, ", "))
	}

	return domain.SubMetric{
		Name:   "test_helpers",
		Score:  score,
		Points: maxPoints,
		Detail: detail,
	}
}

// scoreTestFixtures awards up to 15 points for testdata directories and fixture files.
func scoreTestFixtures(scan *domain.ScanResult) domain.SubMetric {
	const maxPoints = 15

	score := 0
	found := []string{}

	for _, f := range scan.AllFiles {
		lower := strings.ToLower(f)

		// testdata directory
		if strings.Contains(lower, "testdata/") {
			if !containsString(found, "testdata directory") {
				found = append(found, "testdata directory")
				score += 10
			}
		}

		// fixture files (json, yaml, sql, etc. in test-related dirs)
		if containsAny(lower, "fixture", "golden", "snapshot") {
			if !containsString(found, "fixture/golden files") {
				found = append(found, "fixture/golden files")
				score += 5
			}
		}
	}

	if score > maxPoints {
		score = maxPoints
	}

	detail := "no test fixtures found"
	if len(found) > 0 {
		detail = fmt.Sprintf("found: %s", strings.Join(found, ", "))
	}

	return domain.SubMetric{
		Name:   "test_fixtures",
		Score:  score,
		Points: maxPoints,
		Detail: detail,
	}
}

// scoreCIConfig awards up to 20 points for CI/CD configuration that runs tests.
func scoreCIConfig(scan *domain.ScanResult) domain.SubMetric {
	const maxPoints = 20

	score := 0
	found := []string{}

	if scan.HasCIConfig {
		found = append(found, "CI config")
		score += 10
	}

	for _, f := range scan.AllFiles {
		lower := strings.ToLower(f)

		// GitHub Actions workflows
		if strings.Contains(lower, ".github/workflows/") {
			if !containsString(found, "GitHub Actions") {
				found = append(found, "GitHub Actions")
				score += 5
			}
		}

		// Makefile (likely has test target)
		if lower == "makefile" || lower == "gnumakefile" {
			if !containsString(found, "Makefile") {
				found = append(found, "Makefile")
				score += 5
			}
		}

		// Other CI systems
		if containsAny(lower, ".gitlab-ci", ".circleci", "jenkinsfile", ".travis.yml") {
			if !containsString(found, "CI system") {
				found = append(found, "CI system")
				score += 5
			}
		}
	}

	if score > maxPoints {
		score = maxPoints
	}

	detail := "no CI configuration found"
	if len(found) > 0 {
		detail = fmt.Sprintf("found: %s", strings.Join(found, ", "))
	}

	return domain.SubMetric{
		Name:   "ci_config",
		Score:  score,
		Points: maxPoints,
		Detail: detail,
	}
}

// generateTestIssues creates issues for missing test infrastructure.
func generateTestIssues(scan *domain.ScanResult, metrics []domain.SubMetric) []domain.Issue {
	var issues []domain.Issue

	for _, m := range metrics {
		if m.Score == 0 {
			severity := domain.SeverityWarning
			if m.Name == "unit_test_presence" {
				severity = domain.SeverityError
			}
			issues = append(issues, domain.Issue{
				Severity: severity,
				Category: "tests",
				Message:  fmt.Sprintf("missing %s: %s", m.Name, m.Detail),
			})
		}
	}

	return issues
}

// containsAny checks if s contains any of the given substrings.
func containsAny(s string, subs ...string) bool {
	for _, sub := range subs {
		if strings.Contains(s, sub) {
			return true
		}
	}
	return false
}

// containsString checks if a slice contains a string.
func containsString(slice []string, s string) bool {
	for _, item := range slice {
		if item == s {
			return true
		}
	}
	return false
}
