# CLAUDE.md — openkraft

## What is openkraft

openkraft scores software projects on AI-readiness: how easily AI agents can understand, modify, and verify code. It produces a 0-100 score across 6 categories, each measuring a dimension that predicts AI refactoring success. Currently supports Go, with multi-language support planned.

## Scoring Philosophy

**Approach A — only penalize certainties.**

When openkraft reports an error, it is indiscutible. Zero false positives affecting the score. This is the core design constraint.

- **The score is sacred.** If there is doubt about whether something is a real problem, it does not penalize. A false positive destroys more trust than 10 true positives build.
- **Issues are graded by confidence.** Error = certain problem (3x+ threshold). Warning = likely problem (1.5x+). Info = opinion, shown but barely weighted in the score (0.2 weight).
- **Rate-based penalties, not absolute counts.** Same violation rate produces the same penalty regardless of codebase size. A million-line project and a 5,000-line project are compared fairly.
- **Continuous decay, not binary pass/fail.** Functions don't go from "fine" to "terrible" at a threshold. Credit decays linearly from threshold to threshold×5, where it reaches zero.

A project of any size that maintains score 90+ is genuinely world-class in the dimensions we measure. When openkraft says there's a problem, teams act without questioning it.

## The 6 Categories

| Category | Weight | What it measures |
|----------|--------|-----------------|
| code_health | 0.25 | Function size, file size, nesting depth, parameter count, conditional complexity |
| discoverability | 0.20 | Naming uniqueness, file naming conventions, predictable structure, dependency direction |
| structure | 0.15 | Layer presence, expected files, interface contracts, module completeness |
| verifiability | 0.20 | Test presence, test naming, build reproducibility, type safety signals |
| context_quality | 0.15 | AI context files (CLAUDE.md, AGENTS.md, .cursorrules), package docs, architecture docs |
| predictability | 0.10 | Self-describing names, explicit dependencies, error message quality, consistent patterns |

Each category is a pure function in `internal/domain/scoring/`. Same input always produces the same score. The 6 categories are language-agnostic concepts — the scoring logic is universal, only the parsers are language-specific.

## Architecture

Hexagonal (Ports & Adapters). Dependencies flow inward only.

```
main.go                  Entry point
internal/
  domain/                Pure business logic, zero external deps
    scoring/             6 category scorers (pure functions)
    golden/              Golden module selection
    check/               Module comparison engine
    model.go             Score, CategoryScore, Issue types
    ports.go             Interfaces for all external deps
    profile.go           ScoringProfile (thresholds, multipliers)
    config.go            ProjectConfig, ProjectType
  application/           Use case orchestration
    score_service.go     ScoreProject() pipeline
    check_service.go     CheckModule() against blueprint
  adapters/
    inbound/
      cli/               Cobra commands (score, check, init, mcp)
      mcp/               MCP server for AI agents
    outbound/
      scanner/           Filesystem walking
      parser/            Source analysis (Go via go/ast; future: per-language parsers)
      detector/          Module boundary detection
      config/            YAML config loading
      gitinfo/           Git metadata (go-git)
      history/           Score persistence
      cache/             Analysis caching
      tui/               Terminal rendering (lipgloss)
```

### Key rules
- **domain/** imports nothing external. Ever.
- **application/** imports only domain.
- **adapters/** import application + domain. No adapter imports another adapter.
- Scorers are pure functions: `func ScoreX(profile, scan, analyzed) CategoryScore`
- Issue collectors are separate: `func collectXIssues(profile, analyzed) []Issue`

## Key Types

```go
// The score output
Score { Overall int, Categories []CategoryScore }
CategoryScore { Name, Score, Weight, SubMetrics []SubMetric, Issues []Issue }
SubMetric { Name, Score, Points, Detail }
Issue { Severity, Category, SubMetric, File, Line, Message, Pattern }

// The analysis input
ScanResult { RootPath, GoFiles, TestFiles, Layout ArchLayout, ... }
AnalyzedFile { Path, Functions []Function, TotalLines, IsGenerated, HasCGoImport, ... }
Function { Name, LineStart, LineEnd, Params, MaxNesting, MaxCondOps, StringLiteralRatio }

// Configuration
ScoringProfile { MaxFunctionLines, MaxFileLines, MaxNestingDepth, MaxParameters, ... }
ProjectConfig { ProjectType, Weights, Skip, Profile overrides, ExcludePaths }
```

## Code Conventions

### Naming
- Scorers: `Score{Category}()`, sub-scorers: `score{SubMetric}()`
- Issue collectors: `collect{Category}Issues()`
- Services: `{UseCase}Service`
- Adapters: `{Tech}{Pattern}` (GoParser, FileScanner, YAMLLoader)
- Domain models: nouns (Score, Function, Blueprint)
- Ports: `{Noun}{Verb}er` interfaces (ProjectScanner, CodeAnalyzer, ConfigLoader)

### Testing
- **Framework**: testify (assert for non-fatal, require for fatal)
- **Pattern**: table-driven tests with `t.Run`
- **Helpers**: builder functions in test files (`makeFunction`, `makeFile`, `analyzed`)
- **Fixtures**: `testdata/go-hexagonal/{perfect,inconsistent}/`
- **No mocks**: tests use real adapters with testdata
- Same package (not `_test` package)

### Error handling
- Domain: returns plain errors
- Application: wraps with context (`fmt.Errorf("scoring: %w", err)`)
- CLI: returns errors for Cobra to format

## Commands

```bash
openkraft score [path]              # Score a project (text, --json, --badge, --history)
openkraft score [path] --ci --min 70  # CI mode: exit 1 if below threshold
openkraft check [module]            # Compare module against golden blueprint
openkraft init                      # Generate .openkraft.yaml
openkraft mcp serve                 # MCP server for AI agents
```

## Development

```bash
make build      # go build -o bin/openkraft .
make test       # go test ./... -race -count=1
make lint       # golangci-lint run ./...

# Benchmark against 10 popular Go repos
go build -o openkraft .
bash scripts/bench-repos.sh
```

## Multi-language Strategy

The architecture separates language-specific concerns (parsing) from language-agnostic concerns (scoring):

- **Language-specific**: `adapters/outbound/parser/` — one parser per language, all producing the same `AnalyzedFile` + `Function` domain types
- **Language-agnostic**: `domain/scoring/` — all 6 scorers operate on domain types, not ASTs. Adding a new language means writing a parser adapter, not touching scoring logic.
- **Profile per language**: `ScoringProfile` thresholds can vary by language (e.g., Python may allow longer functions than Go). Language defaults live in `DefaultProfileForType()`.

When adding a new language: implement `CodeAnalyzer` interface, map to existing domain types, add language-specific profile defaults. The scorers, penalty system, and decay functions work unchanged.

## Calibration Targets

Based on validation against 16 popular Go repos:

| Tier | Expected Health Score | Examples |
|------|-----------------------|----------|
| Exemplary | 95-100 | uber-go/zap (98), go-chi/chi (96) |
| Professional | 88-94 | spf13/cobra (92), docker/compose (91), containerd (89) |
| Good with debt | 80-87 | hashicorp/terraform (82), ollama (81) |
| Needs work | <80 | — |

When adding new heuristics or changing thresholds, run `scripts/bench-repos.sh` and verify these tiers hold.
