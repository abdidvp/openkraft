package scoring

import (
	"fmt"
	"math"
	"path/filepath"
	"sort"
	"strings"

	"github.com/abdidvp/openkraft/internal/domain"
)

// ImportGraph represents the internal import relationships between packages.
type ImportGraph struct {
	Packages map[string]*PackageNode
}

// PackageNode represents a single package in the import graph.
type PackageNode struct {
	ImportPath      string
	Files           []string
	ImportsInternal []string // outgoing edges (Ce = efferent coupling)
	ImportedBy      []string // incoming edges (Ca = afferent coupling)
	Interfaces      int
	Structs         int
}

// CouplingOutlier represents a package with abnormally high efferent coupling.
type CouplingOutlier struct {
	Package  string
	Ce       int
	MedianCe float64
}

// BuildImportGraph constructs an import graph from analyzed files.
// Only internal imports (matching modulePath prefix) are included.
// Test files and generated files are excluded.
func BuildImportGraph(modulePath string, analyzed map[string]*domain.AnalyzedFile) *ImportGraph {
	if modulePath == "" {
		return nil
	}

	g := &ImportGraph{Packages: make(map[string]*PackageNode)}

	// Group files by package directory.
	for _, af := range analyzed {
		if af.IsGenerated || strings.HasSuffix(af.Path, "_test.go") {
			continue
		}

		dir := filepath.Dir(af.Path)
		if dir == "." {
			dir = ""
		}
		var pkgPath string
		if dir == "" {
			pkgPath = modulePath
		} else {
			pkgPath = modulePath + "/" + filepath.ToSlash(dir)
		}

		node, ok := g.Packages[pkgPath]
		if !ok {
			node = &PackageNode{ImportPath: pkgPath}
			g.Packages[pkgPath] = node
		}
		node.Files = append(node.Files, af.Path)
		node.Interfaces += len(af.Interfaces)
		node.Structs += len(af.Structs)

		// Collect internal imports.
		for _, imp := range af.Imports {
			if strings.HasPrefix(imp, modulePath+"/") || imp == modulePath {
				if imp != pkgPath && !containsString(node.ImportsInternal, imp) {
					node.ImportsInternal = append(node.ImportsInternal, imp)
				}
			}
		}
	}

	// Build reverse edges (ImportedBy).
	for pkgPath, node := range g.Packages {
		for _, imp := range node.ImportsInternal {
			target, ok := g.Packages[imp]
			if !ok {
				// Target package exists in imports but wasn't in analyzed files.
				// Create a stub node so metrics still work.
				target = &PackageNode{ImportPath: imp}
				g.Packages[imp] = target
			}
			if !containsString(target.ImportedBy, pkgPath) {
				target.ImportedBy = append(target.ImportedBy, pkgPath)
			}
		}
	}

	return g
}

// DetectCycles finds all import cycles using DFS with grey/black coloring.
// Each cycle is normalized (rotated to smallest element) and deduplicated.
func (g *ImportGraph) DetectCycles() [][]string {
	if g == nil || len(g.Packages) == 0 {
		return nil
	}

	const (
		white = 0
		grey  = 1
		black = 2
	)

	color := make(map[string]int)
	parent := make(map[string]string)
	var cycles [][]string
	seen := make(map[string]bool) // normalized cycle key → already recorded

	// Sort keys for deterministic output.
	keys := make([]string, 0, len(g.Packages))
	for k := range g.Packages {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	var dfs func(u string)
	dfs = func(u string) {
		color[u] = grey
		node := g.Packages[u]
		if node == nil {
			color[u] = black
			return
		}

		neighbors := make([]string, len(node.ImportsInternal))
		copy(neighbors, node.ImportsInternal)
		sort.Strings(neighbors)

		for _, v := range neighbors {
			if color[v] == grey {
				// Back edge found — extract cycle.
				cycle := []string{v}
				cur := u
				for cur != v {
					cycle = append(cycle, cur)
					cur = parent[cur]
				}
				// Reverse to get forward order.
				for i, j := 0, len(cycle)-1; i < j; i, j = i+1, j-1 {
					cycle[i], cycle[j] = cycle[j], cycle[i]
				}

				// Normalize: rotate so smallest element is first.
				normalized := normalizeCycle(cycle)
				key := strings.Join(normalized, "→")
				if !seen[key] {
					seen[key] = true
					cycles = append(cycles, normalized)
				}
			} else if color[v] == white {
				parent[v] = u
				dfs(v)
			}
		}
		color[u] = black
	}

	for _, k := range keys {
		if color[k] == white {
			dfs(k)
		}
	}

	return cycles
}

// normalizeCycle rotates a cycle so the lexicographically smallest element is first.
func normalizeCycle(cycle []string) []string {
	if len(cycle) == 0 {
		return cycle
	}
	minIdx := 0
	for i, s := range cycle {
		if s < cycle[minIdx] {
			minIdx = i
		}
	}
	result := make([]string, len(cycle))
	for i := range cycle {
		result[i] = cycle[(minIdx+i)%len(cycle)]
	}
	return result
}

// Instability returns Robert Martin's Instability metric for a package.
// I = Ce / (Ca + Ce) where Ce = efferent coupling, Ca = afferent coupling.
func (g *ImportGraph) Instability(pkg string) float64 {
	if g == nil {
		return 0.0
	}
	node, ok := g.Packages[pkg]
	if !ok {
		return 0.0
	}
	ce := float64(len(node.ImportsInternal))
	ca := float64(len(node.ImportedBy))
	if ca+ce == 0 {
		return 0.0
	}
	return ce / (ca + ce)
}

// Abstractness returns Robert Martin's Abstractness metric for a package.
// A = interfaces / (interfaces + structs).
func (g *ImportGraph) Abstractness(pkg string) float64 {
	if g == nil {
		return 0.0
	}
	node, ok := g.Packages[pkg]
	if !ok {
		return 0.0
	}
	total := float64(node.Interfaces + node.Structs)
	if total == 0 {
		return 0.0
	}
	return float64(node.Interfaces) / total
}

// DistanceFromMainSequence returns |A + I - 1| for a package.
// Range [0, 1] where 0 = on the main sequence (ideal).
func (g *ImportGraph) DistanceFromMainSequence(pkg string) float64 {
	return math.Abs(g.Abstractness(pkg) + g.Instability(pkg) - 1)
}

// AverageDistance returns the average distance from the main sequence
// across all packages with at least 1 type definition.
func (g *ImportGraph) AverageDistance() float64 {
	if g == nil || len(g.Packages) == 0 {
		return 0.0
	}
	var total float64
	count := 0
	for pkg, node := range g.Packages {
		if node.Interfaces+node.Structs == 0 {
			continue
		}
		total += g.DistanceFromMainSequence(pkg)
		count++
	}
	if count == 0 {
		return 0.0
	}
	return total / float64(count)
}

// CouplingOutliers returns packages whose efferent coupling exceeds
// multiplier * median(Ce) across all packages.
func (g *ImportGraph) CouplingOutliers(multiplier float64) []CouplingOutlier {
	if g == nil || len(g.Packages) == 0 {
		return nil
	}

	ces := make([]int, 0, len(g.Packages))
	for _, node := range g.Packages {
		ces = append(ces, len(node.ImportsInternal))
	}
	sort.Ints(ces)

	median := medianInt(ces)
	if median < 1.0 {
		// No meaningful baseline — most packages import nothing or very little.
		// Approach A: no confident signal = no penalty.
		return nil
	}

	threshold := multiplier * median

	var outliers []CouplingOutlier
	// Sort keys for deterministic output.
	keys := make([]string, 0, len(g.Packages))
	for k := range g.Packages {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	for _, pkg := range keys {
		node := g.Packages[pkg]
		ce := len(node.ImportsInternal)
		if float64(ce) > threshold {
			outliers = append(outliers, CouplingOutlier{
				Package:  pkg,
				Ce:       ce,
				MedianCe: median,
			})
		}
	}
	return outliers
}

// medianInt returns the median of a sorted slice of ints as float64.
func medianInt(sorted []int) float64 {
	n := len(sorted)
	if n == 0 {
		return 0
	}
	if n%2 == 0 {
		return float64(sorted[n/2-1]+sorted[n/2]) / 2.0
	}
	return float64(sorted[n/2])
}

// EdgeCount returns the total number of directed edges in the import graph.
func (g *ImportGraph) EdgeCount() int {
	if g == nil {
		return 0
	}
	total := 0
	for _, node := range g.Packages {
		total += len(node.ImportsInternal)
	}
	return total
}

func containsString(slice []string, s string) bool {
	for _, v := range slice {
		if v == s {
			return true
		}
	}
	return false
}

// ArchRole represents the detected architectural role of a package.
type ArchRole string

const (
	RoleCore         ArchRole = "core"
	RolePorts        ArchRole = "ports"
	RoleAdapter      ArchRole = "adapter"
	RoleOrchestrator ArchRole = "orchestrator"
	RoleEntryPoint   ArchRole = "entry point"
	RoleUnclassified ArchRole = "—"
)

// PackageViolation represents a concrete dependency rule violation.
type PackageViolation struct {
	Message string
}

// AnnotatedPackage combines a package node with its detected role and violations.
type AnnotatedPackage struct {
	Node       *PackageNode
	Role       ArchRole
	Violations []PackageViolation
}

// ClassifyPackages detects the architectural role and dependency violations
// for every package in the graph. Only penalizes certainties (Approach A).
func (g *ImportGraph) ClassifyPackages(modulePath string, profile *domain.ScoringProfile) map[string]*AnnotatedPackage {
	if g == nil || len(g.Packages) == 0 {
		return nil
	}

	cycleSet := buildCycleSet(g.DetectCycles())
	result := make(map[string]*AnnotatedPackage, len(g.Packages))

	for pkg, node := range g.Packages {
		stripped := strings.TrimPrefix(pkg, modulePath+"/")
		if stripped == pkg {
			// Root module package.
			stripped = ""
		}

		role := classifyRole(stripped, pkg, modulePath, profile)

		var violations []PackageViolation

		// Check dependency direction violations.
		// Use stripped (module-relative) paths to avoid false matches
		// from module path segments (e.g. "github.com/example/app" matching "app" alias).
		for _, imp := range node.ImportsInternal {
			impStripped := strings.TrimPrefix(imp, modulePath+"/")
			if impStripped == imp {
				impStripped = ""
			}
			impLayer := strippedLayer(impStripped, profile)

			switch role {
			case RoleCore, RolePorts:
				switch impLayer {
				case "adapters":
					violations = append(violations, PackageViolation{Message: "imports adapter"})
				case "application":
					violations = append(violations, PackageViolation{Message: "imports application"})
				}
			case RoleOrchestrator:
				if impLayer == "adapters" {
					violations = append(violations, PackageViolation{Message: "imports adapter"})
				}
			case RoleAdapter:
				if impLayer == "adapters" && impStripped != stripped {
					srcTree := adapterSubtree(stripped)
					tgtTree := adapterSubtree(impStripped)
					if srcTree != "" && tgtTree != "" && srcTree != tgtTree {
						srcDir := adapterDirection(stripped)
						tgtDir := adapterDirection(impStripped)

						// inbound → outbound: normal hexagonal wiring — skip
						if srcDir == "inbound" && tgtDir == "outbound" {
							continue
						}
						// Configured composition roots are exempt
						if isCompositionRoot(stripped, profile) {
							continue
						}

						short := impStripped
						if idx := strings.LastIndex(short, "/"); idx >= 0 {
							short = short[idx+1:]
						}
						violations = append(violations, PackageViolation{
							Message: fmt.Sprintf("imports %s", short),
						})
					}
				}
			}
		}

		// Cycle violations.
		if cycleSet[pkg] {
			violations = append(violations, PackageViolation{Message: "in import cycle"})
		}

		result[pkg] = &AnnotatedPackage{
			Node:       node,
			Role:       role,
			Violations: violations,
		}
	}

	return result
}

// classifyRole determines the architectural role of a package.
// Priority order: ports > entry point > adapter > orchestrator > core > unclassified.
func classifyRole(stripped, fullPkg, modulePath string, profile *domain.ScoringProfile) ArchRole {
	normalized := "/" + strings.ReplaceAll(stripped, "\\", "/") + "/"

	// 1. Ports — check before domain since domain/ports matches both.
	if strings.Contains(normalized, "/ports/") || strings.HasSuffix(strings.TrimRight(normalized, "/"), "/ports") {
		return RolePorts
	}

	// 2. Entry point — cmd/ or root main package.
	if strings.Contains(normalized, "/cmd/") || fullPkg == modulePath {
		return RoleEntryPoint
	}

	// 3-5. Use strippedLayer on the module-relative path to avoid false matches
	// from module path segments (e.g. "github.com/example/app" matching "app" alias).
	layer := strippedLayer(stripped, profile)
	switch layer {
	case "adapters":
		return RoleAdapter
	case "application":
		return RoleOrchestrator
	case "domain":
		return RoleCore
	}

	return RoleUnclassified
}

// strippedLayer returns the architectural layer of a module-relative path.
// Unlike importLayer, this operates on the stripped (module-relative) path
// to avoid false matches from module path segments.
func strippedLayer(stripped string, profile *domain.ScoringProfile) string {
	normalized := "/" + strings.ReplaceAll(stripped, "\\", "/") + "/"
	layers := buildLayerMap(profile)
	for name, canonical := range layers {
		if strings.Contains(normalized, "/"+name+"/") {
			return canonical
		}
	}
	// Also check suffix (e.g. "internal/domain" ends with "/domain").
	trimmed := strings.TrimRight(normalized, "/")
	for name, canonical := range layers {
		if strings.HasSuffix(trimmed, "/"+name) {
			return canonical
		}
	}
	return ""
}

// buildCycleSet builds a set of all packages that participate in any cycle.
func buildCycleSet(cycles [][]string) map[string]bool {
	set := make(map[string]bool)
	for _, cycle := range cycles {
		for _, pkg := range cycle {
			set[pkg] = true
		}
	}
	return set
}

// TotalViolations counts the total number of violations across all annotated packages.
func TotalViolations(annotated map[string]*AnnotatedPackage) int {
	total := 0
	for _, ap := range annotated {
		total += len(ap.Violations)
	}
	return total
}

// adapterDirection extracts the direction segment ("inbound" or "outbound")
// from a module-relative adapter path. Returns "" if no direction is found
// (flat adapter structure without inbound/outbound subdivision).
func adapterDirection(strippedPath string) string {
	parts := strings.Split(strings.ReplaceAll(strippedPath, "\\", "/"), "/")
	for i, p := range parts {
		if p == "adapters" || p == "adapter" || p == "infra" || p == "infrastructure" {
			if i+1 < len(parts) {
				dir := parts[i+1]
				if dir == "inbound" || dir == "outbound" {
					return dir
				}
			}
			return ""
		}
	}
	return ""
}

// isCompositionRoot checks whether the stripped path matches or is a child
// of any configured CompositionRoots entry in the profile.
func isCompositionRoot(stripped string, profile *domain.ScoringProfile) bool {
	normalized := strings.ReplaceAll(stripped, "\\", "/")
	for _, root := range profile.CompositionRoots {
		root = strings.ReplaceAll(root, "\\", "/")
		if normalized == root || strings.HasPrefix(normalized, root+"/") {
			return true
		}
	}
	return false
}

// adapterSubtree extracts the adapter subtree prefix.
// e.g. "adapters/outbound/db/schema" → "adapters/outbound/db"
// e.g. "adapters/inbound/http" → "adapters/inbound/http"
// Returns "" if the path doesn't have enough segments after adapters/{inbound,outbound}.
func adapterSubtree(strippedPath string) string {
	parts := strings.Split(strings.ReplaceAll(strippedPath, "\\", "/"), "/")
	// Find "adapters" segment, then expect inbound/outbound, then component name.
	for i, p := range parts {
		if p == "adapters" || p == "adapter" || p == "infra" || p == "infrastructure" {
			// Need at least: adapters / {direction} / {component}
			if i+2 < len(parts) {
				return strings.Join(parts[:i+3], "/")
			}
			// adapters / {component} (no inbound/outbound subdivision)
			if i+1 < len(parts) {
				return strings.Join(parts[:i+2], "/")
			}
			return strippedPath
		}
	}
	return strippedPath
}
