# Audit Reports

Standardized audit reports from testing openkraft against real codebases. Each audit evaluates scoring accuracy, identifies bugs, false positives, false negatives, and produces actionable fix plans.

## Format

Each audit follows the template in `_TEMPLATE.md`. Reports are named:

```
YYYY-MM-DD-audit-<target>-<category>.md
```

## Index

| Date | Target | Category | Score | Key Finding |
|------|--------|----------|-------|-------------|
| 2026-02-26 | bonanza-api | code_health | 91→~80 | 53% false positives from generated/test code |
| 2026-02-26 | bonanza-api | discoverability | 82→~68 | Suffix Jaccard bug (26%→75%), zero issues for 18 lost points |
| 2026-02-26 | bonanza-api | structure | 71→~86 | module_completeness measures uniformity not completeness, zero issues for 29 lost points |
| 2026-02-26 | bonanza-api | verifiability | 74→~82 | Test-file assertions drag safety ratio (90%→14%), zero issues for 26 lost points |
| 2026-02-26 | bonanza-api | predictability | 62→~50 | HasContext parser bug (93.5% FP), test inflation (+11pts), linear penalty cliff, 95.7% issue FP rate |
| 2026-02-26 | bonanza-api | context_quality | 21→~85 | 66/79 lost points are false positives (AI files + library conventions for API project), package dedup bug inflates 4.2x |
