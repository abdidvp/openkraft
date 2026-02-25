package scoring

import (
	"fmt"
	"strings"

	"github.com/openkraft/openkraft/internal/domain"
)

// ScoreStructure evaluates module boundaries and structural consistency.
// Weight: 0.15 (15% of overall score).
func ScoreStructure(profile *domain.ScoringProfile, modules []domain.DetectedModule, scan *domain.ScanResult, analyzed map[string]*domain.AnalyzedFile) domain.CategoryScore {
	cat := domain.CategoryScore{
		Name:   "structure",
		Weight: 0.15,
	}

	sm1 := scoreExpectedLayers(profile, modules, scan)
	sm2 := scoreExpectedFiles(profile, modules)
	sm3 := scoreInterfaceContracts(modules, analyzed)
	sm4 := scoreModuleCompleteness(modules, analyzed)

	cat.SubMetrics = []domain.SubMetric{sm1, sm2, sm3, sm4}

	total := 0
	for _, sm := range cat.SubMetrics {
		total += sm.Score
	}
	cat.Score = total

	cat.Issues = collectStructureIssues(modules, analyzed)
	return cat
}

// scoreExpectedLayers (25 pts): presence of directories per project profile.
func scoreExpectedLayers(profile *domain.ScoringProfile, modules []domain.DetectedModule, scan *domain.ScanResult) domain.SubMetric {
	sm := domain.SubMetric{Name: "expected_layers", Points: 25}

	if scan == nil {
		sm.Detail = "no scan data"
		return sm
	}

	// Check expected top-level dirs from profile.
	dirFound := make(map[string]bool)
	for _, f := range scan.AllFiles {
		for _, dir := range profile.ExpectedDirs {
			if strings.HasPrefix(f, dir+"/") {
				dirFound[dir] = true
			}
		}
	}

	// Check expected layers.
	layerFound := make(map[string]bool)
	if scan.Layout == domain.LayoutCrossCutting {
		for _, f := range scan.AllFiles {
			if !strings.HasPrefix(f, "internal/") {
				continue
			}
			parts := strings.SplitN(strings.TrimPrefix(f, "internal/"), "/", 2)
			if len(parts) > 0 {
				layerFound[normalizeLayerNameWithProfile(parts[0], profile)] = true
			}
		}
	} else {
		for _, m := range modules {
			for _, l := range m.Layers {
				layerFound[normalizeLayerNameWithProfile(l, profile)] = true
			}
		}
	}

	found := 0
	for _, dir := range profile.ExpectedDirs {
		if dirFound[dir] {
			found++
		}
	}
	for _, l := range profile.ExpectedLayers {
		if layerFound[l] {
			found++
		}
	}
	total := len(profile.ExpectedDirs) + len(profile.ExpectedLayers)
	if total == 0 {
		sm.Score = sm.Points
		sm.Detail = "no expected layers/dirs configured"
		return sm
	}

	sm.Score = int(float64(found) / float64(total) * float64(sm.Points))
	if sm.Score > sm.Points {
		sm.Score = sm.Points
	}
	sm.Detail = fmt.Sprintf("%d/%d expected directories/layers present", found, total)
	return sm
}

// normalizeLayerNameWithProfile maps directory name variants using profile aliases.
func normalizeLayerNameWithProfile(name string, profile *domain.ScoringProfile) string {
	if canonical, ok := profile.LayerAliases[name]; ok {
		return canonical
	}
	return name
}

// scoreExpectedFiles (25 pts): per module, ratio of files matching profile's expected suffixes.
func scoreExpectedFiles(profile *domain.ScoringProfile, modules []domain.DetectedModule) domain.SubMetric {
	sm := domain.SubMetric{Name: "expected_files", Points: 25}

	if len(modules) == 0 {
		sm.Detail = "no modules detected"
		return sm
	}

	suffixes := append(profile.ExpectedFileSuffixes, "_test")

	totalRatio := 0.0
	for _, m := range modules {
		if len(m.Files) == 0 {
			continue
		}
		matched := 0
		for _, f := range m.Files {
			name := strings.TrimSuffix(f, ".go")
			for _, suffix := range suffixes {
				if strings.HasSuffix(name, suffix) {
					matched++
					break
				}
			}
		}
		totalRatio += float64(matched) / float64(len(m.Files))
	}

	avgRatio := totalRatio / float64(len(modules))
	sm.Score = int(avgRatio * float64(sm.Points))
	if sm.Score > sm.Points {
		sm.Score = sm.Points
	}
	sm.Detail = fmt.Sprintf("%.0f%% average conventional file coverage across %d modules", avgRatio*100, len(modules))
	return sm
}

// scoreInterfaceContracts (25 pts): checks whether port interfaces defined in
// domain/application files have concrete implementations (receiver methods match).
func scoreInterfaceContracts(_ []domain.DetectedModule, analyzed map[string]*domain.AnalyzedFile) domain.SubMetric {
	sm := domain.SubMetric{Name: "interface_contracts", Points: 25}

	// Collect port interfaces from domain/application files.
	var ports []domain.InterfaceDef
	for _, af := range analyzed {
		if !isDomainOrAppFile(af.Path) {
			continue
		}
		ports = append(ports, af.InterfaceDefs...)
	}

	if len(ports) == 0 {
		sm.Detail = "no port interfaces found"
		return sm
	}

	// Collect methods-by-receiver from all concrete types.
	receivers := map[string]map[string]bool{} // receiver → {method names}
	for _, af := range analyzed {
		for _, fn := range af.Functions {
			if fn.Receiver == "" {
				continue
			}
			recv := strings.TrimPrefix(fn.Receiver, "*")
			if receivers[recv] == nil {
				receivers[recv] = map[string]bool{}
			}
			receivers[recv][fn.Name] = true
		}
	}

	// Check each port: is there a concrete type implementing all its methods?
	satisfied := 0
	for _, iface := range ports {
		if len(iface.Methods) == 0 {
			satisfied++
			continue
		}
		for _, methods := range receivers {
			if implementsAll(iface.Methods, methods) {
				satisfied++
				break
			}
		}
	}

	ratio := float64(satisfied) / float64(len(ports))
	sm.Score = int(ratio * float64(sm.Points))
	if sm.Score > sm.Points {
		sm.Score = sm.Points
	}
	sm.Detail = fmt.Sprintf("%d/%d port interfaces have concrete implementations", satisfied, len(ports))
	return sm
}

func isDomainOrAppFile(path string) bool {
	return strings.Contains(path, "/domain/") || strings.Contains(path, "/application/")
}

func implementsAll(required []string, available map[string]bool) bool {
	for _, m := range required {
		if !available[m] {
			return false
		}
	}
	return true
}

// scoreModuleCompleteness (25 pts): compares file counts among modules sharing
// at least one layer. Modules in different layers are architecturally distinct
// by design and should not be compared.
func scoreModuleCompleteness(modules []domain.DetectedModule, _ map[string]*domain.AnalyzedFile) domain.SubMetric {
	sm := domain.SubMetric{Name: "module_completeness", Points: 25}

	if len(modules) <= 1 {
		if len(modules) == 1 {
			sm.Score = sm.Points
			sm.Detail = "single module"
		} else {
			sm.Detail = "no modules detected"
		}
		return sm
	}

	// Group modules by layer.
	layerModules := map[string][]int{}
	for i, m := range modules {
		for _, l := range m.Layers {
			layerModules[l] = append(layerModules[l], i)
		}
	}

	// For each layer group with 2+ modules, compute file-count similarity.
	var totalRatio float64
	comparisons := 0
	for _, indices := range layerModules {
		if len(indices) < 2 {
			continue
		}
		maxFiles := 0
		for _, idx := range indices {
			if len(modules[idx].Files) > maxFiles {
				maxFiles = len(modules[idx].Files)
			}
		}
		if maxFiles == 0 {
			continue
		}
		for _, idx := range indices {
			ratio := float64(len(modules[idx].Files)) / float64(maxFiles)
			totalRatio += ratio
			comparisons++
		}
	}

	if comparisons == 0 {
		sm.Score = sm.Points
		sm.Detail = "no comparable module pairs"
		return sm
	}

	avg := totalRatio / float64(comparisons)
	sm.Score = int(avg * float64(sm.Points))
	if sm.Score > sm.Points {
		sm.Score = sm.Points
	}
	sm.Detail = fmt.Sprintf("%.0f%% average completeness across %d comparable modules", avg*100, comparisons)
	return sm
}

func collectStructureIssues(modules []domain.DetectedModule, analyzed map[string]*domain.AnalyzedFile) []domain.Issue {
	var issues []domain.Issue

	if len(modules) == 0 {
		issues = append(issues, domain.Issue{
			Severity: domain.SeverityWarning,
			Category: "structure",
			Message:  "no modules detected; cannot evaluate structure",
		})
		return issues
	}

	for _, m := range modules {
		hasInterface := false
		hasDomainOrApp := false
		for _, f := range m.Files {
			norm := strings.ReplaceAll(f, "\\", "/")
			if strings.Contains(norm, "/domain/") || strings.Contains(norm, "/application/") {
				hasDomainOrApp = true
				af, ok := analyzed[f]
				if ok && len(af.Interfaces) > 0 {
					hasInterface = true
				}
			}
		}
		if hasDomainOrApp && !hasInterface {
			issues = append(issues, domain.Issue{
				Severity:  domain.SeverityWarning,
				Category:  "structure",
				SubMetric: "interface_contracts",
				Message:   fmt.Sprintf("module %q has domain/application layer but no port interfaces", m.Name),
			})
		}
	}

	return issues
}
