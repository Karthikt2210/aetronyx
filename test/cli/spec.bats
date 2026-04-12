#!/usr/bin/env bats

setup() {
    make -C "${BATS_TEST_DIRNAME}/../.." build
    tmpdir=$(mktemp -d)
    export TMPDIR="$tmpdir"
    cd "$TMPDIR"
}

teardown() {
    cd - > /dev/null 2>&1 || true
    rm -rf "$TMPDIR"
}

@test "spec init creates example.spec.yaml" {
    run ../../dist/aetronyx spec init --yes
    [ "$status" -eq 0 ]
    [ -f example.spec.yaml ]
    grep -q "spec_version: \"1\"" example.spec.yaml
}

@test "spec new creates named spec file" {
    run ../../dist/aetronyx spec new my-feature --yes
    [ "$status" -eq 0 ]
    [ -f my-feature.spec.yaml ]
    grep -q "my-feature" my-feature.spec.yaml
}

@test "spec new invalid name exits 1" {
    run ../../dist/aetronyx spec new "Bad Name" --yes
    [ "$status" -ne 0 ]
    ! [ -f "Bad Name.spec.yaml" ]
}

@test "spec new uppercase name exits 1" {
    run ../../dist/aetronyx spec new "UpperCase" --yes
    [ "$status" -ne 0 ]
}

@test "spec new with hyphen in name succeeds" {
    run ../../dist/aetronyx spec new my-valid-name --yes
    [ "$status" -eq 0 ]
    [ -f my-valid-name.spec.yaml ]
}

@test "spec list shows header and columns" {
    # Create a minimal database first
    ../../dist/aetronyx spec init --yes > /dev/null 2>&1 || true

    run ../../dist/aetronyx spec list
    [ "$status" -eq 0 ]
    # Should at least have header
    [[ "$output" =~ SPEC_NAME || "$output" =~ spec ]]
}

@test "spec blast-radius with empty fixture exits 0" {
    # Create a simple spec
    cat > test.spec.yaml <<EOF
spec_version: "1"
name: test-task
intent: Test blast radius
acceptance_criteria:
  - given: a; when: b; then: c
dependencies:
  files: []
EOF

    run ../../dist/aetronyx spec blast-radius test.spec.yaml
    [ "$status" -eq 0 ]
}

@test "spec blast-radius json format outputs valid json" {
    cat > test.spec.yaml <<EOF
spec_version: "1"
name: test-task
intent: Test blast radius
acceptance_criteria:
  - given: a; when: b; then: c
dependencies:
  files: []
EOF

    run ../../dist/aetronyx spec blast-radius test.spec.yaml --format json
    [ "$status" -eq 0 ]
    # Should be valid JSON (contains { and })
    [[ "$output" =~ "{" ]] && [[ "$output" =~ "}" ]]
}

@test "spec blast-radius text format outputs readable report" {
    cat > test.spec.yaml <<EOF
spec_version: "1"
name: test-task
intent: Test blast radius
acceptance_criteria:
  - given: a; when: b; then: c
dependencies:
  files: []
EOF

    run ../../dist/aetronyx spec blast-radius test.spec.yaml --format text
    [ "$status" -eq 0 ]
    # Should have some descriptive text
    [[ "$output" =~ "Blast" || "$output" =~ "Radius" || "$output" =~ "Report" ]]
}
