package scoring

import (
	"fmt"
	"math"
	"path/filepath"
	"strings"

	"github.com/abdidvp/openkraft/internal/domain"
)

// ScoreDiscoverability evaluates how easily AI agents can find relevant code.
// Weight: 0.20 (20% of overall score).
func ScoreDiscoverability(profile *domain.ScoringProfile, modules []domain.DetectedModule, scan *domain.ScanResult, analyzed map[string]*domain.AnalyzedFile) domain.CategoryScore {
	if profile == nil {
		p := domain.DefaultProfile()
		profile = &p
	}

	cat := domain.CategoryScore{
		Name:   "discoverability",
		Weight: 0.20,
	}

	sm1 := scoreNamingUniqueness(profile, analyzed)
	sm2 := scoreFileNamingConventions(profile, scan, analyzed)
	sm3 := scorePredictableStructure(profile, modules, scan, analyzed)
	sm4 := scoreDiscoverabilityDependencyDirection(profile, modules, scan, analyzed)

	cat.SubMetrics = []domain.SubMetric{sm1, sm2, sm3, sm4}

	base := 0
	for _, sm := range cat.SubMetrics {
		base += sm.Score
	}

	cat.Issues = collectDiscoverabilityIssues(profile, modules, scan, analyzed)

	funcCount := countExportedFunctions(analyzed)
	if funcCount > 0 {
		cat.Score = max(0, base-severityPenalty(cat.Issues, funcCount))
	} else {
		cat.Score = base
	}

	return cat
}

// scoreNamingUniqueness (25 pts): composite — WCS, specificity, entropy, collision rate.
func scoreNamingUniqueness(profile *domain.ScoringProfile, analyzed map[string]*domain.AnalyzedFile) domain.SubMetric {
	sm := domain.SubMetric{Name: "naming_uniqueness", Points: 25}

	var names []string
	var totalWCS, totalVS float64
	count := 0
	descriptive := 0

	minWCS := profile.MinNamingWordScore
	if minWCS <= 0 {
		minWCS = 0.7
	}
	w := profile.NamingCompositeWeights
	cw := profile.CollisionWeight
	if w == [3]float64{} {
		w = [3]float64{0.30, 0.30, 0.25}
		cw = 0.15
	}

	domainVocab := ExtractDomainVocabulary(analyzed)

	for _, af := range analyzed {
		if af.IsGenerated {
			continue
		}
		for _, fn := range af.Functions {
			if !fn.Exported {
				continue
			}
			names = append(names, fn.Name)
			wcs := WordCountScore(fn.Name)
			totalWCS += wcs
			totalVS += IdentifierSpecificity(fn.Name, domainVocab)
			count++
			if wcs >= minWCS {
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
	collisionRate := SymbolCollisionRate(analyzed)

	composite := avgWCS*w[0] + avgVS*w[1] + entropy*w[2] + (1-collisionRate)*cw
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
		consistency = (c.consistency + suffixReuse(scan.GoFiles, profile.ExpectedFileSuffixes)) / 2.0
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
func suffixReuse(goFiles []string, expectedSuffixes []string) float64 {
	counts := map[string]int{}
	total := 0
	for _, f := range goFiles {
		base := filepath.Base(f)
		if strings.HasSuffix(base, "_test.go") {
			continue
		}
		name := strings.TrimSuffix(base, ".go")
		if hasKnownSuffix(name, expectedSuffixes) {
			idx := strings.LastIndex(name, "_")
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
		sm.Score = sm.Points
		if len(modules) == 1 {
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

// scoreDiscoverabilityDependencyDirection (25 pts): composite of layer violations and import graph signals.
// Layer violations (50%): adapter→adapter, domain→application import direction checks.
// Import graph (50%): cycles, distance from main sequence, coupling outliers.
// When either signal has no data, the other gets 100% weight.
func scoreDiscoverabilityDependencyDirection(profile *domain.ScoringProfile, modules []domain.DetectedModule, scan *domain.ScanResult, analyzed map[string]*domain.AnalyzedFile) domain.SubMetric {
	sm := domain.SubMetric{Name: "dependency_direction", Points: 25}

	// Layer violations
	layerScore, violations, totalChecked := scoreLayerViolations(profile, modules, analyzed)

	// Import graph
	var graph *ImportGraph
	if scan != nil && scan.ModulePath != "" {
		graph = BuildImportGraph(scan.ModulePath, analyzed)
	}
	graphScore := scoreImportGraph(graph, profile)

	// Composite weighting
	if totalChecked == 0 && (graph == nil || len(graph.Packages) <= 1) {
		sm.Score = sm.Points
		sm.Detail = "no layered files or import graph to evaluate"
		return sm
	}

	layerWeight := 0.50
	graphWeight := 0.50
	if totalChecked == 0 {
		layerWeight = 0.0
		graphWeight = 1.0
	}
	if graph == nil || len(graph.Packages) <= 1 {
		layerWeight = 1.0
		graphWeight = 0.0
	}

	composite := layerScore*layerWeight + graphScore*graphWeight
	sm.Score = min(int(math.Round(composite*float64(sm.Points))), sm.Points)
	sm.Detail = formatDependencyDetail(violations, totalChecked, graph, graphScore)
	return sm
}

// scoreLayerViolations checks import direction violations in layered architectures.
// Returns (cleanRate 0.0-1.0, violationCount, totalChecked).
func scoreLayerViolations(profile *domain.ScoringProfile, modules []domain.DetectedModule, analyzed map[string]*domain.AnalyzedFile) (float64, int, int) {
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
			layer := fileLayer(f, profile)
			if layer == "" {
				continue
			}
			totalChecked++
			for _, imp := range af.Imports {
				if violatesDependencyDirection(layer, imp, profile) {
					violations++
				}
			}
		}
	}

	if totalChecked == 0 {
		return 1.0, 0, 0
	}

	rate := max(0, 1.0-float64(violations)/float64(totalChecked))
	return rate, violations, totalChecked
}

// scoreImportGraph computes a 0.0-1.0 score from import graph signals.
func scoreImportGraph(graph *ImportGraph, profile *domain.ScoringProfile) float64 {
	if graph == nil || len(graph.Packages) <= 1 {
		return 1.0
	}

	cycleW := profile.CyclePenaltyWeight
	if cycleW <= 0 {
		cycleW = 0.40
	}
	distW := (1.0 - cycleW) * 0.60
	coupW := (1.0 - cycleW) * 0.40

	// 1. Cycle penalty: any cycles → cycleScore = 0.
	cycles := graph.DetectCycles()
	cycleScore := 1.0
	if len(cycles) > 0 {
		cycleScore = 0.0
	}

	// 2. Distance from main sequence.
	maxDist := profile.MaxDistanceFromMain
	if maxDist <= 0 {
		maxDist = 0.40
	}
	avgDist := graph.AverageDistance()
	distScore := 1.0
	if avgDist > maxDist {
		distScore = max(0, 1.0-(avgDist-maxDist)/(maxDist*2))
	}

	// 3. Coupling outliers.
	multiplier := profile.CouplingOutlierMultiplier
	if multiplier <= 0 {
		multiplier = 2.0
	}
	outliers := graph.CouplingOutliers(multiplier)
	couplingScore := 1.0
	if len(graph.Packages) > 0 {
		couplingScore = 1.0 - float64(len(outliers))/float64(len(graph.Packages))
	}

	return cycleScore*cycleW + distScore*distW + couplingScore*coupW
}

// formatDependencyDetail produces human-readable detail for the dependency_direction sub-metric.
func formatDependencyDetail(violations, totalChecked int, graph *ImportGraph, graphScore float64) string {
	parts := []string{}
	if totalChecked > 0 {
		rate := max(0, 1.0-float64(violations)/float64(totalChecked))
		parts = append(parts, fmt.Sprintf("%d violation(s) in %d layered files (%.0f%% clean)",
			violations, totalChecked, rate*100))
	}
	if graph != nil && len(graph.Packages) > 1 {
		cycles := graph.DetectCycles()
		parts = append(parts, fmt.Sprintf("graph: %d pkgs, %d cycles, score=%.0f%%",
			len(graph.Packages), len(cycles), graphScore*100))
	}
	if len(parts) == 0 {
		return "no layered files or import graph to evaluate"
	}
	return strings.Join(parts, "; ")
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

// countExportedFunctions counts exported functions in non-generated files.
func countExportedFunctions(analyzed map[string]*domain.AnalyzedFile) int {
	count := 0
	for _, af := range analyzed {
		if af.IsGenerated {
			continue
		}
		for _, fn := range af.Functions {
			if fn.Exported {
				count++
			}
		}
	}
	return count
}

func collectDiscoverabilityIssues(profile *domain.ScoringProfile, modules []domain.DetectedModule, scan *domain.ScanResult, analyzed map[string]*domain.AnalyzedFile) []domain.Issue {
	var issues []domain.Issue

	// 1. naming_uniqueness: flag exported single-word functions (WCS < threshold).
	//    Skip: generated files, test files, Go interface methods, methods with receiver + single word.
	minWCS := profile.MinNamingWordScore
	if minWCS <= 0 {
		minWCS = 0.7
	}
	for _, af := range analyzed {
		if af.IsGenerated || strings.HasSuffix(af.Path, "_test.go") {
			continue
		}
		for _, fn := range af.Functions {
			if !fn.Exported {
				continue
			}
			if fn.Receiver != "" && WordCount(fn.Name) == 1 {
				continue
			}
			if WordCountScore(fn.Name) < minWCS {
				wc := WordCount(fn.Name)
				msg := fmt.Sprintf("exported function %q has a single-word name; consider a verb+noun pattern", fn.Name)
				if wc > 1 {
					msg = fmt.Sprintf("exported function %q has %d words; consider a shorter verb+noun pattern", fn.Name, wc)
				}
				issues = append(issues, domain.Issue{
					Severity:  domain.SeverityInfo,
					Category:  "discoverability",
					SubMetric: "naming_uniqueness",
					File:      af.Path,
					Line:      fn.LineStart,
					Message:   msg,
				})
			}
		}
	}

	// 2. file_naming_conventions: flag files violating dominant pattern.
	//    Only when dominant pattern has ≥threshold consistency to avoid FP on 50/50 splits.
	//    Skips generated files via classifyFileNaming.
	consistencyThreshold := profile.NamingConsistencyThreshold
	if consistencyThreshold <= 0 {
		consistencyThreshold = 0.60
	}
	if scan != nil && len(scan.GoFiles) > 0 {
		c := classifyFileNaming(profile, scan.GoFiles, analyzed)
		if c.total > 0 && c.consistency >= consistencyThreshold {
			// Severity based on how inconsistent the project is.
			fileSev := domain.SeverityInfo
			if c.consistency < 0.40 {
				fileSev = domain.SeverityWarning
			}
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
				isSuffixed := hasKnownSuffix(name, profile.ExpectedFileSuffixes)
				if c.dominantIsSuffixed && !isSuffixed {
					issues = append(issues, domain.Issue{
						Severity:  fileSev,
						Category:  "discoverability",
						SubMetric: "file_naming_conventions",
						File:      f,
						Message:   fmt.Sprintf("file %q uses bare naming but project uses suffixed pattern", base),
					})
				} else if !c.dominantIsSuffixed && isSuffixed {
					issues = append(issues, domain.Issue{
						Severity:  fileSev,
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
		peerThreshold := len(modules) / 2
		totalLayers := len(layerCount)
		for _, m := range modules {
			has := map[string]bool{}
			for _, l := range m.Layers {
				has[l] = true
			}
			missingCount := 0
			for layer, count := range layerCount {
				if count > peerThreshold && !has[layer] {
					missingCount++
				}
			}
			// Severity based on fraction of peer layers missing.
			structSev := domain.SeverityInfo
			if totalLayers > 0 {
				missingRatio := float64(missingCount) / float64(totalLayers)
				if missingRatio > 0.75 {
					structSev = domain.SeverityWarning
				}
			}
			for layer, count := range layerCount {
				if count > peerThreshold && !has[layer] {
					issues = append(issues, domain.Issue{
						Severity:  structSev,
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
			layer := fileLayer(f, profile)
			for _, imp := range af.Imports {
				if violatesDependencyDirection(layer, imp, profile) {
					impLayer := importLayer(imp, profile)
					pat := layer + "→" + impLayer
					issues = append(issues, domain.Issue{
						Severity:  domain.SeverityError,
						Category:  "discoverability",
						SubMetric: "dependency_direction",
						File:      f,
						Message:   fmt.Sprintf("%s layer imports %s (dependency direction violation)", layer, imp),
						Pattern:   pat,
					})
				}
			}
		}
	}

	// 5. Import graph: cycles and coupling outliers.
	if scan != nil && scan.ModulePath != "" {
		graph := BuildImportGraph(scan.ModulePath, analyzed)
		if graph != nil {
			for _, cycle := range graph.DetectCycles() {
				issues = append(issues, domain.Issue{
					Severity:  domain.SeverityError,
					Category:  "discoverability",
					SubMetric: "dependency_direction",
					Message:   fmt.Sprintf("import cycle: %s", strings.Join(cycle, " → ")),
					Pattern:   "import-cycle",
				})
			}
			multiplier := profile.CouplingOutlierMultiplier
			if multiplier <= 0 {
				multiplier = 2.0
			}
			for _, outlier := range graph.CouplingOutliers(multiplier) {
				issues = append(issues, domain.Issue{
					Severity:  domain.SeverityWarning,
					Category:  "discoverability",
					SubMetric: "dependency_direction",
					Message:   fmt.Sprintf("package %q imports %d internal packages (median is %.0f)", outlier.Package, outlier.Ce, outlier.MedianCe),
					Pattern:   "coupling-outlier",
				})
			}
		}
	}

	// 6. Symbol collision: flag exported names appearing in 2+ packages.
	type collisionInfo struct {
		packages map[string]bool
	}
	collisionMap := make(map[string]*collisionInfo)
	for _, af := range analyzed {
		if af.IsGenerated || strings.HasSuffix(af.Path, "_test.go") {
			continue
		}
		for _, fn := range af.Functions {
			if !fn.Exported || fn.Receiver != "" {
				continue
			}
			ci, ok := collisionMap[fn.Name]
			if !ok {
				ci = &collisionInfo{packages: make(map[string]bool)}
				collisionMap[fn.Name] = ci
			}
			ci.packages[af.Package] = true
		}
	}
	for name, ci := range collisionMap {
		if len(ci.packages) >= 2 {
			issues = append(issues, domain.Issue{
				Severity:  domain.SeverityInfo,
				Category:  "discoverability",
				SubMetric: "naming_uniqueness",
				Message:   fmt.Sprintf("exported function %q appears in %d packages", name, len(ci.packages)),
			})
		}
	}

	// 6. Package name quality: flag vague package names.
	vaguePackages := map[string]bool{
		"util": true, "utils": true, "common": true, "helpers": true,
		"misc": true, "base": true, "lib": true, "shared": true,
		"tools": true, "types": true,
	}
	seenPackages := make(map[string]bool)
	for _, af := range analyzed {
		if af.IsGenerated || af.Package == "" || seenPackages[af.Package] {
			continue
		}
		seenPackages[af.Package] = true
		if vaguePackages[af.Package] {
			issues = append(issues, domain.Issue{
				Severity:  domain.SeverityInfo,
				Category:  "discoverability",
				SubMetric: "naming_uniqueness",
				File:      af.Path,
				Message:   fmt.Sprintf("package %q is a vague name; consider a more descriptive name", af.Package),
			})
		}
	}

	// 7. Param name quality: flag exported functions where all params are single-letter
	//    and param count >= 2.
	for _, af := range analyzed {
		if af.IsGenerated || strings.HasSuffix(af.Path, "_test.go") {
			continue
		}
		for _, fn := range af.Functions {
			if !fn.Exported || fn.Receiver != "" || len(fn.Params) < 2 {
				continue
			}
			allSingleLetter := true
			for _, p := range fn.Params {
				if len(p.Name) != 1 {
					allSingleLetter = false
					break
				}
			}
			if allSingleLetter {
				issues = append(issues, domain.Issue{
					Severity:  domain.SeverityInfo,
					Category:  "discoverability",
					SubMetric: "naming_uniqueness",
					File:      af.Path,
					Line:      fn.LineStart,
					Message:   fmt.Sprintf("exported function %q has %d single-letter parameters", fn.Name, len(fn.Params)),
				})
			}
		}
	}

	return issues
}

// platformBuildTags are Go platform-specific build constraint suffixes that should
// not be treated as naming convention suffixes.
var platformBuildTags = map[string]bool{
	"_darwin": true, "_linux": true, "_windows": true,
	"_amd64": true, "_arm64": true, "_386": true,
	"_unix": true, "_freebsd": true, "_netbsd": true,
	"_openbsd": true, "_dragonfly": true, "_solaris": true,
	"_plan9": true, "_wasm": true, "_js": true,
	"_wasip1": true, "_ios": true, "_android": true,
	"_mips": true, "_mips64": true, "_ppc64": true,
	"_riscv64": true, "_s390x": true, "_loong64": true,
}

// hasKnownSuffix checks if a filename (without .go extension) has a recognized
// role suffix from the expected suffixes list, after stripping platform build tags.
func hasKnownSuffix(name string, expectedSuffixes []string) bool {
	// Strip platform build tags first.
	for tag := range platformBuildTags {
		if strings.HasSuffix(name, tag) {
			name = strings.TrimSuffix(name, tag)
			break
		}
	}
	idx := strings.LastIndex(name, "_")
	if idx < 0 {
		return false
	}
	suffix := name[idx:]
	for _, expected := range expectedSuffixes {
		if suffix == expected {
			return true
		}
	}
	return false
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
		if hasKnownSuffix(name, profile.ExpectedFileSuffixes) {
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
