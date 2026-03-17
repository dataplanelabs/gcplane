package goclaw

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/dataplanelabs/gcplane/internal/manifest"
	"github.com/dataplanelabs/gcplane/internal/reconciler"
)

// listAllProviders returns ResourceInfo for every provider in GoClaw.
func (p *Provider) listAllProviders() ([]reconciler.ResourceInfo, error) {
	data, err := p.http.Get(context.Background(), "/v1/providers")
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
		infos = append(infos, reconciler.ResourceInfo{
			Kind:      manifest.KindProvider,
			Name:      strVal(prov, "name"),
			CreatedBy: strVal(prov, "created_by"),
		})
	}
	return infos, nil
}

// listAllAgents returns ResourceInfo for every agent in GoClaw.
func (p *Provider) listAllAgents() ([]reconciler.ResourceInfo, error) {
	data, err := p.http.Get(context.Background(), "/v1/agents")
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
		infos = append(infos, reconciler.ResourceInfo{
			Kind:      manifest.KindAgent,
			Name:      strVal(a, "agent_key"),
			CreatedBy: strVal(a, "created_by"),
		})
	}
	return infos, nil
}

// listAllChannels returns ResourceInfo for every channel instance in GoClaw.
func (p *Provider) listAllChannels() ([]reconciler.ResourceInfo, error) {
	data, err := p.http.Get(context.Background(), "/v1/channels/instances")
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
		infos = append(infos, reconciler.ResourceInfo{
			Kind:      manifest.KindChannel,
			Name:      strVal(inst, "name"),
			CreatedBy: strVal(inst, "created_by"),
		})
	}
	return infos, nil
}

// listAllMCPServers returns ResourceInfo for every MCP server in GoClaw.
func (p *Provider) listAllMCPServers() ([]reconciler.ResourceInfo, error) {
	data, err := p.http.Get(context.Background(), "/v1/mcp/servers")
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
		infos = append(infos, reconciler.ResourceInfo{
			Kind:      manifest.KindMCPServer,
			Name:      strVal(s, "name"),
			CreatedBy: strVal(s, "created_by"),
		})
	}
	return infos, nil
}

// listAllCustomTools returns ResourceInfo for every custom tool in GoClaw.
func (p *Provider) listAllCustomTools() ([]reconciler.ResourceInfo, error) {
	data, err := p.http.Get(context.Background(), "/v1/tools/custom")
	if err != nil {
		return nil, fmt.Errorf("list custom tools: %w", err)
	}
	var resp struct {
		Tools []map[string]any `json:"tools"`
	}
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, fmt.Errorf("parse custom tools response: %w", err)
	}
	infos := make([]reconciler.ResourceInfo, 0, len(resp.Tools))
	for _, t := range resp.Tools {
		infos = append(infos, reconciler.ResourceInfo{
			Kind:      manifest.KindTool,
			Name:      strVal(t, "name"),
			CreatedBy: strVal(t, "created_by"),
		})
	}
	return infos, nil
}

// listAllSkills returns ResourceInfo for every skill in GoClaw.
func (p *Provider) listAllSkills() ([]reconciler.ResourceInfo, error) {
	data, err := p.http.Get(context.Background(), "/v1/skills")
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
		infos = append(infos, reconciler.ResourceInfo{
			Kind:      manifest.KindSkill,
			Name:      strVal(s, "slug"),
			CreatedBy: strVal(s, "created_by"),
		})
	}
	return infos, nil
}

// listAllCronJobs returns ResourceInfo for every cron job in GoClaw via WS RPC.
func (p *Provider) listAllCronJobs() ([]reconciler.ResourceInfo, error) {
	if err := p.ensureWS(); err != nil {
		return nil, fmt.Errorf("ws connect for cron: %w", err)
	}
	payload, err := p.ws.Call(context.Background(), "cron.list", nil)
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
		infos = append(infos, reconciler.ResourceInfo{
			Kind:      manifest.KindCronJob,
			Name:      strVal(job, "name"),
			CreatedBy: strVal(job, "created_by"),
		})
	}
	return infos, nil
}

// listAllTeams returns ResourceInfo for every team in GoClaw via WS RPC.
func (p *Provider) listAllTeams() ([]reconciler.ResourceInfo, error) {
	if err := p.ensureWS(); err != nil {
		return nil, fmt.Errorf("ws connect for teams: %w", err)
	}
	payload, err := p.ws.Call(context.Background(), "teams.list", nil)
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
		infos = append(infos, reconciler.ResourceInfo{
			Kind:      manifest.KindAgentTeam,
			Name:      strVal(team, "name"),
			CreatedBy: strVal(team, "created_by"),
		})
	}
	return infos, nil
}
