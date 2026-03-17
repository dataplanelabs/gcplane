package source

import (
	"crypto/sha256"
	"fmt"
	"os"

	"github.com/dataplanelabs/gcplane/internal/manifest"
)

// FileSource reads a manifest from a local file with SHA256 change detection.
type FileSource struct {
	path string
}

// NewFileSource creates a source that reads from a local file path.
func NewFileSource(path string) *FileSource {
	return &FileSource{path: path}
}

// Fetch reads the file, computes its SHA256 hash, loads and validates the manifest.
func (s *FileSource) Fetch() (*manifest.Manifest, string, error) {
	data, err := os.ReadFile(s.path)
	if err != nil {
		return nil, "", fmt.Errorf("read manifest file: %w", err)
	}

	hash := fmt.Sprintf("%x", sha256.Sum256(data))

	m, err := manifest.Load(s.path)
	if err != nil {
		return nil, "", fmt.Errorf("load manifest: %w", err)
	}

	if errs := manifest.Validate(m); len(errs) > 0 {
		return nil, "", fmt.Errorf("validate manifest: %v", errs[0])
	}

	return m, hash, nil
}
