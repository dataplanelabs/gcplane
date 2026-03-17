package controller

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/dataplanelabs/gcplane/internal/provider/goclaw"
	"github.com/dataplanelabs/gcplane/internal/secrets"
	"github.com/dataplanelabs/gcplane/internal/source"
)

// TenantInstance groups per-tenant components.
type TenantInstance struct {
	Name       string
	Controller *Controller
	Tracker    *StatusTracker
	provider   *goclaw.Provider
}

// TenantManager holds all tenant instances and manages their lifecycle.
type TenantManager struct {
	mu      sync.RWMutex
	tenants map[string]*TenantInstance
	logger  *slog.Logger
}

// TenantManagerConfig holds configuration for creating a TenantManager.
type TenantManagerConfig struct {
	TenantsDir string
	Interval   time.Duration
	Prune      bool
	Logger     *slog.Logger
}

// NewTenantManager discovers tenant subdirectories and creates a controller per tenant.
// Tenants with missing or invalid config are skipped with an error log.
func NewTenantManager(cfg TenantManagerConfig) (*TenantManager, error) {
	entries, err := os.ReadDir(cfg.TenantsDir)
	if err != nil {
		return nil, fmt.Errorf("read tenants dir: %w", err)
	}

	tm := &TenantManager{
		tenants: make(map[string]*TenantInstance),
		logger:  cfg.Logger,
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		name := entry.Name()
		tenantDir := filepath.Join(cfg.TenantsDir, name)

		src := source.NewFileSource(tenantDir)
		m, _, err := src.Fetch()
		if err != nil {
			cfg.Logger.Error("skip tenant: manifest load failed", "tenant", name, "error", err)
			continue
		}

		ep := m.Connection.Endpoint
		tok := m.Connection.Token
		if ep == "" || tok == "" {
			cfg.Logger.Error("skip tenant: missing connection endpoint or token", "tenant", name)
			continue
		}

		ep, err = secrets.Resolve(ep)
		if err != nil {
			cfg.Logger.Error("skip tenant: resolve endpoint failed", "tenant", name, "error", err)
			continue
		}
		tok, err = secrets.Resolve(tok)
		if err != nil {
			cfg.Logger.Error("skip tenant: resolve token failed", "tenant", name, "error", err)
			continue
		}

		provider := goclaw.New(ep, tok)
		tracker := NewStatusTracker()
		ctrl := New(Config{
			Source:   src,
			Provider: provider,
			Tracker:  tracker,
			Interval: cfg.Interval,
			Prune:    cfg.Prune,
			Logger:   cfg.Logger.With("tenant", name),
		})

		tm.tenants[name] = &TenantInstance{
			Name:       name,
			Controller: ctrl,
			Tracker:    tracker,
			provider:   provider,
		}
		cfg.Logger.Info("discovered tenant", "name", name)
	}

	if len(tm.tenants) == 0 {
		return nil, fmt.Errorf("no valid tenants found in %s", cfg.TenantsDir)
	}

	return tm, nil
}

// RunAll starts all tenant controller goroutines and blocks until done is closed.
func (tm *TenantManager) RunAll(done <-chan struct{}) {
	tm.mu.RLock()
	for name, inst := range tm.tenants {
		tm.logger.Info("starting tenant controller", "tenant", name)
		go inst.Controller.Run(done)
	}
	tm.mu.RUnlock()
	<-done
}

// Get returns the TenantInstance for the given name, or nil if not found.
func (tm *TenantManager) Get(name string) (*TenantInstance, bool) {
	tm.mu.RLock()
	defer tm.mu.RUnlock()
	inst, ok := tm.tenants[name]
	return inst, ok
}

// All returns a copy of the tenant map.
func (tm *TenantManager) All() map[string]*TenantInstance {
	tm.mu.RLock()
	defer tm.mu.RUnlock()
	out := make(map[string]*TenantInstance, len(tm.tenants))
	for k, v := range tm.tenants {
		out[k] = v
	}
	return out
}

// Trigger triggers an immediate sync for a named tenant. Returns false if not found.
func (tm *TenantManager) Trigger(name string) bool {
	tm.mu.RLock()
	defer tm.mu.RUnlock()
	if inst, ok := tm.tenants[name]; ok {
		inst.Controller.Trigger()
		return true
	}
	return false
}

// TriggerAll triggers an immediate sync for all tenants.
func (tm *TenantManager) TriggerAll() {
	tm.mu.RLock()
	defer tm.mu.RUnlock()
	for _, inst := range tm.tenants {
		inst.Controller.Trigger()
	}
}

// AggregatedStatus returns a map of tenant name to SyncStatus.
func (tm *TenantManager) AggregatedStatus() map[string]SyncStatus {
	tm.mu.RLock()
	defer tm.mu.RUnlock()
	out := make(map[string]SyncStatus, len(tm.tenants))
	for name, inst := range tm.tenants {
		out[name] = inst.Tracker.Get()
	}
	return out
}

// AggregatedMetrics sums metrics across all tenant controllers.
// Returns a pointer to avoid copying the embedded RWMutex.
func (tm *TenantManager) AggregatedMetrics() *Metrics {
	tm.mu.RLock()
	defer tm.mu.RUnlock()
	agg := &Metrics{}
	for _, inst := range tm.tenants {
		snap := inst.Controller.GetMetrics().Snapshot()
		agg.SyncSuccess += snap.SyncSuccess
		agg.SyncErrors += snap.SyncErrors
		if snap.LastSyncTime.After(agg.LastSyncTime) {
			agg.LastSyncTime = snap.LastSyncTime
			agg.SyncDuration = snap.SyncDuration
		}
	}
	return agg
}

// CloseAll closes all tenant providers.
func (tm *TenantManager) CloseAll() {
	tm.mu.RLock()
	defer tm.mu.RUnlock()
	for name, inst := range tm.tenants {
		if err := inst.provider.Close(); err != nil {
			tm.logger.Warn("error closing provider", "tenant", name, "error", err)
		}
	}
}
