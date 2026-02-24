# OpenKraft — Design Document

> **"Stop shipping 80% code."**
>
> The first open-source framework that guarantees AI coding agents finish what they start,
> at the quality level your codebase demands.

**Date:** 2026-02-23
**Author:** Aldrich Cortero
**Status:** Draft
**Language:** Go
**License:** MIT (proposed)
**Repository:** github.com/openkraft/openkraft (to be created)

---

## Table of Contents

1. [Vision & Problem Statement](#1-vision--problem-statement)
2. [Competitive Landscape](#2-competitive-landscape)
3. [Product Overview](#3-product-overview)
4. [Architecture](#4-architecture)
5. [Command Reference](#5-command-reference)
6. [Core Engine: The Completeness Engine](#6-core-engine-the-completeness-engine)
7. [Scoring System](#7-scoring-system)
8. [Pattern Detection System](#8-pattern-detection-system)
9. [Golden Module System](#9-golden-module-system)
10. [Multi-Agent Output System](#10-multi-agent-output-system)
11. [Watch Mode](#11-watch-mode)
12. [Module Scaffold System](#12-module-scaffold-system)
13. [Plugin Architecture](#13-plugin-architecture)
14. [Data Model](#14-data-model)
15. [Technical Stack](#15-technical-stack)
16. [Distribution & Installation](#16-distribution--installation)
17. [Testing Strategy](#17-testing-strategy)
18. [Roadmap](#18-roadmap)
19. [Go-to-Market Strategy](#19-go-to-market-strategy)
20. [Success Metrics](#20-success-metrics)

---

## 1. Vision & Problem Statement

### The 80% Problem

In 2026, AI coding agents (Claude Code, Cursor, Codex) can generate code fast, but they
consistently deliver incomplete, inconsistent, and substandard results:

- **66% of developers** report "AI solutions that are almost right, but not quite" as their
  top frustration (Zencoder, 2026)
- **45%** say "debugging AI code takes longer than writing it myself"
- IEEE Spectrum documented **silent failures** — code that appears to work but contains
  hidden bugs, removed safety checks, or fake outputs
- Addy Osmani (Google VP Engineering) named this **"The 80% Problem"** — AI gets you 80%
  there, but the last 20% costs more than doing it yourself

### The Root Cause

The problem is NOT that AI agents are dumb. The problem is:

1. **Insufficient context** — AI agents don't know your architecture, patterns, or standards
2. **No completeness checking** — nobody tells the AI "you're missing 9 files"
3. **No consistency enforcement** — module A follows patterns, module B doesn't
4. **No quality baseline** — there's no reference for "what good looks like" in YOUR project

### The Vision

OpenKraft is the **quality enforcement layer AND code generation engine** for AI-assisted
development. It analyzes your codebase, learns your patterns, and then:

1. **Diagnoses** — scores how AI-ready your codebase is
2. **Enforces** — validates that every module meets the quality of your best module
3. **Generates** — scaffolds new modules and auto-generates missing files by learning
   from your golden module, not from generic templates
4. **Monitors** — watches in real-time as AI agents write code and catches violations

```
Without OpenKraft:
  Developer → "Create payments module" → AI Agent → 60% complete, inconsistent, half-tested

With OpenKraft:
  Developer → openkraft module create payments → Complete module scaffolded from golden module
  Developer → AI Agent iterates on business logic → openkraft watch catches violations →
  Developer → openkraft check payments → "Score: 94/100. Module complete."
```

### Core Thesis

> Every codebase has a "best module" — the one with proper tests, clean architecture,
> complete documentation, and consistent patterns. OpenKraft makes that module the
> **enforced standard** for every other module.

---

## 2. Competitive Landscape

### Direct Competitors

| Tool | What It Does | What It Lacks |
|------|-------------|---------------|
| **Code Guardian** | Architectural rule enforcement for AI code | No completeness checking, no golden module comparison, no scoring |
| **AgentRules Architect** | Generates CLAUDE.md/.cursorrules/AGENTS.md | Template-based, doesn't analyze your code, no enforcement |
| **CodeScene Code Health** | Code health scoring with MCP integration | Commercial (not open source), generic metrics, no module comparison |
| **Packwerk** (Shopify) | Module boundary enforcement | Ruby-only, no completeness, no AI context |
| **arch-go / ArchUnit** | Architecture tests in Go | Manual test writing, no pattern auto-detection, no scoring |

### Indirect Competitors (Adjacent Space)

| Tool | What It Does | Why It's Not The Same |
|------|-------------|----------------------|
| **OpenCode** (100K+ stars) | Open source AI coding agent in the terminal | Generates code but has zero quality enforcement — exactly the kind of agent that NEEDS OpenKraft |
| **golangci-lint / ESLint** | Language-specific linters | Catch syntax and style issues, not architectural completeness or module consistency |
| **SonarQube** | Code quality platform with AI features | Measures generic code health metrics, doesn't compare modules, doesn't know your architecture |
| **Cursor Bugbot** | AI code review agent for PRs | Reviews what you wrote, doesn't know what you SHOULD have written and didn't |
| **Qodo (formerly CodiumAI)** | AI test generation + code review | Focused on test generation and PR review, no scoring, no module completeness |

### OpenKraft's Unique Positioning

No existing tool combines:
1. **AI-readiness scoring** (Lighthouse-style, numeric, badge-able)
2. **Golden module comparison** (your best code as the enforced standard)
3. **Completeness checking** (knows what files/methods/tests SHOULD exist)
4. **Pattern auto-detection** (learns from your code, not generic rules)
5. **Multi-agent output** (CLAUDE.md + .cursorrules + AGENTS.md from single source)
6. **Real-time watch mode** (catches violations as AI agents write code)
7. **Cross-language** (Go + TypeScript in v1, extensible via plugins)

### Moat

The scoring system creates a **flywheel**:
- Developers try `openkraft score` out of curiosity
- They see 34/100 and want to improve
- They run `openkraft init` to generate AI context files
- They use `openkraft watch` to maintain quality
- They put `openkraft score --ci` in their pipeline
- They add the badge to their README
- Other developers see the badge and try OpenKraft

### Why It's Hard to Copy

1. **The scoring algorithm is the product** — anyone can generate a CLAUDE.md, but calibrating a meaningful score across languages and architectures requires deep analysis work that compounds over time
2. **Pattern library grows with community** — every plugin, every project that adopts OpenKraft adds patterns to the ecosystem
3. **Network effect via badges** — the more READMEs show the badge, the more developers want to score their own projects
4. **Golden module comparison is project-specific** — it learns YOUR code, not generic rules, making it impossible to replicate with a one-size-fits-all approach

---

## 3. Product Overview

### One-Line Description

OpenKraft analyzes your codebase, learns your patterns, scores your AI-readiness, and
enforces that every module meets the quality of your best module.

### User Journey

```
                    ┌─────────────────┐
                    │  openkraft score │ ← "How AI-ready am I?" (curiosity)
                    └────────┬────────┘
                             │
                    ┌────────▼────────┐
                    │  openkraft init  │ ← "Let me improve my score" (adoption)
                    └────────┬────────┘
                             │
                    ┌────────▼────────┐
                    │  openkraft check │ ← "What's missing in my modules?" (enforcement)
                    └────────┬────────┘
                             │
                    ┌────────▼────────────┐
                    │  openkraft fix      │ ← "Auto-fix what's incomplete" (remediation)
                    └────────┬────────────┘
                             │
                    ┌────────▼────────────┐
                    │  openkraft watch    │ ← "Keep my score high going forward" (retention)
                    └────────┬────────────┘
                             │
                    ┌────────▼────────────┐
                    │  openkraft module   │ ← "Build new modules right from day 1" (power user)
                    └─────────────────────┘
```

### Command Summary

```bash
# Diagnose
openkraft score [path]              # AI-readiness score (Lighthouse-style)
openkraft score --ci --min 70       # CI gate: fail if score < 70
openkraft score --badge             # Generate README badge
openkraft score --history           # Score evolution over time
openkraft score --detail            # Full detailed report

# Prepare
openkraft init                      # Analyze codebase + generate everything
openkraft sync                      # Re-sync outputs from manifest
openkraft diff                      # Preview what sync would change

# Enforce
openkraft check [module]            # Completeness + consistency + quality check
openkraft check --all               # Check all modules
openkraft fix [module]              # Auto-generate missing files
openkraft fix --interactive         # Guided completion with prompts

# Monitor
openkraft watch                     # Real-time file watcher (TUI)
openkraft watch --daemon            # Background daemon mode
openkraft watch --pre-commit        # Git pre-commit hook mode

# Build
openkraft module create <name>      # Scaffold new module from golden module
openkraft module list               # List all detected modules
openkraft module compare A B        # Compare two modules side-by-side

# Blueprints
openkraft blueprint extract <module>  # Extract blueprint from existing module
openkraft blueprint list              # List available blueprints
openkraft blueprint validate          # Validate blueprint integrity

# Configuration
openkraft config init               # Interactive configuration setup
openkraft config set <key> <value>  # Set configuration value
openkraft config show               # Show current configuration

# Plugins
openkraft plugin install <name>     # Install community plugin
openkraft plugin list               # List installed plugins
openkraft plugin create <name>      # Scaffold new plugin

# Utilities
openkraft validate                  # Run all gates/validations
openkraft version                   # Show version
openkraft doctor                    # Diagnose OpenKraft installation
```

---

## 4. Architecture

### High-Level Architecture

```
┌──────────────────────────────────────────────────────────────┐
│                        CLI Layer                              │
│  (cobra commands, TUI rendering, output formatting)          │
├──────────────────────────────────────────────────────────────┤
│                     Orchestration Layer                       │
│  (command handlers, workflow coordination)                    │
├────────────┬────────────┬────────────┬───────────────────────┤
│  Scoring   │  Pattern   │  Golden    │  Output               │
│  Engine    │  Detector  │  Module    │  Generator            │
│            │            │  Engine    │  (CLAUDE.md, etc)     │
├────────────┴────────────┴────────────┴───────────────────────┤
│                      Analysis Layer                           │
│  ┌──────────┐ ┌──────────┐ ┌──────────┐ ┌─────────────────┐ │
│  │  Go AST  │ │  TS AST  │ │  File    │ │  Git            │ │
│  │  Parser  │ │  Parser  │ │  Scanner │ │  Analyzer       │ │
│  └──────────┘ └──────────┘ └──────────┘ └─────────────────┘ │
├──────────────────────────────────────────────────────────────┤
│                      Plugin System                            │
│  (language plugins, custom rules, community extensions)      │
├──────────────────────────────────────────────────────────────┤
│                      Storage Layer                            │
│  (.openkraft/ directory, manifest, cache, history)           │
└──────────────────────────────────────────────────────────────┘
```

### Directory Structure (OpenKraft Project)

```
openkraft/
├── cmd/
│   └── openkraft/
│       └── main.go                 # Entry point
├── internal/
│   ├── cli/                        # Cobra commands + TUI
│   │   ├── root.go
│   │   ├── score.go
│   │   ├── init_cmd.go
│   │   ├── check.go
│   │   ├── watch.go
│   │   ├── module.go
│   │   ├── blueprint.go
│   │   ├── fix.go
│   │   └── config.go
│   ├── engine/                     # Core engines
│   │   ├── scoring/
│   │   │   ├── scorer.go           # Main scoring orchestrator
│   │   │   ├── categories.go       # Score category definitions
│   │   │   ├── weights.go          # Category weight configuration
│   │   │   └── report.go           # Score report generation
│   │   ├── completeness/
│   │   │   ├── checker.go          # Completeness checking engine
│   │   │   ├── file_manifest.go    # Expected file detection
│   │   │   ├── method_manifest.go  # Expected method detection
│   │   │   └── comparison.go       # Golden module comparison
│   │   ├── consistency/
│   │   │   ├── checker.go          # Cross-module consistency
│   │   │   ├── naming.go           # Naming convention analysis
│   │   │   ├── structure.go        # Structural consistency
│   │   │   └── patterns.go         # Pattern compliance
│   │   └── quality/
│   │       ├── checker.go          # Quality assessment
│   │       ├── test_coverage.go    # Test infrastructure analysis
│   │       ├── error_handling.go   # Error handling patterns
│   │       └── documentation.go    # Documentation completeness
│   ├── analysis/                   # Code analysis
│   │   ├── analyzer.go             # Main analyzer interface
│   │   ├── go_analyzer.go          # Go AST analysis (go/ast)
│   │   ├── ts_analyzer.go          # TypeScript analysis (tree-sitter)
│   │   ├── file_scanner.go         # File structure analysis
│   │   ├── git_analyzer.go         # Git history analysis
│   │   └── module_detector.go      # Module boundary detection
│   ├── pattern/                    # Pattern detection
│   │   ├── detector.go             # Pattern discovery engine
│   │   ├── matcher.go              # Pattern matching
│   │   ├── catalog.go              # Built-in pattern catalog
│   │   └── custom.go               # User-defined patterns
│   ├── golden/                     # Golden module system
│   │   ├── selector.go             # Auto-detection + ranking
│   │   ├── extractor.go            # Blueprint extraction
│   │   ├── comparator.go           # Module-to-golden comparison
│   │   └── blueprint.go            # Blueprint data model
│   ├── output/                     # Multi-agent output
│   │   ├── generator.go            # Output orchestrator
│   │   ├── claude_md.go            # CLAUDE.md generator
│   │   ├── cursorrules.go          # .cursorrules generator
│   │   ├── agents_md.go            # AGENTS.md generator
│   │   └── manifest.go             # Manifest YAML generator
│   ├── watch/                      # Watch mode
│   │   ├── watcher.go              # File system watcher
│   │   ├── validator.go            # Real-time validation
│   │   ├── feedback.go             # AI feedback loop
│   │   └── tui.go                  # Terminal UI for watch mode
│   ├── scaffold/                   # Module scaffolding
│   │   ├── scaffolder.go           # Module generation
│   │   ├── template_engine.go      # Template processing
│   │   ├── interpolator.go         # Variable interpolation
│   │   └── validator.go            # Post-scaffold validation
│   ├── plugin/                     # Plugin system
│   │   ├── manager.go              # Plugin lifecycle
│   │   ├── registry.go             # Plugin registry
│   │   ├── loader.go               # Plugin loading
│   │   └── api.go                  # Plugin API interface
│   ├── config/                     # Configuration
│   │   ├── config.go               # Config data model
│   │   ├── loader.go               # Config file loading
│   │   └── defaults.go             # Default configuration
│   ├── storage/                    # Persistent storage
│   │   ├── history.go              # Score history
│   │   ├── cache.go                # Analysis cache
│   │   └── manifest.go             # Manifest persistence
│   └── tui/                        # Terminal UI components
│       ├── renderer.go             # Score rendering
│       ├── progress.go             # Progress bars
│       ├── table.go                # Table rendering
│       └── colors.go               # Color definitions
├── pkg/                            # Public API (for plugins)
│   ├── types/                      # Shared types
│   │   ├── module.go
│   │   ├── pattern.go
│   │   ├── score.go
│   │   └── blueprint.go
│   └── plugin/                     # Plugin interface
│       ├── interface.go
│       └── hooks.go
├── plugins/                        # Built-in plugins
│   ├── go/                         # Go language plugin
│   │   ├── plugin.go
│   │   ├── analyzer.go
│   │   ├── patterns.go             # Go-specific patterns
│   │   └── blueprints/             # Go blueprints
│   └── typescript/                 # TypeScript language plugin
│       ├── plugin.go
│       ├── analyzer.go
│       ├── patterns.go
│       └── blueprints/
├── testdata/                       # Test fixtures
│   ├── go-hexagonal/               # Sample Go hexagonal project
│   ├── go-clean/                   # Sample Go clean arch project
│   ├── ts-nextjs/                  # Sample Next.js project
│   └── ts-express/                 # Sample Express project
├── docs/
│   ├── README.md                   # Main documentation
│   ├── getting-started.md
│   ├── scoring.md
│   ├── patterns.md
│   ├── plugins.md
│   └── contributing.md
├── .goreleaser.yml                 # Release automation
├── Makefile
├── go.mod
└── go.sum
```

### Generated Files (In User's Project)

When a user runs `openkraft init`, these files are created:

```
user-project/
├── .openkraft/
│   ├── manifest.yaml               # Project DNA (source of truth)
│   ├── config.yaml                  # User configuration
│   ├── blueprints/                  # Extracted blueprints
│   │   └── <blueprint-name>/
│   │       ├── blueprint.yaml       # Blueprint metadata
│   │       └── structure.yaml       # File structure template
│   ├── patterns/                    # Detected patterns
│   │   └── <pattern-name>.yaml      # Pattern definition
│   ├── gates/                       # Validation gates
│   │   └── <gate-name>.yaml         # Gate definition
│   ├── history/                     # Score history
│   │   └── scores.json              # Historical scores
│   └── cache/                       # Analysis cache
│       └── analysis.json            # Cached analysis results
├── CLAUDE.md                        # Auto-generated for Claude Code
├── .cursorrules                     # Auto-generated for Cursor
└── AGENTS.md                        # Auto-generated for Codex
```

---

## 5. Command Reference

### 5.1 `openkraft score`

The viral entry point. Analyzes a codebase and produces a numeric AI-readiness score.

**Input:** Path to codebase (defaults to `.`)
**Output:** Score 0-100 with category breakdown

```
$ openkraft score .

  ╔══════════════════════════════════════════╗
  ║        OpenKraft AI-Readiness Score      ║
  ║                  47 / 100                ║
  ╚══════════════════════════════════════════╝

  Architecture Clarity     ██████████░░░░░  67/100  (weight: 25%)
  Convention Coverage      ████░░░░░░░░░░░  28/100  (weight: 20%)
  Pattern Compliance       ██████░░░░░░░░░  42/100  (weight: 20%)
  Test Infrastructure      ████████░░░░░░░  55/100  (weight: 15%)
  AI Context Quality       ██░░░░░░░░░░░░░  15/100  (weight: 10%)
  Module Completeness      ██████░░░░░░░░░  44/100  (weight: 10%)

  Run `openkraft init` to improve your score
```

**Flags:**
- `--ci` — Exit code 1 if score < min (for CI pipelines)
- `--min N` — Minimum acceptable score (default: 0)
- `--detail` — Full detailed report with per-file analysis
- `--json` — JSON output for programmatic consumption
- `--badge` — Generate markdown badge URL
- `--history` — Show score history over time
- `--category NAME` — Score only a specific category

**Badge Generation:**
```markdown
[![OpenKraft Score](https://img.shields.io/badge/openkraft-92%2F100-brightgreen)](https://github.com/openkraft/openkraft)
```

### 5.2 `openkraft init`

Analyzes the codebase and generates all configuration files.

**Process:**
1. Detect language and framework
2. Detect architecture pattern (hexagonal, clean, MVC, layered, etc.)
3. Detect modules and their boundaries
4. Auto-detect golden module (highest completeness + quality)
5. Detect recurring patterns
6. Generate manifest, blueprints, patterns, and AI context files
7. Prompt user for confirmation of golden module selection
8. Display score improvement

**Interactive Prompts:**
```
$ openkraft init

  Analyzing codebase...

  Detected: Go 1.24 + chi/v5 + sqlc + wire (hexagonal architecture)
  Modules found: 29

  Golden module auto-detected:
    → internal/tax/ (score: 91/100)
      - 15 domain files with tests
      - Complete application layer (service, ports, commands, responses)
      - Full adapter layer (HTTP handlers, repository, mappers)
      - Integration tests present
      - Documentation complete

  ? Use "tax" as your golden module? [Y/n] Y

  Generating:
    ✓ .openkraft/manifest.yaml
    ✓ .openkraft/blueprints/go-hexagonal-module/
    ✓ .openkraft/patterns/transaction-aware-repository.yaml
    ✓ .openkraft/patterns/rbac-middleware.yaml
    ✓ .openkraft/patterns/error-wrapping.yaml
    ✓ .openkraft/patterns/branch-context-filtering.yaml
    ✓ .openkraft/patterns/cursor-pagination.yaml
    ✓ CLAUDE.md
    ✓ .cursorrules
    ✓ AGENTS.md

  AI-Readiness Score: 47 → 78 (+31 improvement!)
```

### 5.3 `openkraft check`

The completeness engine. Compares a module against the golden module.

**Output Categories:**
- **COMPLETENESS** — Missing files, methods, interfaces
- **CONSISTENCY** — Deviations from golden module patterns
- **QUALITY** — Test coverage, error handling, documentation

Detailed output shown in Section 6.

### 5.4 `openkraft watch`

Real-time monitoring with Terminal UI.

**Modes:**
| Mode | Command | Use Case |
|------|---------|----------|
| TUI | `openkraft watch` | Interactive terminal while developing |
| Daemon | `openkraft watch --daemon` | Background process with notifications |
| Pre-commit | `openkraft watch --pre-commit` | Git hook integration |
| CI | `openkraft watch --ci` | GitHub Action for PR review |

Detailed design in Section 11.

### 5.5 `openkraft module create`

Scaffolds a new module based on the golden module blueprint.

**Process:**
1. Load blueprint from `.openkraft/blueprints/`
2. Prompt for entity name, fields, operations
3. Generate all files following golden module structure
4. Apply all detected patterns
5. Run post-generation validation (`go build`, `go test`, `openkraft check`)
6. Report score for new module

Detailed design in Section 12.

### 5.6 `openkraft fix`

Auto-generates missing files detected by `openkraft check`.

**Modes:**
- `openkraft fix payments` — Auto-generate all missing files silently
- `openkraft fix payments --interactive` — Prompt for each file with preview
- `openkraft fix payments --dry-run` — Show what would be generated without writing

**How it generates files:**
1. Reads the golden module's equivalent file
2. Extracts the structural pattern (imports, types, methods)
3. Replaces golden module names with target module names
4. Applies all registered patterns
5. Writes the file
6. Validates compilation

---

## 6. Core Engine: The Completeness Engine

This is OpenKraft's killer feature — the system that detects what's missing and what's
inconsistent by comparing any module against the golden module.

### How It Works

```
┌─────────────┐     ┌──────────────┐     ┌─────────────────┐
│  Target      │     │  Golden      │     │  Blueprint       │
│  Module      │────►│  Module      │────►│  (extracted)     │
│  (payments/) │     │  (tax/)      │     │  Expected files, │
└──────┬──────┘     └──────┬──────┘     │  methods, types  │
       │                    │            └────────┬────────┘
       │                    │                     │
       ▼                    ▼                     ▼
┌──────────────────────────────────────────────────────┐
│                  Comparison Engine                     │
│                                                       │
│  1. File-level comparison:                            │
│     golden has X.go → target should have X.go         │
│                                                       │
│  2. Structure-level comparison:                       │
│     golden entity has Validate() → target should too  │
│                                                       │
│  3. Pattern-level comparison:                         │
│     golden repo has getQuerier → target should too    │
│                                                       │
│  4. Quality-level comparison:                         │
│     golden has 92% test coverage → target has 0%      │
└──────────────────────────────────────────────────────┘
                          │
                          ▼
┌──────────────────────────────────────────────────────┐
│                   Check Report                        │
│  Completeness: 62%  |  Consistency: 41%  |  Quality: 58%  │
│  Missing: 9 files, 3 methods, 2 interfaces           │
│  Violations: 5 pattern violations                     │
│  Overall: 54/100                                      │
└──────────────────────────────────────────────────────┘
```

### Comparison Levels

#### Level 1: File Manifest Comparison

Golden module `tax/` has:
```
domain/tax_rule.go
domain/tax_rule_test.go
domain/tax_errors.go
domain/tax_status.go
application/tax_service.go
application/tax_ports.go
application/tax_commands.go
application/tax_responses.go
adapters/http/tax_handler.go
adapters/http/tax_routes.go
adapters/http/tax_requests.go
adapters/repository/tax_rate_repository.go
adapters/repository/tax_mappers.go
```

For target module `payments/`, OpenKraft expects:
```
domain/payment.go                     ← maps from tax_rule.go (primary entity)
domain/payment_test.go                ← maps from tax_rule_test.go
domain/payment_errors.go              ← maps from tax_errors.go
domain/payment_status.go              ← maps from tax_status.go (if entity has status)
application/payment_service.go        ← maps from tax_service.go
application/payment_ports.go          ← maps from tax_ports.go
application/payment_commands.go       ← maps from tax_commands.go
application/payment_responses.go      ← maps from tax_responses.go
adapters/http/payment_handler.go      ← maps from tax_handler.go
adapters/http/payment_routes.go       ← maps from tax_routes.go
adapters/http/payment_requests.go     ← maps from tax_requests.go
adapters/repository/payment_repository.go  ← maps from tax_rate_repository.go
adapters/repository/payment_mappers.go     ← maps from tax_mappers.go
```

Mapping rules:
- Replace golden module name with target module name
- Replace primary entity name with target entity name
- Preserve suffix patterns (_test, _errors, _status, etc.)
- Mark optional files (e.g., _status.go only if entity has status enum)

#### Level 2: Structural Comparison (AST-based)

For each file that exists, compare internal structure:

**Domain entity comparison:**
```go
// Golden (tax_rule.go) has:
type TaxRule struct { ... }
func NewTaxRule(...) (*TaxRule, error) { ... }
func (t *TaxRule) Validate() error { ... }
func (t *TaxRule) Update(...) error { ... }

// Target (payment.go) must have:
type Payment struct { ... }
func NewPayment(...) (*Payment, error) { ... }   // ← constructor
func (p *Payment) Validate() error { ... }        // ← validation
func (p *Payment) Update(...) error { ... }        // ← update method (if applicable)
```

**Repository comparison:**
```go
// Golden (tax_rate_repository.go) has:
type PostgresTaxRateRepository struct { queries *sqlc.Queries }
func (r *PostgresTaxRateRepository) getQuerier(ctx) *sqlc.Queries { ... }
func NewPostgresTaxRateRepository(q *sqlc.Queries) *PostgresTaxRateRepository { ... }

// Target (payment_repository.go) must have:
type PostgresPaymentRepository struct { queries *sqlc.Queries }
func (r *PostgresPaymentRepository) getQuerier(ctx) *sqlc.Queries { ... }  // ← CRITICAL
func NewPostgresPaymentRepository(q *sqlc.Queries) *PostgresPaymentRepository { ... }
```

**Ports (interfaces) comparison:**
```go
// Golden (tax_ports.go) has:
type TaxRuleRepository interface {
    Create(ctx, rule) error
    GetByID(ctx, id) (*TaxRule, error)
    List(ctx, params) ([]*TaxRule, error)
}

// Target (payment_ports.go) must define interfaces for declared operations
```

#### Level 3: Pattern Compliance

Check that registered patterns are applied:

```yaml
# .openkraft/patterns/transaction-aware-repository.yaml
name: transaction-aware-repository
required_in: "*_repository.go"
check:
  - type: method_exists
    name: getQuerier
    receiver: "*Postgres*Repository"
    signature: "(ctx context.Context) *sqlc.Queries"
  - type: no_direct_access
    pattern: "r\\.queries\\."
    message: "Use r.getQuerier(ctx) instead of r.queries directly"
```

#### Level 4: Quality Assessment

| Metric | How Measured | Comparison |
|--------|-------------|------------|
| Unit test coverage | Count test files per source file | Golden ratio |
| Integration tests | Presence in tests/integration/ | Exists/not exists |
| Error handling | `fmt.Errorf("...%w", err)` pattern usage | Percentage match |
| Input validation | Validate() methods, request validation | Exists/not exists |
| Documentation | docs/{module}/ directory completeness | File count match |

---

## 7. Scoring System

### Score Calculation

The overall score is a weighted average of 6 categories:

```
Overall Score = Σ (category_score × category_weight) / Σ weights

Default Weights:
  Architecture Clarity:    25%
  Convention Coverage:     20%
  Pattern Compliance:      20%
  Test Infrastructure:     15%
  AI Context Quality:      10%
  Module Completeness:     10%
```

### Category Definitions

#### Architecture Clarity (25%)

Measures how clearly the codebase is organized and whether AI agents can understand
the project structure.

| Sub-metric | Points | Description |
|-----------|--------|-------------|
| Consistent module structure | 30 | All modules follow same directory pattern |
| Layer separation | 25 | Domain doesn't import adapters, etc. |
| Dependency direction | 20 | Dependencies flow inward |
| Module boundary clarity | 15 | Clear separation between modules |
| Architecture documentation | 10 | Documented in manifest or docs/ |

#### Convention Coverage (20%)

Measures how consistently naming and coding conventions are applied.

| Sub-metric | Points | Description |
|-----------|--------|-------------|
| Naming consistency | 30 | Files, types, functions follow patterns |
| Error handling pattern | 25 | Consistent error wrapping approach |
| Import ordering | 15 | Consistent import grouping |
| File organization | 15 | Consistent file placement |
| Code style | 15 | Consistent formatting and idioms |

#### Pattern Compliance (20%)

Measures how well detected patterns are applied across the codebase.

| Sub-metric | Points | Description |
|-----------|--------|-------------|
| Pattern detection coverage | 30 | % of files where patterns should apply |
| Pattern implementation rate | 40 | % of files that correctly implement patterns |
| No anti-patterns | 30 | Absence of known violations |

#### Test Infrastructure (15%)

Measures the quality and completeness of the testing setup.

| Sub-metric | Points | Description |
|-----------|--------|-------------|
| Unit test presence | 25 | _test.go files exist for source files |
| Integration test presence | 25 | Tests in integration directory |
| Test helpers | 15 | Shared test utilities |
| Test data/fixtures | 15 | Test data management |
| CI configuration | 20 | Automated test execution |

#### AI Context Quality (10%)

Measures how well the codebase communicates its patterns to AI agents.

| Sub-metric | Points | Description |
|-----------|--------|-------------|
| CLAUDE.md exists + quality | 25 | Present and comprehensive |
| .cursorrules exists + quality | 25 | Present and comprehensive |
| AGENTS.md exists + quality | 25 | Present and comprehensive |
| OpenKraft manifest | 25 | .openkraft/ directory with manifest |

#### Module Completeness (10%)

Measures how complete modules are compared to the golden module.

| Sub-metric | Points | Description |
|-----------|--------|-------------|
| File completeness avg | 40 | Average file completeness across modules |
| Structural completeness avg | 30 | Average structural completeness |
| Documentation completeness | 30 | Per-module docs present |

### Score Grades

| Score | Grade | Color | Badge Color |
|-------|-------|-------|-------------|
| 90-100 | A+ | Green | brightgreen |
| 80-89 | A | Green | green |
| 70-79 | B | Yellow | yellow |
| 60-69 | C | Orange | orange |
| 50-59 | D | Red | red |
| 0-49 | F | Red | critical |

---

## 8. Pattern Detection System — Hybrid Detection Engine

### Philosophy

Pure AST matching (2020-era) catches exact structural matches but misses semantic
equivalents. A developer who inlines the transaction logic instead of extracting a
`getQuerier` method is doing the same thing — pure AST won't see it. OpenKraft uses
a **two-tier detection engine**: AST for exact matches (fast, deterministic) + LLM
for semantic analysis (catches intent, not just syntax).

### How Patterns Are Detected

```
┌─ Tier 1: AST Detection (Deterministic) ─────────────────────┐
│  1. Parse all files in the codebase                          │
│  2. For each AST node type (function, struct, method):       │
│     a. Extract structural signature                          │
│     b. Group by similarity                                   │
│     c. If pattern appears in >50% of modules → DETECTED      │
│  3. Confidence: based on exact match count                   │
│                                                               │
│  Speed: ~2 seconds for 30 modules                            │
│  Catches: Exact structural patterns (getQuerier, Validate)   │
│  Misses: Semantic equivalents, variant implementations       │
└──────────────────────────┬───────────────────────────────────┘
                           │
┌─ Tier 2: LLM Semantic Analysis (Intelligent) ───────────────┐
│  For files that did NOT match Tier 1 but SHOULD:             │
│  1. Send file + pattern definition + canonical example       │
│  2. LLM answers:                                             │
│     a. Does this file implement the pattern's INTENT?        │
│     b. If yes, how does it differ from canonical form?       │
│     c. Should it be refactored to match, or is the           │
│        variant acceptable?                                   │
│  3. Confidence: LLM certainty × pattern importance           │
│                                                               │
│  Speed: ~5 seconds per file (batched)                        │
│  Catches: Inlined logic, renamed methods, variant patterns   │
│  Cost: Only runs on unmatched files, not entire codebase     │
└──────────────────────────────────────────────────────────────┘
```

### Detection Modes

```yaml
# .openkraft/config.yaml
detection:
  mode: hybrid          # ast_only | hybrid | deep
  # ast_only: Tier 1 only. Fast, free, deterministic.
  # hybrid:   Tier 1 + Tier 2 on unmatched files. Default.
  # deep:     Tier 1 + Tier 2 on ALL files. Thorough but slower.
  llm_provider: anthropic  # Same config as generation engine
```

### Example: Detecting the `getQuerier` Pattern

**Tier 1 (AST):**
```
AST Analysis of internal/*/adapters/repository/*_repository.go:

  tax/adapters/repository/tax_rate_repository.go:
    method: getQuerier(ctx context.Context) *sqlc.Queries  ✓

  inventory/adapters/repository/product_repository.go:
    method: getQuerier(ctx context.Context) *sqlc.Queries  ✓

  sales/adapters/repository/sale_repository.go:
    method: getQuerier(ctx context.Context) *sqlc.Queries  ✓

  ...found in 25/29 modules → PATTERN DETECTED (AST)

  4 modules unmatched → forwarded to Tier 2
```

**Tier 2 (LLM) — on the 4 unmatched modules:**
```
  notifications/adapters/repository/notification_repository.go:
    LLM analysis: "This file inlines transaction.FromContext(ctx) directly
    in each method instead of extracting it to getQuerier. Same intent,
    different implementation."
    → SEMANTIC MATCH (confidence: 0.91)
    → Suggestion: Refactor to extract getQuerier for consistency

  audit/adapters/repository/audit_repository.go:
    LLM analysis: "This repository is read-only and intentionally skips
    transaction support. The pattern does not apply here."
    → INTENTIONAL SKIP (confidence: 0.88)
    → No violation

  2 remaining: genuinely missing → VIOLATION
```

**Result:** Instead of reporting 4 violations, OpenKraft reports 2 violations +
1 refactor suggestion + 1 intentional skip. Far more accurate than pure AST.

### Pattern Definition Schema

```yaml
# .openkraft/patterns/transaction-aware-repository.yaml
name: transaction-aware-repository
description: |
  Every repository must support transactions via context propagation.
  Uses getQuerier(ctx) to extract transaction from context or fall back to default queries.
confidence: 0.86
detected_in: 25
total_modules: 29
canonical_source: internal/tax/adapters/repository/tax_rate_repository.go

rules:
  - id: has-get-querier
    type: method_exists
    scope: "*_repository.go"
    match:
      receiver: "\\*Postgres\\w+Repository"
      name: "getQuerier"
      params: "(ctx context.Context)"
      returns: "*sqlc.Queries"
    severity: error
    message: "Repository missing getQuerier method for transaction support"

  - id: uses-get-querier
    type: method_call
    scope: "*_repository.go"
    match:
      pattern: "r\\.getQuerier\\(ctx\\)"
    must_appear_in: "every exported method"
    severity: error
    message: "Repository method must use r.getQuerier(ctx), not r.queries directly"

  - id: no-direct-queries
    type: forbidden_pattern
    scope: "*_repository.go"
    match:
      pattern: "r\\.queries\\.\\w+"
      exclude: "getQuerier"
    severity: error
    message: "Direct access to r.queries bypasses transaction support"

fix:
  type: inject_method
  template: |
    func (r *{{.ReceiverType}}) getQuerier(ctx context.Context) *sqlc.Queries {
        if tx := transaction.FromContext(ctx); tx != nil {
            return sqlc.New(tx)
        }
        return r.queries
    }
```

### Built-in Pattern Catalog

OpenKraft ships with detectors for common patterns across languages:

**Go Patterns:**
| Pattern | Description |
|---------|-------------|
| transaction-aware-repository | Context-based transaction propagation |
| constructor-validation | NewEntity() with Validate() call |
| interface-compliance | Concrete types implementing declared interfaces |
| error-wrapping | `fmt.Errorf("context: %w", err)` |
| middleware-chain | HTTP middleware application pattern |
| dependency-injection | Wire/constructor injection patterns |

**TypeScript Patterns:**
| Pattern | Description |
|---------|-------------|
| barrel-exports | index.ts re-exporting module contents |
| hook-naming | useXxx naming convention |
| component-structure | Props interface + component + export |
| error-boundary | Error handling component pattern |
| api-client | Consistent API client pattern |

---

## 9. Golden Module System

### Auto-Detection Algorithm

OpenKraft ranks all modules by a composite score to find the "best" module:

```
Golden Score = (
    file_completeness × 0.30 +    # How many expected files exist
    structural_depth × 0.25 +      # How rich is the internal structure
    test_coverage × 0.20 +         # Test file ratio
    pattern_compliance × 0.15 +    # How many patterns it follows
    documentation × 0.10           # Docs directory completeness
)
```

**Process:**
1. Detect all modules
2. Score each module
3. Rank by golden score
4. Present top candidate to user for confirmation
5. Store selection in `.openkraft/config.yaml`

### Multi-Golden Modules

For projects with different module types, OpenKraft supports multiple golden modules:

```yaml
# .openkraft/config.yaml
golden_modules:
  - module: internal/tax
    blueprint: go-hexagonal-module
    applies_to: "internal/*"
  - module: web/components/Button
    blueprint: react-component
    applies_to: "web/components/*"
```

### Blueprint Extraction

When a golden module is selected, OpenKraft extracts a blueprint:

```yaml
# .openkraft/blueprints/go-hexagonal-module/blueprint.yaml
name: go-hexagonal-module
extracted_from: internal/tax
extraction_date: 2026-02-23

structure:
  - path: "domain/{entity}.go"
    type: domain_entity
    required: true
    structural_requirements:
      - has_struct: "{Entity}"
      - has_constructor: "New{Entity}"
      - has_method: "Validate() error"

  - path: "domain/{entity}_test.go"
    type: unit_test
    required: true
    structural_requirements:
      - has_function: "TestNew{Entity}"
      - has_function: "Test{Entity}_Validate"

  - path: "domain/{entity}_errors.go"
    type: error_definitions
    required: true

  - path: "application/{module}_service.go"
    type: application_service
    required: true
    structural_requirements:
      - has_struct: "{Module}Service"
      - has_constructor: "New{Module}Service"

  - path: "application/{module}_ports.go"
    type: ports
    required: true
    structural_requirements:
      - has_interface: "{Entity}Repository"

  - path: "application/{module}_commands.go"
    type: commands
    required: true

  - path: "application/{module}_responses.go"
    type: responses
    required: true

  - path: "adapters/http/{module}_handler.go"
    type: http_handler
    required: true

  - path: "adapters/http/{module}_routes.go"
    type: http_routes
    required: true

  - path: "adapters/http/{module}_requests.go"
    type: http_requests
    required: true

  - path: "adapters/repository/{entity}_repository.go"
    type: repository
    required: true
    patterns_required:
      - transaction-aware-repository

  - path: "adapters/repository/{module}_mappers.go"
    type: mappers
    required: true

external_files:
  - path: "cmd/api/providers/{module}.go"
    type: wire_provider
    required: true

  - path: "db/migrations/{next_number}_create_{module}_tables.up.sql"
    type: migration_up
    required: true

  - path: "db/migrations/{next_number}_create_{module}_tables.down.sql"
    type: migration_down
    required: true

  - path: "db/queries/{module}.sql"
    type: sqlc_queries
    required: true

  - path: "tests/integration/{module}_test.go"
    type: integration_test
    required: true

  - path: "docs/{module}/README.md"
    type: documentation
    required: true
```

---

## 10. Multi-Agent Output System — MCP-First Architecture

### Philosophy

In 2024, the best you could do was generate static files (CLAUDE.md, .cursorrules).
AI agents read them once at context load and worked with a snapshot. In 2026, **MCP
(Model Context Protocol)** is the standard for AI agent communication. Claude Code,
Cursor, Windsurf, and others support MCP servers natively.

OpenKraft doesn't just generate files — **it IS the communication layer between your
codebase and AI agents**. Static files remain as a fallback, but the primary channel
is a live MCP server that agents can query in real-time.

### Architecture: Two Channels

```
                    ┌─ Channel 1: MCP Server (Live) ─────────────────┐
                    │  AI agent asks → OpenKraft responds in real-time│
                    │  "What patterns apply to this file?"           │
                    │  "Is this module complete?"                     │
                    │  "What's the golden module for payments?"       │
                    │  "Show me the canonical getQuerier example"     │
                    └────────────────────────────────────────────────┘

.openkraft/manifest.yaml (source of truth)
        │
        ├──► MCP Server       (live, queryable, real-time)
        │
        └──► Static Files     (fallback for agents without MCP)
             ├──► CLAUDE.md       (Claude Code optimized)
             ├──► .cursorrules    (Cursor optimized)
             └──► AGENTS.md       (Codex/OpenAI optimized)
```

### MCP Server

OpenKraft exposes an MCP server that any compatible AI agent can connect to:

```yaml
# In the user's Claude Code MCP config
{
  "mcpServers": {
    "openkraft": {
      "command": "openkraft",
      "args": ["mcp", "serve"],
      "env": {}
    }
  }
}
```

**MCP Tools exposed by OpenKraft:**

| Tool | Description | Example Use |
|------|-------------|-------------|
| `openkraft_get_patterns` | Returns patterns that apply to a given file path | Agent writing a repository asks "what patterns must I follow?" |
| `openkraft_check_file` | Validates a single file against all rules | Agent just wrote a file and wants instant feedback |
| `openkraft_get_blueprint` | Returns the blueprint for a module type | Agent about to create a new module asks "what files do I need?" |
| `openkraft_get_golden_example` | Returns the canonical implementation of a pattern | Agent asks "show me the correct getQuerier implementation" |
| `openkraft_get_module_status` | Returns completeness status for a module | Agent asks "what's still missing in payments?" |
| `openkraft_get_conventions` | Returns project conventions for a specific area | Agent asks "how should I name error variables?" |
| `openkraft_report_violation` | Watch mode pushes violations directly to agent | Agent receives "you just broke the transaction pattern" |

**MCP Resources exposed by OpenKraft:**

| Resource | URI | Description |
|----------|-----|-------------|
| Project manifest | `openkraft://manifest` | Full project configuration |
| Module blueprint | `openkraft://blueprints/{name}` | Blueprint for module type |
| Pattern definition | `openkraft://patterns/{name}` | Full pattern with rules and examples |
| Score report | `openkraft://score` | Current project score |
| Module status | `openkraft://modules/{name}` | Module completeness details |

This means an AI agent working on your project has **live access** to your architecture
rules, patterns, and quality gates — not a static snapshot from the last time you ran
`openkraft sync`.

### Static Files (Fallback)

For agents that don't support MCP (or for CI environments), OpenKraft still generates
static files from the same manifest:

### Manifest Schema

```yaml
# .openkraft/manifest.yaml
openkraft: "1.0"

project:
  name: bonanza-api
  description: "ERP system for furniture distribution"
  language: go
  version: "1.24"
  framework:
    router: chi/v5
    database: postgresql-16
    orm: sqlc
    di: wire
    testing: testcontainers-go

architecture:
  pattern: hexagonal
  layers:
    domain:
      path: "internal/{module}/domain"
      contains: [entities, value_objects, errors, tests]
      rules:
        - "ZERO external dependencies — only Go stdlib"
        - "No imports from application or adapters layers"
        - "Every entity must have a Validate() error method"
        - "Constructors must call Validate() before returning"
    application:
      path: "internal/{module}/application"
      contains: [services, ports, commands, responses, mappers]
      rules:
        - "Only depends on domain layer"
        - "Ports are interfaces, NEVER concrete types"
        - "Services receive ports via constructor injection"
    adapters_http:
      path: "internal/{module}/adapters/http"
      contains: [handlers, routes, requests]
      rules:
        - "Handlers receive application services, not repositories"
        - "All routes require RBAC middleware"
        - "Input validation on every request struct"
    adapters_repository:
      path: "internal/{module}/adapters/repository"
      contains: [repositories, mappers]
      rules:
        - "MUST use getQuerier(ctx) pattern for transaction support"
        - "NEVER access r.queries directly"
    wiring:
      path: "cmd/api/providers/{module}.go"
      contains: [wire_providers]

patterns:
  - name: transaction-aware-repository
    file: .openkraft/patterns/transaction-aware-repository.yaml
    severity: error
  - name: error-wrapping
    file: .openkraft/patterns/error-wrapping.yaml
    severity: warning
  - name: rbac-middleware
    file: .openkraft/patterns/rbac-middleware.yaml
    severity: error
  - name: branch-context-filtering
    file: .openkraft/patterns/branch-context-filtering.yaml
    severity: warning
  - name: cursor-pagination
    file: .openkraft/patterns/cursor-pagination.yaml
    severity: info

golden_modules:
  - module: internal/tax
    blueprint: go-hexagonal-module

conventions:
  naming:
    files: snake_case
    types: PascalCase
    functions: PascalCase
    variables: camelCase
    constants: PascalCase
    database_tables: snake_case
    migrations: "{number}_{description}.{up|down}.sql"
  errors:
    format: 'fmt.Errorf("{context}: %w", err)'
    domain_errors: "Defined in domain/{module}_errors.go"
  testing:
    unit: "Colocated with source files (*_test.go)"
    integration: "tests/integration/{module}_test.go"
    pattern: "t.Parallel() + suite.BeginTestTx(t)"

commands:
  build: "go build ./..."
  test: "go test ./..."
  lint: "golangci-lint run"
  migrate_up: "make migrate-up"
  migrate_down: "make migrate-down"
  codegen: "make sqlc"
  wire: "wire ./cmd/api/..."
```

### Output Generation

Each output format is optimized for how that specific AI agent consumes context.

**CLAUDE.md Generation:**
- Starts with high-priority rules (what Claude Code reads first)
- Uses imperative language ("ALWAYS use getQuerier", "NEVER import adapters in domain")
- Includes file path examples
- References golden module explicitly
- Structured with markdown headers for Claude's context window

**AGENTS.md Generation:**
- Follows OpenAI's AGENTS.md specification
- More structured sections
- Includes codebase overview, architecture, and conventions
- Optimized for Codex's prompt format

**.cursorrules Generation:**
- Uses Cursor's rule format (migrating to .mdc in .cursor/rules/)
- Concise, directive format
- Includes glob patterns for file matching

### Sync Command

```bash
$ openkraft sync

  Reading .openkraft/manifest.yaml...

  CLAUDE.md:
    + Added pattern: cursor-pagination (new)
    ~ Updated: architecture rules (1 change)
    - Removed: deprecated import rule

  .cursorrules:
    + Added pattern: cursor-pagination (new)
    ~ Updated: architecture rules (1 change)

  AGENTS.md:
    + Added pattern: cursor-pagination (new)
    ~ Updated: architecture rules (1 change)

  3 files updated. Run `openkraft diff` to preview changes before sync.
```

---

## 11. Watch Mode

### Architecture

```
┌─────────────────────────────────────────────┐
│                Watch Mode                     │
├─────────────────────────────────────────────┤
│  File Watcher (fsnotify)                     │
│  └─ Monitors: internal/, db/, cmd/, tests/  │
│                                              │
│  Event Queue                                 │
│  └─ Debounce: 500ms (batch rapid changes)   │
│                                              │
│  Validation Pipeline                         │
│  ├─ 1. Architecture check (layer violations)│
│  ├─ 2. Pattern check (registered patterns)  │
│  ├─ 3. Completeness check (vs blueprint)    │
│  └─ 4. Compilation check (go build)         │
│                                              │
│  Feedback Engine                             │
│  ├─ Terminal notification                    │
│  ├─ TUI update                              │
│  └─ AI context update (optional)            │
└─────────────────────────────────────────────┘
```

### TUI Display

```
  OpenKraft Watch — Monitoring 29 modules, 5 patterns, 12 gates
  ─────────────────────────────────────────────────────
  Score: 78/100 (↑2 since session start)

  Recent Activity:
  14:32:01  ✓  internal/payments/domain/payment.go         OK
  14:32:05  ⚠  internal/payments/adapters/repository/      VIOLATION
             │  Pattern: transaction-aware-repository
             │  Missing: getQuerier method
             └  Fix: openkraft fix payments [f]

  14:33:12  ✓  internal/payments/application/service.go    OK
  14:33:45  ✗  internal/payments/adapters/http/handler.go  ERROR
             │  Architecture: handler imports repository directly
             └  Should import via application service

  Module Status:
  tax          ████████████████████  94/100  ✓
  inventory    ████████████████░░░░  81/100  ✓
  payments     ████████░░░░░░░░░░░░  42/100  ⚠ (in progress)
  sales        ████████████████░░░░  79/100  ✓

  [q] Quit  [f] Fix current  [d] Detail  [s] Score  [r] Refresh
```

### AI Feedback Loop — MCP-First, File-Fallback

When a violation is detected, OpenKraft has **two feedback channels**:

**Channel 1: MCP Push (Real-Time) — Default in 2026**

If the AI agent is connected via MCP, OpenKraft pushes violations directly:

```
Watch detects violation
    │
    ▼
MCP Server calls: openkraft_report_violation
    │
    ▼
AI agent receives IN ITS CURRENT CONTEXT:
  {
    "tool": "openkraft_report_violation",
    "violation": {
      "file": "internal/payments/adapters/repository/payment_repository.go",
      "pattern": "transaction-aware-repository",
      "message": "Missing getQuerier method",
      "canonical_example": "internal/tax/adapters/repository/tax_rate_repository.go",
      "fix_hint": "Add getQuerier(ctx context.Context) *sqlc.Queries method"
    }
  }
```

The agent self-corrects **immediately** — no file re-reading, no context reload.
This is the difference between a supervisor yelling across the room vs tapping
your shoulder.

**Channel 2: File Injection (Fallback)**

For agents without MCP support, the classic file-based approach still works:

```yaml
# .openkraft/config.yaml
watch:
  ai_feedback:
    mode: auto           # auto | mcp_only | file_only
    # auto: Uses MCP if agent is connected, falls back to file injection
    file_target: claude_md  # Append warnings to CLAUDE.md
    auto_cleanup: true      # Remove violations from file when fixed
```

On violation (file mode), OpenKraft appends to CLAUDE.md:

```markdown
<!-- OpenKraft Watch: Active Violations -->
## ⚠ CURRENT VIOLATIONS (auto-generated by OpenKraft Watch)

- **payments/adapters/repository/payment_repository.go**: Missing `getQuerier` method.
  See `internal/tax/adapters/repository/tax_rate_repository.go` for correct pattern.
- **payments/adapters/http/payment_handler.go**: Direct repository import detected.
  Handlers must use application services, not repositories directly.

<!-- End OpenKraft Watch -->
```

When `auto_cleanup: true`, OpenKraft removes the violations block from CLAUDE.md
once the issues are fixed — keeping the file clean.

---

## 12. Module Scaffold System — Hybrid Generation Engine

### Philosophy

Most scaffolding tools in 2025 fall into two camps: **dumb templates** (`{{.Name}}`) or
**raw LLM generation** (prompt and pray). Both fail at scale. Templates produce structurally
correct but semantically empty code. LLMs produce smart code that violates your architecture.

OpenKraft takes a **hybrid approach**: AST transformation for structure + LLM augmentation
for domain intelligence. The golden module provides the skeleton; the LLM fills it with
context-aware business logic.

### Architecture: Three-Layer Generation

```
User: openkraft module create payments --spec "Handles payment processing,
      supports multiple payment methods, integrates with orders module"
         │
         ▼
┌─ Layer 1: Structure (AST) ───────────────────────────────────────┐
│  Load golden module → Parse AST → Transform names & paths        │
│  Result: Compilable skeleton with correct architecture            │
│  - domain/payment.go          (entity stub)                      │
│  - domain/payment_repository.go (port interface)                 │
│  - application/payment_service.go (service with ports)           │
│  - adapters/repository/payment_repository.go (getQuerier ✓)     │
│  - adapters/http/payment_handler.go (routes + middleware)        │
│  - wire.go, routes.go                                            │
└────────────────────────┬─────────────────────────────────────────┘
                         │
┌─ Layer 2: Domain Intelligence (LLM) ────────────────────────────┐
│  For each generated file, LLM receives:                          │
│  - The AST-generated skeleton                                    │
│  - The user's --spec description                                 │
│  - The golden module's real implementation (as reference)         │
│  - The project's registered patterns and conventions              │
│  - Related modules' public interfaces (cross-module awareness)   │
│                                                                   │
│  LLM augments:                                                   │
│  - Domain entity fields + validation rules                       │
│  - Service method business logic                                 │
│  - Handler request/response structs                              │
│  - SQL queries (sqlc .sql files)                                 │
│  - Integration points with existing modules                      │
│  - Meaningful test cases (not just "TestCreate passes")          │
└────────────────────────┬─────────────────────────────────────────┘
                         │
┌─ Layer 3: Validate & Converge ──────────────────────────────────┐
│  Loop until score >= threshold (default: 80/100):                │
│                                                                   │
│  1. go build ./internal/payments/...                             │
│  2. go vet ./internal/payments/...                               │
│  3. openkraft check payments                                     │
│  4. go test ./internal/payments/...                              │
│                                                                   │
│  If any step fails:                                              │
│  - Feed error + context back to LLM                              │
│  - LLM generates fix                                             │
│  - Apply fix and re-run pipeline                                 │
│  - Max 5 iterations (fail with report if exceeded)               │
│                                                                   │
│  Result: Module that compiles, passes tests, and scores ≥80     │
└──────────────────────────────────────────────────────────────────┘
```

### Layer 1: AST Transformation (Structure)

The structural layer guarantees architectural correctness:

1. Parse the golden module file into an AST
2. Walk the AST and identify:
   - Type names to replace (TaxRule → Payment)
   - Function names to replace (NewTaxRule → NewPayment)
   - Package names to replace
   - Import paths to update
3. Generate new AST with replacements
4. Pretty-print the AST to Go source code

This produces a **compilable skeleton** that:
- Preserves formatting and structure perfectly
- Handles edge cases (embedded types, interface assertions)
- Guarantees all registered patterns are present (getQuerier, ports, etc.)
- Takes ~2 seconds — no LLM call needed

### Layer 2: LLM Augmentation (Intelligence)

The intelligence layer transforms the skeleton into a real module. For each file,
OpenKraft constructs a targeted prompt:

```yaml
# Internal prompt construction (not user-facing)
context:
  skeleton: "<AST-generated code>"
  spec: "Handles payment processing, supports multiple payment methods"
  golden_reference: "<full golden module implementation>"
  patterns: "<registered patterns from .openkraft/patterns/>"
  related_modules:
    orders: "<orders module public interface>"
    customers: "<customers module public interface>"
  conventions: "<project CONVENTIONS.md>"

instructions:
  - "Fill the domain entity with fields appropriate for: {spec}"
  - "Add validation rules that make domain sense"
  - "Implement service methods with real business logic"
  - "Generate SQL queries following the project's sqlc patterns"
  - "Create integration points with related modules where appropriate"
  - "DO NOT modify the architectural structure (layers, imports, patterns)"
```

Key constraints for the LLM:
- **Structure is locked** — The LLM cannot change the file layout, import structure,
  or architectural patterns. Those came from Layer 1 and are sacred.
- **Logic is free** — The LLM has full creative freedom for business logic, validation,
  field definitions, and test cases.
- **Context is rich** — The LLM sees the real golden module (not a template), the
  project's conventions, and related modules' interfaces.

### Layer 3: Validate & Converge Loop

After generation, OpenKraft enters a self-healing loop:

```
Attempt 1: go build fails — missing import
  → LLM fixes import → re-run
Attempt 2: go build passes, openkraft check finds missing pattern
  → LLM adds pattern → re-run
Attempt 3: All checks pass, score = 76/100 (below threshold)
  → LLM enhances test coverage → re-run
Attempt 4: Score = 84/100 ✓
  → Module accepted
```

The loop has a hard cap of **5 iterations**. If the module cannot reach the threshold:
- OpenKraft saves the best attempt
- Reports exactly what failed and why
- Suggests manual fixes with references to the golden module

### Cross-Module Awareness

When generating a new module, OpenKraft analyzes existing modules to understand
integration opportunities:

```
openkraft module create payments --spec "..."
  │
  ├─ Scans: internal/orders/domain/ → finds Order entity
  │  └─ Detects: Order has no PaymentID field yet
  │     └─ Suggests: "Add PaymentID to Order entity? [y/n]"
  │
  ├─ Scans: internal/customers/domain/ → finds Customer entity
  │  └─ Detects: Customer could be linked to Payment
  │     └─ Suggests: "Add CustomerID to Payment entity? [y/n]"
  │
  └─ Scans: cmd/api/providers/ → finds existing Wire providers
     └─ Auto-generates: providers/payment.go with correct Wire set
```

### LLM Provider Strategy

OpenKraft is **LLM-agnostic** for the generation layer:

```yaml
# .openkraft/config.yaml
generation:
  provider: anthropic          # anthropic | openai | ollama | none
  model: claude-sonnet-4-6   # Model to use for generation
  fallback: ollama/codestral   # Local fallback if API unavailable
  offline_mode: ast_only       # Degrade to Layer 1 only if no LLM available
```

- **`none`** — Pure AST mode. OpenKraft still works, just produces skeletons
  without domain logic. The user or their AI agent fills in the rest.
- **`ollama`** — Fully local, no API keys needed. Slower but private.
- **`anthropic` / `openai`** — Best quality. Recommended for production use.

This means OpenKraft works **without any API key** — the LLM layer is an enhancement,
not a requirement. The AST layer alone still produces architecturally correct,
pattern-compliant code that scores well on structure.

---

## 13. Plugin Architecture — WASM-First

### Philosophy

Go shared-library plugins (2020-era) are fragile: they require the same Go version,
same OS, same architecture, and break on every minor update. In 2026, **WebAssembly
(WASM)** is the standard for plugin systems (used by Zed editor, Envoy, Fermyon,
Extism, and others).

OpenKraft plugins are WASM modules. Any language that compiles to WASM (Go, Rust,
TypeScript, C, Zig) can extend OpenKraft. Plugins run sandboxed, are portable across
OS/arch, and have zero version coupling with the host.

### Architecture

```
┌─ OpenKraft Host ──────────────────────────────────┐
│                                                    │
│  WASM Runtime (wazero — pure Go, no cgo)          │
│  ├─ Plugin Sandbox                                │
│  │   ├─ Memory isolation (can't access host FS)   │
│  │   ├─ Host functions (controlled API surface)    │
│  │   └─ Resource limits (CPU, memory, time)        │
│  │                                                 │
│  Host Functions (exposed to plugins):              │
│  ├─ read_file(path) → bytes                       │
│  ├─ list_files(glob) → []path                     │
│  ├─ log(level, message)                           │
│  ├─ report_pattern(pattern)                       │
│  ├─ report_violation(violation)                   │
│  └─ get_config(key) → value                       │
│                                                    │
│  Built-in (compiled into binary, NOT WASM):       │
│  ├─ Go analyzer     (uses go/ast natively)        │
│  └─ TS analyzer     (uses tree-sitter via cgo)    │
└────────────────────────────────────────────────────┘
```

**Why wazero:** Pure Go WASM runtime — no cgo dependency for the plugin system itself.
This keeps the OpenKraft binary easy to cross-compile. Only the built-in TypeScript
analyzer uses cgo (for tree-sitter).

### Plugin Interface (Guest API)

Plugins implement a set of exported WASM functions. OpenKraft provides a **Plugin SDK**
in multiple languages to make this easy:

```go
// Go Plugin SDK — openkraft-plugin-sdk-go
package sdk

// Required exports — every plugin must implement these
type Plugin interface {
    // Identity
    Name() string
    Version() string
    Languages() []string

    // Analysis (implement what you need, return nil for the rest)
    Analyzer() Analyzer
    PatternDetectors() []PatternDetector
    BlueprintGenerators() []BlueprintGenerator

    // Scoring
    ScoringRules() []ScoringRule

    // Output
    OutputGenerators() []OutputGenerator
}

type Analyzer interface {
    ParseFile(path string, content []byte) (*FileAST, error)
    DetectModules(root string) ([]Module, error)
    DetectArchitecture(modules []Module) (*Architecture, error)
}

type PatternDetector interface {
    Name() string
    Detect(files []*FileAST) ([]Pattern, error)
    Validate(file *FileAST, pattern Pattern) ([]Violation, error)
}
```

```rust
// Rust Plugin SDK — openkraft-plugin-sdk-rs
// Same interface, idiomatic Rust
pub trait Plugin {
    fn name(&self) -> &str;
    fn version(&self) -> &str;
    fn languages(&self) -> Vec<String>;
    fn analyzer(&self) -> Option<Box<dyn Analyzer>>;
    fn pattern_detectors(&self) -> Vec<Box<dyn PatternDetector>>;
    fn scoring_rules(&self) -> Vec<ScoringRule>;
}
```

### Building a Plugin

```bash
# Go plugin
$ cd my-openkraft-python-plugin
$ tinygo build -o openkraft-python.wasm -target=wasi .

# Rust plugin
$ cargo build --target wasm32-wasip1 --release

# Result: a single .wasm file, portable everywhere
```

### Plugin Discovery

```
Priority order (higher overrides lower):

1. Built-in:   Go + TypeScript (compiled into openkraft binary)
2. Local:      .openkraft/plugins/*.wasm (project-specific)
3. Installed:  ~/.openkraft/plugins/*.wasm (global)
4. Registry:   Downloaded on demand via openkraft plugin install
```

### Plugin Registry

Community plugins are published to a GitHub-based registry with a simple index:

```bash
$ openkraft plugin install python         # → downloads openkraft-python.wasm
$ openkraft plugin install rust           # → downloads openkraft-rust.wasm
$ openkraft plugin install nextjs         # → downloads openkraft-nextjs.wasm
$ openkraft plugin install django         # → downloads openkraft-django.wasm

$ openkraft plugin list
  NAME        VERSION  LANG        SOURCE     SIZE
  go          built-in Go          built-in   —
  typescript  built-in TypeScript  built-in   —
  python      0.3.0    Python      registry   2.1 MB
  nextjs      0.2.1    TypeScript  local      1.8 MB
```

### Security Model

WASM plugins are sandboxed by default:
- **No filesystem access** — plugins call host functions to read files (OpenKraft
  controls which paths are allowed)
- **No network access** — plugins cannot make HTTP calls
- **Resource limits** — max 256MB memory, 30-second timeout per operation
- **Checksums** — installed plugins are verified against registry checksums

This means untrusted community plugins cannot exfiltrate code, mine crypto, or
access files outside the project directory.

---

## 14. Data Model

### Core Types

```go
// Module represents a detected code module
type Module struct {
    Name        string            // e.g., "tax", "payments"
    Path        string            // e.g., "internal/tax"
    Language    string            // e.g., "go", "typescript"
    Files       []ModuleFile      // All files in the module
    Layers      []Layer           // Architectural layers
    Entities    []Entity          // Domain entities
    Patterns    []PatternMatch    // Patterns detected/applied
    Score       *ModuleScore      // Completeness/consistency/quality
}

// ModuleFile represents a file within a module
type ModuleFile struct {
    Path            string
    RelativePath    string         // Path relative to module root
    Layer           string         // "domain", "application", "adapters/http", etc.
    Type            string         // "entity", "service", "handler", "repository", etc.
    AST             *FileAST       // Parsed AST (language-specific)
    Structures      []Structure    // Structs, classes, interfaces
    Functions       []Function     // Functions and methods
    Imports         []Import       // Import statements
}

// Pattern represents a detected code pattern
type Pattern struct {
    Name            string
    Description     string
    Confidence      float64        // 0.0 - 1.0
    DetectedIn      int            // Number of modules
    TotalModules    int
    CanonicalSource string         // File path of best example
    Rules           []PatternRule  // Validation rules
    Fix             *PatternFix    // Auto-fix template (optional)
}

// Blueprint represents an extracted module blueprint
type Blueprint struct {
    Name            string
    ExtractedFrom   string         // Golden module path
    ExtractionDate  time.Time
    Files           []BlueprintFile
    ExternalFiles   []BlueprintFile // Files outside module (migrations, providers, etc.)
    Patterns        []string       // Required pattern names
}

// Score represents a full scoring result
type Score struct {
    Overall             int            // 0-100
    Grade               string         // "A+", "A", "B", "C", "D", "F"
    Categories          []CategoryScore
    Timestamp           time.Time
    CommitHash          string
    ModuleScores        []ModuleScore  // Per-module breakdown
}

// CategoryScore represents a single scoring category
type CategoryScore struct {
    Name        string     // "architecture", "conventions", etc.
    Score       int        // 0-100
    Weight      float64    // 0.0 - 1.0
    SubMetrics  []SubMetric
    Issues      []Issue    // Specific issues found
}

// Issue represents a specific finding
type Issue struct {
    Severity    string     // "error", "warning", "info"
    Category    string
    File        string
    Line        int
    Message     string
    Pattern     string     // Related pattern (if any)
    FixAvailable bool
}
```

---

## 15. Technical Stack

### Core Dependencies

| Purpose | Library | Why |
|---------|---------|-----|
| CLI framework | `cobra` | Industry standard for Go CLIs |
| TUI rendering | `bubbletea` + `lipgloss` | Best Go TUI framework, beautiful output |
| Go AST parsing | `go/ast` + `go/parser` | Standard library, perfect for Go analysis |
| TypeScript AST | `tree-sitter` (via cgo) | Multi-language AST parsing, fast |
| File watching | `fsnotify` | Standard Go file watcher |
| YAML parsing | `gopkg.in/yaml.v3` | Manifest and config files |
| JSON parsing | `encoding/json` | Standard library |
| Git integration | `go-git/v5` | Git history analysis without shell |
| Color output | `lipgloss` | Beautiful terminal colors |
| Testing | `testify` | Assertions and test suites |
| Progress bars | `bubbletea` | Integrated with TUI |

### Build & Release

| Purpose | Tool |
|---------|------|
| Build | `go build` with `-ldflags` for version |
| Release | `goreleaser` — cross-compilation, GitHub releases |
| Homebrew | Goreleaser generates formula automatically |
| npm wrapper | Thin shell script in npm package |
| CI/CD | GitHub Actions |
| Linting | `golangci-lint` |
| Documentation | GitHub Pages + mdBook |

---

## 16. Distribution & Installation

### Installation Methods

```bash
# Homebrew (macOS/Linux)
brew install openkraft/tap/openkraft

# Go install
go install github.com/openkraft/openkraft/cmd/openkraft@latest

# curl (universal)
curl -sSL https://openkraft.dev/install | sh

# npm wrapper (for Node.js developers)
npx openkraft score .
# or
npm install -g openkraft
```

### Release Artifacts

Each release produces:
- `openkraft-darwin-amd64` (macOS Intel)
- `openkraft-darwin-arm64` (macOS Apple Silicon)
- `openkraft-linux-amd64` (Linux x86_64)
- `openkraft-linux-arm64` (Linux ARM)
- `openkraft-windows-amd64.exe` (Windows)
- Homebrew formula
- npm package (wrapper)
- Docker image (for CI)
- SHA256 checksums

---

## 17. Testing Strategy

### Test Pyramid

```
                    ┌──────────────┐
                    │  E2E Tests   │  ← Full CLI runs against test projects
                    │  (10%)       │
                    ├──────────────┤
                    │ Integration  │  ← Engine + Analysis + Output together
                    │  (30%)       │
                    ├──────────────┤
                    │  Unit Tests  │  ← Each engine, analyzer, detector
                    │  (60%)       │
                    └──────────────┘
```

### Test Fixtures (testdata/)

Pre-built project structures for testing:

```
testdata/
├── go-hexagonal/              # Complete Go hexagonal project
│   ├── perfect/               # All modules complete (score ~95)
│   ├── incomplete/            # Some modules missing files (score ~50)
│   ├── inconsistent/          # Modules follow different patterns (score ~40)
│   └── empty/                 # Bare minimum project (score ~15)
├── go-clean/                  # Go clean architecture variant
├── ts-nextjs/                 # Next.js project
│   ├── perfect/
│   ├── incomplete/
│   └── inconsistent/
└── ts-express/                # Express.js project
```

### Key Test Cases

- **Score determinism** — Same codebase always produces same score
- **Pattern detection** — Correctly identifies all registered patterns
- **Golden module selection** — Picks the most complete module
- **Completeness check** — Detects all missing files and structures
- **Cross-module consistency** — Finds deviations between modules
- **Fix generation** — Generated files compile and pass tests
- **Watch mode** — Detects violations on file change
- **Multi-language** — Handles Go and TypeScript in same project

---

## 18. Roadmap

### Phase 1: MVP — "The Score" (Weeks 1-3)

**Goal:** `openkraft score` working on any Go project.

Deliverables:
- [ ] Project scaffolding (Go, cobra, bubbletea)
- [ ] Go AST analyzer (module detection, structure analysis)
- [ ] File scanner (directory structure, naming analysis)
- [ ] Scoring engine (6 categories, weighted average)
- [ ] Score TUI output (colored bars, grades)
- [ ] `openkraft score` command
- [ ] `openkraft score --json` for CI
- [ ] Badge generation
- [ ] Test suite with fixtures
- [ ] README with demo GIF

### Phase 2: Init + Check — "The Engine" (Weeks 4-6)

**Goal:** `openkraft init` + `openkraft check` working.

Deliverables:
- [ ] Pattern detection engine
- [ ] Golden module auto-detection
- [ ] Blueprint extraction
- [ ] Manifest generation
- [ ] CLAUDE.md generator
- [ ] .cursorrules generator
- [ ] AGENTS.md generator
- [ ] `openkraft init` command
- [ ] `openkraft check` (completeness engine)
- [ ] `openkraft sync` command

### Phase 3: Fix + Watch — "The Enforcer" (Weeks 7-9)

**Goal:** `openkraft fix` + `openkraft watch` working.

Deliverables:
- [ ] Hybrid generation engine for `fix` (AST structure + LLM augmentation)
- [ ] `openkraft fix` command (auto-fix with validate & converge loop)
- [ ] `openkraft fix --interactive` (review each fix before applying)
- [ ] File watcher with fsnotify
- [ ] Watch TUI
- [ ] Watch daemon mode
- [ ] Pre-commit hook integration
- [ ] AI feedback loop (CLAUDE.md injection)

### Phase 4: Module + TypeScript — "The Platform" (Weeks 10-12)

**Goal:** `openkraft module create` + TypeScript support.

Deliverables:
- [ ] Module scaffolding engine
- [ ] Interactive entity/field prompts
- [ ] Post-generation validation
- [ ] TypeScript analyzer (tree-sitter)
- [ ] TypeScript pattern detection
- [ ] TypeScript blueprints (React, Next.js, Express)
- [ ] Plugin system foundation

### Phase 5: Launch — "The Movement" (Week 13)

**Goal:** Public launch with maximum impact.

Deliverables:
- [ ] openkraft.dev website
- [ ] Demo video (3 minutes)
- [ ] Blog post: "The 80% Problem is real. Here's how I'm fixing it."
- [ ] Hacker News submission
- [ ] Reddit posts (r/golang, r/programming, r/typescript)
- [ ] Twitter/X thread with demo GIF
- [ ] GitHub release v0.1.0
- [ ] Homebrew tap
- [ ] npm wrapper package

### Future (Post-Launch)

- [ ] Python support plugin
- [ ] Rust support plugin
- [ ] CI/CD GitHub Action (`openkraft-action`)
- [ ] VS Code extension (score in status bar)
- [ ] Leaderboard (public ranking of repos by AI-readiness)
- [ ] Team features (shared patterns, org-wide scoring)
- [ ] SaaS dashboard (score history, team analytics)

---

## 19. Go-to-Market Strategy

### Positioning

**Primary narrative:** "The 80% Problem is killing AI-assisted development. OpenKraft
is the fix."

**Target audience (in order):**
1. Individual developers using Claude Code / Cursor / Codex daily
2. Tech leads maintaining codebases with AI-generated code
3. Teams adopting AI coding agents at scale

### Launch Playbook

**Week -1 (Before launch):**
- Create demo video showing `openkraft score` on a real project
- Write blog post tying to Addy Osmani's "The 80% Problem"
- Create Twitter/X thread with GIF previews
- Set up openkraft.dev landing page

**Launch Day:**
- GitHub release v0.1.0
- Hacker News: "Show HN: OpenKraft — Stop shipping 80% AI-generated code"
- Reddit: r/golang, r/programming, r/typescript
- Twitter/X: Thread + tag Addy Osmani, Thorsten Ball, Mitchell Hashimoto
- Dev.to: Full tutorial post

**Week +1 (Post-launch):**
- Respond to all GitHub issues
- Write follow-up based on community feedback
- Submit to awesome-claude-code, awesome-cursorrules lists

### Viral Mechanics

1. **Score curiosity** — developers run `openkraft score` just to see their number
2. **Badge vanity** — developers want `openkraft: 92/100` badge on their README
3. **Social sharing** — "My project went from 34 to 87 with OpenKraft"
4. **CI enforcement** — once one team member adds it, everyone uses it
5. **Blog posts** — "How I achieved 95/100 AI-readiness with OpenKraft"

---

## 20. Success Metrics

### Month 1

| Metric | Target |
|--------|--------|
| GitHub stars | 1,000+ |
| Unique installs | 500+ |
| `openkraft score` runs | 2,000+ |
| Blog post views | 10,000+ |
| HN upvotes | 100+ |

### Month 3

| Metric | Target |
|--------|--------|
| GitHub stars | 5,000+ |
| Unique installs | 3,000+ |
| Community plugins | 3+ |
| Contributors | 10+ |
| README badges in the wild | 100+ |

### Month 6

| Metric | Target |
|--------|--------|
| GitHub stars | 15,000+ |
| Unique installs | 10,000+ |
| Community plugins | 10+ |
| CI pipeline integrations | 500+ |
| Conference talk / podcast | 1+ |

### North Star Metric

**Number of `openkraft score --ci` runs per week** — this means people are enforcing
quality in their pipelines, which is the strongest signal of real adoption.

---

## Appendix A: Inspiration and References

- **Lighthouse** (Google) — Web performance scoring; inspired the score-first approach
- **Packwerk** (Shopify) — Module boundary enforcement; inspired the architecture validation
- **ArchUnit** (Java) — Architecture testing; inspired pattern compliance checking
- **OpenClaw** (Peter Steinberger) — Viral open source launch strategy
- **The 80% Problem** (Addy Osmani) — The core problem statement
- **Docker** — Creating a standard where none existed; same ambition

## Appendix B: Name and Branding

- **Name:** OpenKraft
- **Tagline:** "Stop shipping 80% code."
- **Domain:** openkraft.dev (to be registered)
- **GitHub:** github.com/openkraft/openkraft
- **Logo concept:** An anvil or forge mark — symbolizing craft, strength, quality
- **Color palette:** Deep blue (#1a1a2e) + electric green (#00ff88) for the score
- **Mascot (optional):** A craftsman/blacksmith character — "The Kraft"

## Appendix C: Risk Analysis

| Risk | Impact | Mitigation |
|------|--------|------------|
| Tree-sitter integration complexity | High | Start with Go (native AST), add TS in Phase 4 |
| AST-based code gen produces broken code | High | Extensive test fixtures, compilation validation |
| Low adoption (nobody cares about scores) | Medium | Focus on HN launch narrative, demo quality |
| Competitor launches similar tool | Medium | Speed to market, community building, open source moat |
| Scope creep delays launch | High | Strict phase discipline, MVP is ONLY `openkraft score` |
| Plugin API instability | Medium | Keep pkg/plugin minimal in v1, expand based on demand |
