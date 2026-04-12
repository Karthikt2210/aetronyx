#!/usr/bin/env bats

setup() {
    make -C "${BATS_TEST_DIRNAME}/../.." build
    tmpdir=$(mktemp -d)
    export TMPDIR="$tmpdir"
}

teardown() {
    rm -rf "$TMPDIR"
}

@test "audit verify on non-existent run exits 1" {
    HOME="$TMPDIR" run ./dist/aetronyx audit verify --run nonexistent
    [ "$status" -eq 1 ]
}

@test "audit show on non-existent run exits 1" {
    HOME="$TMPDIR" run ./dist/aetronyx audit show --run nonexistent
    [ "$status" -eq 1 ]
}

@test "audit verify requires --run flag" {
    HOME="$TMPDIR" run ./dist/aetronyx audit verify
    [ "$status" -ne 0 ]
}

@test "audit show requires --run flag" {
    HOME="$TMPDIR" run ./dist/aetronyx audit show
    [ "$status" -ne 0 ]
}
