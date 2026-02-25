# Phase 5: Config-Aware Scoring — Profiles, Auto-Detection & Interactive Init

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Push `ProjectConfig` into the heart of every scorer so that scoring adapts to the project's actual conventions, architecture, and constraints — instead of comparing against hardcoded assumptions. Add an auto-configurator that detects project characteristics and an interactive `init` that lets users confirm or override.

**Architecture:** New `ScoringProfile` domain struct carries all scorer parameters. A `ProfileBuilder` port builds it from `ProjectConfig` + defaults. Each scorer receives `*ScoringProfile` as its first argument. The `init` command gains an interactive mode that scans the project, proposes a profile, and writes `.openkraft.yaml`.

**Tech Stack:** Go 1.24, existing stack (Cobra, yaml.v3, lipgloss, testify). No new dependencies.

**Prerequisite:** Phase 4 complete (layout-agnostic scoring, all tests passing).

---

## Problem Analysis

The scoring pipeline has **150+ hardcoded values** across 6 scorers. These values encode assumptions that only hold for one style of Go project:

| Hardcoded Assumption | Reality |
|---|---|
| Layers are `domain`, `application`, `adapters` | Teams use `infrastructure`, `gateway`, `core`, `ports` |
| File suffixes are `_handler`, `_service`, etc. | Many Go projects use bare names (`scanner.go`, `parser.go`) |
| Max function size is 50 lines | Generated code, table-driven tests, and DSL builders are legitimately longer |
| CLAUDE.md worth 10 points | Teams using Cursor or Copilot shouldn't be penalized |
| Max 4 parameters per function | Constructors with dependency injection routinely take 5-7 |

**Current flow (broken):**
```
Config loaded → Scorers run with hardcoded values → Config applied as post-filter
```

**Target flow:**
```
Config loaded → ScoringProfile built (defaults + overrides) → Scorers receive profile → Config weight/skip applied
```

---

## Design

### Layer 1: ScoringProfile (Domain)

Pure domain struct. No YAML tags — this is internal. Built by the application layer.

```go
// ScoringProfile carries all parameters that scorers need.
// Built from project-type defaults merged with user overrides.
type ScoringProfile struct {
    // Structure
    ExpectedLayers       []string          // ["domain", "application", "adapters"]
    ExpectedDirs         []string          // ["internal", "cmd"]
    LayerAliases         map[string]string // {"adapter": "adapters", "infra": "adapters"}
    ExpectedFileSuffixes []string          // ["_model", "_service", "_handler", ...]
    NamingConvention     string            // "auto", "bare", "suffixed"

    // Code Health
    MaxFunctionLines  int // 50
    MaxFileLines      int // 300
    MaxNestingDepth   int // 3
    MaxParameters     int // 4
    MaxConditionalOps int // 2

    // Context Quality
    ContextFiles []ContextFileSpec // replaces hardcoded CLAUDE.md/AGENTS.md checks

    // Verifiability
    MinTestRatio float64 // 0.5 — test files / source files for full score

    // Predictability
    MaxGlobalVarPenalty int // 3 — points deducted per global var
}

type ContextFileSpec struct {
    Name    string // "CLAUDE.md"
    Points  int    // max points for this file
    MinSize int    // minimum bytes for size bonus (0 = existence only)
}
```

### Layer 2: Default Profiles (Domain)

Each project type gets a complete profile. These encode **best practices per type**.

```go
func DefaultProfile() ScoringProfile { ... }           // sensible Go defaults
func DefaultProfileForType(pt ProjectType) ScoringProfile // type-specific
```

| Parameter | API | CLI Tool | Library | Microservice |
|---|---|---|---|---|
| `ExpectedLayers` | domain, application, adapters | domain, application | domain | domain, application, adapters |
| `ExpectedDirs` | internal, cmd | internal, cmd | pkg | internal, cmd |
| `ExpectedFileSuffixes` | _model, _service, _handler, _repository, _ports | _model, _service | _model, _errors | _model, _service, _handler, _repository |
| `NamingConvention` | auto | auto | auto | auto |
| `MaxFunctionLines` | 50 | 50 | 40 | 50 |
| `MaxFileLines` | 300 | 300 | 250 | 300 |
| `MaxNestingDepth` | 3 | 3 | 3 | 3 |
| `MaxParameters` | 4 | 4 | 3 | 4 |
| `ContextFiles` | CLAUDE.md(10), AGENTS.md(8), .cursorrules(7) | CLAUDE.md(10), .cursorrules(7) | CLAUDE.md(10), AGENTS.md(8) | CLAUDE.md(10), AGENTS.md(8), .cursorrules(7) |
| `MinTestRatio` | 0.5 | 0.5 | 0.8 | 0.5 |

### Layer 3: ProfileBuilder (Application)

Merges defaults with user config. Lives in the application layer because it orchestrates domain types.

```go
func BuildProfile(cfg ProjectConfig) ScoringProfile {
    base := DefaultProfileForType(cfg.ProjectType)
    // Override only non-zero fields from cfg.Profile
    return merge(base, cfg.Profile)
}
```

### Layer 4: User Config Extension (Domain)

Extend `ProjectConfig` with an optional `Profile` section:

```go
type ProjectConfig struct {
    ProjectType   ProjectType        `yaml:"project_type"`
    Weights       map[string]float64 `yaml:"weights"`
    Skip          SkipConfig         `yaml:"skip"`
    ExcludePaths  []string           `yaml:"exclude_paths"`
    MinThresholds map[string]int     `yaml:"min_thresholds"`
    Profile       ProfileOverrides   `yaml:"profile"`       // NEW
}

type ProfileOverrides struct {
    ExpectedLayers       []string            `yaml:"expected_layers,omitempty"`
    ExpectedDirs         []string            `yaml:"expected_dirs,omitempty"`
    LayerAliases         map[string]string   `yaml:"layer_aliases,omitempty"`
    ExpectedFileSuffixes []string            `yaml:"expected_file_suffixes,omitempty"`
    NamingConvention     string              `yaml:"naming_convention,omitempty"`
    MaxFunctionLines     *int                `yaml:"max_function_lines,omitempty"`
    MaxFileLines         *int                `yaml:"max_file_lines,omitempty"`
    MaxNestingDepth      *int                `yaml:"max_nesting_depth,omitempty"`
    MaxParameters        *int                `yaml:"max_parameters,omitempty"`
    MaxConditionalOps    *int                `yaml:"max_conditional_ops,omitempty"`
    ContextFiles         []ContextFileSpec   `yaml:"context_files,omitempty"`
    MinTestRatio         *float64            `yaml:"min_test_ratio,omitempty"`
}
```

Pointer types (`*int`, `*float64`) para distinguir "no especificado" de "0".

### Layer 5: Auto-Configurator (Application)

New port + adapter. Analyzes the project and returns a `ProfileOverrides`:

```go
// Port (domain)
type ProjectAnalyzer interface {
    Analyze(scan *ScanResult, analyzed map[string]*AnalyzedFile) ProfileOverrides
}
```

What it detects:

| Signal | Detection Method | Maps To |
|---|---|---|
| Layers present | Scan top-level dirs under `internal/` | `ExpectedLayers` |
| Directory structure | Check for `internal/`, `cmd/`, `pkg/` | `ExpectedDirs` |
| Naming convention | Classify files as bare vs suffixed, pick dominant | `NamingConvention` |
| Function size P95 | 95th percentile of function line counts | `MaxFunctionLines` |
| File size P95 | 95th percentile of file line counts | `MaxFileLines` |
| Nesting P95 | 95th percentile of nesting depths | `MaxNestingDepth` |
| Param count P95 | 95th percentile of parameter counts | `MaxParameters` |
| Context files | Check which AI context files exist | `ContextFiles` |
| Test ratio | Current test/source ratio | `MinTestRatio` |
| File suffixes | Extract common suffixes from existing files | `ExpectedFileSuffixes` |

P95 strategy: set the threshold at the 95th percentile of the current codebase. This means 95% of existing code already passes, and only true outliers get flagged.

### Layer 6: Interactive Init (CLI Adapter)

The `init` command becomes interactive by default:

```
$ openkraft init

  Scanning project...

  Detected: Go project, cross-cutting layout
  Found layers: domain, application, adapters
  Naming convention: bare (79% of files)
  Function size P95: 45 lines
  Test ratio: 0.85

  ? Project type:
    > api
      cli-tool
      library
      microservice

  ? Expected layers: [domain, application, adapters]
    (enter to confirm, or type custom list)

  ? Max function lines: 45
    (detected from your codebase, enter to confirm)

  ? Naming convention: bare
    > bare (detected)
      suffixed
      auto (detect each run)

  Created .openkraft.yaml
```

Flags:
- `--auto` — skip questions, use auto-detected values (for CI/scripts)
- `--type <type>` — skip type question, use provided type
- `--force` — overwrite existing config
- `--non-interactive` — same as `--auto` (alias)

---

## Scorer Migration

Every scorer function gains `*ScoringProfile` as first parameter:

```go
// Before
func ScoreCodeHealth(scan *domain.ScanResult, analyzed map[string]*domain.AnalyzedFile) domain.CategoryScore

// After
func ScoreCodeHealth(profile *domain.ScoringProfile, scan *domain.ScanResult, analyzed map[string]*domain.AnalyzedFile) domain.CategoryScore
```

### Specific changes per scorer:

**code_health.go** — Replace 14 hardcoded thresholds:
- `scoreFunctionSize`: read `profile.MaxFunctionLines` instead of `50`/`100`
- `scoreFileSize`: read `profile.MaxFileLines` instead of `300`/`500`
- `scoreNestingDepth`: read `profile.MaxNestingDepth` instead of `3`/`4`
- `scoreParameterCount`: read `profile.MaxParameters` instead of `4`/`6`
- `scoreComplexConditionals`: read `profile.MaxConditionalOps` instead of `2`/`3`
- `collectCodeHealthIssues`: derive issue thresholds from profile (2x the max = warning)

**structure.go** — Replace 19 hardcoded values:
- `scoreExpectedLayers`: read `profile.ExpectedLayers` and `profile.ExpectedDirs`
- `normalizeLayerName`: read `profile.LayerAliases`
- `scoreExpectedFiles`: read `profile.ExpectedFileSuffixes` + respect `profile.NamingConvention`

**discoverability.go** — Replace naming convention assumption:
- `scoreFileNamingConventions`: if `profile.NamingConvention` is `"bare"` or `"suffixed"`, use it directly instead of auto-detecting

**verifiability.go** — Replace test ratio threshold:
- `scoreTestPresence`: read `profile.MinTestRatio` instead of `0.5`

**context_quality.go** — Replace 31 hardcoded file checks:
- `scoreAIContextFiles`: iterate `profile.ContextFiles` instead of hardcoded CLAUDE.md/AGENTS.md/etc.

**predictability.go** — Replace penalty value:
- `scoreExplicitDependencies`: read `profile.MaxGlobalVarPenalty` instead of `3`

---

## Score Service Changes

```go
func (s *ScoreService) ScoreProject(projectPath string) (*domain.Score, error) {
    // 0. Load config
    cfg, err := s.configLoader.Load(projectPath)

    // 1-3. Scan, detect, analyze (unchanged)

    // NEW: Build profile from config + defaults
    profile := BuildProfile(cfg)

    // 4. Run scorers WITH profile
    categories := []domain.CategoryScore{
        scoring.ScoreCodeHealth(&profile, scan, analyzed),
        scoring.ScoreDiscoverability(&profile, modules, scan, analyzed),
        scoring.ScoreStructure(&profile, modules, scan, analyzed),
        scoring.ScoreVerifiability(&profile, scan, analyzed),
        scoring.ScoreContextQuality(&profile, scan, analyzed),
        scoring.ScorePredictability(&profile, modules, scan, analyzed),
    }

    // 5-6. Apply config weights/skips, compute overall (unchanged)
}
```

---

## YAML Format (final)

```yaml
# .openkraft.yaml
project_type: api

weights:
  code_health: 0.25
  discoverability: 0.20
  structure: 0.15
  verifiability: 0.15
  context_quality: 0.15
  predictability: 0.10

skip:
  sub_metrics:
    - module_completeness

profile:
  expected_layers: [domain, application, adapters]
  expected_dirs: [internal, cmd]
  naming_convention: bare
  max_function_lines: 50
  max_file_lines: 300
  max_nesting_depth: 3
  max_parameters: 4
  context_files:
    - name: CLAUDE.md
      points: 10
      min_size: 500
    - name: AGENTS.md
      points: 8
    - name: .cursorrules
      points: 7
      min_size: 200

exclude_paths:
  - generated
  - vendor
```

**Backwards compatibility:** A YAML without `profile:` section works exactly as before. `BuildProfile` returns defaults when `ProfileOverrides` is empty.

---

## Task Breakdown

### Milestone Map

```
Task  1:     ScoringProfile domain type + default profiles
Task  2:     ProfileOverrides in ProjectConfig + YAML parsing + validation
Task  3:     BuildProfile function in application layer
Task  4:     Migrate code_health scorer to use profile
Task  5:     Migrate structure scorer to use profile
Task  6:     Migrate discoverability scorer to use profile
Task  7:     Migrate verifiability scorer to use profile
Task  8:     Migrate context_quality scorer to use profile
Task  9:     Migrate predictability scorer to use profile
Task  10:    Wire profile into ScoreService
Task  11:    ProjectAnalyzer port + auto-detect adapter
Task  12:    Interactive init command
Task  13:    Update generateConfig to include profile section
Task  14:    Update E2E tests + integration tests
Task  15:    Final verification
```

### Task Dependency Graph

```
         [1] ScoringProfile type
         / |  \
       /   |    \
     [2]  [3]   ...
  Config  Builder
     \     |
      \    |
    [4]-[9] Migrate 6 scorers (parallel)
         |
       [10] Wire into ScoreService
         |
       [11] ProjectAnalyzer auto-detect
         |
       [12] Interactive init
         |
       [13] Update generateConfig
         |
       [14] E2E tests
         |
       [15] Verification
```

---

## Task 1: ScoringProfile domain type + default profiles

**Files:**
- Create: `internal/domain/profile.go`
- Modify: `internal/domain/profile_test.go`

**Changes:**

Define `ScoringProfile`, `ContextFileSpec`, `DefaultProfile()`, and `DefaultProfileForType()`.

```go
// internal/domain/profile.go
package domain

type ScoringProfile struct {
    ExpectedLayers       []string
    ExpectedDirs         []string
    LayerAliases         map[string]string
    ExpectedFileSuffixes []string
    NamingConvention     string
    MaxFunctionLines     int
    MaxFileLines         int
    MaxNestingDepth      int
    MaxParameters        int
    MaxConditionalOps    int
    ContextFiles         []ContextFileSpec
    MinTestRatio         float64
    MaxGlobalVarPenalty  int
}

type ContextFileSpec struct {
    Name    string `yaml:"name"    json:"name"`
    Points  int    `yaml:"points"  json:"points"`
    MinSize int    `yaml:"min_size" json:"min_size,omitempty"`
}

func DefaultProfile() ScoringProfile {
    return ScoringProfile{
        ExpectedLayers:       []string{"domain", "application", "adapters"},
        ExpectedDirs:         []string{"internal", "cmd"},
        LayerAliases:         map[string]string{
            "adapter": "adapters", "infra": "adapters",
            "infrastructure": "adapters", "app": "application", "core": "application",
        },
        ExpectedFileSuffixes: []string{"_model", "_service", "_handler", "_repository", "_ports", "_errors", "_routes", "_rule"},
        NamingConvention:     "auto",
        MaxFunctionLines:     50,
        MaxFileLines:         300,
        MaxNestingDepth:      3,
        MaxParameters:        4,
        MaxConditionalOps:    2,
        ContextFiles: []ContextFileSpec{
            {Name: "CLAUDE.md", Points: 10, MinSize: 500},
            {Name: "AGENTS.md", Points: 8},
            {Name: ".cursorrules", Points: 7, MinSize: 200},
            {Name: ".github/copilot-instructions.md", Points: 5},
        },
        MinTestRatio:        0.5,
        MaxGlobalVarPenalty: 3,
    }
}
```

`DefaultProfileForType()` starts from `DefaultProfile()` and adjusts per-type values per the table in the Design section.

**Tests:** Verify each project type returns correct layers, dirs, thresholds. Verify `DefaultProfile()` returns non-zero values for every field.

**Verify:** `go test ./internal/domain/ -run TestProfile -v -count=1`

---

## Task 2: ProfileOverrides in ProjectConfig

**Files:**
- Modify: `internal/domain/config.go`
- Modify: `internal/domain/config_test.go`

**Changes:**

Add `ProfileOverrides` struct and `Profile` field to `ProjectConfig`. Extend `Validate()` to validate profile fields (positive ints, known naming conventions).

```go
type ProfileOverrides struct {
    ExpectedLayers       []string          `yaml:"expected_layers,omitempty"`
    ExpectedDirs         []string          `yaml:"expected_dirs,omitempty"`
    LayerAliases         map[string]string `yaml:"layer_aliases,omitempty"`
    ExpectedFileSuffixes []string          `yaml:"expected_file_suffixes,omitempty"`
    NamingConvention     string            `yaml:"naming_convention,omitempty"`
    MaxFunctionLines     *int              `yaml:"max_function_lines,omitempty"`
    MaxFileLines         *int              `yaml:"max_file_lines,omitempty"`
    MaxNestingDepth      *int              `yaml:"max_nesting_depth,omitempty"`
    MaxParameters        *int              `yaml:"max_parameters,omitempty"`
    MaxConditionalOps    *int              `yaml:"max_conditional_ops,omitempty"`
    ContextFiles         []ContextFileSpec `yaml:"context_files,omitempty"`
    MinTestRatio         *float64          `yaml:"min_test_ratio,omitempty"`
    MaxGlobalVarPenalty  *int              `yaml:"max_global_var_penalty,omitempty"`
}
```

Validation rules:
- `naming_convention` must be `""`, `"auto"`, `"bare"`, or `"suffixed"`
- `*int` fields must be > 0 if set
- `*float64` fields must be in [0.0, 1.0] if set
- `ContextFiles[].Points` must be > 0
- `ContextFiles[].Name` must not be empty

**Tests:** Validate rejects negative thresholds, unknown naming conventions. Accepts valid YAML with profile section.

**Verify:** `go test ./internal/domain/ -run TestConfig -v -count=1`

---

## Task 3: BuildProfile in application layer

**Files:**
- Modify: `internal/application/score_service.go` — add `BuildProfile` function
- Create: `internal/application/profile_builder_test.go`

**Changes:**

```go
func BuildProfile(cfg ProjectConfig) ScoringProfile {
    base := domain.DefaultProfileForType(cfg.ProjectType)
    p := cfg.Profile

    if len(p.ExpectedLayers) > 0       { base.ExpectedLayers = p.ExpectedLayers }
    if len(p.ExpectedDirs) > 0         { base.ExpectedDirs = p.ExpectedDirs }
    if len(p.LayerAliases) > 0         { base.LayerAliases = p.LayerAliases }
    if len(p.ExpectedFileSuffixes) > 0 { base.ExpectedFileSuffixes = p.ExpectedFileSuffixes }
    if p.NamingConvention != ""        { base.NamingConvention = p.NamingConvention }
    if p.MaxFunctionLines != nil       { base.MaxFunctionLines = *p.MaxFunctionLines }
    if p.MaxFileLines != nil           { base.MaxFileLines = *p.MaxFileLines }
    if p.MaxNestingDepth != nil        { base.MaxNestingDepth = *p.MaxNestingDepth }
    if p.MaxParameters != nil          { base.MaxParameters = *p.MaxParameters }
    if p.MaxConditionalOps != nil      { base.MaxConditionalOps = *p.MaxConditionalOps }
    if len(p.ContextFiles) > 0        { base.ContextFiles = p.ContextFiles }
    if p.MinTestRatio != nil           { base.MinTestRatio = *p.MinTestRatio }
    if p.MaxGlobalVarPenalty != nil    { base.MaxGlobalVarPenalty = *p.MaxGlobalVarPenalty }

    return base
}
```

**Tests:**
- Empty config returns default profile
- `project_type: cli-tool` with no overrides returns CLI defaults
- Single override (e.g., `max_function_lines: 80`) merges correctly
- Multiple overrides merge correctly
- Override with `expected_layers: [domain, infra]` replaces (not appends)

**Verify:** `go test ./internal/application/ -run TestBuildProfile -v -count=1`

---

## Tasks 4-9: Migrate 6 scorers

Each task follows the same pattern. All 6 can run **in parallel** once Tasks 1-3 are done.

### Task 4: Migrate code_health

**Files:** `internal/domain/scoring/code_health.go`, `internal/domain/scoring/code_health_test.go`

**Signature change:**
```go
func ScoreCodeHealth(profile *domain.ScoringProfile, scan *domain.ScanResult, analyzed map[string]*domain.AnalyzedFile) domain.CategoryScore
```

**Replacements:**
- `scoreFunctionSize`: `50` → `profile.MaxFunctionLines`, `100` → `profile.MaxFunctionLines * 2`
- `scoreFileSize`: `300` → `profile.MaxFileLines`, `500` → `int(float64(profile.MaxFileLines) * 1.67)`
- `scoreNestingDepth`: `3` → `profile.MaxNestingDepth`, `4` → `profile.MaxNestingDepth + 1`
- `scoreParameterCount`: `4` → `profile.MaxParameters`, `6` → `profile.MaxParameters + 2`
- `scoreComplexConditionals`: `2` → `profile.MaxConditionalOps`, `3` → `profile.MaxConditionalOps + 1`
- `collectCodeHealthIssues`: derive thresholds as `2 * profile.MaxXxx`

Every sub-scorer gains `profile *domain.ScoringProfile` as first param.

**Tests:** Add test with custom profile (e.g., `MaxFunctionLines: 20`) and verify scoring changes.

### Task 5: Migrate structure

**Files:** `internal/domain/scoring/structure.go`, `internal/domain/scoring/structure_test.go`

**Replacements:**
- `scoreExpectedLayers`: `expectedLayers` from `profile.ExpectedLayers`, dir check from `profile.ExpectedDirs`
- `normalizeLayerName`: iterate `profile.LayerAliases` map
- `scoreExpectedFiles`: use `profile.ExpectedFileSuffixes`; if `profile.NamingConvention == "bare"`, give full credit on this sub-metric

### Task 6: Migrate discoverability

**Files:** `internal/domain/scoring/discoverability.go`, `internal/domain/scoring/discoverability_test.go`

**Replacements:**
- `scoreFileNamingConventions`: if `profile.NamingConvention` is `"bare"` or `"suffixed"`, skip auto-detection and measure consistency against the declared convention

### Task 7: Migrate verifiability

**Files:** `internal/domain/scoring/verifiability.go`, `internal/domain/scoring/verifiability_test.go`

**Replacements:**
- `scoreTestPresence`: `0.5` → `profile.MinTestRatio`

### Task 8: Migrate context_quality

**Files:** `internal/domain/scoring/context_quality.go`, `internal/domain/scoring/context_quality_test.go`

**Replacements:**
- `scoreAIContextFiles`: iterate `profile.ContextFiles` instead of hardcoded file list. Total points = sum of all `ContextFileSpec.Points`. Normalize to 30-point sub-metric.

### Task 9: Migrate predictability

**Files:** `internal/domain/scoring/predictability.go`, `internal/domain/scoring/predictability_test.go`

**Replacements:**
- `scoreExplicitDependencies`: `3` → `profile.MaxGlobalVarPenalty`

---

## Task 10: Wire profile into ScoreService

**Files:**
- Modify: `internal/application/score_service.go`

**Changes:**

Add `BuildProfile` call between config load and scorer invocation. Pass `&profile` to every scorer.

```go
profile := BuildProfile(cfg)

categories := []domain.CategoryScore{
    scoring.ScoreCodeHealth(&profile, scan, analyzed),
    scoring.ScoreDiscoverability(&profile, modules, scan, analyzed),
    // ...
}
```

**Tests:** Existing `score_service_test.go` must still pass. Add test that a custom config with `profile.max_function_lines: 80` produces different scores than default.

**Verify:** `go test ./internal/application/ -v -count=1`

---

## Task 11: ProjectAnalyzer — auto-detect adapter

**Files:**
- Modify: `internal/domain/ports.go` — add `ProjectAnalyzer` interface
- Create: `internal/adapters/outbound/analyzer/analyzer.go`
- Create: `internal/adapters/outbound/analyzer/analyzer_test.go`

**Port definition:**
```go
type ProjectAnalyzer interface {
    Analyze(scan *ScanResult, analyzed map[string]*AnalyzedFile) ProfileOverrides
}
```

**Adapter implementation:**

```go
package analyzer

type Analyzer struct{}

func New() *Analyzer { return &Analyzer{} }

func (a *Analyzer) Analyze(scan *domain.ScanResult, analyzed map[string]*domain.AnalyzedFile) domain.ProfileOverrides {
    p := domain.ProfileOverrides{}

    p.ExpectedLayers = detectLayers(scan)
    p.ExpectedDirs = detectDirs(scan)
    p.NamingConvention = detectNamingConvention(scan)
    p.ExpectedFileSuffixes = detectSuffixes(scan)

    maxFunc := percentile95FunctionLines(analyzed)
    p.MaxFunctionLines = &maxFunc
    // ... same for file size, nesting, params

    p.ContextFiles = detectContextFiles(scan)

    testRatio := detectTestRatio(scan)
    p.MinTestRatio = &testRatio

    return p
}
```

Detection functions:
- `detectLayers`: scan `internal/*/` dirs, return unique layer names
- `detectDirs`: check for `internal/`, `cmd/`, `pkg/`
- `detectNamingConvention`: classify files as bare/suffixed, return dominant
- `detectSuffixes`: extract `_xxx` suffixes from all Go files, return those appearing 2+ times
- `percentile95FunctionLines`: sort all function line counts, return P95
- `detectContextFiles`: check which AI context files exist, return with default points
- `detectTestRatio`: `len(TestFiles) / len(GoFiles - TestFiles)`

**Tests:** Run analyzer against `testdata/go-hexagonal/perfect` and verify detected layers, naming, thresholds.

**Verify:** `go test ./internal/adapters/outbound/analyzer/ -v -count=1`

---

## Task 12: Interactive init command

**Files:**
- Modify: `internal/adapters/inbound/cli/init.go`

**Changes:**

Rewrite `newInitCmd` to support interactive mode:

1. If `--auto` flag: scan → auto-detect → write config → done
2. If interactive (default, when stdin is a terminal):
   - Scan the project
   - Run `ProjectAnalyzer.Analyze()`
   - Present each detected value as a prompt with the detected value as default
   - User presses Enter to accept or types a new value
   - Write final `.openkraft.yaml`

Use Go stdlib `bufio.Scanner` for input — no TUI library needed. Keep it simple.

Prompt flow:
```
1. Project type    (select from list, default: auto-detected)
2. Expected layers (show detected, confirm or edit)
3. Naming convention (show detected, confirm or edit)
4. Max function lines (show P95, confirm or edit)
5. Max file lines  (show P95, confirm or edit)
6. Context files   (show detected, confirm or edit)
```

Flags:
- `--auto` / `--non-interactive`: skip prompts, use auto-detected values
- `--type <type>`: pre-set project type
- `--force`: overwrite existing config

**Tests:** Test with `--auto` flag against test fixtures. Verify generated YAML parses correctly and contains detected values.

**Verify:** `go test ./internal/adapters/inbound/cli/ -run TestInit -v -count=1`

---

## Task 13: Update generateConfig

**Files:**
- Modify: `internal/adapters/inbound/cli/init.go` — `generateConfig` function

**Changes:**

`generateConfig` now accepts a `ScoringProfile` (from auto-detect or interactive) and emits the `profile:` YAML section. Only emit non-default values to keep the YAML clean.

**Verify:** Generated YAML round-trips through `YAMLLoader.Load()` correctly.

---

## Task 14: E2E + integration tests

**Files:**
- Modify: `tests/e2e/e2e_test.go`
- Modify: `internal/domain/scoring/integration_test.go`

**Tests to add:**
1. Score with no config → same results as before (backwards compat)
2. Score with `project_type: cli-tool` → profile applies CLI defaults
3. Score with custom `profile.max_function_lines: 80` → code_health threshold changes
4. Score with `profile.expected_layers: [domain, infra]` → structure detects custom layers
5. Score with `profile.naming_convention: bare` → discoverability gives full naming credit
6. `init --auto` on testdata fixtures → generates valid YAML with detected values

**Verify:** `go test ./... -race -count=1`

---

## Task 15: Final verification

```bash
go clean -testcache
go test ./... -race -count=1
go build -o ./openkraft ./cmd/openkraft

# Without config (backwards compat)
rm -f .openkraft.yaml
./openkraft score . --json

# With auto-detected config
./openkraft init --auto --force
cat .openkraft.yaml
./openkraft score . --json

# Test fixtures
./openkraft score testdata/go-hexagonal/perfect --json
./openkraft score testdata/go-hexagonal/empty --json
```

**Expected:**
- No config: same scores as Phase 4
- With auto-detected config: scores reflect actual project characteristics, no false positives from hardcoded assumptions
- Perfect fixture: still scores high
- Empty fixture: still scores low

---

## Execution Order

```
Week 1: Tasks 1-3 (foundation: types, config, builder)
Week 2: Tasks 4-9 (migrate 6 scorers — parallelizable)
Week 3: Task 10 (wire into service)
Week 3: Tasks 11-12 (auto-detect + interactive init)
Week 4: Tasks 13-15 (generate config, tests, verification)
```

## Risk Mitigation

| Risk | Mitigation |
|---|---|
| Breaking existing configs | `ProfileOverrides` is optional; empty = unchanged behavior |
| Scorer signature change breaks tests | Tasks 4-9 each update their own tests |
| P95 detection on small codebases | Floor at sensible minimums (e.g., `MaxFunctionLines >= 30`) |
| Interactive init breaks CI | `--auto` flag for non-interactive use |
| Too many YAML fields overwhelm users | `generateConfig` only emits non-default values |
