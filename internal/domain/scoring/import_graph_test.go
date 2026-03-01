package scoring

import (
	"testing"

	"github.com/abdidvp/openkraft/internal/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func makeAnalyzedFile(path, pkg string, imports []string, interfaces, structs []string) *domain.AnalyzedFile {
	return &domain.AnalyzedFile{
		Path:       path,
		Package:    pkg,
		Imports:    imports,
		Interfaces: interfaces,
		Structs:    structs,
	}
}

// --- BuildImportGraph tests ---

func TestBuildImportGraph_BasicConstruction(t *testing.T) {
	mod := "github.com/example/app"
	analyzed := map[string]*domain.AnalyzedFile{
		"domain/model.go": makeAnalyzedFile("domain/model.go", "domain", nil,
			[]string{"Repository"}, []string{"User", "Order"}),
		"application/service.go": makeAnalyzedFile("application/service.go", "application",
			[]string{mod + "/domain"}, nil, []string{"UserService"}),
		"adapters/handler.go": makeAnalyzedFile("adapters/handler.go", "adapters",
			[]string{mod + "/application", mod + "/domain"}, nil, []string{"Handler"}),
	}

	g := BuildImportGraph(mod, analyzed)
	require.NotNil(t, g)
	assert.Len(t, g.Packages, 3)

	domainNode := g.Packages[mod+"/domain"]
	require.NotNil(t, domainNode)
	assert.Equal(t, 1, domainNode.Interfaces)
	assert.Equal(t, 2, domainNode.Structs)
	assert.Empty(t, domainNode.ImportsInternal, "domain imports nothing internal")
	assert.Len(t, domainNode.ImportedBy, 2, "domain imported by application and adapters")

	appNode := g.Packages[mod+"/application"]
	require.NotNil(t, appNode)
	assert.Len(t, appNode.ImportsInternal, 1)
	assert.Contains(t, appNode.ImportsInternal, mod+"/domain")

	adapterNode := g.Packages[mod+"/adapters"]
	require.NotNil(t, adapterNode)
	assert.Len(t, adapterNode.ImportsInternal, 2)
}

func TestBuildImportGraph_SkipsExternalImports(t *testing.T) {
	mod := "github.com/example/app"
	analyzed := map[string]*domain.AnalyzedFile{
		"main.go": makeAnalyzedFile("main.go", "main",
			[]string{"fmt", "github.com/other/lib", mod + "/domain"}, nil, nil),
		"domain/model.go": makeAnalyzedFile("domain/model.go", "domain", nil, nil, []string{"User"}),
	}

	g := BuildImportGraph(mod, analyzed)
	require.NotNil(t, g)

	mainNode := g.Packages[mod]
	require.NotNil(t, mainNode)
	assert.Len(t, mainNode.ImportsInternal, 1, "only internal import counted")
	assert.Contains(t, mainNode.ImportsInternal, mod+"/domain")
}

func TestBuildImportGraph_SkipsTestFiles(t *testing.T) {
	mod := "github.com/example/app"
	analyzed := map[string]*domain.AnalyzedFile{
		"domain/model.go": makeAnalyzedFile("domain/model.go", "domain", nil, nil, []string{"User"}),
		"domain/model_test.go": makeAnalyzedFile("domain/model_test.go", "domain_test",
			[]string{mod + "/adapters"}, nil, nil),
	}

	g := BuildImportGraph(mod, analyzed)
	require.NotNil(t, g)
	assert.Len(t, g.Packages, 1, "test file should not create any nodes or edges")
}

func TestBuildImportGraph_SkipsGeneratedFiles(t *testing.T) {
	mod := "github.com/example/app"
	analyzed := map[string]*domain.AnalyzedFile{
		"domain/model.go": makeAnalyzedFile("domain/model.go", "domain", nil, nil, []string{"User"}),
		"generated/mock.go": {
			Path: "generated/mock.go", Package: "generated", IsGenerated: true,
			Imports: []string{mod + "/domain"}, Structs: []string{"MockRepo"},
		},
	}

	g := BuildImportGraph(mod, analyzed)
	require.NotNil(t, g)
	assert.Len(t, g.Packages, 1, "generated files should be excluded")
}

func TestBuildImportGraph_EmptyModulePath(t *testing.T) {
	analyzed := map[string]*domain.AnalyzedFile{
		"main.go": makeAnalyzedFile("main.go", "main", nil, nil, nil),
	}
	g := BuildImportGraph("", analyzed)
	assert.Nil(t, g)
}

func TestBuildImportGraph_SinglePackage(t *testing.T) {
	mod := "github.com/example/app"
	analyzed := map[string]*domain.AnalyzedFile{
		"main.go":   makeAnalyzedFile("main.go", "main", []string{"fmt"}, nil, []string{"App"}),
		"config.go": makeAnalyzedFile("config.go", "main", nil, nil, []string{"Config"}),
	}

	g := BuildImportGraph(mod, analyzed)
	require.NotNil(t, g)
	assert.Len(t, g.Packages, 1, "both files in root → single package")
	node := g.Packages[mod]
	require.NotNil(t, node)
	assert.Empty(t, node.ImportsInternal)
}

// --- DetectCycles tests ---

func TestDetectCycles_NoCycles(t *testing.T) {
	g := &ImportGraph{Packages: map[string]*PackageNode{
		"a": {ImportPath: "a", ImportsInternal: []string{"b"}},
		"b": {ImportPath: "b", ImportsInternal: []string{"c"}},
		"c": {ImportPath: "c", ImportsInternal: []string{"d"}},
		"d": {ImportPath: "d"},
	}}
	// Build reverse edges.
	g.Packages["b"].ImportedBy = []string{"a"}
	g.Packages["c"].ImportedBy = []string{"b"}
	g.Packages["d"].ImportedBy = []string{"c"}

	cycles := g.DetectCycles()
	assert.Empty(t, cycles)
}

func TestDetectCycles_SimpleCycle(t *testing.T) {
	g := &ImportGraph{Packages: map[string]*PackageNode{
		"a": {ImportPath: "a", ImportsInternal: []string{"b"}},
		"b": {ImportPath: "b", ImportsInternal: []string{"a"}},
	}}

	cycles := g.DetectCycles()
	require.Len(t, cycles, 1)
	assert.Len(t, cycles[0], 2)
}

func TestDetectCycles_TransitiveCycle(t *testing.T) {
	g := &ImportGraph{Packages: map[string]*PackageNode{
		"a": {ImportPath: "a", ImportsInternal: []string{"b"}},
		"b": {ImportPath: "b", ImportsInternal: []string{"c"}},
		"c": {ImportPath: "c", ImportsInternal: []string{"a"}},
	}}

	cycles := g.DetectCycles()
	require.Len(t, cycles, 1)
	assert.Len(t, cycles[0], 3)
}

func TestDetectCycles_MultipleCycles(t *testing.T) {
	g := &ImportGraph{Packages: map[string]*PackageNode{
		"a": {ImportPath: "a", ImportsInternal: []string{"b"}},
		"b": {ImportPath: "b", ImportsInternal: []string{"a"}},
		"c": {ImportPath: "c", ImportsInternal: []string{"d"}},
		"d": {ImportPath: "d", ImportsInternal: []string{"c"}},
	}}

	cycles := g.DetectCycles()
	assert.Len(t, cycles, 2)
}

func TestDetectCycles_SelfLoop(t *testing.T) {
	g := &ImportGraph{Packages: map[string]*PackageNode{
		"a": {ImportPath: "a", ImportsInternal: []string{"a"}},
	}}

	cycles := g.DetectCycles()
	require.Len(t, cycles, 1)
	assert.Equal(t, []string{"a"}, cycles[0])
}

func TestDetectCycles_EmptyGraph(t *testing.T) {
	g := &ImportGraph{Packages: map[string]*PackageNode{}}
	cycles := g.DetectCycles()
	assert.Empty(t, cycles)
}

// --- Instability tests ---

func TestInstability_PurelyStable(t *testing.T) {
	g := &ImportGraph{Packages: map[string]*PackageNode{
		"domain": {ImportPath: "domain", ImportedBy: []string{"app", "adapters"}},
	}}
	assert.Equal(t, 0.0, g.Instability("domain"))
}

func TestInstability_PurelyUnstable(t *testing.T) {
	g := &ImportGraph{Packages: map[string]*PackageNode{
		"main": {ImportPath: "main", ImportsInternal: []string{"app", "domain"}},
	}}
	assert.Equal(t, 1.0, g.Instability("main"))
}

func TestInstability_Balanced(t *testing.T) {
	g := &ImportGraph{Packages: map[string]*PackageNode{
		"app": {ImportPath: "app", ImportsInternal: []string{"domain"}, ImportedBy: []string{"main"}},
	}}
	assert.Equal(t, 0.5, g.Instability("app"))
}

func TestInstability_Isolated(t *testing.T) {
	g := &ImportGraph{Packages: map[string]*PackageNode{
		"isolated": {ImportPath: "isolated"},
	}}
	assert.Equal(t, 0.0, g.Instability("isolated"))
}

// --- Abstractness tests ---

func TestAbstractness_AllInterfaces(t *testing.T) {
	g := &ImportGraph{Packages: map[string]*PackageNode{
		"ports": {ImportPath: "ports", Interfaces: 3, Structs: 0},
	}}
	assert.Equal(t, 1.0, g.Abstractness("ports"))
}

func TestAbstractness_AllConcrete(t *testing.T) {
	g := &ImportGraph{Packages: map[string]*PackageNode{
		"models": {ImportPath: "models", Interfaces: 0, Structs: 5},
	}}
	assert.Equal(t, 0.0, g.Abstractness("models"))
}

func TestAbstractness_Mixed(t *testing.T) {
	g := &ImportGraph{Packages: map[string]*PackageNode{
		"domain": {ImportPath: "domain", Interfaces: 2, Structs: 2},
	}}
	assert.Equal(t, 0.5, g.Abstractness("domain"))
}

func TestAbstractness_Empty(t *testing.T) {
	g := &ImportGraph{Packages: map[string]*PackageNode{
		"empty": {ImportPath: "empty"},
	}}
	assert.Equal(t, 0.0, g.Abstractness("empty"))
}

// --- DistanceFromMainSequence tests ---

func TestDistanceFromMainSequence_OnSequence(t *testing.T) {
	// A=1.0, I=0.0 → A+I=1 → D=0
	g := &ImportGraph{Packages: map[string]*PackageNode{
		"ports": {ImportPath: "ports", Interfaces: 3, Structs: 0, ImportedBy: []string{"impl"}},
	}}
	assert.InDelta(t, 0.0, g.DistanceFromMainSequence("ports"), 0.01)
}

func TestDistanceFromMainSequence_ZoneOfPain(t *testing.T) {
	// A≈0 (all concrete), I≈0 (only imported, never imports) → D ≈ 1.0
	g := &ImportGraph{Packages: map[string]*PackageNode{
		"pain": {ImportPath: "pain", Interfaces: 0, Structs: 5, ImportedBy: []string{"a", "b"}},
	}}
	assert.InDelta(t, 1.0, g.DistanceFromMainSequence("pain"), 0.01)
}

func TestDistanceFromMainSequence_ZoneOfUselessness(t *testing.T) {
	// A≈1 (all interfaces), I≈1 (imports everything, nobody imports it) → D ≈ 1.0
	g := &ImportGraph{Packages: map[string]*PackageNode{
		"useless": {ImportPath: "useless", Interfaces: 5, Structs: 0, ImportsInternal: []string{"a", "b"}},
	}}
	assert.InDelta(t, 1.0, g.DistanceFromMainSequence("useless"), 0.01)
}

// --- AverageDistance tests ---

func TestAverageDistance_MultiplePkgs(t *testing.T) {
	g := &ImportGraph{Packages: map[string]*PackageNode{
		// On main sequence: A=1, I=0 → D=0
		"ports": {ImportPath: "ports", Interfaces: 3, Structs: 0, ImportedBy: []string{"impl"}},
		// Zone of pain: A=0, I=0 → D=1
		"pain": {ImportPath: "pain", Structs: 5, ImportedBy: []string{"a"}},
		// Mid: A=0.5, I=0.5 → D=0
		"mid": {ImportPath: "mid", Interfaces: 2, Structs: 2,
			ImportsInternal: []string{"ports"}, ImportedBy: []string{"pain"}},
	}}
	// Average of 0, 1, 0 = 0.333...
	avg := g.AverageDistance()
	assert.InDelta(t, 0.333, avg, 0.01)
}

func TestAverageDistance_SkipsEmptyPkgs(t *testing.T) {
	g := &ImportGraph{Packages: map[string]*PackageNode{
		"empty":  {ImportPath: "empty"}, // 0 types → skipped
		"domain": {ImportPath: "domain", Structs: 2, ImportedBy: []string{"app"}},
	}}
	// Only domain counts: A=0, I=0 → D=1.0
	avg := g.AverageDistance()
	assert.InDelta(t, 1.0, avg, 0.01)
}

func TestAverageDistance_SinglePkg(t *testing.T) {
	g := &ImportGraph{Packages: map[string]*PackageNode{
		"domain": {ImportPath: "domain", Interfaces: 1, Structs: 1, ImportedBy: []string{"app"}},
	}}
	// A=0.5, I=0 → D=0.5
	assert.InDelta(t, 0.5, g.AverageDistance(), 0.01)
}

// --- CouplingOutliers tests ---

func TestCouplingOutliers_NoOutliers(t *testing.T) {
	g := &ImportGraph{Packages: map[string]*PackageNode{
		"a": {ImportPath: "a", ImportsInternal: []string{"b"}},
		"b": {ImportPath: "b", ImportsInternal: []string{"c"}},
		"c": {ImportPath: "c", ImportsInternal: []string{"a"}},
	}}
	outliers := g.CouplingOutliers(2.0)
	assert.Empty(t, outliers, "all packages have Ce=1, no outliers")
}

func TestCouplingOutliers_OneOutlier(t *testing.T) {
	// Ensure median Ce ≥ 1 so outlier detection is meaningful.
	g := &ImportGraph{Packages: map[string]*PackageNode{
		"god": {ImportPath: "god", ImportsInternal: []string{"a", "b", "c", "d", "e"}},
		"a":   {ImportPath: "a", ImportsInternal: []string{"b"}},
		"b":   {ImportPath: "b", ImportsInternal: []string{"c"}},
		"c":   {ImportPath: "c", ImportsInternal: []string{"d"}},
		"d":   {ImportPath: "d", ImportsInternal: []string{"e"}},
		"e":   {ImportPath: "e", ImportsInternal: []string{"a"}},
	}}
	// Sorted Ce: [1, 1, 1, 1, 1, 5] → median = 1.0
	// Threshold = 2.0 * 1.0 = 2.0, "god" Ce=5 > 2.0 → outlier
	outliers := g.CouplingOutliers(2.0)
	require.Len(t, outliers, 1)
	assert.Equal(t, "god", outliers[0].Package)
	assert.Equal(t, 5, outliers[0].Ce)
}

func TestCouplingOutliers_LowMedianReturnsNone(t *testing.T) {
	// When median Ce < 1, there's no meaningful baseline → Approach A: no penalty.
	g := &ImportGraph{Packages: map[string]*PackageNode{
		"a": {ImportPath: "a", ImportsInternal: []string{"b", "c", "d"}},
		"b": {ImportPath: "b", ImportsInternal: []string{"c"}},
		"c": {ImportPath: "c"},
		"d": {ImportPath: "d"},
	}}
	// Sorted Ce: [0, 0, 1, 3] → median = 0.5 < 1.0 → no outliers
	outliers := g.CouplingOutliers(2.0)
	assert.Empty(t, outliers, "median < 1 → no confident signal, no penalty")
}

func TestCouplingOutliers_CustomMultiplier(t *testing.T) {
	// All packages have Ce ≥ 1, so median is meaningful.
	g := &ImportGraph{Packages: map[string]*PackageNode{
		"god":  {ImportPath: "god", ImportsInternal: []string{"a", "b", "c", "d", "e"}},
		"a":    {ImportPath: "a", ImportsInternal: []string{"b"}},
		"b":    {ImportPath: "b", ImportsInternal: []string{"c"}},
		"c":    {ImportPath: "c", ImportsInternal: []string{"d"}},
		"d":    {ImportPath: "d", ImportsInternal: []string{"e"}},
		"e":    {ImportPath: "e", ImportsInternal: []string{"a"}},
	}}
	// Sorted Ce: [1, 1, 1, 1, 1, 5] → median = 1.0
	// multiplier=2.0: threshold=2.0, "god" Ce=5 > 2.0 → outlier
	outliers := g.CouplingOutliers(2.0)
	require.Len(t, outliers, 1)
	assert.Equal(t, "god", outliers[0].Package)

	// multiplier=5.0: threshold=5.0, "god" Ce=5 is NOT > 5.0
	outliers = g.CouplingOutliers(5.0)
	assert.Empty(t, outliers)
}

// --- ClassifyPackages tests ---

func TestClassifyPackages_HexagonalRoles(t *testing.T) {
	mod := "github.com/example/app"
	g := &ImportGraph{Packages: map[string]*PackageNode{
		mod + "/internal/domain":                    {ImportPath: mod + "/internal/domain"},
		mod + "/internal/domain/ports":              {ImportPath: mod + "/internal/domain/ports"},
		mod + "/internal/adapters/outbound/db":      {ImportPath: mod + "/internal/adapters/outbound/db"},
		mod + "/internal/application":               {ImportPath: mod + "/internal/application"},
		mod + "/cmd/server":                         {ImportPath: mod + "/cmd/server"},
		mod:                                         {ImportPath: mod},
	}}
	profile := domain.DefaultProfile()
	annotated := g.ClassifyPackages(mod, &profile)

	tests := []struct {
		pkg  string
		role ArchRole
	}{
		{mod + "/internal/domain", RoleCore},
		{mod + "/internal/domain/ports", RolePorts},
		{mod + "/internal/adapters/outbound/db", RoleAdapter},
		{mod + "/internal/application", RoleOrchestrator},
		{mod + "/cmd/server", RoleEntryPoint},
		{mod, RoleEntryPoint}, // root = entry point
	}

	for _, tt := range tests {
		t.Run(tt.pkg, func(t *testing.T) {
			require.Contains(t, annotated, tt.pkg)
			assert.Equal(t, tt.role, annotated[tt.pkg].Role)
		})
	}
}

func TestClassifyPackages_DependencyViolation(t *testing.T) {
	mod := "github.com/example/app"
	g := &ImportGraph{Packages: map[string]*PackageNode{
		mod + "/internal/domain": {
			ImportPath:      mod + "/internal/domain",
			ImportsInternal: []string{mod + "/internal/adapters/outbound/db"},
		},
		mod + "/internal/adapters/outbound/db": {
			ImportPath: mod + "/internal/adapters/outbound/db",
			ImportedBy: []string{mod + "/internal/domain"},
		},
	}}
	profile := domain.DefaultProfile()
	annotated := g.ClassifyPackages(mod, &profile)

	domainPkg := annotated[mod+"/internal/domain"]
	require.NotNil(t, domainPkg)
	require.Len(t, domainPkg.Violations, 1)
	assert.Equal(t, "imports adapter", domainPkg.Violations[0].Message)
}

func TestClassifyPackages_InboundToOutbound_Allowed(t *testing.T) {
	mod := "github.com/example/app"
	g := &ImportGraph{Packages: map[string]*PackageNode{
		mod + "/internal/adapters/inbound/http": {
			ImportPath:      mod + "/internal/adapters/inbound/http",
			ImportsInternal: []string{mod + "/internal/adapters/outbound/db"},
		},
		mod + "/internal/adapters/outbound/db": {
			ImportPath: mod + "/internal/adapters/outbound/db",
			ImportedBy: []string{mod + "/internal/adapters/inbound/http"},
		},
	}}
	profile := domain.DefaultProfile()
	annotated := g.ClassifyPackages(mod, &profile)

	httpPkg := annotated[mod+"/internal/adapters/inbound/http"]
	require.NotNil(t, httpPkg)
	assert.Empty(t, httpPkg.Violations, "inbound → outbound is normal hexagonal wiring, not a violation")
}

func TestClassifyPackages_OutboundToOutbound_Violation(t *testing.T) {
	mod := "github.com/example/app"
	g := &ImportGraph{Packages: map[string]*PackageNode{
		mod + "/internal/adapters/outbound/db": {
			ImportPath:      mod + "/internal/adapters/outbound/db",
			ImportsInternal: []string{mod + "/internal/adapters/outbound/cache"},
		},
		mod + "/internal/adapters/outbound/cache": {
			ImportPath: mod + "/internal/adapters/outbound/cache",
			ImportedBy: []string{mod + "/internal/adapters/outbound/db"},
		},
	}}
	profile := domain.DefaultProfile()
	annotated := g.ClassifyPackages(mod, &profile)

	dbPkg := annotated[mod+"/internal/adapters/outbound/db"]
	require.NotNil(t, dbPkg)
	require.Len(t, dbPkg.Violations, 1)
	assert.Equal(t, "imports cache", dbPkg.Violations[0].Message)
}

func TestClassifyPackages_OutboundToInbound_Violation(t *testing.T) {
	mod := "github.com/example/app"
	g := &ImportGraph{Packages: map[string]*PackageNode{
		mod + "/internal/adapters/outbound/db": {
			ImportPath:      mod + "/internal/adapters/outbound/db",
			ImportsInternal: []string{mod + "/internal/adapters/inbound/http"},
		},
		mod + "/internal/adapters/inbound/http": {
			ImportPath: mod + "/internal/adapters/inbound/http",
			ImportedBy: []string{mod + "/internal/adapters/outbound/db"},
		},
	}}
	profile := domain.DefaultProfile()
	annotated := g.ClassifyPackages(mod, &profile)

	dbPkg := annotated[mod+"/internal/adapters/outbound/db"]
	require.NotNil(t, dbPkg)
	require.Len(t, dbPkg.Violations, 1)
	assert.Equal(t, "imports http", dbPkg.Violations[0].Message)
}

func TestClassifyPackages_InboundToInbound_Violation(t *testing.T) {
	mod := "github.com/example/app"
	g := &ImportGraph{Packages: map[string]*PackageNode{
		mod + "/internal/adapters/inbound/http": {
			ImportPath:      mod + "/internal/adapters/inbound/http",
			ImportsInternal: []string{mod + "/internal/adapters/inbound/grpc"},
		},
		mod + "/internal/adapters/inbound/grpc": {
			ImportPath: mod + "/internal/adapters/inbound/grpc",
			ImportedBy: []string{mod + "/internal/adapters/inbound/http"},
		},
	}}
	profile := domain.DefaultProfile()
	annotated := g.ClassifyPackages(mod, &profile)

	httpPkg := annotated[mod+"/internal/adapters/inbound/http"]
	require.NotNil(t, httpPkg)
	require.Len(t, httpPkg.Violations, 1)
	assert.Equal(t, "imports grpc", httpPkg.Violations[0].Message)
}

func TestClassifyPackages_FlatAdapters_Violation(t *testing.T) {
	mod := "github.com/example/app"
	g := &ImportGraph{Packages: map[string]*PackageNode{
		mod + "/internal/adapters/db": {
			ImportPath:      mod + "/internal/adapters/db",
			ImportsInternal: []string{mod + "/internal/adapters/cache"},
		},
		mod + "/internal/adapters/cache": {
			ImportPath: mod + "/internal/adapters/cache",
			ImportedBy: []string{mod + "/internal/adapters/db"},
		},
	}}
	profile := domain.DefaultProfile()
	annotated := g.ClassifyPackages(mod, &profile)

	dbPkg := annotated[mod+"/internal/adapters/db"]
	require.NotNil(t, dbPkg)
	require.Len(t, dbPkg.Violations, 1, "flat adapters (no direction info) should still be violations")
	assert.Equal(t, "imports cache", dbPkg.Violations[0].Message)
}

func TestClassifyPackages_CompositionRoot_Exempt(t *testing.T) {
	mod := "github.com/example/app"
	g := &ImportGraph{Packages: map[string]*PackageNode{
		mod + "/internal/adapters/wire": {
			ImportPath:      mod + "/internal/adapters/wire",
			ImportsInternal: []string{mod + "/internal/adapters/db"},
		},
		mod + "/internal/adapters/db": {
			ImportPath: mod + "/internal/adapters/db",
			ImportedBy: []string{mod + "/internal/adapters/wire"},
		},
	}}
	profile := domain.DefaultProfile()
	profile.CompositionRoots = []string{"internal/adapters/wire"}
	annotated := g.ClassifyPackages(mod, &profile)

	wirePkg := annotated[mod+"/internal/adapters/wire"]
	require.NotNil(t, wirePkg)
	assert.Empty(t, wirePkg.Violations, "configured composition root should be exempt from adapter-to-adapter violations")
}

func TestClassifyPackages_AdapterSameSubtree(t *testing.T) {
	mod := "github.com/example/app"
	g := &ImportGraph{Packages: map[string]*PackageNode{
		mod + "/internal/adapters/outbound/db": {
			ImportPath:      mod + "/internal/adapters/outbound/db",
			ImportsInternal: []string{mod + "/internal/adapters/outbound/db/schema"},
		},
		mod + "/internal/adapters/outbound/db/schema": {
			ImportPath: mod + "/internal/adapters/outbound/db/schema",
			ImportedBy: []string{mod + "/internal/adapters/outbound/db"},
		},
	}}
	profile := domain.DefaultProfile()
	annotated := g.ClassifyPackages(mod, &profile)

	dbPkg := annotated[mod+"/internal/adapters/outbound/db"]
	require.NotNil(t, dbPkg)
	assert.Empty(t, dbPkg.Violations, "same subtree imports are not violations")
}

func TestClassifyPackages_CycleViolation(t *testing.T) {
	mod := "github.com/example/app"
	g := &ImportGraph{Packages: map[string]*PackageNode{
		mod + "/internal/domain/a": {
			ImportPath:      mod + "/internal/domain/a",
			ImportsInternal: []string{mod + "/internal/domain/b"},
			ImportedBy:      []string{mod + "/internal/domain/b"},
		},
		mod + "/internal/domain/b": {
			ImportPath:      mod + "/internal/domain/b",
			ImportsInternal: []string{mod + "/internal/domain/a"},
			ImportedBy:      []string{mod + "/internal/domain/a"},
		},
	}}
	profile := domain.DefaultProfile()
	annotated := g.ClassifyPackages(mod, &profile)

	for _, pkg := range []string{mod + "/internal/domain/a", mod + "/internal/domain/b"} {
		ap := annotated[pkg]
		require.NotNil(t, ap)
		var hasMsg bool
		for _, v := range ap.Violations {
			if v.Message == "in import cycle" {
				hasMsg = true
			}
		}
		assert.True(t, hasMsg, "%s should have 'in import cycle' violation", pkg)
	}
}

func TestClassifyPackages_CleanHexagonNoViolations(t *testing.T) {
	mod := "github.com/example/app"
	g := &ImportGraph{Packages: map[string]*PackageNode{
		mod + "/internal/domain": {
			ImportPath: mod + "/internal/domain",
			ImportedBy: []string{mod + "/internal/application", mod + "/internal/adapters/inbound/http"},
		},
		mod + "/internal/application": {
			ImportPath:      mod + "/internal/application",
			ImportsInternal: []string{mod + "/internal/domain"},
			ImportedBy:      []string{mod + "/internal/adapters/inbound/http"},
		},
		mod + "/internal/adapters/inbound/http": {
			ImportPath:      mod + "/internal/adapters/inbound/http",
			ImportsInternal: []string{mod + "/internal/application", mod + "/internal/domain"},
		},
	}}
	profile := domain.DefaultProfile()
	annotated := g.ClassifyPackages(mod, &profile)

	assert.Equal(t, 0, TotalViolations(annotated))
}

func TestClassifyPackages_LayerAliases(t *testing.T) {
	mod := "github.com/example/app"
	g := &ImportGraph{Packages: map[string]*PackageNode{
		mod + "/internal/infra/db": {ImportPath: mod + "/internal/infra/db"},
	}}
	profile := domain.DefaultProfile()
	profile.LayerAliases = map[string]string{"infra": "adapters"}
	annotated := g.ClassifyPackages(mod, &profile)

	ap := annotated[mod+"/internal/infra/db"]
	require.NotNil(t, ap)
	assert.Equal(t, RoleAdapter, ap.Role)
}

func TestAdapterDirection(t *testing.T) {
	tests := []struct {
		path string
		want string
	}{
		{"internal/adapters/inbound/cli", "inbound"},
		{"internal/adapters/outbound/db", "outbound"},
		{"internal/adapters/inbound/http/middleware", "inbound"},
		{"internal/adapters/outbound/cache/redis", "outbound"},
		{"internal/adapters/db", ""},           // flat — no direction
		{"internal/infra/inbound/http", "inbound"}, // alias
		{"internal/infrastructure/outbound/db", "outbound"},
		{"domain/model", ""},                   // not an adapter at all
	}
	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			assert.Equal(t, tt.want, adapterDirection(tt.path))
		})
	}
}

func TestIsCompositionRoot(t *testing.T) {
	profile := &domain.ScoringProfile{
		CompositionRoots: []string{"internal/adapters/wire", "cmd/server"},
	}

	tests := []struct {
		path string
		want bool
	}{
		{"internal/adapters/wire", true},       // exact match
		{"internal/adapters/wire/di", true},    // child match
		{"cmd/server", true},                   // exact match
		{"cmd/server/routes", true},            // child match
		{"internal/adapters/db", false},        // non-match
		{"internal/adapters/wired", false},     // prefix but not a child
	}
	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			assert.Equal(t, tt.want, isCompositionRoot(tt.path, profile))
		})
	}
}

func TestTotalViolations(t *testing.T) {
	annotated := map[string]*AnnotatedPackage{
		"a": {Violations: []PackageViolation{{Message: "v1"}, {Message: "v2"}}},
		"b": {Violations: nil},
		"c": {Violations: []PackageViolation{{Message: "v3"}}},
	}
	assert.Equal(t, 3, TotalViolations(annotated))
}

// --- scoreImportGraph tests ---

func TestScoreImportGraph_NilGraph(t *testing.T) {
	p := domain.DefaultProfile()
	assert.Equal(t, 1.0, scoreImportGraph(nil, &p))
}

func TestScoreImportGraph_SinglePackage(t *testing.T) {
	p := domain.DefaultProfile()
	g := &ImportGraph{Packages: map[string]*PackageNode{
		"main": {ImportPath: "main"},
	}}
	assert.Equal(t, 1.0, scoreImportGraph(g, &p))
}

func TestScoreImportGraph_CleanGraph(t *testing.T) {
	p := domain.DefaultProfile()
	g := &ImportGraph{Packages: map[string]*PackageNode{
		"domain": {ImportPath: "domain", Interfaces: 2, Structs: 1, ImportedBy: []string{"app"}},
		"app":    {ImportPath: "app", Structs: 1, ImportsInternal: []string{"domain"}, ImportedBy: []string{"main"}},
		"main":   {ImportPath: "main", Structs: 1, ImportsInternal: []string{"app"}},
	}}
	score := scoreImportGraph(g, &p)
	assert.Greater(t, score, 0.5, "clean DAG should score well")
}

func TestScoreImportGraph_WithCycles(t *testing.T) {
	p := domain.DefaultProfile()
	g := &ImportGraph{Packages: map[string]*PackageNode{
		"a": {ImportPath: "a", Structs: 1, ImportsInternal: []string{"b"}},
		"b": {ImportPath: "b", Structs: 1, ImportsInternal: []string{"a"}},
	}}
	score := scoreImportGraph(g, &p)
	assert.Less(t, score, 0.7, "cycles should significantly reduce score")
}
