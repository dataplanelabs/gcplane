package source

import (
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// initBareRepo creates a local bare git repo with one commit containing manifest.yaml.
// Returns the file:// URL to the bare repo.
func initBareRepo(t *testing.T, manifestContent string) string {
	t.Helper()

	// Create a working tree, commit, then create bare clone
	work := t.TempDir()
	bare := t.TempDir()

	gitCmds := [][]string{
		{"git", "-C", work, "init", "-b", "main"},
		{"git", "-C", work, "config", "user.email", "test@test.com"},
		{"git", "-C", work, "config", "user.name", "Test"},
	}
	for _, args := range gitCmds {
		if out, err := exec.Command(args[0], args[1:]...).CombinedOutput(); err != nil {
			t.Fatalf("setup git %v: %s", args, out)
		}
	}

	mPath := filepath.Join(work, "manifest.yaml")
	if err := os.WriteFile(mPath, []byte(manifestContent), 0600); err != nil {
		t.Fatal(err)
	}

	addCommit := [][]string{
		{"git", "-C", work, "add", "manifest.yaml"},
		{"git", "-C", work, "commit", "-m", "initial"},
	}
	for _, args := range addCommit {
		if out, err := exec.Command(args[0], args[1:]...).CombinedOutput(); err != nil {
			t.Fatalf("git commit: %s", out)
		}
	}

	// Clone as bare repo so we can push to it
	if out, err := exec.Command("git", "clone", "--bare", work, bare).CombinedOutput(); err != nil {
		t.Fatalf("git clone --bare: %s", out)
	}

	return "file://" + bare
}

// addCommitToRepo adds a new commit to an existing non-bare working repo.
func addCommitToRepo(t *testing.T, repoURL, newContent string) {
	t.Helper()

	// Clone into temp, modify, push
	tmp := t.TempDir()
	clone := [][]string{
		{"git", "clone", repoURL, tmp},
		{"git", "-C", tmp, "config", "user.email", "test@test.com"},
		{"git", "-C", tmp, "config", "user.name", "Test"},
	}
	for _, args := range clone {
		if out, err := exec.Command(args[0], args[1:]...).CombinedOutput(); err != nil {
			t.Fatalf("git setup for push: %s", out)
		}
	}
	if err := os.WriteFile(filepath.Join(tmp, "manifest.yaml"), []byte(newContent), 0600); err != nil {
		t.Fatal(err)
	}
	push := [][]string{
		{"git", "-C", tmp, "add", "manifest.yaml"},
		{"git", "-C", tmp, "commit", "-m", "update"},
		{"git", "-C", tmp, "push"},
	}
	for _, args := range push {
		if out, err := exec.Command(args[0], args[1:]...).CombinedOutput(); err != nil {
			t.Fatalf("git push: %s", out)
		}
	}
}

func TestNewGitSource_Defaults(t *testing.T) {
	gs, err := NewGitSource("https://example.com/repo.git", "", "", slog.Default())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gs.branch != "main" {
		t.Errorf("expected default branch 'main', got %q", gs.branch)
	}
	if gs.path != "manifest.yaml" {
		t.Errorf("expected default path 'manifest.yaml', got %q", gs.path)
	}
}

func TestNewGitSource_RejectsDashPrefix(t *testing.T) {
	cases := []struct {
		repo, branch, path string
	}{
		{repo: "-bad-repo", branch: "main", path: "manifest.yaml"},
		{repo: "https://example.com", branch: "-bad-branch", path: "manifest.yaml"},
		{repo: "https://example.com", branch: "main", path: "-bad-path"},
	}
	for _, tc := range cases {
		_, err := NewGitSource(tc.repo, tc.branch, tc.path, slog.Default())
		if err == nil {
			t.Errorf("expected error for dash-prefixed param: repo=%q branch=%q path=%q", tc.repo, tc.branch, tc.path)
		}
	}
}

func TestGitSource_Fetch_Clone(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}

	repoURL := initBareRepo(t, validManifestYAML)

	gs, err := NewGitSource(repoURL, "main", "manifest.yaml", slog.Default())
	if err != nil {
		t.Fatalf("NewGitSource: %v", err)
	}
	defer gs.Cleanup()

	m, hash, err := gs.Fetch()
	if err != nil {
		t.Fatalf("Fetch (clone): %v", err)
	}
	if m == nil {
		t.Fatal("expected manifest, got nil")
	}
	if len(hash) != 40 {
		t.Errorf("expected 40-char git SHA1, got %d chars: %q", len(hash), hash)
	}
	if gs.dir == "" {
		t.Error("expected dir to be set after clone")
	}
}

func TestGitSource_Fetch_HashChangesOnNewCommit(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}

	repoURL := initBareRepo(t, validManifestYAML)

	gs, err := NewGitSource(repoURL, "main", "manifest.yaml", slog.Default())
	if err != nil {
		t.Fatalf("NewGitSource: %v", err)
	}
	defer gs.Cleanup()

	_, hash1, err := gs.Fetch()
	if err != nil {
		t.Fatalf("first Fetch: %v", err)
	}

	// Push a new commit
	addCommitToRepo(t, repoURL, validManifestYAML+"  # changed\n")

	_, hash2, err := gs.Fetch()
	if err != nil {
		t.Fatalf("second Fetch: %v", err)
	}

	if hash1 == hash2 {
		t.Error("expected hash to change after new commit")
	}
}

func TestGitSource_Cleanup_RemovesDir(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}

	repoURL := initBareRepo(t, validManifestYAML)

	gs, err := NewGitSource(repoURL, "main", "manifest.yaml", slog.Default())
	if err != nil {
		t.Fatalf("NewGitSource: %v", err)
	}

	if _, _, err := gs.Fetch(); err != nil {
		t.Fatalf("Fetch: %v", err)
	}

	cloneDir := gs.dir
	if cloneDir == "" {
		t.Fatal("dir should be set after fetch")
	}

	gs.Cleanup()

	if _, err := os.Stat(cloneDir); !os.IsNotExist(err) {
		t.Error("expected clone dir to be removed after Cleanup")
	}
}

func TestGitSource_Cleanup_BeforeClone_NoOp(t *testing.T) {
	gs, err := NewGitSource("https://example.com/repo.git", "main", "manifest.yaml", slog.Default())
	if err != nil {
		t.Fatal(err)
	}
	// Should not panic or error
	gs.Cleanup()
}

func TestGitSource_Fetch_InvalidRepo(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}

	gs, err := NewGitSource("file:///nonexistent/repo.git", "main", "manifest.yaml", slog.Default())
	if err != nil {
		t.Fatalf("NewGitSource: %v", err)
	}
	defer gs.Cleanup()

	_, _, err = gs.Fetch()
	if err == nil {
		t.Error("expected error for non-existent repo")
	}
	if !strings.Contains(err.Error(), "git clone") {
		t.Errorf("expected clone error, got: %v", err)
	}
}

func TestGitSource_Fetch_SubdirectoryPath(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}

	// Repo with manifest at config/manifest.yaml
	work := t.TempDir()
	bare := t.TempDir()

	setup := [][]string{
		{"git", "-C", work, "init", "-b", "main"},
		{"git", "-C", work, "config", "user.email", "test@test.com"},
		{"git", "-C", work, "config", "user.name", "Test"},
	}
	for _, args := range setup {
		if out, err := exec.Command(args[0], args[1:]...).CombinedOutput(); err != nil {
			t.Fatalf("%v: %s", args, out)
		}
	}

	subdir := filepath.Join(work, "config")
	if err := os.Mkdir(subdir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(subdir, "manifest.yaml"), []byte(validManifestYAML), 0600); err != nil {
		t.Fatal(err)
	}

	commit := [][]string{
		{"git", "-C", work, "add", "."},
		{"git", "-C", work, "commit", "-m", "add config"},
	}
	for _, args := range commit {
		if out, err := exec.Command(args[0], args[1:]...).CombinedOutput(); err != nil {
			t.Fatalf("%v: %s", args, out)
		}
	}
	if out, err := exec.Command("git", "clone", "--bare", work, bare).CombinedOutput(); err != nil {
		t.Fatalf("git clone --bare: %s", out)
	}

	gs, err := NewGitSource("file://"+bare, "main", "config/manifest.yaml", slog.Default())
	if err != nil {
		t.Fatalf("NewGitSource: %v", err)
	}
	defer gs.Cleanup()

	m, hash, err := gs.Fetch()
	if err != nil {
		t.Fatalf("Fetch with subdir path: %v", err)
	}
	if m == nil {
		t.Fatal("expected manifest")
	}
	if hash == "" {
		t.Error("expected non-empty hash")
	}
}
