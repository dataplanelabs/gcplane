// Package tui implements a k9s-style terminal UI for monitoring GoClaw resources.
package tui

import (
	"strings"
	"sync"
	"time"

	"github.com/dataplanelabs/gcplane/internal/manifest"
	"github.com/dataplanelabs/gcplane/internal/reconciler"
)

// Model holds the shared state for the TUI, protected by a read-write mutex.
type Model struct {
	mu           sync.RWMutex
	manifest     *manifest.Manifest
	plan         *reconciler.Plan
	currentKind  manifest.ResourceKind // empty string = all kinds
	searchFilter string                // case-insensitive name filter
	endpoint     string
	manifestName string
	lastRefresh  time.Time
	interval     time.Duration
	err          error
}

// NewModel creates a Model from a loaded manifest and refresh interval.
func NewModel(m *manifest.Manifest, ep string, interval time.Duration) *Model {
	return &Model{
		manifest:     m,
		endpoint:     ep,
		manifestName: m.Metadata.Name,
		interval:     interval,
	}
}

// UpdatePlan replaces the current plan and clears any previous error.
func (m *Model) UpdatePlan(plan *reconciler.Plan) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.plan = plan
	m.lastRefresh = time.Now()
	m.err = nil
}

// SetError records the last refresh error.
func (m *Model) SetError(err error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.err = err
	m.lastRefresh = time.Now()
}

// GetError returns the last error, if any.
func (m *Model) GetError() error {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.err
}

// SetKind sets the resource kind filter. Empty string means all kinds.
func (m *Model) SetKind(kind manifest.ResourceKind) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.currentKind = kind
}

// GetKind returns the current kind filter.
func (m *Model) GetKind() manifest.ResourceKind {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.currentKind
}

// SetFilter sets the search filter string (case-insensitive name match).
func (m *Model) SetFilter(filter string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.searchFilter = filter
}

// GetFilter returns the current search filter.
func (m *Model) GetFilter() string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.searchFilter
}

// GetChanges returns plan changes filtered by current kind and search filter.
func (m *Model) GetChanges() []reconciler.Change {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if m.plan == nil {
		return nil
	}
	var filtered []reconciler.Change
	for _, c := range m.plan.Changes {
		if m.currentKind != "" && c.Kind != m.currentKind {
			continue
		}
		if m.searchFilter != "" && !strings.Contains(strings.ToLower(c.Name), strings.ToLower(m.searchFilter)) {
			continue
		}
		filtered = append(filtered, c)
	}
	return filtered
}

// GetPlan returns the current plan snapshot.
func (m *Model) GetPlan() *reconciler.Plan {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.plan
}

// GetManifestName returns the manifest name.
func (m *Model) GetManifestName() string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.manifestName
}

// GetEndpoint returns the GoClaw endpoint.
func (m *Model) GetEndpoint() string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.endpoint
}

// GetLastRefresh returns the last refresh timestamp.
func (m *Model) GetLastRefresh() time.Time {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.lastRefresh
}

// GetInterval returns the refresh interval.
func (m *Model) GetInterval() time.Duration {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.interval
}
