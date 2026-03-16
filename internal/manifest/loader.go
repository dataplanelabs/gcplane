package manifest

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// Load reads a manifest from a file or merges all YAML files in a directory.
func Load(path string) (*Manifest, error) {
	info, err := os.Stat(path)
	if err != nil {
		return nil, fmt.Errorf("stat %s: %w", path, err)
	}

	if info.IsDir() {
		return loadDir(path)
	}
	return loadFile(path)
}

// loadFile parses a single YAML manifest file.
func loadFile(path string) (*Manifest, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", path, err)
	}

	var m Manifest
	if err := yaml.Unmarshal(data, &m); err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}

	if m.APIVersion == "" {
		m.APIVersion = "gcplane.io/v1"
	}
	if m.Kind == "" {
		m.Kind = "Manifest"
	}

	return &m, nil
}

// loadDir loads all .yaml/.yml files in a directory and merges resources.
func loadDir(dir string) (*Manifest, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("readdir %s: %w", dir, err)
	}

	merged := &Manifest{
		APIVersion: "gcplane.io/v1",
		Kind:       "Manifest",
	}

	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		ext := strings.ToLower(filepath.Ext(e.Name()))
		if ext != ".yaml" && ext != ".yml" {
			continue
		}

		m, err := loadFile(filepath.Join(dir, e.Name()))
		if err != nil {
			return nil, err
		}

		// First file with connection info wins
		if merged.Connection.Endpoint == "" && m.Connection.Endpoint != "" {
			merged.Connection = m.Connection
		}
		if merged.Metadata.Name == "" && m.Metadata.Name != "" {
			merged.Metadata = m.Metadata
		}

		merged.Resources = append(merged.Resources, m.Resources...)
	}

	return merged, nil
}
