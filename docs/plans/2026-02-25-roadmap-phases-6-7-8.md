# OpenKraft Roadmap -- Consistency Coordinator for Multi-Agent Development

OpenKraft becomes the infrastructure that ensures codebase consistency when
multiple AI agents work on the same project. Not an LLM, not a linter -- the
**coordination layer** that prevents drift when code is written at machine speed.

## The Core Insight

AI models get better at writing individual files every 6 months. What they
cannot solve -- regardless of intelligence -- is **consistency across
invocations**. When 5 agents write 5 PRs, each PR is well-written, but the
project drifts: naming conventions fragment, architecture erodes, patterns
diverge. This is a coordination problem, not a capability problem. And it gets
worse as AI adoption grows -- more agents = more drift risk.

OpenKraft solves this. It detects the project's own patterns, codifies them as
an executable contract, and enforces consistency in real-time.

## Current State (Phases 1-5 Complete)

- 6 scoring categories, 24 sub-metrics
- Config-aware scoring with project-type profiles
- MCP server with 6 tools and 4 resources
- CLI with score, check, init commands
- Golden module detection and blueprint extraction
- History tracking and TUI output
- Go-only, static analysis

---

## Phase 6: Consistency Coordinator

**Goal:** Detect, codify, and enforce the project's own patterns so that any
agent writing code produces output consistent with existing conventions.

### New Capabilities

1. **`openkraft onboard`** -- Generates the **consistency contract**: a CLAUDE.md
   derived from real codebase analysis (naming conventions, golden module
   blueprint, dependency rules, statistical norms). Every rule is prescriptive
   and backed by evidence from the project itself.

2. **`openkraft fix`** -- Corrects **drift from established patterns**. Safe
   fixes (create CLAUDE.md, scaffold tests) applied directly. Complex drift
   (naming inconsistency, architecture erosion) returned as instructions with
   `project_norm` evidence -- the specific data from the project that proves
   the drift.

3. **`openkraft validate <files>`** -- Incremental **drift detection** in
   <500ms. Agent checks each change against the project's baseline. Returns
   drift issues classified by type: naming_drift, structure_drift, size_drift,
   dependency_drift.

### Agent Workflow

```
Agent receives task
  -> calls openkraft_onboard (reads consistency contract)
  -> writes code following the contract
  -> calls openkraft_validate (checks for drift)
  -> if drift: calls openkraft_fix (gets project-specific corrections)
  -> repeat until no drift
  -> opens PR (guaranteed consistent with existing codebase)
```

### Key Principles

- **Relative, not absolute.** "Function is 82 lines" means nothing. "Function
  is 82 lines in a project where p90 is 50" means drift. All thresholds come
  from the project's own statistical norms.
- **Complementary, not competitive.** Models write good code. Openkraft ensures
  it looks like it belongs in this specific project.
- **Evidence-based.** Every drift instruction includes `project_norm` -- the
  concrete evidence from the codebase that defines what "consistent" means.

**Success metric:** Agents using openkraft produce PRs with zero naming,
structure, or dependency drift from established project patterns.

---

## Phase 7: Drift Gate

**Goal:** No PR -- from any agent or human -- introduces drift that degrades
codebase consistency.

### New Capabilities

1. **`openkraft gate`** -- Branch comparison (head vs base). Detects new drift
   introduced by the PR. Fails if consistency score drops more than N points.

2. **GitHub Action** -- `uses: openkraft/action@v1`. Comments on PRs with
   drift report: what changed, what drifted, what resolved.

3. **Score trending** -- Regression detection over time. Alerts on sustained
   consistency declines across multiple commits.

### Key Principle

The same drift detection engine runs in CI. The gate catches what validate
missed -- cumulative drift across multiple files in a single PR.

**Success metric:** Zero PRs merge that silently degrade consistency by >5 pts.

---

## Phase 8: Platform SDK + Multi-Language

**Goal:** Extend drift detection beyond Go. Agent platforms integrate openkraft
as a dependency, not a CLI wrapper.

### New Capabilities

1. **Go SDK** -- `sdk.Score()`, `sdk.Onboard()`, `sdk.Validate()`,
   `sdk.Fix()`, `sdk.Gate()` as a clean programmatic API.

2. **Multi-language** -- TypeScript and Python drift detection using the same
   framework with language-specific parsers. Same 6 categories. Same
   consistency-first approach.

3. **Plugin system** -- Custom drift rules via YAML definitions (v1) and
   WASM modules (v2). Organizations codify their own consistency standards.

### Key Principle

Additive, not breaking. Go scoring unchanged. Multi-language uses the same
scoring framework with language-specific parsers. Plugins are opt-in.

**Success metric:** Drift detection works for TypeScript projects with the same
accuracy as Go. At least one external platform integrates the SDK.

---

## Dependency Chain

```
Phase 6 (Consistency Coordinator) -- standalone, builds on Phase 5
  |
  v
Phase 7 (Drift Gate) -- requires Phase 6 drift detection + fix infrastructure
  |
  v
Phase 8 (Platform SDK) -- requires Phase 6+7 stable API surface
```

## Timeline Estimate

- **Phase 6:** Core capability (onboard + fix + validate) -- 6 weeks, 16 tasks
- **Phase 7:** CI integration -- 4 weeks, 14 tasks
- **Phase 8:** SDK + multi-lang -- largest scope, split into sub-phases

## Design Principles Across All Phases

1. **Consistency over quality.** Openkraft does not judge if code is "good" --
   it judges if code is consistent with the project's own patterns. Models
   handle quality. Openkraft handles coordination.

2. **Relative over absolute.** All thresholds derive from the project's own
   statistical norms (p90 of function lengths, dominant naming pattern, golden
   module blueprint). No arbitrary global limits.

3. **Evidence over opinion.** Every drift detection includes the specific data
   from the project that proves the drift. "92% of files use bare naming" is
   evidence. "Use shorter names" is opinion.

4. **Complementary to models.** As AI models improve, openkraft becomes more
   valuable, not less. Better models writing code faster = more drift risk =
   more need for coordination.

5. **MCP-first.** Every capability is an MCP tool first, CLI command second.

6. **Additive evolution.** Each phase adds capabilities without breaking
   existing ones. Zero config works.
