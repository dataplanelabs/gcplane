# GCPlane Code Standards

## Language & Tooling
- **Go 1.25+** (pure Go, no CGO dependencies)
- **Cobra** for CLI commands
- **YAML v3** for manifest parsing
- **gorilla/websocket** for WS RPC
- **modernc.org/sqlite** for pure-Go SQLite

## File Organization
- `cmd/` — CLI commands, one file per command
- `internal/` — private packages, organized by domain
- Each resource type gets its own file in `provider/goclaw/`
- Keep files under 200 lines where practical

## Naming Conventions
- **Files**: snake_case (Go convention)
- **Packages**: lowercase, single word
- **Types/Functions**: PascalCase (exported), camelCase (unexported)
- **Resource keys**: kebab-case in manifests

## Error Handling
- Sentinel errors for HTTP status classification (`ErrNotFound`, `ErrUnauthorized`)
- Wrap errors with context: `fmt.Errorf("action %s: %w", key, err)`
- Provider methods return `nil, nil` for "resource not found" (not an error)

## Provider Pattern
Each resource implements 3 methods on `*Provider`:
```go
func (p *Provider) observeX(key string) (map[string]any, error)  // nil = not found
func (p *Provider) createX(key string, spec map[string]any) error
func (p *Provider) updateX(key string, spec map[string]any) error
```

Routing in `provider.go` dispatches by `ResourceKind`.

## Testing
- Unit tests in `*_test.go` alongside source files
- Use `testing.T` and table-driven tests
- Mock the `ProviderInterface` for reconciler tests
- Use `t.TempDir()` for file-based tests (auto-cleanup)
- No external services required for unit tests

## Secret Handling
- Never log or display resolved secrets
- API keys masked as `"***"` in GoClaw responses — skip in comparison
- Support `${ENV_VAR}` and `file://path` patterns
