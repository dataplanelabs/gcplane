package controller

import (
	"sync"
	"time"
)

// Metrics tracks sync counters for Prometheus exposition. Thread-safe via mutex.
type Metrics struct {
	mu             sync.RWMutex
	SyncSuccess    int64
	SyncErrors     int64
	SyncDuration   time.Duration
	LastSyncTime   time.Time
	DriftDetected  int64 // counter: total drift events (cumulative)
	DriftResources int64 // gauge: resources drifted in last sync cycle
}

// Snapshot returns a copy of the current metrics (thread-safe).
func (m *Metrics) Snapshot() Metrics {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return Metrics{
		SyncSuccess:    m.SyncSuccess,
		SyncErrors:     m.SyncErrors,
		SyncDuration:   m.SyncDuration,
		LastSyncTime:   m.LastSyncTime,
		DriftDetected:  m.DriftDetected,
		DriftResources: m.DriftResources,
	}
}
