# OpenKraft Phase 1 MVP: score + check + MCP — Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Build `openkraft score` (diagnóstico), `openkraft check` (prescripción), and `openkraft mcp serve` (bridge to AI agents) — the complete value loop where OpenKraft analyzes, AI agents fix.

**Architecture:** Vertical-slice build in 4 milestones. Score working by task 10, full scoring by task 15, check engine by task 21, MCP server by task 25. Each layer builds on the previous. All analysis is pure Go AST — no LLM, no WASM, no TypeScript.

**Tech Stack:** Go 1.24, Cobra (CLI), Lipgloss (TUI), go/ast + go/parser (analysis), go-git/v5 (git), mark3labs/mcp-go (MCP server), testify (testing)

**Design doc:** `docs/plans/2026-02-23-openkraft-design.md` (vision document — this plan replaces its roadmap)

---

## Strategic Context

The original design doc is a vision for the full product. This plan is the realistic first delivery:

- **In scope:** `score` + `check` + MCP server. Go only. Pure AST. No LLM.
- **Out of scope permanently (no `fix`):** OpenKraft does NOT generate code. AI agents (Claude Code, Cursor) do the fixing via MCP.
- **Out of scope for now:** TypeScript, WASM plugins, watch mode, `init`, static file generation, npm/Homebrew distribution.

**The value loop:**
```
Developer runs: openkraft score → "47/100"
Developer runs: openkraft check payments → "missing 9 files, 3 methods"
Developer opens Claude Code with OpenKraft MCP connected
Claude Code asks OpenKraft: "what's missing in payments?" → gets structured answer
Claude Code generates the missing code following golden module patterns
Developer runs: openkraft score → "82/100"
```

**GO/KILL gate after completion:**
- Run against 15+ real repos, document results
- False positive rate <20%
- 5+ external users ran score voluntarily
- Score consistently differentiates good vs bad projects
- If scoring is not useful after 3 calibration rounds → pivot or kill

---

## Milestone Map

```
Tasks  1-5:   Foundation (types, fixture, scanner, module detector, AST analyzer)
Tasks  6-10:  WORKING SCORE CLI (2 scorers + orchestrator + TUI + command)
Tasks 11-15:  FULL SCORING (4 remaining scorers + varied fixtures)
Tasks 16-21:  WORKING CHECK (golden module, blueprint, comparator, check command)
Tasks 22-25:  WORKING MCP SERVER (tools + resources + serve command)
Tasks 26-28:  POLISH (E2E tests, git/history, README)
```

---

## Task 1: Project Scaffolding

**Files:**
- Create: `go.mod`
- Create: `cmd/openkraft/main.go`
- Create: `internal/cli/root.go`
- Create: `internal/cli/version.go`
- Create: `Makefile`
- Create: `.golangci.yml`
- Create: `.gitignore`

**Step 1: Initialize Go module**

```bash
go mod init github.com/openkraft/openkraft
```

**Step 2: Create main entry point**

`cmd/openkraft/main.go`:
```go
package main

import (
	"os"

	"github.com/openkraft/openkraft/internal/cli"
)

func main() {
	if err := cli.Execute(); err != nil {
		os.Exit(1)
	}
}
```

**Step 3: Create root command**

`internal/cli/root.go`:
```go
package cli

import "github.com/spf13/cobra"

var (
	version = "dev"
	commit  = "none"
)

func newRootCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "openkraft",
		Short: "Stop shipping 80% code",
		Long:  "OpenKraft scores your codebase's AI-readiness and enforces that every module meets the quality of your best module.",
		SilenceUsage:  true,
		SilenceErrors: true,
	}
	cmd.AddCommand(newVersionCmd())
	return cmd
}

// NewRootCmdForTest returns the root command for testing.
func NewRootCmdForTest() *cobra.Command {
	return newRootCmd()
}

func Execute() error {
	return newRootCmd().Execute()
}
```

`internal/cli/version.go`:
```go
package cli

import (
	"fmt"
	"github.com/spf13/cobra"
)

func newVersionCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Show OpenKraft version",
		RunE: func(cmd *cobra.Command, args []string) error {
			fmt.Fprintf(cmd.OutOrStdout(), "openkraft %s (%s)\n", version, commit)
			return nil
		},
	}
}
```

**Step 4: Create Makefile**

```makefile
.PHONY: build test lint

VERSION ?= dev
COMMIT := $(shell git rev-parse --short HEAD 2>/dev/null || echo "none")
LDFLAGS := -ldflags "-X github.com/openkraft/openkraft/internal/cli.version=$(VERSION) -X github.com/openkraft/openkraft/internal/cli.commit=$(COMMIT)"

build:
	go build $(LDFLAGS) -o bin/openkraft ./cmd/openkraft

test:
	go test ./... -race -count=1

lint:
	golangci-lint run ./...

install:
	go install $(LDFLAGS) ./cmd/openkraft
```

**Step 5: Create .gitignore and .golangci.yml**

`.gitignore`:
```
bin/
dist/
*.exe
*.test
*.out
.DS_Store
```

`.golangci.yml`:
```yaml
run:
  timeout: 5m
linters:
  enable:
    - errcheck
    - govet
    - staticcheck
    - unused
    - gosimple
    - ineffassign
```

**Step 6: Install dependencies and verify**

```bash
go get github.com/spf13/cobra@latest
go mod tidy
make build
./bin/openkraft version
```

Expected: `openkraft dev (<hash>)`

**Step 7: Commit**

```bash
git add -A && git commit -m "feat: project scaffolding with cobra CLI"
```

---

## Task 2: Core Types — Score, Module, Issue

**Files:**
- Create: `pkg/types/score.go`
- Create: `pkg/types/module.go`
- Create: `pkg/types/issue.go`
- Test: `pkg/types/score_test.go`

**Step 1: Write score types test**

`pkg/types/score_test.go`:
```go
package types_test

import (
	"testing"
	"github.com/openkraft/openkraft/pkg/types"
	"github.com/stretchr/testify/assert"
)

func TestScore_Grade(t *testing.T) {
	tests := []struct {
		score int
		grade string
	}{
		{95, "A+"}, {85, "A"}, {75, "B"}, {65, "C"}, {55, "D"}, {45, "F"}, {0, "F"}, {100, "A+"},
	}
	for _, tt := range tests {
		s := types.Score{Overall: tt.score}
		assert.Equal(t, tt.grade, s.Grade(), "score %d", tt.score)
	}
}

func TestScore_WeightedAverage(t *testing.T) {
	categories := []types.CategoryScore{
		{Name: "architecture", Score: 80, Weight: 0.25},
		{Name: "conventions", Score: 60, Weight: 0.20},
		{Name: "patterns", Score: 40, Weight: 0.20},
		{Name: "tests", Score: 70, Weight: 0.15},
		{Name: "ai_context", Score: 20, Weight: 0.10},
		{Name: "completeness", Score: 50, Weight: 0.10},
	}
	score := types.ComputeOverallScore(categories)
	assert.Equal(t, 58, score) // (80*25+60*20+40*20+70*15+20*10+50*10)/100 = 57.5 → 58
}
```

**Step 2: Run test, verify it fails**

```bash
go test ./pkg/types/... -v
```

**Step 3: Implement types**

`pkg/types/score.go`:
```go
package types

import (
	"math"
	"time"
)

type Score struct {
	Overall      int             `json:"overall"`
	Categories   []CategoryScore `json:"categories"`
	Timestamp    time.Time       `json:"timestamp"`
	CommitHash   string          `json:"commit_hash,omitempty"`
	ModuleScores []ModuleScore   `json:"module_scores,omitempty"`
}

func (s Score) Grade() string { return GradeFor(s.Overall) }

func GradeFor(score int) string {
	switch {
	case score >= 90: return "A+"
	case score >= 80: return "A"
	case score >= 70: return "B"
	case score >= 60: return "C"
	case score >= 50: return "D"
	default:          return "F"
	}
}

func BadgeColor(score int) string {
	switch {
	case score >= 90: return "brightgreen"
	case score >= 80: return "green"
	case score >= 70: return "yellow"
	case score >= 60: return "orange"
	case score >= 50: return "red"
	default:          return "critical"
	}
}

type CategoryScore struct {
	Name       string      `json:"name"`
	Score      int         `json:"score"`
	Weight     float64     `json:"weight"`
	SubMetrics []SubMetric `json:"sub_metrics,omitempty"`
	Issues     []Issue     `json:"issues,omitempty"`
}

type SubMetric struct {
	Name   string `json:"name"`
	Score  int    `json:"score"`
	Points int    `json:"points"`
	Detail string `json:"detail,omitempty"`
}

type ModuleScore struct {
	Name           string `json:"name"`
	Path           string `json:"path"`
	Score          int    `json:"score"`
	FileCount      int    `json:"file_count"`
	MissingFiles   int    `json:"missing_files"`
	MissingMethods int    `json:"missing_methods"`
	Issues         []Issue `json:"issues,omitempty"`
}

func ComputeOverallScore(categories []CategoryScore) int {
	var totalWeighted, totalWeight float64
	for _, c := range categories {
		totalWeighted += float64(c.Score) * c.Weight
		totalWeight += c.Weight
	}
	if totalWeight == 0 { return 0 }
	return int(math.Round(totalWeighted / totalWeight))
}
```

`pkg/types/issue.go`:
```go
package types

const (
	SeverityError   = "error"
	SeverityWarning = "warning"
	SeverityInfo    = "info"
)

type Issue struct {
	Severity     string `json:"severity"`
	Category     string `json:"category"`
	File         string `json:"file,omitempty"`
	Line         int    `json:"line,omitempty"`
	Message      string `json:"message"`
	Pattern      string `json:"pattern,omitempty"`
	FixAvailable bool   `json:"fix_available"`
}
```

`pkg/types/module.go`:
```go
package types

type Module struct {
	Name     string       `json:"name"`
	Path     string       `json:"path"`
	Language string       `json:"language"`
	Files    []ModuleFile `json:"files"`
	Layers   []string     `json:"layers,omitempty"`
}

type ModuleFile struct {
	Path         string   `json:"path"`
	RelativePath string   `json:"relative_path"`
	Layer        string   `json:"layer,omitempty"`
	Type         string   `json:"type,omitempty"`
	HasTest      bool     `json:"has_test"`
	Functions    []string `json:"functions,omitempty"`
	Structs      []string `json:"structs,omitempty"`
	Interfaces   []string `json:"interfaces,omitempty"`
	Imports      []string `json:"imports,omitempty"`
}
```

**Step 4: Run tests, verify pass**

```bash
go get github.com/stretchr/testify@latest && go mod tidy
go test ./pkg/types/... -v
```

**Step 5: Commit**

```bash
git add -A && git commit -m "feat: core types for score, module, and issue"
```

---

## Task 3: Test Fixture (Perfect) + File Scanner

**Files:**
- Create: `testdata/go-hexagonal/perfect/` (full fixture)
- Create: `internal/analysis/file_scanner.go`
- Test: `internal/analysis/file_scanner_test.go`

### Part A: Create "perfect" test fixture

Every `.go` file must be valid, compilable Go. This fixture is used by ALL subsequent tests.

```
testdata/go-hexagonal/perfect/
├── go.mod                            (module example.com/perfect)
├── cmd/api/main.go
├── internal/
│   ├── tax/
│   │   ├── domain/
│   │   │   ├── tax_rule.go           (type TaxRule struct; func NewTaxRule; func (*TaxRule) Validate() error)
│   │   │   ├── tax_rule_test.go      (func TestNewTaxRule; func TestTaxRule_Validate)
│   │   │   └── tax_errors.go         (var ErrTaxRuleInvalid = errors.New(...))
│   │   ├── application/
│   │   │   ├── tax_service.go        (type TaxService struct; func NewTaxService)
│   │   │   └── tax_ports.go          (type TaxRuleRepository interface{Create;GetByID;List})
│   │   └── adapters/
│   │       ├── http/
│   │       │   ├── tax_handler.go    (type TaxHandler struct)
│   │       │   └── tax_routes.go     (func RegisterTaxRoutes)
│   │       └── repository/
│   │           └── tax_repository.go (type PostgresTaxRuleRepository; func getQuerier)
│   ├── inventory/
│   │   ├── domain/
│   │   │   ├── product.go            (type Product struct; func NewProduct; func (*Product) Validate() error)
│   │   │   └── product_test.go
│   │   ├── application/
│   │   │   ├── inventory_service.go
│   │   │   └── inventory_ports.go
│   │   └── adapters/
│   │       ├── http/
│   │       │   └── inventory_handler.go
│   │       └── repository/
│   │           └── product_repository.go (with getQuerier)
│   └── payments/                      (intentionally INCOMPLETE — only domain, no tests, no Validate)
│       └── domain/
│           └── payment.go             (type Payment struct — no Validate, no test)
├── CLAUDE.md                          (>500 bytes with ## sections)
└── .cursorrules                       (>200 bytes)
```

### Part B: File Scanner

`internal/analysis/file_scanner.go` — Walks project directory, ignores vendor/node_modules/.git, detects Go files, test files, AI context files. Returns `ScanResult`.

**Step 1: Write file scanner test**

`internal/analysis/file_scanner_test.go`:
```go
package analysis_test

import (
	"testing"
	"github.com/openkraft/openkraft/internal/analysis"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFileScanner_ScanProject(t *testing.T) {
	scanner := analysis.NewFileScanner()
	result, err := scanner.Scan("../../testdata/go-hexagonal/perfect")
	require.NoError(t, err)
	assert.Equal(t, "go", result.Language)
	assert.True(t, len(result.GoFiles) > 0)
	assert.True(t, len(result.TestFiles) > 0)
}

func TestFileScanner_DetectsAIContextFiles(t *testing.T) {
	scanner := analysis.NewFileScanner()
	result, err := scanner.Scan("../../testdata/go-hexagonal/perfect")
	require.NoError(t, err)
	assert.True(t, result.HasClaudeMD)
	assert.True(t, result.HasCursorRules)
}
```

**Step 2: Implement file scanner** (see previous plan for full code)

**Step 3: Run tests, verify pass**

**Step 4: Commit**

```bash
git add -A && git commit -m "feat: test fixture (perfect) and file scanner"
```

---

## Task 4: Module Detector

**Files:**
- Create: `internal/analysis/module_detector.go`
- Test: `internal/analysis/module_detector_test.go`

Detects module boundaries from ScanResult. Modules are `internal/*/` dirs with Go files. Detects layers (domain, application, adapters/http, adapters/repository) by subdirectory structure.

**Step 1: Write test** — Verify finds tax, inventory, payments. Verify tax has 4 layers. Verify files assigned.

**Step 2: Implement** — Returns `[]DetectedModule{Name, Path, Layers, Files}`.

**Step 3: Run tests, commit**

```bash
git add -A && git commit -m "feat: module detector for Go hexagonal projects"
```

---

## Task 5: Go AST Analyzer

**Files:**
- Create: `internal/analysis/go_analyzer.go`
- Test: `internal/analysis/go_analyzer_test.go`

Uses `go/ast` + `go/parser` to parse Go files. Extracts: structs, functions, methods (with receiver), interfaces, imports. Returns `AnalyzedFile`.

**Step 1: Write test** — Parse `tax_rule.go`, verify finds `TaxRule` struct, `NewTaxRule` func, `Validate` method with `*TaxRule` receiver.

**Step 2: Implement** — `go/parser.ParseFile` + `ast.Inspect` walker.

**Step 3: Run tests, commit**

```bash
git add -A && git commit -m "feat: Go AST analyzer for struct/function/interface extraction"
```

---

## Task 6: Architecture Clarity Scorer (25% weight)

**Files:**
- Create: `internal/engine/scoring/architecture.go`
- Test: `internal/engine/scoring/architecture_test.go`

Sub-metrics (per design doc Section 7):
- Consistent module structure (30 pts) — same subdirectory pattern across modules
- Layer separation (25 pts) — domain doesn't import adapters
- Dependency direction (20 pts) — dependencies flow inward
- Module boundary clarity (15 pts) — clear separation
- Architecture documentation (10 pts) — docs exist

**Step 1: Write test** — Perfect fixture scores high. Receives analyzed modules + scan result.

**Step 2: Implement** — Returns `types.CategoryScore`.

**Step 3: Run tests, commit**

```bash
git add -A && git commit -m "feat: architecture clarity scorer"
```

---

## Task 7: Test Infrastructure Scorer (15% weight)

**Files:**
- Create: `internal/engine/scoring/tests.go`
- Test: `internal/engine/scoring/tests_test.go`

Sub-metrics: unit test presence (25), integration tests (25), test helpers (15), test fixtures (15), CI config (20).

**Step 1: Write test**

**Step 2: Implement** — Uses file scanner results mostly. Calculates test-to-source ratio.

**Step 3: Run tests, commit**

```bash
git add -A && git commit -m "feat: test infrastructure scorer"
```

---

## Task 8: Scoring Orchestrator

**Files:**
- Create: `internal/engine/scoring/scorer.go`
- Test: `internal/engine/scoring/scorer_test.go`

Wires analysis → scoring. Initially 2 categories, designed to accept all 6. Adding a scorer = adding one function call, no structural change.

1. File scan → 2. Module detection → 3. Go AST analysis → 4. Run scorers → 5. Weighted average → 6. Return `types.Score`

**Step 1: Write test** — Score perfect fixture, verify 2 categories returned, overall computed.

**Step 2: Implement**

**Step 3: Run tests, commit**

```bash
git add -A && git commit -m "feat: scoring orchestrator with architecture + tests categories"
```

---

## Task 9: TUI Renderer

**Files:**
- Create: `internal/tui/renderer.go`
- Create: `internal/tui/colors.go`
- Test: `internal/tui/renderer_test.go`

Renders the Lighthouse-style score display with Lipgloss:

```
  ╔══════════════════════════════════════════╗
  ║        OpenKraft AI-Readiness Score      ║
  ║                  47 / 100                ║
  ╚══════════════════════════════════════════╝

  Architecture Clarity     ██████████░░░░░  67/100  (weight: 25%)
  Convention Coverage      ████░░░░░░░░░░░  28/100  (weight: 20%)
  ...

  Grade: D  |  Run `openkraft init` to improve your score
```

Color palette from design doc: deep blue (#1a1a2e) + electric green (#00ff88).

**Step 1: Implement colors** — Grade colors (green A+/A, yellow B, orange C, red D/F).

**Step 2: Write renderer test** — `RenderScore()` returns non-empty string with score and category names.

**Step 3: Implement renderer** — Lipgloss borders, `█`/`░` progress bars.

**Step 4: Run tests, commit**

```bash
git add -A && git commit -m "feat: TUI score renderer with Lipgloss styling"
```

---

## Task 10: `openkraft score` CLI Command

**Files:**
- Create: `internal/cli/score.go`
- Test: `internal/cli/score_test.go`
- Modify: `internal/cli/root.go` — add `newScoreCmd()`

Flags: `--json`, `--ci`, `--min N`, `--badge`, `--detail` (stub), `--category NAME`

**Step 1: Write CLI test**

```go
func TestScoreCommand_JSON(t *testing.T) {
	cmd := cli.NewRootCmdForTest()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetArgs([]string{"score", "../../testdata/go-hexagonal/perfect", "--json"})
	require.NoError(t, cmd.Execute())
	assert.Contains(t, buf.String(), `"overall"`)
}

func TestScoreCommand_CIFails(t *testing.T) {
	cmd := cli.NewRootCmdForTest()
	cmd.SetArgs([]string{"score", "../../testdata/go-hexagonal/perfect", "--ci", "--min", "100"})
	assert.Error(t, cmd.Execute())
}

func TestScoreCommand_Badge(t *testing.T) {
	cmd := cli.NewRootCmdForTest()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetArgs([]string{"score", "../../testdata/go-hexagonal/perfect", "--badge"})
	require.NoError(t, cmd.Execute())
	assert.Contains(t, buf.String(), "img.shields.io")
}
```

**Step 2: Implement score command**

**Step 3: Manual smoke test**

```bash
make build
./bin/openkraft score ./testdata/go-hexagonal/perfect
./bin/openkraft score ./testdata/go-hexagonal/perfect --json
./bin/openkraft score ./testdata/go-hexagonal/perfect --badge
```

**Step 4: Commit**

```bash
git add -A && git commit -m "feat: openkraft score command with JSON, CI, and badge support"
```

---

> ### MILESTONE 1: Working `openkraft score`
> From here on, every change produces visible output.

---

## Task 11: Additional Test Fixtures

**Files:**
- Create: `testdata/go-hexagonal/incomplete/` — 3 modules, 1 complete, 1 missing tests, 1 missing layers. No CLAUDE.md.
- Create: `testdata/go-hexagonal/inconsistent/` — mixed naming (camelCase + snake_case files), different error handling, mixed structure.
- Create: `testdata/go-hexagonal/empty/` — just `go.mod` + `cmd/main.go`. No modules, no tests.

**Step 1: Create all 3 fixtures**

**Step 2: Write ordering tests**

```go
func TestScore_PerfectHigherThanIncomplete(t *testing.T) { ... }
func TestScore_IncompleteHigherThanEmpty(t *testing.T) { ... }
func TestScore_Deterministic(t *testing.T) { ... }
```

**Step 3: Commit**

```bash
git add -A && git commit -m "feat: test fixtures for incomplete, inconsistent, and empty projects"
```

---

## Task 12: Convention Coverage Scorer (20% weight)

**Files:**
- Create: `internal/engine/scoring/conventions.go`
- Test: `internal/engine/scoring/conventions_test.go`
- Modify: `internal/engine/scoring/scorer.go` — register scorer

Sub-metrics: naming consistency (30), error handling `%w` (25), import ordering (15), file organization (15), code style (15).

Tests against all 4 fixtures. Register in orchestrator. Smoke test shows 3 categories.

**Commit:** `git commit -m "feat: convention coverage scorer"`

---

## Task 13: Pattern Compliance Scorer (20% weight)

**Files:**
- Create: `internal/engine/scoring/patterns.go`
- Test: `internal/engine/scoring/patterns_test.go`
- Modify: `internal/engine/scoring/scorer.go` — register scorer

Auto-detects patterns: methods/structures appearing in >50% of modules. Measures compliance rate across the rest. Example: `Validate() error` in 2/3 entities → 1 flagged.

**Commit:** `git commit -m "feat: pattern compliance scorer with auto-detection"`

---

## Task 14: AI Context Quality Scorer (10% weight)

**Files:**
- Create: `internal/engine/scoring/ai_context.go`
- Test: `internal/engine/scoring/ai_context_test.go`
- Modify: `internal/engine/scoring/scorer.go` — register scorer

Checks: CLAUDE.md exists + >500 bytes + has headers (25), .cursorrules exists + >200 bytes (25), AGENTS.md (25), .openkraft/ manifest (25).

**Commit:** `git commit -m "feat: AI context quality scorer"`

---

## Task 15: Module Completeness Scorer (10% weight)

**Files:**
- Create: `internal/engine/scoring/completeness.go`
- Test: `internal/engine/scoring/completeness_test.go`
- Modify: `internal/engine/scoring/scorer.go` — register scorer

Proto-golden detection: module with most files + layers = baseline. Others scored as % of that. Sub-metrics: file completeness avg (40), structural completeness avg (30), documentation (30).

**Commit:** `git commit -m "feat: module completeness scorer with proto-golden detection"`

---

> ### MILESTONE 2: Full 6-category scoring
> `openkraft score` produces the complete Lighthouse-style output from the design doc.

---

## Task 16: Golden Module Selector

**Files:**
- Create: `internal/golden/selector.go`
- Test: `internal/golden/selector_test.go`

Implements the golden score algorithm from design doc Section 9:

```
Golden Score = (
    file_completeness × 0.30 +
    structural_depth  × 0.25 +
    test_coverage     × 0.20 +
    pattern_compliance× 0.15 +
    documentation     × 0.10
)
```

1. Score each module
2. Rank by golden score
3. Return top candidate with score breakdown

**Step 1: Write test** — Tax should rank #1 in perfect fixture (most complete). Payments should rank last.

**Step 2: Implement** — `SelectGolden(modules []DetectedModule, analyzed map) (*GoldenModule, error)`

**Step 3: Run tests, commit**

```bash
git add -A && git commit -m "feat: golden module auto-detection and ranking"
```

---

## Task 17: Blueprint Extractor

**Files:**
- Create: `internal/golden/extractor.go`
- Create: `pkg/types/blueprint.go`
- Test: `internal/golden/extractor_test.go`

Extracts a blueprint from the golden module: expected file structure, structural requirements (which structs/methods/interfaces each file type should have), required patterns.

`pkg/types/blueprint.go`:
```go
package types

type Blueprint struct {
	Name          string          `json:"name"`
	ExtractedFrom string          `json:"extracted_from"`
	Files         []BlueprintFile `json:"files"`
	ExternalFiles []BlueprintFile `json:"external_files,omitempty"`
	Patterns      []string        `json:"patterns,omitempty"`
}

type BlueprintFile struct {
	PathPattern    string   `json:"path_pattern"`    // e.g. "domain/{entity}.go"
	Type           string   `json:"type"`            // "domain_entity", "service", etc.
	Required       bool     `json:"required"`
	RequiredStructs   []string `json:"required_structs,omitempty"`   // e.g. ["{Entity}"]
	RequiredFunctions []string `json:"required_functions,omitempty"` // e.g. ["New{Entity}"]
	RequiredMethods   []string `json:"required_methods,omitempty"`   // e.g. ["Validate() error"]
	RequiredInterfaces []string `json:"required_interfaces,omitempty"`
}
```

**Step 1: Write test** — Extract blueprint from tax module. Verify it captures domain entity file with TaxRule struct, NewTaxRule constructor, Validate method. Verify it captures all layers.

**Step 2: Implement** — Walks golden module's analyzed files, groups by layer, extracts structural patterns, generalizes names (TaxRule → {Entity}, tax → {module}).

**Step 3: Run tests, commit**

```bash
git add -A && git commit -m "feat: blueprint extractor from golden module"
```

---

## Task 18: Completeness Comparator — File Level

**Files:**
- Create: `internal/engine/completeness/checker.go`
- Create: `internal/engine/completeness/file_manifest.go`
- Test: `internal/engine/completeness/checker_test.go`

Level 1 + Level 2 comparison from design doc Section 6:

**Level 1 (File Manifest):** Golden has `domain/tax_rule.go` → target should have `domain/payment.go`. Maps entity name, preserves suffixes (_test, _errors, _status).

**Level 2 (Structural):** Golden's `tax_rule.go` has `TaxRule` struct + `NewTaxRule` + `Validate()` → target's `payment.go` should have `Payment` struct + `NewPayment` + `Validate()`.

Returns `CheckReport`:
```go
type CheckReport struct {
	Module         string         `json:"module"`
	GoldenModule   string         `json:"golden_module"`
	Score          int            `json:"score"`
	MissingFiles   []MissingFile  `json:"missing_files"`
	MissingStructs []MissingItem  `json:"missing_structs"`
	MissingMethods []MissingItem  `json:"missing_methods"`
	MissingInterfaces []MissingItem `json:"missing_interfaces"`
	Issues         []types.Issue  `json:"issues"`
}
```

**Step 1: Write test** — Check payments module against tax (golden). Verify reports missing test file, missing Validate method, missing application layer, missing adapters.

**Step 2: Implement**

**Step 3: Run tests, commit**

```bash
git add -A && git commit -m "feat: completeness checker with file manifest and structural comparison"
```

---

## Task 19: Pattern Compliance Checker

**Files:**
- Create: `internal/engine/completeness/patterns.go`
- Test: `internal/engine/completeness/patterns_test.go`

Level 3 from design doc. Checks auto-detected patterns across target module:
- Does the repository have `getQuerier`?
- Does the entity have `Validate()`?
- Does the service have constructor injection?

Uses the same auto-detection from the pattern scorer (Task 13), but applies per-module instead of aggregate.

**Step 1: Write test** — Payments module should report missing getQuerier (no repository), missing Validate.

**Step 2: Implement**

**Step 3: Run tests, commit**

```bash
git add -A && git commit -m "feat: pattern compliance checker for check command"
```

---

## Task 20: Check Report + TUI

**Files:**
- Create: `internal/tui/check_renderer.go`
- Test: `internal/tui/check_renderer_test.go`

Renders the check report beautifully:

```
  openkraft check payments

  ╔══════════════════════════════════════════════╗
  ║  Module: payments — Score: 23/100            ║
  ║  Golden module: tax (94/100)                 ║
  ╚══════════════════════════════════════════════╝

  MISSING FILES (7):
  ✗ domain/payment_test.go          (expected: unit tests for entity)
  ✗ domain/payment_errors.go        (expected: error definitions)
  ✗ application/payment_service.go  (expected: application service)
  ✗ application/payment_ports.go    (expected: port interfaces)
  ✗ adapters/http/payment_handler.go
  ✗ adapters/http/payment_routes.go
  ✗ adapters/repository/payment_repository.go

  MISSING STRUCTURES (3):
  ⚠ domain/payment.go — Missing: Validate() error method
  ⚠ domain/payment.go — Missing: NewPayment constructor
  ⚠ No port interfaces defined

  PATTERN VIOLATIONS (1):
  ⚠ No repository with getQuerier pattern

  Run with an AI agent (Claude Code + MCP) to fix these issues automatically.
```

**Step 1: Write test** — Verify renderer output contains missing file names and structure warnings.

**Step 2: Implement**

**Step 3: Run tests, commit**

```bash
git add -A && git commit -m "feat: check report TUI renderer"
```

---

## Task 21: `openkraft check` CLI Command

**Files:**
- Create: `internal/cli/check.go`
- Test: `internal/cli/check_test.go`
- Modify: `internal/cli/root.go` — add `newCheckCmd()`

Flags: `--all` (check all modules), `--json`, `--ci --min N`

```bash
openkraft check payments          # Check one module
openkraft check --all             # Check all modules
openkraft check payments --json   # JSON output for programmatic use
openkraft check --all --ci --min 70  # CI gate
```

**Step 1: Write CLI test**

```go
func TestCheckCommand_SingleModule(t *testing.T) {
	cmd := cli.NewRootCmdForTest()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetArgs([]string{"check", "payments", "--path", "../../testdata/go-hexagonal/perfect"})
	require.NoError(t, cmd.Execute())
	assert.Contains(t, buf.String(), "MISSING")
}

func TestCheckCommand_JSON(t *testing.T) {
	cmd := cli.NewRootCmdForTest()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetArgs([]string{"check", "payments", "--path", "../../testdata/go-hexagonal/perfect", "--json"})
	require.NoError(t, cmd.Execute())
	assert.Contains(t, buf.String(), `"missing_files"`)
}
```

**Step 2: Implement** — Wires golden selector + blueprint extractor + completeness checker + renderer.

**Step 3: Manual smoke test**

```bash
make build
./bin/openkraft check payments --path ./testdata/go-hexagonal/perfect
./bin/openkraft check --all --path ./testdata/go-hexagonal/perfect
./bin/openkraft check payments --path ./testdata/go-hexagonal/perfect --json
```

**Step 4: Commit**

```bash
git add -A && git commit -m "feat: openkraft check command with golden module comparison"
```

---

> ### MILESTONE 3: Working `openkraft check`
> Full diagnostic + prescription. `score` tells you the number, `check` tells you exactly what's missing.

---

## Task 22: MCP Server Foundation

**Files:**
- Create: `internal/mcp/server.go`
- Test: `internal/mcp/server_test.go`

Sets up the MCP server using `mark3labs/mcp-go`. Stdio transport. Registers tool and resource capabilities.

```go
package mcp

import (
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

func NewOpenKraftMCPServer(projectPath string) *server.MCPServer {
	s := server.NewMCPServer(
		"openkraft",
		"0.1.0",
		server.WithToolCapabilities(true),
		server.WithResourceCapabilities(true, false),
	)

	registerTools(s, projectPath)
	registerResources(s, projectPath)

	return s
}
```

**Step 1: Write test** — Verify server creates without error, has tools registered.

**Step 2: Implement server skeleton**

**Step 3: Install dependency**

```bash
go get github.com/mark3labs/mcp-go@latest
go mod tidy
```

**Step 4: Run tests, commit**

```bash
git add -A && git commit -m "feat: MCP server foundation with mark3labs/mcp-go"
```

---

## Task 23: MCP Tools

**Files:**
- Create: `internal/mcp/tools.go`
- Test: `internal/mcp/tools_test.go`

Register 6 MCP tools that AI agents can call:

| Tool | Input | Output |
|------|-------|--------|
| `openkraft_score` | `{path?: string}` | Full score JSON |
| `openkraft_check_module` | `{module: string, path?: string}` | Check report JSON |
| `openkraft_get_blueprint` | `{path?: string}` | Blueprint JSON from golden module |
| `openkraft_get_golden_example` | `{file_type: string, path?: string}` | Source code of golden module's file of that type |
| `openkraft_get_conventions` | `{path?: string}` | Detected conventions (naming, error handling, etc.) |
| `openkraft_check_file` | `{file: string, path?: string}` | Issues for a single file |

**Step 1: Write test for each tool**

```go
func TestMCPTool_Score(t *testing.T) {
	s := mcp.NewOpenKraftMCPServer("../../testdata/go-hexagonal/perfect")
	result, err := callTool(s, "openkraft_score", map[string]any{})
	require.NoError(t, err)
	assert.Contains(t, result, "overall")
}

func TestMCPTool_CheckModule(t *testing.T) {
	s := mcp.NewOpenKraftMCPServer("../../testdata/go-hexagonal/perfect")
	result, err := callTool(s, "openkraft_check_module", map[string]any{"module": "payments"})
	require.NoError(t, err)
	assert.Contains(t, result, "missing_files")
}

func TestMCPTool_GetBlueprint(t *testing.T) {
	s := mcp.NewOpenKraftMCPServer("../../testdata/go-hexagonal/perfect")
	result, err := callTool(s, "openkraft_get_blueprint", map[string]any{})
	require.NoError(t, err)
	assert.Contains(t, result, "files")
}

func TestMCPTool_GetGoldenExample(t *testing.T) {
	s := mcp.NewOpenKraftMCPServer("../../testdata/go-hexagonal/perfect")
	result, err := callTool(s, "openkraft_get_golden_example", map[string]any{"file_type": "domain_entity"})
	require.NoError(t, err)
	assert.Contains(t, result, "TaxRule") // returns actual golden source
}
```

**Step 2: Implement each tool handler**

Each handler reuses the existing scoring/check engines — no new logic, just wiring.

`openkraft_get_golden_example` is the killer feature for AI agents: it returns the **actual source code** of the golden module's file, so the agent sees exactly how to implement things.

**Step 3: Run tests, commit**

```bash
git add -A && git commit -m "feat: MCP tools for score, check, blueprint, and golden examples"
```

---

## Task 24: MCP Resources

**Files:**
- Create: `internal/mcp/resources.go`
- Test: `internal/mcp/resources_test.go`

Register MCP resources (read-only data AI agents can access):

| Resource | URI | Description |
|----------|-----|-------------|
| Score report | `openkraft://score` | Current project score |
| Module blueprint | `openkraft://blueprint` | Extracted blueprint |
| Module status | `openkraft://modules/{name}` | Per-module completeness |
| Conventions | `openkraft://conventions` | Project conventions |

Resources complement tools: tools are for actions ("check this module"), resources are for context ("what are the project conventions?").

**Step 1: Write test**

**Step 2: Implement resource handlers**

**Step 3: Run tests, commit**

```bash
git add -A && git commit -m "feat: MCP resources for score, blueprint, modules, and conventions"
```

---

## Task 25: `openkraft mcp serve` CLI Command

**Files:**
- Create: `internal/cli/mcp.go`
- Test: `internal/cli/mcp_test.go`
- Modify: `internal/cli/root.go` — add `newMCPCmd()`

```bash
openkraft mcp serve                    # Start stdio MCP server
openkraft mcp serve --path /project    # Specify project path
```

The user configures it in Claude Code's MCP settings:

```json
{
  "mcpServers": {
    "openkraft": {
      "command": "openkraft",
      "args": ["mcp", "serve"]
    }
  }
}
```

**Step 1: Write test** — Verify command creates server and runs (with timeout).

**Step 2: Implement**

```go
func newMCPCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "mcp",
		Short: "MCP server commands",
	}
	cmd.AddCommand(newMCPServeCmd())
	return cmd
}

func newMCPServeCmd() *cobra.Command {
	var projectPath string
	cmd := &cobra.Command{
		Use:   "serve",
		Short: "Start OpenKraft MCP server (stdio)",
		RunE: func(cmd *cobra.Command, args []string) error {
			if projectPath == "" {
				projectPath = "."
			}
			s := mcp.NewOpenKraftMCPServer(projectPath)
			return server.ServeStdio(s)
		},
	}
	cmd.Flags().StringVar(&projectPath, "path", "", "Project path (defaults to cwd)")
	return cmd
}
```

**Step 3: Manual integration test**

```bash
make build
echo '{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"test","version":"1.0"}}}' | ./bin/openkraft mcp serve --path ./testdata/go-hexagonal/perfect
```

Should return MCP initialize response with tool/resource capabilities.

**Step 4: Commit**

```bash
git add -A && git commit -m "feat: openkraft mcp serve command for AI agent integration"
```

---

> ### MILESTONE 4: Working MCP server
> The complete value loop works: score → check → AI agent fixes via MCP → score again.

---

## Task 26: Git Analyzer + Score History

**Files:**
- Create: `internal/analysis/git_analyzer.go`
- Create: `internal/storage/history.go`
- Test: `internal/analysis/git_analyzer_test.go`
- Test: `internal/storage/history_test.go`
- Modify: `internal/cli/score.go` — add `--history` flag

Git analyzer (go-git/v5): extracts commit hash, detects if git repo. Graceful degradation if not.

Score history: stores scores in `.openkraft/history/scores.json`. `--history` shows evolution:
```
  Score History:
  2026-02-25  abc1234  47/100  D
  2026-02-26  def5678  62/100  C  ↑15
```

**Step 1: Write tests for both**

**Step 2: Implement both**

**Step 3: Add `--history` flag, commit**

```bash
git add -A && git commit -m "feat: git analyzer and score history tracking"
```

---

## Task 27: End-to-End Tests

**Files:**
- Create: `tests/e2e/score_test.go`
- Create: `tests/e2e/check_test.go`

Full E2E tests that build the binary and run it against fixtures:

```go
func TestE2E_Score(t *testing.T)              { /* verify output, exit code */ }
func TestE2E_ScoreJSON(t *testing.T)          { /* verify valid JSON with 6 categories */ }
func TestE2E_ScoreCI(t *testing.T)            { /* verify exit 1 when below min */ }
func TestE2E_ScoreOrdering(t *testing.T)      { /* perfect > incomplete > empty */ }
func TestE2E_CheckModule(t *testing.T)        { /* verify missing files reported */ }
func TestE2E_CheckAll(t *testing.T)           { /* verify all modules checked */ }
func TestE2E_CheckJSON(t *testing.T)          { /* verify valid JSON check report */ }
```

**Step 1: Write E2E tests**

**Step 2: Run and verify**

```bash
make build && go test ./tests/e2e/... -v
```

**Step 3: Commit**

```bash
git add -A && git commit -m "test: end-to-end tests for score, check, and MCP"
```

---

## Task 28: README and Documentation

**Files:**
- Create: `README.md`

Write the README with:
- Tagline: "Stop shipping 80% code."
- One-line description
- Install: `go install github.com/openkraft/openkraft/cmd/openkraft@latest`
- Quick start: `openkraft score .` and `openkraft check payments`
- Example TUI output for both score and check
- MCP setup instructions (Claude Code config JSON)
- Badge example
- CI integration (GitHub Actions snippet)
- How it works (score → check → MCP → AI agent fixes)
- Contributing + License (MIT)

**Commit:**

```bash
git add -A && git commit -m "docs: README with score, check, and MCP usage"
```

---

## Verification

After all 28 tasks:

1. **Build:** `make build` → `bin/openkraft`
2. **Tests:** `make test` → all pass
3. **Lint:** `make lint` → clean
4. **Score smoke test:**
   ```bash
   ./bin/openkraft score ./testdata/go-hexagonal/perfect
   ./bin/openkraft score ./testdata/go-hexagonal/perfect --json | jq .
   ./bin/openkraft score ./testdata/go-hexagonal/perfect --badge
   ./bin/openkraft score ./testdata/go-hexagonal/perfect --ci --min 50
   ```
5. **Check smoke test:**
   ```bash
   ./bin/openkraft check payments --path ./testdata/go-hexagonal/perfect
   ./bin/openkraft check --all --path ./testdata/go-hexagonal/perfect
   ./bin/openkraft check payments --path ./testdata/go-hexagonal/perfect --json
   ```
6. **MCP smoke test:**
   ```bash
   echo '<initialize request>' | ./bin/openkraft mcp serve --path ./testdata/go-hexagonal/perfect
   ```
7. **Score determinism:** same project = same score
8. **Score ordering:** perfect > incomplete > empty
9. **Real project test:** run against a real Go project outside testdata

---

## File Tree (Phase 1 Final State)

```
openkraft/
├── cmd/openkraft/main.go
├── internal/
│   ├── cli/
│   │   ├── root.go
│   │   ├── version.go
│   │   ├── score.go
│   │   ├── check.go
│   │   └── mcp.go
│   ├── engine/
│   │   ├── scoring/
│   │   │   ├── scorer.go            # Orchestrator
│   │   │   ├── architecture.go      # 25%
│   │   │   ├── conventions.go       # 20%
│   │   │   ├── patterns.go          # 20%
│   │   │   ├── tests.go             # 15%
│   │   │   ├── ai_context.go        # 10%
│   │   │   └── completeness.go      # 10%
│   │   └── completeness/
│   │       ├── checker.go           # Completeness comparison engine
│   │       ├── file_manifest.go     # File-level comparison
│   │       └── patterns.go          # Pattern compliance checking
│   ├── analysis/
│   │   ├── file_scanner.go
│   │   ├── module_detector.go
│   │   ├── go_analyzer.go
│   │   └── git_analyzer.go
│   ├── golden/
│   │   ├── selector.go              # Auto-detection + ranking
│   │   └── extractor.go             # Blueprint extraction
│   ├── mcp/
│   │   ├── server.go                # MCP server setup
│   │   ├── tools.go                 # 6 MCP tools
│   │   └── resources.go             # 4 MCP resources
│   ├── storage/
│   │   └── history.go
│   └── tui/
│       ├── renderer.go              # Score rendering
│       ├── check_renderer.go        # Check report rendering
│       └── colors.go
├── pkg/types/
│   ├── score.go
│   ├── module.go
│   ├── issue.go
│   └── blueprint.go
├── testdata/go-hexagonal/
│   ├── perfect/
│   ├── incomplete/
│   ├── inconsistent/
│   └── empty/
├── tests/e2e/
│   ├── score_test.go
│   └── check_test.go
├── docs/plans/
│   ├── 2026-02-23-openkraft-design.md
│   └── 2026-02-24-phase1-mvp-plan.md
├── .golangci.yml
├── .gitignore
├── Makefile
├── go.mod
├── go.sum
└── README.md
```

---

## Post-MVP: Validation Phase

After shipping, before expanding:

1. Run `openkraft score` + `openkraft check` on 15+ real Go repos
2. Document false positives and false negatives
3. Calibrate scoring weights (may need 2-3 rounds)
4. Get 5+ external users to try it
5. Collect qualitative feedback: "does the check output match reality?"
6. **GO decision:** scoring is useful, check finds real gaps → proceed to Phase 2
7. **KILL decision:** scoring is noise after 3 calibrations → pivot or abandon

## Phase 2 (only after validation): `openkraft init`
- Generate CLAUDE.md, .cursorrules, AGENTS.md from analysis
- Generate .openkraft/manifest.yaml
- This is additive — uses the same analysis + golden module engine
