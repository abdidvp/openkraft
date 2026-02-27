# Audit: context_quality — bonanza-api

> **Date:** 2026-02-26
> **Target:** bonanza-api (multi-module Go API, hexagonal architecture, ~17k functions)
> **Category:** context_quality
> **Reported Score:** 21/100
> **Estimated Accurate Score:** 83-87/100 (FP corrected) or ~70-75/100 (FN accounted)
> **Auditors:** 4 parallel agents (false positives, false negatives, thresholds/math, issue quality)

---

## 1. Summary

Openkraft scored bonanza-api's context_quality at 21/100 with 3 issues. The score is **deflated by ~62-66 points** due to fundamental design flaws: the scorer penalizes a production API for lacking AI-specific files (CLAUDE.md, AGENTS.md, .cursorrules = 30 points lost) and library conventions (example_test.go, Example* functions = 25 points lost) that are irrelevant to its project type. A P0 bug in `package_documentation` uses bare package names for deduplication — 23 `domain` directories collapse into one map entry, inflating the ratio 4.2x. Meanwhile, the most critical context signal for AI agents — exported function documentation (godoc) — is completely unmeasured. bonanza-api has a 39KB README, 120 SQL migration files, 12+ docs files, documented domain types, and a 107-line .env.example, none of which contribute to the score.

## 2. Target Project Profile

| Metric | Value |
|--------|-------|
| Language | Go 1.24 |
| Architecture | Hexagonal, per-feature layout |
| README size | 39 KB |
| docs/ files | 12+ |
| SQL migration files | 120 |
| sqlc query files | 84 |
| .env.example | 107 lines, commented |
| Makefile targets | 49, self-documenting |
| AI context files | 0 (CLAUDE.md, AGENTS.md, .cursorrules, copilot-instructions) |
| example_test.go files | 0 |
| Example* functions | 0 |
| Package directories | ~127 |
| Unique package names | 31 |
| Documented packages (by name) | 8/31 |
| Documented packages (by path) | ~9/127 |

## 3. Bugs Found

| # | Type | Severity | Description | File:Line |
|---|------|----------|-------------|-----------|
| 1 | Bug | **P0** | `package_documentation` deduplicates by `af.Package` (bare name like "domain"), not directory path. 23 `domain` directories collapse to 1 entry. If any has a doc comment, all 23 are "documented". Inflates ratio 4.2x (8/31=26% vs 9/127=7%). | `scoring/context_quality.go:122-123` |
| 2 | Bug | **P1** | CLAUDE.md "example" match at line 248 is too loose — any CLAUDE.md containing the English word "example" (e.g., "For example, run...") earns 10 free points (40% of canonical_examples budget). | `scoring/context_quality.go:247-249` |
| 3 | Bug | **P1** | ADR detection uses `strings.Contains(lower, "adr")` — matches "leadership.md", "address-handling.md", "squadron.md" as false ADR files. | `scoring/context_quality.go:176` |
| 4 | Bug | **P1** | `FixAvailable: true` on .cursorrules and AGENTS.md issues, but fix service only handles CLAUDE.md. Dishonest claim. | `scoring/context_quality.go:283,293` |
| 5 | Bug | **P1** | copilot-instructions missing from issue collector (hardcoded 3 of 4 profile entries). | `scoring/context_quality.go:266-301` |
| 6 | Bug | **P1** | User profile overrides can push category total past 100 (no normalization). | `scoring/context_quality.go:25-29` |
| 7 | Bug | **P2** | `int()` truncation in package_documentation. | `scoring/context_quality.go:135` |
| 8 | Bug | **P2** | copilot-instructions size hardcoded to 0 in `contextFileStatus` — MinSize threshold can never be met. | `scoring/context_quality.go:98` |
| 9 | Bug | **P2** | Points=1 with MinSize collapses: halfPts guarded to 1, remainder=0. No size distinction. | `scoring/context_quality.go:63-64` |

## 4. False Positives

### ai_context_files: 0/30 → should be ~0/15 (weight too high) or separate category

bonanza-api has zero AI context files. This is normal for a production business API. The 30-point penalty measures **AI tool adoption**, not context quality. A codebase with flawless documentation but no CLAUDE.md caps at 70/100.

**Root cause:** No project-type awareness. The scorer applies AI-tool file expectations uniformly.

### canonical_examples: 0/25 → should be ~0/10 (library convention)

The three checks are library conventions irrelevant to internal APIs:
- `example_test.go` files (10 pts): Go convention for libraries published to pkg.go.dev. An internal API's "consumers" use HTTP endpoints, not Go packages.
- `Example*` functions (5 pts): Same — library documentation convention.
- CLAUDE.md references (10 pts): Requires CLAUDE.md to exist first — **hidden 10-point double penalty** on top of the 10 pts already lost in ai_context_files.

### package_documentation: 6/25 → inflated (should be ~1/25)

The dedup bug inflates from 9/127=7% to 8/31=26%:

| Package name | Directories | Documented dirs |
|-------------|------------|-----------------|
| `domain` | 23 | 2 |
| `application` | 22 | 0 |
| `http` | 21 | 0 |
| `repository` | 20 | 0 |
| `external` | 10 | 0 |

Additionally, `sqlc` (generated code) is penalized for missing docs. Conventional hexagonal layer names (`application`, `repository`, `http`) are self-describing within the architecture.

### Summary of false positives

| Sub-metric | Points Lost | False Positive Points | Reason |
|-----------|------------|----------------------|--------|
| ai_context_files | 30 | **30** | AI tooling adoption, not context quality |
| canonical_examples | 25 | **25** | Library conventions for internal API |
| package_documentation | 19 | **~8** | Generated code + architectural layer names |
| architecture_docs | 5 | **~3** | DECISIONS.md not recognized as ADR |
| **Total** | **79** | **~66** | |

## 5. False Negatives

| Missing Metric | Severity | Evidence in bonanza-api |
|----------------|----------|------------------------|
| Exported function godoc | **Critical** | credit.go: 80 exports, only 30 documented (37.5%). This is THE most important context signal for AI agents. `Function` struct has no `HasDoc` field. |
| Type/struct documentation | **High** | All domain types have doc comments (`// Credit represents...`), 221 domain files — zero credit |
| Database migration files | **High** | 120 SQL migrations in `db/migrations/`, 84 sqlc query files — zero credit |
| Error message quality as context | **Medium** | Structured error library: `errors.NewNotFound(errors.CodeNotFound, "Branch not found", source)` — zero credit |
| .env.example | **Medium** | 107 lines, every var commented with valid values — zero credit |
| Makefile documentation | **Medium** | 49 targets with `## description` self-documenting pattern, `make help` — zero credit |
| Per-module READMEs | **Low** | Not present in bonanza, but scorer can't reward projects that have them |
| Inline comment density | **Low** | credit.go: 13.6% comment density with section separators — zero credit |

## 6. Threshold Analysis

| Metric | Current Behavior | Concern |
|--------|-----------------|---------|
| ai_context_files weight | 30/100 pts (30%) | Dominates category for files most projects lack |
| Package dedup | By bare package name | 127 directories → 31 entries. Arbitrary collisions |
| canonical_examples | Library conventions | Irrelevant for service/API projects |
| CLAUDE.md example match | `Contains("example")` | Any English prose matches — 10 free points |
| ADR detection | `Contains("adr")` | Substring match — false positives on "address", "leadership" |
| README size threshold | 500 bytes for full credit | Reasonable |

## 7. Scoring Math

### ai_context_files (0/30)
```
CLAUDE.md: missing → 0
AGENTS.md: missing → 0
.cursorrules: missing → 0
copilot-instructions: missing → 0
Total: 0/30
```

### package_documentation (6/25) — inflated by dedup bug
```
Reported: 8/31 packages documented = 25.8%
score = int(0.258 * 25) = 6

Correct (by path): 9/127 = 7.1%
score = int(0.071 * 25) = 1
Inflation: +5 points from dedup bug
```

### architecture_docs (15/20)
```
README.md: 39KB > 500 bytes → 8 pts
docs/: exists → 7 pts
ADR: not found (DECISIONS.md not recognized) → 0 pts
Total: 15/20
```

### canonical_examples (0/25)
```
example_test.go files: 0 → 0 pts
Example* functions: 0 → 0 pts
CLAUDE.md references: no CLAUDE.md → 0 pts (double penalty)
Total: 0/25
```

## 8. Issue Quality

### 3 issues for 79 lost points

| Sub-metric | Points Lost | Issues | Gap |
|------------|-----------|--------|-----|
| ai_context_files | 30 | 3 | Missing copilot-instructions issue. 2 of 3 claim FixAvailable but fix service doesn't handle them |
| package_documentation | 19 | 0 | Should list undocumented packages |
| architecture_docs | 5 | 0 | Should flag missing ADR files |
| canonical_examples | 25 | 0 | Should suggest creating examples |

**62% of lost points (49/79) have zero corresponding issues.**

### SubMetric field never set

All 3 issues have empty `SubMetric`. Breaks filtering and grouping.

### FixAvailable dishonesty

| Issue | FixAvailable | Fix service handles it? |
|-------|-------------|------------------------|
| CLAUDE.md missing | true | YES — generates from onboard report |
| .cursorrules missing | true | **NO** — not implemented |
| AGENTS.md missing | true | **NO** — not implemented |

### Issue collector can't see analysis data

```go
func collectContextQualityIssues(scan *domain.ScanResult) []domain.Issue
```

Only receives `scan`, not `analyzed`. Cannot generate per-package or per-file issues for package_documentation or canonical_examples. Architectural limitation.

### Comparison with other scorers

| Category | Points Lost | Issues | Ratio |
|----------|-----------|--------|-------|
| code_health | 9 | 456 | 50.7 issues/pt |
| predictability | 38 | 23 | 0.6 issues/pt |
| **context_quality** | **79** | **3** | **0.04 issues/pt** |
| discoverability | 18 | 0 | 0 |
| structure | 29 | 0 | 0 |
| verifiability | 26 | 0 | 0 |

## 9. Recommendations

### P0 — Bugs
1. **Fix package deduplication** — use `filepath.Dir(af.Path)` instead of `af.Package`. Current dedup inflates ratio 4.2x.
2. **Fix FixAvailable dishonesty** — set `FixAvailable: false` on .cursorrules and AGENTS.md issues, or implement generation.
3. **Make issue collector profile-driven** — iterate `profile.ContextFiles` instead of hardcoding 3 of 4 files.

### P1 — Scoring Accuracy
4. **Reduce ai_context_files weight** — from 30 to 15 points. Redistribute to new metrics (godoc, migrations).
5. **Add project-type awareness** — detect library vs service/API and adjust expectations. Services don't need example_test.go.
6. **Tighten CLAUDE.md "example" match** — require `"example_test"`, `"Example"` + uppercase, or backtick-wrapped paths.
7. **Fix ADR detection** — match as path component (`/adr/`, `adr-` prefix) not substring.
8. **Exclude generated packages** from package_documentation.
9. **Use `math.Round()`** instead of `int()`.
10. **Pass `analyzed` to issue collector** — change signature to enable per-package issues.

### P1 — New Metrics (Critical)
11. **Add exported_func_documentation** (15-20 pts) — ratio of exported functions with godoc comments. Requires adding `HasDoc bool` to `Function` struct. This is THE most important context signal for AI agents.
12. **Add type_documentation** (5-8 pts) — ratio of documented exported types.
13. **Add schema_context** (5 pts) — detect `migrations/*.sql`, `sqlc.yaml`, query files.

### P2 — UX
14. **Add issue generation for all sub-metrics** — package_documentation (list undocumented packages), architecture_docs (flag missing items), canonical_examples (suggest what to create).
15. **Set SubMetric field** on all issues.
16. **Recognize .env.example, Makefile documentation** as context signals.
17. **Normalize category total to 100** when user overrides context file points.

## 10. Fix Plan Reference

Implementation plan: consolidated plan after all 6 audits complete.
