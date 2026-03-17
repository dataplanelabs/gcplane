package source

import (
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/dataplanelabs/gcplane/internal/manifest"
)

// FileSource reads a manifest from a local file or directory with SHA256 change detection.
type FileSource struct {
	path string
}

// NewFileSource creates a source that reads from a local file or directory path.
func NewFileSource(path string) *FileSource {
	return &FileSource{path: path}
}

// Fetch reads the file or directory, computes a SHA256 hash, loads and validates the manifest.
func (s *FileSource) Fetch() (*manifest.Manifest, string, error) {
	info, err := os.Stat(s.path)
	if err != nil {
		return nil, "", fmt.Errorf("stat manifest path: %w", err)
	}

	var hash string
	if info.IsDir() {
		hash, err = hashDir(s.path)
		if err != nil {
			return nil, "", fmt.Errorf("hash dir: %w", err)
		}
	} else {
		data, readErr := os.ReadFile(s.path)
		if readErr != nil {
			return nil, "", fmt.Errorf("read manifest file: %w", readErr)
		}
		hash = fmt.Sprintf("%x", sha256.Sum256(data))
	}

	m, err := manifest.Load(s.path)
	if err != nil {
		return nil, "", fmt.Errorf("load manifest: %w", err)
	}

	if errs := manifest.Validate(m); len(errs) > 0 {
		return nil, "", fmt.Errorf("validate manifest: %v", errs[0])
	}

	return m, hash, nil
}

// hashDir computes a SHA256 hash over all YAML files in a directory (sorted by name).
func hashDir(dir string) (string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return "", fmt.Errorf("readdir %s: %w", dir, err)
	}

	// Collect yaml files sorted by name for deterministic hashing
	var files []string
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		ext := strings.ToLower(filepath.Ext(e.Name()))
		if ext == ".yaml" || ext == ".yml" {
			files = append(files, e.Name())
		}
	}
	sort.Strings(files)

	h := sha256.New()
	for _, name := range files {
		data, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			return "", fmt.Errorf("read %s: %w", name, err)
		}
		fmt.Fprintf(h, "%s:", name)
		h.Write(data)
	}
	return fmt.Sprintf("%x", h.Sum(nil)), nil
}
