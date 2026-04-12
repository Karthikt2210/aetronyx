#!/usr/bin/env bats

setup() {
    make -C "${BATS_TEST_DIRNAME}/../.." build
    tmpdir=$(mktemp -d)
    export TMPDIR="$tmpdir"
}

teardown() {
    # Kill any lingering aetronyx processes
    pkill -f "aetronyx serve" || true
    sleep 0.5
    rm -rf "$TMPDIR"
}

@test "serve starts and responds to health endpoint" {
    # Start server in background
    HOME="$TMPDIR" ./dist/aetronyx serve --port 7778 &
    server_pid=$!

    # Give it time to start
    sleep 1

    # Get the auth token
    token=$(cat "$TMPDIR/.aetronyx/auth-token" 2>/dev/null || echo "")
    [ -n "$token" ]

    # Hit health endpoint with token
    run curl -s -H "Authorization: Bearer $token" http://127.0.0.1:7778/api/v1/health
    [ "$status" -eq 0 ]
    echo "$output" | grep -q '"status":"ok"'

    # Clean up
    kill $server_pid 2>/dev/null || true
    wait $server_pid 2>/dev/null || true
}

@test "serve rejects request without token" {
    # Start server in background
    HOME="$TMPDIR" ./dist/aetronyx serve --port 7779 &
    server_pid=$!

    sleep 1

    # Try without token
    run curl -s -w "\n%{http_code}" http://127.0.0.1:7779/api/v1/health
    # Should get 401 (unauthorized)
    [[ "$output" =~ 401$ ]]

    # Clean up
    kill $server_pid 2>/dev/null || true
    wait $server_pid 2>/dev/null || true
}

@test "serve rejects request with wrong token" {
    # Start server in background
    HOME="$TMPDIR" ./dist/aetronyx serve --port 7780 &
    server_pid=$!

    sleep 1

    # Try with wrong token
    run curl -s -w "\n%{http_code}" -H "Authorization: Bearer wrong-token" http://127.0.0.1:7780/api/v1/health
    [[ "$output" =~ 401$ ]]

    # Clean up
    kill $server_pid 2>/dev/null || true
    wait $server_pid 2>/dev/null || true
}

@test "serve version endpoint works" {
    # Start server in background
    HOME="$TMPDIR" ./dist/aetronyx serve --port 7781 &
    server_pid=$!

    sleep 1

    # Get the auth token
    token=$(cat "$TMPDIR/.aetronyx/auth-token" 2>/dev/null)

    # Hit version endpoint
    run curl -s -H "Authorization: Bearer $token" http://127.0.0.1:7781/api/v1/version
    [ "$status" -eq 0 ]
    echo "$output" | grep -q '"version"'

    # Clean up
    kill $server_pid 2>/dev/null || true
    wait $server_pid 2>/dev/null || true
}
