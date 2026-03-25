package reconciler

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"sync"

	"golang.org/x/sync/errgroup"

	"github.com/dataplanelabs/gcplane/internal/manifest"
	"github.com/dataplanelabs/gcplane/internal/secrets"
)

// ProviderInterface defines the operations a provider must support.
type ProviderInterface interface {
	Observe(ctx context.Context, kind manifest.ResourceKind, key string) (map[string]any, error)
	Create(ctx context.Context, kind manifest.ResourceKind, key string, spec map[string]any) error
	Update(ctx context.Context, kind manifest.ResourceKind, key string, spec map[string]any) error
	Delete(ctx context.Context, kind manifest.ResourceKind, key string) error
	ListAll(ctx context.Context, kind manifest.ResourceKind) ([]ResourceInfo, error)
}

// Engine is the Observe→Compare→Act reconciliation engine.
type Engine struct {
	provider ProviderInterface
	logger   *slog.Logger
}

// NewEngine creates a reconciler engine with the given provider and optional logger.
// If logger is nil, log output is discarded.
func NewEngine(provider ProviderInterface, logger *slog.Logger) *Engine {
	if logger == nil {
		logger = slog.New(slog.NewTextHandler(io.Discard, nil))
	}
	return &Engine{provider: provider, logger: logger}
}

// Reconcile processes a manifest and returns a plan.
// If opts.DryRun=false, it also executes the changes via the provider.
// If opts.Prune=true, orphaned gcplane-owned resources are deleted.
// opts.Concurrency controls max parallel resources per kind (0 = sequential).
func (e *Engine) Reconcile(ctx context.Context, m *manifest.Manifest, opts ReconcileOpts) (*Plan, *ApplyResult) {
	plan := &Plan{}
	result := &ApplyResult{}

	// Group resources by kind for dependency ordering
	byKind := make(map[manifest.ResourceKind][]manifest.Resource)
	for _, r := range m.Resources {
		byKind[r.Kind] = append(byKind[r.Kind], r)
	}

	// Process in dependency order (cross-kind order is always sequential)
	for _, kind := range manifest.ApplyOrder() {
		resources, ok := byKind[kind]
		if !ok {
			continue
		}

		// Observe phase (optionally parallel within this kind)
		changes := e.reconcileKind(ctx, resources, opts.Force, opts.Concurrency)
		for i, change := range changes {
			plan.Changes = append(plan.Changes, change)
			switch change.Action {
			case ActionCreate:
				plan.Creates++
			case ActionUpdate:
				plan.Updates++
			case ActionNoop:
				plan.Noops++
			}
			if change.Error != "" {
				plan.Errors = append(plan.Errors, fmt.Sprintf("%s/%s: %s", resources[i].Kind, resources[i].Name, change.Error))
			}
		}

		// Execute phase (optionally parallel within this kind)
		if !opts.DryRun {
			kindResult := e.executeChanges(ctx, changes, resources, opts.Concurrency)
			result.Applied += kindResult.Applied
			result.Failed += kindResult.Failed
			result.Errors = append(result.Errors, kindResult.Errors...)
		}
	}

	// Prune phase: detect orphaned gcplane-owned resources
	if opts.Prune {
		pruneChanges, pruneResult := e.detectAndExecutePrunes(ctx, m, opts.DryRun)
		plan.Changes = append(plan.Changes, pruneChanges...)
		for _, c := range pruneChanges {
			if c.Action == ActionDelete {
				plan.Deletes++
			}
		}
		result.Applied += pruneResult.Applied
		result.Failed += pruneResult.Failed
		result.Errors = append(result.Errors, pruneResult.Errors...)
	}

	return plan, result
}

// reconcileKind runs the observe phase for all resources of a single kind.
// When concurrency <= 1, resources are processed sequentially (default).
func (e *Engine) reconcileKind(ctx context.Context, resources []manifest.Resource, force bool, concurrency int) []Change {
	changes := make([]Change, len(resources))

	if concurrency <= 1 {
		for i, res := range resources {
			changes[i] = e.reconcileOne(ctx, res, force)
		}
		return changes
	}

	g, _ := errgroup.WithContext(ctx)
	g.SetLimit(concurrency)

	for i, res := range resources {
		i, res := i, res
		g.Go(func() error {
			changes[i] = e.reconcileOne(ctx, res, force)
			return nil // errors captured in Change.Error
		})
	}
	_ = g.Wait()
	return changes
}

// executeChanges runs the execute phase for a slice of changes.
// When concurrency <= 1, changes are executed sequentially (default).
func (e *Engine) executeChanges(ctx context.Context, changes []Change, resources []manifest.Resource, concurrency int) *ApplyResult {
	result := &ApplyResult{}

	if concurrency <= 1 {
		for i, change := range changes {
			if change.Action == ActionNoop || change.Error != "" {
				continue
			}
			if err := e.execute(ctx, change, resources[i]); err != nil {
				result.Failed++
				result.Errors = append(result.Errors, fmt.Sprintf("%s/%s: %v", change.Kind, change.Name, err))
			} else {
				result.Applied++
			}
		}
		return result
	}

	var mu sync.Mutex
	g, _ := errgroup.WithContext(ctx)
	g.SetLimit(concurrency)

	for i, change := range changes {
		if change.Action == ActionNoop || change.Error != "" {
			continue
		}
		i, change := i, change
		g.Go(func() error {
			err := e.execute(ctx, change, resources[i])
			mu.Lock()
			defer mu.Unlock()
			if err != nil {
				result.Failed++
				result.Errors = append(result.Errors, fmt.Sprintf("%s/%s: %v", change.Kind, change.Name, err))
			} else {
				result.Applied++
			}
			return nil
		})
	}
	_ = g.Wait()
	return result
}

func (e *Engine) detectAndExecutePrunes(ctx context.Context, m *manifest.Manifest, dryRun bool) ([]Change, *ApplyResult) {
	result := &ApplyResult{}
	var changes []Change

	// Build manifest resource set per kind
	manifestSet := make(map[manifest.ResourceKind]map[string]bool)
	for _, r := range m.Resources {
		if manifestSet[r.Kind] == nil {
			manifestSet[r.Kind] = make(map[string]bool)
		}
		manifestSet[r.Kind][r.Name] = true
	}

	// Check each kind in reverse dependency order
	for _, kind := range manifest.DeleteOrder() {
		// Skip non-deletable and non-enumerable kinds
		if kind == manifest.KindSkill || kind == manifest.KindTTSConfig ||
			kind == manifest.KindBuiltinToolConfig || kind == manifest.KindSkillConfig || kind == manifest.KindMCPCredentials {
			continue
		}

		remotes, err := e.provider.ListAll(ctx, kind)
		if err != nil {
			// Can't list this kind — skip silently
			continue
		}

		for _, remote := range remotes {
			// Only prune gcplane-owned resources
			if remote.CreatedBy != "gcplane" {
				continue
			}
			// Skip if resource is in manifest
			if manifestSet[kind][remote.Name] {
				continue
			}

			change := Change{Kind: kind, Name: remote.Name, Action: ActionDelete}
			changes = append(changes, change)

			e.logger.Info("pruning resource",
				slog.String("kind", string(kind)),
				slog.String("name", remote.Name))

			if !dryRun {
				if err := e.provider.Delete(ctx, kind, remote.Name); err != nil {
					e.logger.Error("prune failed",
						slog.String("kind", string(kind)),
						slog.String("name", remote.Name),
						slog.Any("error", err))
					result.Failed++
					result.Errors = append(result.Errors, fmt.Sprintf("%s/%s: %v", kind, remote.Name, err))
				} else {
					result.Applied++
				}
			}
		}
	}
	return changes, result
}

// reconcileContext holds state passed through the subreconciler step pipeline.
type reconcileContext struct {
	resource manifest.Resource
	spec     map[string]any
	current  map[string]any
	change   Change
	force    bool
}

// reconcileOne runs the subreconciler pipeline: resolve → observe → compare.
func (e *Engine) reconcileOne(ctx context.Context, res manifest.Resource, force bool) Change {
	rc := &reconcileContext{
		resource: res,
		change:   Change{Kind: res.Kind, Name: res.Name},
		force:    force,
	}

	steps := []func(context.Context, *reconcileContext) error{
		e.stepResolveSecrets,
		e.stepObserve,
		e.stepCompare,
	}

	for _, step := range steps {
		if err := step(ctx, rc); err != nil {
			break
		}
	}
	return rc.change
}

// stepResolveSecrets resolves secret references (${ENV_VAR}, file://) in the resource spec.
func (e *Engine) stepResolveSecrets(_ context.Context, rc *reconcileContext) error {
	rc.spec = e.resolveSpecSecrets(rc.resource.Spec)
	return nil
}

// stepObserve queries the provider for the current state of the resource.
func (e *Engine) stepObserve(ctx context.Context, rc *reconcileContext) error {
	e.logger.Debug("observing resource",
		slog.String("kind", string(rc.resource.Kind)),
		slog.String("name", rc.resource.Name))

	current, err := e.provider.Observe(ctx, rc.resource.Kind, rc.resource.Name)
	if err != nil {
		e.logger.Warn("observe failed",
			slog.String("kind", string(rc.resource.Kind)),
			slog.String("name", rc.resource.Name),
			slog.Any("error", err))
		rc.change.Action = ActionNoop
		rc.change.Error = fmt.Sprintf("observe failed: %v", err)
		return err
	}

	e.logger.Debug("observe result",
		slog.String("kind", string(rc.resource.Kind)),
		slog.String("name", rc.resource.Name),
		slog.Bool("exists", current != nil))

	rc.current = current
	return nil
}

// stepCompare compares desired spec against observed state to determine the action.
func (e *Engine) stepCompare(_ context.Context, rc *reconcileContext) error {
	// Resource doesn't exist — create
	if rc.current == nil {
		rc.change.Action = ActionCreate
		return nil
	}

	// Compare desired vs current, skipping write-only fields
	exclude := manifest.WriteOnlyFields(rc.resource.Kind)
	diffs := CompareSpecExcluding(rc.spec, rc.current, exclude)
	if len(diffs) == 0 {
		if rc.force {
			rc.change.Action = ActionUpdate
			rc.change.Forced = true
		} else {
			rc.change.Action = ActionNoop
		}
		return nil
	}

	rc.change.Action = ActionUpdate
	rc.change.Diff = diffs
	return nil
}

func (e *Engine) execute(ctx context.Context, change Change, res manifest.Resource) error {
	spec := e.resolveSpecSecrets(res.Spec)

	switch change.Action {
	case ActionCreate:
		e.logger.Info("creating resource",
			slog.String("kind", string(res.Kind)),
			slog.String("name", res.Name))
		if err := e.provider.Create(ctx, res.Kind, res.Name, spec); err != nil {
			e.logger.Error("create failed",
				slog.String("kind", string(res.Kind)),
				slog.String("name", res.Name),
				slog.Any("error", err))
			return err
		}
		return nil
	case ActionUpdate:
		e.logger.Info("updating resource",
			slog.String("kind", string(res.Kind)),
			slog.String("name", res.Name))
		if err := e.provider.Update(ctx, res.Kind, res.Name, spec); err != nil {
			e.logger.Error("update failed",
				slog.String("kind", string(res.Kind)),
				slog.String("name", res.Name),
				slog.Any("error", err))
			return err
		}
		return nil
	default:
		return nil
	}
}

// resolveSpecSecrets walks a spec map and resolves secret references in string values.
func (e *Engine) resolveSpecSecrets(spec map[string]any) map[string]any {
	out := make(map[string]any, len(spec))
	for k, v := range spec {
		out[k] = e.resolveValue(v)
	}
	return out
}

func (e *Engine) resolveValue(v any) any {
	switch val := v.(type) {
	case string:
		resolved, err := secrets.Resolve(val)
		if err != nil {
			e.logger.Warn("secret resolve failed",
				slog.String("value", val),
				slog.Any("error", err))
			return val // Return original on error
		}
		return resolved
	case map[string]any:
		return e.resolveSpecSecrets(val)
	case []any:
		resolved := make([]any, len(val))
		for i, item := range val {
			resolved[i] = e.resolveValue(item)
		}
		return resolved
	default:
		return v
	}
}
