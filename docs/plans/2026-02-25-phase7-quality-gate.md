# Phase 7: Quality Gate — CI/CD Without Surprises

**Goal:** Make openkraft the quality guardian for AI-assisted development. When multiple agents generate PRs in parallel, codebase quality degrades silently. Phase 7 adds branch comparison, CI integration, and regression detection so no PR merges without a score check.

**Architecture:** Three new capabilities layered on the existing scoring infrastructure: a `gate` command for branch comparison, a GitHub Action for one-liner CI integration, and a `trend` command for historical regression detection.

**Tech Stack:** Go 1.24, existing stack (Cobra, lipgloss). GitHub Action as a composite action (bash + openkraft binary). No external services.

**Prerequisite:** Phase 6 (Agent Companion) complete. Phase 7 builds on the same scoring infrastructure but adds comparison and CI integration.

---

## Problem Analysis

AI coding agents (Claude, Copilot, Cursor) generate PRs at machine speed. A single agent can produce well-structured code, but when 3-5 agents work in parallel on different features, subtle quality regressions compound:

| Scenario | What Happens Today | What Should Happen |
|---|---|---|
| Agent adds a 200-line function | PR merges, code_health degrades silently | Gate blocks: "code_health dropped 4 points" |
| Agent skips test files | PR merges, verifiability drops | Gate blocks: "verifiability below threshold" |
| 5 PRs each drop score by 1 point | Cumulative 5-point drop over a day | Trend alerts: "3 consecutive declines in structure" |
| Agent restructures directories | Breaking change to architecture | Gate shows delta: "structure dropped 12 points" |

The core insight: absolute thresholds ("score must be > 70") are insufficient. A repo at 85 should not accept a PR that drops it to 80, even though 80 > 70. **Relative comparison against the base branch** is the correct gate.

---

## Design

### Capability 1: `openkraft gate` — Branch Comparison

The `gate` command compares the score of the current working tree (HEAD) against a base branch. It answers one question: did this PR make things worse?

#### How It Works

1. Score the current working tree (HEAD).
2. Stash uncommitted changes, checkout the base branch, score it, then restore the original state.
3. Compute the delta per category and overall.
4. Apply rules: fail if overall drops more than N points, or if any category drops below its configured `min_threshold`.

#### Implementation

```go
// internal/application/gate_service.go
type GateService struct {
    scoreService *ScoreService
    gitOps       GitOperations
}

type GateResult struct {
    BaseScore       int                    `json:"base_score"`
    HeadScore       int                    `json:"head_score"`
    Delta           int                    `json:"delta"`
    CategoryDeltas  map[string]int         `json:"category_deltas"`
    Status          string                 `json:"status"` // "pass" or "fail"
    FailReasons     []string               `json:"fail_reasons,omitempty"`
    NewIssues       []domain.Issue         `json:"new_issues,omitempty"`
    ResolvedIssues  []domain.Issue         `json:"resolved_issues,omitempty"`
}

func (g *GateService) Compare(projectPath, baseBranch string, maxDrop int) (*GateResult, error) {
    // 1. Score HEAD
    headScore, err := g.scoreService.ScoreProject(projectPath)
    if err != nil {
        return nil, fmt.Errorf("scoring HEAD: %w", err)
    }

    // 2. Score base branch
    baseScore, err := g.scoreBaseRef(projectPath, baseBranch)
    if err != nil {
        return nil, fmt.Errorf("scoring base branch %s: %w", baseBranch, err)
    }

    // 3. Compute deltas
    result := &GateResult{
        BaseScore:      baseScore.Overall,
        HeadScore:      headScore.Overall,
        Delta:          headScore.Overall - baseScore.Overall,
        CategoryDeltas: computeCategoryDeltas(baseScore, headScore),
        Status:         "pass",
    }

    // 4. Diff issues
    result.NewIssues = diffIssues(baseScore.Issues, headScore.Issues)
    result.ResolvedIssues = diffIssues(headScore.Issues, baseScore.Issues)

    // 5. Apply gate rules
    if result.Delta < -maxDrop {
        result.Status = "fail"
        result.FailReasons = append(result.FailReasons,
            fmt.Sprintf("overall score dropped %d points (max allowed: %d)", -result.Delta, maxDrop))
    }

    return result, nil
}
```

#### Git Operations Port

The gate needs to checkout a different ref and restore state. This is a new port to keep the domain clean:

```go
// internal/domain/ports.go
type GitOperations interface {
    CurrentRef() (string, error)
    StashAndCheckout(ref string) (restore func() error, err error)
    DetectBaseBranch() (string, error)
}
```

The adapter uses `os/exec` to call git commands. `StashAndCheckout` creates a stash (if dirty), checks out the target ref, and returns a restore function that checks out the original ref and pops the stash.

#### CLI Interface

```
openkraft gate [--base main] [--max-drop 5] [--json] [--format table]
```

Flags:
- `--base`: base branch to compare against (default: auto-detect from `main` or `master`)
- `--max-drop`: maximum allowed score drop before failing (default: 5)
- `--json`: output as JSON for CI parsing
- `--format table`: output as a formatted table (default)

Exit codes:
- `0`: pass (score did not degrade beyond threshold)
- `1`: fail (score degraded beyond threshold)
- `2`: error (could not complete comparison)

#### Output Format

Default table output:

```
OpenKraft Gate: PASS

Score: 82 (+3 from main)

Category        Base  Head  Delta
-----------     ----  ----  -----
code_health       93    95     +2
discoverability   85    85      0
structure         74    74      0
verifiability     89    91     +2
context_quality   15    15      0
predictability    79    79      0

New issues: 0
Resolved issues: 1
  - [code_health] Function exceeding 100 lines in handler.go (resolved)
```

JSON output (for CI consumption):

```json
{
  "base_score": 79,
  "head_score": 82,
  "delta": 3,
  "category_deltas": {
    "code_health": 2,
    "structure": 0,
    "discoverability": 0,
    "verifiability": 2,
    "context_quality": 0,
    "predictability": 0
  },
  "status": "pass",
  "new_issues": [],
  "resolved_issues": [
    {
      "category": "code_health",
      "message": "Function exceeding 100 lines in handler.go"
    }
  ]
}
```

---

### Capability 2: GitHub Action

An official `openkraft/action` composite action for one-liner CI integration. Lives in a separate repository (`openkraft/action`).

#### Usage

```yaml
# .github/workflows/openkraft.yml
name: OpenKraft Quality Gate
on: [pull_request]

jobs:
  quality-gate:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
        with:
          fetch-depth: 0  # Full history for branch comparison

      - uses: openkraft/action@v1
        with:
          min-score: 70
          max-drop: 5
          comment: true
```

#### Action Inputs

| Input | Default | Description |
|---|---|---|
| `min-score` | `0` | Absolute minimum score (fail if HEAD score is below this) |
| `max-drop` | `5` | Maximum allowed score drop from base branch |
| `comment` | `true` | Post a PR comment with the score comparison |
| `badge` | `false` | Update a score badge in the README |
| `version` | `latest` | openkraft version to install |

#### Action Implementation

The composite action:

1. Installs the openkraft binary (cached via `actions/cache`).
2. Detects the base branch from the PR context (`$GITHUB_BASE_REF`).
3. Runs `openkraft gate --base $GITHUB_BASE_REF --max-drop $MAX_DROP --json`.
4. Parses the JSON output.
5. If `comment: true`, posts or updates a PR comment via `gh pr comment`.
6. Sets the step outcome based on gate status.

```yaml
# action.yml
name: 'OpenKraft Quality Gate'
description: 'Score AI-readiness and block PRs that degrade codebase quality'
inputs:
  min-score:
    description: 'Minimum absolute score'
    default: '0'
  max-drop:
    description: 'Maximum allowed score drop from base branch'
    default: '5'
  comment:
    description: 'Post PR comment with score comparison'
    default: 'true'
  version:
    description: 'openkraft version'
    default: 'latest'
runs:
  using: 'composite'
  steps:
    - name: Install openkraft
      shell: bash
      run: |
        # Download and cache binary
        VERSION="${{ inputs.version }}"
        curl -sSL "https://github.com/openkraft/openkraft/releases/download/${VERSION}/openkraft-linux-amd64" -o /usr/local/bin/openkraft
        chmod +x /usr/local/bin/openkraft

    - name: Run quality gate
      id: gate
      shell: bash
      run: |
        RESULT=$(openkraft gate --base "$GITHUB_BASE_REF" --max-drop "${{ inputs.max-drop }}" --json)
        echo "result=$RESULT" >> "$GITHUB_OUTPUT"
        STATUS=$(echo "$RESULT" | jq -r '.status')
        echo "status=$STATUS" >> "$GITHUB_OUTPUT"

    - name: Comment on PR
      if: inputs.comment == 'true' && github.event_name == 'pull_request'
      shell: bash
      run: |
        # Generate markdown comment from gate result
        # Post or update existing comment via gh CLI
```

#### PR Comment Format

```
## OpenKraft Score: 82 (+3)

| Category | Base | Head | Delta |
|----------|------|------|-------|
| code_health | 93 | 95 | +2 |
| discoverability | 85 | 85 | 0 |
| structure | 74 | 74 | 0 |
| verifiability | 89 | 91 | +2 |
| context_quality | 15 | 15 | 0 |
| predictability | 79 | 79 | 0 |

**New issues:** 0 | **Resolved:** 1

---
*Powered by [OpenKraft](https://github.com/openkraft/openkraft)*
```

The comment uses a marker (`<!-- openkraft-gate -->`) so subsequent pushes to the same PR update the existing comment rather than creating new ones.

#### Score Caching

To avoid re-scoring the base branch on every push to the same PR:

- Cache key: `openkraft-score-{base-branch}-{base-commit-sha}`
- Cache path: `.openkraft/cache/`
- The base branch score only changes when the base branch itself changes (new commits merged to main).

---

### Capability 3: Score Trending and Regression Detection

Expand the existing history system (`.openkraft/history/scores.json`) to track per-commit scores with category breakdowns, detect sustained regressions, and provide a `trend` command.

#### History Format

Extend the existing score history file:

```json
{
  "entries": [
    {
      "commit": "abc123",
      "branch": "main",
      "timestamp": "2026-02-25T10:30:00Z",
      "overall": 82,
      "categories": {
        "code_health": 95,
        "discoverability": 85,
        "structure": 74,
        "verifiability": 91,
        "context_quality": 15,
        "predictability": 79
      }
    }
  ]
}
```

#### TrendService

```go
// internal/application/trend_service.go
type TrendService struct {
    history HistoryStore
}

type TrendResult struct {
    Entries     []HistoryEntry        `json:"entries"`
    Regressions []RegressionAlert     `json:"regressions,omitempty"`
    Direction   string                `json:"direction"` // "improving", "stable", "declining"
}

type RegressionAlert struct {
    Category      string `json:"category"`
    ConsecutiveDrops int `json:"consecutive_drops"`
    TotalDrop     int    `json:"total_drop"`
    Message       string `json:"message"`
}

func (t *TrendService) Analyze(last int) (*TrendResult, error) {
    entries, err := t.history.LastN(last)
    if err != nil {
        return nil, err
    }

    result := &TrendResult{Entries: entries}
    result.Regressions = detectRegressions(entries)
    result.Direction = overallDirection(entries)
    return result, nil
}
```

#### Regression Detection

A regression is flagged when a single category declines for 3 or more consecutive commits:

```go
func detectRegressions(entries []HistoryEntry) []RegressionAlert {
    if len(entries) < 3 {
        return nil
    }

    categories := []string{
        "code_health", "discoverability", "structure",
        "verifiability", "context_quality", "predictability",
    }

    var alerts []RegressionAlert
    for _, cat := range categories {
        consecutiveDrops := 0
        totalDrop := 0
        for i := 1; i < len(entries); i++ {
            prev := entries[i-1].Categories[cat]
            curr := entries[i].Categories[cat]
            if curr < prev {
                consecutiveDrops++
                totalDrop += prev - curr
            } else {
                if consecutiveDrops >= 3 {
                    alerts = append(alerts, RegressionAlert{
                        Category:         cat,
                        ConsecutiveDrops: consecutiveDrops,
                        TotalDrop:        totalDrop,
                        Message: fmt.Sprintf(
                            "%s declined %d points over %d consecutive commits",
                            cat, totalDrop, consecutiveDrops),
                    })
                }
                consecutiveDrops = 0
                totalDrop = 0
            }
        }
        // Check trailing regression
        if consecutiveDrops >= 3 {
            alerts = append(alerts, RegressionAlert{
                Category:         cat,
                ConsecutiveDrops: consecutiveDrops,
                TotalDrop:        totalDrop,
                Message: fmt.Sprintf(
                    "%s declined %d points over %d consecutive commits",
                    cat, totalDrop, consecutiveDrops),
            })
        }
    }
    return alerts
}
```

#### CLI Interface

```
openkraft trend [--last 30] [--json] [--category code_health]
```

Default output:

```
OpenKraft Score Trend (last 10 commits)

  95 |                          *  *
  90 |                    *  *
  85 |              *  *
  80 | *  *  *  *
     +--+--+--+--+--+--+--+--+--+--
       1  2  3  4  5  6  7  8  9  10

Direction: improving (+12 over 10 commits)

Regressions: none
```

When a regression is detected:

```
WARNING: context_quality declined 6 points over 4 consecutive commits
  commit abc123: 15 -> 13
  commit def456: 13 -> 11
  commit 789abc: 11 -> 9
  commit cde012: 9 -> 9
```

#### History Recording

The `gate` command and `score` command both record entries automatically when the `--record` flag is set (or when running in CI, detected via `$CI` environment variable):

```
openkraft score . --record     # Score and record to history
openkraft gate --record        # Gate records both base and head scores
```

---

## Architecture Changes

### New Domain Types

```go
// internal/domain/gate.go
type GateResult struct { ... }    // As defined above
type GateConfig struct {
    MaxDrop       int
    MinScore      int
    BaseBranch    string
}

// internal/domain/trend.go
type HistoryEntry struct { ... }  // As defined above
type RegressionAlert struct { ... }
type TrendResult struct { ... }
```

### New Ports

```go
// internal/domain/ports.go (additions)

type GitOperations interface {
    CurrentRef() (string, error)
    StashAndCheckout(ref string) (restore func() error, err error)
    DetectBaseBranch() (string, error)
}

type HistoryStore interface {
    Record(entry HistoryEntry) error
    LastN(n int) ([]HistoryEntry, error)
    ForBranch(branch string, n int) ([]HistoryEntry, error)
}
```

### New Application Services

```
internal/application/
  gate_service.go       # Orchestrates base vs head comparison
  gate_service_test.go
  trend_service.go      # Analyzes score history for regressions
  trend_service_test.go
```

### New Adapters

```
internal/adapters/
  outbound/
    git/
      git_ops.go        # GitOperations adapter using os/exec
      git_ops_test.go
    history/
      json_store.go     # HistoryStore adapter using .openkraft/history/scores.json
      json_store_test.go
  inbound/
    cli/
      gate.go           # openkraft gate command
      trend.go          # openkraft trend command
```

### GitHub Action (Separate Repository)

```
openkraft/action/
  action.yml            # Composite action definition
  scripts/
    install.sh          # Download and cache openkraft binary
    gate.sh             # Run gate and parse output
    comment.sh          # Generate and post PR comment
  README.md
```

---

## Task Breakdown

### Milestone Map

```
Task  1:  Domain types (GateResult, HistoryEntry, TrendResult, RegressionAlert)
Task  2:  GitOperations port + git adapter
Task  3:  HistoryStore port + JSON file adapter
Task  4:  GateService implementation
Task  5:  TrendService implementation with regression detection
Task  6:  CLI: openkraft gate command
Task  7:  CLI: openkraft trend command
Task  8:  Score recording (--record flag, auto-detect CI)
Task  9:  GitHub Action composite action
Task 10:  PR comment generation and formatting
Task 11:  Score caching for base branch
Task 12:  Integration tests
Task 13:  E2E tests (gate on openkraft's own repo)
Task 14:  Final verification
```

### Task Dependency Graph

```
    [1] Domain types
    / |  \
  /   |    \
[2]  [3]   ...
Git  History
  \   |
   \  |
   [4] GateService ----+
    |                   |
   [5] TrendService     |
    |                   |
   [6] gate CLI  [7] trend CLI
    |                   |
   [8] Score recording  |
    |                   |
   [9] GitHub Action ---+
    |
  [10] PR comment formatting
    |
  [11] Score caching
    |
  [12] Integration tests
    |
  [13] E2E tests
    |
  [14] Verification
```

---

## Task 1: Domain Types

**Files:**
- Create: `internal/domain/gate.go`
- Create: `internal/domain/trend.go`

Define `GateResult`, `GateConfig`, `HistoryEntry`, `RegressionAlert`, and `TrendResult` as pure domain structs with no external dependencies.

**Tests:** Verify struct construction and JSON serialization round-trips correctly.

**Verify:** `go test ./internal/domain/ -run TestGate -v -count=1`

---

## Task 2: GitOperations Port and Adapter

**Files:**
- Modify: `internal/domain/ports.go` -- add `GitOperations` interface
- Create: `internal/adapters/outbound/git/git_ops.go`
- Create: `internal/adapters/outbound/git/git_ops_test.go`

The adapter shells out to `git` via `os/exec`:

```go
package git

type Ops struct {
    workDir string
}

func New(workDir string) *Ops {
    return &Ops{workDir: workDir}
}

func (g *Ops) CurrentRef() (string, error) {
    out, err := g.run("rev-parse", "--abbrev-ref", "HEAD")
    return strings.TrimSpace(out), err
}

func (g *Ops) StashAndCheckout(ref string) (func() error, error) {
    originalRef, err := g.CurrentRef()
    if err != nil {
        return nil, err
    }

    // Stash if dirty
    dirty, _ := g.isDirty()
    if dirty {
        if _, err := g.run("stash", "push", "-m", "openkraft-gate"); err != nil {
            return nil, fmt.Errorf("stashing changes: %w", err)
        }
    }

    // Checkout base
    if _, err := g.run("checkout", ref); err != nil {
        return nil, fmt.Errorf("checking out %s: %w", ref, err)
    }

    restore := func() error {
        if _, err := g.run("checkout", originalRef); err != nil {
            return fmt.Errorf("restoring %s: %w", originalRef, err)
        }
        if dirty {
            if _, err := g.run("stash", "pop"); err != nil {
                return fmt.Errorf("restoring stash: %w", err)
            }
        }
        return nil
    }

    return restore, nil
}

func (g *Ops) DetectBaseBranch() (string, error) {
    // Check for common base branches
    for _, branch := range []string{"main", "master"} {
        if _, err := g.run("rev-parse", "--verify", branch); err == nil {
            return branch, nil
        }
    }
    return "", fmt.Errorf("could not detect base branch (tried main, master)")
}
```

**Tests:** Use a temporary git repository created in the test to verify stash/checkout/restore cycle.

**Verify:** `go test ./internal/adapters/outbound/git/ -v -count=1`

---

## Task 3: HistoryStore Port and Adapter

**Files:**
- Modify: `internal/domain/ports.go` -- add `HistoryStore` interface
- Create: `internal/adapters/outbound/history/json_store.go`
- Create: `internal/adapters/outbound/history/json_store_test.go`

The JSON store reads and writes `.openkraft/history/scores.json`:

```go
package history

type JSONStore struct {
    path string
}

func New(projectPath string) *JSONStore {
    return &JSONStore{
        path: filepath.Join(projectPath, ".openkraft", "history", "scores.json"),
    }
}

func (s *JSONStore) Record(entry domain.HistoryEntry) error {
    entries, _ := s.readAll() // Ignore error for new files
    entries = append(entries, entry)
    return s.writeAll(entries)
}

func (s *JSONStore) LastN(n int) ([]domain.HistoryEntry, error) {
    entries, err := s.readAll()
    if err != nil {
        return nil, err
    }
    if len(entries) <= n {
        return entries, nil
    }
    return entries[len(entries)-n:], nil
}
```

**Tests:** Record entries, read them back, verify ordering and LastN behavior.

**Verify:** `go test ./internal/adapters/outbound/history/ -v -count=1`

---

## Task 4: GateService

**Files:**
- Create: `internal/application/gate_service.go`
- Create: `internal/application/gate_service_test.go`

Orchestrates: score HEAD, checkout base, score base, compute deltas, apply rules. Uses `ScoreService`, `GitOperations`, and optionally `HistoryStore`.

**Tests:** Mock `GitOperations` and `ScoreService` to test delta computation and gate rules without requiring a real git repo.

**Verify:** `go test ./internal/application/ -run TestGate -v -count=1`

---

## Task 5: TrendService

**Files:**
- Create: `internal/application/trend_service.go`
- Create: `internal/application/trend_service_test.go`

Reads history from `HistoryStore`, runs regression detection, computes overall direction.

**Tests:**
- 3 consecutive drops in one category triggers regression alert
- 2 consecutive drops does not trigger
- Mixed improvements and declines across categories are handled correctly
- Empty history returns no regressions

**Verify:** `go test ./internal/application/ -run TestTrend -v -count=1`

---

## Task 6: CLI `openkraft gate`

**Files:**
- Create: `internal/adapters/inbound/cli/gate.go`
- Modify: `internal/adapters/inbound/cli/root.go` -- register gate command

Cobra command that wires `GateService` and formats output as table or JSON.

**Verify:** `go build -o ./openkraft ./cmd/openkraft && ./openkraft gate --help`

---

## Task 7: CLI `openkraft trend`

**Files:**
- Create: `internal/adapters/inbound/cli/trend.go`
- Modify: `internal/adapters/inbound/cli/root.go` -- register trend command

Cobra command that wires `TrendService` and renders the sparkline or table output.

**Verify:** `go build -o ./openkraft ./cmd/openkraft && ./openkraft trend --help`

---

## Task 8: Score Recording

**Files:**
- Modify: `internal/adapters/inbound/cli/score.go` -- add `--record` flag
- Modify: `internal/adapters/inbound/cli/gate.go` -- add `--record` flag

When `--record` is set (or `$CI` is detected), write the score result to the history store after scoring. The gate command records both the base and head scores.

**Verify:** `./openkraft score . --record && cat .openkraft/history/scores.json`

---

## Task 9: GitHub Action

**Repository:** `openkraft/action` (separate repo)

**Files:**
- `action.yml`
- `scripts/install.sh`
- `scripts/gate.sh`
- `scripts/comment.sh`

The composite action installs openkraft, runs the gate, and optionally posts a PR comment. Uses `actions/cache` for the binary and `gh pr comment` for PR interaction.

**Verify:** Test in a fork with a dummy PR.

---

## Task 10: PR Comment Formatting

**Files:**
- Create: `internal/adapters/outbound/formatter/pr_comment.go`
- Create: `internal/adapters/outbound/formatter/pr_comment_test.go`

Generates the markdown table for PR comments from a `GateResult`. Used by both the GitHub Action scripts and potentially by MCP tool responses.

**Verify:** `go test ./internal/adapters/outbound/formatter/ -v -count=1`

---

## Task 11: Score Caching

**Files:**
- Modify: `internal/application/gate_service.go`

Cache base branch scores keyed by `{branch}-{commit-sha}`. Check cache before scoring base. Store cache in `.openkraft/cache/`.

This avoids re-scoring the base branch when a PR receives multiple pushes. The cache is invalidated when the base branch moves forward.

**Verify:** Run gate twice on the same PR, confirm second run is faster.

---

## Tasks 12-13: Integration and E2E Tests

**Task 12 files:** `internal/application/gate_service_test.go`, `internal/application/trend_service_test.go`

Integration tests with real scoring (no mocks) on test fixtures:
- Gate comparison between two fixture directories with known score differences
- Trend analysis with a pre-built history file containing known regressions

**Task 13 files:** `tests/e2e/e2e_test.go`

E2E tests:
- `openkraft gate` on a git repo with two branches of different quality
- `openkraft trend` with recorded history
- `openkraft score --record` writes to history file

**Verify:** `go test ./... -race -count=1`

---

## Task 14: Final Verification

```bash
go clean -testcache
go test ./... -race -count=1
go build -o ./openkraft ./cmd/openkraft

# Gate on openkraft's own repo
./openkraft gate --base main --max-drop 5 --json

# Record some scores
./openkraft score . --record
./openkraft score . --record

# Check trend
./openkraft trend --last 10
```

---

## Success Criteria

| Criterion | Target |
|---|---|
| `openkraft gate` execution time | < 10 seconds (two full scores) |
| Gate exit codes | 0 = pass, 1 = fail, 2 = error |
| GitHub Action PR comment | Clear, useful, auto-updating |
| False-positive gate failures | Zero on well-maintained repos |
| Regression detection accuracy | Catches real regressions (validated on openkraft's own history) |
| Backwards compatibility | Existing commands unchanged, new commands are additive |

## Risk Mitigation

| Risk | Mitigation |
|---|---|
| `git stash` fails with conflicts | Use `git worktree` as fallback for isolated base scoring |
| Scoring takes too long for CI | Cache base branch scores by commit SHA |
| PR comment exceeds GitHub size limit | Truncate issue lists, link to full report |
| History file grows unbounded | Cap at 1000 entries, rotate oldest |
| Gate blocks legitimate refactors | `--max-drop` is configurable; document that large refactors may need a higher threshold |
| Concurrent CI runs corrupt history file | Use file locking or atomic writes |

## Execution Order

```
Week 1: Tasks 1-3 (domain types, git adapter, history store)
Week 2: Tasks 4-5 (gate service, trend service)
Week 2: Tasks 6-8 (CLI commands, recording)
Week 3: Tasks 9-11 (GitHub Action, PR comments, caching)
Week 4: Tasks 12-14 (tests, verification)
```
