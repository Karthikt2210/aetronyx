# Contributing to Aetronyx

Thank you for your interest. Aetronyx is currently in pre-release development (Milestone 1 of 6). Contributions are welcome but please read this guide first.

## Development environment

> Full setup instructions will be added at the end of Milestone 1 when the dev environment is stable.

Prerequisites (once M1 is complete):
- Go 1.23+
- Node.js 20+ (for the frontend, Milestone 3+)
- `make`
- `golangci-lint`

```sh
git clone https://github.com/karthikcodes/aetronyx
cd aetronyx
make dev
```

## Running tests

```sh
make test        # all Go tests
make lint        # golangci-lint
make fmt         # gofmt + goimports
```

## Commit conventions

We use [Conventional Commits](https://www.conventionalcommits.org/):

```
feat(agent): add Anthropic streaming adapter
fix(audit): correct Ed25519 chain hash order
docs(spec): update YAML format example
test(store): add migration rollback test
```

Types: `feat`, `fix`, `docs`, `test`, `refactor`, `chore`, `ci`

Scope is the internal package name (`agent`, `spec`, `llm`, `audit`, `cost`, `store`, etc.).

## Pull requests

- Reference the PRD acceptance criterion your PR satisfies in the description.
- All PRs must have tests per the relevant milestone PRD's testing section.
- Update `CHANGELOG.md` under `[Unreleased]`.
- Use the PR template (`.github/PULL_REQUEST_TEMPLATE.md`).

## Architecture decisions

Read `prd/00-MASTER-ARCHITECTURE.md` before proposing structural changes. Interface names, field names, and the repo layout are defined there and are not negotiable in v1.

## Code of conduct

Be direct, be respectful, be professional. No harassment, no gatekeeping.
