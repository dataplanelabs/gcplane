// Package skillpkg builds deterministic ZIP archives from on-disk skill
// directories. The output is byte-for-byte stable across runs: sorted file
// paths, zeroed timestamps, fixed mode bits. Stability lets callers SHA-256
// the archive as a content version.
package skillpkg

import (
	"archive/zip"
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// SkillFiles is the canonical filename for a skill's entrypoint.
const SkillFilename = "SKILL.md"

// MaxFileSize caps any single file inside a skill ZIP at 4 MiB.
// Large binaries belong in package managers, not skills.
const MaxFileSize = 4 << 20

// PackResult captures the build output.
type PackResult struct {
	ZIP         []byte // ZIP archive contents
	ContentHash string // SHA-256 of the archive (hex)
	SkillMD     string // contents of SKILL.md (UTF-8)
}

// PackDir walks dir (must contain SKILL.md at its root), packs every regular
// file into a deterministic ZIP, and returns the archive + content hash.
// Symlinks, hidden files (.git, .DS_Store, etc.), and oversize files are
// skipped to keep uploads safe.
func PackDir(dir string) (*PackResult, error) {
	skillPath := filepath.Join(dir, SkillFilename)
	skillBytes, err := os.ReadFile(skillPath)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", SkillFilename, err)
	}

	files, err := collectFiles(dir)
	if err != nil {
		return nil, err
	}
	sort.Strings(files)

	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	for _, rel := range files {
		abs := filepath.Join(dir, rel)
		data, err := os.ReadFile(abs)
		if err != nil {
			return nil, fmt.Errorf("read %s: %w", rel, err)
		}
		if int64(len(data)) > MaxFileSize {
			return nil, fmt.Errorf("%s exceeds %d bytes", rel, MaxFileSize)
		}
		fh := &zip.FileHeader{
			Name:   filepath.ToSlash(rel),
			Method: zip.Deflate,
		}
		// Zero out time so identical content always hashes the same.
		fh.SetMode(0o644)
		w, err := zw.CreateHeader(fh)
		if err != nil {
			return nil, fmt.Errorf("zip create %s: %w", rel, err)
		}
		if _, err := io.Copy(w, bytes.NewReader(data)); err != nil {
			return nil, fmt.Errorf("zip write %s: %w", rel, err)
		}
	}
	if err := zw.Close(); err != nil {
		return nil, fmt.Errorf("zip close: %w", err)
	}

	sum := sha256.Sum256(buf.Bytes())
	return &PackResult{
		ZIP:         buf.Bytes(),
		ContentHash: hex.EncodeToString(sum[:]),
		SkillMD:     string(skillBytes),
	}, nil
}

func collectFiles(root string) ([]string, error) {
	var out []string
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if isSkippedDir(d.Name()) && path != root {
				return fs.SkipDir
			}
			return nil
		}
		if !d.Type().IsRegular() {
			return nil // skip symlinks, devices, etc.
		}
		if isSkippedFile(d.Name()) {
			return nil
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		out = append(out, rel)
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("walk %s: %w", root, err)
	}
	return out, nil
}

func isSkippedDir(name string) bool {
	switch name {
	case ".git", ".github", "node_modules", "__pycache__", ".venv", "venv":
		return true
	}
	return strings.HasPrefix(name, ".")
}

func isSkippedFile(name string) bool {
	switch name {
	case ".DS_Store", "Thumbs.db", ".gitkeep", ".gitignore":
		return true
	}
	return strings.HasPrefix(name, ".")
}
