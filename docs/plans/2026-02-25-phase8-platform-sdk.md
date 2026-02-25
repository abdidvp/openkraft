# Phase 8: Platform SDK — Openkraft as a Dependency

**Goal:** Agent platforms integrate openkraft as a dependency, not a CLI wrapper. Package the scoring engine as an importable Go SDK with a clean programmatic API, extend analysis to TypeScript and Python codebases, and allow organizations to define custom scoring categories through a plugin system.

**Architecture:** A public `pkg/openkraft/` package wraps existing application services with functional options. New outbound adapters per language produce the same `AnalyzedFile` domain type from language-specific source. A plugin system (YAML rules for v1, WASM for v2) lets organizations extend the 6 built-in categories with custom scorers.

**Tech Stack:** Go 1.24, existing stack (Cobra, lipgloss, go/ast). New: tree-sitter-go bindings (TS/Py parsing, optional build tag), wazero (WASM runtime, v2 only). No CGo required for v1.

**Prerequisite:** Phase 6 (Agent Companion) and Phase 7 (Quality Gate) must be complete. The SDK surfaces `Score`, `Onboard`, `Validate`, `Fix`, and `Gate` -- all of which depend on those phases.

---

## Problem Analysis

Agent platforms -- Devin, Factory, Codegen, and custom internal agents -- want to integrate openkraft's scoring and onboarding intelligence into their workflows. Today they have three options, none acceptable for tight integration:

| Integration Method | Problem |
|---|---|
| Shell out to CLI | Fragile, slow (process spawn), hard error handling, requires binary on host |
| Call MCP server | Requires running a server process, JSON-RPC overhead, complex for Go-native platforms |
| Vendor internal packages | Breaks on internal API changes, couples to implementation details |

What platforms actually want: `go get github.com/openkraft/openkraft` and call `sdk.Score()` directly.

Additionally, openkraft only scores Go projects today. TypeScript and Python are the two most common languages in AI-assisted codebases. An agent platform building in Node.js or Python cannot use openkraft at all, even through the CLI.

Finally, organizations have domain-specific quality signals -- security practices, compliance rules, internal conventions -- that the default 6 categories do not cover. Without a plugin system, they must fork or ignore these signals entirely.

---

## Design

### Capability 1: Go SDK -- Clean Programmatic API

A new top-level `pkg/openkraft/` package exports openkraft's core operations as plain Go functions. This is the public API surface -- everything else remains in `internal/`.

#### Public API

```go
// pkg/openkraft/sdk.go
package openkraft

import "context"

// Score runs the full scoring pipeline on a project directory.
// Returns structured results with per-category breakdowns, sub-metrics, and issues.
func Score(ctx context.Context, projectPath string, opts ...Option) (*ScoreResult, error)

// Onboard generates AI context files (CLAUDE.md, AGENTS.md, etc.) for a project.
// Returns the generated content without writing to disk unless WriteFiles option is set.
func Onboard(ctx context.Context, projectPath string, opts ...Option) (*OnboardResult, error)

// Validate runs incremental scoring on a set of changed files.
// Faster than a full Score -- only re-evaluates metrics affected by the changed files.
func Validate(ctx context.Context, projectPath string, changedFiles []string, opts ...Option) (*ValidateResult, error)

// Fix analyzes a project and returns a plan of automated improvements.
// Does not apply changes unless the Apply option is set.
func Fix(ctx context.Context, projectPath string, opts ...Option) (*FixResult, error)

// Gate compares scores between the current state and a base branch.
// Returns pass/fail based on configured thresholds and score deltas.
func Gate(ctx context.Context, projectPath string, opts ...GateOption) (*GateResult, error)
```

#### Usage Examples

```go
import "github.com/openkraft/openkraft/pkg/openkraft"

// Simple: score a project with defaults
result, err := openkraft.Score(ctx, "/path/to/project")
fmt.Println(result.Overall)           // 82
fmt.Println(result.Grade)             // "A"
fmt.Println(result.Categories[0].Name) // "code_health"

// Configured: override profile and weights
result, err := openkraft.Score(ctx, "/path/to/project",
    openkraft.WithProjectType("api"),
    openkraft.WithWeights(map[string]float64{
        "code_health":    0.30,
        "verifiability":  0.25,
        "structure":      0.15,
        "discoverability": 0.10,
        "context_quality": 0.10,
        "predictability":  0.10,
    }),
    openkraft.WithTimeout(30*time.Second),
)

// Gate: compare against base branch
gate, err := openkraft.Gate(ctx, "/path/to/project",
    openkraft.WithBase("main"),
    openkraft.WithMaxDrop(5),
)
if gate.Status == "fail" {
    for _, reason := range gate.FailReasons {
        fmt.Println("BLOCKED:", reason)
    }
}
```

#### Design Principles

1. **Thin wrapper over existing services.** `pkg/openkraft/` delegates to `ScoreService`, `OnboardService`, `FixService`, and `GateService` in the application layer. Zero business logic lives in the SDK package.
2. **No global state.** All configuration passed explicitly via functional options. No `init()` side effects. No package-level variables that hold state.
3. **Context-aware.** Every function accepts `context.Context` for cancellation and timeout propagation. Long-running scans respect context deadlines.
4. **Same domain types.** Return types wrap `domain.Score`, `domain.FixPlan`, `domain.GateResult`, etc. The SDK re-exports domain types with stable public names -- consumers are not exposed to internal type paths.
5. **Zero dependencies beyond openkraft.** Consumers import `pkg/openkraft` and get everything they need. No transitive dependency on Cobra, lipgloss, or any CLI-specific package.

#### Functional Options

```go
// pkg/openkraft/options.go
package openkraft

import "time"

type options struct {
    projectType string
    weights     map[string]float64
    skipCats    []string
    skipSubs    []string
    excludePaths []string
    language    string
    plugins     []PluginConfig
    timeout     time.Duration
}

type Option func(*options)

func WithProjectType(pt string) Option {
    return func(o *options) { o.projectType = pt }
}

func WithWeights(w map[string]float64) Option {
    return func(o *options) { o.weights = w }
}

func WithSkipCategories(cats ...string) Option {
    return func(o *options) { o.skipCats = cats }
}

func WithSkipSubMetrics(subs ...string) Option {
    return func(o *options) { o.skipSubs = subs }
}

func WithExcludePaths(paths ...string) Option {
    return func(o *options) { o.excludePaths = paths }
}

func WithLanguage(lang string) Option {
    return func(o *options) { o.language = lang }
}

func WithPlugins(plugins ...PluginConfig) Option {
    return func(o *options) { o.plugins = plugins }
}

func WithTimeout(d time.Duration) Option {
    return func(o *options) { o.timeout = d }
}
```

Gate-specific options:

```go
type gateOptions struct {
    base    string
    maxDrop int
    record  bool
}

type GateOption func(*gateOptions)

func WithBase(branch string) GateOption {
    return func(o *gateOptions) { o.base = branch }
}

func WithMaxDrop(points int) GateOption {
    return func(o *gateOptions) { o.maxDrop = points }
}
```

#### Client for Repeated Calls

The top-level functions construct dependencies on each call -- fine for single invocations. For platforms that score many projects in sequence, a `Client` reuses internal instances:

```go
// pkg/openkraft/client.go
package openkraft

type Client struct {
    scanner      domain.ProjectScanner
    detector     domain.ModuleDetector
    analyzer     domain.CodeAnalyzer
    configLoader domain.ConfigLoader
    scoreService *application.ScoreService
}

func NewClient(opts ...Option) (*Client, error) {
    // Build dependency graph once
    // Return client that reuses instances
}

func (c *Client) Score(ctx context.Context, projectPath string, opts ...Option) (*ScoreResult, error) {
    // Delegate to pre-built scoreService
}

func (c *Client) Close() error {
    // Clean up any held resources (WASM runtimes, etc.)
}
```

Usage:

```go
client, err := openkraft.NewClient(
    openkraft.WithProjectType("api"),
)
defer client.Close()

// Reuse across many projects -- no re-initialization cost
for _, project := range projects {
    result, err := client.Score(ctx, project)
    // ...
}
```

#### Internal Wiring

The SDK constructs the same dependency graph the CLI uses, minus any CLI-specific packages:

```go
func Score(ctx context.Context, projectPath string, opts ...Option) (*ScoreResult, error) {
    o := defaultOptions()
    for _, opt := range opts {
        opt(&o)
    }

    // Apply context timeout
    if o.timeout > 0 {
        var cancel context.CancelFunc
        ctx, cancel = context.WithTimeout(ctx, o.timeout)
        defer cancel()
    }

    // Build dependencies (same as CLI bootstrap, no cobra/lipgloss)
    sc := scanner.New()
    det := detector.New()
    az := parser.New()
    cl := config.NewYAMLLoader()

    svc := application.NewScoreService(sc, det, az, cl)

    score, err := svc.ScoreProject(projectPath)
    if err != nil {
        return nil, err
    }

    return toScoreResult(score), nil
}
```

#### SDK Result Types

Stable public types that wrap domain types. These are the API contract:

```go
// pkg/openkraft/types.go
package openkraft

type ScoreResult struct {
    Overall    int              `json:"overall"`
    Grade      string           `json:"grade"`
    Categories []CategoryResult `json:"categories"`
    Issues     []IssueResult    `json:"issues,omitempty"`
}

type CategoryResult struct {
    Name       string            `json:"name"`
    Score      int               `json:"score"`
    Weight     float64           `json:"weight"`
    SubMetrics []SubMetricResult `json:"sub_metrics,omitempty"`
    Issues     []IssueResult     `json:"issues,omitempty"`
}

type SubMetricResult struct {
    Name   string `json:"name"`
    Score  int    `json:"score"`
    Points int    `json:"points"`
    Detail string `json:"detail,omitempty"`
}

type IssueResult struct {
    Severity  string `json:"severity"`
    Category  string `json:"category"`
    SubMetric string `json:"sub_metric,omitempty"`
    File      string `json:"file,omitempty"`
    Line      int    `json:"line,omitempty"`
    Message   string `json:"message"`
}

type GateResult struct {
    BaseScore      int            `json:"base_score"`
    HeadScore      int            `json:"head_score"`
    Delta          int            `json:"delta"`
    CategoryDeltas map[string]int `json:"category_deltas"`
    Status         string         `json:"status"` // "pass" or "fail"
    FailReasons    []string       `json:"fail_reasons,omitempty"`
}

type ValidateResult struct {
    Status     string          `json:"status"` // "pass" or "fail"
    Score      int             `json:"score"`
    Delta      int             `json:"delta"`
    NewIssues  []IssueResult   `json:"new_issues,omitempty"`
}

type OnboardResult struct {
    ClaudeMD       string `json:"claude_md"`
    AgentsMD       string `json:"agents_md"`
    CursorRules    string `json:"cursor_rules"`
    FilesWritten   []string `json:"files_written,omitempty"`
}

type FixResult struct {
    Fixes   []FixAction `json:"fixes"`
    Applied int         `json:"applied"`
}

type FixAction struct {
    Path        string `json:"path"`
    Description string `json:"description"`
    Category    string `json:"category"`
    Applied     bool   `json:"applied"`
}
```

---

### Capability 2: Multi-Language Support

Add TypeScript and Python support using the same scoring framework. The key insight: openkraft's 6 scoring categories are language-agnostic concepts. What changes per language is how we extract signals from source code.

#### Architecture

```
                        +-----------------+
                        |  ScoreService   |
                        +--------+--------+
                                 |
                    +------------+------------+
                    |                         |
              [language detect]         [6 scorers]
                    |                    (unchanged)
           +--------+--------+
           |        |        |
        GoParser  TSParser  PyParser
           |        |        |
      AnalyzedFile (same domain type for all languages)
```

Scorers in `internal/domain/scoring/` stay language-agnostic. They already operate on `AnalyzedFile`, which is abstract: function counts, line counts, nesting depths, naming patterns, import lists. New language-specific parsers produce `AnalyzedFile` from TypeScript or Python source.

#### What Changes Per Language

| Aspect | Go | TypeScript | Python |
|---|---|---|---|
| File extensions | `.go` | `.ts`, `.tsx` | `.py` |
| Function detection | `func` keyword | `function`, `=>`, class method | `def`, class method |
| Test file patterns | `_test.go` | `*.test.ts`, `*.spec.ts` | `*_test.py`, `test_*.py` |
| Naming conventions | snake_case files, CamelCase exports | camelCase files, PascalCase classes | snake_case files and functions |
| Package structure | `go.mod`, `internal/`, `pkg/` | `package.json`, `src/` | `pyproject.toml`, `__init__.py` |
| Import analysis | `import` statements, module graph | `import`/`require`, barrel files | `import`/`from`, relative vs absolute |
| Layer detection | `internal/{layer}/` | `src/{layer}/` | `{package}/{layer}/` |

#### What Stays the Same

- All 6 scoring categories and their weights
- Config system (`.openkraft.yaml`)
- MCP server interface
- CLI commands
- Scoring thresholds and calibration logic
- `ScoringProfile` and `ProfileOverrides`

#### New Parser Adapters

TypeScript parser:

```go
// internal/adapters/outbound/parser/ts_parser.go
type TypeScriptParser struct{}

func (p *TypeScriptParser) Parse(path string, content []byte) (*domain.AnalyzedFile, error) {
    // Extract: functions, classes, exports, imports, nesting, line counts
    // v1: regex-based extraction (no CGo dependency)
    // v2: tree-sitter for full AST accuracy (optional build tag)
}
```

Python parser:

```go
// internal/adapters/outbound/parser/py_parser.go
type PythonParser struct{}

func (p *PythonParser) Parse(path string, content []byte) (*domain.AnalyzedFile, error) {
    // Extract: functions (def), classes, imports, nesting via indentation, line counts
    // v1: regex + indentation-based scoping
    // v2: tree-sitter (optional build tag)
}
```

Multi-language scanner:

```go
// internal/adapters/outbound/scanner/multi_scanner.go
type MultiScanner struct {
    goScanner *GoScanner
    tsParser  *TypeScriptParser
    pyParser  *PythonParser
}

func (s *MultiScanner) Scan(projectPath string, excludePaths ...string) (*domain.ScanResult, error) {
    lang := detectLanguage(projectPath)
    switch lang {
    case "go":
        return s.goScanner.Scan(projectPath, excludePaths...)
    case "typescript":
        return s.scanWithParser(projectPath, s.tsParser, tsExtensions, tsTestPatterns, excludePaths)
    case "python":
        return s.scanWithParser(projectPath, s.pyParser, pyExtensions, pyTestPatterns, excludePaths)
    default:
        return nil, fmt.Errorf("unsupported language: %s", lang)
    }
}
```

#### Language Detection

Auto-detect from file extensions during scan. Count source files per language, use the dominant one. Support explicit override via config:

```yaml
# .openkraft.yaml
language: typescript  # skip auto-detection
```

For mixed-language projects (Go backend + TypeScript frontend), score each language subtree independently:

```yaml
language: auto  # default
paths:
  - path: backend/
    language: go
  - path: frontend/
    language: typescript
```

#### Shared Scoring Specification

To ensure consistency across the Go scoring engine and any future native language implementations, define the scoring algorithm as a JSON schema:

```json
{
  "$schema": "https://json-schema.org/draft/2020-12/schema",
  "title": "OpenKraft Scoring Specification",
  "description": "Canonical definition of scoring categories, sub-metrics, and algorithms",
  "properties": {
    "version": { "const": "1.0" },
    "categories": {
      "type": "array",
      "items": {
        "type": "object",
        "properties": {
          "name": { "type": "string" },
          "default_weight": { "type": "number" },
          "sub_metrics": {
            "type": "array",
            "items": {
              "type": "object",
              "properties": {
                "name": { "type": "string" },
                "points": { "type": "integer" },
                "algorithm": { "type": "string" },
                "thresholds": { "type": "object" }
              }
            }
          }
        }
      }
    }
  }
}
```

This spec serves as the source of truth. The Go scorers implement it. Future TypeScript/Python SDK implementations (if native) must conform to it. Conformance tests validate that all implementations produce identical scores for the same input.

#### Calibration

TypeScript and Python scoring must be validated against real projects before release:

- 10+ real TypeScript projects (mix of React, Node.js, library)
- 10+ real Python projects (mix of Django, FastAPI, library, ML)
- Scores should distribute similarly to Go projects (median 50-60, well-structured projects 75+)

Language-specific adjustments will be needed in `ScoringProfile` defaults per language. For example, Python functions are typically longer than Go functions, so `MaxFunctionLines` defaults should be higher for Python.

---

### Capability 3: Plugin System

Allow organizations to define custom scoring categories that integrate seamlessly into the overall score. Plugin results appear in CLI output, MCP responses, and SDK results alongside built-in categories.

#### Plugin Interface

```go
// internal/domain/plugin.go
type ScorerPlugin interface {
    Name() string
    Score(scan *ScanResult, analyzed map[string]*AnalyzedFile) CategoryScore
}
```

Plugins receive the same inputs as built-in scorers and return the same `CategoryScore` type. This means plugin-generated scores are indistinguishable from built-in scores in all output formats.

#### Configuration

```yaml
# .openkraft.yaml
plugins:
  - name: security_practices
    path: .openkraft/plugins/security.yaml
    weight: 0.10
  - name: compliance
    path: .openkraft/plugins/compliance.wasm
    weight: 0.05
```

Plugin weights are added to the scoring pool. The 6 built-in categories are re-normalized so all weights (built-in + plugins) sum to 1.0.

#### v1: YAML Rule Definitions

The simplest approach. Organizations define rules as patterns and file checks:

```yaml
# .openkraft/plugins/security.yaml
name: security_practices
weight: 0.10
sub_metrics:
  - name: no_hardcoded_secrets
    points: 30
    rule:
      type: pattern_absence
      pattern: "(password|secret|api_key)\\s*=\\s*\"[^\"]+\""
      scope: "**/*.go"
      message: "Hardcoded secret found in %s at line %d"

  - name: input_validation
    points: 20
    rule:
      type: pattern_presence
      pattern: "validate|sanitize"
      scope: "internal/adapters/inbound/**/*.go"
      min_matches: 1
      message: "No input validation found in HTTP handler layer"

  - name: error_handling_no_panic
    points: 25
    rule:
      type: pattern_absence
      pattern: "panic\\("
      scope: "**/*.go"
      exclude: "**/*_test.go"
      message: "panic() call found in %s at line %d"

  - name: dependency_pinning
    points: 25
    rule:
      type: file_existence
      files: ["go.sum", "package-lock.json", "poetry.lock"]
      min_present: 1
      message: "No dependency lock file found"
```

Rule types supported in v1:

| Rule Type | Description |
|---|---|
| `pattern_absence` | Regex must NOT match any file in scope. Each match is a violation. |
| `pattern_presence` | Regex must match at least `min_matches` times in scope. |
| `file_existence` | At least `min_present` of the named files must exist in the project root. |
| `naming_convention` | Files in scope must match a naming pattern (e.g., `snake_case.go`). |

The `RuleBasedScorer` reads YAML and implements `ScorerPlugin`:

```go
// internal/domain/plugins/rule_scorer.go
type RuleBasedScorer struct {
    config RuleConfig
}

func (s *RuleBasedScorer) Name() string { return s.config.Name }

func (s *RuleBasedScorer) Score(scan *ScanResult, analyzed map[string]*AnalyzedFile) CategoryScore {
    var subMetrics []SubMetric
    var issues []Issue

    for _, rule := range s.config.SubMetrics {
        sm, ruleIssues := evaluateRule(rule, scan, analyzed)
        subMetrics = append(subMetrics, sm)
        issues = append(issues, ruleIssues...)
    }

    earned, total := 0, 0
    for _, sm := range subMetrics {
        earned += sm.Score
        total += sm.Points
    }

    score := 0
    if total > 0 {
        score = int(math.Round(float64(earned) / float64(total) * 100))
    }

    return CategoryScore{
        Name:       s.config.Name,
        Score:      score,
        Weight:     s.config.Weight,
        SubMetrics: subMetrics,
        Issues:     issues,
    }
}
```

#### v2: WASM Plugins

For organizations that need full programmatic control. Uses wazero (pure Go WASM runtime -- no CGo).

```go
// internal/domain/plugins/wasm_plugin.go
type WASMPlugin struct {
    runtime wazero.Runtime
    module  wazero.CompiledModule
    name    string
}

func LoadWASMPlugin(ctx context.Context, path string) (*WASMPlugin, error) {
    // 1. Read .wasm file
    // 2. Compile with wazero
    // 3. Validate exported functions: "name", "score"
    // 4. Return plugin
}

func (p *WASMPlugin) Score(scan *ScanResult, analyzed map[string]*AnalyzedFile) CategoryScore {
    // 1. Serialize scan + analyzed to JSON
    // 2. Allocate memory in WASM module, write JSON
    // 3. Call exported "score" function
    // 4. Read result memory, deserialize CategoryScore JSON
    // 5. Return result
}
```

WASM plugins can be written in any language that compiles to WASM (Rust, Go via TinyGo, C, AssemblyScript). They receive scan data as JSON and return a `CategoryScore` as JSON through a well-defined ABI:

```
// Required WASM exports:
//   name()        -> ptr to UTF-8 string
//   score(ptr, len) -> ptr to JSON CategoryScore
//   alloc(size)   -> ptr (memory allocation for host)
//   dealloc(ptr)  -> void (memory deallocation)
```

**Recommendation:** Ship YAML rules first. WASM support follows once the plugin interface is validated by real usage.

#### Plugin Discovery and Loading

```go
// internal/domain/plugins/loader.go
func LoadPlugins(cfg domain.ProjectConfig) ([]domain.ScorerPlugin, error) {
    var plugins []domain.ScorerPlugin

    for _, pc := range cfg.Plugins {
        ext := filepath.Ext(pc.Path)
        switch ext {
        case ".yaml", ".yml":
            p, err := LoadYAMLPlugin(pc.Path)
            if err != nil {
                return nil, fmt.Errorf("loading YAML plugin %s: %w", pc.Name, err)
            }
            plugins = append(plugins, p)

        case ".wasm":
            p, err := LoadWASMPlugin(context.Background(), pc.Path)
            if err != nil {
                return nil, fmt.Errorf("loading WASM plugin %s: %w", pc.Name, err)
            }
            plugins = append(plugins, p)

        default:
            return nil, fmt.Errorf("unsupported plugin format %q for %s", ext, pc.Name)
        }
    }

    return plugins, nil
}
```

Automatic discovery from `.openkraft/plugins/` directory:

```go
func DiscoverPlugins(projectPath string) ([]PluginConfig, error) {
    pluginDir := filepath.Join(projectPath, ".openkraft", "plugins")
    entries, err := os.ReadDir(pluginDir)
    if os.IsNotExist(err) {
        return nil, nil // No plugins directory -- not an error
    }
    // ...scan for .yaml and .wasm files, return PluginConfig for each
}
```

#### Weight Normalization

When plugins are present, all weights are re-normalized so they sum to 1.0:

```go
func normalizeWeights(categories []CategoryScore, plugins []ScorerPlugin) {
    total := 0.0
    for _, c := range categories {
        total += c.Weight
    }
    for _, p := range plugins {
        total += p.Weight()
    }

    if total == 0 {
        return
    }

    // Scale each weight so they sum to 1.0
    for i := range categories {
        categories[i].Weight /= total
    }
}
```

Adding a plugin at weight 0.10 slightly reduces each built-in category's effective weight. This is the correct behavior -- the plugin is taking a share of the overall score.

---

## Architecture Changes

### New Packages

| Package | Purpose |
|---|---|
| `pkg/openkraft/sdk.go` | Public SDK entry points: `Score`, `Onboard`, `Validate`, `Fix`, `Gate` |
| `pkg/openkraft/options.go` | Functional options for SDK calls |
| `pkg/openkraft/client.go` | Reusable client for repeated scoring calls |
| `pkg/openkraft/types.go` | Stable public result types (wrapping domain types) |
| `internal/adapters/outbound/parser/ts_parser.go` | TypeScript source analysis |
| `internal/adapters/outbound/parser/py_parser.go` | Python source analysis |
| `internal/adapters/outbound/scanner/multi_scanner.go` | Language detection and delegation |
| `internal/domain/plugins/loader.go` | Plugin discovery and loading |
| `internal/domain/plugins/rule_scorer.go` | YAML rule engine |
| `internal/domain/plugins/wasm_plugin.go` | WASM runtime adapter (v2) |

### New Domain Types

```go
// internal/domain/plugin.go
type ScorerPlugin interface {
    Name() string
    Score(scan *ScanResult, analyzed map[string]*AnalyzedFile) CategoryScore
}

type PluginConfig struct {
    Name   string  `yaml:"name"   json:"name"`
    Path   string  `yaml:"path"   json:"path"`
    Weight float64 `yaml:"weight" json:"weight"`
}
```

### Modified Types

```go
// internal/domain/config.go -- additions
type ProjectConfig struct {
    // ... existing fields ...
    Language string         `yaml:"language,omitempty" json:"language,omitempty"`
    Plugins  []PluginConfig `yaml:"plugins,omitempty"  json:"plugins,omitempty"`
    Paths    []PathConfig   `yaml:"paths,omitempty"    json:"paths,omitempty"`
}

type PathConfig struct {
    Path     string `yaml:"path"     json:"path"`
    Language string `yaml:"language" json:"language"`
}
```

### Dependency Graph

```
pkg/openkraft/
  -> internal/application/          (ScoreService, OnboardService, etc.)
    -> internal/domain/               (Score, AnalyzedFile, ScorerPlugin)
    -> internal/domain/scoring/       (6 built-in scorers)
    -> internal/domain/plugins/       (RuleBasedScorer, WASMPlugin)
    -> internal/adapters/outbound/
        scanner/                      (MultiScanner)
        parser/                       (GoParser, TSParser, PyParser)
        config/                       (YAMLLoader)
```

The SDK imports application services. Application services import domain types and outbound ports. Adapters implement ports. The hexagonal architecture is preserved -- the SDK is a new inbound adapter alongside CLI and MCP.

---

## Task Breakdown

### Milestone Map

```
Task  1:  SDK package structure and Score function
Task  2:  SDK result types and domain-to-public conversion
Task  3:  SDK Onboard, Validate, Fix, Gate functions
Task  4:  SDK Client (reusable instance) and functional options
Task  5:  Language detection and MultiScanner skeleton
Task  6:  TypeScript parser adapter
Task  7:  Python parser adapter
Task  8:  Language-specific ScoringProfile defaults
Task  9:  Plugin domain types and ScorerPlugin interface
Task 10:  YAML rule engine (RuleBasedScorer)
Task 11:  Plugin discovery, loading, and weight normalization
Task 12:  Wire plugins into ScoreService
Task 13:  SDK and plugin integration tests
Task 14:  Multi-language calibration (TS + Python, 10+ projects each)
Task 15:  Final verification and benchmarks
```

### Task Dependency Graph

```
[1] SDK Score function
 |
[2] SDK result types (depends on 1)
 |
[3] SDK remaining functions (depends on 2)
 |
[4] SDK Client + options (depends on 2)

[5] Language detection + MultiScanner
 |  \
[6]  [7] TS parser / Py parser (parallel, depend on 5)
 |    |
 +----+
 |
[8] Language-specific profiles (depends on 6, 7)
 |
[14] Calibration (depends on 8)

[9] Plugin domain types
 |
[10] YAML rule engine (depends on 9)
 |
[11] Plugin loading + weight normalization (depends on 10)
 |
[12] Wire plugins into ScoreService (depends on 11)

[13] Integration tests (depends on 4, 8, 12)
[15] Verification (depends on 13, 14)
```

Three workstreams can proceed in parallel after the initial SDK skeleton:
- **SDK functions** (Tasks 1-4): public API surface
- **Multi-language** (Tasks 5-8, 14): TS and Python parsing
- **Plugins** (Tasks 9-12): custom scoring categories

---

## Task 1: SDK Package Structure and Score Function

**Files:**
- Create: `pkg/openkraft/sdk.go`

Create the `pkg/openkraft/` package with the `Score` function. This is the minimal viable SDK -- a single function that constructs internal dependencies and delegates to `ScoreService`.

```go
func Score(ctx context.Context, projectPath string, opts ...Option) (*ScoreResult, error) {
    o := defaultOptions()
    for _, opt := range opts {
        opt(&o)
    }
    // Build ScoreService, delegate, convert result
}
```

**Tests:** `pkg/openkraft/sdk_test.go` -- score openkraft's own repo, verify non-zero result with all 6 categories.

**Verify:** `go test ./pkg/openkraft/ -run TestScore -v -count=1`

---

## Task 2: SDK Result Types and Conversion

**Files:**
- Create: `pkg/openkraft/types.go`
- Create: `pkg/openkraft/convert.go`

Define all public result types (`ScoreResult`, `CategoryResult`, `IssueResult`, etc.) and conversion functions from `domain.*` to public types. These types are the SDK's API contract and must be stable.

**Tests:** Round-trip conversion: `domain.Score` -> `ScoreResult` -> JSON -> deserialize -> verify all fields preserved.

**Verify:** `go test ./pkg/openkraft/ -run TestConvert -v -count=1`

---

## Task 3: SDK Remaining Functions

**Files:**
- Modify: `pkg/openkraft/sdk.go` -- add `Onboard`, `Validate`, `Fix`, `Gate`

Each function follows the same pattern: apply options, build dependencies, delegate to application service, convert result.

`Validate` is new application logic: given a list of changed files, determine which scoring categories are affected, and re-score only those. If a changed file is a test file, only `verifiability` needs re-scoring. If it is a source file in `internal/domain/`, `structure` and `code_health` need re-scoring.

**Tests:** Each function tested against openkraft's own repo or test fixtures.

**Verify:** `go test ./pkg/openkraft/ -run TestSDK -v -count=1`

---

## Task 4: SDK Client and Options

**Files:**
- Create: `pkg/openkraft/client.go`
- Create: `pkg/openkraft/options.go`

Implement the `Client` struct that pre-builds the dependency graph. Implement all functional options. The `Client` must be safe for concurrent use (multiple goroutines calling `client.Score` simultaneously).

**Tests:**
- Concurrent scoring: 10 goroutines scoring different fixture directories simultaneously
- Options: verify each option modifies behavior correctly
- Client reuse: score two different projects with the same client, verify independent results

**Verify:** `go test ./pkg/openkraft/ -run TestClient -race -v -count=1`

---

## Task 5: Language Detection and MultiScanner Skeleton

**Files:**
- Create: `internal/adapters/outbound/scanner/multi_scanner.go`
- Create: `internal/adapters/outbound/scanner/language.go`

Language detection logic: walk the project directory, count files by extension, return the dominant language. Support `go`, `typescript`, `python`. Handle edge cases:
- Mixed language projects: use the `paths` config or fall back to dominant language
- No recognized source files: return error
- Monorepo with multiple languages: detect per-subdirectory

The `MultiScanner` implements `domain.ProjectScanner` and delegates to language-specific scanners.

**Tests:**
- Go project with `go.mod` detected as Go
- Project with `package.json` and `.ts` files detected as TypeScript
- Project with `pyproject.toml` and `.py` files detected as Python
- Mixed project defaults to dominant language

**Verify:** `go test ./internal/adapters/outbound/scanner/ -run TestMultiScanner -v -count=1`

---

## Task 6: TypeScript Parser Adapter

**Files:**
- Create: `internal/adapters/outbound/parser/ts_parser.go`
- Create: `internal/adapters/outbound/parser/ts_parser_test.go`

Extract `AnalyzedFile` from TypeScript source using regex-based parsing (v1). Must handle:
- `function` declarations, arrow functions, class methods
- `export` detection (named exports, default exports)
- `import` statements (ES modules and CommonJS `require`)
- Nesting depth (via brace counting with string/comment awareness)
- JSX/TSX (skip JSX expressions when counting nesting)

**Tests:** Parse real TypeScript files from well-known open-source projects. Verify function count, export detection, and import extraction against manually verified baselines.

**Verify:** `go test ./internal/adapters/outbound/parser/ -run TestTypeScript -v -count=1`

---

## Task 7: Python Parser Adapter

**Files:**
- Create: `internal/adapters/outbound/parser/py_parser.go`
- Create: `internal/adapters/outbound/parser/py_parser_test.go`

Extract `AnalyzedFile` from Python source. Key challenge: Python uses indentation for scoping, not braces. Must handle:
- `def` and `async def` function declarations
- `class` declarations and methods
- `import` and `from ... import` statements
- Nesting depth via indentation level tracking
- Decorators (`@property`, `@staticmethod`, etc.)
- Type hints for parameter counting

**Tests:** Parse real Python files. Verify function detection, class method extraction, and import analysis.

**Verify:** `go test ./internal/adapters/outbound/parser/ -run TestPython -v -count=1`

---

## Task 8: Language-Specific ScoringProfile Defaults

**Files:**
- Modify: `internal/domain/profile.go` -- add `DefaultProfileForLanguage`
- Modify: `internal/domain/config.go` -- add `Language` field

Each language needs adjusted defaults:

| Parameter | Go | TypeScript | Python |
|---|---|---|---|
| `MaxFunctionLines` | 50 | 40 | 60 |
| `MaxFileLines` | 500 | 400 | 500 |
| `MaxNestingDepth` | 4 | 5 | 4 |
| `MaxParameters` | 5 | 5 | 6 |
| `ExpectedLayers` | domain, application, adapters | components, hooks, services, utils | models, services, views, utils |
| `NamingConvention` | auto | camelCase | snake_case |
| `MinTestRatio` | 0.3 | 0.2 | 0.2 |

These are starting points. Task 14 (calibration) will refine them based on real-world scoring results.

**Tests:** Verify each language returns appropriate profile defaults. Verify that user overrides still take precedence.

**Verify:** `go test ./internal/domain/ -run TestLanguageProfile -v -count=1`

---

## Task 9: Plugin Domain Types and ScorerPlugin Interface

**Files:**
- Create: `internal/domain/plugin.go`
- Modify: `internal/domain/config.go` -- add `Plugins` and `PluginConfig`

Define the `ScorerPlugin` interface and `PluginConfig` type. Add `Plugins []PluginConfig` to `ProjectConfig`. Update config validation to validate plugin entries (name required, weight > 0, path exists).

**Tests:**
- Config with plugins validates correctly
- Config with invalid plugin weight (negative, > 1.0) fails validation
- Config with missing plugin path fails validation

**Verify:** `go test ./internal/domain/ -run TestPlugin -v -count=1`

---

## Task 10: YAML Rule Engine (RuleBasedScorer)

**Files:**
- Create: `internal/domain/plugins/rule_scorer.go`
- Create: `internal/domain/plugins/rule_scorer_test.go`

Implement the `RuleBasedScorer` that reads YAML rule definitions and evaluates them against scan results. Support all four rule types: `pattern_absence`, `pattern_presence`, `file_existence`, `naming_convention`.

Each rule evaluation:
1. Compile the regex pattern (cached after first compilation)
2. Walk files matching the `scope` glob
3. Apply the rule logic
4. Return a `SubMetric` with earned points and any issues

**Tests:**
- `pattern_absence`: file with hardcoded secret scores 0, clean file scores full points
- `pattern_presence`: file with validation call scores full points, missing scores 0
- `file_existence`: project with `go.sum` scores full points, without scores 0
- `naming_convention`: files matching convention score full points

**Verify:** `go test ./internal/domain/plugins/ -run TestRuleScorer -v -count=1`

---

## Task 11: Plugin Discovery, Loading, and Weight Normalization

**Files:**
- Create: `internal/domain/plugins/loader.go`
- Create: `internal/domain/plugins/loader_test.go`

Implement `LoadPlugins` (from config) and `DiscoverPlugins` (from `.openkraft/plugins/` directory). Implement weight normalization that re-scales all weights (built-in + plugin) to sum to 1.0.

**Tests:**
- Load a YAML plugin from a temp directory
- Discover plugins from `.openkraft/plugins/`
- Weight normalization: 6 built-in at 0.15 each (0.90 total) + 1 plugin at 0.10 = all weights scaled to sum to 1.0
- Empty plugins list: weights unchanged

**Verify:** `go test ./internal/domain/plugins/ -run TestLoader -v -count=1`

---

## Task 12: Wire Plugins into ScoreService

**Files:**
- Modify: `internal/application/score_service.go`

After running the 6 built-in scorers, load and run any configured plugins. Append plugin `CategoryScore` results to the categories list. Apply weight normalization before computing the overall score.

```go
func (s *ScoreService) ScoreProject(projectPath string) (*domain.Score, error) {
    // ... existing steps 1-5 ...

    // 5b. Load and run plugins
    plugins, err := plugins.LoadPlugins(cfg)
    if err != nil {
        return nil, fmt.Errorf("loading plugins: %w", err)
    }
    for _, p := range plugins {
        pluginScore := p.Score(scan, analyzed)
        categories = append(categories, pluginScore)
    }

    // 5c. Normalize weights if plugins present
    if len(plugins) > 0 {
        normalizeWeights(categories)
    }

    // 6. Apply config (skip, filter) -- now includes plugin categories
    categories = applyConfig(categories, cfg)

    // 7. Compute overall
    overall := domain.ComputeOverallScore(categories)
    // ...
}
```

**Tests:**
- Score a project with no plugins: identical to current behavior
- Score with a YAML plugin: 7 categories in result, weights sum to 1.0
- Score with a plugin that finds violations: plugin score below 100, issues reported

**Verify:** `go test ./internal/application/ -run TestScoreWithPlugins -v -count=1`

---

## Task 13: SDK and Plugin Integration Tests

**Files:**
- Create: `pkg/openkraft/sdk_integration_test.go`
- Create: `tests/e2e/sdk_test.go`
- Expand: `tests/e2e/e2e_test.go`

Integration tests with real scoring (no mocks):

1. `sdk.Score` on openkraft's own repo returns score > 0 with 6 categories
2. `sdk.Score` with a YAML plugin returns 7 categories
3. `sdk.Score` with `WithLanguage("go")` returns same result as auto-detection
4. `Client` concurrent usage: 5 goroutines scoring simultaneously
5. `sdk.Gate` on a real repo returns pass/fail status
6. TypeScript fixture project scores in expected range (requires Task 6)
7. Python fixture project scores in expected range (requires Task 7)

**Verify:** `go test ./... -race -count=1`

---

## Task 14: Multi-Language Calibration

**No code files -- analysis and tuning task.**

Score 10+ real TypeScript projects and 10+ real Python projects. For each:
1. Run openkraft and record the overall score and per-category breakdown
2. Compare against a manual assessment of the project's AI-agent readiness
3. Identify scoring outliers (e.g., high-quality project scoring low, or vice versa)
4. Adjust language-specific profile defaults (Task 8) to bring scores into alignment

Target distribution: median 50-60, well-structured projects 75+, poorly-structured projects 30-45.

Document calibration results in `docs/calibration/typescript.md` and `docs/calibration/python.md`.

**Verify:** Re-score all calibration projects after adjustment, confirm distribution matches target.

---

## Task 15: Final Verification and Benchmarks

```bash
# Full test suite
go clean -testcache
go test ./... -race -count=1

# Build
go build -o ./openkraft ./cmd/openkraft

# SDK benchmark
go test ./pkg/openkraft/ -bench=BenchmarkScore -benchmem

# Score openkraft itself via SDK
go test ./pkg/openkraft/ -run TestScoreOwnRepo -v

# Plugin example
./openkraft score . --json  # With example security plugin in .openkraft/plugins/

# Verify no CLI regression
./openkraft score .
./openkraft init .
./openkraft check .
```

Performance targets:
- `sdk.Score` overhead vs direct `ScoreService`: < 5ms
- `Client` reuse speedup: > 20% faster than repeated `sdk.Score` calls
- YAML plugin evaluation: < 10ms per plugin for a 500-file project

---

## Success Criteria

| Criterion | Measurement |
|---|---|
| SDK API is stable and documented | All 5 functions have godoc, examples compile, types are exported cleanly |
| SDK overhead is minimal | `sdk.Score` adds < 5ms vs calling `ScoreService` directly (benchmark test) |
| TypeScript scoring is calibrated | Validated against 10+ real TS projects; scores distribute 40-90 range |
| Python scoring is calibrated | Validated against 10+ real Python projects; scores distribute 40-90 range |
| Custom plugin works end-to-end | Example `security_practices` YAML plugin scores a project, appears in output |
| No global state in SDK | Concurrent `sdk.Score` calls with different configs produce correct independent results |
| Backwards compatibility | Existing CLI and MCP server behavior unchanged; no new required config fields |
| Plugin weight normalization is correct | Built-in + plugin weights always sum to 1.0 |

---

## Migration Path

- **Existing CLI and MCP server remain unchanged.** They continue to work as before. The SDK is a new entry point, not a replacement.
- **SDK wraps the same application services.** No logic duplication. If a scorer improves, the SDK benefits automatically.
- **Multi-language is additive.** Go scoring is unaffected. TypeScript and Python support is opt-in via language detection or config override.
- **Plugins are opt-in.** A project with no `plugins:` section in config behaves identically to today. Plugin weights are only applied when plugins are configured.
- **Semver commitment.** The `pkg/openkraft/` package follows Go module versioning. Breaking changes require a major version bump.

---

## Risk Mitigation

| Risk | Mitigation |
|---|---|
| Tree-sitter CGo dependency complicates builds | Start with regex-based TS/Py parsers; add tree-sitter as optional build tag |
| WASM plugin interface is hard to stabilize | Ship YAML rules first; defer WASM to v2 after interface is validated |
| Multi-language dilutes Go scoring quality | Separate calibration per language; language-specific profile defaults |
| Plugin weight normalization surprises users | Document clearly; warn when plugins reduce built-in weights by more than 20% |
| SDK public API surface grows uncontrollably | Minimal API (5 functions + Client); all extensions via options pattern |
| Concurrent SDK usage causes data races | No shared mutable state; each call builds its own dependency graph; `Client` uses sync.Mutex where needed |
| TypeScript/Python parsers produce inaccurate AnalyzedFile | Validate parser output against known projects; unit test each extraction |
| Regex-based parsers miss edge cases | Document known limitations; track accuracy metrics; plan tree-sitter upgrade path |
| Plugin YAML rules are too limited | Design rule types to cover 80% of use cases; WASM covers the remaining 20% |

---

## Execution Order

```
Week 1:   Tasks 1-4    (SDK skeleton, all functions, Client, options)
Week 2:   Tasks 5-7    (language detection, TS parser, Py parser)
Week 3:   Tasks 8-10   (language profiles, plugin types, rule engine)
Week 4:   Tasks 11-12  (plugin loading, wire into ScoreService)
Week 5-6: Tasks 13-14  (integration tests, calibration)
Week 7:   Task 15      (final verification, benchmarks)
```
