package reconciler

import (
	"fmt"

	"github.com/dataplanelabs/gcplane/internal/manifest"
	"github.com/dataplanelabs/gcplane/internal/secrets"
)

// ProviderInterface defines the operations a provider must support.
type ProviderInterface interface {
	Observe(kind manifest.ResourceKind, key string) (map[string]any, error)
	Create(kind manifest.ResourceKind, key string, spec map[string]any) error
	Update(kind manifest.ResourceKind, key string, spec map[string]any) error
}

// Engine is the Observe→Compare→Act reconciliation engine.
type Engine struct {
	provider ProviderInterface
}

// NewEngine creates a reconciler engine with the given provider.
func NewEngine(provider ProviderInterface) *Engine {
	return &Engine{provider: provider}
}

// Reconcile processes a manifest and returns a plan.
// If dryRun=false, it also executes the changes via the provider.
func (e *Engine) Reconcile(m *manifest.Manifest, dryRun bool) (*Plan, *ApplyResult) {
	plan := &Plan{}
	result := &ApplyResult{}

	// Group resources by kind for dependency ordering
	byKind := make(map[manifest.ResourceKind][]manifest.Resource)
	for _, r := range m.Resources {
		byKind[r.Kind] = append(byKind[r.Kind], r)
	}

	// Process in dependency order
	for _, kind := range manifest.ApplyOrder() {
		resources, ok := byKind[kind]
		if !ok {
			continue
		}

		for _, res := range resources {
			change := e.reconcileOne(res)
			plan.Changes = append(plan.Changes, change)

			switch change.Action {
			case ActionCreate:
				plan.Creates++
			case ActionUpdate:
				plan.Updates++
			case ActionNoop:
				plan.Noops++
			}

			// If error during observe, record and skip execution
			if change.Error != "" {
				plan.Errors = append(plan.Errors, fmt.Sprintf("%s/%s: %s", res.Kind, res.Key, change.Error))
				continue
			}

			// Execute if not dry-run
			if !dryRun && change.Action != ActionNoop {
				err := e.execute(change, res)
				if err != nil {
					result.Failed++
					result.Errors = append(result.Errors, fmt.Sprintf("%s/%s: %v", res.Kind, res.Key, err))
				} else {
					result.Applied++
				}
			}
		}
	}

	return plan, result
}

func (e *Engine) reconcileOne(res manifest.Resource) Change {
	change := Change{
		Kind: res.Kind,
		Key:  res.Key,
	}

	// Resolve secrets in spec
	spec := resolveSpecSecrets(res.Spec)

	// Observe current state
	current, err := e.provider.Observe(res.Kind, res.Key)
	if err != nil {
		change.Action = ActionNoop
		change.Error = fmt.Sprintf("observe failed: %v", err)
		return change
	}

	// Resource doesn't exist — create
	if current == nil {
		change.Action = ActionCreate
		return change
	}

	// Compare desired vs current
	diffs := CompareSpec(spec, current)
	if len(diffs) == 0 {
		change.Action = ActionNoop
		return change
	}

	change.Action = ActionUpdate
	change.Diff = diffs
	return change
}

func (e *Engine) execute(change Change, res manifest.Resource) error {
	spec := resolveSpecSecrets(res.Spec)

	switch change.Action {
	case ActionCreate:
		return e.provider.Create(res.Kind, res.Key, spec)
	case ActionUpdate:
		return e.provider.Update(res.Kind, res.Key, spec)
	default:
		return nil
	}
}

// resolveSpecSecrets walks a spec map and resolves secret references in string values.
func resolveSpecSecrets(spec map[string]any) map[string]any {
	out := make(map[string]any, len(spec))
	for k, v := range spec {
		out[k] = resolveValue(v)
	}
	return out
}

func resolveValue(v any) any {
	switch val := v.(type) {
	case string:
		resolved, err := secrets.Resolve(val)
		if err != nil {
			return val // Return original on error
		}
		return resolved
	case map[string]any:
		return resolveSpecSecrets(val)
	case []any:
		resolved := make([]any, len(val))
		for i, item := range val {
			resolved[i] = resolveValue(item)
		}
		return resolved
	default:
		return v
	}
}
