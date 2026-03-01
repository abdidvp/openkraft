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
	ImportsStdlibIO bool // at least one stdlib I/O import
	ImportsExtIO    bool // at least one external I/O import
	HasMain         bool // contains func main()
	HasIOParams     bool // has functions with I/O parameter types
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

		// Collect internal imports and classify non-internal imports.
		for _, imp := range af.Imports {
			if strings.HasPrefix(imp, modulePath+"/") || imp == modulePath {
				if imp != pkgPath && !containsString(node.ImportsInternal, imp) {
					node.ImportsInternal = append(node.ImportsInternal, imp)
				}
				continue
			}
			if !node.ImportsStdlibIO && isStdlibIO(imp) {
				node.ImportsStdlibIO = true
			}
			if !node.ImportsExtIO && isExternalIO(imp) {
				node.ImportsExtIO = true
			}
		}

		// Check for main func and I/O params.
		for _, fn := range af.Functions {
			if fn.Name == "main" && fn.Receiver == "" {
				node.HasMain = true
			}
			if !node.HasIOParams {
				for _, param := range fn.Params {
					if isIOParamType(param.Type) {
						node.HasIOParams = true
						break
					}
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

func isStdlibIO(imp string) bool {
	if stdlibIO[imp] {
		return true
	}
	for pkg := range stdlibIO {
		if strings.HasPrefix(imp, pkg+"/") {
			return true
		}
	}
	return false
}

func isExternalIO(imp string) bool {
	for _, prefix := range externalIOPrefixes {
		if strings.HasPrefix(imp, prefix) {
			return true
		}
	}
	return false
}

func isIOParamType(typeName string) bool {
	for _, t := range ioParamTypes {
		if strings.Contains(typeName, t) {
			return true
		}
	}
	return false
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

// RoleSignal represents a classification signal with confidence.
type RoleSignal struct {
	Role       ArchRole
	Confidence float64
}

// roleHints maps well-known directory names to (role, confidence).
var roleHints = map[string]RoleSignal{
	// Core
	"domain": {RoleCore, 0.85}, "model": {RoleCore, 0.85}, "models": {RoleCore, 0.85},
	"entity": {RoleCore, 0.85}, "entities": {RoleCore, 0.85}, "types": {RoleCore, 0.80},
	"schema": {RoleCore, 0.75}, "core": {RoleCore, 0.80},
	// Orchestrator
	"application": {RoleOrchestrator, 0.80}, "app": {RoleOrchestrator, 0.70},
	"service": {RoleOrchestrator, 0.75}, "services": {RoleOrchestrator, 0.75},
	"usecase": {RoleOrchestrator, 0.80}, "usecases": {RoleOrchestrator, 0.80},
	"interactor": {RoleOrchestrator, 0.80},
	// Adapter
	"adapter": {RoleAdapter, 0.85}, "adapters": {RoleAdapter, 0.85},
	"handler": {RoleAdapter, 0.80}, "handlers": {RoleAdapter, 0.80},
	"controller": {RoleAdapter, 0.80}, "controllers": {RoleAdapter, 0.80},
	"repository": {RoleAdapter, 0.80}, "repositories": {RoleAdapter, 0.80},
	"middleware": {RoleAdapter, 0.75}, "server": {RoleAdapter, 0.75},
	"client": {RoleAdapter, 0.75}, "gateway": {RoleAdapter, 0.80},
	"api": {RoleAdapter, 0.70}, "grpc": {RoleAdapter, 0.85},
	"graphql": {RoleAdapter, 0.85}, "store": {RoleAdapter, 0.75},
	"cache": {RoleAdapter, 0.80}, "queue": {RoleAdapter, 0.80},
	"infra": {RoleAdapter, 0.85}, "infrastructure": {RoleAdapter, 0.85},
	"transport": {RoleAdapter, 0.80}, "delivery": {RoleAdapter, 0.80},
	"driver": {RoleAdapter, 0.80}, "drivers": {RoleAdapter, 0.80},
	// Ports
	"ports": {RolePorts, 0.90}, "port": {RolePorts, 0.90},
	// Entry point
	"cmd": {RoleEntryPoint, 0.95},
}

// stdlibIO is the set of stdlib packages that indicate I/O / infrastructure.
var stdlibIO = map[string]bool{
	"net": true, "net/http": true, "net/rpc": true, "net/smtp": true,
	"os": true, "database/sql": true,
	"encoding/json": true, "encoding/xml": true, "encoding/csv": true,
	"crypto/tls": true, "html/template": true, "text/template": true,
	"log": true, "log/slog": true,
}

// externalIOPrefixes identifies external dependencies that indicate adapter role.
var externalIOPrefixes = []string{
	"github.com/lib/pq", "github.com/go-redis/", "github.com/jackc/pgx",
	"github.com/go-sql-driver/", "google.golang.org/grpc", "go.mongodb.org/",
	"github.com/nats-io/", "github.com/segmentio/kafka",
	"github.com/gin-gonic/", "github.com/labstack/echo",
	"github.com/gorilla/", "github.com/gofiber/",
}

// ioParamTypes identifies function parameter types that indicate adapter role.
var ioParamTypes = []string{
	"http.Request", "http.ResponseWriter", "http.Handler", "http.HandlerFunc",
	"sql.DB", "sql.Tx", "sql.Conn", "sql.Row", "sql.Rows",
	"io.Reader", "io.Writer", "io.ReadCloser", "io.WriteCloser", "io.ReadWriter",
	"net.Conn", "net.Listener",
}

// classifyByNaming checks directory segments against the roleHints table.
// Uses the deepest (most specific) match.
func classifyByNaming(stripped string, pkgName string) RoleSignal {
	if pkgName == "main" {
		return RoleSignal{RoleEntryPoint, 0.95}
	}
	parts := strings.Split(strings.ReplaceAll(stripped, "\\", "/"), "/")
	var best RoleSignal
	for _, seg := range parts {
		if hint, ok := roleHints[seg]; ok {
			best = hint // last match = deepest = most specific
		}
	}
	return best
}

// classifyByImports uses stdlib/external import composition.
func classifyByImports(node *PackageNode) RoleSignal {
	if node.ImportsStdlibIO || node.ImportsExtIO {
		return RoleSignal{RoleAdapter, 0.70}
	}
	if node.Interfaces > 0 && len(node.ImportsInternal) == 0 {
		return RoleSignal{RoleCore, 0.65}
	}
	if node.Interfaces > 0 {
		return RoleSignal{RoleCore, 0.55}
	}
	// Pure library: no I/O, has types, imported by others → likely core.
	if !node.ImportsStdlibIO && !node.ImportsExtIO && !node.HasIOParams &&
		(node.Structs > 0 || node.Interfaces > 0) && len(node.ImportedBy) > 0 {
		return RoleSignal{RoleCore, 0.55}
	}
	return RoleSignal{}
}

// classifyByAST uses structural composition from analyzed files.
func classifyByAST(node *PackageNode) RoleSignal {
	if node.HasMain {
		return RoleSignal{RoleEntryPoint, 0.95}
	}
	if node.HasIOParams {
		return RoleSignal{RoleAdapter, 0.75}
	}
	total := node.Interfaces + node.Structs
	if total > 0 && float64(node.Interfaces)/float64(total) > 0.5 {
		return RoleSignal{RolePorts, 0.70}
	}
	// Stable library: imported by multiple packages, has types, no I/O → core lean.
	if len(node.ImportedBy) >= 2 && total > 0 && !node.ImportsStdlibIO && !node.ImportsExtIO {
		return RoleSignal{RoleCore, 0.60}
	}
	return RoleSignal{}
}

// fuseSignals combines multiple classification signals into a final role and confidence.
func fuseSignals(signals ...RoleSignal) (ArchRole, float64) {
	var valid []RoleSignal
	for _, s := range signals {
		if s.Confidence >= 0.30 && s.Role != "" {
			valid = append(valid, s)
		}
	}
	if len(valid) == 0 {
		return RoleUnclassified, 0.0
	}

	type vote struct {
		count   int
		maxConf float64
	}
	votes := make(map[ArchRole]*vote)
	for _, s := range valid {
		v, ok := votes[s.Role]
		if !ok {
			v = &vote{}
			votes[s.Role] = v
		}
		v.count++
		if s.Confidence > v.maxConf {
			v.maxConf = s.Confidence
		}
	}

	var bestRole ArchRole
	var bestVote *vote
	for role, v := range votes {
		if bestVote == nil || v.count > bestVote.count ||
			(v.count == bestVote.count && v.maxConf > bestVote.maxConf) {
			bestRole = role
			bestVote = v
		}
	}

	conf := bestVote.maxConf
	if bestVote.count >= 2 {
		conf += 0.10
		if conf > 0.95 {
			conf = 0.95
		}
	}

	return bestRole, conf
}

// PackageViolation represents a concrete dependency rule violation.
type PackageViolation struct {
	Message string
}

// AnnotatedPackage combines a package node with its detected role and violations.
type AnnotatedPackage struct {
	Node       *PackageNode
	Role       ArchRole
	Confidence float64
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

		role, confidence := classifyRole(stripped, pkg, modulePath, profile, node)

		var violations []PackageViolation

		// Check dependency direction violations.
		for _, imp := range node.ImportsInternal {
			impStripped := strings.TrimPrefix(imp, modulePath+"/")
			if impStripped == imp {
				impStripped = ""
			}
			impNode := g.Packages[imp]
			impRole, _ := classifyRole(impStripped, imp, modulePath, profile, impNode)

			switch role {
			case RoleCore, RolePorts:
				switch impRole {
				case RoleAdapter:
					violations = append(violations, PackageViolation{Message: "imports adapter"})
				case RoleOrchestrator:
					violations = append(violations, PackageViolation{Message: "imports application"})
				}
			case RoleOrchestrator:
				if impRole == RoleAdapter {
					violations = append(violations, PackageViolation{Message: "imports adapter"})
				}
			case RoleAdapter:
				if impRole == RoleAdapter && impStripped != stripped {
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
			Confidence: confidence,
			Violations: violations,
		}
	}

	return result
}

// classifyRole determines the architectural role of a package using
// multi-signal classification: naming, imports, and AST composition.
func classifyRole(stripped, fullPkg, modulePath string, profile *domain.ScoringProfile, node *PackageNode) (ArchRole, float64) {
	normalized := "/" + strings.ReplaceAll(stripped, "\\", "/") + "/"

	// Hard-coded rules for high-confidence patterns (legacy compatibility).
	if strings.Contains(normalized, "/cmd/") || fullPkg == modulePath {
		return RoleEntryPoint, 0.95
	}
	if strings.Contains(normalized, "/ports/") || strings.HasSuffix(strings.TrimRight(normalized, "/"), "/ports") {
		return RolePorts, 0.90
	}

	// Extract package name (last path segment).
	pkgName := ""
	parts := strings.Split(stripped, "/")
	if len(parts) > 0 {
		pkgName = parts[len(parts)-1]
	}

	// Gather signals.
	sigNaming := classifyByNaming(stripped, pkgName)

	var sigImports RoleSignal
	if node != nil {
		sigImports = classifyByImports(node)
	}

	var sigAST RoleSignal
	if node != nil {
		sigAST = classifyByAST(node)
	}

	role, conf := fuseSignals(sigNaming, sigImports, sigAST)

	if conf < 0.70 {
		return RoleUnclassified, conf
	}
	return role, conf
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
