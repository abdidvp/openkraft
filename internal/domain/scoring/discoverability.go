package scoring

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/openkraft/openkraft/internal/domain"
)

// ScoreDiscoverability evaluates how easily AI agents can find relevant code.
// Weight: 0.20 (20% of overall score).
func ScoreDiscoverability(profile *domain.ScoringProfile, modules []domain.DetectedModule, scan *domain.ScanResult, analyzed map[string]*domain.AnalyzedFile) domain.CategoryScore {
	cat := domain.CategoryScore{
		Name:   "discoverability",
		Weight: 0.20,
	}

	sm1 := scoreNamingUniqueness(analyzed)
	sm2 := scoreFileNamingConventions(profile, scan)
	sm3 := scorePredictableStructure(profile, modules, scan)
	sm4 := scoreDiscoverabilityDependencyDirection(modules, analyzed)

	cat.SubMetrics = []domain.SubMetric{sm1, sm2, sm3, sm4}

	total := 0
	for _, sm := range cat.SubMetrics {
		total += sm.Score
	}
	cat.Score = total

	cat.Issues = collectDiscoverabilityIssues(modules, scan, analyzed)
	return cat
}

// scoreNamingUniqueness (25 pts): composite — word count 40% + vocabulary specificity 30% + Shannon entropy 30%.
func scoreNamingUniqueness(analyzed map[string]*domain.AnalyzedFile) domain.SubMetric {
	sm := domain.SubMetric{Name: "naming_uniqueness", Points: 25}

	var names []string
	var totalWCS, totalVS float64
	count := 0

	for _, af := range analyzed {
		for _, fn := range af.Functions {
			if !fn.Exported {
				continue
			}
			names = append(names, fn.Name)
			totalWCS += WordCountScore(fn.Name)
			totalVS += VocabularySpecificity(fn.Name)
			count++
		}
	}

	if count == 0 {
		sm.Detail = "no exported functions found"
		return sm
	}

	avgWCS := totalWCS / float64(count)
	avgVS := totalVS / float64(count)
	entropy := ShannonEntropy(names)

	composite := avgWCS*0.4 + avgVS*0.3 + entropy*0.3
	sm.Score = int(composite * float64(sm.Points))
	if sm.Score > sm.Points {
		sm.Score = sm.Points
	}
	sm.Detail = fmt.Sprintf("word count=%.2f, specificity=%.2f, entropy=%.2f across %d exported functions",
		avgWCS, avgVS, entropy, count)
	return sm
}

// scoreFileNamingConventions (25 pts): measures internal naming consistency.
// Respects profile.NamingConvention: "bare" or "suffixed" enforces that pattern;
// "auto" (default) detects the dominant pattern and scores consistency.
func scoreFileNamingConventions(profile *domain.ScoringProfile, scan *domain.ScanResult) domain.SubMetric {
	sm := domain.SubMetric{Name: "file_naming_conventions", Points: 25}

	if scan == nil || len(scan.GoFiles) == 0 {
		sm.Detail = "no Go files to evaluate"
		return sm
	}

	// Classify non-test, non-main Go files as "bare" or "suffixed".
	bare, suffixed, total := 0, 0, 0
	for _, f := range scan.GoFiles {
		base := filepath.Base(f)
		if strings.HasSuffix(base, "_test.go") {
			continue
		}
		name := strings.TrimSuffix(base, ".go")
		if name == "main" || name == "doc" {
			continue
		}
		total++
		if strings.Contains(name, "_") {
			suffixed++
		} else {
			bare++
		}
	}

	if total == 0 {
		sm.Detail = "no scorable files"
		return sm
	}

	// Determine expected pattern from profile.
	convention := profile.NamingConvention
	var dominant int
	var patternName string

	switch convention {
	case "bare":
		dominant = bare
		patternName = "bare"
	case "suffixed":
		dominant = suffixed
		patternName = "suffixed"
	default: // "auto" or empty — detect dominant pattern.
		if suffixed > bare {
			dominant = suffixed
			patternName = "suffixed"
		} else {
			dominant = bare
			patternName = "bare"
		}
	}

	consistency := float64(dominant) / float64(total)

	// Bonus for suffix reuse among suffixed files.
	if patternName == "suffixed" {
		consistency = (consistency + suffixReuse(scan.GoFiles)) / 2.0
	}

	sm.Score = int(consistency * float64(sm.Points))
	if sm.Score > sm.Points {
		sm.Score = sm.Points
	}
	sm.Detail = fmt.Sprintf("%d/%d files follow %s pattern (%.0f%%)", dominant, total, patternName, consistency*100)
	return sm
}

// suffixReuse returns the ratio of suffixed files whose suffix appears more than once.
func suffixReuse(goFiles []string) float64 {
	counts := map[string]int{}
	total := 0
	for _, f := range goFiles {
		base := filepath.Base(f)
		if strings.HasSuffix(base, "_test.go") {
			continue
		}
		name := strings.TrimSuffix(base, ".go")
		if idx := strings.LastIndex(name, "_"); idx >= 0 {
			counts[name[idx:]]++
			total++
		}
	}
	if total == 0 {
		return 1.0
	}
	reused := 0
	for _, c := range counts {
		if c > 1 {
			reused += c
		}
	}
	return float64(reused) / float64(total)
}

// scorePredictableStructure (25 pts): 3-signal composite measuring structural consistency.
//   - Layer consistency (50%): Jaccard of normalized layer sets across modules.
//   - Suffix Jaccard (30%): Jaccard of role-indicating file suffixes across modules.
//     When naming convention is "bare", suffix Jaccard is replaced with full credit.
//   - File count similarity (20%): min(a,b)/max(a,b) averaged across pairs.
func scorePredictableStructure(profile *domain.ScoringProfile, modules []domain.DetectedModule, scan *domain.ScanResult) domain.SubMetric {
	sm := domain.SubMetric{Name: "predictable_structure", Points: 25}

	if len(modules) <= 1 {
		if len(modules) == 1 {
			sm.Score = sm.Points
			sm.Detail = "single module, nothing to compare"
		} else {
			sm.Detail = "no modules detected"
		}
		return sm
	}

	// Detect naming convention: explicit from profile, or auto-detect from scan.
	isBareNaming := false
	switch profile.NamingConvention {
	case "bare":
		isBareNaming = true
	case "suffixed":
		isBareNaming = false
	default: // "auto" — detect from scan
		if scan != nil {
			bare, suffixed := 0, 0
			for _, f := range scan.GoFiles {
				base := filepath.Base(f)
				if strings.HasSuffix(base, "_test.go") {
					continue
				}
				name := strings.TrimSuffix(base, ".go")
				if name == "main" || name == "doc" {
					continue
				}
				if strings.Contains(name, "_") {
					suffixed++
				} else {
					bare++
				}
			}
			isBareNaming = bare > suffixed
		}
	}

	// Build per-module layer sets, suffix sets, and file counts.
	layerSets := make([]map[string]bool, len(modules))
	suffixSets := make([]map[string]bool, len(modules))
	fileCounts := make([]int, len(modules))

	for i, m := range modules {
		layerSets[i] = make(map[string]bool)
		for _, l := range m.Layers {
			layerSets[i][l] = true
		}

		suffixSets[i] = make(map[string]bool)
		nonTestFiles := 0
		for _, f := range m.Files {
			base := filepath.Base(f)
			name := strings.TrimSuffix(base, ".go")
			if strings.HasSuffix(name, "_test") {
				continue
			}
			nonTestFiles++
			if idx := strings.LastIndex(name, "_"); idx >= 0 {
				suffixSets[i][name[idx:]] = true
			} else {
				suffixSets[i][name] = true
			}
		}
		fileCounts[i] = nonTestFiles
	}

	// Average pairwise scores — only compare modules sharing at least one layer.
	var totalLayer, totalSuffix, totalFileCount float64
	pairs := 0
	for i := 0; i < len(modules); i++ {
		for j := i + 1; j < len(modules); j++ {
			if !sharesLayer(layerSets[i], layerSets[j]) {
				continue
			}
			totalLayer += jaccard(layerSets[i], layerSets[j])
			if isBareNaming {
				// Bare naming has no meaningful suffixes — award full credit
				// since consistency is measured by scoreFileNamingConventions.
				totalSuffix += 1.0
			} else {
				totalSuffix += jaccard(suffixSets[i], suffixSets[j])
			}
			a, b := float64(fileCounts[i]), float64(fileCounts[j])
			if a > 0 || b > 0 {
				mn, mx := a, b
				if mn > mx {
					mn, mx = mx, mn
				}
				totalFileCount += mn / mx
			} else {
				totalFileCount += 1.0
			}
			pairs++
		}
	}

	if pairs == 0 {
		sm.Score = sm.Points
		sm.Detail = fmt.Sprintf("no comparable module pairs across %d modules", len(modules))
		return sm
	}

	avgLayer := totalLayer / float64(pairs)
	avgSuffix := totalSuffix / float64(pairs)
	avgFileCount := totalFileCount / float64(pairs)

	composite := avgLayer*0.5 + avgSuffix*0.3 + avgFileCount*0.2
	sm.Score = int(composite * float64(sm.Points))
	if sm.Score > sm.Points {
		sm.Score = sm.Points
	}
	suffixLabel := fmt.Sprintf("suffixes=%.0f%%", avgSuffix*100)
	if isBareNaming {
		suffixLabel = "naming=bare(ok)"
	}
	sm.Detail = fmt.Sprintf("layers=%.0f%%, %s, file-count=%.0f%% across %d modules",
		avgLayer*100, suffixLabel, avgFileCount*100, len(modules))
	return sm
}

// scoreDiscoverabilityDependencyDirection (25 pts): import violations (adapter→adapter, domain→application).
// Migrated from architecture.go:scoreDependencyDirection.
func scoreDiscoverabilityDependencyDirection(modules []domain.DetectedModule, analyzed map[string]*domain.AnalyzedFile) domain.SubMetric {
	sm := domain.SubMetric{Name: "dependency_direction", Points: 25}

	if len(modules) == 0 {
		sm.Detail = "no modules to evaluate"
		return sm
	}

	totalChecked := 0
	violations := 0

	for _, m := range modules {
		for _, f := range m.Files {
			if strings.HasSuffix(f, "_test.go") {
				continue
			}
			af, ok := analyzed[f]
			if !ok {
				continue
			}
			layer := fileLayer(f)
			if layer == "" {
				continue
			}
			totalChecked++
			for _, imp := range af.Imports {
				if violatesDependencyDirection(layer, imp) {
					violations++
				}
			}
		}
	}

	if totalChecked == 0 {
		sm.Detail = "no layered files to evaluate"
		return sm
	}

	if violations == 0 {
		sm.Score = sm.Points
		sm.Detail = fmt.Sprintf("all %d layered files follow correct dependency direction", totalChecked)
	} else {
		penalty := violations * 5
		sm.Score = sm.Points - penalty
		if sm.Score < 0 {
			sm.Score = 0
		}
		sm.Detail = fmt.Sprintf("%d dependency direction violation(s) found", violations)
	}
	return sm
}

func jaccard(a, b map[string]bool) float64 {
	if len(a) == 0 && len(b) == 0 {
		return 1.0
	}
	intersection := 0
	union := make(map[string]bool)
	for k := range a {
		union[k] = true
	}
	for k := range b {
		union[k] = true
		if a[k] {
			intersection++
		}
	}
	if len(union) == 0 {
		return 1.0
	}
	return float64(intersection) / float64(len(union))
}

func collectDiscoverabilityIssues(modules []domain.DetectedModule, _ *domain.ScanResult, analyzed map[string]*domain.AnalyzedFile) []domain.Issue {
	var issues []domain.Issue

	// Flag dependency violations (production code only).
	for _, m := range modules {
		for _, f := range m.Files {
			if strings.HasSuffix(f, "_test.go") {
				continue
			}
			af, ok := analyzed[f]
			if !ok {
				continue
			}
			layer := fileLayer(f)
			for _, imp := range af.Imports {
				if violatesDependencyDirection(layer, imp) {
					issues = append(issues, domain.Issue{
						Severity: domain.SeverityError,
						Category: "discoverability",
						File:     f,
						Message:  fmt.Sprintf("%s layer imports %s (dependency direction violation)", layer, imp),
					})
				}
			}
		}
	}

	return issues
}

// sharesLayer returns true if the two layer sets have at least one layer in common.
func sharesLayer(a, b map[string]bool) bool {
	for k := range a {
		if b[k] {
			return true
		}
	}
	return false
}
