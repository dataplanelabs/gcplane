package source

import (
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/dataplanelabs/gcplane/internal/manifest"
)

// GitSource clones/fetches a git repo and reads a manifest from it.
type GitSource struct {
	repo   string // remote URL (SSH or HTTPS)
	branch string // branch name
	path   string // manifest file path within repo
	dir    string // local clone directory (set after first fetch)
	logger *slog.Logger
}

// NewGitSource creates a source that watches a git repository.
// Returns error if inputs contain potentially dangerous values.
func NewGitSource(repo, branch, path string, logger *slog.Logger) (*GitSource, error) {
	if branch == "" {
		branch = "main"
	}
	if path == "" {
		path = "manifest.yaml"
	}
	// Prevent git argument injection
	for _, v := range []string{repo, branch, path} {
		if strings.HasPrefix(v, "-") {
			return nil, fmt.Errorf("invalid git parameter %q: must not start with '-'", v)
		}
	}
	return &GitSource{repo: repo, branch: branch, path: path, logger: logger}, nil
}

// Fetch clones (first call) or fetches (subsequent calls) the repo,
// then loads the manifest. Returns the git commit hash for change detection.
func (s *GitSource) Fetch() (*manifest.Manifest, string, error) {
	if s.dir == "" {
		if err := s.clone(); err != nil {
			return nil, "", err
		}
	} else {
		if err := s.fetch(); err != nil {
			return nil, "", err
		}
	}

	hash, err := s.runGit("rev-parse", "HEAD")
	if err != nil {
		return nil, "", fmt.Errorf("git rev-parse HEAD: %w", err)
	}

	manifestPath := filepath.Join(s.dir, s.path)
	m, err := manifest.Load(manifestPath)
	if err != nil {
		return nil, "", fmt.Errorf("load manifest from repo: %w", err)
	}

	if errs := manifest.Validate(m); len(errs) > 0 {
		return nil, "", fmt.Errorf("validate manifest: %v", errs[0])
	}

	return m, hash, nil
}

// Cleanup removes the temporary clone directory.
func (s *GitSource) Cleanup() {
	if s.dir != "" {
		os.RemoveAll(s.dir)
	}
}

func (s *GitSource) clone() error {
	dir, err := os.MkdirTemp("", "gcplane-git-*")
	if err != nil {
		return fmt.Errorf("create temp dir: %w", err)
	}

	s.logger.Info("cloning repo", "repo", s.repo, "branch", s.branch)
	_, err = s.runGitIn("", "clone", "--depth", "1", "--single-branch",
		"--branch", s.branch, s.repo, dir)
	if err != nil {
		os.RemoveAll(dir)
		return fmt.Errorf("git clone: %w", err)
	}

	s.dir = dir
	return nil
}

func (s *GitSource) fetch() error {
	if _, err := s.runGit("fetch", "origin", s.branch); err != nil {
		return fmt.Errorf("git fetch: %w", err)
	}
	if _, err := s.runGit("reset", "--hard", "origin/"+s.branch); err != nil {
		return fmt.Errorf("git reset: %w", err)
	}
	return nil
}

// runGit executes a git command in the clone directory.
func (s *GitSource) runGit(args ...string) (string, error) {
	return s.runGitIn(s.dir, args...)
}

// runGitIn executes a git command in the specified directory.
func (s *GitSource) runGitIn(dir string, args ...string) (string, error) {
	cmd := exec.Command("git", args...)
	if dir != "" {
		cmd.Dir = dir
	}
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("%s: %s", err, strings.TrimSpace(string(out)))
	}
	return strings.TrimSpace(string(out)), nil
}
