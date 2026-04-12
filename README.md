# Aetronyx

> Spec-driven AI coding agent with built-in cost guardrails and audit-grade governance. One binary, one command.

---

## What is this?

Aetronyx runs AI-assisted coding tasks from a single spec file. You describe what you want built — acceptance criteria, budget limits, invariants — and the agent plans, iterates, verifies, and halts within your guardrails.

```bash
aetronyx run my-task.spec.yaml
```

## Current status

Early development (M2). The core agent loop, spec validation, and multi-provider LLM support (Anthropic, OpenAI, Ollama) are working. Cost guardrails, dashboard, and governance features are in progress.

## Spec format

A spec file defines the task:

```yaml
spec_version: "1"
name: refactor-auth
intent: Replace session tokens with short-lived JWTs in the auth module

budget:
  max_cost_usd: 2.00
  max_iterations: 10

acceptance_criteria:
  - given: A user with an expired token
    when:  The auth middleware validates it
    then:  A 401 is returned with code TOKEN_EXPIRED
```

See [`docs/reference/spec-format.md`](./docs/reference/spec-format.md) for the full field reference.

## Install

```bash
go install github.com/karthikcodes/aetronyx@latest
```

Or build from source:

```bash
git clone https://github.com/Karthikt2210/aetronyx.git
cd aetronyx
make build
./dist/aetronyx --help
```

## Commands

| Command | Description |
|---|---|
| `aetronyx run <spec>` | Execute the agent loop |
| `aetronyx validate <spec>` | Validate a spec file (exit 0 / 10) |
| `aetronyx spec init` | Scaffold a new spec file |
| `aetronyx spec blast-radius <spec>` | Show which files the task touches |
| `aetronyx spec list` | List all specs in the database |

## License

Apache 2.0 — see [LICENSE](./LICENSE).
