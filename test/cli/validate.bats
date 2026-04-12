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
