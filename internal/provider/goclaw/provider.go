// Package goclaw implements the GoClaw provider for the reconciler.
// It communicates with GoClaw via HTTP REST (primary) and WS RPC (fallback).
package goclaw

import (
	"fmt"
	"net/http"

	"github.com/dataplanelabs/gcplane/internal/manifest"
)

// Provider communicates with a GoClaw instance to observe and mutate resources.
type Provider struct {
	endpoint   string
	token      string
	httpClient *http.Client
}

// New creates a GoClaw provider with the given connection config.
func New(endpoint, token string) *Provider {
	return &Provider{
		endpoint:   endpoint,
		token:      token,
		httpClient: &http.Client{},
	}
}

// Observe fetches the current state of a resource from GoClaw.
func (p *Provider) Observe(kind manifest.ResourceKind, key string) (map[string]any, error) {
	switch kind {
	case manifest.KindProvider:
		return p.observeProvider(key)
	case manifest.KindAgent:
		return p.observeAgent(key)
	case manifest.KindChannelInstance:
		return p.observeChannelInstance(key)
	case manifest.KindMCPServer:
		return p.observeMCPServer(key)
	case manifest.KindSkill:
		return p.observeSkill(key)
	case manifest.KindCustomTool:
		return p.observeCustomTool(key)
	// CronJob, Team, TTSConfig will use WS RPC
	default:
		return nil, fmt.Errorf("observe not implemented for kind %s", kind)
	}
}

// Create creates a new resource in GoClaw.
func (p *Provider) Create(kind manifest.ResourceKind, key string, spec map[string]any) error {
	switch kind {
	case manifest.KindProvider:
		return p.createProvider(key, spec)
	case manifest.KindAgent:
		return p.createAgent(key, spec)
	case manifest.KindChannelInstance:
		return p.createChannelInstance(key, spec)
	case manifest.KindMCPServer:
		return p.createMCPServer(key, spec)
	default:
		return fmt.Errorf("create not implemented for kind %s", kind)
	}
}

// Update patches an existing resource in GoClaw.
func (p *Provider) Update(kind manifest.ResourceKind, key string, spec map[string]any) error {
	switch kind {
	case manifest.KindProvider:
		return p.updateProvider(key, spec)
	case manifest.KindAgent:
		return p.updateAgent(key, spec)
	case manifest.KindChannelInstance:
		return p.updateChannelInstance(key, spec)
	case manifest.KindMCPServer:
		return p.updateMCPServer(key, spec)
	default:
		return fmt.Errorf("update not implemented for kind %s", kind)
	}
}

// --- HTTP REST resource handlers (stubs) ---

func (p *Provider) observeProvider(key string) (map[string]any, error) {
	// TODO: GET /v1/providers + filter by key
	return nil, fmt.Errorf("not implemented")
}

func (p *Provider) createProvider(key string, spec map[string]any) error {
	// TODO: POST /v1/providers
	return fmt.Errorf("not implemented")
}

func (p *Provider) updateProvider(key string, spec map[string]any) error {
	// TODO: PATCH /v1/providers/:id
	return fmt.Errorf("not implemented")
}

func (p *Provider) observeAgent(key string) (map[string]any, error) {
	// TODO: GET /v1/agents + filter by agent_key
	return nil, fmt.Errorf("not implemented")
}

func (p *Provider) createAgent(key string, spec map[string]any) error {
	// TODO: POST /v1/agents
	return fmt.Errorf("not implemented")
}

func (p *Provider) updateAgent(key string, spec map[string]any) error {
	// TODO: PATCH /v1/agents/:id
	return fmt.Errorf("not implemented")
}

func (p *Provider) observeChannelInstance(key string) (map[string]any, error) {
	// TODO: GET /v1/channels/instances + filter
	return nil, fmt.Errorf("not implemented")
}

func (p *Provider) createChannelInstance(key string, spec map[string]any) error {
	// TODO: POST /v1/channels/instances
	return fmt.Errorf("not implemented")
}

func (p *Provider) updateChannelInstance(key string, spec map[string]any) error {
	// TODO: PATCH /v1/channels/instances/:id
	return fmt.Errorf("not implemented")
}

func (p *Provider) observeMCPServer(key string) (map[string]any, error) {
	// TODO: GET /v1/mcp/servers + filter
	return nil, fmt.Errorf("not implemented")
}

func (p *Provider) createMCPServer(key string, spec map[string]any) error {
	// TODO: POST /v1/mcp/servers
	return fmt.Errorf("not implemented")
}

func (p *Provider) updateMCPServer(key string, spec map[string]any) error {
	// TODO: PATCH /v1/mcp/servers/:id
	return fmt.Errorf("not implemented")
}

func (p *Provider) observeSkill(key string) (map[string]any, error) {
	// TODO: GET /v1/skills + filter
	return nil, fmt.Errorf("not implemented")
}

func (p *Provider) observeCustomTool(key string) (map[string]any, error) {
	// TODO: GET /v1/tools/custom + filter
	return nil, fmt.Errorf("not implemented")
}
