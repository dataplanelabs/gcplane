package goclaw

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"mime/multipart"

	"github.com/dataplanelabs/gcplane/internal/skillpkg"
)

// ErrSkillSourceMissing is returned when a KindSkill create has no
// sourceDir field — gcplane has nothing to upload.
var ErrSkillSourceMissing = errors.New("KindSkill spec must include sourceDir (path to a directory containing SKILL.md)")

// skillWritableFields lists the field names (camelCase) that gcplane sends
// to PUT /v1/skills/{id}. goclaw strips id/owner_id/file_path/is_system/enabled
// server-side; the toggle endpoint owns the `enabled` flip. Version, tags,
// visibility, status, description, and name are accepted by the update handler.
var skillWritableFields = map[string]struct{}{
	"description": {},
	"visibility":  {},
	"status":      {},
	"tags":        {},
	"version":     {},
	"name":        {},
}

// observeSkill fetches a skill by slug from GoClaw.
// goclaw's list response keys skills by `slug`; the older code matched on
// `key`, which never resolved against a real backend.
func (p *Provider) observeSkill(ctx context.Context, key string) (map[string]any, error) {
	data, err := p.http.Get(ctx, "/v1/skills")
	if err != nil {
		return nil, fmt.Errorf("list skills: %w", err)
	}

	var resp struct {
		Skills []map[string]any `json:"skills"`
	}
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, fmt.Errorf("parse skills response: %w", err)
	}

	for _, s := range resp.Skills {
		if matchSkillKey(s, key) && p.matchesTenant(ctx, s) {
			return translateResult(stripInternal(s)), nil
		}
	}
	return nil, nil
}

// matchSkillKey accepts a row keyed by either `slug` (canonical) or `key`
// (legacy test fixtures); real goclaw responses always carry `slug`.
func matchSkillKey(row map[string]any, key string) bool {
	if strVal(row, "slug") == key {
		return true
	}
	return strVal(row, "key") == key
}

// createSkill uploads a skill ZIP built from spec["sourceDir"] to
// POST /v1/skills/upload. goclaw parses SKILL.md frontmatter to derive
// slug, name, description — gcplane doesn't pass those as form fields.
// Idempotent server-side: re-uploading identical content returns
// status:"unchanged" without bumping the version.
func (p *Provider) createSkill(ctx context.Context, key string, spec map[string]any) error {
	sourceDir, _ := spec["sourceDir"].(string)
	if sourceDir == "" {
		return fmt.Errorf("skill %s: %w", key, ErrSkillSourceMissing)
	}

	pkg, err := skillpkg.PackDir(sourceDir)
	if err != nil {
		return fmt.Errorf("pack skill %s: %w", key, err)
	}

	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	fw, err := mw.CreateFormFile("file", key+".zip")
	if err != nil {
		return fmt.Errorf("multipart create: %w", err)
	}
	if _, err := fw.Write(pkg.ZIP); err != nil {
		return fmt.Errorf("multipart write: %w", err)
	}
	if err := mw.Close(); err != nil {
		return fmt.Errorf("multipart close: %w", err)
	}

	_, err = p.http.PostMultipart(ctx, "/v1/skills/upload", &buf, mw.FormDataContentType())
	if err != nil {
		return fmt.Errorf("upload skill %s: %w", key, err)
	}

	// goclaw extracts name/description/slug from SKILL.md frontmatter on
	// upload but hardcodes visibility="internal" and ignores tags/status
	// (handleUpload only reads the `file` form field). To honour the
	// manifest's overlay fields, follow up with a PUT that sends just the
	// writable subset. Skip the PUT when there are no overlay fields to
	// avoid a wasted round-trip.
	if hasOverlay(spec) {
		if err := p.updateSkill(ctx, key, spec); err != nil {
			return fmt.Errorf("apply overlay to skill %s after upload: %w", key, err)
		}
	}
	return nil
}

// hasOverlay returns true when spec carries any field that requires a
// post-upload PUT (visibility/tags/status/description override).
func hasOverlay(spec map[string]any) bool {
	for k := range skillWritableFields {
		if v, ok := spec[k]; ok && v != nil && v != "" {
			return true
		}
	}
	return false
}

// deleteSkill removes a skill via DELETE /v1/skills/{id}.
// Looks up the UUID via the existing list endpoint.
func (p *Provider) deleteSkill(ctx context.Context, key string) error {
	current, err := p.observeSkill(ctx, key)
	if err != nil {
		return err
	}
	if current == nil {
		return nil // idempotent — already absent
	}
	id, ok := current["id"].(string)
	if !ok || id == "" {
		return fmt.Errorf("skill %s: missing id in observed result", key)
	}
	return p.http.Delete(ctx, "/v1/skills/"+id)
}

// updateSkill updates an existing skill in GoClaw.
// Sends only writable fields; observed-only fields (version, etc.) are
// allowed through but goclaw will accept or ignore them.
func (p *Provider) updateSkill(ctx context.Context, key string, spec map[string]any) error {
	current, err := p.observeSkill(ctx, key)
	if err != nil {
		return err
	}
	if current == nil {
		return fmt.Errorf("skill %s not found for update", key)
	}

	id, ok := current["id"].(string)
	if !ok {
		return fmt.Errorf("skill %s: missing id", key)
	}

	writable := make(map[string]any, len(spec))
	for k, v := range spec {
		if _, ok := skillWritableFields[k]; ok {
			writable[k] = v
		}
	}
	body := translateSpec(writable)
	_, err = p.http.Put(ctx, "/v1/skills/"+id, body)
	return err
}
