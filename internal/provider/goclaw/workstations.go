package goclaw

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
)

// workstationInternalFields are WS RPC response fields (camelCase) not present
// in manifests; stripped to prevent phantom diffs.
var workstationInternalFields = []string{
	"id", "tenantId", "createdAt", "updatedAt", "createdBy", "metadataSummary",
}

// observeWorkstation fetches a workstation by workstationKey via WS RPC.
// workstations.list returns SanitizedView — metadata (host/port/user/privateKey)
// is never returned by the API, so those fields are write-only in field_config.go.
func (p *Provider) observeWorkstation(ctx context.Context, key string) (map[string]any, error) {
	if err := p.ensureWS(ctx); err != nil {
		return nil, fmt.Errorf("ws connect for workstations: %w", err)
	}

	payload, err := p.ws.Call(ctx, "workstations.list", nil)
	if err != nil {
		return nil, fmt.Errorf("workstations.list: %w", err)
	}

	var resp struct {
		Workstations []map[string]any `json:"workstations"`
	}
	if err := json.Unmarshal(payload, &resp); err != nil {
		return nil, fmt.Errorf("parse workstations.list response: %w", err)
	}

	for _, ws := range resp.Workstations {
		// workstations.list returns camelCase (WS RPC convention).
		if strVal(ws, "workstationKey") != key {
			continue
		}
		result := copyMap(ws)
		for _, f := range workstationInternalFields {
			delete(result, f)
		}
		return result, nil
	}
	return nil, nil
}

// resolveWorkstationID returns the UUID for a workstation by key.
func (p *Provider) resolveWorkstationID(ctx context.Context, key string) (string, error) {
	if err := p.ensureWS(ctx); err != nil {
		return "", fmt.Errorf("ws connect for workstations: %w", err)
	}

	payload, err := p.ws.Call(ctx, "workstations.list", nil)
	if err != nil {
		return "", fmt.Errorf("workstations.list: %w", err)
	}

	var resp struct {
		Workstations []map[string]any `json:"workstations"`
	}
	if err := json.Unmarshal(payload, &resp); err != nil {
		return "", fmt.Errorf("parse workstations.list response: %w", err)
	}

	for _, ws := range resp.Workstations {
		if strVal(ws, "workstationKey") == key {
			if id := strVal(ws, "id"); id != "" {
				return id, nil
			}
		}
	}
	return "", fmt.Errorf("workstation %q not found", key)
}

// createWorkstation creates a workstation via WS RPC then reconciles its
// allowlist and agent links.
//
// Spec layout:
//
//	displayName, backendType, defaultCwd — top-level columns.
//	host/port/user/privateKey/knownHostsFingerprint — folded into metadata{} object.
//	allowlist: []string  → workstations.permissions.add per entry.
//	agents: []string     → agentKey → UUID → workstations.linkAgent per entry.
func (p *Provider) createWorkstation(ctx context.Context, key string, spec map[string]any) error {
	if err := p.ensureWS(ctx); err != nil {
		return fmt.Errorf("ws connect for workstations: %w", err)
	}

	params := buildWorkstationCreateParams(key, spec)
	if _, err := p.ws.Call(ctx, "workstations.create", params); err != nil {
		return fmt.Errorf("workstations.create %s: %w", key, err)
	}

	wsID, err := p.resolveWorkstationID(ctx, key)
	if err != nil {
		return fmt.Errorf("workstation %s: resolve after create: %w", key, err)
	}

	if err := p.reconcileAllowlist(ctx, wsID, nil, extractStringSlice(spec, "allowlist")); err != nil {
		return fmt.Errorf("workstation %s: reconcile allowlist: %w", key, err)
	}
	if err := p.reconcileWorkstationAgentLinks(ctx, wsID, nil, extractStringSlice(spec, "agents")); err != nil {
		return fmt.Errorf("workstation %s: reconcile agent links: %w", key, err)
	}

	return nil
}

// updateWorkstation patches a workstation and reconciles its allowlist and agent links.
func (p *Provider) updateWorkstation(ctx context.Context, key string, spec map[string]any) error {
	if err := p.ensureWS(ctx); err != nil {
		return fmt.Errorf("ws connect for workstations: %w", err)
	}

	wsID, err := p.resolveWorkstationID(ctx, key)
	if err != nil {
		return fmt.Errorf("workstation %s not found for update: %w", key, err)
	}

	patch := buildWorkstationUpdatePatch(spec)
	if len(patch) > 0 {
		if _, err := p.ws.Call(ctx, "workstations.update", map[string]any{
			"id":      wsID,
			"updates": patch,
		}); err != nil {
			return fmt.Errorf("workstations.update %s: %w", key, err)
		}
	}

	currentPerms, err := p.listWorkstationPermissions(ctx, wsID)
	if err != nil {
		return fmt.Errorf("workstation %s: list permissions: %w", key, err)
	}
	if err := p.reconcileAllowlist(ctx, wsID, currentPerms, extractStringSlice(spec, "allowlist")); err != nil {
		return fmt.Errorf("workstation %s: reconcile allowlist: %w", key, err)
	}

	// GoClaw's SanitizedView does not expose linked agents, so we cannot fetch
	// the current link set; pass nil → reconcileWorkstationAgentLinks skips unlink.
	// This means update only adds missing links, never removes surplus ones.
	// Full bidirectional sync requires a workstations.links.list RPC (not yet in goclaw).
	if err := p.reconcileWorkstationAgentLinks(ctx, wsID, nil, extractStringSlice(spec, "agents")); err != nil {
		return fmt.Errorf("workstation %s: reconcile agent links: %w", key, err)
	}

	return nil
}

// deleteWorkstation removes a workstation by key via WS RPC. Idempotent.
func (p *Provider) deleteWorkstation(ctx context.Context, key string) error {
	if err := p.ensureWS(ctx); err != nil {
		return fmt.Errorf("ws connect for workstations: %w", err)
	}

	wsID, err := p.resolveWorkstationID(ctx, key)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			return nil
		}
		return fmt.Errorf("workstation %s: resolve for delete: %w", key, err)
	}

	if _, err := p.ws.Call(ctx, "workstations.delete", map[string]any{"id": wsID}); err != nil {
		return fmt.Errorf("workstations.delete %s: %w", key, err)
	}
	return nil
}

// --- allowlist reconciliation ---

type permEntry struct {
	id      string
	pattern string
}

func (p *Provider) listWorkstationPermissions(ctx context.Context, wsID string) ([]permEntry, error) {
	payload, err := p.ws.Call(ctx, "workstations.permissions.list", map[string]any{
		"workstationId": wsID,
	})
	if err != nil {
		return nil, fmt.Errorf("workstations.permissions.list: %w", err)
	}
	var resp struct {
		Permissions []struct {
			ID      string `json:"id"`
			Pattern string `json:"pattern"`
		} `json:"permissions"`
	}
	if err := json.Unmarshal(payload, &resp); err != nil {
		return nil, fmt.Errorf("parse permissions.list response: %w", err)
	}
	entries := make([]permEntry, len(resp.Permissions))
	for i, pe := range resp.Permissions {
		entries[i] = permEntry{id: pe.ID, pattern: pe.Pattern}
	}
	return entries, nil
}

// reconcileAllowlist adds/removes permission patterns to match desired.
// current == nil means we have no current state (create path); skip removes.
func (p *Provider) reconcileAllowlist(ctx context.Context, wsID string, current []permEntry, desired []string) error {
	currentSet := make(map[string]string, len(current)) // pattern → id
	for _, e := range current {
		currentSet[e.pattern] = e.id
	}
	desiredSet := make(map[string]bool, len(desired))
	for _, d := range desired {
		desiredSet[d] = true
	}

	for _, d := range desired {
		if _, exists := currentSet[d]; !exists {
			if _, err := p.ws.Call(ctx, "workstations.permissions.add", map[string]any{
				"workstationId": wsID,
				"pattern":       d,
			}); err != nil {
				return fmt.Errorf("add permission %q: %w", d, err)
			}
		}
	}

	if current == nil {
		return nil
	}
	for pattern, id := range currentSet {
		if !desiredSet[pattern] {
			if _, err := p.ws.Call(ctx, "workstations.permissions.remove", map[string]any{
				"id": id,
			}); err != nil {
				return fmt.Errorf("remove permission %q: %w", pattern, err)
			}
		}
	}
	return nil
}

// --- agent link reconciliation ---

// reconcileWorkstationAgentLinks links agents by key to the workstation.
// current == nil → only adds (no unlinks); non-nil → full add+remove diff.
func (p *Provider) reconcileWorkstationAgentLinks(ctx context.Context, wsID string, current []string, desired []string) error {
	currentSet := make(map[string]bool, len(current))
	for _, k := range current {
		currentSet[k] = true
	}
	desiredSet := make(map[string]bool, len(desired))
	for _, k := range desired {
		desiredSet[k] = true
	}

	for _, agentKey := range desired {
		if currentSet[agentKey] {
			continue
		}
		agentID, err := p.resolveAgentID(ctx, agentKey)
		if err != nil {
			return fmt.Errorf("resolve agent %q for link: %w", agentKey, err)
		}
		if _, err := p.ws.Call(ctx, "workstations.linkAgent", map[string]any{
			"workstationId": wsID,
			"agentId":       agentID,
			"isDefault":     false,
		}); err != nil {
			return fmt.Errorf("linkAgent %s → %s: %w", agentKey, wsID, err)
		}
	}

	if current == nil {
		return nil
	}
	for _, agentKey := range current {
		if desiredSet[agentKey] {
			continue
		}
		agentID, err := p.resolveAgentID(ctx, agentKey)
		if err != nil {
			return fmt.Errorf("resolve agent %q for unlink: %w", agentKey, err)
		}
		if _, err := p.ws.Call(ctx, "workstations.unlinkAgent", map[string]any{
			"workstationId": wsID,
			"agentId":       agentID,
		}); err != nil {
			return fmt.Errorf("unlinkAgent %s → %s: %w", agentKey, wsID, err)
		}
	}
	return nil
}

// --- spec translation helpers ---

// buildWorkstationCreateParams maps a manifest spec to workstations.create RPC params.
// SSH connection fields (host/port/user/privateKey/knownHostsFingerprint) are lifted
// from spec top-level into the nested metadata object the RPC expects.
func buildWorkstationCreateParams(key string, spec map[string]any) map[string]any {
	params := map[string]any{
		"workstationKey": key,
		"createdBy":      "gcplane",
	}
	if v, ok := spec["displayName"]; ok {
		params["name"] = v
	}
	if v, ok := spec["backendType"]; ok {
		params["backendType"] = v
	}
	if v, ok := spec["defaultCwd"]; ok {
		params["defaultCwd"] = v
	}

	meta := map[string]any{}
	for _, f := range []string{"host", "port", "user", "privateKey", "knownHostsFingerprint", "connectTimeoutSec"} {
		if v, ok := spec[f]; ok {
			meta[f] = v
		}
	}
	if len(meta) > 0 {
		params["metadata"] = meta
	}
	return params
}

// buildWorkstationUpdatePatch builds the workstations.update `updates` map.
// Strips manifest-only fields (allowlist/agents/SSH secrets).
func buildWorkstationUpdatePatch(spec map[string]any) map[string]any {
	// Manifest keys explicitly handled or intentionally excluded.
	skipKeys := map[string]bool{
		"allowlist": true, "agents": true,
		// Renamed: displayName → name (handled above)
		"displayName": true,
		// Scalar top-level fields already set above
		"backendType": true, "defaultCwd": true,
	}
	sshMetaKeys := map[string]bool{
		"host": true, "port": true, "user": true,
		"privateKey": true, "knownHostsFingerprint": true, "connectTimeoutSec": true,
	}
	patch := map[string]any{}

	if v, ok := spec["displayName"]; ok {
		patch["name"] = v
	}
	if v, ok := spec["backendType"]; ok {
		patch["backendType"] = v
	}
	if v, ok := spec["defaultCwd"]; ok {
		patch["defaultCwd"] = v
	}

	meta := map[string]any{}
	for f := range sshMetaKeys {
		if v, ok := spec[f]; ok {
			meta[f] = v
		}
	}
	if len(meta) > 0 {
		patch["metadata"] = meta
	}

	for k, v := range spec {
		if skipKeys[k] || sshMetaKeys[k] {
			continue
		}
		if _, handled := patch[k]; handled {
			continue
		}
		patch[k] = v
	}
	return patch
}

// extractStringSlice safely reads a []string from a map[string]any spec field.
func extractStringSlice(spec map[string]any, key string) []string {
	raw, ok := spec[key]
	if !ok {
		return nil
	}
	list, ok := raw.([]any)
	if !ok {
		return nil
	}
	out := make([]string, 0, len(list))
	for _, v := range list {
		if s, ok := v.(string); ok {
			out = append(out, s)
		}
	}
	return out
}
