package goclaw

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/dataplanelabs/gcplane/internal/manifest"
	"github.com/dataplanelabs/gcplane/internal/reconciler"
)

// listAllProviders returns ResourceInfo for every provider in GoClaw.
func (p *Provider) listAllProviders(ctx context.Context) ([]reconciler.ResourceInfo, error) {
	data, err := p.http.Get(ctx, "/v1/providers")
	if err != nil {
		return nil, fmt.Errorf("list providers: %w", err)
	}
	var resp struct {
		Providers []map[string]any `json:"providers"`
	}
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, fmt.Errorf("parse providers response: %w", err)
	}
	infos := make([]reconciler.ResourceInfo, 0, len(resp.Providers))
	for _, prov := range resp.Providers {
		if !p.matchesTenant(ctx, prov) {
			continue
		}
		infos = append(infos, reconciler.ResourceInfo{
			Kind:      manifest.KindProvider,
			Name:      strVal(prov, "name"),
			CreatedBy: strVal(prov, "created_by"),
		})
	}
	return infos, nil
}

// listAllAgents returns ResourceInfo for every agent in GoClaw.
func (p *Provider) listAllAgents(ctx context.Context) ([]reconciler.ResourceInfo, error) {
	data, err := p.http.Get(ctx, "/v1/agents")
	if err != nil {
		return nil, fmt.Errorf("list agents: %w", err)
	}
	var resp struct {
		Agents []map[string]any `json:"agents"`
	}
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, fmt.Errorf("parse agents response: %w", err)
	}
	infos := make([]reconciler.ResourceInfo, 0, len(resp.Agents))
	for _, a := range resp.Agents {
		if !p.matchesTenant(ctx, a) {
			continue
		}
		infos = append(infos, reconciler.ResourceInfo{
			Kind:      manifest.KindAgent,
			Name:      strVal(a, "agent_key"),
			CreatedBy: strVal(a, "created_by"),
		})
	}
	return infos, nil
}

// listAllChannels returns ResourceInfo for every channel instance in GoClaw.
func (p *Provider) listAllChannels(ctx context.Context) ([]reconciler.ResourceInfo, error) {
	data, err := p.http.Get(ctx, "/v1/channels/instances")
	if err != nil {
		return nil, fmt.Errorf("list channel instances: %w", err)
	}
	var resp struct {
		Instances []map[string]any `json:"instances"`
	}
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, fmt.Errorf("parse channel instances response: %w", err)
	}
	infos := make([]reconciler.ResourceInfo, 0, len(resp.Instances))
	for _, inst := range resp.Instances {
		if !p.matchesTenant(ctx, inst) {
			continue
		}
		infos = append(infos, reconciler.ResourceInfo{
			Kind:      manifest.KindChannel,
			Name:      strVal(inst, "name"),
			CreatedBy: strVal(inst, "created_by"),
		})
	}
	return infos, nil
}

// listAllMCPServers returns ResourceInfo for every MCP server in GoClaw.
func (p *Provider) listAllMCPServers(ctx context.Context) ([]reconciler.ResourceInfo, error) {
	data, err := p.http.Get(ctx, "/v1/mcp/servers")
	if err != nil {
		return nil, fmt.Errorf("list mcp servers: %w", err)
	}
	var resp struct {
		Servers []map[string]any `json:"servers"`
	}
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, fmt.Errorf("parse mcp servers response: %w", err)
	}
	infos := make([]reconciler.ResourceInfo, 0, len(resp.Servers))
	for _, s := range resp.Servers {
		if !p.matchesTenant(ctx, s) {
			continue
		}
		infos = append(infos, reconciler.ResourceInfo{
			Kind:      manifest.KindMCPServer,
			Name:      strVal(s, "name"),
			CreatedBy: strVal(s, "created_by"),
		})
	}
	return infos, nil
}

// listAllSkills returns ResourceInfo for every skill in GoClaw.
func (p *Provider) listAllSkills(ctx context.Context) ([]reconciler.ResourceInfo, error) {
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
	infos := make([]reconciler.ResourceInfo, 0, len(resp.Skills))
	for _, s := range resp.Skills {
		if !p.matchesTenant(ctx, s) {
			continue
		}
		infos = append(infos, reconciler.ResourceInfo{
			Kind:      manifest.KindSkill,
			Name:      strVal(s, "slug"),
			CreatedBy: strVal(s, "created_by"),
			Source:    strVal(s, "source"),
			IsSystem:  boolVal(s, "is_system"),
		})
	}
	return infos, nil
}

// listAllCronJobs returns ResourceInfo for every cron job in GoClaw via WS RPC.
func (p *Provider) listAllCronJobs(ctx context.Context) ([]reconciler.ResourceInfo, error) {
	if err := p.ensureWS(ctx); err != nil {
		return nil, fmt.Errorf("ws connect for cron: %w", err)
	}
	// includeDisabled so prune/diff sees disabled crons too (matches observeCronJob).
	payload, err := p.ws.Call(ctx, "cron.list", map[string]any{"includeDisabled": true})
	if err != nil {
		return nil, fmt.Errorf("cron.list: %w", err)
	}
	var resp struct {
		Jobs []map[string]any `json:"jobs"`
	}
	if err := json.Unmarshal(payload, &resp); err != nil {
		return nil, fmt.Errorf("parse cron.list response: %w", err)
	}
	infos := make([]reconciler.ResourceInfo, 0, len(resp.Jobs))
	for _, job := range resp.Jobs {
		if !p.matchesTenant(ctx, job) {
			continue
		}
		infos = append(infos, reconciler.ResourceInfo{
			Kind:      manifest.KindCronJob,
			Name:      strVal(job, "name"),
			CreatedBy: strVal(job, "created_by"),
		})
	}
	return infos, nil
}

// listAllTeams returns ResourceInfo for every team in GoClaw via WS RPC.
func (p *Provider) listAllTeams(ctx context.Context) ([]reconciler.ResourceInfo, error) {
	if err := p.ensureWS(ctx); err != nil {
		return nil, fmt.Errorf("ws connect for teams: %w", err)
	}
	payload, err := p.ws.Call(ctx, "teams.list", nil)
	if err != nil {
		return nil, fmt.Errorf("teams.list: %w", err)
	}
	var resp struct {
		Teams []map[string]any `json:"teams"`
	}
	if err := json.Unmarshal(payload, &resp); err != nil {
		return nil, fmt.Errorf("parse teams.list response: %w", err)
	}
	infos := make([]reconciler.ResourceInfo, 0, len(resp.Teams))
	for _, team := range resp.Teams {
		if !p.matchesTenant(ctx, team) {
			continue
		}
		infos = append(infos, reconciler.ResourceInfo{
			Kind:      manifest.KindAgentTeam,
			Name:      strVal(team, "name"),
			CreatedBy: strVal(team, "created_by"),
		})
	}
	return infos, nil
}

// listAllSecureCLIs returns ResourceInfo for every secure CLI binary in GoClaw.
func (p *Provider) listAllSecureCLIs(ctx context.Context) ([]reconciler.ResourceInfo, error) {
	data, err := p.http.Get(ctx, "/v1/cli-credentials")
	if err != nil {
		return nil, fmt.Errorf("list cli-credentials: %w", err)
	}
	var resp struct {
		Items []map[string]any `json:"items"`
	}
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, fmt.Errorf("parse cli-credentials response: %w", err)
	}
	infos := make([]reconciler.ResourceInfo, 0, len(resp.Items))
	for _, item := range resp.Items {
		if !p.matchesTenant(ctx, item) {
			continue
		}
		infos = append(infos, reconciler.ResourceInfo{
			Kind:      manifest.KindSecureCLI,
			Name:      strVal(item, "binary_name"),
			CreatedBy: strVal(item, "created_by"),
		})
	}
	return infos, nil
}

// listAllAgentLinks returns ResourceInfo for every agent link in GoClaw via WS RPC.
// agents.links.list requires an agentId — there is no tenant-wide list endpoint.
// Strategy: enumerate all agents in the tenant, list links from each as source
// (direction=from), and emit one ResourceInfo per link with composite name
// "sourceKey--targetKey". Each link has exactly one source so no dedup needed.
// Skips links with team_id set — those are managed by the AgentTeam subsystem.
func (p *Provider) listAllAgentLinks(ctx context.Context) ([]reconciler.ResourceInfo, error) {
	if err := p.ensureWS(ctx); err != nil {
		return nil, fmt.Errorf("ws connect for agent links: %w", err)
	}

	// Enumerate agents in tenant.
	agentData, err := p.http.Get(ctx, "/v1/agents")
	if err != nil {
		return nil, fmt.Errorf("list agents: %w", err)
	}
	var agentResp struct {
		Agents []map[string]any `json:"agents"`
	}
	if err := json.Unmarshal(agentData, &agentResp); err != nil {
		return nil, fmt.Errorf("parse agents response: %w", err)
	}

	// Build agentID → agentKey map for translating target IDs back to keys.
	keyByID := make(map[string]string, len(agentResp.Agents))
	var sourceAgents []map[string]any
	for _, a := range agentResp.Agents {
		if !p.matchesTenant(ctx, a) {
			continue
		}
		id := strVal(a, "id")
		key := strVal(a, "agent_key")
		if id == "" || key == "" {
			continue
		}
		keyByID[id] = key
		sourceAgents = append(sourceAgents, a)
	}

	infos := make([]reconciler.ResourceInfo, 0)
	for _, a := range sourceAgents {
		sourceID := strVal(a, "id")
		sourceKey := strVal(a, "agent_key")

		payload, err := p.ws.Call(ctx, "agents.links.list", map[string]any{
			"agentId":   sourceID,
			"direction": "from",
		})
		if err != nil {
			return nil, fmt.Errorf("agents.links.list for %s: %w", sourceKey, err)
		}
		var resp struct {
			Links []map[string]any `json:"links"`
		}
		if err := json.Unmarshal(payload, &resp); err != nil {
			return nil, fmt.Errorf("parse agents.links.list response: %w", err)
		}

		for _, link := range resp.Links {
			camel := translateResult(link)
			// Skip team-managed links — owned by AgentTeam subsystem.
			if tid := strVal(camel, "teamId"); tid != "" {
				continue
			}
			targetID := strVal(camel, "targetAgentId")
			targetKey := keyByID[targetID]
			if targetKey == "" {
				// Prefer the joined key if denormalized in the response; else skip.
				targetKey = strVal(camel, "targetAgentKey")
				if targetKey == "" {
					continue
				}
			}
			infos = append(infos, reconciler.ResourceInfo{
				Kind:      manifest.KindAgentLink,
				Name:      sourceKey + "--" + targetKey,
				CreatedBy: strVal(camel, "createdBy"),
			})
		}
	}
	return infos, nil
}

// listAllWorkstations returns ResourceInfo for every workstation in GoClaw via WS RPC.
func (p *Provider) listAllWorkstations(ctx context.Context) ([]reconciler.ResourceInfo, error) {
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
	infos := make([]reconciler.ResourceInfo, 0, len(resp.Workstations))
	for _, ws := range resp.Workstations {
		infos = append(infos, reconciler.ResourceInfo{
			Kind:      manifest.KindWorkstation,
			Name:      strVal(ws, "workstationKey"),
			CreatedBy: strVal(ws, "createdBy"),
		})
	}
	return infos, nil
}

// listAllTenants returns ResourceInfo for every tenant in GoClaw.
func (p *Provider) listAllTenants(ctx context.Context) ([]reconciler.ResourceInfo, error) {
	data, err := p.http.Get(ctx, "/v1/tenants")
	if err != nil {
		return nil, fmt.Errorf("list tenants: %w", err)
	}
	var resp struct {
		Tenants []map[string]any `json:"tenants"`
	}
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, fmt.Errorf("parse tenants response: %w", err)
	}
	infos := make([]reconciler.ResourceInfo, 0, len(resp.Tenants))
	for _, t := range resp.Tenants {
		infos = append(infos, reconciler.ResourceInfo{
			Kind:      manifest.KindTenant,
			Name:      strVal(t, "slug"),
			CreatedBy: strVal(t, "created_by"),
		})
	}
	return infos, nil
}
