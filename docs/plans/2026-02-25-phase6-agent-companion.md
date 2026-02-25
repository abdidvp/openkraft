# Phase 6: Agent Companion — Onboard, Fix, Validate

**Goal:** Transform openkraft from a passive scoring tool into a real-time companion for AI coding agents. Agents consult openkraft in real-time to write correct code from the first attempt.

**Architecture:** Three new capabilities layered on the existing scoring infrastructure: an `onboard` command for CLAUDE.md generation, a `fix` command for hybrid auto-repair, and a `validate` command for incremental file-level checking. Same hexagonal pattern. MCP-first design.

**Tech Stack:** Go 1.24, existing stack (Cobra, mark3labs/mcp-go, yaml.v3, lipgloss, testify). No new dependencies. No LLM dependency -- all analysis is deterministic AST + heuristics. Cache via JSON files in `.openkraft/cache/`.

**Prerequisite:** Phase 5 (config-aware scoring with profiles) complete and all tests passing.

---

## Problem Analysis

Today openkraft answers one question: "How AI-ready is this codebase?" That produces a score. But a score alone does not help an AI agent write better code. The agent needs three things openkraft does not yet provide:

| What the agent needs | Current state | Phase 6 solution |
|---|---|---|
| Project context before writing code | Agent reads files manually, misses conventions | `openkraft onboard` generates a CLAUDE.md from real analysis |
| Fix guidance after scoring | Score report lists issues but agent must interpret them | `openkraft fix` applies safe fixes and returns structured instructions |
| Fast feedback during coding | Full project re-scan on every change (slow, seconds) | `openkraft validate` does incremental checks in <500ms |

The agent workflow today is: guess context, write code, run full scan, read report, interpret issues, try again. The target workflow is: get context, write code, validate incrementally, get exact fix instructions, iterate until pass.

---

## Design

### Capability 1: `openkraft onboard` -- Auto-generate CLAUDE.md

Analyzes the real codebase and generates a CLAUDE.md that reflects actual structure, patterns, and conventions. Not a generic template -- every line comes from codebase analysis.

#### What It Detects

| Detection | Method | Output in CLAUDE.md |
|---|---|---|
| Architecture layout | Directory structure analysis | "Hexagonal with cross-cutting layers" |
| Module structure | Scan `internal/` subdirectories via existing `ModuleDetector` | Module list with paths and layers |
| Naming conventions | Classify files as bare vs suffixed via existing `scoring.ScoreDiscoverability` logic | "Files use bare naming (scanner.go, not scanner_service.go)" |
| Golden module | Highest-scoring module via existing `golden.SelectGolden` | "Follow `internal/domain/scoring` as the canonical example" |
| Build/test commands | Parse Makefile, go.mod | `go test ./...`, `make build` |
| Dependency rules | Import graph analysis between layers | "domain/ has zero imports from adapters/" |
| Key interfaces | AST analysis of interface declarations and satisfying types | Port-to-implementation mapping table |

Also generates `.cursorrules` and `AGENTS.md` variants from the same analysis.

#### CLI Interface

```
openkraft onboard [path]
```

Flags:
- `--force` -- overwrite existing CLAUDE.md
- `--format md|json` -- output format (default: md)
- `--append` -- add to existing CLAUDE.md instead of replacing

Exit codes: `0` success, `1` error.

#### MCP Tool

```
openkraft_onboard(path: string, format?: "md" | "json") -> string
```

Returns the generated content directly for the agent to consume. The agent reads this before writing any code.

#### Generated CLAUDE.md Format

Target: under 200 lines. Every line is actionable. Empty sections are omitted entirely.

```markdown
# Project: {name}

{one-line description from go.mod or package doc}

## Architecture

{project_type} using {architecture_style} with {layout_style} layout.
Golden module: `{path}` -- follow this as the canonical example.

## Conventions

- Naming: {bare|suffixed} -- e.g., `scanner.go` not `scanner_service.go`
- Files: max {n} lines, functions: max {n} lines, params: max {n}
- Tests: `{pattern}` alongside source files

## Module Structure

| Module | Path | Layers |
{detected modules}

## Dependency Rules

- domain/ has zero imports from application/ or adapters/
- application/ imports domain/ only

## Key Interfaces

| Port | Implementation | File |
{detected interface -> implementation mappings}

## Build & Test

{detected build and test commands}
```

---

### Capability 2: `openkraft fix` -- Hybrid Auto-Fix

Takes scoring results and applies fixes. Safe fixes applied directly. Complex fixes returned as structured instructions for the agent to execute.

#### Safe Auto-Fixes (Applied Directly)

| Fix | What it does |
|---|---|
| Generate CLAUDE.md | Calls `onboard` internally |
| Add package doc comments | Inserts `// Package xxx ...` to all packages missing one |
| Create test stubs | Generates missing `_test.go` files with package declaration and one placeholder test |
| Generate .cursorrules | Writes detected conventions in Cursor-compatible format |
| Create AGENTS.md skeleton | Writes a structured AGENTS.md from detected project metadata |
| Add .golangci.yml | Generates a basic linter config matching the project's detected thresholds |

#### Instruction-Only Fixes (Returned as Structured Output)

| Fix | What it returns |
|---|---|
| Split long functions | Suggests split points with function names |
| Reduce parameter count | Suggests grouping parameters into structs |
| Fix dependency violations | Suggests correct import paths |
| Add missing module files | Provides file list based on golden module blueprint |

#### Implementation

```go
// internal/application/fix_service.go
type FixService struct {
    scoreService   *ScoreService
    onboardService *OnboardService
    fileWriter     domain.FileWriter
}

type FixOptions struct {
    DryRun   bool
    AutoOnly bool
    Category string
}

func (s *FixService) PlanFixes(projectPath string, opts FixOptions) (*domain.FixPlan, error) {
    // 1. Score the project
    score, err := s.scoreService.ScoreProject(projectPath)
    if err != nil {
        return nil, fmt.Errorf("scoring project: %w", err)
    }

    plan := &domain.FixPlan{ScoreBefore: score.Overall}

    // 2. Identify safe fixes from score issues
    plan.Applied = s.identifySafeFixes(projectPath, score, opts)

    // 3. Generate instructions for complex fixes
    if !opts.AutoOnly {
        plan.Instructions = s.generateInstructions(score, opts)
    }

    // 4. Apply safe fixes (unless dry run)
    if !opts.DryRun {
        if err := s.applyFixes(projectPath, plan); err != nil {
            return nil, fmt.Errorf("applying fixes: %w", err)
        }
        // Re-score to get the after score
        afterScore, _ := s.scoreService.ScoreProject(projectPath)
        if afterScore != nil {
            plan.ScoreAfter = afterScore.Overall
        }
    }

    return plan, nil
}
```

#### Output Format

```json
{
  "applied": [
    {"type": "create_file", "path": "CLAUDE.md", "description": "Generated from codebase analysis"},
    {"type": "create_file", "path": "internal/payments/service_test.go", "description": "Test stub"}
  ],
  "instructions": [
    {
      "type": "refactor",
      "file": "internal/x/service.go",
      "line": 45,
      "message": "Split processOrder (82 lines) into processOrder + validateOrderItems",
      "priority": "high"
    }
  ],
  "score_before": 78,
  "score_after": 83
}
```

#### CLI Interface

```
openkraft fix [path]
```

Flags:
- `--dry-run` -- show what would change without applying
- `--auto-only` -- only apply safe auto-fixes, skip instruction generation
- `--category <name>` -- fix only issues in a specific scoring category

Exit codes: `0` success, `1` error.

#### MCP Tool

```
openkraft_fix(path: string, dry_run?: bool, category?: string) -> FixPlan
```

Agent calls this after scoring to get a complete fix plan as JSON.

---

### Capability 3: `openkraft validate <files>` -- Incremental Validation

Scores only the changed files, not the whole project. Uses a cached baseline scan for performance (<500ms target). Returns exact score delta per category.

#### How It Works

1. On first call, run a full project scan and cache the result in `.openkraft/cache/`.
2. On subsequent calls, re-analyze only the specified changed files.
3. Merge updated file data into the cached baseline.
4. Re-run only the affected scorers.
5. Return the exact score delta compared to the cached baseline.

#### Which Scorers Re-Run

| Changed file type | Scorers affected |
|---|---|
| `*.go` source file | code_health, structure, discoverability, predictability |
| `*_test.go` file | verifiability |
| `CLAUDE.md`, `AGENTS.md`, `.cursorrules` | context_quality |

#### Implementation

```go
// internal/application/validate_service.go
type ValidateService struct {
    scanner      domain.ProjectScanner
    analyzer     domain.CodeAnalyzer
    detector     domain.ModuleDetector
    scoreService *ScoreService
    cache        domain.CacheStore
    configLoader domain.ConfigLoader
}

func (s *ValidateService) Validate(projectPath string, files []string, strict bool) (*domain.ValidationResult, error) {
    // 1. Load or create cache
    cached, err := s.cache.Load(projectPath)
    if err != nil || cached == nil || cached.IsStale() {
        cached, err = s.createCache(projectPath)
        if err != nil {
            return nil, fmt.Errorf("creating cache: %w", err)
        }
    }

    // 2. Re-analyze only changed files
    for _, f := range files {
        af, err := s.analyzer.AnalyzeFile(filepath.Join(projectPath, f))
        if err != nil {
            continue
        }
        af.Path = f
        cached.AnalyzedFiles[f] = af
    }

    // 3. Re-run affected scorers with merged data
    newScore := s.scoreWithMergedData(cached, files)

    // 4. Compute delta
    result := computeDelta(cached.BaselineScore, newScore, files, strict)
    return result, nil
}
```

#### Response Format

```json
{
  "status": "pass",
  "issues": [
    {
      "file": "internal/payments/service.go",
      "line": 23,
      "severity": "warning",
      "message": "Function processPayment has 5 parameters (max: 4)",
      "category": "code_health"
    }
  ],
  "score_impact": {
    "overall": 2,
    "categories": {"code_health": 3, "structure": -1}
  },
  "suggestions": ["Consider adding a test file for service.go"]
}
```

Fail conditions:
- Any new issue with severity `"error"` -> status `"fail"`
- Overall score drops below configured `min_threshold` -> status `"fail"`
- New warnings only -> status `"warn"`
- No new issues -> status `"pass"`

#### CLI Interface

```
openkraft validate <file1> <file2> ...
```

Flags:
- `--strict` -- fail on any warning (not just errors)
- `--no-cache` -- force full re-scan

Exit codes: `0` pass, `1` fail, `2` warn (unless `--strict`, then warn is also `1`).

#### MCP Tool

```
openkraft_validate(path: string, files: string, strict?: bool) -> ValidationResult
```

This is the primary tool agents call after each change. The `files` parameter is a comma-separated list of changed file paths relative to the project root.

---

## Agent Workflow

```
Agent receives task
  -> calls openkraft_onboard (understands project)
  -> writes code
  -> calls openkraft_validate (checks work)
  -> if issues: calls openkraft_fix (gets corrections)
  -> repeat until pass
  -> opens PR
```

---

## Architecture Changes

### New Domain Types

```go
// internal/domain/onboard.go
type OnboardReport struct {
    ProjectType       string             `json:"project_type"`
    ArchitectureStyle string             `json:"architecture_style"`
    LayoutStyle       string             `json:"layout_style"`
    Modules           []ModuleInfo       `json:"modules"`
    NamingConvention  string             `json:"naming_convention"`
    GoldenModule      string             `json:"golden_module"`
    BuildCommands     []string           `json:"build_commands"`
    TestCommands      []string           `json:"test_commands"`
    DependencyRules   []DependencyRule   `json:"dependency_rules"`
    FilePatterns      []FilePattern      `json:"file_patterns"`
    Interfaces        []InterfaceMapping `json:"interfaces"`
}

// internal/domain/fix.go
type FixPlan struct {
    Applied      []AppliedFix  `json:"applied"`
    Instructions []Instruction `json:"instructions"`
    ScoreBefore  int           `json:"score_before"`
    ScoreAfter   int           `json:"score_after"`
}

// internal/domain/validate.go
type ValidationResult struct {
    Status      string            `json:"status"`
    Issues      []ValidationIssue `json:"issues"`
    ScoreImpact ScoreImpact       `json:"score_impact"`
    Suggestions []string          `json:"suggestions"`
}

// internal/domain/cache.go
type ProjectCache struct {
    Timestamp     time.Time                  `json:"timestamp"`
    ProjectPath   string                     `json:"project_path"`
    ScanResult    *ScanResult                `json:"scan_result"`
    AnalyzedFiles map[string]*AnalyzedFile   `json:"analyzed_files"`
    BaselineScore *Score                     `json:"baseline_score"`
}
```

### New Ports

```go
// internal/domain/ports.go (additions)

type FileWriter interface {
    CreateFile(path string, content []byte) error
    AppendToFile(path string, content []byte) error
}

type CacheStore interface {
    Load(projectPath string) (*ProjectCache, error)
    Save(cache *ProjectCache) error
    Invalidate(projectPath string) error
}
```

### New Application Services

```
internal/application/
  onboard_service.go       # Orchestrates codebase analysis and CLAUDE.md rendering
  onboard_service_test.go
  fix_service.go           # Identifies safe fixes and generates instructions
  fix_service_test.go
  validate_service.go      # Cache management and incremental score delta
  validate_service_test.go
```

### New Adapters

```
internal/adapters/
  outbound/
    cache/
      cache.go             # CacheStore adapter using .openkraft/cache/ JSON files
      cache_test.go
    filewriter/
      writer.go            # FileWriter adapter for applying safe fixes
      writer_test.go
  inbound/
    cli/
      onboard.go           # openkraft onboard command
      fix.go               # openkraft fix command
      validate.go          # openkraft validate command
    mcp/
      tools.go             # New tools: openkraft_onboard, openkraft_fix, openkraft_validate
```

---

## Task Breakdown

### Milestone Map

```
Task  1:  Domain types (OnboardReport, FixPlan, ValidationResult, ProjectCache)
Task  2:  Domain ports (FileWriter, CacheStore)
Task  3:  CacheStore outbound adapter
Task  4:  FileWriter outbound adapter
Task  5:  OnboardService -- codebase analysis and pattern detection
Task  6:  CLAUDE.md renderer (markdown + JSON output)
Task  7:  FixService -- safe fix identification and application
Task  8:  FixService -- instruction generation for complex fixes
Task  9:  ValidateService -- cache management and incremental re-analysis
Task 10:  ValidateService -- score delta computation
Task 11:  CLI commands: onboard, fix, validate
Task 12:  MCP tools: openkraft_onboard, openkraft_fix, openkraft_validate
Task 13:  Integration tests
Task 14:  E2E tests
Task 15:  Final verification
```

### Task Dependency Graph

```
       [1] Domain types
       / \
     [2]  |
   Ports  |
    / \   |
  [3] [4] |
Cache Writer
   \  |  /
    \ | /
  [5] OnboardService -----> [6] CLAUDE.md renderer
   |                          |
  [7] FixService (safe) <----+
   |
  [8] FixService (instructions)
   |
  [9] ValidateService (cache) ---> [10] ValidateService (delta)
   |                                 |
   +------+------+------------------+
   |      |
 [11]   [12]
 CLI    MCP
 cmds   tools
   |      |
   +------+
      |
    [13] Integration tests
      |
    [14] E2E tests
      |
    [15] Verification
```

---

## Task 1: Domain Types

**Files:**
- Create: `internal/domain/onboard.go`
- Create: `internal/domain/fix.go`
- Create: `internal/domain/validate.go`
- Create: `internal/domain/cache.go`

Define all structs: `OnboardReport`, `ModuleInfo`, `DependencyRule`, `FilePattern`, `InterfaceMapping`, `FixPlan`, `AppliedFix`, `Instruction`, `ValidationResult`, `ValidationIssue`, `ScoreImpact`, `ProjectCache`.

**Tests:** Verify JSON marshaling/unmarshaling round-trips. Verify `ProjectCache.IsStale()` returns true after 5 minutes.

**Verify:** `go test ./internal/domain/ -run "TestOnboard|TestFix|TestValidat|TestCache" -v -count=1`

---

## Task 2: Domain Ports

**Files:**
- Modify: `internal/domain/ports.go`

Add `FileWriter` and `CacheStore` interfaces.

**Verify:** `go build ./internal/domain/`

---

## Task 3: CacheStore Outbound Adapter

**Files:**
- Create: `internal/adapters/outbound/cache/cache.go`
- Create: `internal/adapters/outbound/cache/cache_test.go`

Cache files stored as JSON in `.openkraft/cache/` with a hash of the project path as the filename. `Load` returns `nil, nil` if no cache exists. `Save` creates the cache directory if needed. `Invalidate` removes the cache file.

**Tests:** Save/load round-trip, load on non-existent cache, invalidation, concurrent save/load safety.

**Verify:** `go test ./internal/adapters/outbound/cache/ -v -count=1`

---

## Task 4: FileWriter Outbound Adapter

**Files:**
- Create: `internal/adapters/outbound/filewriter/writer.go`
- Create: `internal/adapters/outbound/filewriter/writer_test.go`

`CreateFile` writes content, fails if file exists. `AppendToFile` creates if missing. Both create parent directories as needed.

**Tests:** Create, overwrite prevention, append, parent directory creation.

**Verify:** `go test ./internal/adapters/outbound/filewriter/ -v -count=1`

---

## Task 5: OnboardService -- Codebase Analysis

**Files:**
- Create: `internal/application/onboard_service.go`
- Create: `internal/application/onboard_service_test.go`

`GenerateOnboardReport` orchestrates: scan (existing `ProjectScanner`), detect modules (existing `ModuleDetector`), analyze files (existing `CodeAnalyzer`), detect architecture style from layout, detect naming convention (reuse `discoverability` scorer logic), find golden module (reuse `golden.SelectGolden`), detect build/test commands (parse Makefile), analyze import graph for dependency rules, find interface-to-implementation mappings via AST.

**Tests:** Run against `testdata/go-hexagonal/perfect`. Verify detected layers, modules, golden module. Verify naming convention detection.

**Verify:** `go test ./internal/application/ -run TestOnboard -v -count=1`

---

## Task 6: CLAUDE.md Renderer

**Files:**
- Modify: `internal/application/onboard_service.go` -- add `RenderCLAUDEmd` and `RenderJSON`

`RenderCLAUDEmd` produces markdown following the template. Uses `text/template` for clean separation. Output must be under 200 lines. Empty sections omitted entirely. `RenderJSON` marshals to indented JSON.

**Tests:** Render against populated `OnboardReport`, verify markdown structure, verify line count under 200, verify JSON round-trip.

**Verify:** `go test ./internal/application/ -run TestRender -v -count=1`

---

## Task 7: FixService -- Safe Fixes

**Files:**
- Create: `internal/application/fix_service.go`
- Create: `internal/application/fix_service_test.go`

`PlanFixes` runs the scorer, identifies issues, classifies them. Safe fix generators: CLAUDE.md (calls OnboardService), package doc comments (AST), test stubs (find missing `_test.go`), .cursorrules (conventions), AGENTS.md (metadata), .golangci.yml (thresholds). `ApplyFixes` calls `FileWriter` for each applied fix.

**Tests:** Plan fixes for project missing CLAUDE.md. Plan fixes for project missing test files. Apply fixes creates files. Dry run creates no files.

**Verify:** `go test ./internal/application/ -run TestFix -v -count=1`

---

## Task 8: FixService -- Instructions

**Files:**
- Modify: `internal/application/fix_service.go`

Generate structured instructions for complex issues: long functions (AST split points), too many parameters (struct suggestion), dependency violations (correct import path), missing module files (golden module comparison). Each instruction includes file, line, message, and priority.

**Tests:** Instructions for 80-line function. Instructions for 6-parameter function. Priority ordering.

**Verify:** `go test ./internal/application/ -run TestFixInstruction -v -count=1`

---

## Task 9: ValidateService -- Cache Management

**Files:**
- Create: `internal/application/validate_service.go`
- Create: `internal/application/validate_service_test.go`

Cache lifecycle: check cache via `CacheStore.Load`, if stale (>5 min) or missing run full scan and save, re-analyze only specified files, merge into cached baseline, pass to delta computation. Cache invalidated when `.openkraft.yaml` changes.

**Tests:** First call creates cache. Second call uses cache (verify via mock call counts). Cache invalidation forces re-scan.

**Verify:** `go test ./internal/application/ -run TestValidate -v -count=1`

---

## Task 10: ValidateService -- Score Delta

**Files:**
- Modify: `internal/application/validate_service.go`

After merging updated file data: re-run affected scorers (per file-type-to-scorer mapping), compute per-category delta, compute overall delta, classify issues by severity, set status, generate suggestions.

**Tests:** File improving code_health (positive delta). File worsening structure (negative delta). Error-severity issue (status "fail"). Strict mode with warnings (status "fail").

**Verify:** `go test ./internal/application/ -run TestValidateDelta -v -count=1`

---

## Task 11: CLI Commands

**Files:**
- Create: `internal/adapters/inbound/cli/onboard.go`
- Create: `internal/adapters/inbound/cli/fix.go`
- Create: `internal/adapters/inbound/cli/validate.go`
- Modify: `internal/adapters/inbound/cli/root.go` -- register all three commands

Three Cobra commands wiring the corresponding application services. Styled output using lipgloss (consistent with existing `score` command).

`onboard` prints content to stdout and writes file. `fix` shows applied fixes with checkmarks and instructions with priority colors. `validate` shows status, score impact, and suggestions.

**Tests:** Test each command with mock services. Verify flag parsing. Verify exit codes.

**Verify:** `go test ./internal/adapters/inbound/cli/ -run "TestOnboard|TestFixCmd|TestValidateCmd" -v -count=1`

---

## Task 12: MCP Tools

**Files:**
- Modify: `internal/adapters/inbound/mcp/tools.go`

Register three new tools: `openkraft_onboard`, `openkraft_fix`, `openkraft_validate`. Each tool creates its service, calls the handler, and returns JSON (or markdown for onboard with format=md).

**Tests:** Test each tool handler returns valid content for test fixtures.

**Verify:** `go test ./internal/adapters/inbound/mcp/ -run "TestOnboard|TestFix|TestValidate" -v -count=1`

---

## Task 13: Integration Tests

**Files:**
- Create: `internal/application/integration_test.go`

Tests with real scoring (no mocks) on test fixtures:
1. OnboardService against `testdata/go-hexagonal/perfect` -- verify complete OnboardReport
2. FixService against a fixture with known issues -- verify correct fix classification
3. ValidateService incremental check -- verify score delta computation
4. Full workflow: onboard -> fix -> validate -> verify score improvement

**Verify:** `go test ./internal/application/ -run TestIntegration -v -count=1`

---

## Task 14: E2E Tests

**Files:**
- Modify: `tests/e2e/e2e_test.go`

Tests:
1. `openkraft onboard testdata/go-hexagonal/perfect` -- verify CLAUDE.md file created
2. `openkraft onboard --format json` -- verify valid JSON output
3. `openkraft fix --dry-run testdata/go-hexagonal/minimal` -- verify fix plan without file changes
4. `openkraft validate testdata/go-hexagonal/perfect/internal/domain/model.go` -- verify pass status
5. Full agent loop: onboard -> change -> validate -> fix -> validate

**Verify:** `go test ./tests/e2e/ -v -count=1`

---

## Task 15: Final Verification

```bash
go clean -testcache
go test ./... -race -count=1
go build -o ./openkraft ./cmd/openkraft

# Test onboard
./openkraft onboard .
cat CLAUDE.md
wc -l CLAUDE.md  # must be under 200

# Test fix
./openkraft fix --dry-run .

# Test validate
./openkraft validate internal/domain/config.go

# Performance
time ./openkraft onboard .                          # must be <2s
time ./openkraft validate internal/domain/config.go  # must be <500ms (cached)
```

---

## Success Criteria

| Metric | Target |
|---|---|
| `openkraft onboard` latency | <2s for a 50-file project |
| `openkraft validate` latency | <500ms for incremental checks |
| `openkraft fix` correctness | Safe fixes produce no broken files (compile check) |
| CLAUDE.md quality | Under 200 lines, every section populated from analysis |
| Agent workflow | An AI agent using openkraft MCP tools produces higher-quality code than without |
| Backwards compatibility | Existing commands unchanged, new commands are additive |

## Risk Mitigation

| Risk | Mitigation |
|---|---|
| CLAUDE.md generation produces generic content | Every line must trace to a specific detection. No static filler text. |
| Safe fixes break compilation | Run `go build` after applying fixes. Roll back on failure. |
| Cache invalidation misses changes | Conservative staleness (5 min TTL). Invalidate on config changes. Provide `--no-cache`. |
| Incremental validation misses cross-file issues | Track file dependency graph. Re-analyze importers of changed files. |
| MCP tool response too large | Cap CLAUDE.md at 200 lines. Cap fix instructions at 50 per call. |
| Performance regression on large codebases | Profile with pprof. Cache AST parsing. Parallelize file analysis. |

## Execution Order

```
Week 1: Tasks 1-4   (domain types, ports, adapters -- foundation)
Week 2: Tasks 5-6   (onboard service + renderer)
Week 3: Tasks 7-8   (fix service -- safe fixes + instructions)
Week 4: Tasks 9-10  (validate service -- cache + delta)
Week 5: Tasks 11-12 (CLI commands + MCP tools)
Week 6: Tasks 13-15 (integration tests, E2E tests, verification)
```
