package scoring

import (
	"fmt"
	"strings"

	"github.com/openkraft/openkraft/internal/domain"
)

// ScoreVerifiability evaluates how easily AI agents can verify their changes.
// Weight: 0.15 (15% of overall score).
func ScoreVerifiability(profile *domain.ScoringProfile, scan *domain.ScanResult, analyzed map[string]*domain.AnalyzedFile) domain.CategoryScore {
	cat := domain.CategoryScore{
		Name:   "verifiability",
		Weight: 0.15,
	}

	sm1 := scoreTestPresence(profile, scan)
	sm2 := scoreTestNaming(scan, analyzed)
	sm3 := scoreBuildReproducibility(scan)
	sm4 := scoreTypeSafetySignals(scan, analyzed)

	cat.SubMetrics = []domain.SubMetric{sm1, sm2, sm3, sm4}

	total := 0
	for _, sm := range cat.SubMetrics {
		total += sm.Score
	}
	cat.Score = total

	cat.Issues = collectVerifiabilityIssues(scan, cat.SubMetrics)
	return cat
}

// scoreTestPresence (25 pts): ratio of .go files with _test.go.
// Uses profile.MinTestRatio as the target for full credit.
func scoreTestPresence(profile *domain.ScoringProfile, scan *domain.ScanResult) domain.SubMetric {
	sm := domain.SubMetric{Name: "test_presence", Points: 25}

	sourceCount := len(scan.GoFiles) - len(scan.TestFiles)
	testCount := len(scan.TestFiles)

	if sourceCount <= 0 {
		sm.Detail = "no Go source files found"
		return sm
	}

	ratio := float64(testCount) / float64(sourceCount)
	target := profile.MinTestRatio
	if target <= 0 {
		target = 0.5
	}
	score := int(ratio / target * float64(sm.Points))
	if score > sm.Points {
		score = sm.Points
	}
	sm.Score = score
	sm.Detail = fmt.Sprintf("%d test files for %d source files (ratio %.2f, target %.2f)", testCount, sourceCount, ratio, target)
	return sm
}

// scoreTestNaming (25 pts): Test<Func>_<Scenario> pattern + t.Run subtests.
func scoreTestNaming(_ *domain.ScanResult, analyzed map[string]*domain.AnalyzedFile) domain.SubMetric {
	sm := domain.SubMetric{Name: "test_naming", Points: 25}

	totalTests := 0
	wellNamed := 0

	for _, af := range analyzed {
		if !strings.HasSuffix(af.Path, "_test.go") {
			continue
		}
		for _, fn := range af.Functions {
			if !strings.HasPrefix(fn.Name, "Test") {
				continue
			}
			totalTests++
			// Good naming: Test<Something>_<Scenario> (contains underscore after Test prefix)
			after := strings.TrimPrefix(fn.Name, "Test")
			if strings.Contains(after, "_") {
				wellNamed++
			}
		}
	}

	if totalTests == 0 {
		sm.Detail = "no test functions found"
		return sm
	}

	ratio := float64(wellNamed) / float64(totalTests)
	sm.Score = int(ratio * float64(sm.Points))
	if sm.Score > sm.Points {
		sm.Score = sm.Points
	}
	sm.Detail = fmt.Sprintf("%d/%d test functions follow Test<Func>_<Scenario> naming", wellNamed, totalTests)
	return sm
}

// scoreBuildReproducibility (25 pts): go.sum (10), Makefile/Taskfile (8), CI config (7).
func scoreBuildReproducibility(scan *domain.ScanResult) domain.SubMetric {
	sm := domain.SubMetric{Name: "build_reproducibility", Points: 25}

	if scan == nil {
		sm.Detail = "no scan data"
		return sm
	}

	points := 0
	found := []string{}

	// go.sum (10 pts)
	for _, f := range scan.AllFiles {
		if f == "go.sum" {
			points += 10
			found = append(found, "go.sum")
			break
		}
	}

	// Makefile/Taskfile (8 pts)
	for _, f := range scan.AllFiles {
		lower := strings.ToLower(f)
		if lower == "makefile" || lower == "taskfile.yml" || lower == "taskfile.yaml" {
			points += 8
			found = append(found, f)
			break
		}
	}

	// CI config (7 pts)
	if scan.HasCIConfig {
		points += 7
		found = append(found, "CI config")
	}

	if points > sm.Points {
		points = sm.Points
	}
	sm.Score = points
	if len(found) > 0 {
		sm.Detail = fmt.Sprintf("found: %s", strings.Join(found, ", "))
	} else {
		sm.Detail = "no build reproducibility signals found"
	}
	return sm
}

// scoreTypeSafetySignals (25 pts): .golangci.yml (10), low interface{}/any (10), safe type assertions (5).
func scoreTypeSafetySignals(scan *domain.ScanResult, analyzed map[string]*domain.AnalyzedFile) domain.SubMetric {
	sm := domain.SubMetric{Name: "type_safety_signals", Points: 25}

	points := 0

	// .golangci.yml (10 pts)
	if scan != nil {
		for _, f := range scan.AllFiles {
			if f == ".golangci.yml" || f == ".golangci.yaml" {
				points += 10
				break
			}
		}
	}

	// Low interface{}/any usage (10 pts) — check param types across all functions.
	totalParams := 0
	emptyInterfaceParams := 0
	for _, af := range analyzed {
		for _, fn := range af.Functions {
			for _, p := range fn.Params {
				totalParams++
				if p.Type == "interface{}" || p.Type == "any" {
					emptyInterfaceParams++
				}
			}
		}
	}
	if totalParams > 0 {
		ratio := float64(emptyInterfaceParams) / float64(totalParams)
		if ratio < 0.05 {
			points += 10
		} else if ratio < 0.15 {
			points += 5
		}
	} else {
		points += 10 // No params to check = clean
	}

	// Safe type assertions (5 pts)
	totalAssertions := 0
	safeAssertions := 0
	for _, af := range analyzed {
		for _, ta := range af.TypeAssertions {
			totalAssertions++
			if ta.Safe {
				safeAssertions++
			}
		}
	}
	if totalAssertions == 0 {
		points += 5 // No assertions = clean
	} else if float64(safeAssertions)/float64(totalAssertions) >= 0.8 {
		points += 5
	} else if float64(safeAssertions)/float64(totalAssertions) >= 0.5 {
		points += 3
	}

	if points > sm.Points {
		points = sm.Points
	}
	sm.Score = points
	sm.Detail = fmt.Sprintf("linter config, %d/%d safe type assertions, %d/%d clean params",
		safeAssertions, totalAssertions, totalParams-emptyInterfaceParams, totalParams)
	return sm
}

func collectVerifiabilityIssues(_ *domain.ScanResult, metrics []domain.SubMetric) []domain.Issue {
	var issues []domain.Issue

	for _, m := range metrics {
		if m.Score == 0 {
			severity := domain.SeverityWarning
			if m.Name == "test_presence" {
				severity = domain.SeverityError
			}
			issues = append(issues, domain.Issue{
				Severity: severity,
				Category: "verifiability",
				Message:  fmt.Sprintf("missing %s: %s", m.Name, m.Detail),
			})
		}
	}

	return issues
}
