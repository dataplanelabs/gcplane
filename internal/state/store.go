// Package state tracks the reconciliation state of managed resources.
package state

import "time"

// ResourceState tracks the last known state of a managed resource.
type ResourceState struct {
	Kind       string    `json:"kind"`
	Key        string    `json:"key"`
	ExternalID string    `json:"externalId,omitempty"` // GoClaw UUID
	SpecHash   string    `json:"specHash"`             // SHA256 of desired spec
	Synced     bool      `json:"synced"`
	LastSync   time.Time `json:"lastSync"`
	Error      string    `json:"error,omitempty"`
}

// Store is the interface for persisting reconciliation state.
type Store interface {
	// Get returns the state for a resource, or nil if not tracked.
	Get(kind, key string) (*ResourceState, error)

	// Put upserts the state for a resource.
	Put(state *ResourceState) error

	// List returns all tracked resource states.
	List() ([]*ResourceState, error)

	// Delete removes a tracked resource.
	Delete(kind, key string) error

	// Close releases any resources held by the store.
	Close() error
}
