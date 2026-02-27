# Audit: code_health — Ollama Deep Dive

> **Date:** 2026-02-27
> **Target:** ollama/ollama (Go, ML inference engine)
> **Category:** code_health
> **Score:** 74/100
> **Methodology:** 4 independent agents auditing in parallel to avoid bias
> **Previous audit:** 2026-02-26-audit-multirepo-code-health.md (10-repo benchmark)

---

## 1. Summary

Ollama scores 74/100 on code_health. The sub-metrics give it 99/100 (near-perfect ratios), but the severity penalty deducts 25 points. We audited whether this penalty is fair by sending 4 independent agents to examine different angles: false positives, ML domain patterns, penalty math, and generated code detection.

**Headline finding: The score is mathematically correct but ~5-8 points too severe.** Approximately 56-71% of error-level issues are false positives from domain-specific patterns (ML model code, table-driven tests, CGo bindings). The penalty formula's linear extrapolation is too aggressive for debt ratios above 10%. A fair score would be 79-82, aligning with SonarQube B-grade (75-85 range).

## 2. Target Profile

| Metric | Value |
|--------|-------|
| Functions | 5,680 |
| Files | 673 |
| Total issues | 1,206 |
| Error-level | 117 (2.06% of functions) |
| Warning-level | 479 (8.43%) |
| Info-level | 610 (10.74%) |
| Weighted debt ratio | 16.76% |
| Sub-metric base | 99/100 |
| Severity penalty | -25 |
| Final score | 74 |

### Sub-metric breakdown

| Sub-metric | Score | Detail |
|------------|-------|--------|
| function_size | 20/20 | 98% of 5680 functions within limits |
| file_size | 19/20 | 93% of 673 files within limits |
| nesting_depth | 20/20 | 100% within limits |
| parameter_count | 20/20 | 100% within limits |
| complex_conditionals | 20/20 | 100% within limits |

### Top offending directories

| Directory | Issues | % of total |
|-----------|--------|------------|
| x/imagegen | 161 | 13.3% |
| model/models | 124 | 10.3% |
| model/parsers | 69 | 5.7% |
| cmd/config | 48 | 4.0% |
| model/renderers | 43 | 3.6% |

## 3. False Positives Found

### 3.1 Table-driven test functions (~35-40 error-level FPs)

**Agent 1 finding.** Go's idiomatic table-driven test pattern produces functions that are 300-2000+ lines but consist almost entirely of data declarations (struct literals) with trivial test logic (a `t.Run` loop at the bottom).

Examples:
- `discover/cpu_linux_test.go:9` — `TestLinuxCPUDetails` 2076 lines (98% embedded `/proc/cpuinfo` text)
- `tools/tools_test.go:35` — `TestParser` 754 lines (table of input/expected pairs)
- `model/renderers/deepseek3_test.go:11` — `TestDeepSeekRenderer` 1033 lines (message/expected pairs)
- `server/routes_generate_test.go` — multiple 400-700 line test functions

**Verdict:** These are NOT maintainability problems. Splitting table-driven tests reduces readability and violates Go conventions. Our 2x test file multiplier is insufficient for data-heavy tests.

### 3.2 ML model constructors and Forward methods (~15-20 error-level FPs)

**Agent 2 finding.** Model `New()` constructors (164-168 lines) are flat config deserialization — each line maps one GGUF key to one struct field. `Forward()` methods are direct translations of research paper pseudocode with sequential tensor operations and minimal branching.

Examples:
- `model/models/qwen3next/model.go:453` — `New` 168 lines (39 config fields)
- `model/models/qwen3next/deltanet.go:72` — `Forward` 162 lines (Gated DeltaNet algorithm)
- `model/models/nemotronh/model.go:144` — `New` 164 lines
- `model/models/lfm2/model.go:201` — `New` 166 lines

**Verdict:** Domain pattern. Sequential config reads and tensor operation chains cannot be meaningfully decomposed without creating artificial abstractions.

### 3.3 CGo/FFI bindings (~23 issues, all FPs)

**Agent 2 finding.** `x/imagegen/mlx/mlx.go` has 197 functions, nearly all 3-5 line CGo wrappers around Apple's MLX C API. Parameter counts (8-12) are dictated by the C API, not Go design choices.

Examples:
- `x/mlxrunner/mlx/ops_extra.go:70` — `GatherQMM` 11 params (mirrors C signature)
- `x/mlxrunner/mlx/ops_extra.go:54` — `QuantizedMatmul` 8 params
- `ml/backend/ggml/ggml.go:1663` — `Conv3D` 12 params

**Verdict:** Domain pattern. FFI wrappers must match the underlying C API.

### 3.4 Parser state machines and renderers (~80 issues, mostly FPs)

**Agent 2 finding.** Parsers (`model/parsers/`) use `eat()` functions (140-170 lines) that are streaming FSM implementations. Renderers (`model/renderers/`) have `Render()` functions that build prompt strings with model-specific tokens.

**Caveat:** Agent 2 found real code duplication between cogito and deepseek3 parsers (~60% structural similarity, copy-paste). Our tool flags the wrong thing (function length) but the real issue (duplication) exists and goes undetected.

### 3.5 HTTP handlers (~10-15 FPs, mixed)

**Agent 1 finding.** `GenerateHandler` (482 lines) and `ChatHandler` (519 lines) are large but follow linear request-processing pipelines with early-return guards. They could benefit from extraction but error-level severity is too aggressive. `cmd.go` handlers (200-270 lines) are false positives for CLI commands.

**Partial exception:** These are the closest thing to legitimate issues. Warning-level would be appropriate for the server handlers.

### Summary: false positive rate

| Category | Error FPs | All FPs | FP Rate |
|----------|-----------|---------|---------|
| Table-driven tests | 35-40 | 35-40 | 100% |
| ML constructors/Forward | 15-20 | 100+ | 100% |
| CGo/FFI bindings | 0 | 23 | 100% |
| Parsers/renderers | 5-8 | 80+ | ~85% |
| HTTP handlers | 10-15 | 10-15 | ~70% |
| **Total** | **65-83** | **250-260** | — |
| **Of 117 error-level** | **56-71%** | — | — |

## 4. Penalty Formula Analysis

**Agent 3 finding.** The severity penalty formula was calibrated for debt ratios of 3-7% (clean OSS projects). Ollama's 16.76% debt ratio is 2.5-5x higher than the calibration range, and the linear formula extrapolates aggressively.

### The math

```
weight     = 117×3.0 + 479×1.0 + 610×0.2 = 952
debtRatio  = 952 / 5680 = 0.1676
penalty    = round(0.1676 × 150) = 25
```

### SonarQube comparison

| SonarQube Grade | Debt Ratio | Approx Score |
|-----------------|------------|--------------|
| A | ≤5% | 90-100 |
| B | ≤10% | 75-85 |
| C | ≤20% | 60-75 |

Ollama at 16.8% sits at the B/C boundary. SonarQube would likely give it a low B. Our score of 74 puts it just below the B range — slightly harsh.

### Double punishment

Sub-metrics deducted 1 point (100→99) and the severity penalty deducted 25 points — both from the same issues. The overlap is only 1 point, but the design problem is that the penalty completely dominates: 5 carefully designed sub-metrics contribute almost nothing to score differentiation.

### Calibration options

| Approach | Ollama Score | Clean Repo Impact |
|----------|-------------|-------------------|
| Current (scale=150) | 74 | No change |
| Reduce scale to 120 | 79 | -1 to -2 points |
| Log compression | 79 | -1 point |

## 5. Generated Code Detection

**Agent 4 finding.** Only 8 of 1206 issues (0.7%) come from code that should be excluded — a vendored `app/dialog/` directory with its own LICENSE. Current detection (comment markers + filename conventions) is working correctly. No missing generated file patterns found.

## 6. Legitimate Issues in Ollama

Not everything is a false positive. Real code health concerns include:

| File | Issue | Severity |
|------|-------|----------|
| `server/routes.go` | 2657 lines, handlers of 500+ lines | Error — genuinely oversized |
| `app/ui/ui.go` | `chat()` function 628 lines | Error — needs extraction |
| `cmd/cmd.go` | 2412 lines, `NewCLI` 271 lines | Error — god file |
| `x/cmd/run.go` | `GenerateInteractive` 422 lines | Error — complex CLI logic |
| `readline/readline.go` | `Readline` 276 lines | Warning — should be refactored |
| parsers cogito ≈ deepseek3 | Cross-file duplication | Not detected — new capability needed |

## 7. Recommended Improvements for openkraft

### Priority 1: Table-driven test detection
Tests with >80% struct literal data + `t.Run` loop should not generate error-level issues. Would eliminate ~40 of 117 errors across ollama.

### Priority 2: ML domain patterns
- Config deserialization `New(config)` with flat field mapping → relaxed threshold (~200 lines)
- Sequential tensor operation `Forward()` chains → score by cyclomatic complexity, not LOC
- CGo files (`import "C"`) → raise parameter threshold to 12

### Priority 3: Penalty calibration
Reduce `severityPenaltyScale` from 150 to 120 (simple) or introduce logarithmic compression (principled). Moves ollama from 74 to ~79, aligning with SonarQube B-grade.

### Priority 4: Cross-file duplication detection (new capability)
The parsers and renderers have significant structural duplication that our tool doesn't detect at all. This is where real issues exist but go unreported.

## 8. Expected Score After Fixes

If all recommended improvements were implemented:
- ~343 domain-pattern issues reclassified → reduced severity weight
- Scale adjusted to 120 → less aggressive extrapolation
- Estimated new score: **82-85** (SonarQube B-grade, reflecting real issues in routes.go/cmd.go while not penalizing domain patterns)

## 9. Audit Methodology

Four independent agents analyzed the same codebase simultaneously to avoid confirmation bias:

1. **Agent 1 (False Positives):** Read flagged files, categorized each error-level issue as FP or legitimate
2. **Agent 2 (ML Patterns):** Focused on model/, x/imagegen/, x/mlxrunner/ directories
3. **Agent 3 (Penalty Math):** Mathematical analysis of the formula calibration
4. **Agent 4 (Generated Code):** Searched for undetected generated/vendored code

All four agents converged independently on the same conclusion: the score is mechanically correct but ~5-8 points too severe due to domain-specific false positives and aggressive penalty extrapolation.
