# Audit: structure — bonanza-api

> **Date:** 2026-02-26
> **Target:** bonanza-api (multi-module Go API, hexagonal architecture, ~17k functions)
> **Category:** structure
> **Reported Score:** 71/100
> **Estimated Accurate Score:** 83-89/100 (FP corrected + FN accounted)
> **Auditors:** 4 parallel agents (false positives, false negatives, thresholds/math, issue quality)

---

## 1. Summary

Openkraft scored bonanza-api's structure at 71/100 with zero issues reported. The score is **deflated by ~24 points** due to a fundamentally flawed `module_completeness` metric (measures size uniformity, not completeness) and an overly narrow suffix list in `expected_files`. Meanwhile, real structural problems (application layer importing pgx.Tx, cross-module coupling, monolithic sqlc package) go completely undetected.

## 2. Target Project Profile

| Metric | Value |
|--------|-------|
| Language | Go 1.24 |
| Architecture | Hexagonal, per-feature layout |
| Modules | 27 |
| Largest module | inventory (168 files, 8+ sub-domains) |
| Smallest module | config (1 file) |
| Port interfaces | 160 |
| Interface implementations | 155/160 (97%) |

## 3. Bugs Found

| # | Type | Severity | Description | File:Line |
|---|------|----------|-------------|-----------|
| 1 | Bug | **P0** | `module_completeness` compares file count vs max module — measures uniformity not completeness. settings (11 files, complete) scores 6.5% because inventory has 168 files. | `scoring/structure.go:220-277` |
| 2 | Bug | **P0** | Zero issues generated for expected_files, interface_contracts, and module_completeness. 29 points lost with no explanation. | `scoring/structure.go:279-315` |
| 3 | Bug | **P1** | Multi-layer modules counted multiple times in completeness (3x penalty for 3-layer modules). "66 comparable modules" is 27 modules × ~2.5 layers. | `scoring/structure.go:230-245` |
| 4 | Bug | **P1** | `expected_files` denominator includes zero-file modules that are skipped in numerator, deflating the average. | `scoring/structure.go:135-137` |
| 5 | Bug | **P1** | `append(profile.ExpectedFileSuffixes, "_test")` — latent slice mutation if profile has spare capacity. | `scoring/structure.go:117` |

## 4. False Positives

### module_completeness: 7/25 → should be ~22-25/25

The metric compares every module's file count against the largest module in its layer group. With inventory at 168 files, every other module scores <50%.

| Module | Files | Ratio vs 168 | Fair? |
|--------|-------|-------------|-------|
| inventory | 168 | 100% | Outlier — 8 sub-domains |
| visits | 87 | 52% | Complete module, half penalty |
| settings | 11 | 6.5% | Complete module, 93% penalty |
| outbox | 9 | 5.4% | Complete module, 95% penalty |

**Root cause:** File count does not equal completeness. A module handling 1 domain entity needs fewer files than one handling 8. The metric should use median-based comparison, layer-presence checking, or coefficient of variation.

### expected_files: 15/25 → should be ~23-25/25

The suffix list misses common Go hexagonal patterns:

| Missing suffix | Count in bonanza | Example |
|----------------|-----------------|---------|
| `_adapter` | 26 files | `activities_adapter.go`, `pricing_adapter.go` |
| `_status` | 34 files | `credit_status.go`, `sale_status.go` |
| `_type` | 28 files | `delivery_type.go`, `interest_type.go` |
| `_mappers` | 12+ files | `credits_mappers.go`, `sales_mappers.go` |
| `_commands` | several | `sales_commands.go` |
| `_requests` | several | `credits_requests.go` |
| Bare entity names | ~200+ | `credit.go`, `sale.go`, `customer.go` |

Bare domain entity files are standard Go: one type per file, named after the type. They should count as convention-conforming.

### interface_contracts: 24/25 — mostly accurate

5 unimplemented interfaces:
- `DomainEvent` (auth) — truly unimplemented (true positive)
- `PaymentMethodValidator` (visits) — truly unimplemented (true positive)
- `PasswordHasher` (auth) — functionality exists but via different pattern (false positive)
- 2 others likely due to name-only matching limitations

## 5. False Negatives

| Problem | Severity | Example |
|---------|----------|---------|
| Application layer imports concrete driver | **High** | `inventory/application/stock_ports.go` imports `pgx/v5` — leaks PostgreSQL into port definitions |
| Cross-module application coupling | **High** | `inventory/application/product_search_doc.go` imports `search/application` directly |
| Monolithic shared data layer | **High** | `database/sqlc/` — 87 files, 57K lines, single package for ALL modules |
| `shared/` catch-all package | **Medium** | 76 files, 13 sub-packages mixing middleware, cache, errors, pagination, codegen |
| God module | **Medium** | inventory: 168 files covering products, stock, movements, transfers, serials, lots, warehouses, categories |
| Adapter naming inconsistency | **Medium** | auth uses `token/`, documents uses `imaging/` + `storage/`, search uses `typesense/` |
| Non-standard layers | **Low** | `audit/mapper/` — not domain/application/adapters |
| Duplicate infrastructure | **Low** | `platform/validator/` and `shared/validator/` coexist |

## 6. Threshold Analysis

| Metric | Current Behavior | Concern |
|--------|-----------------|---------|
| Expected suffixes | 8 hardcoded suffixes | Too narrow — misses 6+ common Go patterns |
| Module completeness | Ratio vs max file count | Penalizes small-but-complete modules |
| Interface matching | Name-only duck typing | Ignores signatures and package scope |

## 7. Scoring Math

### module_completeness (7/25)
```
Groups modules by layer → finds max file count per layer
Per module: ratio = files / maxFiles
Average all ratios → 31%
score = int(0.31 * 25) = 7
```
**Bug:** inventory (168 files) is the implicit "golden" reference. Every other module is penalized against it.

### expected_files (15/25)
```
Per module: ratio = files_matching_suffix / total_files
Average across 27 modules → 62%
score = int(0.62 * 25) = 15
```
**Bug:** denominator includes zero-file modules. Missing common suffixes.

### interface_contracts (24/25)
```
Port interfaces (from domain/ + application/): 160
Matched by name-only duck typing: 155
score = int(155/160 * 25) = 24
```

## 8. Issue Quality

### Zero issues for 29 lost points

| Sub-metric | Points Lost | Issues | Gap |
|------------|------------|--------|-----|
| expected_files | 10 | 0 | Should list modules with low coverage + missing suffixes |
| interface_contracts | 1 | 0 | Should list the 5 unimplemented interfaces |
| module_completeness | 18 | 0 | Should list modules with low ratios |

### Detail strings not actionable
- "62% average conventional file coverage across 27 modules" → which modules? which files?
- "31% average completeness across 66 comparable modules" → what does "completeness" mean here?
- "155/160 port interfaces have concrete implementations" → which 5 are missing?

### SubMetric field
The one existing issue type (`interface_contracts` warning for no-interface modules) correctly sets `SubMetric`. But it never fires for bonanza-api. All missing issue types don't exist yet.

## 9. Recommendations

### P0 — Bugs
1. **Replace `module_completeness` metric** — use median-based comparison or layer-presence checking instead of max-based file count ratio. Or use coefficient of variation to measure uniformity without penalizing intentionally small modules.
2. **Implement issue generation** for expected_files (per-module low coverage), interface_contracts (list unimplemented interfaces), and module_completeness (low-ratio modules).

### P1 — Scoring Accuracy
3. **Expand suffix list** — add `_adapter`, `_status`, `_type`, `_mappers`, `_commands`, `_requests`, `_responses`, `_middleware`, `_client`, `_validator`.
4. **Count bare domain entity files** as convention-conforming (files in `domain/` named after their primary type).
5. **Fix zero-file module denominator** — only count modules with files in the average.
6. **Fix slice mutation** — use `copy()` instead of `append` on profile suffix slice.
7. **Count modules once** in completeness, not once per layer.
8. **Improve interface matching** — match on name + parameter count, not name only.

### P2 — New Metrics / UX
9. **Add import-graph analysis** — detect application/domain importing concrete infrastructure packages (pgx, gorm, etc.).
10. **Add cross-module coupling detection** — flag direct imports between module application layers.
11. **Add god-module detection** — flag modules with file count > 2 standard deviations above mean.
12. **Rebalance weights** — `interface_contracts` to 30-35pts, `module_completeness` to 15-20pts.
13. **Enrich detail strings** — include specific modules and missing items.

## 10. Fix Plan Reference

Implementation plan: consolidated plan after all 6 audits complete.
