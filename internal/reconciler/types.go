// Package reconciler implements the Observe→Compare→Act reconciliation engine.
package reconciler

import "github.com/dataplanelabs/gcplane/internal/manifest"

// Action represents a planned change to a resource.
type Action string

const (
	ActionCreate Action = "create"
	ActionUpdate Action = "update"
	ActionNoop   Action = "noop"
	ActionDelete Action = "delete"
)

// Change describes a single planned resource change.
type Change struct {
	Kind   manifest.ResourceKind `json:"kind"`
	Name   string                `json:"name"`
	Action Action                `json:"action"`
	Diff   map[string]FieldDiff  `json:"diff,omitempty"`
	Error  string                `json:"error,omitempty"`
	Forced bool                  `json:"forced,omitempty"` // true when update triggered by --force with no diff
}

// FieldDiff shows the before/after for a single field.
type FieldDiff struct {
	Old any `json:"old,omitempty"`
	New any `json:"new,omitempty"`
}

// Plan is the result of a dry-run reconciliation.
type Plan struct {
	Changes []Change `json:"changes"`
	Creates int      `json:"creates"`
	Updates int      `json:"updates"`
	Deletes int      `json:"deletes"`
	Noops   int      `json:"noops"`
	Errors  []string `json:"errors,omitempty"`
}

// ReconcileOpts controls reconciliation behaviour.
type ReconcileOpts struct {
	DryRun bool
	Prune  bool
	Force  bool // Force re-applies all resources, even when no diff detected
}

// ApplyResult is the result of applying a plan.
type ApplyResult struct {
	Applied int      `json:"applied"`
	Failed  int      `json:"failed"`
	Errors  []string `json:"errors,omitempty"`
}

// ResourceInfo is a lightweight resource reference for prune discovery.
type ResourceInfo struct {
	Kind      manifest.ResourceKind
	Name      string
	CreatedBy string
}
