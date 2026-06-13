package goclaw

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"strings"
	"time"
)

// maxContextFileSize caps a single decompressed context-file tar entry (gzip-bomb guard).
const maxContextFileSize = 8 << 20

// syncContextFiles upserts agent context files via the merge import API.
// Builds a tar.gz archive with context_files/{name} entries and POSTs it
// to /v1/agents/{id}/import?include=context_files as multipart form data.
// files is expected to be []any where each element is map[string]any{name, content}.
func (p *Provider) syncContextFiles(ctx context.Context, agentID string, files []any) error {
	if len(files) == 0 {
		return nil
	}

	archive, err := buildContextFilesArchive(files)
	if err != nil {
		return fmt.Errorf("build context files archive: %w", err)
	}

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	part, err := writer.CreateFormFile("file", "context-files.tar.gz")
	if err != nil {
		return fmt.Errorf("create form file: %w", err)
	}
	if _, err := io.Copy(part, archive); err != nil {
		return fmt.Errorf("write archive to form: %w", err)
	}
	writer.Close()

	url := p.endpoint + "/v1/agents/" + agentID + "/import?include=context_files"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, body)
	if err != nil {
		return fmt.Errorf("create import request: %w", err)
	}
	req.Header.Set("Content-Type", writer.FormDataContentType())
	req.Header.Set("Authorization", "Bearer "+p.token)
	for k, v := range p.http.headers {
		req.Header.Set(k, v)
	}

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("import context files: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		return fmt.Errorf("import context files: status %d: %s", resp.StatusCode, string(respBody))
	}

	p.logger.Info("synced context files",
		"agent_id", agentID,
		"files", len(files))

	return nil
}

// DownloadAgentContextFiles fetches context files for an agent from goclaw.
// Calls GET /v1/agents/{id}/export?sections=context_files&stream=false
// and untars the returned gzip archive into [{name, content}] pairs.
func (p *Provider) DownloadAgentContextFiles(ctx context.Context, agentKey string) ([]map[string]string, error) {
	id, err := p.resolveAgentID(ctx, agentKey)
	if err != nil {
		return nil, err
	}

	gz, err := p.http.GetRaw(ctx, "/v1/agents/"+id+"/export?sections=context_files&stream=false")
	if err != nil {
		return nil, fmt.Errorf("export context files for agent %s: %w", agentKey, err)
	}

	return parseContextFilesArchive(gz)
}

// parseContextFilesArchive untars a gzip archive and returns entries under
// the "context_files/" prefix as [{name, content}] pairs.
func parseContextFilesArchive(gz []byte) ([]map[string]string, error) {
	gr, err := gzip.NewReader(bytes.NewReader(gz))
	if err != nil {
		return nil, fmt.Errorf("gzip open: %w", err)
	}
	defer gr.Close()

	tr := tar.NewReader(gr)
	const prefix = "context_files/"
	var out []map[string]string
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("tar read: %w", err)
		}
		if hdr.FileInfo().IsDir() {
			continue
		}
		name := hdr.Name
		if !strings.HasPrefix(name, prefix) {
			continue
		}
		name = strings.TrimPrefix(name, prefix)
		if name == "" {
			continue
		}
		data, err := io.ReadAll(io.LimitReader(tr, maxContextFileSize+1))
		if err != nil {
			return nil, fmt.Errorf("read tar entry %s: %w", hdr.Name, err)
		}
		if int64(len(data)) > maxContextFileSize {
			return nil, fmt.Errorf("context file %s exceeds %d bytes", name, maxContextFileSize)
		}
		out = append(out, map[string]string{"name": name, "content": string(data)})
	}
	return out, nil
}

// buildContextFilesArchive creates a tar.gz archive with context_files/{name} entries.
func buildContextFilesArchive(files []any) (*bytes.Buffer, error) {
	buf := &bytes.Buffer{}
	gw := gzip.NewWriter(buf)
	tw := tar.NewWriter(gw)

	for _, f := range files {
		fm, ok := f.(map[string]any)
		if !ok {
			continue
		}
		name, _ := fm["name"].(string)
		content, _ := fm["content"].(string)
		if name == "" {
			continue
		}

		header := &tar.Header{
			Name:    "context_files/" + name,
			Size:    int64(len(content)),
			Mode:    0644,
			ModTime: time.Now(),
		}
		if err := tw.WriteHeader(header); err != nil {
			return nil, fmt.Errorf("write tar header for %s: %w", name, err)
		}
		if _, err := tw.Write([]byte(content)); err != nil {
			return nil, fmt.Errorf("write tar content for %s: %w", name, err)
		}
	}

	if err := tw.Close(); err != nil {
		return nil, fmt.Errorf("close tar: %w", err)
	}
	if err := gw.Close(); err != nil {
		return nil, fmt.Errorf("close gzip: %w", err)
	}

	return buf, nil
}
