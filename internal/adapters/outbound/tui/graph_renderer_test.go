package tui

import (
	"fmt"
	"strings"
	"testing"

	"github.com/abdidvp/openkraft/internal/domain"
	"github.com/abdidvp/openkraft/internal/domain/scoring"
	"github.com/stretchr/testify/assert"
)

func TestRenderGraph_NilGraph(t *testing.T) {
	profile := domain.DefaultProfile()
	out := RenderGraph(nil, "example.com/app", &profile)
	assert.Contains(t, out, "No import graph available")
}

func TestRenderGraph_EmptyGraph(t *testing.T) {
	graph := &scoring.ImportGraph{Packages: map[string]*scoring.PackageNode{}}
	profile := domain.DefaultProfile()
	out := RenderGraph(graph, "example.com/app", &profile)
	assert.Contains(t, out, "No import graph available")
}

func TestRenderGraph_SinglePackage(t *testing.T) {
	graph := &scoring.ImportGraph{
		Packages: map[string]*scoring.PackageNode{
			"example.com/app": {
				ImportPath: "example.com/app",
				Structs:    2,
			},
		},
	}
	profile := domain.DefaultProfile()
	out := RenderGraph(graph, "example.com/app", &profile)

	assert.Contains(t, out, "Import Graph")
	assert.Contains(t, out, "1 packages")
	assert.Contains(t, out, "0 edges")
	assert.Contains(t, out, "0 cycles")
	assert.Contains(t, out, "(none)") // no cycles, no outliers
}

func TestRenderGraph_BasicOutput(t *testing.T) {
	graph := &scoring.ImportGraph{
		Packages: map[string]*scoring.PackageNode{
			"example.com/proj/domain": {
				ImportPath: "example.com/proj/domain",
				ImportedBy: []string{"example.com/proj/application"},
				Interfaces: 2,
				Structs:    3,
			},
			"example.com/proj/application": {
				ImportPath:      "example.com/proj/application",
				ImportsInternal: []string{"example.com/proj/domain"},
				Structs:         1,
			},
			"example.com/proj/adapters": {
				ImportPath:      "example.com/proj/adapters",
				ImportsInternal: []string{"example.com/proj/application", "example.com/proj/domain"},
				Structs:         2,
			},
		},
	}
	profile := domain.DefaultProfile()
	out := RenderGraph(graph, "example.com/proj", &profile)

	assert.Contains(t, out, "Import Graph")
	assert.Contains(t, out, "3 packages")
	assert.Contains(t, out, "3 edges")
	assert.Contains(t, out, "Package")
	assert.Contains(t, out, "Ca")
	assert.Contains(t, out, "Ce")
	assert.Contains(t, out, "Role")
	assert.Contains(t, out, "Violations")
	// I/A/D/Status columns should NOT be present.
	assert.NotContains(t, out, "Status")
	// Packages should appear with stripped prefix.
	assert.Contains(t, out, "domain")
	assert.Contains(t, out, "application")
	assert.Contains(t, out, "adapters")
}

func TestRenderGraph_CyclesShown(t *testing.T) {
	graph := &scoring.ImportGraph{
		Packages: map[string]*scoring.PackageNode{
			"example.com/proj/a": {
				ImportPath:      "example.com/proj/a",
				ImportsInternal: []string{"example.com/proj/b"},
				ImportedBy:      []string{"example.com/proj/b"},
				Structs:         1,
			},
			"example.com/proj/b": {
				ImportPath:      "example.com/proj/b",
				ImportsInternal: []string{"example.com/proj/a"},
				ImportedBy:      []string{"example.com/proj/a"},
				Structs:         1,
			},
		},
	}
	profile := domain.DefaultProfile()
	out := RenderGraph(graph, "example.com/proj", &profile)

	assert.Contains(t, out, "Cycles")
	// Should show a → b → a cycle notation
	assert.True(t, strings.Contains(out, "→"), "cycle should contain arrow notation")
}

func TestRenderGraph_OutliersShown(t *testing.T) {
	// Chain: a→b→c→d→e, plus hub→(a,b,c,d,e).
	// Ce values: a=1, b=1, c=1, d=1, e=0, hub=5 → median=1, hub is outlier (5 > 2*1).
	graph := &scoring.ImportGraph{
		Packages: map[string]*scoring.PackageNode{
			"example.com/proj/hub": {
				ImportPath:      "example.com/proj/hub",
				ImportsInternal: []string{"example.com/proj/a", "example.com/proj/b", "example.com/proj/c", "example.com/proj/d", "example.com/proj/e"},
				Structs:         1,
			},
			"example.com/proj/a": {ImportPath: "example.com/proj/a", ImportsInternal: []string{"example.com/proj/b"}, ImportedBy: []string{"example.com/proj/hub"}, Structs: 1},
			"example.com/proj/b": {ImportPath: "example.com/proj/b", ImportsInternal: []string{"example.com/proj/c"}, ImportedBy: []string{"example.com/proj/hub", "example.com/proj/a"}, Structs: 1},
			"example.com/proj/c": {ImportPath: "example.com/proj/c", ImportsInternal: []string{"example.com/proj/d"}, ImportedBy: []string{"example.com/proj/hub", "example.com/proj/b"}, Structs: 1},
			"example.com/proj/d": {ImportPath: "example.com/proj/d", ImportsInternal: []string{"example.com/proj/e"}, ImportedBy: []string{"example.com/proj/hub", "example.com/proj/c"}, Structs: 1},
			"example.com/proj/e": {ImportPath: "example.com/proj/e", ImportedBy: []string{"example.com/proj/hub", "example.com/proj/d"}, Structs: 1},
		},
	}
	profile := domain.DefaultProfile()
	out := RenderGraph(graph, "example.com/proj", &profile)

	assert.Contains(t, out, "Coupling Outliers")
	assert.Contains(t, out, "imports 5 packages")
}

func TestRenderGraph_TruncatesLargeProjects(t *testing.T) {
	packages := make(map[string]*scoring.PackageNode)
	for i := 0; i < 20; i++ {
		pkg := fmt.Sprintf("example.com/proj/pkg%02d", i)
		packages[pkg] = &scoring.PackageNode{
			ImportPath: pkg,
			Structs:    1,
		}
	}
	graph := &scoring.ImportGraph{Packages: packages}
	profile := domain.DefaultProfile()
	out := RenderGraph(graph, "example.com/proj", &profile)

	assert.Contains(t, out, "more packages")
}

func TestRenderGraph_RoleLabelsShown(t *testing.T) {
	graph := &scoring.ImportGraph{
		Packages: map[string]*scoring.PackageNode{
			"example.com/proj/internal/domain": {
				ImportPath: "example.com/proj/internal/domain",
				ImportedBy: []string{"example.com/proj/internal/application"},
			},
			"example.com/proj/internal/application": {
				ImportPath:      "example.com/proj/internal/application",
				ImportsInternal: []string{"example.com/proj/internal/domain"},
				ImportedBy:      []string{"example.com/proj/internal/adapters/http"},
			},
			"example.com/proj/internal/adapters/http": {
				ImportPath:      "example.com/proj/internal/adapters/http",
				ImportsInternal: []string{"example.com/proj/internal/application"},
			},
		},
	}
	profile := domain.DefaultProfile()
	out := RenderGraph(graph, "example.com/proj", &profile)

	assert.Contains(t, out, "core")
	assert.Contains(t, out, "orchestrator")
	assert.Contains(t, out, "adapter")
}

func TestRenderGraph_ViolationShown(t *testing.T) {
	graph := &scoring.ImportGraph{
		Packages: map[string]*scoring.PackageNode{
			"example.com/proj/internal/domain": {
				ImportPath:      "example.com/proj/internal/domain",
				ImportsInternal: []string{"example.com/proj/internal/adapters/db"},
			},
			"example.com/proj/internal/adapters/db": {
				ImportPath: "example.com/proj/internal/adapters/db",
				ImportedBy: []string{"example.com/proj/internal/domain"},
			},
		},
	}
	profile := domain.DefaultProfile()
	out := RenderGraph(graph, "example.com/proj", &profile)

	assert.Contains(t, out, "imports adapter")
}

func TestRenderGraph_ZeroViolationsInHeader(t *testing.T) {
	graph := &scoring.ImportGraph{
		Packages: map[string]*scoring.PackageNode{
			"example.com/proj/internal/domain": {
				ImportPath: "example.com/proj/internal/domain",
				ImportedBy: []string{"example.com/proj/internal/application"},
			},
			"example.com/proj/internal/application": {
				ImportPath:      "example.com/proj/internal/application",
				ImportsInternal: []string{"example.com/proj/internal/domain"},
			},
		},
	}
	profile := domain.DefaultProfile()
	out := RenderGraph(graph, "example.com/proj", &profile)

	assert.Contains(t, out, "0 violations")
}
