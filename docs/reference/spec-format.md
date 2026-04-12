# Aetronyx Spec Format Reference (v1)

A spec file is a YAML document that fully describes an AI-assisted task: its goal,
budget guardrails, acceptance tests, and routing hints. Aetronyx validates every field
before starting a run.

---

## Top-level fields

| Field | Type | Required | Description |
|---|---|---|---|
| `spec_version` | string | yes | Must be `"1"` |
| `name` | string | yes | Kebab-case identifier matching `^[a-z0-9][a-z0-9-]{0,63}$` |
| `intent` | string | yes | One-sentence goal (≥20 chars recommended, ≥80 for best results) |
| `budget` | object | no | Cost and iteration guardrails |
| `acceptance_criteria` | list | yes | GWT scenarios; at least one required |
| `invariants` | list | no | Rules the agent must never violate |
| `out_of_scope` | list | no | Paths/features the agent must not touch |
| `dependencies` | object | no | Files, services, and APIs the task depends on |
| `test_contracts` | list | no | Shell commands that verify acceptance criteria |
| `approval_gates` | list | no | Human-approval hooks at specific phases |
| `routing_hint` | object | no | Override planning/execution model selection |
| `metadata` | map[string]string | no | Arbitrary key-value pairs for tooling |

---

## `budget`

```yaml
budget:
  max_cost_usd: 2.00          # (0, 10000] — hard spend ceiling
  max_iterations: 10          # [1, 500]  — loop iteration limit
  max_wall_time_minutes: 30   # [1, 720]  — wall-clock timeout
  max_tokens: 200000          # [1000, 10000000] — total token budget
```

Aetronyx halts the run (status `halted_max_iters`, `halted_max_time`, or
`halted_budget`) the moment any limit is exceeded. Unset fields are not validated.

---

## `acceptance_criteria`

Given-When-Then scenarios that describe the definition of done.
Each scenario must map to at least one `test_contracts` entry.

```yaml
acceptance_criteria:
  - given: A user with an expired session token
    when:  The auth middleware validates the token
    then:  A 401 response is returned with error code TOKEN_EXPIRED

  - given: A valid JWT with required claims
    when:  The auth service verifies it
    then:  The user context is populated and the request proceeds
```

All three fields (`given`, `when`, `then`) must be non-empty.

---

## `invariants`

Constraints the agent must uphold throughout the run. Each entry must be ≥10 chars.

```yaml
invariants:
  - No plaintext secrets committed to the repository
  - All database access must go through the store.Store typed methods
  - Public API surface must remain backwards-compatible
```

---

## `out_of_scope`

Paths or features the agent must not modify. Warn if any path also appears in
`dependencies.files` (rule 15).

```yaml
out_of_scope:
  - internal/billing/
  - ui/
  - Any changes to the public REST API shape
```

---

## `dependencies`

```yaml
dependencies:
  files:
    - internal/auth/          # glob-expanded; fatal if nothing matches
    - internal/store/
  services:
    - redis                   # checked: localhost:6379
    - postgres                # checked: localhost:5432
  apis:
    - github.com/golang-jwt/jwt/v5
```

`dependencies.files` entries are glob-expanded relative to the workspace.
The validator emits a fatal error if any pattern matches no files (rule 9).

---

## `test_contracts`

Shell commands verified after each iteration. A failing command causes the agent
to feed the output back into the next iteration (up to `max_iterations`).

```yaml
test_contracts:
  - name: auth-unit-tests
    command: go test ./internal/auth/...
    maps_to:
      - acceptance_criteria[0]
      - acceptance_criteria[1]
```

`maps_to` entries must reference existing `acceptance_criteria[N]` or `invariants[N]`
indices. The first token of `command` must be on `PATH` (rule 11).

---

## `approval_gates`

Optional human-approval checkpoints. `after`/`before` must be one of:
`planning`, `schema_change`, `pre_merge`, `iteration`, or a `custom:*` prefix.

```yaml
approval_gates:
  - after: planning
    required: true
  - before: pre_merge
    required: true
```

---

## `routing_hint`

Override the model used for planning and execution. Both values must exist in
the configured adapter's pricing table (rule 14).

```yaml
routing_hint:
  planning_model: claude-sonnet-4-6
  execution_model: claude-haiku-4-5-20251001
```

---

## Complete annotated example

See `examples/refactor-auth.spec.yaml` for a full working example.

---

## Validation exit codes

| Code | Meaning |
|---|---|
| `0` | Valid (warnings allowed) |
| `10` | One or more fatal validation errors |

Pass `--strict` to promote warnings to fatal errors.
