package skillpkg

import (
	"bytes"
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// SkillFile is a single file retrieved from the goclaw skill file API.
type SkillFile struct {
	Path string // relative, slash-separated
	Data []byte
}

// UnpackTo writes files into outDir, creating parent dirs as needed.
// Only writes a file when its content differs from what's on disk (sha256 compare).
// Rejects ".." traversal and absolute paths; skips hidden/system artifacts.
// prune=true deletes local files absent from the server response.
// Returns the relative paths of files actually written.
func UnpackTo(outDir string, files []SkillFile, prune bool) ([]string, error) {
	if outDir == "" {
		return nil, fmt.Errorf("outDir must not be empty")
	}

	written := make(map[string]bool, len(files))
	var changed []string

	for _, f := range files {
		rel := filepath.FromSlash(f.Path)

		// Reject absolute paths and ".." traversal.
		if filepath.IsAbs(rel) || strings.Contains(rel, "..") {
			return nil, fmt.Errorf("rejected unsafe path %q", f.Path)
		}

		name := filepath.Base(rel)
		if isSkippedFile(name) || isSkippedDir(strings.Split(rel, string(filepath.Separator))[0]) {
			continue
		}

		if int64(len(f.Data)) > MaxFileSize {
			return nil, fmt.Errorf("file %q exceeds max size %d bytes", f.Path, MaxFileSize)
		}

		abs := filepath.Join(outDir, rel)
		// Double-check the resolved path is within outDir.
		if !strings.HasPrefix(abs, outDir+string(filepath.Separator)) && abs != outDir {
			return nil, fmt.Errorf("rejected path escaping outDir: %q", f.Path)
		}

		if !needsWrite(abs, f.Data) {
			written[rel] = true
			continue
		}

		if err := os.MkdirAll(filepath.Dir(abs), 0o755); err != nil {
			return nil, fmt.Errorf("mkdir %s: %w", filepath.Dir(abs), err)
		}
		if err := os.WriteFile(abs, f.Data, 0o644); err != nil {
			return nil, fmt.Errorf("write %s: %w", rel, err)
		}
		changed = append(changed, rel)
		written[rel] = true
	}

	if prune {
		pruned, err := pruneLocal(outDir, written)
		if err != nil {
			return nil, err
		}
		changed = append(changed, pruned...)
	}

	return changed, nil
}

// needsWrite returns true when the file is absent or its content hash differs.
func needsWrite(abs string, data []byte) bool {
	existing, err := os.ReadFile(abs)
	if err != nil {
		return true
	}
	want := sha256.Sum256(data)
	got := sha256.Sum256(existing)
	return !bytes.Equal(want[:], got[:])
}

// pruneLocal deletes files under outDir not in the keep set.
func pruneLocal(outDir string, keep map[string]bool) ([]string, error) {
	var deleted []string
	err := filepath.WalkDir(outDir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		rel, err := filepath.Rel(outDir, path)
		if err != nil {
			return err
		}
		if !keep[rel] {
			if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
				return fmt.Errorf("prune %s: %w", rel, err)
			}
			deleted = append(deleted, rel)
		}
		return nil
	})
	return deleted, err
}
