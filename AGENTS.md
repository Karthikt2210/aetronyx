# AGENTS.md — Aetronyx

> This file follows the Linux Foundation AGENTS.md standard adopted by 60k+ repositories. It describes how any AI coding agent should behave when working in this repository. For Claude Code–specific guidance, see [CLAUDE.md](./CLAUDE.md).

## Repository overview

Single Go module monorepo. Go binary in the root, Next.js frontend in `ui/`, specs in `prd/`, docs in `docs/`.

## Before writing any code

1. Read `prd/PRD.md` and `prd/00-MASTER-ARCHITECTURE.md`. These are the source of truth.
2. Read the relevant milestone PRD (`prd/01` through `prd/06`) for the work being done.
3. Match the repo layout in `prd/00-MASTER-ARCHITECTURE.md §2` exactly.

## Build commands

```sh
make build      # compile the Go binary
make test       # run all Go tests
make lint       # run golangci-lint
make fmt        # gofmt + goimports
make ui-build   # build the Next.js frontend
make dev        # start dev server (Go + Next.js hot reload)
```

## Rules for all agents

- Use exact interface names, method signatures, and field names from the PRD. Do not invent new ones.
- IDs are ULIDs. Timestamps are Unix milliseconds UTC.
- SQLite driver is `modernc.org/sqlite` (pure Go — no CGo, do not substitute).
- All errors must be wrapped with `fmt.Errorf("context: %w", err)`. No silent swallows.
- No `TODO` without a milestone reference, e.g. `// TODO(M2): add validator`.
- Tests are required per each milestone PRD's testing section. No untested code shipped.
- Do not push, open PRs, or modify CI configuration without explicit user instruction.
