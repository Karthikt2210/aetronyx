#!/usr/bin/env bats

setup() {
    make -C "${BATS_TEST_DIRNAME}/../.." build
}

@test "version exits 0" {
    run ./dist/aetronyx version
    [ "$status" -eq 0 ]
}

@test "version output format" {
    run ./dist/aetronyx version
    [[ "$output" =~ ^v[0-9] ]]
}

@test "version has three fields" {
    run ./dist/aetronyx version
    # Should be: version commit builtAt
    word_count=$(echo "$output" | wc -w)
    [ "$word_count" -eq 3 ]
}

@test "version with json flag" {
    run ./dist/aetronyx --log-format json version
    echo "$output" | grep -q '"version"'
    [ $? -eq 0 ]
}

@test "version with json flag has all fields" {
    run ./dist/aetronyx --log-format json version
    echo "$output" | grep -q '"commit"'
    [ $? -eq 0 ]
    echo "$output" | grep -q '"built_at"'
    [ $? -eq 0 ]
}
