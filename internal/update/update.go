// Package update checks for newer gcplane releases via GitHub API.
// Caches result for 24h to avoid hitting API on every run.
package update

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

const (
	repo     = "dataplanelabs/gcplane"
	checkTTL = 24 * time.Hour
)

// ReleaseInfo from GitHub releases API.
type ReleaseInfo struct {
	Version string `json:"tag_name"`
	URL     string `json:"html_url"`
}

type state struct {
	CheckedAt time.Time   `json:"checked_at"`
	Release   ReleaseInfo `json:"release"`
}

// ShouldCheck returns true if update check is appropriate.
func ShouldCheck() bool {
	if os.Getenv("GCPLANE_NO_UPDATE_NOTIFIER") != "" {
		return false
	}
	if os.Getenv("CI") != "" {
		return false
	}
	fi, _ := os.Stdout.Stat()
	return fi != nil && fi.Mode()&os.ModeCharDevice != 0
}

// Check queries GitHub for the latest release. Returns non-nil ReleaseInfo
// only if a newer version exists. Silently returns nil on any error.
func Check(ctx context.Context, currentVersion string) *ReleaseInfo {
	stateFile := stateFilePath()

	// Read cached state
	if s, err := readState(stateFile); err == nil {
		if time.Since(s.CheckedAt) < checkTTL {
			if versionGreaterThan(s.Release.Version, currentVersion) {
				return &s.Release
			}
			return nil
		}
	}

	// Fetch latest release
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	rel, err := fetchLatest(ctx)
	if err != nil {
		return nil
	}

	// Cache result
	_ = writeState(stateFile, state{CheckedAt: time.Now(), Release: *rel})

	if versionGreaterThan(rel.Version, currentVersion) {
		return rel
	}
	return nil
}

func fetchLatest(ctx context.Context) (*ReleaseInfo, error) {
	url := fmt.Sprintf("https://api.github.com/repos/%s/releases/latest", repo)
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _, _ = io.Copy(io.Discard, resp.Body); resp.Body.Close() }()

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("HTTP %d", resp.StatusCode)
	}
	var rel ReleaseInfo
	if err := json.NewDecoder(resp.Body).Decode(&rel); err != nil {
		return nil, err
	}
	return &rel, nil
}

func stateFilePath() string {
	dir, err := os.UserConfigDir()
	if err != nil {
		dir = os.TempDir()
	}
	return filepath.Join(dir, "gcplane", "state.json")
}

func readState(path string) (*state, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var s state
	if err := json.Unmarshal(data, &s); err != nil {
		return nil, err
	}
	return &s, nil
}

func writeState(path string, s state) error {
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	data, err := json.Marshal(s)
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0600)
}

// versionGreaterThan compares semver strings (v1.2.3 format).
func versionGreaterThan(latest, current string) bool {
	parse := func(v string) (int, int, int) {
		v = strings.TrimPrefix(v, "v")
		parts := strings.SplitN(v, ".", 3)
		if len(parts) < 2 {
			return 0, 0, 0
		}
		major, _ := strconv.Atoi(parts[0])
		minor, _ := strconv.Atoi(parts[1])
		patch := 0
		if len(parts) > 2 {
			p := strings.SplitN(parts[2], "-", 2)
			patch, _ = strconv.Atoi(p[0])
		}
		return major, minor, patch
	}
	lM, lm, lp := parse(latest)
	cM, cm, cp := parse(current)
	if lM != cM {
		return lM > cM
	}
	if lm != cm {
		return lm > cm
	}
	return lp > cp
}
