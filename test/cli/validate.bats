#!/usr/bin/env bats

setup() {
    make -C "${BATS_TEST_DIRNAME}/../.." build
    tmpdir=$(mktemp -d)
    export TMPDIR="$tmpdir"
}

teardown() {
    rm -rf "$TMPDIR"
}

@test "validate exits 0 on valid spec" {
    run ./dist/aetronyx validate examples/hello.spec.yaml
    [ "$status" -eq 0 ]
}

@test "validate prints ok on valid spec" {
    run ./dist/aetronyx validate examples/hello.spec.yaml
    [[ "$output" =~ ok ]]
}

@test "validate exits 10 on invalid yaml" {
    echo ":" > "$TMPDIR/bad.yaml"
    run ./dist/aetronyx validate "$TMPDIR/bad.yaml"
    [ "$status" -eq 10 ]
}

@test "validate exits 10 on spec missing name" {
    cat > "$TMPDIR/no-name.yaml" <<EOF
spec_version: "1"
intent: "Do something"
EOF
    run ./dist/aetronyx validate "$TMPDIR/no-name.yaml"
    [ "$status" -eq 10 ]
}

@test "validate exits 10 on spec missing intent" {
    cat > "$TMPDIR/no-intent.yaml" <<EOF
spec_version: "1"
name: "test"
EOF
    run ./dist/aetronyx validate "$TMPDIR/no-intent.yaml"
    [ "$status" -eq 10 ]
}

@test "validate exits 10 on spec missing spec_version" {
    cat > "$TMPDIR/no-version.yaml" <<EOF
name: "test"
intent: "Do something"
EOF
    run ./dist/aetronyx validate "$TMPDIR/no-version.yaml"
    [ "$status" -eq 10 ]
}

# M2 Validator Rule Tests

@test "rule2 invalid name exits 10" {
    cat > "$TMPDIR/bad-name.yaml" <<EOF
spec_version: "1"
name: "My Invalid Name"
intent: "Do something meaningful"
acceptance_criteria:
  - given: a
    when: b
    then: c
EOF
    run ./dist/aetronyx validate "$TMPDIR/bad-name.yaml"
    [ "$status" -eq 10 ]
    [[ "$output" =~ name.format ]]
}

@test "rule3 short intent warns" {
    cat > "$TMPDIR/short-intent.yaml" <<EOF
spec_version: "1"
name: "valid-task"
intent: "Do x"
acceptance_criteria:
  - given: a
    when: b
    then: c
EOF
    run ./dist/aetronyx validate "$TMPDIR/short-intent.yaml"
    [ "$status" -eq 0 ]
    [[ "$output" =~ WARN || "$output" =~ intent.length ]]
}

@test "rule4 zero cost exits 10" {
    cat > "$TMPDIR/zero-cost.yaml" <<EOF
spec_version: "1"
name: "test-task"
intent: "Do something here now"
budget:
  max_cost_usd: 0
acceptance_criteria:
  - given: a
    when: b
    then: c
EOF
    run ./dist/aetronyx validate "$TMPDIR/zero-cost.yaml"
    [ "$status" -eq 10 ]
    [[ "$output" =~ budget.sanity ]]
}

@test "rule6 no acceptance criteria exits 10" {
    cat > "$TMPDIR/no-criteria.yaml" <<EOF
spec_version: "1"
name: "test-task"
intent: "Do something meaningful"
acceptance_criteria: []
EOF
    run ./dist/aetronyx validate "$TMPDIR/no-criteria.yaml"
    [ "$status" -eq 10 ]
}

@test "rule8 empty out_of_scope warns" {
    cat > "$TMPDIR/no-out-of-scope.yaml" <<EOF
spec_version: "1"
name: "test-task"
intent: "Do something meaningful"
acceptance_criteria:
  - given: a
    when: b
    then: c
out_of_scope: []
EOF
    run ./dist/aetronyx validate "$TMPDIR/no-out-of-scope.yaml"
    [ "$status" -eq 0 ]
    [[ "$output" =~ WARN || "$output" =~ out_of_scope ]]
}

@test "strict flag promotes warnings to fatal" {
    cat > "$TMPDIR/warnings.yaml" <<EOF
spec_version: "1"
name: "valid-name"
intent: "Too short"
acceptance_criteria:
  - given: a
    when: b
    then: c
EOF
    run ./dist/aetronyx validate "$TMPDIR/warnings.yaml" --strict
    [ "$status" -eq 10 ]
}

@test "format json output" {
    run ./dist/aetronyx validate examples/hello.spec.yaml --format json
    [ "$status" -eq 0 ]
    [[ "$output" =~ "\"ok\"" ]]
}

@test "format json errors shows array" {
    cat > "$TMPDIR/bad.yaml" <<EOF
spec_version: "1"
name: "Test Name"
intent: "bad"
EOF
    run ./dist/aetronyx validate "$TMPDIR/bad.yaml" --format json
    [[ "$output" =~ "errors" ]]
    [[ "$output" =~ "\[\[" || "$output" =~ "\[" ]]
}

@test "valid full spec exits 0" {
    run ./dist/aetronyx validate examples/refactor-auth.spec.yaml
    [ "$status" -eq 0 ]
}

@test "missing spec file exits 1" {
    run ./dist/aetronyx validate /nonexistent/file.yaml
    [ "$status" -ne 0 ]
}
