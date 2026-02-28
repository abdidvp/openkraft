package scoring

import (
	"fmt"
	"math"
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
	sm2 := scoreFileNamingConventions(profile, scan, analyzed)
	sm3 := scorePredictableStructure(profile, modules, scan, analyzed)
	sm4 := scoreDiscoverabilityDependencyDirection(modules, analyzed)

	cat.SubMetrics = []domain.SubMetric{sm1, sm2, sm3, sm4}

	total := 0
	for _, sm := range cat.SubMetrics {
		total += sm.Score
	}
	cat.Score = total

	cat.Issues = collectDiscoverabilityIssues(profile, modules, scan, analyzed)
	return cat
}

// scoreNamingUniqueness (25 pts): composite — word count 40% + vocabulary specificity 30% + Shannon entropy 30%.
func scoreNamingUniqueness(analyzed map[string]*domain.AnalyzedFile) domain.SubMetric {
	sm := domain.SubMetric{Name: "naming_uniqueness", Points: 25}

	var names []string
	var totalWCS, totalVS float64
	count := 0
	descriptive := 0

	for _, af := range analyzed {
		for _, fn := range af.Functions {
			if !fn.Exported {
				continue
			}
			names = append(names, fn.Name)
			wcs := WordCountScore(fn.Name)
			totalWCS += wcs
			totalVS += VocabularySpecificity(fn.Name)
			count++
			if wcs >= 0.7 {
				descriptive++
			}
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
	sm.Score = min(int(math.Round(composite*float64(sm.Points))), sm.Points)
	sm.Detail = fmt.Sprintf("%d of %d exported functions have descriptive names (2+ words)",
		descriptive, count)
	return sm
}

// scoreFileNamingConventions (25 pts): measures internal naming consistency.
// Respects profile.NamingConvention: "bare" or "suffixed" enforces that pattern;
// "auto" (default) detects the dominant pattern and scores consistency.
func scoreFileNamingConventions(profile *domain.ScoringProfile, scan *domain.ScanResult, analyzed map[string]*domain.AnalyzedFile) domain.SubMetric {
	sm := domain.SubMetric{Name: "file_naming_conventions", Points: 25}

	if scan == nil || len(scan.GoFiles) == 0 {
		sm.Detail = "no Go files to evaluate"
		return sm
	}

	c := classifyFileNaming(profile, scan.GoFiles, analyzed)
	if c.total == 0 {
		sm.Detail = "no scorable files"
		return sm
	}

	consistency := c.consistency
	patternName := "bare"
	if c.dominantIsSuffixed {
		patternName = "suffixed"
		consistency = (c.consistency + suffixReuse(scan.GoFiles)) / 2.0
	}

	sm.Score = min(int(math.Round(consistency*float64(sm.Points))), sm.Points)
	if patternName == "suffixed" {
		sm.Detail = fmt.Sprintf("%d/%d files follow suffixed pattern (%.0f%% raw, %.0f%% with suffix reuse)",
			c.dominantCount, c.total, c.consistency*100, consistency*100)
	} else {
		sm.Detail = fmt.Sprintf("%d/%d files follow bare pattern (%.0f%%)",
			c.dominantCount, c.total, c.consistency*100)
	}
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
func scorePredictableStructure(profile *domain.ScoringProfile, modules []domain.DetectedModule, scan *domain.ScanResult, analyzed map[string]*domain.AnalyzedFile) domain.SubMetric {
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

	// Detect naming convention via shared classifier.
	isBareNaming := false
	if scan != nil {
		c := classifyFileNaming(profile, scan.GoFiles, analyzed)
		isBareNaming = !c.dominantIsSuffixed
	} else if profile.NamingConvention == "bare" {
		isBareNaming = true
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
				suffix := name[idx:]
				for _, expected := range profile.ExpectedFileSuffixes {
					if suffix == expected {
						suffixSets[i][suffix] = true
						break
					}
				}
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
	sm.Score = min(int(math.Round(composite*float64(sm.Points))), sm.Points)
	suffixLabel := fmt.Sprintf("suffixes=%.0f%%", avgSuffix*100)
	if isBareNaming {
		suffixLabel = "naming=bare(ok)"
	}
	sm.Detail = fmt.Sprintf("layers=%.0f%%, %s, file-count=%.0f%% across %d module pairs",
		avgLayer*100, suffixLabel, avgFileCount*100, pairs)
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

func collectDiscoverabilityIssues(profile *domain.ScoringProfile, modules []domain.DetectedModule, scan *domain.ScanResult, analyzed map[string]*domain.AnalyzedFile) []domain.Issue {
	var issues []domain.Issue

	// 1. naming_uniqueness: flag exported single-word functions (WCS < 0.7).
	//    Skip: generated files, test files, Go interface methods, methods with receiver + single word.
	for _, af := range analyzed {
		if af.IsGenerated || strings.HasSuffix(af.Path, "_test.go") {
			continue
		}
		for _, fn := range af.Functions {
			if !fn.Exported {
				continue
			}
			if fn.Receiver != "" && WordCount(fn.Name) == 1 {
				// Methods with a receiver provide context through the type name
				// (e.g., (*User).String(), (*Repo).Close()) — works for any
				// interface in any project, no hardcoded exemption list needed.
				continue
			}
			if WordCountScore(fn.Name) < 0.7 {
				issues = append(issues, domain.Issue{
					Severity:  domain.SeverityInfo,
					Category:  "discoverability",
					SubMetric: "naming_uniqueness",
					File:      af.Path,
					Line:      fn.LineStart,
					Message:   fmt.Sprintf("exported function %q has a single-word name; consider a verb+noun pattern", fn.Name),
				})
			}
		}
	}

	// 2. file_naming_conventions: flag files violating dominant pattern.
	//    Only when dominant pattern has ≥60% consistency to avoid FP on 50/50 splits.
	//    Skips generated files via classifyFileNaming.
	if scan != nil && len(scan.GoFiles) > 0 {
		c := classifyFileNaming(profile, scan.GoFiles, analyzed)
		if c.total > 0 && c.consistency >= 0.60 {
			for _, f := range scan.GoFiles {
				base := filepath.Base(f)
				if strings.HasSuffix(base, "_test.go") {
					continue
				}
				name := strings.TrimSuffix(base, ".go")
				if name == "main" || name == "doc" {
					continue
				}
				if af, ok := analyzed[f]; ok && af.IsGenerated {
					continue
				}
				isSuffixed := strings.Contains(name, "_")
				if c.dominantIsSuffixed && !isSuffixed {
					issues = append(issues, domain.Issue{
						Severity:  domain.SeverityInfo,
						Category:  "discoverability",
						SubMetric: "file_naming_conventions",
						File:      f,
						Message:   fmt.Sprintf("file %q uses bare naming but project uses suffixed pattern", base),
					})
				} else if !c.dominantIsSuffixed && isSuffixed {
					issues = append(issues, domain.Issue{
						Severity:  domain.SeverityInfo,
						Category:  "discoverability",
						SubMetric: "file_naming_conventions",
						File:      f,
						Message:   fmt.Sprintf("file %q uses suffixed naming but project uses bare pattern", base),
					})
				}
			}
		}
	}

	// 3. predictable_structure: flag modules missing layers that >50% of peers have.
	if len(modules) > 1 {
		layerCount := map[string]int{}
		for _, m := range modules {
			for _, l := range m.Layers {
				layerCount[l]++
			}
		}
		threshold := len(modules) / 2
		for _, m := range modules {
			has := map[string]bool{}
			for _, l := range m.Layers {
				has[l] = true
			}
			for layer, count := range layerCount {
				if count > threshold && !has[layer] {
					issues = append(issues, domain.Issue{
						Severity:  domain.SeverityInfo,
						Category:  "discoverability",
						SubMetric: "predictable_structure",
						File:      m.Path,
						Message:   fmt.Sprintf("module %q is missing %q layer that %d/%d peers have", m.Name, layer, count, len(modules)),
					})
				}
			}
		}
	}

	// 4. dependency_direction: flag import violations (production code only).
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
						Severity:  domain.SeverityError,
						Category:  "discoverability",
						SubMetric: "dependency_direction",
						File:      f,
						Message:   fmt.Sprintf("%s layer imports %s (dependency direction violation)", layer, imp),
					})
				}
			}
		}
	}

	return issues
}

// fileClassification holds the result of classifying Go files by naming convention.
type fileClassification struct {
	bare, suffixed, total int
	dominantIsSuffixed    bool
	dominantCount         int
	consistency           float64
}

// classifyFileNaming classifies Go source files as bare or suffixed and determines
// the dominant convention. Skips test files, main.go, doc.go, and generated files.
func classifyFileNaming(profile *domain.ScoringProfile, goFiles []string, analyzed map[string]*domain.AnalyzedFile) fileClassification {
	var c fileClassification
	for _, f := range goFiles {
		base := filepath.Base(f)
		if strings.HasSuffix(base, "_test.go") {
			continue
		}
		name := strings.TrimSuffix(base, ".go")
		if name == "main" || name == "doc" {
			continue
		}
		if af, ok := analyzed[f]; ok && af.IsGenerated {
			continue
		}
		c.total++
		if strings.Contains(name, "_") {
			c.suffixed++
		} else {
			c.bare++
		}
	}

	if c.total == 0 {
		return c
	}

	switch profile.NamingConvention {
	case "bare":
		c.dominantIsSuffixed = false
		c.dominantCount = c.bare
	case "suffixed":
		c.dominantIsSuffixed = true
		c.dominantCount = c.suffixed
	default:
		if c.suffixed > c.bare {
			c.dominantIsSuffixed = true
			c.dominantCount = c.suffixed
		} else {
			c.dominantIsSuffixed = false
			c.dominantCount = c.bare
		}
	}
	c.consistency = float64(c.dominantCount) / float64(c.total)
	return c
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
