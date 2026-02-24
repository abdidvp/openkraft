# OpenKraft

**Stop shipping 80% code.**

OpenKraft scores your Go codebase's AI-readiness and enforces that every module meets the quality of your best module.

## Install

```bash
go install github.com/openkraft/openkraft/cmd/openkraft@latest
```

## Quick Start

### Score your project

```bash
openkraft score .
```

```
╔══════════════════════════════════════════════╗
║       OpenKraft AI-Readiness Score           ║
║              71 / 100                        ║
╚══════════════════════════════════════════════╝

  architecture           ██████████████░  90/100  (weight: 25%)
  conventions            █████████████░░  85/100  (weight: 20%)
  patterns               ████████████░░░  80/100  (weight: 20%)
  tests                  ████████░░░░░░░  55/100  (weight: 15%)
  ai_context             ███████░░░░░░░░  50/100  (weight: 10%)
  completeness           █████████░░░░░░  60/100  (weight: 10%)

  Grade: B
```

### Check a module against the golden module

```bash
openkraft check payments
```

```
Check Report: payments vs tax (golden)

  Score: 65/100

  Missing Files:
    ✗ {module}/application/service.go
    ✗ {module}/application/service_test.go

  Missing Methods:
    ✗ NewService in service.go
    ✗ Process in service.go
```

### Check all modules

```bash
openkraft check --all
```

## Scoring Categories

| Category | Weight | What it measures |
|----------|--------|-----------------|
| Architecture | 25% | Hexagonal layers, dependency direction, module boundaries |
| Conventions | 20% | Naming consistency, file naming, receiver naming |
| Patterns | 20% | Error handling, interface compliance, constructor patterns |
| Tests | 15% | Test coverage ratio, test file presence |
| AI Context | 10% | CLAUDE.md, .cursorrules, AGENTS.md, .openkraft/ |
| Completeness | 10% | File manifest coverage, structural completeness |

## Output Formats

```bash
# JSON output
openkraft score . --json

# Shields.io badge URL
openkraft score . --badge

# Score history
openkraft score . --history
```

## CI Integration

```bash
# Fail if score drops below 70
openkraft score . --ci --min 70

# Fail if any module scores below 60
openkraft check --all --ci --min 60
```

### GitHub Actions

```yaml
name: OpenKraft
on: [push, pull_request]
jobs:
  score:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with:
          go-version: '1.24'
      - run: go install github.com/openkraft/openkraft/cmd/openkraft@latest
      - run: openkraft score . --ci --min 70
```

## MCP Server (AI Agent Integration)

OpenKraft includes a Model Context Protocol server that lets AI agents (Claude Code, Cursor) understand your codebase structure and fix issues.

### Claude Code Setup

Add to your project's `.mcp.json`:

```json
{
  "mcpServers": {
    "openkraft": {
      "command": "openkraft",
      "args": ["mcp", "serve"]
    }
  }
}
```

### Available MCP Tools

| Tool | Description |
|------|-------------|
| `openkraft_score` | Score a project's AI-readiness |
| `openkraft_check_module` | Check a module against the golden blueprint |
| `openkraft_get_blueprint` | Get the structural blueprint from the golden module |
| `openkraft_get_golden_example` | Get example code from the golden module |
| `openkraft_get_conventions` | Get detected naming conventions |
| `openkraft_check_file` | Check a specific file for issues |

## How It Works

```
Developer runs: openkraft score → "47/100"
Developer runs: openkraft check payments → "missing 9 files, 3 methods"
Developer opens Claude Code with OpenKraft MCP connected
Claude Code asks OpenKraft: "what's missing in payments?" → gets structured answer
Claude Code generates the missing code following golden module patterns
Developer runs: openkraft score → "82/100"
```

1. **Score** — Diagnose your codebase with 6 weighted categories
2. **Check** — Prescribe exactly what's missing vs your best module
3. **MCP** — Bridge to AI agents that can fix the issues

## Architecture

OpenKraft uses hexagonal architecture with pure Go AST analysis — no LLM, no WASM, fully deterministic.

```
internal/
  domain/           ← Pure domain logic, zero external deps
    scoring/        ← 6 category scorers
    golden/         ← Golden module selection + blueprint extraction
    check/          ← Module comparison engine
  application/      ← Use case orchestration
  adapters/
    inbound/
      cli/          ← Cobra commands
      mcp/          ← MCP server
    outbound/
      scanner/      ← Filesystem scanning
      detector/     ← Module boundary detection
      parser/       ← Go AST analysis
      tui/          ← Terminal UI rendering
      gitinfo/      ← Git metadata
      history/      ← Score history persistence
```

## Contributing

1. Fork and clone
2. `make test` to run all tests
3. `make build` to build the binary
4. Submit a PR

## License

MIT
