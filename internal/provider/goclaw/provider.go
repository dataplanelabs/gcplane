// Package goclaw implements the GoClaw provider for the reconciler.
// It communicates with GoClaw via HTTP REST (primary) and WS RPC (fallback).
package goclaw

import (
	"context"
	"fmt"
	"sync"

	"github.com/dataplanelabs/gcplane/internal/manifest"
)

// Provider communicates with a GoClaw instance to observe and mutate resources.
type Provider struct {
	endpoint string
	token    string
	http     *HTTPClient
	ws       *WSClient
	wsOnce   sync.Once
	wsErr    error
}

// New creates a GoClaw provider with the given connection config.
func New(endpoint, token string) *Provider {
	return &Provider{
		endpoint: endpoint,
		token:    token,
		http:     NewHTTPClient(endpoint, token),
		ws:       NewWSClient(endpoint, token),
	}
}

// ensureWS lazily connects the WebSocket client on first WS resource call.
func (p *Provider) ensureWS() error {
	p.wsOnce.Do(func() {
		p.wsErr = p.ws.Connect(context.Background())
	})
	return p.wsErr
}

// Close releases provider resources (WS connection).
func (p *Provider) Close() error {
	if p.ws != nil {
		return p.ws.Close()
	}
	return nil
}

// Observe fetches the current state of a resource from GoClaw.
func (p *Provider) Observe(kind manifest.ResourceKind, key string) (map[string]any, error) {
	switch kind {
	case manifest.KindProvider:
		return p.observeProvider(key)
	case manifest.KindAgent:
		return p.observeAgent(key)
	case manifest.KindChannel:
		return p.observeChannelInstance(key)
	case manifest.KindMCPServer:
		return p.observeMCPServer(key)
	case manifest.KindSkill:
		return p.observeSkill(key)
	case manifest.KindTool:
		return p.observeCustomTool(key)
	case manifest.KindCronJob:
		return p.observeCronJob(key)
	case manifest.KindTeam:
		return p.observeTeam(key)
	case manifest.KindTTSConfig:
		return p.observeTTSConfig(key)
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
	case manifest.KindChannel:
		return p.createChannelInstance(key, spec)
	case manifest.KindMCPServer:
		return p.createMCPServer(key, spec)
	case manifest.KindTool:
		return p.createCustomTool(key, spec)
	case manifest.KindCronJob:
		return p.createCronJob(key, spec)
	case manifest.KindTeam:
		return p.createTeam(key, spec)
	case manifest.KindTTSConfig:
		return p.createTTSConfig(key, spec)
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
	case manifest.KindChannel:
		return p.updateChannelInstance(key, spec)
	case manifest.KindMCPServer:
		return p.updateMCPServer(key, spec)
	case manifest.KindSkill:
		return p.updateSkill(key, spec)
	case manifest.KindTool:
		return p.updateCustomTool(key, spec)
	case manifest.KindCronJob:
		return p.updateCronJob(key, spec)
	case manifest.KindTeam:
		return p.updateTeam(key, spec)
	case manifest.KindTTSConfig:
		return p.updateTTSConfig(key, spec)
	default:
		return fmt.Errorf("update not implemented for kind %s", kind)
	}
}
