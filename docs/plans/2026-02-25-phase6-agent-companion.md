# Phase 6: Agent Companion -- Onboard, Fix, Validate

**Goal:** Transform openkraft from a passive scoring tool into a real-time companion for AI coding agents. Agents consult openkraft in real-time to write correct code from the first attempt.

**Architecture:** Three new capabilities layered on the existing scoring infrastructure: an `onboard` command for CLAUDE.md generation, a `fix` command for hybrid auto-repair, and a `validate` command for incremental file-level checking. Same hexagonal pattern. MCP-first design.

**Tech Stack:** Go 1.24, existing stack (Cobra, mark3labs/mcp-go, yaml.v3, lipgloss, testify). No new dependencies. No LLM dependency -- all analysis is deterministic AST + heuristics. Cache via JSON files in `.openkraft/cache/`.

**Prerequisite:** Phase 5 (config-aware scoring with profiles) complete and all tests passing.

**Key constraint: reuse existing infrastructure.** The golden package (`internal/domain/golden/`) already has `SelectGolden`, `ScoreModule`, and `ExtractBlueprint`. The MCP server already has 6 tools including `openkraft_get_blueprint`, `openkraft_get_conventions`, and `openkraft_get_golden_example`. Phase 6 composes these -- it does not rewrite them.

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

#### Reuses Existing Infrastructure

| Existing component | Location | What onboard uses it for |
|---|---|---|
| `golden.SelectGolden` | `internal/domain/golden/selector.go` | Find best-structured module as canonical reference |
| `golden.ExtractBlueprint` | `internal/domain/golden/blueprint.go` | Get structural blueprint (layers, file roles) |
| `ModuleDetector.Detect` | `internal/domain/ports.go` | Detect all modules with their layers |
| `ProjectScanner.Scan` | `internal/domain/ports.go` | Scan file tree, detect CI, context files, etc. |
| `CodeAnalyzer.AnalyzeFile` | `internal/domain/ports.go` | AST analysis for functions, imports, interfaces |
| File naming classification | `internal/domain/scoring/discoverability.go` | Detect bare vs suffixed naming convention |

`OnboardService` is a **composition layer** -- it calls these existing components and assembles the results into an `OnboardReport`.

#### What It Detects

| Detection | Method | Output in CLAUDE.md |
|---|---|---|
| Architecture layout | Directory structure analysis | "Hexagonal with cross-cutting layers" |
| Module structure | Existing `ModuleDetector.Detect` | Module list with paths and layers |
| Naming conventions | Reuse bare/suffixed classification from discoverability scorer | "Files use bare naming (scanner.go, not scanner_service.go)" |
| Golden module | Existing `golden.SelectGolden` | "Follow `internal/domain/scoring` as the canonical example" |
| Build/test commands | Parse Makefile, go.mod | `go test ./...`, `make build` |
| Dependency rules | Import graph analysis between layers | "domain/ has zero imports from adapters/" |
| Key interfaces | AST analysis of interface declarations and satisfying types | Port-to-implementation mapping table |

#### CLI Interface

```
openkraft onboard [path]
```

Flags:
- `--force` -- overwrite existing CLAUDE.md (default: fail if file exists)
- `--format md|json` -- output format (default: md)
- `--output cursorrules|agents` -- generate .cursorrules or AGENTS.md instead of CLAUDE.md (deferred to later task if scope is too large)

Exit codes: `0` success, `1` error.

If CLAUDE.md already exists and `--force` is not set, exit with error. No `--append` flag -- partial updates create inconsistencies. Regenerate the whole file.

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

#### .cursorrules and AGENTS.md

Deferred to a follow-up task within Phase 6. The core value is CLAUDE.md generation. Once the `OnboardReport` is solid, rendering to other formats is mechanical. The `--output` flag will gate this.

---

### Capability 2: `openkraft fix` -- Hybrid Auto-Fix

Takes scoring results and applies fixes. Safe fixes applied directly. Complex fixes returned as structured instructions for the agent to execute.

#### Safe Auto-Fixes (Applied Directly)

Only fixes that are guaranteed to not break compilation:

| Fix | What it does |
|---|---|
| Generate CLAUDE.md | Calls `onboard` internally |
| Create test stubs | Generates missing `_test.go` files with package declaration and one placeholder test |
| Add .golangci.yml | Generates a basic linter config matching the project's detected thresholds |

These are **file creation only** -- they never modify existing files.

#### Instruction-Only Fixes (Returned as Structured Output)

Everything else is an instruction for the agent to execute, including things that seem simple but can produce bad output if auto-generated:

| Fix | What it returns |
|---|---|
| Add package doc comments | Suggests which packages need `// Package xxx ...` (agent writes better docs than we can auto-generate) |
| Generate .cursorrules | Instructions with detected conventions for the agent to write |
| Create AGENTS.md | Instructions with project metadata for the agent to compose |
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

Note: `FixService` uses `os.WriteFile` and `os.MkdirAll` directly for applying safe fixes (file creation only). No `FileWriter` port needed -- writing files to disk is not an external dependency that requires abstraction.

#### Output Format

```json
{
  "applied": [
    {"type": "create_file", "path": "CLAUDE.md", "description": "Generated from codebase analysis"},
    {"type": "create_file", "path": "internal/payments/service_test.go", "description": "Test stub"}
  ],
  "instructions": [
    {
      "type": "add_package_doc",
      "file": "internal/payments/service.go",
      "message": "Add package doc comment: // Package payments handles payment processing and validation.",
      "priority": "medium"
    },
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
2. On subsequent calls, accept a list of changed/added/deleted files.
3. For changed/added files: re-analyze with `CodeAnalyzer` and merge into cached data.
4. For deleted files: remove from cached `AnalyzedFiles` and `ScanResult` file lists.
5. For new files: add to cached `ScanResult` file lists (GoFiles, TestFiles, AllFiles).
6. Re-run all 6 scorers with the merged data (scorers are fast, <50ms total -- selective re-run adds complexity without meaningful performance gain).
7. Return the exact score delta compared to the cached baseline.

#### Cache Invalidation Strategy

**Content-based, not time-based.** A 5-minute TTL is wrong because the agent may change files within that window.

The cache stores a fingerprint map: `filename -> mod_time` for all Go files, `go.mod`, and `.openkraft.yaml`. On each `validate` call:

1. Check if `go.mod` or `.openkraft.yaml` mod_time changed -> full invalidation.
2. For the specified files, update the cache with fresh analysis.
3. The cache is valid indefinitely as long as the caller correctly reports which files changed.

The `--no-cache` flag forces a full re-scan for cases where the caller is unsure.

```go
// internal/domain/cache.go
type ProjectCache struct {
    ProjectPath   string                   `json:"project_path"`
    ConfigHash    string                   `json:"config_hash"`
    GoModHash     string                   `json:"go_mod_hash"`
    ScanResult    *ScanResult              `json:"scan_result"`
    AnalyzedFiles map[string]*AnalyzedFile `json:"analyzed_files"`
    BaselineScore *Score                   `json:"baseline_score"`
    FileModTimes  map[string]int64         `json:"file_mod_times"`
}

func (c *ProjectCache) IsInvalidated(goModHash, configHash string) bool {
    return c.GoModHash != goModHash || c.ConfigHash != configHash
}
```

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

func (s *ValidateService) Validate(projectPath string, changed, added, deleted []string, strict bool) (*domain.ValidationResult, error) {
    // 1. Load or create cache
    cached, err := s.cache.Load(projectPath)
    if err != nil || cached == nil || cached.IsInvalidated(goModHash, configHash) {
        cached, err = s.createCache(projectPath)
        if err != nil {
            return nil, fmt.Errorf("creating cache: %w", err)
        }
    }

    // 2. Handle deleted files
    for _, f := range deleted {
        delete(cached.AnalyzedFiles, f)
        cached.ScanResult.RemoveFile(f)
    }

    // 3. Handle added files
    for _, f := range added {
        cached.ScanResult.AddFile(f)
        af, err := s.analyzer.AnalyzeFile(filepath.Join(projectPath, f))
        if err != nil {
            continue
        }
        af.Path = f
        cached.AnalyzedFiles[f] = af
    }

    // 4. Handle changed files
    for _, f := range changed {
        af, err := s.analyzer.AnalyzeFile(filepath.Join(projectPath, f))
        if err != nil {
            continue
        }
        af.Path = f
        cached.AnalyzedFiles[f] = af
    }

    // 5. Re-detect modules and re-run all scorers
    modules := s.detector.Detect(cached.ScanResult)
    newScore := s.scoreService.ScoreWithData(cached.ScanResult, modules, cached.AnalyzedFiles)

    // 6. Compute delta
    result := computeDelta(cached.BaselineScore, newScore, strict)
    return result, nil
}
```

Note: `ScanResult` needs two new methods: `AddFile(path)` and `RemoveFile(path)` that maintain GoFiles, TestFiles, and AllFiles lists consistently.

#### Response Format

```json
{
  "status": "pass",
  "files_checked": ["internal/payments/service.go"],
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
- `--deleted <file1>,<file2>` -- specify deleted files (CLI cannot auto-detect)

Exit codes: `0` pass, `1` fail, `2` warn (unless `--strict`, then warn is also `1`).

#### MCP Tool

```
openkraft_validate(path: string, changed: []string, added?: []string, deleted?: []string, strict?: bool) -> ValidationResult
```

This is the primary tool agents call after each change. All file path parameters are arrays (MCP supports array parameters natively). Paths are relative to the project root.

---

## Agent Workflow

```
Agent receives task
  -> calls openkraft_onboard (understands project)
  -> writes code
  -> calls openkraft_validate(changed: [...]) (checks work)
  -> if issues: calls openkraft_fix (gets corrections)
  -> repeat until status == "pass"
  -> opens PR
```

---

## Architecture Changes

### New Domain Types

```go
// internal/domain/onboard.go
type OnboardReport struct {
    ProjectName       string             `json:"project_name"`
    ProjectType       string             `json:"project_type"`
    ArchitectureStyle string             `json:"architecture_style"`
    LayoutStyle       string             `json:"layout_style"`
    Modules           []ModuleInfo       `json:"modules"`
    NamingConvention  string             `json:"naming_convention"`
    GoldenModule      string             `json:"golden_module"`
    BuildCommands     []string           `json:"build_commands"`
    TestCommands      []string           `json:"test_commands"`
    DependencyRules   []DependencyRule   `json:"dependency_rules"`
    Interfaces        []InterfaceMapping `json:"interfaces"`
    Profile           *ScoringProfile    `json:"profile"`
}

// internal/domain/fix.go
type FixPlan struct {
    Applied      []AppliedFix  `json:"applied"`
    Instructions []Instruction `json:"instructions"`
    ScoreBefore  int           `json:"score_before"`
    ScoreAfter   int           `json:"score_after"`
}

type AppliedFix struct {
    Type        string `json:"type"`
    Path        string `json:"path"`
    Description string `json:"description"`
}

type Instruction struct {
    Type     string `json:"type"`
    File     string `json:"file"`
    Line     int    `json:"line,omitempty"`
    Message  string `json:"message"`
    Priority string `json:"priority"`
}

// internal/domain/validate.go
type ValidationResult struct {
    Status       string            `json:"status"`
    FilesChecked []string          `json:"files_checked"`
    Issues       []ValidationIssue `json:"issues"`
    ScoreImpact  ScoreImpact       `json:"score_impact"`
    Suggestions  []string          `json:"suggestions"`
}

type ValidationIssue struct {
    File     string `json:"file"`
    Line     int    `json:"line,omitempty"`
    Severity string `json:"severity"`
    Message  string `json:"message"`
    Category string `json:"category"`
}

type ScoreImpact struct {
    Overall    int            `json:"overall"`
    Categories map[string]int `json:"categories"`
}

// internal/domain/cache.go
type ProjectCache struct {
    ProjectPath   string                   `json:"project_path"`
    ConfigHash    string                   `json:"config_hash"`
    GoModHash     string                   `json:"go_mod_hash"`
    ScanResult    *ScanResult              `json:"scan_result"`
    AnalyzedFiles map[string]*AnalyzedFile `json:"analyzed_files"`
    BaselineScore *Score                   `json:"baseline_score"`
    FileModTimes  map[string]int64         `json:"file_mod_times"`
}
```

### New Port (CacheStore only)

```go
// internal/domain/ports.go (addition)

type CacheStore interface {
    Load(projectPath string) (*ProjectCache, error)
    Save(cache *ProjectCache) error
    Invalidate(projectPath string) error
}
```

No `FileWriter` port. Safe fixes use `os.WriteFile` directly -- writing files to the local filesystem is not an external dependency that needs abstraction.

### New Methods on Existing Types

```go
// internal/domain/model.go (additions to ScanResult)

func (s *ScanResult) AddFile(path string)    // adds to GoFiles/TestFiles/AllFiles
func (s *ScanResult) RemoveFile(path string) // removes from GoFiles/TestFiles/AllFiles
```

### New Application Services

```
internal/application/
  onboard_service.go       # Composes existing infrastructure into OnboardReport
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
  inbound/
    cli/
      onboard.go           # openkraft onboard command
      fix.go               # openkraft fix command
      validate.go          # openkraft validate command
    mcp/
      tools.go             # New tools: openkraft_onboard, openkraft_fix, openkraft_validate
```

Note: no `filewriter/` adapter. Removed as over-engineering.

---

## Task Breakdown

### Milestone Map

```
Task  1:  Domain types (OnboardReport, FixPlan, ValidationResult, ProjectCache)
Task  2:  CacheStore port + outbound adapter
Task  3:  ScanResult.AddFile/RemoveFile methods
Task  4:  OnboardService -- compose existing infrastructure into OnboardReport
Task  5:  CLAUDE.md renderer (markdown + JSON output)
Task  6:  FixService -- safe fix identification and application
Task  7:  FixService -- instruction generation for complex fixes
Task  8:  ValidateService -- cache management with content-based invalidation
Task  9:  ValidateService -- score delta computation
Task 10:  ScoreService.ScoreWithData (score from pre-loaded data, no disk I/O)
Task 11:  CLI commands: onboard, fix, validate
Task 12:  MCP tools: openkraft_onboard, openkraft_fix, openkraft_validate
Task 13:  Integration tests
Task 14:  E2E tests
Task 15:  Final verification
```

### Task Dependency Graph

```
    [1] Domain types
    / |  \
  [2] [3] |
Cache AddFile
   \  |  /
    \ | /
  [4] OnboardService -----> [5] CLAUDE.md renderer
   |                          |
  [6] FixService (safe) <----+
   |
  [7] FixService (instructions)
   |
  [10] ScoreService.ScoreWithData
   |
  [8] ValidateService (cache) ---> [9] ValidateService (delta)
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

Define all structs as specified in the Architecture Changes section. `ProjectCache.IsInvalidated` compares config and go.mod hashes.

**Tests:** JSON marshaling/unmarshaling round-trips. `IsInvalidated` returns true when hashes differ.

**Verify:** `go test ./internal/domain/ -run "TestOnboard|TestFix|TestValidat|TestCache" -v -count=1`

---

## Task 2: CacheStore Port + Adapter

**Files:**
- Modify: `internal/domain/ports.go` -- add `CacheStore` interface
- Create: `internal/adapters/outbound/cache/cache.go`
- Create: `internal/adapters/outbound/cache/cache_test.go`

Cache files stored as JSON in `.openkraft/cache/` with a hash of the project path as the filename. `Load` returns `nil, nil` if no cache exists. `Save` creates the cache directory if needed. `Invalidate` removes the cache file.

**Tests:** Save/load round-trip, load on non-existent cache, invalidation.

**Verify:** `go test ./internal/adapters/outbound/cache/ -v -count=1`

---

## Task 3: ScanResult.AddFile/RemoveFile

**Files:**
- Modify: `internal/domain/model.go`
- Add tests in `internal/domain/model_test.go`

`AddFile(path)`: classifies path (Go file, test file, or other) and adds to the appropriate slices. `RemoveFile(path)`: removes from all slices. Both maintain slice consistency.

**Tests:** Add a `.go` file -> appears in GoFiles and AllFiles. Add a `_test.go` -> appears in TestFiles, GoFiles, AllFiles. Remove a file -> disappears from all. Remove non-existent -> no-op.

**Verify:** `go test ./internal/domain/ -run "TestScanResult" -v -count=1`

---

## Task 4: OnboardService -- Compose Existing Infrastructure

**Files:**
- Create: `internal/application/onboard_service.go`
- Create: `internal/application/onboard_service_test.go`

`GenerateOnboardReport` orchestrates:
1. Scan via existing `ProjectScanner`
2. Detect modules via existing `ModuleDetector`
3. Analyze files via existing `CodeAnalyzer`
4. Select golden module via existing `golden.SelectGolden`
5. Extract blueprint via existing `golden.ExtractBlueprint`
6. Detect naming convention by reusing bare/suffixed classification from discoverability scorer
7. Detect build/test commands by parsing Makefile targets
8. Analyze import graph for dependency rules
9. Find interface-to-implementation mappings via AST

Steps 1-6 are pure composition of existing code. Steps 7-9 are new logic.

**Tests:** Run against `testdata/go-hexagonal/perfect`. Verify detected layers, modules, golden module, naming convention.

**Verify:** `go test ./internal/application/ -run TestOnboard -v -count=1`

---

## Task 5: CLAUDE.md Renderer

**Files:**
- Modify: `internal/application/onboard_service.go` -- add `RenderCLAUDEmd` and `RenderJSON`

`RenderCLAUDEmd` produces markdown following the template. Uses `text/template` for clean separation. Output must be under 200 lines. Empty sections omitted entirely. `RenderJSON` marshals to indented JSON.

**Tests:** Render against populated `OnboardReport`, verify markdown structure, verify line count under 200, verify JSON round-trip.

**Verify:** `go test ./internal/application/ -run TestRender -v -count=1`

---

## Task 6: FixService -- Safe Fixes

**Files:**
- Create: `internal/application/fix_service.go`
- Create: `internal/application/fix_service_test.go`

Safe fixes are limited to: CLAUDE.md generation (calls OnboardService), test stub creation (find missing `_test.go`), .golangci.yml generation. All safe fixes create new files only -- never modify existing files. Uses `os.WriteFile` and `os.MkdirAll` directly.

After applying fixes, runs `go build ./...` to verify compilation. If build fails, rolls back created files and returns error.

**Tests:** Plan fixes for project missing CLAUDE.md. Plan fixes for project missing test files. Apply fixes creates files. Dry run creates no files. Build verification catches broken output.

**Verify:** `go test ./internal/application/ -run TestFix -v -count=1`

---

## Task 7: FixService -- Instructions

**Files:**
- Modify: `internal/application/fix_service.go`

Generate structured instructions for: package doc comments (suggest which packages need docs), long functions (AST split points), too many parameters (struct suggestion), dependency violations (correct import path), missing module files (golden module comparison), .cursorrules content, AGENTS.md content. Each instruction includes file, line, message, and priority.

**Tests:** Instructions for 80-line function. Instructions for 6-parameter function. Instructions for missing package docs. Priority ordering.

**Verify:** `go test ./internal/application/ -run TestFixInstruction -v -count=1`

---

## Task 8: ValidateService -- Cache Management

**Files:**
- Create: `internal/application/validate_service.go`
- Create: `internal/application/validate_service_test.go`

Cache lifecycle:
1. Check cache via `CacheStore.Load`
2. If missing or invalidated (go.mod/config hash changed), run full scan and save
3. Process changed/added/deleted files and update cached data
4. Pass to delta computation

Content-based invalidation. No time-based TTL.

**Tests:** First call creates cache. Second call uses cache (verify via mock call counts). Config change forces re-scan. go.mod change forces re-scan.

**Verify:** `go test ./internal/application/ -run TestValidate -v -count=1`

---

## Task 9: ValidateService -- Score Delta

**Files:**
- Modify: `internal/application/validate_service.go`

After merging updated file data: re-run all 6 scorers via `ScoreService.ScoreWithData`, compute per-category delta, compute overall delta, classify issues by severity, set status, generate suggestions.

**Tests:** Changed file improving code_health (positive delta). Changed file worsening structure (negative delta). Added file affects file counts. Deleted file removes from analysis. Error-severity issue sets status "fail". Strict mode with warnings sets status "fail".

**Verify:** `go test ./internal/application/ -run TestValidateDelta -v -count=1`

---

## Task 10: ScoreService.ScoreWithData

**Files:**
- Modify: `internal/application/score_service.go`

New method `ScoreWithData(scan *ScanResult, modules []DetectedModule, analyzed map[string]*AnalyzedFile) *Score` that runs the 6 scorers with pre-loaded data. No disk I/O. This is the hot path for `validate` (<50ms).

The existing `ScoreProject` can be refactored to call `ScoreWithData` internally after loading data from disk.

**Tests:** ScoreWithData produces same result as ScoreProject for same input.

**Verify:** `go test ./internal/application/ -run TestScoreWithData -v -count=1`

---

## Task 11: CLI Commands

**Files:**
- Create: `internal/adapters/inbound/cli/onboard.go`
- Create: `internal/adapters/inbound/cli/fix.go`
- Create: `internal/adapters/inbound/cli/validate.go`
- Modify: `internal/adapters/inbound/cli/root.go` -- register all three commands

Three Cobra commands wiring the corresponding application services. Styled output using lipgloss (consistent with existing `score` command).

`onboard` prints content to stdout and writes file. `fix` shows applied fixes and instructions with priority colors. `validate` shows status, score impact, and suggestions.

**Tests:** Test each command with mock services. Verify flag parsing. Verify exit codes.

**Verify:** `go test ./internal/adapters/inbound/cli/ -run "TestOnboard|TestFixCmd|TestValidateCmd" -v -count=1`

---

## Task 12: MCP Tools

**Files:**
- Modify: `internal/adapters/inbound/mcp/tools.go`

Register three new tools: `openkraft_onboard`, `openkraft_fix`, `openkraft_validate`. Each tool creates its service, calls the handler, and returns JSON (or markdown for onboard with format=md). `openkraft_validate` accepts `changed`, `added`, and `deleted` as array parameters.

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
4. `openkraft validate <file>` -- verify pass status
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
| Cache invalidation misses changes | Content-based invalidation (config + go.mod hashes). Provide `--no-cache`. |
| Incremental validation misses cross-file issues | Re-run all 6 scorers (not selective). Re-detect modules after file changes. |
| MCP tool response too large | Cap CLAUDE.md at 200 lines. Cap fix instructions at 50 per call. |
| Performance regression on large codebases | Profile with pprof. Cache AST parsing. Parallelize file analysis. |
| Deleted/renamed files not handled | Explicit `deleted` parameter in validate. ScanResult.RemoveFile maintains consistency. |

## Execution Order

```
Week 1: Tasks 1-3   (domain types, cache adapter, ScanResult methods)
Week 2: Tasks 4-5   (onboard service + renderer)
Week 3: Tasks 6-7   (fix service -- safe fixes + instructions)
Week 4: Tasks 8-10  (validate service + ScoreWithData)
Week 5: Tasks 11-12 (CLI commands + MCP tools)
Week 6: Tasks 13-15 (integration tests, E2E tests, verification)
```
