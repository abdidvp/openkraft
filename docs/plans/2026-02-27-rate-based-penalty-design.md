# Design: Rate-Based Severity Penalty for code_health

**Date**: 2026-02-27
**Status**: Approved

## Problem

The volume-based severity penalty (`severity_weight / ln(funcCount+1) * scale`) penalizes large codebases disproportionately. A repo with 4000 functions and 5% violation rate loses ~10 points, while a 100-function repo with the same 5% rate loses 0. This conflicts with industry tools (SonarQube, Go Report Card) that normalize by codebase size and give all 5 benchmark repos A/A+ grades.

Concrete: fiber scores 48 (grade F) while Go Report Card gives it A+ (92.4%) and SonarQube would give it A or B.

## Solution

Replace the volume-based formula with a rate-based debt ratio, following the SonarQube SQALE model:

```go
debtRatio := severity_weight / float64(funcCount)
penalty := int(math.Round(debtRatio * severityPenaltyScale))
if hasError && penalty < 1 {
    penalty = 1
}
```

### Parameters

- **severityPenaltyScale = 150**: calibrated so that a 3% debt ratio yields ~5 points penalty, 6% yields ~9-10.
- **Severity weights**: error=3.0, warning=1.0, info=0.2 (unchanged).
- **Error floor**: at least 1 point deducted if any error-level issue exists.

### What changes

1. `severityPenalty()` formula: replace `weight / ln(funcCount+1) * 2.0` with `weight / funcCount * 150`
2. Add error floor logic
3. Change `severityPenaltyScale` from 2.0 to 150.0

### What does NOT change

- Sub-metric scoring (continuous decay)
- Issue generation and thresholds
- Severity classification (error/warning/info)
- API surface

## Expected scores

| Repo | Before (volume) | After (rate) |
|------|-----------------|-------------|
| wild-workouts | 98 | ~95 |
| chi | 94 | ~91 |
| bubbletea | 91 | ~90 |
| fiber | 48 | ~90 |
| iris | 54 | ~91 |

Spread narrows from 50 to ~10 points, aligned with industry tools.
