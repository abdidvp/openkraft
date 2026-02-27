# Audit: verifiability — bonanza-api

> **Date:** 2026-02-26
> **Target:** bonanza-api (multi-module Go API, hexagonal architecture, ~17k functions)
> **Category:** verifiability
> **Reported Score:** 74/100
> **Estimated Accurate Score:** 81-85/100 (FP corrected) or ~60-65/100 (FN accounted)
> **Auditors:** 4 parallel agents (false positives, false negatives, thresholds/math, issue quality)

---

## 1. Summary

Openkraft scored bonanza-api's verifiability at 74/100 with zero issues reported. The score is **deflated by ~7 points** due to test-file type assertions dragging down the safety ratio (13.9% vs 90% in source-only) and an overly strict underscore-only heuristic for test naming. Meanwhile, strong verifiability signals — 14,713 testify assertions, 2,518 subtests, testcontainer integration tests, 4,309 error wrappings, 75 sentinel errors, Docker multi-stage builds — contribute zero points.

## 2. Target Project Profile

| Metric | Value |
|--------|-------|
| Language | Go 1.24 |
| Architecture | Hexagonal, per-feature layout |
| Test files | 594 |
| Source files | 887 |
| Test ratio | 0.67 (target 0.50) |
| Test functions | 6,022 |
| Testify assertions | 14,713 |
| t.Run subtests | 2,518 |
| Type assertions (source) | 40 (90% safe) |
| Type assertions (test) | 1,168 (11.3% safe) |

## 3. Bugs Found

| # | Type | Severity | Description | File:Line |
|---|------|----------|-------------|-----------|
| 1 | Bug | **P1** | `int()` truncation in test_naming: 0.9052*25=22.63 → 22 instead of 23. | `scoring/verifiability.go:92` |
| 2 | Bug | **P1** | `int()` truncation in test_presence: latent, activates at boundary (ratio just below target). | `scoring/verifiability.go:53` |
| 3 | Bug | **P1** | Detail string always says "linter config" even when `.golangci.yml` is absent. | `scoring/verifiability.go:221` |
| 4 | Bug | **P2** | Comment on line 100 misstates point allocation: says "Makefile/Taskfile (8), CI config (7)" but code uses 7 and 5. | `scoring/verifiability.go:100` |

## 4. False Positives

### type_safety_signals: 10/25 → should be ~20/25

**Safe type assertions (0/5 → should be 5/5):**

Test file assertions dominate and drag down the ratio:

| Scope | Total | Safe | Ratio |
|-------|-------|------|-------|
| Source files only | 40 | 36 | **90.0%** (would earn 5/5) |
| Test files only | 1,168 | 132 | 11.3% |
| Combined (what scorer uses) | 1,208 | 168 | **13.9%** (earns 0/5) |

The 1,036 "unsafe" test assertions are idiomatic Go test code — `productID := product["id"].(string)` — where panics are caught by the test runner. Penalizing this is a clear false positive worth 5 points.

**Root cause:** Lines 199-215 iterate ALL files including `_test.go`.

### test_naming: 22/25 → should be ~24/25

571 of 6,022 test functions penalized for lacking underscores. Breakdown:

| Category | Count | Example | Legitimate? |
|----------|-------|---------|-------------|
| Table-driven with t.Run | 271 | `TestNewBranch` with `t.Run("valid input", ...)` | YES — t.Run provides scenario naming |
| Constructor tests | ~149 | `TestNewCustomer` | Mixed — name is clear |
| Parser tests | ~91 | `TestParseGender` | Mixed |
| TestMain | 1 | `TestMain` | YES — Go framework function |

**Root cause:** Lines 79-81 require underscore after "Test" prefix. Doesn't recognize t.Run as equivalent scenario naming. `TestMain` is not a test function.

### .golangci.yml double-counted

`.golangci.yml` is scored in two sub-metrics:
- `build_reproducibility`: 3 points (line 138)
- `type_safety_signals`: 10 points (line 166)
- Total: 13 points for one file (13% of total score)

bonanza-api lacks this file so no false positive here, but a project WITH it gets 13 free points for a single config file.

## 5. False Negatives

| Missing Metric | Severity | Evidence in bonanza-api |
|----------------|----------|------------------------|
| Test quality signals (assertions, helpers, subtests) | **High** | 14,713 testify assertions, 2,518 t.Run subtests, 41 t.Helper() calls, testcontainers integration — zero credit |
| Test coverage distribution | **High** | 27 of 124 directories (21.8%) have zero tests, including `auth/adapters/repository`, `credits/adapters/http`, `inventory/adapters/repository` |
| Error verifiability (%w, sentinels, Is/As) | **Medium** | 4,309 fmt.Errorf with %w, 75 sentinel errors, custom AppError.Is() — zero credit |
| Docker/container reproducibility | **Medium** | Dockerfile (multi-stage), docker-compose.yml, .dockerignore — zero credit |
| Dependency pinning quality | **Low** | All 22 direct deps pinned to exact semver, no pseudo-versions — zero credit beyond go.sum presence |
| Structured logging | **Low** | zerolog in 712 locations — zero credit |

## 6. Threshold Analysis

| Metric | Current Behavior | Concern |
|--------|-----------------|---------|
| Test naming | Underscore after "Test" only | Misses t.Run subtest pattern (47% of penalized functions) |
| Type assertions | All files including tests | Test assertions are idiomatically unsafe; should filter `_test.go` |
| Test presence | Binary above target ratio | Ratio 0.51 and 0.99 score identically — no gradient above target |
| .golangci.yml | 13 pts across 2 metrics | Disproportionate weight for one config file |
| Source count | Includes generated files | Inflates denominator, deflates ratio |

## 7. Scoring Math

### test_presence (25/25) — correct
```
ratio = 594 / 887 = 0.6697
raw = 0.6697 / 0.50 * 25 = 33.48
int(33.48) = 33, capped at 25 ✓
```

### test_naming (22/25) — truncation bug
```
ratio = 5451 / 6022 = 0.9052
raw = 0.9052 * 25 = 22.63
int(22.63) = 22  ← should be math.Round(22.63) = 23
```

### build_reproducibility (17/25) — correct
```
go.sum: 10 + Makefile: 7 + CI: 0 + linter: 0 = 17 ✓
```

### type_safety_signals (10/25) — false positive on assertions
```
.golangci.yml: 0
interface{}/any: 59/24843 = 0.24% < 5% → 10 pts
safe assertions: 168/1208 = 13.9% < 50% → 0 pts
Total: 10 ✓ (but assertion check includes test files — FP)
```

## 8. Issue Quality

### Zero issues for 26 lost points

| Sub-metric | Points Lost | Issues | Gap |
|------------|-----------|--------|-----|
| test_naming | 3 | 0 | Should list 571 poorly-named test functions |
| build_reproducibility | 8 | 0 | Should flag missing CI config and linter config |
| type_safety_signals | 15 | 0 | Should list 1,040 unsafe type assertions |

**Root cause:** `collectVerifiabilityIssues` (line 226) only generates issues when sub-metric scores exactly 0. All of bonanza's sub-metrics score > 0.

### Function signature prevents per-entity issues

```go
func collectVerifiabilityIssues(_ *domain.ScanResult, metrics []domain.SubMetric) []domain.Issue
```

Takes only aggregated `SubMetric` scores. Has no access to `analyzed` map. Cannot generate per-file or per-function issues. Compare with code_health which receives `analyzed map[string]*domain.AnalyzedFile`.

### SubMetric field never set

Issues don't populate the `SubMetric` field, breaking `skip.sub_metrics` filtering.

### No severity gradation

Only two states: SeverityError (test_presence=0) or SeverityWarning (everything else at 0). Never uses SeverityInfo. No proportional severity.

### Detail string misleading

`"linter config, 168/1208 safe type assertions, 24784/24843 clean params"` — says "linter config" when no linter config was found.

## 9. Recommendations

### P0 — Bugs
1. **Use `math.Round()` instead of `int()`** in test_presence and test_naming — same cross-category fix needed.
2. **Fix detail string** — conditionally include "linter config" only when found.

### P1 — Scoring Accuracy
3. **Exclude `_test.go` from type assertion safety** — source-only ratio is 90% (5/5) vs combined 13.9% (0/5). Test assertions are idiomatically bare.
4. **Recognize t.Run as valid test naming** — functions using t.Run internally should get credit even without underscores.
5. **Exempt TestMain** from test naming evaluation — it's a Go framework function.
6. **Deduplicate .golangci.yml scoring** — either score in build_reproducibility OR type_safety_signals, not both.
7. **Exclude generated files from source count** in test_presence.

### P1 — New Metrics
8. **Add test_quality_signals** — detect testify/assertion libraries, t.Helper, t.Run, testcontainers.
9. **Add test_coverage_distribution** — per-package test presence, penalize untested packages.
10. **Add error_verifiability** — %w wrapping ratio, sentinel errors, Is/As implementations.
11. **Add Docker detection** to build_reproducibility — Dockerfile, docker-compose.yml.

### P1 — Issue Quality
12. **Rewrite collectVerifiabilityIssues** — pass `analyzed` map, generate per-entity issues during scoring loops (not post-hoc from aggregates).
13. **Set SubMetric field** on all issues.
14. **Lower threshold from score==0 to score<points** — generate issues whenever points are lost.
15. **Add severity gradation** — error/warning/info based on ratio to threshold.

### P2 — UX
16. **Fix comment/code mismatch** on line 100.
17. **Rebalance points** — test_presence and test_naming are over-weighted (50/100) relative to deeper quality signals.

## 10. Fix Plan Reference

Implementation plan: consolidated plan after all 6 audits complete.
