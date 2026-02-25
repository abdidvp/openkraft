# OpenKraft Roadmap -- From Scoring Tool to Agent Infrastructure

OpenKraft becomes the operating system for AI code quality. Not an LLM -- the
infrastructure that makes any LLM produce production-quality code. Three phases
transform openkraft from a passive report card into an active agent companion.

## Current State (Phases 1-5 Complete)

- 6 scoring categories, 24 sub-metrics
- Config-aware scoring with project-type profiles
- MCP server with 6 tools and 4 resources
- CLI with score, check, init commands
- History tracking and TUI output
- Go-only, static analysis

---

## Phase 6: Agent Companion

**Goal:** AI agents consult openkraft in real-time to write correct code from
the first attempt.

### New Capabilities

1. **`openkraft onboard`** -- Auto-generates CLAUDE.md from real codebase
   analysis. Detects structure, patterns, conventions, and golden modules.
   Gives agents immediate project understanding.

2. **`openkraft fix`** -- Hybrid auto-fix system. Safe fixes are applied
   directly (create CLAUDE.md, add package docs, scaffold test stubs). Complex
   fixes are returned as structured instructions for the agent.

3. **`openkraft validate <files>`** -- Incremental validation with exact score
   impact. Agent checks each change in <500ms. Cached scan for performance.

### Agent Workflow

```
Agent receives task
  -> calls openkraft_onboard (understands project)
  -> writes code
  -> calls openkraft_validate (checks work)
  -> if issues: calls openkraft_fix (gets corrections)
  -> repeat until pass
  -> opens PR
```

### Key Principle

MCP-first design. The MCP tools are the primary interface for agents. CLI is
secondary.

**Success metric:** Agents using openkraft produce PRs with score >= 75
consistently.

---

## Phase 7: Quality Gate

**Goal:** No PR from any agent (or human) degrades codebase quality.

### New Capabilities

1. **`openkraft gate`** -- Branch comparison (head vs base). Fails if score
   drops more than N points.

2. **GitHub Action** -- `uses: openkraft/action@v1`. Comments on PRs with
   score diff table.

3. **Score trending** -- Regression detection over time. Alerts on sustained
   declines.

### Key Principle

The same scoring engine runs in CI. No separate system.

**Success metric:** Zero PRs merge that silently degrade score by >5 points.

---

## Phase 8: Platform SDK

**Goal:** Agent platforms integrate openkraft as a dependency, not a CLI
wrapper.

### New Capabilities

1. **Go SDK** -- `sdk.Score()`, `sdk.Onboard()`, `sdk.Validate()`,
   `sdk.Fix()`, `sdk.Gate()` as a clean programmatic API.

2. **Multi-language** -- TypeScript and Python scoring using the same
   framework with language-specific parsers.

3. **Plugin system** -- Custom scoring categories via YAML rules (v1) and
   WASM modules (v2).

### Key Principle

Additive, not breaking. Go scoring unchanged. Plugins are opt-in.

**Success metric:** At least one external platform integrates the openkraft
SDK.

---

## Dependency Chain

```
Phase 6 (Agent Companion) -- standalone, builds on Phase 5
  |
  v
Phase 7 (Quality Gate) -- requires Phase 6 scoring + fix infrastructure
  |
  v
Phase 8 (Platform SDK) -- requires Phase 6+7 stable API surface
```

## Timeline Estimate

- **Phase 6:** Core capability (onboard + fix + validate) -- largest phase
- **Phase 7:** CI integration -- medium scope, mostly new commands + GitHub Action
- **Phase 8:** SDK + multi-lang -- largest scope, can be split into sub-phases

## Design Principles Across All Phases

1. **No LLM dependency.** All analysis is deterministic (AST, heuristics,
   pattern matching). Openkraft complements LLMs, it does not compete with
   them.

2. **MCP-first.** Every capability is an MCP tool first, CLI command second.

3. **Zero config works.** Defaults are good. Config is for customization, not
   setup.

4. **Exact over fast.** Prefer accurate results over approximate ones. 500ms
   with correct data beats 50ms with guesses.

5. **Additive evolution.** Each phase adds capabilities without breaking
   existing ones.
