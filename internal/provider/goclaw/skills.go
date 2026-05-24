package goclaw

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"mime/multipart"
	"path/filepath"
	"strings"

	"github.com/dataplanelabs/gcplane/internal/manifest"
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
//
// Also folds per-agent grants into spec["grants"]["agents"] so the reconciler
// can detect grant drift the same way it does for MCPServer.
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
			skillID := strVal(s, "id")
			result := translateResult(stripInternal(s))
			if skillID != "" {
				if agentKeys, err := p.listSkillGrantAgentKeys(ctx, skillID); err == nil {
					result["grants"] = map[string]any{"agents": agentKeys}
				}
			}
			return result, nil
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

	// requires.cli cross-check: fail apply pre-upload if the skill needs a
	// CLI version that's not installed on the server. Network/lookup
	// failures degrade to a warning — the rest of apply proceeds.
	if installed, err := p.GetCLIVersions(ctx); err == nil {
		warnings, mismatch := manifest.CrossCheckRequiresCli(spec, installed)
		for _, w := range warnings {
			slog.Warn("skill.requires_cli_warning", "skill", key, "warning", w)
		}
		if mismatch != nil {
			return fmt.Errorf("skill %s: %w", key, mismatch)
		}
	} else {
		slog.Warn("skill.requires_cli_lookup_failed", "skill", key, "error", err)
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
	// source=gcplane lets goclaw refuse non-gcplane overwrites unless force_imperative.
	if err := mw.WriteField("source", "gcplane"); err != nil {
		return fmt.Errorf("multipart write source: %w", err)
	}
	// is_system=true for skills under _system/ — closes the B1 manual DB
	// backfill (issue dataplanelabs/gcplane#14). Detection: sourceDir path
	// contains a /_system/ segment OR starts with _system/. Other tenants'
	// skills land with is_system=false (the goclaw upload default).
	if isSystemSourceDir(sourceDir) {
		if err := mw.WriteField("is_system", "true"); err != nil {
			return fmt.Errorf("multipart write is_system: %w", err)
		}
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
		if err := p.putSkillFields(ctx, key, spec); err != nil {
			return fmt.Errorf("apply overlay to skill %s after upload: %w", key, err)
		}
	}
	// Reconcile per-agent grants when the manifest declares any. Mirrors
	// createMCPServer: on create we only add (no implicit revoke), since
	// there can be no pre-existing state for a brand-new skill. The
	// update path below handles declarative revokes.
	if agents := extractGrantAgents(spec); len(agents) > 0 {
		if err := p.applySkillGrants(ctx, key, agents); err != nil {
			return fmt.Errorf("apply grants for skill %s: %w", key, err)
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
// Sends only writable fields, then reconciles per-agent grants.
func (p *Provider) updateSkill(ctx context.Context, key string, spec map[string]any) error {
	if err := p.putSkillFields(ctx, key, spec); err != nil {
		return err
	}
	return p.applySkillGrants(ctx, key, extractGrantAgents(spec))
}

// putSkillFields PUTs the writable subset of spec to /v1/skills/{id}.
// Observed-only fields (version, etc.) are allowed through but goclaw will
// accept or ignore them. Returns an error if the skill does not exist.
func (p *Provider) putSkillFields(ctx context.Context, key string, spec map[string]any) error {
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

// resolveSkillIDFromList returns the UUID of a skill by slug. Mirrors the
// MCPServer helper so applySkillGrants stays symmetric with applyMCPGrants.
func (p *Provider) resolveSkillIDFromList(ctx context.Context, slug string) (string, error) {
	data, err := p.http.Get(ctx, "/v1/skills")
	if err != nil {
		return "", fmt.Errorf("list skills: %w", err)
	}
	var resp struct {
		Skills []map[string]any `json:"skills"`
	}
	if err := json.Unmarshal(data, &resp); err != nil {
		return "", fmt.Errorf("parse skills response: %w", err)
	}
	for _, s := range resp.Skills {
		if strVal(s, "slug") == slug && p.matchesTenant(ctx, s) {
			if id := strVal(s, "id"); id != "" {
				return id, nil
			}
		}
	}
	return "", fmt.Errorf("skill %q not found", slug)
}

// applySkillGrants reconciles desired per-agent grants for a skill:
// adds missing grants and removes extra ones. Symmetric with applyMCPGrants.
//
// Passing a nil/empty desiredAgents revokes all current grants — same
// semantics as MCPServer so a manifest that drops a name actually unwires it.
func (p *Provider) applySkillGrants(ctx context.Context, slug string, desiredAgents []string) error {
	skillID, err := p.resolveSkillIDFromList(ctx, slug)
	if err != nil {
		return err
	}

	currentAgents, err := p.listSkillGrantAgentKeys(ctx, skillID)
	if err != nil {
		return fmt.Errorf("list current grants for skill %s: %w", slug, err)
	}

	currentIDs := make(map[string]string, len(currentAgents)) // agent_key → agent_id
	for _, agentKey := range currentAgents {
		agentID, err := p.resolveAgentID(ctx, agentKey)
		if err != nil {
			return fmt.Errorf("resolve current grant agent %q: %w", agentKey, err)
		}
		currentIDs[agentKey] = agentID
	}

	desiredIDs := make(map[string]string, len(desiredAgents))
	for _, agentKey := range desiredAgents {
		agentID, err := p.resolveAgentID(ctx, agentKey)
		if err != nil {
			return fmt.Errorf("resolve desired grant agent %q: %w", agentKey, err)
		}
		desiredIDs[agentKey] = agentID
	}

	// Add missing grants. version=0 lets goclaw default to 1 (handleGrantAgent).
	for agentKey, agentID := range desiredIDs {
		if _, exists := currentIDs[agentKey]; exists {
			continue
		}
		body := map[string]any{"agent_id": agentID}
		if _, err := p.http.Post(ctx, "/v1/skills/"+skillID+"/grants/agent", body); err != nil {
			return fmt.Errorf("grant skill %s to agent %s: %w", slug, agentKey, err)
		}
	}

	// Remove extra grants.
	for agentKey, agentID := range currentIDs {
		if _, wanted := desiredIDs[agentKey]; wanted {
			continue
		}
		path := "/v1/skills/" + skillID + "/grants/agent/" + agentID
		if err := p.http.Delete(ctx, path); err != nil && !errors.Is(err, ErrNotFound) {
			return fmt.Errorf("revoke skill %s from agent %s: %w", slug, agentKey, err)
		}
	}
	return nil
}

// listSkillGrantAgentKeys returns the agent_keys that currently have this
// skill granted. goclaw exposes grants per agent (GET /v1/agents/{id}/skills)
// but not per skill, so we fan out across the tenant's agents and filter for
// granted == true. Small tenants only; not optimised for hundreds of agents.
func (p *Provider) listSkillGrantAgentKeys(ctx context.Context, skillID string) ([]string, error) {
	agentData, err := p.http.Get(ctx, "/v1/agents")
	if err != nil {
		return nil, fmt.Errorf("list agents for skill-grant resolution: %w", err)
	}
	var agentResp struct {
		Agents []map[string]any `json:"agents"`
	}
	if err := json.Unmarshal(agentData, &agentResp); err != nil {
		return nil, fmt.Errorf("parse agents response: %w", err)
	}

	keys := make([]string, 0)
	for _, a := range agentResp.Agents {
		if !p.matchesTenant(ctx, a) {
			continue
		}
		agentID := strVal(a, "id")
		agentKey := strVal(a, "agent_key")
		if agentID == "" || agentKey == "" {
			continue
		}
		granted, err := p.agentHasSkillGrant(ctx, agentID, skillID)
		if err != nil {
			return nil, err
		}
		if granted {
			keys = append(keys, agentKey)
		}
	}
	return keys, nil
}

// agentHasSkillGrant returns true when GET /v1/agents/{agentID}/skills
// reports skillID with granted=true.
func (p *Provider) agentHasSkillGrant(ctx context.Context, agentID, skillID string) (bool, error) {
	data, err := p.http.Get(ctx, "/v1/agents/"+agentID+"/skills")
	if err != nil {
		return false, fmt.Errorf("list skills for agent %s: %w", agentID, err)
	}
	var resp struct {
		Skills []map[string]any `json:"skills"`
	}
	if err := json.Unmarshal(data, &resp); err != nil {
		return false, fmt.Errorf("parse agent skills response: %w", err)
	}
	for _, s := range resp.Skills {
		if strVal(s, "id") != skillID {
			continue
		}
		if g, ok := s["granted"].(bool); ok && g {
			return true, nil
		}
		return false, nil
	}
	return false, nil
}

// isSystemSourceDir reports whether the given filesystem path is under a
// `_system/` directory anywhere in its ancestry. Used by createSkill to mark
// uploads with is_system=true so goclaw exposes them cross-tenant without a
// manual DB UPDATE. See dataplanelabs/gcplane#14.
func isSystemSourceDir(sourceDir string) bool {
	clean := filepath.ToSlash(filepath.Clean(sourceDir))
	if clean == "_system" || strings.HasPrefix(clean, "_system/") {
		return true
	}
	return strings.Contains(clean, "/_system/")
}
