// Package controller implements the continuous reconciliation loop.
package controller

import (
	"sync"
	"time"

	"github.com/dataplanelabs/gcplane/internal/manifest"
)

// ConditionType classifies the sync state.
type ConditionType string

const (
	ConditionSynced  ConditionType = "Synced"
	ConditionError   ConditionType = "Error"
	ConditionDrifted ConditionType = "Drifted"
)

// Condition describes a sync state at a point in time (k8s-style).
type Condition struct {
	Type               ConditionType `json:"type"`
	Status             string        `json:"status"` // "True" or "False"
	Message            string        `json:"message,omitempty"`
	LastTransitionTime time.Time     `json:"lastTransitionTime"`
}

// ResourceStatus tracks per-resource sync outcome.
type ResourceStatus struct {
	Kind    manifest.ResourceKind `json:"kind"`
	Key     string                `json:"key"`
	Status  string                `json:"status"` // InSync, Created, Updated, Error
	Message string                `json:"message,omitempty"`
}

// SyncStatus is the overall reconciliation status snapshot.
type SyncStatus struct {
	LastSyncTime time.Time        `json:"lastSyncTime"`
	LastSyncHash string           `json:"lastSyncHash"`
	Conditions   []Condition      `json:"conditions"`
	Resources    []ResourceStatus `json:"resources"`
}

// StatusTracker provides thread-safe read/write access to SyncStatus.
type StatusTracker struct {
	mu     sync.RWMutex
	status SyncStatus
}

// NewStatusTracker creates a new empty status tracker.
func NewStatusTracker() *StatusTracker { return &StatusTracker{} }

// Get returns a copy of the current sync status.
func (t *StatusTracker) Get() SyncStatus {
	t.mu.RLock()
	defer t.mu.RUnlock()
	s := t.status
	s.Conditions = append([]Condition(nil), t.status.Conditions...)
	s.Resources = append([]ResourceStatus(nil), t.status.Resources...)
	return s
}

// Update replaces the current sync status.
func (t *StatusTracker) Update(s SyncStatus) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.status = s
}

// SetCondition upserts a condition by type.
// Only updates LastTransitionTime when Status changes.
func (t *StatusTracker) SetCondition(cond Condition) {
	t.mu.Lock()
	defer t.mu.Unlock()
	for i, c := range t.status.Conditions {
		if c.Type == cond.Type {
			if c.Status != cond.Status {
				cond.LastTransitionTime = time.Now()
			} else {
				cond.LastTransitionTime = c.LastTransitionTime
			}
			t.status.Conditions[i] = cond
			return
		}
	}
	cond.LastTransitionTime = time.Now()
	t.status.Conditions = append(t.status.Conditions, cond)
}

// IsSynced returns true if the Synced condition is "True".
func (t *StatusTracker) IsSynced() bool {
	t.mu.RLock()
	defer t.mu.RUnlock()
	for _, c := range t.status.Conditions {
		if c.Type == ConditionSynced {
			return c.Status == "True"
		}
	}
	return false
}
