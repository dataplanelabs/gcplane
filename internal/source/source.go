// Package source provides manifest sources for the reconcile loop.
package source

import "github.com/dataplanelabs/gcplane/internal/manifest"

// ManifestSource fetches a manifest and returns a content hash for change detection.
type ManifestSource interface {
	// Fetch returns the current manifest, a content hash (opaque string for
	// equality comparison), and any error. Implementations validate the manifest.
	Fetch() (*manifest.Manifest, string, error)
}
