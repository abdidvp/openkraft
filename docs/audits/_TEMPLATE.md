# Audit: [Category] — [Target Project]

> **Date:** YYYY-MM-DD
> **Target:** project-name (brief description)
> **Category:** scoring category audited
> **Reported Score:** X/100
> **Estimated Accurate Score:** Y/100
> **Auditors:** 4 parallel agents (false positives, false negatives, thresholds/math, issue quality)

---

## 1. Summary

One paragraph: what was tested, what the score was, and the headline finding.

## 2. Target Project Profile

| Metric | Value |
|--------|-------|
| Language | |
| Architecture | |
| Files analyzed | |
| Functions analyzed | |
| Test ratio | |

## 3. Bugs Found

| # | Type | Severity | Description | File:Line |
|---|------|----------|-------------|-----------|
| 1 | Bug / Design / UX | P0-P2 | | |

## 4. False Positives

Issues reported that should NOT have been reported.

| Category | Count | % of Total | Root Cause |
|----------|-------|------------|------------|
| | | | |

### Examples

Concrete examples with file paths and why they are false positives.

## 5. False Negatives

Real problems the scorer MISSED.

| Missing Metric | Impact | Example in Target |
|----------------|--------|-------------------|
| | | |

## 6. Threshold Analysis

How do defaults compare to industry standards (golangci-lint, revive, etc.)?

| Metric | openkraft | Industry Default | Verdict |
|--------|-----------|-----------------|---------|
| | | | |

## 7. Scoring Math

Analysis of the formula, edge cases, and fairness.

## 8. Issue Quality

Are the reported issues actionable, prioritized, and useful?

## 9. Recommendations

Prioritized list of fixes.

### P0 — Bugs
### P1 — Scoring Accuracy
### P2 — Issue Quality / UX

## 10. Fix Plan Reference

Link to the implementation plan: `docs/plans/YYYY-MM-DD-fix-<category>.md`
