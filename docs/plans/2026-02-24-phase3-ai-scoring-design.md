# Phase 3: AI-Specific Scoring System

## Problem

OpenKraft claims to measure "AI-Readiness" but 90% of its scoring (architecture, conventions, patterns, tests, completeness) measures generic software quality. Only `ai_context` (10% weight) is genuinely AI-specific. The name generates expectations the tool doesn't meet.

**Evidence:**
- Running OpenKraft against itself with `cli-tool` config scores 42/100 — most deductions come from checks that don't measure whether an AI agent can work effectively with the code.
- Existing categories map directly to traditional software engineering dimensions, not AI-agent operational needs.

## Solution

Replace the 6 Phase 1 categories with 6 new categories structured around **how AI coding agents actually operate** — backed by empirical research from 2025-2026.

## Research Foundation

| Source | Key Finding | Impact on Design |
|--------|-------------|------------------|
| CodeScene arXiv:2601.02200 (n=5000, 7 LLMs) | Code health predicts AI refactoring success. 5 specific smells identified as most damaging. | `code_health` category with the exact 5 smells |
| arXiv:2602.07882 (Feb 2026) | Traditional cyclomatic complexity doesn't predict LLM performance. Semantic linearity does. | Focus on nesting/branching, not cyclomatic complexity |
| Cognition SWE-grep | Agents spend >60% of first turn searching for context. 8 parallel searches per turn. | `discoverability` as highest-weight category |
| Cursor best practices | "Canonical examples over exhaustive docs." Typed languages + linters = clear verification signals. | `context_quality.canonical_examples`, `verifiability.type_safety_signals` |
| Factory.ai Agent Readiness | 8-pillar framework with build reproducibility as a full pillar. | `verifiability.build_reproducibility` |
| Anthropic Claude Code | CLAUDE.md is the primary agent-codebase interface. Context engineering > prompt engineering. | `context_quality.ai_context_files` |
| Martin Fowler/Boeckeler | "Keep data structures stable and define/enforce module boundaries." | `structure` category retained |
| GitHub Copilot research | Code standardization within an org is key determinant of AI value. Readability +3.62%. | `predictability.consistent_patterns` |
| Feitelson et al. IEEE TSE 2020 | More words in identifier name = more concepts captured = higher descriptiveness. | `discoverability.naming_uniqueness` uses CamelCase word count |
| Arnaoudova et al. ESE 2014 | 17 linguistic antipatterns detectable by static analysis correlate with code quality. | Naming quality scoring heuristics |

---

## Category Structure

| Category | Weight | What it measures | Why it matters for AI agents |
|----------|--------|------------------|------------------------------|
| **code_health** | 0.25 | Code smells that empirically break LLM refactoring | CodeScene proved this is the #1 predictor of AI success |
| **discoverability** | 0.20 | Can an agent find what it needs via grep/glob? | Agents spend >60% of initial work searching (SWE-grep) |
| **structure** | 0.15 | Does the project have expected layers and modules? | Module boundaries enable multi-hop reasoning (Sourcegraph) |
| **verifiability** | 0.15 | Can an agent confirm its changes are correct? | Test loop is the agent's verification mechanism (Cursor) |
| **context_quality** | 0.15 | Is there explicit guidance for the agent? | Context files are the primary agent-codebase interface (Anthropic) |
| **predictability** | 0.10 | Does code behave as its name/structure suggests? | Reduces agent reasoning errors and hallucinations |

---

## Sub-Metrics

### 1. code_health (0.25) — 5 sub-metrics, 100 points

Based directly on the CodeScene paper's top 5 damaging smells for LLMs.

| Sub-metric | Points | Detection | Thresholds | Evidence |
|------------|--------|-----------|------------|----------|
| `function_size` | 20 | Lines per function via AST `*ast.FuncDecl` position math | <=50 full, 51-100 partial, >100 zero | SWE-Agent view window ~100 lines |
| `file_size` | 20 | Lines per `.go` file | <=300 full, 301-500 partial, >500 zero | Factory.ai context window research |
| `nesting_depth` | 20 | Max nesting depth per function via AST block walk | <=3 full, 4 partial, >=5 zero | CodeScene: root node in 5/5 decision trees |
| `parameter_count` | 20 | Parameter count per `*ast.FuncDecl.Type.Params` | <=4 full, 5-6 partial, >=7 zero | CodeScene: top-5 damaging smells |
| `complex_conditionals` | 20 | Count `&&`/`||` operators in `*ast.IfStmt.Cond` | <=2 full, 3 partial, >=4 zero | CodeScene + LM-CC paper |

**Implementation:** New scorer `scoring/code_health.go`. Requires enhanced parser output with line numbers and parameter lists.

**Parser enhancement needed:** The current `go_parser.go` extracts function names and receivers. Must add:
- `LineStart`, `LineEnd` per function (for size calculation)
- `Params []Param` with `Name` and `Type` (for parameter count)
- `Returns []string` (for pattern consistency analysis)

```go
type Function struct {
    Name      string  `json:"name"`
    Receiver  string  `json:"receiver,omitempty"`
    Exported  bool    `json:"exported"`
    LineStart int     `json:"line_start"`
    LineEnd   int     `json:"line_end"`
    Params    []Param `json:"params,omitempty"`
    Returns   []string `json:"returns,omitempty"`
}

type Param struct {
    Name string `json:"name"`
    Type string `json:"type"`
}
```

**Nesting depth detection:** Walk the AST within each `*ast.FuncDecl.Body`, incrementing depth on `*ast.IfStmt`, `*ast.ForStmt`, `*ast.RangeStmt`, `*ast.SwitchStmt`, `*ast.SelectStmt`, `*ast.TypeSwitchStmt`. Track max depth per function.

**Complex conditionals detection:** Walk `*ast.BinaryExpr` nodes within `*ast.IfStmt.Cond`, count `token.LAND` (&&) and `token.LOR` (||) operators.

### 2. discoverability (0.20) — 4 sub-metrics, 100 points

Measures how effectively an agent can navigate the codebase via search.

| Sub-metric | Points | Detection | Scoring | Evidence |
|------------|--------|-----------|---------|----------|
| `naming_uniqueness` | 25 | Composite: CamelCase word count + vocabulary specificity + Shannon entropy | See composite scoring below | Feitelson 2020, Arnaoudova 2014 |
| `file_naming_conventions` | 25 | Ratio of `.go` files with conventional suffixes (`_test`, `_handler`, `_service`, `_repository`, `_model`) | ratio * 25 | SWE-Agent: `find_file` is top-3 operation |
| `predictable_structure` | 25 | Structural variance between same-level packages/modules | Low variance = high score | Cursor: "predictable file structure" |
| `dependency_direction` | 25 | Import violations: adapter→adapter, domain→application | Migrated from Phase 1 `architecture.dependency_direction` | Sourcegraph: graph traversability |

**Naming uniqueness composite scoring (no LLM required):**

Three deterministic signals combined:

1. **Word count score (40%):** Split CamelCase identifiers (using `fatih/camelcase` algorithm). >=3 words = 1.0, 2 words = 0.8, 1 long word = 0.5, 1 short vague word = 0.2.

2. **Vocabulary specificity (30%):** Curated sets of vague words (`Handle`, `Process`, `Data`, `Run`, `Do`, `Execute`, `Manage`, `Util`, `Helper`, `Info`, `Stuff`, `Thing`, `Item`, `Object`, `Temp`) and specific action verbs (`Create`, `Delete`, `Update`, `Validate`, `Calculate`, `Parse`, `Serialize`, `Publish`, `Authenticate`). Score = ratio of non-vague words in name. A name can use `Handle` if combined with specifics: `HandlePaymentWebhook` = 2/3 specific = 0.67.

3. **Shannon entropy (30%):** Measures uniqueness across the entire codebase. If 15 functions share the name `Handle`, entropy is low (grep produces noise). If each has a unique name, entropy is high (grep is precise). Normalized to [0,1] via `entropy / log2(totalFunctions)`.

**Predictable structure detection:** For each pair of same-level packages (e.g., all packages under `internal/`), compute the set of file suffixes present. Measure Jaccard similarity between sets. Average Jaccard across all pairs = structure consistency score.

### 3. structure (0.15) — 4 sub-metrics, 100 points

Validates that the project has the architectural skeleton its type requires.

| Sub-metric | Points | Detection | Scoring | Evidence |
|------------|--------|-----------|---------|----------|
| `expected_layers` | 25 | Check presence of directories per project type | Ratio of present/expected | Martin Fowler: "define/enforce module boundaries" |
| `expected_files` | 25 | Per module, check for expected file types | Avg file presence ratio across modules | Factory.ai: Task Discovery pillar |
| `interface_contracts` | 25 | Count interfaces in domain/ports vs. concrete cross-package deps | Ratio: interfaces / (interfaces + concrete deps) | Augment: "code relationships are critical" |
| `module_completeness` | 25 | Golden module comparison (from Phase 1) | Avg completeness ratio across modules | OpenKraft core value proposition |

**Expected layers per project type:**
- `api`: `cmd/`, `internal/domain/`, `internal/application/`, `internal/adapters/`
- `cli-tool`: `cmd/`, `internal/`
- `library`: `pkg/` or exported packages, `internal/`
- `microservice`: `cmd/`, `internal/{module}/domain/`, `internal/{module}/application/`

**Expected files per module type:**
- `api` module: model/entity, service, handler, repository, ports
- `cli-tool` module: command, service
- `library` module: exported types, tests

These are configurable per project type via the `.openkraft.yaml` config system from Phase 2.

### 4. verifiability (0.15) — 4 sub-metrics, 100 points

Measures the quality of the agent's feedback loop: can it run tests, get clear results, and trust the build?

| Sub-metric | Points | Detection | Scoring | Evidence |
|------------|--------|-----------|---------|----------|
| `test_presence` | 25 | Ratio of `.go` files with corresponding `_test.go` | ratio * 25 | Cursor: TDD is the recommended agent workflow |
| `test_naming` | 25 | Pattern `Test<Function>_<Scenario>` with descriptive subtests | Ratio of well-named tests / total tests | SWE-Agent reads test names to understand scope |
| `build_reproducibility` | 25 | Presence of: go.sum, Makefile/Taskfile, CI config | Each item = points (go.sum=10, Makefile=8, CI=7) | Factory.ai: Build System is a full pillar |
| `type_safety_signals` | 25 | Linter config presence + `interface{}`/`any` ratio + unchecked type assertions | Composite: config=10, low any=10, checked assertions=5 | Cursor: "typed languages + linters = clear verification signals" |

**Test naming detection:** Parse `_test.go` files via AST. For each `Test*` function, check if the name follows `Test<Function>_<Scenario>` pattern (contains underscore after `Test` prefix + a known function name). For table-driven tests, check for `t.Run` calls with string literals.

**Type assertion safety:** Walk AST for `*ast.TypeAssertExpr`. Check if used in assignment with 2 LHS values (`v, ok := x.(T)`) vs. 1 LHS (`v := x.(T)`). Ratio of safe/total assertions.

### 5. context_quality (0.15) — 4 sub-metrics, 100 points

Measures the explicit guidance available to an AI agent.

| Sub-metric | Points | Detection | Scoring | Evidence |
|------------|--------|-----------|---------|----------|
| `ai_context_files` | 30 | CLAUDE.md, AGENTS.md, .cursorrules, copilot-instructions.md | Presence + min size (>100 bytes) + has headers | Anthropic, Cursor, Factory.ai |
| `package_documentation` | 25 | `// Package foo ...` doc comment per package | Ratio of documented packages / total | GitHub Copilot: documentation density correlates with effectiveness |
| `architecture_docs` | 20 | README.md >500 bytes, docs/ directory, ADR files | Tiered: README=8, docs/=7, ADRs=5 | Addy Osmani: "focus on what and why" |
| `canonical_examples` | 25 | `example_test.go` files, or CLAUDE.md referencing files as patterns | Count of example files + CLAUDE.md pattern refs | Cursor: "canonical examples over exhaustive docs" |

**Canonical example detection:** Two signals:
1. Files matching `*example*_test.go` or `*_example.go` patterns
2. In CLAUDE.md/AGENTS.md, regex for phrases like "see `path/to/file`", "refer to `file.go`", "canonical example", "reference implementation"

### 6. predictability (0.10) — 4 sub-metrics, 100 points

Measures whether code behaves as its name and structure suggest — reducing agent reasoning errors.

| Sub-metric | Points | Detection | Scoring | Evidence |
|------------|--------|-----------|---------|----------|
| `self_describing_names` | 25 | Exported functions with verb+noun pattern | Ratio of verb-prefixed exported functions / total exported | GitHub Copilot: readability +3.62% |
| `explicit_dependencies` | 25 | Zero globals: count mutable `var` at package level + `init()` functions | 0 globals+inits = full, 1-2 = partial, 3+ = zero | Cursor: "predictable file structure" |
| `error_message_quality` | 25 | Composite: wrapping ratio + context richness + convention compliance | See composite below | Factory.ai: Debugging & Observability pillar |
| `consistent_patterns` | 25 | Signature consistency within function role groups | Avg consistency across handler/service/repo groups | Cursor: "module system consistency" |

**Error message quality composite (AST-based, no LLM):**

Four signals from parsing `fmt.Errorf` and `errors.New` calls:

1. **Wrapping ratio (40%):** % of error returns using `fmt.Errorf("...: %w", err)`. Detected by checking `*ast.CallExpr` where `Fun` is `fmt.Errorf` and format string contains `%w`.

2. **Context richness (30%):** % of error messages with variable interpolation (`%s`, `%v`, `%d`), not just static strings. An error like `errors.New("failed")` scores 0, while `fmt.Errorf("creating payment %s: %w", id, err)` scores 1.

3. **Convention compliance (20%):** Go error conventions — starts lowercase, no trailing period, uses `:` separator for context chain. Detected by inspecting the `*ast.BasicLit` format string.

4. **Sentinel presence (10%):** Package-level `var Err* = errors.New(...)` sentinel errors that agents can grep for. Detected via `*ast.ValueSpec` where name matches `Err*` prefix and value is `errors.New`.

**Consistent patterns detection:**

1. Classify functions by role using file suffix: `*_handler.go` = handler, `*_service.go` = service, `*_repository.go` = repository.
2. Extract normalized signatures: `(paramCount, returnCount, hasContext, hasError)`.
3. For each role group, find the modal signature (most common shape).
4. Consistency = fraction of functions matching the mode.

---

## Migration from Phase 1/2

### Checks that migrate to new categories

| Phase 1 check | Old category | New category.sub_metric |
|----------------|-------------|------------------------|
| `dependency_direction` | architecture | discoverability.dependency_direction |
| `layer_separation` | architecture | structure.expected_layers (absorbed) |
| `consistent_module_structure` | architecture | discoverability.predictable_structure |
| `module_boundary_clarity` | architecture | structure.expected_layers (absorbed) |
| `architecture_documentation` | architecture | context_quality.architecture_docs |
| `naming_consistency` | conventions | discoverability.naming_uniqueness (enhanced) |
| `error_handling` | conventions | predictability.error_message_quality (enhanced) |
| `import_ordering` | conventions | Removed (low AI-relevance, cosmetic) |
| `file_organization` | conventions | discoverability.file_naming_conventions |
| `code_style` | conventions | predictability.consistent_patterns (absorbed) |
| `entity_patterns` | patterns | predictability.consistent_patterns (absorbed) |
| `repository_patterns` | patterns | predictability.consistent_patterns (absorbed) |
| `service_patterns` | patterns | predictability.consistent_patterns (absorbed) |
| `port_patterns` | patterns | structure.interface_contracts |
| `handler_patterns` | patterns | predictability.consistent_patterns (absorbed) |
| `unit_test_presence` | tests | verifiability.test_presence |
| `integration_tests` | tests | verifiability.test_presence (absorbed) |
| `test_helpers` | tests | Removed (low signal — presence of helpers != quality) |
| `test_fixtures` | tests | Removed (low signal) |
| `ci_config` | tests | verifiability.build_reproducibility |
| `claude_md` | ai_context | context_quality.ai_context_files |
| `cursor_rules` | ai_context | context_quality.ai_context_files |
| `agents_md` | ai_context | context_quality.ai_context_files |
| `openkraft_dir` | ai_context | Removed (self-referential, not a real quality signal) |
| `file_completeness` | completeness | structure.module_completeness |
| `structural_completeness` | completeness | structure.expected_layers (absorbed) |
| `documentation_completeness` | completeness | context_quality.package_documentation |

### Checks removed entirely

| Check | Reason |
|-------|--------|
| `import_ordering` | Cosmetic. Doesn't affect agent navigation or success. |
| `test_helpers` | Presence of helper files doesn't indicate test quality. |
| `test_fixtures` | Presence of fixture dirs doesn't indicate test quality. |
| `openkraft_dir` | Self-referential — measuring our own metadata isn't a quality signal. |
| `getQuerier` pattern | Too opinionated. Only applies to one specific database pattern. |

### New checks (not in Phase 1)

| Check | Category | Why it's new |
|-------|----------|-------------|
| `function_size` | code_health | Never measured before. Top predictor per CodeScene paper. |
| `file_size` | code_health | Never measured before. Context window fitness. |
| `nesting_depth` | code_health | Never measured before. #1 smell in CodeScene decision trees. |
| `parameter_count` | code_health | Never measured before. Top-5 damaging smell. |
| `complex_conditionals` | code_health | Never measured before. LLMs misinterpret compound conditions. |
| `naming_uniqueness` | discoverability | Phase 1 had basic naming_consistency. This is a 3-signal composite with entropy. |
| `test_naming` | verifiability | Phase 1 checked test presence, not test quality. |
| `build_reproducibility` | verifiability | New. Agents need deterministic environments. |
| `type_safety_signals` | verifiability | New. Typed errors help agents verify correctness. |
| `canonical_examples` | context_quality | New. Cursor explicitly recommends this pattern. |
| `self_describing_names` | predictability | New. Verb+noun pattern analysis. |
| `explicit_dependencies` | predictability | New. Global state detection. |
| `error_message_quality` | predictability | Phase 1 had basic error_handling. This is a 4-signal composite. |
| `consistent_patterns` | predictability | Phase 1 checked specific patterns. This measures signature consistency. |

---

## Solving the Three Problems

### Problem 1: Fragile heuristics

**Solution:** Every "hard" metric is a composite of 3-4 deterministic signals, not a single heuristic. This follows CodeScene's approach (CodeHealth = combination of 25 smells, not one).

- `naming_uniqueness` = word count (40%) + vocabulary specificity (30%) + Shannon entropy (30%)
- `error_message_quality` = wrapping ratio (40%) + context richness (30%) + convention compliance (20%) + sentinel presence (10%)
- `consistent_patterns` = signature shape extraction + modal consistency ratio

No signal alone is fragile. Combined, they're robust. A name can score low on word count but high on specificity and entropy. Gaming one signal doesn't game the composite.

### Problem 2: No empirical validation

**Solution in 3 phases:**

**Phase A — Free, immediate:** Score the 12 SWE-bench Python repos (and Go repos from Multi-SWE-bench) with OpenKraft. Cross-reference with published agent resolve rates. This is the first public dataset connecting code quality scores to agent success. N=12 is small but publishable and nobody else has done it.

**Phase B — The `benchmark` command:**
```
openkraft benchmark [path] [--tasks N] [--model MODEL] [--dry-run]
```

Uses **PR-inversion**: walks git history, finds merged PRs with test changes, reverts code (keeps tests), asks an agent to make tests pass. The repo's own test suite is the ground truth.

Cost: ~$0.14/task with Sonnet, ~$0.03/task with Haiku. A 10-task benchmark costs $1.40.

The benchmark command is a separate Phase (Phase 4) but the scoring system is designed to support it from day 1.

**Phase C — Community dataset:** Each benchmark run optionally reports anonymized results: `{score, categories, success_rate, model, repo_size}`. Over time, this builds the correlation evidence and enables weight recalibration.

### Problem 3: Easy metrics are gameable

**Solution:** Three layers of anti-gaming:

1. **code_health is the floor, not the ceiling.** It's 25% — important but not sufficient. You can't score 100 just by having small functions.

2. **Composite metrics resist individual optimization.** Making functions small (high function_size) but naming them `Handle1`, `Handle2` tanks naming_uniqueness. Splitting files (high file_size) but losing structure tanks predictable_structure.

3. **The benchmark command is the ultimate validator.** Static metrics say the code looks AI-friendly. The benchmark proves it actually is. If someone games the metrics, their benchmark success rate will expose the disconnect.

---

## Config System Integration

Phase 3 categories replace Phase 1 categories in the config system. The `.openkraft.yaml` schema stays the same but valid category/sub-metric names change:

**Valid categories (Phase 3):**
`code_health`, `discoverability`, `structure`, `verifiability`, `context_quality`, `predictability`

**Valid sub-metrics (Phase 3):**
`function_size`, `file_size`, `nesting_depth`, `parameter_count`, `complex_conditionals`, `naming_uniqueness`, `file_naming_conventions`, `predictable_structure`, `dependency_direction`, `expected_layers`, `expected_files`, `interface_contracts`, `module_completeness`, `test_presence`, `test_naming`, `build_reproducibility`, `type_safety_signals`, `ai_context_files`, `package_documentation`, `architecture_docs`, `canonical_examples`, `self_describing_names`, `explicit_dependencies`, `error_message_quality`, `consistent_patterns`

**Project type defaults update:**

### `api` (default)
| Category | Weight |
|----------|--------|
| code_health | 0.25 |
| discoverability | 0.20 |
| structure | 0.15 |
| verifiability | 0.15 |
| context_quality | 0.15 |
| predictability | 0.10 |

No skipped sub-metrics.

### `cli-tool`
| Category | Weight |
|----------|--------|
| code_health | 0.25 |
| discoverability | 0.20 |
| structure | 0.10 |
| verifiability | 0.20 |
| context_quality | 0.15 |
| predictability | 0.10 |

Skipped: none (Phase 3 metrics are universal).

### `library`
| Category | Weight |
|----------|--------|
| code_health | 0.25 |
| discoverability | 0.20 |
| structure | 0.10 |
| verifiability | 0.25 |
| context_quality | 0.10 |
| predictability | 0.10 |

Skipped: none.

### `microservice`
| Category | Weight |
|----------|--------|
| code_health | 0.25 |
| discoverability | 0.20 |
| structure | 0.20 |
| verifiability | 0.15 |
| context_quality | 0.10 |
| predictability | 0.10 |

Skipped: none.

---

## Parser Enhancement Summary

The current `go_parser.go` must be enhanced to support Phase 3 metrics. New data extracted per file:

| Data | Used by | AST source |
|------|---------|------------|
| Function line start/end | code_health.function_size | `fset.Position(decl.Pos()).Line`, `fset.Position(decl.End()).Line` |
| Function parameters (name + type) | code_health.parameter_count, predictability.consistent_patterns | `decl.Type.Params.List` |
| Function returns (types) | predictability.consistent_patterns | `decl.Type.Results.List` |
| Max nesting depth per function | code_health.nesting_depth | Custom AST walk within `decl.Body` |
| Max conditional complexity per function | code_health.complex_conditionals | Walk `*ast.BinaryExpr` in `*ast.IfStmt.Cond` |
| Error creation calls | predictability.error_message_quality | `*ast.CallExpr` matching `fmt.Errorf` / `errors.New` |
| Package-level `var` declarations | predictability.explicit_dependencies | `*ast.GenDecl` with `token.VAR` at file scope |
| `init()` functions | predictability.explicit_dependencies | `*ast.FuncDecl` where `Name.Name == "init"` |
| Type assertions (safe vs unsafe) | verifiability.type_safety_signals | `*ast.TypeAssertExpr` context check |
| `t.Run` calls in test files | verifiability.test_naming | `*ast.CallExpr` in `_test.go` files |
| `// Package ...` doc comments | context_quality.package_documentation | `*ast.File.Doc` |

**Approach:** Enhance the existing `AnalyzedFile` struct in `domain/model.go` with new fields. The parser populates them. Scorers consume them as pure functions. No new adapter needed.

```go
// Enhanced fields on AnalyzedFile
type AnalyzedFile struct {
    // ... existing fields ...
    Functions      []Function     `json:"functions,omitempty"`       // ENHANCED: was []string
    PackageDoc     bool           `json:"package_doc,omitempty"`     // NEW
    InitFunctions  int            `json:"init_functions,omitempty"`  // NEW
    GlobalVars     []string       `json:"global_vars,omitempty"`     // NEW
    ErrorCalls     []ErrorCall    `json:"error_calls,omitempty"`     // NEW
    TypeAssertions []TypeAssert   `json:"type_assertions,omitempty"` // NEW
}

type Function struct {
    Name        string   `json:"name"`
    Receiver    string   `json:"receiver,omitempty"`
    Exported    bool     `json:"exported"`
    LineStart   int      `json:"line_start"`
    LineEnd     int      `json:"line_end"`
    Params      []Param  `json:"params,omitempty"`
    Returns     []string `json:"returns,omitempty"`
    MaxNesting  int      `json:"max_nesting"`
    MaxCondOps  int      `json:"max_cond_ops"` // max &&/|| in a single if
}

type Param struct {
    Name string `json:"name"`
    Type string `json:"type"`
}

type ErrorCall struct {
    Type       string `json:"type"`        // "fmt.Errorf" or "errors.New"
    HasWrap    bool   `json:"has_wrap"`     // contains %w
    HasContext bool   `json:"has_context"`  // has variable interpolation
    Format     string `json:"format"`       // the format string
}

type TypeAssert struct {
    Safe bool `json:"safe"` // has comma-ok pattern
}
```

**Breaking change:** `Functions` changes from `[]string` to `[]Function`. All 6 Phase 1 scorers that read `Functions` must be updated. Since Phase 3 replaces all scorers anyway, this is acceptable.

---

## Implementation Scope

### New files
- `internal/domain/scoring/code_health.go` + test
- `internal/domain/scoring/discoverability.go` + test
- `internal/domain/scoring/structure.go` + test (replaces completeness.go)
- `internal/domain/scoring/verifiability.go` + test
- `internal/domain/scoring/context_quality.go` + test (replaces ai_context.go)
- `internal/domain/scoring/predictability.go` + test

### Modified files
- `internal/domain/model.go` — enhanced AnalyzedFile, Function type
- `internal/adapters/outbound/parser/go_parser.go` — extract all new AST data
- `internal/adapters/outbound/parser/go_parser_test.go` — test new extractions
- `internal/application/score_service.go` — call new scorers
- `internal/application/score_service_test.go` — update for new categories
- `internal/domain/config.go` — update valid categories/sub-metrics
- `internal/domain/config_test.go` — update validation tests
- `internal/adapters/outbound/tui/renderer.go` — update category rendering
- `internal/adapters/outbound/tui/renderer_test.go`
- `tests/e2e/e2e_test.go` — update expected categories
- Test fixtures — may need updates for new metrics

### Deleted files
- `internal/domain/scoring/architecture.go` + test
- `internal/domain/scoring/conventions.go` + test
- `internal/domain/scoring/patterns.go` + test
- `internal/domain/scoring/completeness.go` + test
- `internal/domain/scoring/ai_context.go` + test
- `internal/domain/scoring/tests.go` + test

### External dependencies
- `github.com/fatih/camelcase` — CamelCase splitting for naming analysis (zero deps, 1 file)

---

## Verification

1. `make test` — all packages pass
2. `make build` — binary builds
3. `openkraft score testdata/go-hexagonal/perfect` — produces scores across 6 new categories
4. `openkraft score testdata/go-hexagonal/perfect --json` — JSON output with new category names
5. `openkraft score .` — score OpenKraft itself, expect improvement with cli-tool config
6. Backward compatibility: `.openkraft.yaml` with old category names in `weights` or `skip` → clear migration error message
7. Each new sub-metric produces actionable issues that an AI agent can fix via MCP
