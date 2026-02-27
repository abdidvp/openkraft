# Audit: predictability — bonanza-api

> **Date:** 2026-02-26
> **Target:** bonanza-api (multi-module Go API, hexagonal architecture, ~17k functions)
> **Category:** predictability
> **Reported Score:** 62/100
> **Estimated Accurate Score:** 42-50/100 (bugs fixed, test inflation removed) or ~35-45/100 (FN accounted)
> **Auditors:** 4 parallel agents (false positives, false negatives, thresholds/math, issue quality)

---

## 1. Summary

Openkraft scored bonanza-api's predictability at 62/100 with 23 issues. The score is **inflated by ~11 points** because `self_describing_names` includes test functions (which always have 2+ words) in the denominator. The `explicit_dependencies` sub-metric scores 0/25 due to a linear penalty cliff where 9+ exported vars guarantee zero — and 28 of 39 penalized vars are Wire dependency injection sets (immutable, compile-time DI). The `HasContext` check in the parser has a P0 bug: it checks if English prose contains letters like "s" or "v" instead of format verbs like `%s` or `%v`, producing a 93.5% false positive rate. Meanwhile, 95.7% of generated issues are false positives (sentinel error files flagged as mutable state). Critical predictability problems — cross-module verb inconsistency (Delete returns error in 9 modules, void in 8), List return-type divergence (3 pagination patterns), magic numbers — go undetected.

## 2. Target Project Profile

| Metric | Value |
|--------|-------|
| Language | Go 1.24 |
| Architecture | Hexagonal, per-feature layout |
| Exported functions (all files) | 15,271 |
| Exported functions (source only) | ~3,295 |
| Test functions | ~11,976 |
| Exported non-Err global vars | 39 |
| Wire provider sets in those 39 | 28 (72%) |
| Error calls | 2,434 |
| Wrap ratio | ~60% |
| Context ratio (reported) | ~64% (inflated — real ~6%) |

## 3. Bugs Found

| # | Type | Severity | Description | File:Line |
|---|------|----------|-------------|-----------|
| 1 | Bug | **P0** | `HasContext` in parser checks `strings.ContainsAny(lit.Value, "svdxfgq")` — matches English letters, not format verbs. 93.5% false positive rate (1,384/1,481 fmt.Errorf calls falsely marked as "having context"). | `parser/go_parser.go:303` |
| 2 | Bug | **P0** | `self_describing_names` includes `_test.go` files. ~11,976 test functions (always 2+ words) inflate ratio from ~47.6% to 88.7%, adding ~11 phantom points. | `scoring/predictability.go:44` |
| 3 | Bug | **P0** | `explicit_dependencies` linear penalty: `score = 25 - (count * 3)`. Only 9 vars needed to zero out 25 points. No discrimination between 9 and 39 violations. | `scoring/predictability.go:102-106` |
| 4 | Bug | **P0** | Issue collector uses `len(af.GlobalVars) > 3` (raw count including Err* and unexported) but scorer filters to exported non-Err only. 22 of 23 issues (95.7%) are false positives. | `scoring/predictability.go:294` |
| 5 | Bug | **P1** | `error_message_quality` returns score=0 when `totalErrors==0` ("no error handling found"). Should be full credit — zero errors means nothing to mishandle. | `scoring/predictability.go:138-142` |
| 6 | Bug | **P1** | `error_message_quality` includes test files in error counts (no `_test.go` filter), unlike other 3 sub-metrics which filter. | `scoring/predictability.go:120` |
| 7 | Bug | **P1** | `int()` truncation across all sub-metrics. Worst: consistent_patterns loses 0.92 pts (22 instead of 23). | `scoring/predictability.go:62,161,265` |
| 8 | Bug | **P2** | `hasVerbNounPattern` just checks `camelcase.Split >= 2` words. `CreditPlan` (noun+noun) passes as "verb+noun". Metric name is misleading. | `scoring/naming.go:91-96` |

## 4. False Positives

### self_describing_names: 22/25 → real source-only score ~11/25

Test functions inflate the metric by 11 points:

| Population | Exported Funcs | Pass Check | Ratio | Score |
|-----------|---------------|-----------|-------|-------|
| All files (current) | 15,271 | 13,543 | 88.7% | 22/25 |
| Source only (correct) | ~3,295 | ~1,567 | ~47.6% | ~11/25 |

Of the ~1,728 source functions penalized, the majority are legitimate Go patterns:

| Category | Count | Example | Legitimate? |
|----------|-------|---------|-------------|
| Getters on private-field structs | 2,121 | `(b *Branch) Name()`, `(a *Address) Street()` | YES — Go convention: no "Get" prefix |
| Interface implementations | 295 | `String()`, `Error()`, `Value()`, `Scan()` | YES — mandated by Go interfaces |
| Constructors | 12 | `New()` | YES |

### explicit_dependencies: 0/25 → should be ~15-22/25

All 39 penalized vars are legitimate Go patterns:

| Category | Count | Example | Mutable? |
|----------|-------|---------|----------|
| Wire provider sets | 28 | `var AuditSet = wire.NewSet(...)` | NO — compile-time DI |
| Interface-typed presets | 5 | `var FormatDashed Format = PrefixSequentialFormat{...}` | NO — can't be const |
| Decimal thresholds | 3 | `var AmountThresholdHigh = decimal.NewFromInt(10000)` | NO — can't be const |
| Config presets | 3 | `var DefaultConfig = Config{...}` | NO — struct literal |

**Root cause:** Linear penalty (`count * 3`) with no awareness of immutability. Any project with 9+ exported non-Err vars gets 0/25 regardless of whether they're mutable.

### error_message_quality: 18/25 — inflated by HasContext bug

The `HasContext` check at `parser/go_parser.go:303` uses `strings.ContainsAny(lit.Value, "svdxfgq")` which matches English letters, not `%s`/`%v` format verbs. Result:

| Metric | Reported | Actual |
|--------|----------|--------|
| Context ratio | 64% | ~6% |
| Composite score | 0.732 → 18/25 | ~0.55 → 13/25 |

### consistent_patterns: 22/25 — survivorship bias

366 bare-named files (28% of methods) are silently skipped because they have no underscore suffix. Only suffixed files participate in consistency analysis, biasing toward files already following conventions.

### Issue false positives: 22/23 (95.7%)

| File type | Issues | Penalized in scoring? |
|-----------|--------|----------------------|
| `*_errors.go` (100% Err* sentinels) | 21 | NO — all exempted |
| `case.go` (unexported vars) | 1 | NO — all lowercase |
| `format.go` (exported Format vars) | 1 | YES — true positive |

## 5. False Negatives

| Missing Metric | Severity | Evidence in bonanza-api |
|----------------|----------|------------------------|
| Cross-module verb inconsistency | **High** | `Delete` returns error in 9 modules, void in 8. `Delete(uuid.UUID)` vs `Delete(*uuid.UUID)` across credits/catalogs. `Get` vs `Find` vs `Fetch` for retrieval |
| CRUD return type divergence | **High** | `List` has 3 patterns: `([]Response, int64, error)` offset (9 services), `(*CursorListResponse, error)` cursor (6 services), `(ListResponse, error)` value (2 services) |
| Create parameter inconsistency | **Medium** | Some use `Create(ctx, cmd, createdBy)`, others embed actor in command struct |
| Magic numbers | **Medium** | `PageSize > 100` hardcoded in 10+ modules (25+ occurrences) without named constant. `5 * time.Second` duplicated in audit module |
| Cross-cutting concern consistency | **Medium** | Entire credits module (6 services) has zero logger injection while 30+ other services inject `zerolog.Logger` |
| Pagination pattern divergence | **Medium** | Three approaches: filter-based offset/limit, cursor-based, ad-hoc handler parsing — coexist without abstraction |
| Package API surface explosion | **Low-Med** | `inventory/domain` exports 517 declarations vs `search/domain` with 9 (57x difference) |

## 6. Threshold Analysis

| Metric | Current Behavior | Concern |
|--------|-----------------|---------|
| MaxGlobalVarPenalty | 3 pts/var, linear | 9 vars = instant zero. No cap or diminishing returns |
| Verb+noun check | camelcase.Split >= 2 | Accepts noun+noun, doesn't verify verb presence |
| HasContext check | `ContainsAny("svdxfgq")` | Matches English letters, not format verbs — 93.5% FP rate |
| conventionCompliance | Cliff at wrapRatio 0.5 | 51% → 1.0, 49% → 0.7 (1.5 pt swing at single threshold) |
| Consistent patterns bare files | Skipped entirely | 28% of methods invisible to consistency analysis |
| No-roles partial credit | 50% (12/25) | May be generous for unanalyzable codebases |

## 7. Scoring Math

### self_describing_names (22/25) — inflated by test files
```
total = 15,271 (includes ~11,976 test functions!)
verbNoun = 13,543
ratio = 13543 / 15271 = 0.887
score = int(0.887 * 25) = 22

Source-only estimate:
total = ~3,295, verbNoun = ~1,567
ratio = 0.476, score = int(0.476 * 25) = 11
```

### explicit_dependencies (0/25) — linear cliff
```
mutableState = 39 (exported non-Err vars + init functions)
MaxGlobalVarPenalty = 3 (from profile.go:67)
penalty = 39 * 3 = 117
score = 25 - 117 = -92, clamped to 0
```
Only 9 vars needed to guarantee zero (`ceil(25/3) = 9`).

### error_message_quality (18/25) — HasContext bug inflates
```
totalErrors = 2434, wrapped ≈ 1460 (60%), withContext ≈ 1558 (64% — inflated)
conventionCompliance = 1.0 (wrapRatio > 0.5)
sentinelScore = 1.0
composite = 0.60*0.4 + 0.64*0.3 + 1.0*0.2 + 1.0*0.1 = 0.732
score = int(0.732 * 25) = 18

With HasContext fix (real context ~6%):
composite = 0.60*0.4 + 0.06*0.3 + 1.0*0.2 + 1.0*0.1 = 0.558
score = int(0.558 * 25) = 13
```

### consistent_patterns (22/25) — survivorship bias
```
consistentRoles = 132, totalRoles = 144
ratio = 132/144 = 0.917
score = int(0.917 * 25) = 22  ← int() truncation (Round → 23)
```
366 bare files (28% of methods) excluded from analysis.

### Corrected estimate (all bugs fixed)
```
self_describing_names: ~11/25 (test filter)
explicit_dependencies: ~15-22/25 (diminishing penalty, Wire exemption)
error_message_quality: ~13/25 (HasContext fix)
consistent_patterns: ~18/25 (include bare files)
Total: ~57-78/100 range depending on fix approach
```

## 8. Issue Quality

### 23 issues — 22 are false positives (95.7%)

| Sub-metric | Points Lost | Issues | Gap |
|------------|-----------|--------|-----|
| self_describing_names | 3 (reported) / 14 (real) | 0 | Should list functions failing verb+noun |
| explicit_dependencies | 25 | 23 | 22 are false positives (sentinel error files) |
| error_message_quality | 7 | 0 | Should list unwrapped errors, missing context |
| consistent_patterns | 3 | 0 | Should list 12 inconsistent role groups |

### SubMetric field never set

All 23 issues have empty `SubMetric`. Breaks `skip.sub_metrics` filtering.

### Scoring/issue filter mismatch (P0)

The scoring function at lines 82-89 filters to exported non-Err vars. The issue collector at line 294 uses raw `len(af.GlobalVars) > 3`. Result: files like `inventory_errors.go` (133 `Err*` sentinel errors) are flagged as "file has 133 package-level variables (prefer explicit injection)" — advice that is completely wrong for sentinel errors.

### No severity graduation

Global vars get Warning, init() gets Info. No Error tier. A file with 133 vars gets the same severity as one with 5.

### Detail strings misleading

`"file has 47 package-level variables (prefer explicit injection)"` for a file containing only sentinel errors tells developers to refactor idiomatic Go code.

## 9. Recommendations

### P0 — Bugs
1. **Fix `HasContext` in parser** — check for `%s`, `%v`, `%d`, etc. (percent-prefixed format verbs), not bare English letters.
2. **Filter `_test.go` from `self_describing_names`** — test functions inflate ratio from ~47.6% to 88.7%.
3. **Replace linear penalty in `explicit_dependencies`** — use diminishing returns (e.g., `score = int(25 * math.Exp(-0.15 * count))`) or ratio-based scoring.
4. **Fix issue collector filter** — apply same exported non-Err filter as scoring function.

### P1 — Scoring Accuracy
5. **Filter `_test.go` from `error_message_quality`** — other 3 sub-metrics already filter.
6. **Fix zero-error edge case** — return full credit when `totalErrors == 0`.
7. **Use `math.Round()` instead of `int()`** — consistent_patterns loses 0.92 pts to truncation.
8. **Exempt Go interface methods and getters** from self_describing_names penalty.
9. **Include bare files in consistent_patterns** — currently 28% of methods are invisible.
10. **Set SubMetric field** on all generated issues.
11. **Recognize immutable var patterns** — Wire sets, `regexp.MustCompile`, `decimal.NewFromInt` are not mutable state.

### P1 — New Metrics
12. **Add cross_module_verb_consistency** — detect Delete/Remove, Get/Find/Fetch divergence.
13. **Add cross_module_signature_consistency** — detect same-verb different-return-type patterns (3 List pagination shapes).

### P2 — UX
14. **Rename `self_describing_names`** to `multi_word_names` or implement real verb detection.
15. **Add issue generation for all 4 sub-metrics** — currently only explicit_dependencies generates issues.
16. **Add severity graduation** — error for extreme violations, info for minor.
17. **Add magic number detection** — flag repeated hardcoded literals (e.g., `PageSize > 100` in 10+ modules).

## 10. Fix Plan Reference

Implementation plan: consolidated plan after all 6 audits complete.
