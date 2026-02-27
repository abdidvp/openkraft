# Design: Continuous Decay Scoring for code_health

**Date**: 2026-02-26
**Status**: Approved

## Problem

The 3-tier credit system (full=1.0, partial=0.5, zero=0.0) lacks resolution. A 51-line function and a 99-line function both earn 0.5. A 101-line and 798-line function both earn 0.0. This compresses scores into a narrow 90-100 band across repos with very different quality profiles.

## Solution

Replace the 3-tier credit system with linear continuous decay across all 5 sub-metrics.

### Formula

```go
func decayCredit(value, threshold, k int) float64 {
    if value <= threshold { return 1.0 }
    credit := 1.0 - float64(value-threshold)/float64(threshold*k)
    return max(0.0, credit)
}
```

- `k=4`: zero credit at `threshold * 5` (same as current outlier boundary)
- Linear: transparent, auditable, user can reason about their score
- One formula, one parameter, applied uniformly to all 5 metrics

### Credit tables (default profile, k=4)

**function_size** (threshold=50):

| Lines | Credit |
|-------|--------|
| 50    | 1.0    |
| 75    | 0.875  |
| 100   | 0.75   |
| 150   | 0.5    |
| 250   | 0.0    |

**nesting_depth** (threshold=3):

| Depth | Credit |
|-------|--------|
| 3     | 1.0    |
| 4     | 0.917  |
| 5     | 0.833  |
| 9     | 0.5    |
| 15    | 0.0    |

**parameter_count** (threshold=4):

| Params | Credit |
|--------|--------|
| 4      | 1.0    |
| 5      | 0.9375 |
| 8      | 0.75   |
| 20     | 0.0    |

### What changes

1. Add `decayCredit()` — pure function, defined once
2. Replace `switch` tiers in all 5 `score*` functions with `decayCredit(value, effectiveMax, k)`
3. Remove outlier penalties — decay handles them (>5x → 0.0)
4. k=4 hardcoded (not configurable — YAGNI)

### What does NOT change

- API: `ScoreCodeHealth` signature unchanged
- Domain types: `SubMetric`, `CategoryScore`, `Issue` unchanged
- Issue generation: `collectCodeHealthIssues` unchanged
- Issue thresholds: aligned with scoring boundary (recent fix)
- `issueSeverity`: ratio-based tiers unchanged
- Test file relaxation, generated file exclusion, exempt patterns

## Expected impact

- Score spread widens from 10 points (90-100) to 15-20+ points
- Outlier functions naturally penalized by magnitude, not just counted
- Simpler code: remove 5 switch statements + outlier penalty loops, replace with 1-line calls
