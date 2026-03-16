// Package reconciler implements the Observe→Compare→Act reconciliation engine.
package reconciler

import "github.com/dataplanelabs/gcplane/internal/manifest"

// Action represents a planned change to a resource.
type Action string

const (
	ActionCreate Action = "create"
	ActionUpdate Action = "update"
	ActionNoop   Action = "noop"
)

// Change describes a single planned resource change.
type Change struct {
	Kind   manifest.ResourceKind `json:"kind"`
	Key    string                `json:"key"`
	Action Action                `json:"action"`
	Diff   map[string]FieldDiff  `json:"diff,omitempty"`
}

// FieldDiff shows the before/after for a single field.
type FieldDiff struct {
	Old any `json:"old,omitempty"`
	New any `json:"new,omitempty"`
}

// Plan is the result of a dry-run reconciliation.
type Plan struct {
	Changes  []Change `json:"changes"`
	Creates  int      `json:"creates"`
	Updates  int      `json:"updates"`
	Noops    int      `json:"noops"`
	Errors   []string `json:"errors,omitempty"`
}

// ApplyResult is the result of applying a plan.
type ApplyResult struct {
	Applied int      `json:"applied"`
	Failed  int      `json:"failed"`
	Errors  []string `json:"errors,omitempty"`
}
