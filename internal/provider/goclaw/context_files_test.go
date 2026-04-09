package goclaw

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestBuildContextFilesArchive(t *testing.T) {
	files := []any{
		map[string]any{"name": "IDENTITY.md", "content": "# Agent Name"},
		map[string]any{"name": "SOUL.md", "content": "## Personality\n- Helpful"},
	}

	buf, err := buildContextFilesArchive(files)
	if err != nil {
		t.Fatalf("buildContextFilesArchive: %v", err)
	}

	// Decompress and verify tar entries
	gr, err := gzip.NewReader(buf)
	if err != nil {
		t.Fatalf("gzip reader: %v", err)
	}
	defer gr.Close()

	tr := tar.NewReader(gr)
	found := make(map[string]string)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatalf("tar next: %v", err)
		}
		content, _ := io.ReadAll(tr)
		found[hdr.Name] = string(content)
	}

	if len(found) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(found))
	}
	if found["context_files/IDENTITY.md"] != "# Agent Name" {
		t.Errorf("IDENTITY.md content = %q", found["context_files/IDENTITY.md"])
	}
	if found["context_files/SOUL.md"] != "## Personality\n- Helpful" {
		t.Errorf("SOUL.md content = %q", found["context_files/SOUL.md"])
	}
}

func TestBuildContextFilesArchive_SkipsInvalid(t *testing.T) {
	files := []any{
		"not a map",                                 // skip
		map[string]any{"name": "", "content": "x"},  // skip: empty name
		map[string]any{"name": "OK.md", "content": "valid"},
	}

	buf, err := buildContextFilesArchive(files)
	if err != nil {
		t.Fatalf("buildContextFilesArchive: %v", err)
	}

	gr, _ := gzip.NewReader(buf)
	defer gr.Close()
	tr := tar.NewReader(gr)
	count := 0
	for {
		_, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatalf("tar next: %v", err)
		}
		count++
	}
	if count != 1 {
		t.Errorf("expected 1 valid entry, got %d", count)
	}
}

func TestBuildContextFilesArchive_Empty(t *testing.T) {
	buf, err := buildContextFilesArchive(nil)
	if err != nil {
		t.Fatalf("expected no error for nil files, got: %v", err)
	}
	if buf.Len() == 0 {
		t.Error("expected non-zero buffer even for empty archive")
	}
}

func TestSyncContextFiles_PostsToImportEndpoint(t *testing.T) {
	var receivedPath string
	var receivedContentType string

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/v1/agents":
			json.NewEncoder(w).Encode(map[string]any{
				"agents": []map[string]any{
					{"id": "uuid-123", "agent_key": "test-agent", "agent_type": "predefined"},
				},
			})
		case r.Method == http.MethodPost && r.URL.Path == "/v1/agents/uuid-123/import":
			receivedPath = r.URL.String()
			receivedContentType = r.Header.Get("Content-Type")
			// Verify it's multipart form data with a file
			if err := r.ParseMultipartForm(10 << 20); err != nil {
				t.Errorf("parse multipart: %v", err)
				w.WriteHeader(http.StatusBadRequest)
				return
			}
			f, _, err := r.FormFile("file")
			if err != nil {
				t.Errorf("form file: %v", err)
				w.WriteHeader(http.StatusBadRequest)
				return
			}
			defer f.Close()

			// Verify it's a valid gzip/tar
			data, _ := io.ReadAll(f)
			gr, err := gzip.NewReader(bytes.NewReader(data))
			if err != nil {
				t.Errorf("gzip read: %v", err)
				w.WriteHeader(http.StatusBadRequest)
				return
			}
			tr := tar.NewReader(gr)
			count := 0
			for {
				_, err := tr.Next()
				if err == io.EOF {
					break
				}
				if err != nil {
					t.Errorf("tar read: %v", err)
					break
				}
				count++
			}
			if count != 2 {
				t.Errorf("expected 2 files in archive, got %d", count)
			}

			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(map[string]any{
				"context_files": 2,
			})
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer srv.Close()

	p := New(srv.URL, "test-token")
	files := []any{
		map[string]any{"name": "IDENTITY.md", "content": "# Test"},
		map[string]any{"name": "SOUL.md", "content": "## Soul"},
	}

	err := p.syncContextFiles(context.Background(), "uuid-123", files)
	if err != nil {
		t.Fatalf("syncContextFiles: %v", err)
	}

	if receivedPath != "/v1/agents/uuid-123/import?include=context_files" {
		t.Errorf("expected import path with include=context_files, got %s", receivedPath)
	}
	if receivedContentType == "" {
		t.Error("expected Content-Type header to be set")
	}
}

func TestSyncContextFiles_NoFiles_Noop(t *testing.T) {
	called := false
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	p := New(srv.URL, "test-token")
	err := p.syncContextFiles(context.Background(), "uuid-123", nil)
	if err != nil {
		t.Fatalf("expected nil error, got: %v", err)
	}
	if called {
		t.Error("expected no HTTP call for nil files")
	}
}
