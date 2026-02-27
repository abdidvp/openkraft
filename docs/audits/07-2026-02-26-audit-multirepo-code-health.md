# Audit: code_health — Multi-Repo Validation

> **Date:** 2026-02-26
> **Targets:** chi, fiber, bubbletea, wild-workouts-go-ddd-example, iris
> **Category:** code_health
> **Score range observed:** 95–100
> **Expected useful range:** 70–100
> **Methodology:** Independent Go AST analysis tool compared against openkraft output

---

## 1. Summary

We audited code_health scoring against 5 popular Go repos of varying size and architecture: a minimal router (chi, 352 funcs), two large frameworks (fiber 3286 funcs, iris 3970 funcs), a TUI library (bubbletea 618 funcs), and a DDD microservice (wild-workouts 332 funcs).

**Headline finding: The score is mechanically accurate but not useful for comparison.** All repos score 95–100 despite dramatically different code quality profiles. Fiber has a 798-line function and 49 oversized files but still scores 95. Iris has 115 functions in the silent penalty zone and 75 issues but scores 99. The 20-point sub-metric scale with percentage-based scoring compresses all real-world repos into a 5-point band.

## 2. Target Project Profiles

| Repo | Type | Files | Functions | Generated | Test files | Score |
|------|------|-------|-----------|-----------|------------|-------|
| chi | Router library | 74 | 352 | 0 | 21 | 99 |
| fiber | HTTP framework | 237 | 3286 | 15 | 87 | 95 |
| bubbletea | TUI library | 109 | 618 | 0 | 7 | 99 |
| wild-workouts | DDD microservice | 99 | 332 | 16 | 12 | 100 |
| iris | HTTP framework | 761 | 3970 | 21 | 117 | 99 |

## 3. Bugs Found

| # | Type | Severity | Description |
|---|------|----------|-------------|
| 1 | Design | P1 | **Score compression**: 20-point scale + percentage = all repos 95-100. A 5-point spread across repos with 2 to 75 issues is useless for comparison. |
| 2 | UX | P1 | **Silent penalty zone**: Functions between threshold and 2x threshold (e.g., 51–100 lines) lose 0.5 points in scoring but generate NO issue. Users can't see what's penalizing them. Fiber has 115 such functions. |
| 3 | Design | P2 | **Outlier insensitivity**: Fiber's `cache.go:New()` is 798 lines (16x the threshold!) but contributes only 1/3286 = 0.03% to the score. A single catastrophic function has near-zero impact. |
| 4 | Design | P2 | **Rounding absorbs penalties**: Need >2.5% violations to drop from 20 to 19. Chi has 4% violations in function_size but still gets 20/20 due to rounding. |

## 4. False Positives

**None found.** Every issue reported by openkraft was verified against the real code:

- chi `tree.go:401 findRoute` = 144 lines — confirmed with Go AST
- chi `mux_test.go` = 2071 lines — confirmed with `wc -l`
- fiber `ctx_test.go` = 9034 lines — confirmed
- fiber `middleware/cache/cache.go:109 New` = 798 lines — confirmed with awk brace-matching AND Go AST
- Generated file exclusion works correctly: fiber has 15 generated files (84 functions) properly excluded

All function counts, line counts, and severity assignments are accurate.

## 5. False Negatives

| Finding | Scope | Impact |
|---------|-------|--------|
| Silent zone functions not reported as issues | 14 in chi, 115 in fiber, 9 in bubbletea, 1 in wild-workouts, 115 in iris | Users can't diagnose why they're losing points |
| No "outlier penalty" for extreme violations | fiber `New()` at 798 lines is 16x threshold | Catastrophic functions are drowned by the aggregate percentage |

### Silent Zone Details

Scoring threshold vs issue threshold gap:

| Sub-metric | Score penalizes at | Issue reports at | Silent zone |
|------------|-------------------|-----------------|-------------|
| function_size | >50 lines (src) | >100 lines (src) | 51–100 lines |
| function_size | >100 lines (test) | >200 lines (test) | 101–200 lines |
| file_size | >300 lines (src) | >500 lines (src) | 301–500 lines |
| file_size | >600 lines (test) | >1000 lines (test) | 601–1000 lines |
| nesting_depth | >3 (src) | ≥5 (src) | depth 4 |
| parameter_count | >4 (src) | ≥7 (src) | 5–6 params |
| complex_conditionals | >2 (src) | ≥4 (src) | 3 ops |

## 6. Threshold Analysis

| Metric | openkraft default | golangci-lint (funlen) | revive | Verdict |
|--------|------------------|----------------------|--------|---------|
| Max function lines | 50 | 60 | 75 | openkraft stricter — reasonable for AI context |
| Max file lines | 300 | — | — | No standard; 300 is reasonable |
| Max nesting | 3 | — | — | Common in cognitive complexity tools |
| Max params | 4 | — | 5 (revive) | Slightly strict but OK |
| Max cond ops | 2 | — | — | No direct equivalent |

Thresholds are appropriate. The issue isn't the thresholds — it's that violations have too little impact on the final score.

## 7. Scoring Math

### Current formula

```
sub_metric_score = round(earned / total * 20)
```

Where earned gives 1.0 for under threshold, 0.5 for under 2x threshold, 0 for above.

### Problem: score sensitivity

| Score/20 | Compliance needed |
|----------|------------------|
| 20 | ≥97.5% |
| 19 | 92.5–97.5% |
| 18 | 87.5–92.5% |
| 17 | 82.5–87.5% |

All 5 repos are in the 97–98% band for function_size. They all score 19 or 20.

### Problem: large codebases dilute violations

Fiber has 97 functions over 100 lines. But 97/3286 = 3% → still scores 19/20. A repo with 10 functions where 3 are over 100 lines (30% violation rate) would score 14/20. The larger your codebase, the easier it is to hide violations.

### Problem: no outlier penalty

A single 798-line function (fiber `cache.go:New`) contributes the same -1 as a 51-line function. Extreme outliers should have outsized impact.

## 8. Issue Quality

### Strengths
- Issues are specific: include file, line, function name, and measured value
- Severity tiers (info/warning/error) at ≥1.5x and ≥3x thresholds work well
- Generated files correctly excluded
- Test file thresholds correctly relaxed

### Weaknesses
- No issues for silent zone violations (biggest gap)
- Issue severity `issueSeverity(actual, threshold)` uses the ISSUE threshold, not the SCORING threshold — so severity starts from 2x the scoring limit
- No aggregation: a file with 10 oversized functions shows 10 individual issues instead of "file X has systematic problems"

## 9. Recommendations

### P1 — Scoring Discrimination

**Option A: Lower issue threshold to match scoring threshold.** Report issues starting at the scoring threshold (>50 lines) instead of at 2x. This eliminates the silent zone entirely. Issues at the scoring boundary get severity `info`.

**Option B: Add an outlier penalty.** For each extreme violation (≥3x threshold), subtract a fixed bonus from the sub-metric. Example: a 798-line function (16x) could subtract 2 points directly from function_size's 20-point pool.

**Option C: Use weighted violations instead of binary good/partial/zero.** Instead of 1.0/0.5/0.0, use a continuous penalty: `max(0, 1 - (lines - threshold) / threshold)`. This makes a 90-line function (1.8x) worse than a 55-line function (1.1x).

### P2 — Issue Reporting

**Fix silent zone**: Generate `info`-level issues for everything that penalizes the score. The user should always be able to map "lost X points" → "here are the Y functions causing it."

**Add worst-offender highlight**: In the JSON output, include a `worst_offenders` array per sub-metric with the top 3 most egregious violations. This makes it immediately actionable.

## 10. Verified Correct

The following aspects were verified as correct:

- Go AST parser (`processFunc`) correctly computes LineStart/LineEnd via `decl.Pos()`/`decl.End()`
- Function counts match independent analysis (352 for chi, 3286 for fiber — exact)
- Generated file detection works (`"Code generated" + "DO NOT EDIT"`)
- Test file threshold relaxation (2x for functions, 2x for files) correctly applied
- Nesting depth calculation matches independent implementation
- Parameter counting matches independent implementation
- Conditional operator counting matches independent implementation
- Severity assignment formula is consistent
