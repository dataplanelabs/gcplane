package controller

import (
	"log/slog"
	"sync"
	"time"

	"github.com/dataplanelabs/gcplane/internal/reconciler"
	"github.com/dataplanelabs/gcplane/internal/source"
)

// Controller orchestrates the periodic reconcile loop.
type Controller struct {
	source    source.ManifestSource
	provider  reconciler.ProviderInterface
	tracker   *StatusTracker
	metrics   *Metrics
	interval  time.Duration
	triggerCh chan struct{}
	logger    *slog.Logger
	lastHash  string
}

// Metrics tracks sync counters for Prometheus exposition. Thread-safe via mutex.
type Metrics struct {
	mu           sync.RWMutex
	SyncSuccess  int64
	SyncErrors   int64
	SyncDuration time.Duration
	LastSyncTime time.Time
}

// Snapshot returns a copy of the current metrics (thread-safe).
func (m *Metrics) Snapshot() Metrics {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return Metrics{SyncSuccess: m.SyncSuccess, SyncErrors: m.SyncErrors, SyncDuration: m.SyncDuration, LastSyncTime: m.LastSyncTime}
}

// Config holds controller dependencies.
type Config struct {
	Source   source.ManifestSource
	Provider reconciler.ProviderInterface
	Tracker  *StatusTracker
	Interval time.Duration
	Logger   *slog.Logger
}

// New creates a controller with the given config.
func New(cfg Config) *Controller {
	return &Controller{
		source:    cfg.Source,
		provider:  cfg.Provider,
		tracker:   cfg.Tracker,
		metrics:   &Metrics{},
		interval:  cfg.Interval,
		triggerCh: make(chan struct{}, 1),
		logger:    cfg.Logger,
	}
}

// Metrics returns the current metrics snapshot.
func (c *Controller) GetMetrics() *Metrics { return c.metrics }

// Run starts the reconcile loop. Blocks until ctx is cancelled.
func (c *Controller) Run(done <-chan struct{}) {
	// Run initial reconcile immediately
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
		c.logger.Info("manifest unchanged, skipping", "hash", hash[:12])
		return
	}

	engine := reconciler.NewEngine(c.provider)
	plan, result := engine.Reconcile(m, false)

	// Build resource statuses from plan + result
	resources := buildResourceStatuses(plan, result)
	duration := time.Since(start)

	status := SyncStatus{
		LastSyncTime: time.Now(),
		LastSyncHash: hash,
		Resources:    resources,
	}
	c.tracker.Update(status)

	// Set conditions
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
	} else {
		c.tracker.SetCondition(Condition{Type: ConditionDrifted, Status: "False"})
	}

	c.lastHash = hash
	c.metrics.mu.Lock()
	if hasErrors {
		c.metrics.SyncErrors++
	} else {
		c.metrics.SyncSuccess++
	}
	c.metrics.SyncDuration = duration
	c.metrics.LastSyncTime = time.Now()
	c.metrics.mu.Unlock()

	c.logger.Info("reconcile complete",
		"creates", plan.Creates, "updates", plan.Updates, "noops", plan.Noops,
		"applied", result.Applied, "failed", result.Failed,
		"duration", duration.Round(time.Millisecond))
}

// buildResourceStatuses maps plan changes + apply results to per-resource statuses.
func buildResourceStatuses(plan *reconciler.Plan, _ *reconciler.ApplyResult) []ResourceStatus {
	statuses := make([]ResourceStatus, 0, len(plan.Changes))
	for _, c := range plan.Changes {
		rs := ResourceStatus{Kind: c.Kind, Name: c.Name}
		switch {
		case c.Error != "":
			rs.Status = "Error"
			rs.Message = c.Error
		case c.Action == reconciler.ActionNoop:
			rs.Status = "InSync"
		case c.Action == reconciler.ActionCreate:
			rs.Status = "Created"
		case c.Action == reconciler.ActionUpdate:
			rs.Status = "Updated"
		}
		statuses = append(statuses, rs)
	}
	return statuses
}
