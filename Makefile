# Load .env if it exists
ifneq (,$(wildcard .env))
include .env
export
endif

BINARY := gcplane
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
LDFLAGS := -ldflags "-X github.com/dataplanelabs/gcplane/cmd.Version=$(VERSION)"

.PHONY: build run test clean validate plan apply setup goclaw-up goclaw-down goclaw-logs

## Build
build:
	go build $(LDFLAGS) -o $(BINARY) .

## Run commands (usage: make validate F=examples/minimal.yaml)
F ?= examples/local-dev.yaml

# Path to GoClaw repo (for docker compose)
GOCLAW_DIR ?= ../../nextlevelbuilder/goclaw

validate: build
	./$(BINARY) validate -f $(F)

plan: build
	./$(BINARY) plan -f $(F)

apply: build
	./$(BINARY) apply -f $(F)

## Development
test:
	go test ./... -count=1

test-v:
	go test ./... -v -count=1

test-cover:
	go test ./... -coverprofile=coverage.out
	go tool cover -func=coverage.out

## GoClaw local instance
goclaw-up:
	cd $(GOCLAW_DIR) && docker compose -f docker-compose.yml -f docker-compose.postgres.yml -f docker-compose.selfservice.yml up -d --build

goclaw-down:
	cd $(GOCLAW_DIR) && docker compose -f docker-compose.yml -f docker-compose.postgres.yml -f docker-compose.selfservice.yml down

goclaw-logs:
	cd $(GOCLAW_DIR) && docker compose -f docker-compose.yml -f docker-compose.postgres.yml -f docker-compose.selfservice.yml logs -f goclaw

## One-click setup: start GoClaw + apply config (skips setup wizard)
setup: build goclaw-up
	@echo "Waiting for GoClaw to be ready..."
	@until curl -sf http://localhost:18790/health > /dev/null 2>&1; do sleep 1; done
	@echo "GoClaw is ready. Applying manifest..."
	./$(BINARY) apply -f $(F) --auto-approve

## Cleanup
clean:
	rm -f $(BINARY) coverage.out
