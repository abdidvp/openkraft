# Audit Reports

Standardized audit reports from testing openkraft against real codebases. Each audit evaluates scoring accuracy, identifies bugs, false positives, false negatives, and produces actionable fix plans.

## Format

Each audit follows the template in `_TEMPLATE.md`. Reports are named:

```
NN-YYYY-MM-DD-audit-<target>-<category>.md
```

Where `NN` is the sequential audit number.

## Index (chronological)

### Round 1 — 2026-02-26: bonanza-api (single repo, all categories)

First audit pass against our own bonanza-api project. Identified major bugs across all 6 scoring categories.

| # | File | Target | Category | Score | Key Finding |
|---|------|--------|----------|-------|-------------|
| 01 | `01-2026-02-26-audit-bonanza-code-health.md` | bonanza-api | code_health | 91→~80 | 53% false positives from generated/test code |
| 02 | `02-2026-02-26-audit-bonanza-discoverability.md` | bonanza-api | discoverability | 82→~68 | Suffix Jaccard bug (26%→75%), zero issues for 18 lost points |
| 03 | `03-2026-02-26-audit-bonanza-structure.md` | bonanza-api | structure | 71→~86 | module_completeness measures uniformity not completeness, zero issues for 29 lost points |
| 04 | `04-2026-02-26-audit-bonanza-verifiability.md` | bonanza-api | verifiability | 74→~82 | Test-file assertions drag safety ratio (90%→14%), zero issues for 26 lost points |
| 05 | `05-2026-02-26-audit-bonanza-predictability.md` | bonanza-api | predictability | 62→~50 | HasContext parser bug (93.5% FP), test inflation (+11pts), linear penalty cliff, 95.7% issue FP rate |
| 06 | `06-2026-02-26-audit-bonanza-context-quality.md` | bonanza-api | context_quality | 21→~85 | 66/79 lost points are false positives (AI files + library conventions for API project), package dedup bug inflates 4.2x |

### Round 2 — 2026-02-26: Multi-repo benchmark (10 repos, code_health)

Validated code_health fixes from Round 1 against 10 popular Go repos of varying size and quality.

| # | File | Target | Category | Score Range | Key Finding |
|---|------|--------|----------|-------------|-------------|
| 07 | `07-2026-02-26-audit-multirepo-code-health.md` | 10 repos | code_health | 72–98 | Scores mechanically correct; identified decayK miscalibration (k=2→4), template function false positives, incomplete generated file detection |

### Round 3 — 2026-02-27: Ollama deep dive (single repo, code_health)

Deep audit of ollama (score 74) using 4 parallel agents to investigate whether the low score is justified.

| # | File | Target | Category | Score | Key Finding |
|---|------|--------|----------|-------|-------------|
| 08 | `08-2026-02-27-audit-ollama-code-health.md` | ollama | code_health | 74 (~5-8pts too severe) | 56-71% of error-level issues are FPs from domain patterns (ML constructors, table-driven tests, CGo bindings). Penalty formula extrapolates aggressively beyond calibration range. Fair score: 79-82. |
