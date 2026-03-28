#!/usr/bin/env bash
# Run full E2E test suite against a running GoClaw instance
set -euo pipefail

BINARY="${BINARY:-./gcplane}"
F="${F:-examples/local-dev}"
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

# --- SystemConfig CRUD ---
test_system_config() {
  local tmpdir
  tmpdir=$(mktemp -d)
  cat > "$tmpdir/manifest.yaml" <<'YAML'
apiVersion: gcplane.io/v1
kind: Manifest
metadata:
  name: syscfg-test
connection:
  endpoint: http://localhost:18790
  token: ${GOCLAW_TOKEN}
resources:
  - kind: SystemConfig
    name: gcplane.e2e.test
    spec:
      value: "hello"
YAML

  # Apply
  $BINARY apply -f "$tmpdir" --auto-approve

  # Idempotent
  $BINARY plan -f "$tmpdir" | grep -q "0 to create, 0 to update"

  # Update value
  sed -i.bak 's/hello/world/' "$tmpdir/manifest.yaml" && rm -f "$tmpdir/manifest.yaml.bak"
  $BINARY plan -f "$tmpdir" | grep -q "1 to update"
  $BINARY apply -f "$tmpdir" --auto-approve
  $BINARY plan -f "$tmpdir" | grep -q "0 to create, 0 to update"

  # Cleanup
  $BINARY destroy -f "$tmpdir" --auto-approve
  rm -rf "$tmpdir"
}
run_test "system-config-crud" test_system_config

# --- Destroy ---
test_destroy() {
  $BINARY apply -f examples/minimal.yaml --auto-approve
  $BINARY plan -f examples/minimal.yaml | grep -q "0 to create, 0 to update"
  $BINARY destroy -f examples/minimal.yaml --auto-approve
  $BINARY plan -f examples/minimal.yaml | grep -q "to create"
}
run_test "destroy" test_destroy

# ============================================================
# Multi-Tenant Tests (requires GoClaw v2.x with tenant API)
# ============================================================
MT="examples/local-dev-mt"
GOCLAW_EP="${GCPLANE_ENDPOINT:-http://localhost:18790}"
GOCLAW_TOK="${GOCLAW_TOKEN:-}"

tenant_api_available() {
  [ -n "$GOCLAW_TOK" ] && \
  curl -sf "$GOCLAW_EP/v1/tenants" \
    -H "Authorization: Bearer $GOCLAW_TOK" \
    -H "X-GoClaw-User-Id: gcplane" > /dev/null 2>&1
}

if tenant_api_available; then

  # --- Tenant CRUD ---
  test_tenant_crud() {
    $BINARY plan -f "$MT/_system" -v | grep -q "Tenant"
    $BINARY apply -f "$MT/_system" --auto-approve
    $BINARY plan -f "$MT/_system" | grep -q "0 to create, 0 to update"
  }
  run_test "tenant-crud" test_tenant_crud

  # --- Tenant-scoped apply (Acme Corp) ---
  test_tenant_scoped_apply() {
    $BINARY apply -f "$MT/acme-corp" --auto-approve
    $BINARY plan -f "$MT/acme-corp" | grep -q "0 to create, 0 to update"
  }
  run_test "tenant-scoped-apply" test_tenant_scoped_apply

  # --- Tenant isolation (Startup.io can't see Acme's resources) ---
  test_tenant_isolation() {
    $BINARY apply -f "$MT/startup-io" --auto-approve
    # Startup.io plan should show 0 changes (its own resources)
    $BINARY plan -f "$MT/startup-io" | grep -q "0 to create, 0 to update"
  }
  run_test "tenant-isolation" test_tenant_isolation

  # --- Multi-tenant serve ---
  test_tenant_serve() {
    $BINARY serve --tenants-dir "$MT" --interval 30s &
    local pid=$!
    sleep 4

    local ok=true
    curl -sf http://localhost:8480/healthz > /dev/null || ok=false
    curl -sf http://localhost:8480/api/v1/status > /dev/null || ok=false

    kill "$pid" 2>/dev/null; wait "$pid" 2>/dev/null || true
    $ok
  }
  run_test "tenant-serve" test_tenant_serve

  # --- Tenant destroy (cleanup) ---
  test_tenant_destroy() {
    $BINARY destroy -f "$MT/startup-io" --auto-approve
    $BINARY destroy -f "$MT/acme-corp" --auto-approve
    $BINARY destroy -f "$MT/_system" --auto-approve
    # Tenants should need re-creation
    $BINARY plan -f "$MT/_system" | grep -q "to create"
  }
  run_test "tenant-destroy" test_tenant_destroy

else
  echo ""
  echo "SKIP: Multi-tenant tests (tenant API not available — need GoClaw v2.x)"
fi

# --- Summary ---
echo ""
echo "==============================="
echo "E2E Results: $PASS passed, $FAIL failed"
echo "==============================="
[ "$FAIL" -eq 0 ]
