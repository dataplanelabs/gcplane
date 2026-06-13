package goclaw

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"testing"
	"time"
)

// buildTestArchive creates a tar.gz with context_files/{name} entries for testing.
func buildTestArchive(t *testing.T, entries map[string]string) []byte {
	t.Helper()
	var buf bytes.Buffer
	gw := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gw)
	for name, content := range entries {
		hdr := &tar.Header{
			Name:    "context_files/" + name,
			Size:    int64(len(content)),
			Mode:    0644,
			ModTime: time.Now(),
		}
		if err := tw.WriteHeader(hdr); err != nil {
			t.Fatalf("tar header: %v", err)
		}
		if _, err := io.WriteString(tw, content); err != nil {
			t.Fatalf("tar write: %v", err)
		}
	}
	if err := tw.Close(); err != nil {
		t.Fatalf("tar close: %v", err)
	}
	if err := gw.Close(); err != nil {
		t.Fatalf("gzip close: %v", err)
	}
	return buf.Bytes()
}

func TestDownloadAgentContextFiles_HappyPath(t *testing.T) {
	archive := buildTestArchive(t, map[string]string{
		"IDENTITY.md": "# Agent\nName: Bot",
		"SOUL.md":     "## Personality",
	})

	p, cleanup := newTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/v1/agents":
			json.NewEncoder(w).Encode(map[string]any{
				"agents": []map[string]any{
					{"id": "agent-uuid", "agent_key": "my-bot"},
				},
			})
		case r.Method == http.MethodGet && r.URL.Path == "/v1/agents/agent-uuid/export":
			w.Header().Set("Content-Type", "application/gzip")
			w.Write(archive)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer cleanup()

	files, err := p.DownloadAgentContextFiles(context.Background(), "my-bot")
	if err != nil {
		t.Fatalf("DownloadAgentContextFiles: %v", err)
	}
	if len(files) != 2 {
		t.Fatalf("expected 2 files, got %d", len(files))
	}

	byName := make(map[string]string)
	for _, f := range files {
		byName[f["name"]] = f["content"]
	}
	if byName["IDENTITY.md"] != "# Agent\nName: Bot" {
		t.Errorf("IDENTITY.md mismatch: %q", byName["IDENTITY.md"])
	}
	if byName["SOUL.md"] != "## Personality" {
		t.Errorf("SOUL.md mismatch: %q", byName["SOUL.md"])
	}
}

func TestDownloadAgentContextFiles_AgentNotFound(t *testing.T) {
	p, cleanup := newTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{"agents": []map[string]any{}})
	}))
	defer cleanup()

	_, err := p.DownloadAgentContextFiles(context.Background(), "nonexistent")
	if err == nil {
		t.Fatal("expected error for missing agent")
	}
}

func TestParseContextFilesArchive_SkipsNonPrefixed(t *testing.T) {
	var buf bytes.Buffer
	gw := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gw)

	for _, entry := range []struct{ name, content string }{
		{"context_files/IDENTITY.md", "# Hi"},
		{"other/file.txt", "should be skipped"},
	} {
		hdr := &tar.Header{Name: entry.name, Size: int64(len(entry.content)), Mode: 0644, ModTime: time.Now()}
		tw.WriteHeader(hdr)               //nolint:errcheck
		io.WriteString(tw, entry.content) //nolint:errcheck
	}
	tw.Close() //nolint:errcheck
	gw.Close() //nolint:errcheck

	out, err := parseContextFilesArchive(buf.Bytes())
	if err != nil {
		t.Fatalf("parseContextFilesArchive: %v", err)
	}
	if len(out) != 1 {
		t.Fatalf("expected 1 entry, got %d: %v", len(out), out)
	}
	if out[0]["name"] != "IDENTITY.md" {
		t.Errorf("expected IDENTITY.md, got %q", out[0]["name"])
	}
}

func TestParseContextFilesArchive_EmptyArchive(t *testing.T) {
	var buf bytes.Buffer
	gw := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gw)
	tw.Close() //nolint:errcheck
	gw.Close() //nolint:errcheck

	out, err := parseContextFilesArchive(buf.Bytes())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(out) != 0 {
		t.Errorf("expected 0 entries for empty archive, got %d", len(out))
	}
}

func TestParseContextFilesArchive_RejectsOversizeEntry(t *testing.T) {
	archive := buildTestArchive(t, map[string]string{
		"huge.md": string(bytes.Repeat([]byte("x"), maxContextFileSize+1)),
	})
	if _, err := parseContextFilesArchive(archive); err == nil {
		t.Fatal("expected error for oversize context file, got nil")
	}
}
