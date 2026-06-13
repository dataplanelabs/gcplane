package goclaw

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/dataplanelabs/gcplane/internal/keyconv"
)

// translateSpec converts manifest camelCase keys to GoClaw snake_case for API calls.
func translateSpec(spec map[string]any) map[string]any {
	return keyconv.CamelToSnake(spec)
}

// translateResult converts GoClaw snake_case keys to manifest camelCase for comparison.
func translateResult(result map[string]any) map[string]any {
	return keyconv.SnakeToCamel(result)
}

// resolveAgentID looks up an agent by key and returns its UUID.
func (p *Provider) resolveAgentID(ctx context.Context, agentKey string) (string, error) {
	data, err := p.http.Get(ctx, "/v1/agents")
	if err != nil {
		return "", fmt.Errorf("list agents: %w", err)
	}

	var resp struct {
		Agents []map[string]any `json:"agents"`
	}
	if err := json.Unmarshal(data, &resp); err != nil {
		return "", fmt.Errorf("parse agents response: %w", err)
	}

	for _, a := range resp.Agents {
		if strVal(a, "agent_key") == agentKey {
			if id := strVal(a, "id"); id != "" {
				return id, nil
			}
		}
	}
	return "", fmt.Errorf("agent %q not found", agentKey)
}

// internalFields are API response fields not present in manifests.
// Stripping them prevents phantom diffs during comparison.
// Note: "id" is intentionally excluded — update/delete paths need it to resolve
// the resource UUID before issuing PUT/DELETE requests.
var internalFields = []string{
	// Timestamps & ownership
	"created_at", "updated_at", "created_by", "owner_id",
	// Tenant metadata
	"tenant_id", "tenant_name", "tenant_slug",
	// Workspace (managed by GoClaw UI)
	"restrict_to_workspace", "workspace",
	// Complex JSONB configs (managed by GoClaw UI, not manifests)
	"tools_config", "sandbox_config", "subagents_config",
	"memory_config", "compaction_config", "context_pruning", "other_config",
	// v3 promoted JSONB configs (complex structures, managed via UI)
	"reasoning_config", "workspace_sharing", "chatgpt_oauth_routing",
	"shell_deny_groups", "kg_dedup_config",
	// Accounting & metadata
	"frontmatter",
	"agent_count", "has_credentials", "credentials",
	// Skill-specific server-managed fields (not authored in manifests)
	"file_path", "file_size", "file_hash", "path", "baseDir", "source",
	"missing_deps", "deps", "is_system", "author",
	// Skill tenant-config field — surfaced separately via KindSkillConfig
	"tenant_enabled",
}

// stripInternal removes API-internal fields from an observed resource.
func stripInternal(m map[string]any) map[string]any {
	for _, f := range internalFields {
		delete(m, f)
	}
	return m
}

// matchesTenant returns true if the resource belongs to the provider's tenant.
// Always true if provider is not tenant-scoped (single-tenant mode).
// When the API response lacks tenant_id, trusts API header-based scoping.
func (p *Provider) matchesTenant(ctx context.Context, resource map[string]any) bool {
	if p.tenantID == "" {
		return true
	}
	tid := strVal(resource, "tenant_id")
	if tid == "" {
		return true // API doesn't include tenant_id — trust header scoping
	}
	uuid, err := p.resolveTenantUUID(ctx)
	if err != nil || uuid == "" {
		return true
	}
	return tid == uuid
}

// strVal safely extracts a string value from a map.
func strVal(m map[string]any, key string) string {
	v, ok := m[key]
	if !ok {
		return ""
	}
	s, ok := v.(string)
	if !ok {
		return ""
	}
	return s
}

// boolVal safely extracts a bool value from a map.
func boolVal(m map[string]any, key string) bool {
	b, _ := m[key].(bool)
	return b
}

// copyMap creates a shallow copy of a map.
func copyMap(m map[string]any) map[string]any {
	out := make(map[string]any, len(m))
	for k, v := range m {
		out[k] = v
	}
	return out
}
