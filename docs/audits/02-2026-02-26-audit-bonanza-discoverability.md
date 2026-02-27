# Audit: discoverability — bonanza-api

> **Date:** 2026-02-26
> **Target:** bonanza-api (multi-module Go API, hexagonal architecture, ~17k functions)
> **Category:** discoverability
> **Reported Score:** 82/100
> **Estimated Accurate Score:** 65-72/100 (with false negatives accounted for)
> **Auditors:** 4 parallel agents (false positives, false negatives, thresholds/math, issue quality)

---

## 1. Summary

Openkraft scored bonanza-api's discoverability at 82/100 with zero issues reported. Audit reveals a P0 bug where bare filenames contaminate the suffix Jaccard calculation (26% reported vs ~75% actual), zero issue generation for 3 of 4 sub-metrics despite losing 18 points, and several missing metrics (godoc coverage, package name quality, verb consistency) that would lower the true score to ~65-72.

## 2. Target Project Profile

| Metric | Value |
|--------|-------|
| Language | Go 1.24 |
| Architecture | Hexagonal, per-feature layout |
| Files analyzed (non-test) | 886 |
| Exported functions | 15,271 |
| Modules | 27 |
| Naming convention | suffixed (83%) |
| Dependency violations | 0 |

## 3. Bugs Found

| # | Type | Severity | Description | File:Line |
|---|------|----------|-------------|-----------|
| 1 | Bug | **P0** | Bare filenames (`credit.go`, `errors.go`) added to suffix set as full names, contaminating Jaccard denominator. `suffixes=26%` should be ~75%. | `scoring/discoverability.go:244-246` |
| 2 | Bug | **P0** | Zero issues generated for naming_uniqueness, file_naming_conventions, and predictable_structure. `collectDiscoverabilityIssues` only checks dependency_direction. `scan` param received as `_`. | `scoring/discoverability.go:379-407` |
| 3 | Bug | **P1** | `int()` truncation instead of `math.Round()` — systematic 0-1 point downward bias on all sub-metrics | `scoring/discoverability.go:66,139,291` |
| 4 | Bug | **P1** | Detail string "83%" is the blended ratio (with suffixReuse), not the raw 735/886=82.96%. Misleading. | `scoring/discoverability.go:143` |

## 4. False Positives

| Category | Count/Impact | Root Cause |
|----------|-------------|------------|
| Bare domain entity files penalized | ~152 files, ~5 pts lost | `errors.go`, `credit.go`, `config.go` etc. are standard Go names that don't need suffixes |
| Suffix Jaccard polluted by bare names | ~4 pts lost on predictable_structure | Bug #1: bare filenames treated as "suffixes" |
| Interface methods penalized | ~2 pts lost on naming_uniqueness | `String()`, `Error()`, `Scan()`, `Value()` are mandated by Go interfaces |
| Generated files counted | Minor | sqlc files included in file_naming_conventions |
| Small packages penalized | entropy=0 for 1 export | Packages with 1 exported function capped at 17/25 |

### Examples

**Bare files (false positive on file_naming_conventions):**
- `internal/credits/domain/credit.go` — standard: one type per file named after the type
- `internal/shared/errors/errors.go` — standard Go convention
- `internal/database/sqlc/models.go` — auto-generated, naming not user-controlled
- `cmd/api/providers/credits.go` — DI wiring file named after module

**Suffix Jaccard (false positive on predictable_structure):**
- Domain suffixes like `_plan`, `_term`, `_strategy` are entity compound names, NOT role suffixes
- Role suffixes (`_handler`, `_service`, `_repository`) are consistent at ~80% across modules
- Mixing both types in one Jaccard calculation gives misleading 26%

**Naming uniqueness (false positive):**
- `String()` implements `fmt.Stringer` — penalized as single-word
- `Scan()` implements `database/sql.Scanner` — penalized
- `IsValid()` is standard Go enum validation — penalized

## 5. False Negatives

| Missing Metric | Impact | Example in bonanza-api |
|----------------|--------|------------------------|
| Package name quality | High | 5 generic packages: `shared` (47 files, 12 sub-packages), `platform`, `config`, `database`, `testkit` |
| Godoc coverage | High | 1,283/3,380 exports undocumented (37.9%). Worst: `combos` at 92% undocumented |
| Verb consistency | Medium | `Create*` vs `Add*` for child entities, `Delete*` vs `Remove*` across modules |
| API contract consistency | Medium | 3 different pagination patterns (limit/offset, page/pageSize, cursor) coexisting |
| Adapter sub-package naming | Low-Med | 6 modules use bespoke adapter names (`token`, `imaging`, `storage`, `typesense`) |
| Cross-module coupling | Medium | dependency_direction only checks layer violations, not cross-module imports |
| File cohesion | Low-Med | Handler files declaring application-layer interfaces |

## 6. Threshold Analysis

| Metric | Current Behavior | Concern |
|--------|-----------------|---------|
| Word count scoring | 1 word=0.5, 2-4=1.0, 5=0.7 | Single-word penalty too harsh for Go interface methods |
| File naming | bare vs suffixed binary classification | Doesn't distinguish role suffixes from compound names |
| Suffix Jaccard | All file suffixes compared | Should only compare role-indicating suffixes |
| Dependency direction | 5 violations = 0/25 | Aggressive but reasonable for architecture violations |

## 7. Scoring Math

### naming_uniqueness (21/25)
```
composite = 0.69*0.4 + 0.99*0.3 + 0.91*0.3 = 0.846
score = int(0.846 * 25) = 21  (math.Round would give 21)
```

### file_naming_conventions (20/25)
```
raw_consistency = 735/886 = 0.8296
blended = (0.8296 + suffixReuse) / 2.0 ≈ 0.83
score = int(0.83 * 25) = 20
```

### predictable_structure (16/25)
```
composite = 0.92*0.5 + 0.26*0.3 + 0.54*0.2 = 0.646
score = int(0.646 * 25) = 16

With bug fix (suffixes ~0.75):
composite = 0.92*0.5 + 0.75*0.3 + 0.54*0.2 = 0.793
score = int(0.793 * 25) = 19  (+3 points)
```

### Key inconsistency
`file_naming_conventions` says 83% suffixed, `predictable_structure` says suffixes=26%. Different metrics measuring different things, but the 26% is artificially low due to Bug #1.

## 8. Issue Quality

### Zero issues for 18 lost points

| Category | Score | Issues | Gap |
|----------|-------|--------|-----|
| code_health | 91 | 456 | Baseline |
| discoverability | 82 | **0** | **18 points with no explanation** |

### Missing issue types that should exist
1. Files violating naming convention — data already computed, ~151 potential issues
2. Exported functions with poor word count (single-word) — ~4,731 potential issues
3. Functions with vague names (`Handle`, `Process`, `Data`) — data in `vagueWords` map
4. Modules missing expected layers — Jaccard data already computed
5. Modules with inconsistent suffix sets — data already computed

### Detail strings not actionable
- `"word count=0.69, specificity=0.99, entropy=0.91"` — meaningless to a developer
- Should be: `"4,731 of 15,271 exported functions have single-word names (aim for 2-4 words)"`

## 9. Recommendations

### P0 — Bugs
1. **Fix bare filename pollution in suffix Jaccard** — remove the `else` branch at line 244 that adds full bare names to suffix sets. Only include actual underscore-delimited suffixes.
2. **Implement issue generation for all sub-metrics** — emit per-file issues for naming convention violations, per-function issues for poor naming, and per-module issues for structural gaps.

### P1 — Scoring Accuracy
3. **Use `math.Round()` instead of `int()`** — same fix needed across all scorers.
4. **Distinguish role suffixes from domain compound names** — `_handler`, `_service`, `_repository` are role suffixes; `_plan`, `_term`, `_strategy` are entity names. Only compare role suffixes in Jaccard.
5. **Exempt Go interface methods from word count penalty** — `String()`, `Error()`, `Scan()`, `Value()`, `IsValid()` should not be penalized.
6. **Exclude generated files** from file_naming_conventions — detect `// Code generated` header.
7. **Fix misleading detail string** — report raw ratio and blended ratio separately.
8. **Handle small packages** — when exports <= 3, default entropy to 1.0 instead of 0.

### P2 — New Metrics / UX
9. **Add package_name_quality** — penalize `shared`, `utils`, `helpers`, `common`, `misc`.
10. **Add godoc_coverage** — ratio of documented exports.
11. **Add verb_consistency** — detect Create vs Add, Delete vs Remove divergence.
12. **Rewrite detail strings** — human-readable with actionable context.

## 10. Fix Plan Reference

Implementation plan: `docs/plans/2026-02-26-fix-discoverability-accuracy.md` (to be created after all 6 audits complete)
