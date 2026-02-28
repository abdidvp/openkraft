# Contributing to OpenKraft

Thanks for your interest in contributing to OpenKraft!

## Getting Started

1. Fork the repository
2. Clone your fork: `git clone https://github.com/<your-user>/openkraft.git`
3. Create a branch: `git checkout -b my-feature`
4. Make your changes
5. Run tests: `make test`
6. Run linter: `make lint`
7. Commit and push
8. Open a Pull Request

## Development

```bash
make build      # Build binary to bin/openkraft
make test       # Run tests with race detector
make lint       # Run golangci-lint
```

## Architecture

OpenKraft uses hexagonal architecture. Dependencies flow inward only:

- **domain/** — Pure business logic, zero external dependencies
- **application/** — Use case orchestration, imports only domain
- **adapters/** — External integrations, imports application + domain

Key rules:
- Scorers are pure functions: same input always produces the same score
- No adapter imports another adapter
- Domain types are language-agnostic; only parsers are language-specific

## Testing

- Use `testify` (assert for non-fatal, require for fatal)
- Table-driven tests with `t.Run`
- No mocks — tests use real adapters with `testdata/` fixtures
- Run the benchmark after changing thresholds: `bash scripts/bench-repos.sh`

## Pull Requests

- Keep PRs focused on a single change
- Include tests for new functionality
- Ensure CI passes (tests + lint)
- Update documentation if behavior changes

## Reporting Issues

Use GitHub Issues. Include:
- What you expected
- What happened
- Steps to reproduce
- `openkraft` version (`openkraft --version`)
