# Phase 6: Agent Companion -- Consistency Coordinator for Multi-Agent Development

**Goal:** Transform openkraft from a passive scoring tool into a **consistency coordinator** for AI coding agents. When multiple agents work on the same repo, openkraft ensures they all follow the same conventions, architecture, and patterns -- something no model can do alone, no matter how intelligent it becomes.

**Identity shift:** Openkraft does not compete with AI models. Models write good code. Openkraft ensures that code written by 5 different agents over 3 months stays **internally consistent**. This is a coordination problem, not a capability problem -- and it gets harder as AI adoption grows, not easier.

**Architecture:** Three new capabilities: `onboard` generates the project's consistency contract, `fix` corrects drift from established patterns, `validate` detects drift incrementally in <500ms. Same hexagonal pattern. MCP-first design.

**Tech Stack:** Go 1.24, existing stack (Cobra, mark3labs/mcp-go, yaml.v3, lipgloss, testify). No new dependencies. No LLM dependency -- all analysis is deterministic AST + heuristics.

**Prerequisite:** Phase 5 (config-aware scoring with profiles) complete and all tests passing.

**Key constraint: reuse existing infrastructure.** The golden package (`internal/domain/golden/`) already has `SelectGolden`, `ScoreModule`, and `ExtractBlueprint`. The MCP server already has 6 tools including `openkraft_get_blueprint`, `openkraft_get_conventions`, and `openkraft_get_golden_example`. Phase 6 composes these -- it does not rewrite them.

---

## Why Consistency, Not Quality

AI models improve every 6 months. Each generation writes cleaner functions, better naming, more correct tests. Openkraft should not compete on things models will learn to do well.

What models **cannot** solve -- regardless of intelligence -- is **coordination across invocations**:

| Problem | Why models can't solve it | What openkraft does |
|---|---|---|
| Agent A uses bare naming, Agent B uses suffixed | Each agent sees only its own context. Neither knows what the other did. | Detects naming drift: "3 files use suffixed but project convention is bare" |
| 5 PRs each move a little logic from domain to application | Each PR looks reasonable in isolation. Cumulative effect is architecture erosion. | Detects architectural drift: "domain layer lost 3 functions to application in last 5 changes" |
| New module doesn't follow the golden module's pattern | Agent doesn't know which module is canonical | Detects structural drift: "module `payments` has 2 layers but golden module has 4" |
| Agent creates `utils.go` in a project that avoids utility files | Agent follows general Go best practices, not this project's conventions | Detects convention drift: "utils.go violates project pattern -- no utility files in codebase" |

The key insight: **the problem grows with AI adoption**. More agents = more drift risk. This makes openkraft more valuable over time, not less.

---

## Design

### Capability 1: `openkraft onboard` -- Generate the Consistency Contract

Analyzes the real codebase and generates a CLAUDE.md that serves as the **executable specification of project standards**. Not documentation -- a contract. Openkraft generates it, keeps it updated, and uses it as the reference for detecting drift.

Every line comes from codebase analysis. No generic content. No filler.

#### Reuses Existing Infrastructure

| Existing component | Location | What onboard uses it for |
|---|---|---|
| `golden.SelectGolden` | `internal/domain/golden/selector.go` | Find the canonical module that defines "how we do things here" |
| `golden.ExtractBlueprint` | `internal/domain/golden/blueprint.go` | Extract the structural blueprint other modules should follow |
| `ModuleDetector.Detect` | `internal/domain/ports.go` | Detect all modules with their layers |
| `ProjectScanner.Scan` | `internal/domain/ports.go` | Scan file tree, detect CI, context files, etc. |
| `CodeAnalyzer.AnalyzeFile` | `internal/domain/ports.go` | AST analysis for functions, imports, interfaces |
| File naming classification | `internal/domain/scoring/discoverability.go` | Detect bare vs suffixed naming convention |

`OnboardService` is a **composition layer** -- it calls these existing components and assembles the results into an `OnboardReport`.

#### What It Detects (and Codifies as Contract)

| Detection | Method | Contract rule in CLAUDE.md |
|---|---|---|
| Naming convention | Classify files as bare vs suffixed (existing scorer logic) | "ALWAYS use bare naming: `scanner.go`, never `scanner_service.go`" |
| Architecture layout | Directory structure + module detection | "This is hexagonal. domain/ never imports from adapters/." |
| Golden module | `golden.SelectGolden` | "Follow `internal/domain/scoring/` as the canonical example for new modules" |
| Module blueprint | `golden.ExtractBlueprint` | "Every module must have: domain types, port interfaces, at least one adapter" |
| File size norms | Statistical analysis of existing files | "Functions: max 50 lines. Files: max 300 lines." |
| Test patterns | Scan test files for naming conventions | "Tests follow `Test<Func>_<Scenario>` naming" |
| Build commands | Parse Makefile, go.mod | "`make test`, `make lint`, `go build ./...`" |
| Dependency rules | Import graph between layers | "domain/ has zero imports from application/ or adapters/" |
| Key interfaces | AST: interface declarations + satisfying types | "Port `ScoreHistory` -> implementation `JSONHistory` in `adapters/outbound/history/`" |

#### Generated CLAUDE.md Format

Target: under 200 lines. Every line is a rule, not a description. Empty sections omitted.

```markdown
# Project: {name}

{one-line description from go.mod or package doc}

## Architecture Contract

{project_type} using {architecture_style} with {layout_style} layout.
Golden module: `{path}` -- all new modules MUST follow this structure.

## Naming Rules

- File naming: {bare|suffixed} -- e.g., `scanner.go` not `scanner_service.go`
- Functions: max {n} lines, max {n} parameters
- Files: max {n} lines
- Test naming: `Test<Func>_<Scenario>` pattern

## Module Blueprint

Every module must contain:
{layers from golden module blueprint}

| Module | Path | Layers | Follows blueprint |
{detected modules with compliance status}

## Dependency Rules

- domain/ MUST NOT import from application/ or adapters/
- application/ imports domain/ only
- adapters/ import application/ and domain/
{additional detected rules}

## Key Interfaces

| Port | Implementation | Package |
{detected interface -> implementation mappings}

## Build & Test

{detected build and test commands}
```

The difference from a normal CLAUDE.md: every section is **prescriptive** ("MUST", "ALWAYS", "NEVER"), not descriptive ("uses", "has", "contains"). This is a contract that openkraft enforces via `validate`.

#### CLI Interface

```
openkraft onboard [path]
```

Flags:
- `--force` -- overwrite existing CLAUDE.md (default: fail if file exists)
- `--format md|json` -- output format (default: md)

Exit codes: `0` success, `1` error.

If CLAUDE.md already exists and `--force` is not set, exit with error. No `--append` flag -- partial updates create inconsistencies. Regenerate the whole file.

#### MCP Tool

```
openkraft_onboard(path: string, format?: "md" | "json") -> string
```

Returns the consistency contract directly. The agent reads this before writing any code.

#### .cursorrules and AGENTS.md

Deferred to a follow-up task. The core value is CLAUDE.md as the consistency contract. Once the `OnboardReport` is solid, rendering to other formats is mechanical.

---

### Capability 2: `openkraft fix` -- Correct Drift from Established Patterns

Takes scoring results and corrects deviations from the project's established patterns. Safe fixes applied directly. Complex drift corrections returned as structured instructions.

The key reframing: fix does not improve "quality" in the abstract. It **aligns code back to the project's own conventions**.

#### Safe Auto-Fixes (Applied Directly)

Only fixes that are guaranteed to not break compilation and that establish missing infrastructure:

| Fix | What it does | Why it's safe |
|---|---|---|
| Generate CLAUDE.md | Calls `onboard` internally | Creates new file, never modifies existing |
| Create test stubs | Generates missing `_test.go` with package declaration + placeholder | Creates new file only |
| Add .golangci.yml | Generates linter config matching project's detected thresholds | Creates new file only |

These are **file creation only** -- they never modify existing files.

#### Drift Correction Instructions (Returned as Structured Output)

Everything else is an instruction for the agent, framed as "align to project pattern":

| Drift type | What it returns |
|---|---|
| Naming inconsistency | "File `payment_service.go` uses suffixed naming but project convention is bare. Rename to `payment.go`." |
| Missing package docs | "Package `payments` has no doc comment. See `internal/domain/` for the project's doc style." |
| Module structure gap | "Module `auth` has 2 layers but golden module has 4. Add: port interfaces, adapter directory." |
| Function too long (vs project norm) | "Function `processOrder` is 82 lines. Project norm is 50 (p90 of existing functions). Split into `processOrder` + `validateOrderItems`." |
| Too many parameters (vs project norm) | "Function has 6 params. Project norm is 4. Group into `OrderRequest` struct." |
| Dependency violation | "`domain/user.go` imports `adapters/db`. Project rule: domain/ has zero adapter imports. Move to application layer." |
| Architecture erosion | "3 functions moved from domain to application in recent changes. This violates the project's layering pattern." |

Note: thresholds are **relative to the project itself** (p90 of function lengths, not an arbitrary global limit). This is what makes it drift detection, not generic linting.

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
    // 1. Score the project (detects drift)
    score, err := s.scoreService.ScoreProject(projectPath)
    if err != nil {
        return nil, fmt.Errorf("scoring project: %w", err)
    }

    // 2. Generate onboard report (establishes what "consistent" means)
    report, err := s.onboardService.GenerateOnboardReport(projectPath)
    if err != nil {
        return nil, fmt.Errorf("generating onboard report: %w", err)
    }

    plan := &domain.FixPlan{ScoreBefore: score.Overall}

    // 3. Identify safe fixes (missing infrastructure)
    plan.Applied = s.identifySafeFixes(projectPath, score, opts)

    // 4. Generate drift correction instructions (relative to project norms)
    if !opts.AutoOnly {
        plan.Instructions = s.generateDriftCorrections(score, report, opts)
    }

    // 5. Apply safe fixes (unless dry run)
    if !opts.DryRun {
        if err := s.applyFixes(projectPath, plan); err != nil {
            return nil, fmt.Errorf("applying fixes: %w", err)
        }
        afterScore, _ := s.scoreService.ScoreProject(projectPath)
        if afterScore != nil {
            plan.ScoreAfter = afterScore.Overall
        }
    }

    return plan, nil
}
```

Note: `FixService` uses `os.WriteFile` and `os.MkdirAll` directly for safe fixes (file creation only). No `FileWriter` port -- writing files to disk is not an external dependency that needs abstraction.

#### Output Format

```json
{
  "applied": [
    {"type": "create_file", "path": "CLAUDE.md", "description": "Generated consistency contract from codebase analysis"}
  ],
  "instructions": [
    {
      "type": "naming_drift",
      "file": "internal/payments/payment_service.go",
      "message": "Rename to payment.go -- project convention is bare naming (detected from 23/25 existing files)",
      "priority": "high",
      "project_norm": "bare naming (92% of files)"
    },
    {
      "type": "structure_drift",
      "file": "internal/auth/",
      "message": "Module has 2 layers but golden module (internal/domain/scoring/) has 4. Missing: port interfaces, test files.",
      "priority": "high",
      "project_norm": "4 layers per module"
    },
    {
      "type": "size_drift",
      "file": "internal/orders/service.go",
      "line": 45,
      "message": "Function processOrder is 82 lines. Project p90 is 50 lines. Split into processOrder + validateOrderItems.",
      "priority": "medium",
      "project_norm": "p90 function length: 50 lines"
    }
  ],
  "score_before": 78,
  "score_after": 83
}
```

Each instruction includes `project_norm` -- the specific evidence from the project that defines what "consistent" means. This makes the instruction verifiable, not opinionated.

#### CLI Interface

```
openkraft fix [path]
```

Flags:
- `--dry-run` -- show drift corrections without applying
- `--auto-only` -- only apply safe auto-fixes, skip drift instructions
- `--category <name>` -- fix only drift in a specific scoring category

Exit codes: `0` success, `1` error.

#### MCP Tool

```
openkraft_fix(path: string, dry_run?: bool, category?: string) -> FixPlan
```

Agent calls this after scoring to get drift corrections with project-specific evidence.

---

### Capability 3: `openkraft validate` -- Incremental Drift Detection

The primary tool agents call after every change. Answers one question: **"Does this change drift from the project's established patterns?"**

Not "is this good code?" -- the model already knows that. But "is this code **consistent with what already exists here?**"

#### How It Works

1. On first call, run a full project scan and cache the result in `.openkraft/cache/`.
2. On subsequent calls, accept lists of changed/added/deleted files.
3. For changed/added files: re-analyze with `CodeAnalyzer` and merge into cached data.
4. For deleted files: remove from cached `AnalyzedFiles` and `ScanResult` file lists.
5. For new files: add to cached `ScanResult` file lists (GoFiles, TestFiles, AllFiles).
6. Re-run all 6 scorers with the merged data (scorers are fast, <50ms total).
7. Return the exact score delta + specific drift issues compared to baseline.

#### Cache Invalidation Strategy

**Content-based, not time-based.** The cache stores hashes of `go.mod` and `.openkraft.yaml`. On each call:

1. Check if `go.mod` or `.openkraft.yaml` hash changed -> full invalidation.
2. For the specified files, update the cache with fresh analysis.
3. Cache is valid indefinitely as long as caller reports which files changed.

`--no-cache` flag forces full re-scan.

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

    // 6. Compute delta and identify drift
    result := computeDelta(cached.BaselineScore, newScore, strict)
    return result, nil
}
```

Note: `ScanResult` needs two new methods: `AddFile(path)` and `RemoveFile(path)`.

#### Response Format

```json
{
  "status": "pass",
  "files_checked": ["internal/payments/service.go"],
  "drift_issues": [
    {
      "file": "internal/payments/service.go",
      "line": 23,
      "severity": "warning",
      "message": "Function processPayment has 5 parameters (project norm: max 4)",
      "category": "code_health",
      "drift_type": "size_drift"
    }
  ],
  "score_impact": {
    "overall": -2,
    "categories": {"code_health": -3, "structure": 0, "discoverability": 1}
  },
  "suggestions": [
    "This file introduces suffixed naming (payment_service.go) in a project that uses bare naming (92% of files). Consider renaming."
  ]
}
```

Fail conditions:
- Any new drift issue with severity `"error"` -> status `"fail"`
- Overall score drops below configured `min_threshold` -> status `"fail"`
- New warnings only -> status `"warn"`
- No drift detected -> status `"pass"`

#### CLI Interface

```
openkraft validate <file1> <file2> ...
```

Flags:
- `--strict` -- fail on any drift warning (not just errors)
- `--no-cache` -- force full re-scan
- `--deleted <file1>,<file2>` -- specify deleted files

Exit codes: `0` pass, `1` fail, `2` warn (unless `--strict`, then warn is also `1`).

#### MCP Tool

```
openkraft_validate(path: string, changed: []string, added?: []string, deleted?: []string, strict?: bool) -> ValidationResult
```

Primary tool agents call after each change. Array parameters (MCP supports natively). Paths relative to project root.

---

## Agent Workflow

```
Agent receives task
  -> calls openkraft_onboard (reads the consistency contract)
  -> writes code following the contract
  -> calls openkraft_validate(changed: [...]) (checks for drift)
  -> if drift: calls openkraft_fix (gets project-specific corrections)
  -> repeat until status == "pass" (no drift)
  -> opens PR (guaranteed consistent with existing codebase)
```

The difference from a quality workflow: the agent doesn't need to "write good code" (it already does). It needs to "write code that looks like it belongs in this project." That's what openkraft coordinates.

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
    ModuleBlueprint   []string           `json:"module_blueprint"`
    BuildCommands     []string           `json:"build_commands"`
    TestCommands      []string           `json:"test_commands"`
    DependencyRules   []DependencyRule   `json:"dependency_rules"`
    Interfaces        []InterfaceMapping `json:"interfaces"`
    Profile           *ScoringProfile    `json:"profile"`
    // Project norms (statistical, not arbitrary)
    NormFunctionLines int     `json:"norm_function_lines"` // p90 of existing functions
    NormFileLines     int     `json:"norm_file_lines"`     // p90 of existing files
    NormParameters    int     `json:"norm_parameters"`     // p90 of existing functions
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
    Type        string `json:"type"`
    File        string `json:"file"`
    Line        int    `json:"line,omitempty"`
    Message     string `json:"message"`
    Priority    string `json:"priority"`
    ProjectNorm string `json:"project_norm"` // evidence from the project itself
}

// internal/domain/validate.go
type ValidationResult struct {
    Status       string            `json:"status"`
    FilesChecked []string          `json:"files_checked"`
    DriftIssues  []DriftIssue      `json:"drift_issues"`
    ScoreImpact  ScoreImpact       `json:"score_impact"`
    Suggestions  []string          `json:"suggestions"`
}

type DriftIssue struct {
    File      string `json:"file"`
    Line      int    `json:"line,omitempty"`
    Severity  string `json:"severity"`
    Message   string `json:"message"`
    Category  string `json:"category"`
    DriftType string `json:"drift_type"` // naming_drift, structure_drift, size_drift, dependency_drift
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

No `FileWriter` port. Safe fixes use `os.WriteFile` directly.

### New Methods on Existing Types

```go
// internal/domain/model.go (additions to ScanResult)

func (s *ScanResult) AddFile(path string)    // adds to GoFiles/TestFiles/AllFiles
func (s *ScanResult) RemoveFile(path string) // removes from GoFiles/TestFiles/AllFiles
```

### New Application Services

```
internal/application/
  onboard_service.go       # Composes existing infrastructure into OnboardReport + consistency contract
  onboard_service_test.go
  fix_service.go           # Detects drift and generates project-relative corrections
  fix_service_test.go
  validate_service.go      # Cache management and incremental drift detection
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

---

## Task Breakdown

### Milestone Map

```
Task  1:  Domain types (OnboardReport, FixPlan, ValidationResult, ProjectCache, DriftIssue)
Task  2:  CacheStore port + outbound adapter
Task  3:  ScanResult.AddFile/RemoveFile methods
Task  4:  Project norms computation (p90 function lines, file lines, parameters)
Task  5:  OnboardService -- compose existing infrastructure into OnboardReport
Task  6:  Consistency contract renderer (CLAUDE.md markdown + JSON output)
Task  7:  FixService -- safe fix identification and application
Task  8:  FixService -- drift correction instructions with project norms
Task  9:  ValidateService -- cache management with content-based invalidation
Task 10:  ValidateService -- drift detection and score delta computation
Task 11:  ScoreService.ScoreWithData (score from pre-loaded data, no disk I/O)
Task 12:  CLI commands: onboard, fix, validate
Task 13:  MCP tools: openkraft_onboard, openkraft_fix, openkraft_validate
Task 14:  Integration tests
Task 15:  E2E tests
Task 16:  Final verification
```

### Task Dependency Graph

```
    [1] Domain types
    / |  \
  [2] [3] [4]
Cache Add  Norms
   \  |   /
    \ |  /
  [5] OnboardService -----> [6] Contract renderer
   |                          |
  [7] FixService (safe) <----+
   |
  [8] FixService (drift instructions)
   |
  [11] ScoreService.ScoreWithData
   |
  [9] ValidateService (cache) ---> [10] ValidateService (drift detection)
   |                                 |
   +------+------+------------------+
   |      |
 [12]   [13]
 CLI    MCP
 cmds   tools
   |      |
   +------+
      |
    [14] Integration tests
      |
    [15] E2E tests
      |
    [16] Verification
```

---

## Task 1: Domain Types

**Files:**
- Create: `internal/domain/onboard.go`
- Create: `internal/domain/fix.go`
- Create: `internal/domain/validate.go`
- Create: `internal/domain/cache.go`

Define all structs as specified in Architecture Changes. `ProjectCache.IsInvalidated` compares config and go.mod hashes. `DriftIssue` includes `DriftType` enum. `Instruction` includes `ProjectNorm` evidence field.

**Tests:** JSON round-trips. `IsInvalidated` logic. DriftType values.

**Verify:** `go test ./internal/domain/ -run "TestOnboard|TestFix|TestValidat|TestCache|TestDrift" -v -count=1`

---

## Task 2: CacheStore Port + Adapter

**Files:**
- Modify: `internal/domain/ports.go` -- add `CacheStore` interface
- Create: `internal/adapters/outbound/cache/cache.go`
- Create: `internal/adapters/outbound/cache/cache_test.go`

Cache files stored as JSON in `.openkraft/cache/` with a hash of the project path as filename. `Load` returns `nil, nil` if no cache exists. `Save` creates directory if needed. `Invalidate` removes cache file.

**Tests:** Save/load round-trip, load on non-existent cache, invalidation.

**Verify:** `go test ./internal/adapters/outbound/cache/ -v -count=1`

---

## Task 3: ScanResult.AddFile/RemoveFile

**Files:**
- Modify: `internal/domain/model.go`
- Add tests in `internal/domain/model_test.go`

`AddFile(path)`: classifies path (Go file, test file, or other) and adds to appropriate slices. `RemoveFile(path)`: removes from all slices. Both maintain consistency.

**Tests:** Add `.go` -> GoFiles + AllFiles. Add `_test.go` -> TestFiles + GoFiles + AllFiles. Remove -> disappears from all. Remove non-existent -> no-op.

**Verify:** `go test ./internal/domain/ -run "TestScanResult" -v -count=1`

---

## Task 4: Project Norms Computation

**Files:**
- Create: `internal/domain/norms.go`
- Create: `internal/domain/norms_test.go`

Compute statistical norms from the actual codebase:
- `ComputeNorms(analyzed map[string]*AnalyzedFile) ProjectNorms`
- P90 of function line counts (not mean -- p90 captures "what's normal here")
- P90 of file line counts
- P90 of parameter counts
- Dominant naming convention with percentage

These norms are what make drift detection relative, not arbitrary. "Your function is 82 lines" means nothing. "Your function is 82 lines in a project where p90 is 50" means drift.

**Tests:** Compute norms for known distributions. Verify p90 calculation. Edge cases: empty input, single file.

**Verify:** `go test ./internal/domain/ -run "TestNorms" -v -count=1`

---

## Task 5: OnboardService -- Compose Existing Infrastructure

**Files:**
- Create: `internal/application/onboard_service.go`
- Create: `internal/application/onboard_service_test.go`

`GenerateOnboardReport` orchestrates:
1. Scan via existing `ProjectScanner`
2. Detect modules via existing `ModuleDetector`
3. Analyze files via existing `CodeAnalyzer`
4. Select golden module via existing `golden.SelectGolden`
5. Extract blueprint via existing `golden.ExtractBlueprint`
6. Detect naming convention (reuse discoverability scorer logic)
7. Compute project norms via `ComputeNorms` (Task 4)
8. Detect build/test commands by parsing Makefile targets
9. Analyze import graph for dependency rules
10. Find interface-to-implementation mappings via AST

Steps 1-7 are composition of existing code. Steps 8-10 are new logic.

**Tests:** Run against `testdata/go-hexagonal/perfect`. Verify detected layers, modules, golden module, naming convention, norms.

**Verify:** `go test ./internal/application/ -run TestOnboard -v -count=1`

---

## Task 6: Consistency Contract Renderer

**Files:**
- Modify: `internal/application/onboard_service.go` -- add `RenderContract` and `RenderJSON`

`RenderContract` produces prescriptive markdown (MUST/ALWAYS/NEVER language). Uses `text/template`. Output under 200 lines. Empty sections omitted. `RenderJSON` marshals `OnboardReport` to indented JSON.

**Tests:** Render against populated `OnboardReport`. Verify prescriptive language. Verify line count under 200. Verify JSON round-trip.

**Verify:** `go test ./internal/application/ -run TestRender -v -count=1`

---

## Task 7: FixService -- Safe Fixes

**Files:**
- Create: `internal/application/fix_service.go`
- Create: `internal/application/fix_service_test.go`

Safe fixes: CLAUDE.md (calls OnboardService), test stubs, .golangci.yml. File creation only -- never modifies existing. Uses `os.WriteFile` directly.

After applying, runs `go build ./...` to verify. Rolls back on failure.

**Tests:** Plan fixes for missing CLAUDE.md. Plan fixes for missing tests. Apply creates files. Dry run creates nothing. Build check catches broken output.

**Verify:** `go test ./internal/application/ -run TestFix -v -count=1`

---

## Task 8: FixService -- Drift Correction Instructions

**Files:**
- Modify: `internal/application/fix_service.go`

Generate instructions with `ProjectNorm` evidence:
- Naming drift: "File uses suffixed but project is 92% bare"
- Structure drift: "Module has 2 layers but golden has 4"
- Size drift: "Function is 82 lines but project p90 is 50"
- Dependency drift: "domain/ imports adapters/ -- violates project rule"
- Missing package docs: "Package has no doc comment, see `internal/domain/` for project style"

Each instruction includes `project_norm` field with the statistical or structural evidence.

**Tests:** Instructions for naming drift. Instructions for structure gap vs golden. Instructions for oversized function vs p90. Priority ordering.

**Verify:** `go test ./internal/application/ -run TestFixInstruction -v -count=1`

---

## Task 9: ValidateService -- Cache Management

**Files:**
- Create: `internal/application/validate_service.go`
- Create: `internal/application/validate_service_test.go`

Cache lifecycle: load -> check hashes -> if invalid: full scan -> process changed/added/deleted -> pass to drift detection.

Content-based invalidation only. No TTL.

**Tests:** First call creates cache. Second uses cache. Config change forces re-scan. go.mod change forces re-scan. Added/deleted files update ScanResult.

**Verify:** `go test ./internal/application/ -run TestValidate -v -count=1`

---

## Task 10: ValidateService -- Drift Detection and Score Delta

**Files:**
- Modify: `internal/application/validate_service.go`

After merging data: re-run all 6 scorers via `ScoreService.ScoreWithData`, compute per-category delta, classify drift issues by type (`naming_drift`, `structure_drift`, `size_drift`, `dependency_drift`), set status, generate suggestions.

Drift classification uses project norms from OnboardReport to contextualize each issue.

**Tests:** File introducing naming drift. File improving consistency. Added file affects structure. Deleted file. Error-severity drift -> "fail". Strict mode.

**Verify:** `go test ./internal/application/ -run TestValidateDrift -v -count=1`

---

## Task 11: ScoreService.ScoreWithData

**Files:**
- Modify: `internal/application/score_service.go`

New method `ScoreWithData(scan *ScanResult, modules []DetectedModule, analyzed map[string]*AnalyzedFile) *Score` -- runs 6 scorers with pre-loaded data. No disk I/O. Hot path for validate (<50ms).

Refactor existing `ScoreProject` to call `ScoreWithData` internally.

**Tests:** ScoreWithData produces same result as ScoreProject for same input.

**Verify:** `go test ./internal/application/ -run TestScoreWithData -v -count=1`

---

## Task 12: CLI Commands

**Files:**
- Create: `internal/adapters/inbound/cli/onboard.go`
- Create: `internal/adapters/inbound/cli/fix.go`
- Create: `internal/adapters/inbound/cli/validate.go`
- Modify: `internal/adapters/inbound/cli/root.go` -- register commands

Three Cobra commands. Styled output using lipgloss.

`onboard` prints contract + writes file. `fix` shows applied fixes + drift instructions with norms. `validate` shows status + drift issues + score impact.

**Tests:** Flag parsing. Exit codes.

**Verify:** `go test ./internal/adapters/inbound/cli/ -run "TestOnboard|TestFixCmd|TestValidateCmd" -v -count=1`

---

## Task 13: MCP Tools

**Files:**
- Modify: `internal/adapters/inbound/mcp/tools.go`

Register: `openkraft_onboard`, `openkraft_fix`, `openkraft_validate`. Validate accepts `changed`, `added`, `deleted` as array parameters.

**Tests:** Tool handlers return valid content for test fixtures.

**Verify:** `go test ./internal/adapters/inbound/mcp/ -run "TestOnboard|TestFix|TestValidate" -v -count=1`

---

## Task 14: Integration Tests

**Files:**
- Create: `internal/application/integration_test.go`

1. OnboardService against `testdata/go-hexagonal/perfect` -- verify complete report with norms
2. FixService against fixture with known drift -- verify correct classification and project_norm evidence
3. ValidateService incremental -- verify drift detection and score delta
4. Full workflow: onboard -> fix -> validate -> verify drift resolved

**Verify:** `go test ./internal/application/ -run TestIntegration -v -count=1`

---

## Task 15: E2E Tests

**Files:**
- Modify: `tests/e2e/e2e_test.go`

1. `openkraft onboard` -- verify CLAUDE.md created with prescriptive language
2. `openkraft onboard --format json` -- verify JSON with norms
3. `openkraft fix --dry-run` -- verify drift instructions with project_norm
4. `openkraft validate <file>` -- verify drift detection
5. Full agent loop: onboard -> introduce drift -> validate catches it -> fix corrects it

**Verify:** `go test ./tests/e2e/ -v -count=1`

---

## Task 16: Final Verification

```bash
go clean -testcache
go test ./... -race -count=1
go build -o ./openkraft ./cmd/openkraft

# Consistency contract
./openkraft onboard .
cat CLAUDE.md
wc -l CLAUDE.md  # must be under 200
grep -c "MUST\|ALWAYS\|NEVER" CLAUDE.md  # must have prescriptive language

# Drift detection
./openkraft fix --dry-run .

# Incremental validation
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
| Consistency contract quality | Under 200 lines, prescriptive language, every rule backed by analysis |
| Drift detection accuracy | Catches naming, structure, size, and dependency drift with zero false positives on well-maintained repos |
| Instructions include evidence | Every drift instruction has `project_norm` field with statistical or structural proof |
| Backwards compatibility | Existing commands unchanged, new commands are additive |

## Risk Mitigation

| Risk | Mitigation |
|---|---|
| Contract generates generic content | Every line traces to a detection. No filler. Prescriptive language only. |
| Project norms are skewed by outliers | Use p90, not mean. Exclude generated/vendor files. Require minimum sample size (10+ functions). |
| Safe fixes break compilation | Run `go build` after applying. Roll back on failure. |
| Cache invalidation misses changes | Content-based (config + go.mod hashes). Provide `--no-cache`. |
| Drift detection is too noisy | Only flag drift when >10% divergence from project norm. Configurable sensitivity. |
| Validate misses cross-file issues | Re-run all 6 scorers. Re-detect modules after changes. |
| Deleted/renamed files not handled | Explicit `deleted` parameter. ScanResult.RemoveFile maintains consistency. |

## Execution Order

```
Week 1: Tasks 1-4   (domain types, cache adapter, ScanResult methods, norms computation)
Week 2: Tasks 5-6   (onboard service + contract renderer)
Week 3: Tasks 7-8   (fix service -- safe fixes + drift instructions)
Week 4: Tasks 9-11  (validate service + ScoreWithData)
Week 5: Tasks 12-13 (CLI commands + MCP tools)
Week 6: Tasks 14-16 (integration tests, E2E tests, verification)
```
