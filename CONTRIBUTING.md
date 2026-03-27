# Contributing to DevRune

Thanks for your interest in contributing to DevRune! Every contribution helps make AI agent configuration better for everyone.

## Reporting Bugs

Open a [GitHub Issue](https://github.com/davidarce/DevRune/issues/new?template=bug_report.md) with:

- Steps to reproduce
- Expected vs actual behavior
- DevRune version (`devrune --version`), OS, and Go version

## Suggesting Features

Open a [GitHub Issue](https://github.com/davidarce/DevRune/issues/new?template=feature_request.md) describing the problem you'd like solved and your proposed approach.

## Development Setup

```bash
# Clone your fork
git clone https://github.com/<your-user>/DevRune.git
cd DevRune

# Install git hooks and dev dependencies
make setup

# Run tests
make test

# Run linter
make lint

# Run all checks (lint + test)
make check
```

## Commit Convention

We follow [Conventional Commits](https://www.conventionalcommits.org/):

```
feat: add new feature
fix: correct a bug
refactor: restructure code without behavior change
test: add or update tests
docs: update documentation
chore: maintenance tasks
ci: CI/CD pipeline changes
```

## Pull Request Process

1. Fork the repository
2. Create a feature branch from `main`
3. Make your changes with clear, focused commits
4. Ensure `make check` passes
5. Open a PR against `main`

Keep PRs small and focused on a single change when possible.

## Code Style

- Format all code with `gofmt`
- Pass `golangci-lint` without warnings
- Add doc comments on all exported types, functions, and methods
- Write tests for new functionality

## License

By contributing, you agree that your contributions will be licensed under the [MIT License](LICENSE).
