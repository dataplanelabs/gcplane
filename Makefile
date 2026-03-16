BINARY := gcplane
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
LDFLAGS := -ldflags "-X github.com/dataplanelabs/gcplane/cmd.Version=$(VERSION)"

.PHONY: build run test clean validate plan apply

## Build
build:
	go build $(LDFLAGS) -o $(BINARY) .

## Run commands (usage: make validate F=examples/minimal.yaml)
F ?= examples/minimal.yaml

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

## Cleanup
clean:
	rm -f $(BINARY) coverage.out
