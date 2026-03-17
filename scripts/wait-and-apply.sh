#!/usr/bin/env bash
# Wait for GoClaw to be healthy, then apply manifest
set -euo pipefail

BINARY="${BINARY:-./gcplane}"
F="${F:-examples/local-dev.yaml}"
MAX_WAIT=60

echo "Waiting for GoClaw to be ready..."
elapsed=0
until curl -sf http://localhost:18790/health > /dev/null 2>&1; do
  sleep 1
  elapsed=$((elapsed + 1))
  if [ "$elapsed" -ge "$MAX_WAIT" ]; then
    echo "ERROR: GoClaw not ready after ${MAX_WAIT}s"
    exit 1
  fi
done

echo "GoClaw is ready. Applying manifest..."
$BINARY apply -f "$F" --auto-approve
