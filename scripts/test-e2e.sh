#!/usr/bin/env bash
# Run full E2E test suite against a running GoClaw instance
set -euo pipefail

BINARY="${BINARY:-./gcplane}"
F="${F:-examples/local-dev.yaml}"
PASS=0
FAIL=0

run_test() {
  local name="$1"
  shift
  echo ""
  echo "=== Test: $name ==="
  if "$@"; then
    echo "PASS: $name"
    PASS=$((PASS + 1))
  else
    echo "FAIL: $name"
    FAIL=$((FAIL + 1))
  fi
}

# --- Plan ---
run_test "plan" $BINARY plan -f "$F" -v

# --- Apply idempotency ---
test_apply() {
  $BINARY apply -f "$F" --auto-approve
  echo "--- Second apply (should be 0 changes) ---"
  $BINARY plan -f "$F" | grep -q "0 to create, 0 to update"
}
run_test "apply-idempotency" test_apply

# --- Diff (no drift after apply) ---
test_diff() {
  $BINARY diff -f "$F" 2>&1 | grep -q "No drift"
}
run_test "diff-no-drift" test_diff

# --- Composite ---
test_composite() {
  $BINARY validate -f examples/composite-example.yaml
  $BINARY plan -f examples/composite-example.yaml -v
}
run_test "composite" test_composite

# --- Serve ---
test_serve() {
  $BINARY serve -f "$F" --interval 30s &
  local pid=$!
  sleep 3

  local ok=true
  curl -sf http://localhost:8480/healthz > /dev/null || ok=false
  curl -sf http://localhost:8480/readyz > /dev/null || ok=false
  curl -sf http://localhost:8480/api/v1/status > /dev/null || ok=false
  curl -sf http://localhost:8480/metrics > /dev/null || ok=false
  curl -sf -X POST http://localhost:8480/api/v1/sync > /dev/null || ok=false

  kill "$pid" 2>/dev/null; wait "$pid" 2>/dev/null || true
  $ok
}
run_test "serve" test_serve

# --- Destroy ---
test_destroy() {
  $BINARY apply -f examples/minimal.yaml --auto-approve
  $BINARY plan -f examples/minimal.yaml | grep -q "0 to create, 0 to update"
  $BINARY destroy -f examples/minimal.yaml --auto-approve
  $BINARY plan -f examples/minimal.yaml | grep -q "to create"
}
run_test "destroy" test_destroy

# --- Summary ---
echo ""
echo "==============================="
echo "E2E Results: $PASS passed, $FAIL failed"
echo "==============================="
[ "$FAIL" -eq 0 ]
