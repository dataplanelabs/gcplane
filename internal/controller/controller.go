package controller

import (
	"context"
	"log/slog"
	"time"

	"github.com/dataplanelabs/gcplane/internal/reconciler"
	"github.com/dataplanelabs/gcplane/internal/source"
)

// Notifier sends drift alerts for a sync cycle.
type Notifier interface {
	NotifyDrift(ctx context.Context, changes []reconciler.Change) error
}

// Controller orchestrates the periodic reconcile loop.
type Controller struct {
	source    source.ManifestSource
	provider  reconciler.ProviderInterface
	tracker   *StatusTracker
	metrics   *Metrics
	notifier  Notifier
	interval  time.Duration
	prune     bool
	triggerCh chan struct{}
	logger    *slog.Logger
	lastHash  string
}

// Config holds controller dependencies.
type Config struct {
	Source   source.ManifestSource
	Provider reconciler.ProviderInterface
	Tracker  *StatusTracker
	Notifier Notifier
	Interval time.Duration
	Prune    bool
	Logger   *slog.Logger
}

// New creates a controller with the given config.
func New(cfg Config) *Controller {
	return &Controller{
		source:    cfg.Source,
		provider:  cfg.Provider,
		tracker:   cfg.Tracker,
		metrics:   &Metrics{},
		notifier:  cfg.Notifier,
		interval:  cfg.Interval,
		prune:     cfg.Prune,
		triggerCh: make(chan struct{}, 1),
		logger:    cfg.Logger,
	}
}

// GetMetrics returns the current metrics snapshot.
func (c *Controller) GetMetrics() *Metrics { return c.metrics }

// Run starts the reconcile loop. Blocks until done is closed.
func (c *Controller) Run(done <-chan struct{}) {
	c.reconcileOnce()

	ticker := time.NewTicker(c.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			c.reconcileOnce()
		case <-c.triggerCh:
			c.reconcileOnce()
		case <-done:
			c.logger.Info("controller stopped")
			return
		}
	}
}

// Trigger requests an immediate reconcile. Non-blocking; drops if already pending.
func (c *Controller) Trigger() {
	select {
	case c.triggerCh <- struct{}{}:
	default:
	}
}

// reconcileOnce fetches the manifest and reconciles against GoClaw.
func (c *Controller) reconcileOnce() {
	start := time.Now()
	c.logger.Info("reconcile started")

	m, hash, err := c.source.Fetch()
	if err != nil {
		c.logger.Error("fetch manifest failed", "error", err)
		c.tracker.SetCondition(Condition{Type: ConditionError, Status: "True", Message: err.Error()})
		c.tracker.SetCondition(Condition{Type: ConditionSynced, Status: "False"})
		c.metrics.mu.Lock()
		c.metrics.SyncErrors++
		c.metrics.mu.Unlock()
		return
	}

	// Skip if manifest unchanged
	if hash == c.lastHash && hash != "" {
		display := hash
		if len(display) > 12 {
			display = display[:12]
		}
		c.logger.Info("manifest unchanged, skipping", "hash", display)
		return
	}

	engine := reconciler.NewEngine(c.provider)
	plan, result := engine.Reconcile(m, reconciler.ReconcileOpts{DryRun: false, Prune: c.prune})

	resources := buildResourceStatuses(plan, result)
	duration := time.Since(start)

	status := SyncStatus{
		LastSyncTime: time.Now(),
		LastSyncHash: hash,
		Resources:    resources,
	}
	c.tracker.Update(status)

	hasErrors := result.Failed > 0 || len(plan.Errors) > 0
	hasDrift := plan.Creates > 0 || plan.Updates > 0

	if hasErrors {
		c.tracker.SetCondition(Condition{Type: ConditionSynced, Status: "False"})
		c.tracker.SetCondition(Condition{Type: ConditionError, Status: "True", Message: "sync completed with errors"})
	} else {
		c.tracker.SetCondition(Condition{Type: ConditionSynced, Status: "True"})
		c.tracker.SetCondition(Condition{Type: ConditionError, Status: "False"})
	}

	if hasDrift {
		c.tracker.SetCondition(Condition{Type: ConditionDrifted, Status: "True", Message: "resources were created or updated"})
		c.logDriftChanges(plan.Changes)
		c.notifyDrift(plan.Changes)
	} else {
		c.tracker.SetCondition(Condition{Type: ConditionDrifted, Status: "False"})
	}

	c.lastHash = hash

	driftCount := int64(plan.Creates + plan.Updates)
	c.metrics.mu.Lock()
	if hasErrors {
		c.metrics.SyncErrors++
	} else {
		c.metrics.SyncSuccess++
	}
	c.metrics.SyncDuration = duration
	c.metrics.LastSyncTime = time.Now()
	c.metrics.DriftResources = driftCount
	if hasDrift {
		c.metrics.DriftDetected++
	}
	c.metrics.mu.Unlock()

	c.logger.Info("reconcile complete",
		"creates", plan.Creates, "updates", plan.Updates, "noops", plan.Noops,
		"applied", result.Applied, "failed", result.Failed,
		"duration", duration.Round(time.Millisecond))
}

// logDriftChanges logs each drifted resource at Info level.
func (c *Controller) logDriftChanges(changes []reconciler.Change) {
	for _, ch := range changes {
		if ch.Action == reconciler.ActionCreate || ch.Action == reconciler.ActionUpdate {
			c.logger.Info("drift detected", "kind", ch.Kind, "name", ch.Name, "action", ch.Action)
		}
	}
}

// notifyDrift calls the notifier if one is configured. Logs errors but does not fail the sync.
func (c *Controller) notifyDrift(changes []reconciler.Change) {
	if c.notifier == nil {
		return
	}
	drifted := make([]reconciler.Change, 0, len(changes))
	for _, ch := range changes {
		if ch.Action == reconciler.ActionCreate || ch.Action == reconciler.ActionUpdate {
			drifted = append(drifted, ch)
		}
	}
	if len(drifted) == 0 {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := c.notifier.NotifyDrift(ctx, drifted); err != nil {
		c.logger.Warn("drift notification failed", "error", err)
	}
}

// buildResourceStatuses maps plan changes + apply results to per-resource statuses.
func buildResourceStatuses(plan *reconciler.Plan, _ *reconciler.ApplyResult) []ResourceStatus {
	statuses := make([]ResourceStatus, 0, len(plan.Changes))
	for _, ch := range plan.Changes {
		rs := ResourceStatus{Kind: ch.Kind, Name: ch.Name}
		switch {
		case ch.Error != "":
			rs.Status = "Error"
			rs.Message = ch.Error
		case ch.Action == reconciler.ActionNoop:
			rs.Status = "InSync"
		case ch.Action == reconciler.ActionCreate:
			rs.Status = "Created"
		case ch.Action == reconciler.ActionUpdate:
			rs.Status = "Updated"
		}
		statuses = append(statuses, rs)
	}
	return statuses
}

