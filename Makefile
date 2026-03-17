# Load .env if it exists
ifneq (,$(wildcard .env))
include .env
export
endif

BINARY := gcplane
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
LDFLAGS := -ldflags "-X github.com/dataplanelabs/gcplane/cmd.Version=$(VERSION)"

.PHONY: build test test-v test-cover clean
.PHONY: validate plan apply serve
.PHONY: goclaw-up goclaw-down goclaw-reset goclaw-logs
.PHONY: setup reset
.PHONY: test-e2e test-serve test-plan test-apply test-diff test-destroy test-composite

## Run commands (usage: make validate F=examples/minimal.yaml)
F ?= examples/local-dev.yaml

# Path to GoClaw repo (for docker compose)
GOCLAW_DIR ?= ../../nextlevelbuilder/goclaw
GOCLAW_COMPOSE = cd $(GOCLAW_DIR) && docker compose -f docker-compose.yml -f docker-compose.postgres.yml -f docker-compose.selfservice.yml

# ============================================================
# Build
# ============================================================
build:
	go build $(LDFLAGS) -o $(BINARY) .

clean:
	rm -f $(BINARY) coverage.out

# ============================================================
# Unit Tests
# ============================================================
test:
	go test ./... -count=1

test-v:
	go test ./... -v -count=1

test-cover:
	go test ./... -coverprofile=coverage.out
	go tool cover -func=coverage.out

# ============================================================
# CLI Commands
# ============================================================
validate: build
	./$(BINARY) validate -f $(F)

plan: build
	./$(BINARY) plan -f $(F)

apply: build
	./$(BINARY) apply -f $(F)

serve: build
	./$(BINARY) serve -f $(F) --interval 10s

# ============================================================
# GoClaw Instance
# ============================================================
goclaw-up:
	$(GOCLAW_COMPOSE) up -d --build

goclaw-down:
	$(GOCLAW_COMPOSE) down

goclaw-reset:
	$(GOCLAW_COMPOSE) down -v
	$(GOCLAW_COMPOSE) up -d --build

goclaw-logs:
	$(GOCLAW_COMPOSE) logs -f goclaw

# ============================================================
# Setup (one-click: fresh GoClaw + apply config)
# ============================================================
setup: build goclaw-up
	@echo "Waiting for GoClaw to be ready..."
	@until curl -sf http://localhost:18790/health > /dev/null 2>&1; do sleep 1; done
	@echo "GoClaw is ready. Applying manifest..."
	./$(BINARY) apply -f $(F) --auto-approve

## Full reset: wipe GoClaw volumes + re-setup from scratch
reset: build goclaw-reset
	@echo "Waiting for GoClaw to be ready..."
	@until curl -sf http://localhost:18790/health > /dev/null 2>&1; do sleep 1; done
	@echo "GoClaw is ready. Applying manifest..."
	./$(BINARY) apply -f $(F) --auto-approve

# ============================================================
# E2E Tests (requires running GoClaw)
# ============================================================

## Full e2e: reset GoClaw + test all features
test-e2e: reset test-plan test-apply test-diff test-composite test-serve test-destroy
	@echo ""
	@echo "=== All E2E tests passed ==="

## Test: plan shows correct state
test-plan: build
	@echo ""
	@echo "=== Test: plan ==="
	./$(BINARY) plan -f $(F) -v
	@echo "PASS: plan"

## Test: apply is idempotent (second apply = 0 changes)
test-apply: build
	@echo ""
	@echo "=== Test: apply idempotency ==="
	./$(BINARY) apply -f $(F) --auto-approve
	@echo "--- Second apply (should be 0 changes) ---"
	./$(BINARY) plan -f $(F) | grep -q "0 to create, 0 to update" && echo "PASS: idempotent" || (echo "FAIL: not idempotent" && exit 1)

## Test: diff shows no drift after apply (verifies id-stripping fix)
test-diff: build
	@echo ""
	@echo "=== Test: diff (no drift after apply) ==="
	./$(BINARY) diff -f $(F) 2>&1 | grep -q "No drift" && echo "PASS: no drift" || (echo "FAIL: unexpected drift detected" && exit 1)

## Test: composite expansion validates and plans correctly
test-composite: build
	@echo ""
	@echo "=== Test: composite ==="
	./$(BINARY) validate -f examples/composite-example.yaml
	./$(BINARY) plan -f examples/composite-example.yaml -v
	@echo "PASS: composite"

## Test: destroy actually removes resources (uses minimal.yaml to avoid side effects)
test-destroy: build
	@echo ""
	@echo "=== Test: destroy ==="
	./$(BINARY) apply -f examples/minimal.yaml --auto-approve
	./$(BINARY) plan -f examples/minimal.yaml | grep -q "0 to create, 0 to update" || (echo "FAIL: minimal apply failed" && exit 1)
	./$(BINARY) destroy -f examples/minimal.yaml --auto-approve
	./$(BINARY) plan -f examples/minimal.yaml | grep -q "to create" && echo "PASS: resources destroyed and re-creatable" || (echo "FAIL: destroy did not remove resources" && exit 1)

## Test: serve starts, syncs, responds to health checks
test-serve: build
	@echo ""
	@echo "=== Test: serve ==="
	@./$(BINARY) serve -f $(F) --interval 30s &
	@SERVE_PID=$$!; \
	sleep 3; \
	echo "--- healthz ---"; \
	curl -sf http://localhost:8480/healthz || (kill $$SERVE_PID 2>/dev/null; echo "FAIL: healthz"; exit 1); \
	echo ""; \
	echo "--- readyz ---"; \
	curl -sf http://localhost:8480/readyz || (kill $$SERVE_PID 2>/dev/null; echo "FAIL: readyz"; exit 1); \
	echo ""; \
	echo "--- status ---"; \
	curl -sf http://localhost:8480/api/v1/status | python3 -m json.tool; \
	echo "--- metrics ---"; \
	curl -sf http://localhost:8480/metrics | head -6; \
	echo "--- sync trigger ---"; \
	curl -sf -X POST http://localhost:8480/api/v1/sync; \
	echo ""; \
	kill $$SERVE_PID 2>/dev/null; \
	wait $$SERVE_PID 2>/dev/null; \
	echo "PASS: serve"
