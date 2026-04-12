# Example Specs

Five reference `.spec.yaml` files covering the most common agent task patterns. Each conforms to spec format v1 and can be used as a starting template.

| File | Language / Framework | What it demonstrates |
|------|---------------------|----------------------|
| [01-add-rate-limiting.spec.yaml](./01-add-rate-limiting.spec.yaml) | TypeScript / Express | Full spec with every field: budget, Gherkin acceptance criteria, invariants, out_of_scope, dependencies, test_contracts, approval_gates, and routing_hint — adding per-IP rate limiting to an auth endpoint. |
| [02-refactor-auth-middleware.spec.yaml](./02-refactor-auth-middleware.spec.yaml) | Go / Gin | Pure structural refactor; shows how to use invariants to lock down behavior preservation when no observable feature is added. |
| [03-fix-race-condition.spec.yaml](./03-fix-race-condition.spec.yaml) | Python / Flask | Bug fix with a pre-existing reproducer test; shows a spec built around making a flaky test deterministically green. |
| [04-upgrade-dependency.spec.yaml](./04-upgrade-dependency.spec.yaml) | Rust / Axum | Dependency upgrade across multiple files; shows how to use the blast radius feature and approval_gates to control a high-risk change. |
| [05-add-test-coverage.spec.yaml](./05-add-test-coverage.spec.yaml) | JavaScript / Jest | Test-addition task with a coverage threshold test_contract; shows a spec where the agent writes tests, not production code. |

## Using these as templates

```sh
# scaffold a new spec from the built-in template (M1+)
aetronyx spec new my-feature

# validate a spec before running
aetronyx validate examples/01-add-rate-limiting.spec.yaml

# see what files would be affected before running
aetronyx spec blast-radius examples/01-add-rate-limiting.spec.yaml
```

See `docs/reference/spec-format.md` for the full field reference (available after M2).
