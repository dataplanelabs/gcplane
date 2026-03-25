// Package goclaw implements the GoClaw provider for the reconciler.
// It communicates with GoClaw via HTTP REST (primary) and WS RPC (fallback).
package goclaw

import (
	"context"
	"fmt"
	"sync"

	"github.com/dataplanelabs/gcplane/internal/manifest"
	"github.com/dataplanelabs/gcplane/internal/reconciler"
)

// Option configures a Provider.
type Option func(*Provider)

// WithTenantID sets the tenant ID for all requests made by this provider.
// When set, X-GoClaw-Tenant-Id header is sent with HTTP requests and
// tenant_id is included in WS connect handshake.
func WithTenantID(id string) Option {
	return func(p *Provider) {
		p.tenantID = id
	}
}

// WithUserID sets the X-GoClaw-User-Id header. Default is "gcplane".
// Must match GOCLAW_OWNER_IDS for system-level operations (e.g., Tenant CRUD).
func WithUserID(id string) Option {
	return func(p *Provider) {
		p.userID = id
	}
}

// Provider communicates with a GoClaw instance to observe and mutate resources.
type Provider struct {
	endpoint   string
	token      string
	tenantID   string // slug (e.g., "acme-corp")
	tenantUUID string // resolved UUID — lazily populated on first create
	userID     string // X-GoClaw-User-Id header (default: "gcplane")
	http       *HTTPClient
	ws         *WSClient
	wsMu       sync.Mutex
	wsReady    bool
	tenantOnce sync.Once
	tenantErr  error
}

// TenantID returns the provider's tenant ID (empty string if not scoped).
func (p *Provider) TenantID() string {
	return p.tenantID
}

// New creates a GoClaw provider with the given connection config.
func New(endpoint, token string, opts ...Option) *Provider {
	p := &Provider{
		endpoint: endpoint,
		token:    token,
	}
	for _, opt := range opts {
		opt(p)
	}
	p.http = NewHTTPClient(endpoint, token, p.tenantID, p.userID)
	p.ws = NewWSClient(endpoint, token, p.tenantID, p.userID)
	return p
}

// ensureWS lazily connects the WebSocket client, retrying on transient failures.
// Unlike sync.Once, a failed connection does not permanently disable WS operations.
func (p *Provider) ensureWS(ctx context.Context) error {
	p.wsMu.Lock()
	defer p.wsMu.Unlock()

	if p.wsReady {
		return nil
	}

	if err := p.ws.Connect(ctx); err != nil {
		return err
	}
	p.wsReady = true
	return nil
}

// Close releases provider resources (WS connection).
func (p *Provider) Close() error {
	p.wsMu.Lock()
	p.wsReady = false
	p.wsMu.Unlock()

	if p.ws != nil {
		return p.ws.Close()
	}
	return nil
}

// Observe fetches the current state of a resource from GoClaw.
func (p *Provider) Observe(ctx context.Context, kind manifest.ResourceKind, key string) (map[string]any, error) {
	switch kind {
	case manifest.KindTenant:
		return p.observeTenant(ctx, key)
	case manifest.KindProvider:
		return p.observeProvider(ctx, key)
	case manifest.KindAgent:
		return p.observeAgent(ctx, key)
	case manifest.KindChannel:
		return p.observeChannelInstance(ctx, key)
	case manifest.KindMCPServer:
		return p.observeMCPServer(ctx, key)
	case manifest.KindSkill:
		return p.observeSkill(ctx, key)
	case manifest.KindTool:
		return p.observeCustomTool(ctx, key)
	case manifest.KindCronJob:
		return p.observeCronJob(ctx, key)
	case manifest.KindAgentTeam:
		return p.observeTeam(ctx, key)
	case manifest.KindTTSConfig:
		return p.observeTTSConfig(ctx, key)
	case manifest.KindBuiltinToolConfig:
		return p.observeBuiltinToolConfig(ctx, key)
	case manifest.KindSkillConfig:
		return p.observeSkillConfig(ctx, key)
	case manifest.KindMCPCredentials:
		return p.observeMCPCredentials(ctx, key)
	default:
		return nil, fmt.Errorf("observe not implemented for kind %s", kind)
	}
}

// resolveTenantUUID resolves the tenant slug to a UUID (cached after first call).
// No-op when tenantID is empty (single-tenant mode).
func (p *Provider) resolveTenantUUID(ctx context.Context) (string, error) {
	if p.tenantID == "" {
		return "", nil
	}
	p.tenantOnce.Do(func() {
		tenant, err := p.observeTenant(ctx, p.tenantID)
		if err != nil {
			p.tenantErr = fmt.Errorf("resolve tenant UUID for %q: %w", p.tenantID, err)
			return
		}
		if tenant == nil {
			p.tenantErr = fmt.Errorf("tenant %q not found", p.tenantID)
			return
		}
		id, ok := tenant["id"].(string)
		if !ok {
			p.tenantErr = fmt.Errorf("tenant %q: missing id", p.tenantID)
			return
		}
		p.tenantUUID = id
	})
	return p.tenantUUID, p.tenantErr
}

// Create creates a new resource in GoClaw.
func (p *Provider) Create(ctx context.Context, kind manifest.ResourceKind, key string, spec map[string]any) error {
	// Inject tenant_id UUID into spec for tenant-scoped creates
	if p.tenantID != "" && kind != manifest.KindTenant {
		uuid, err := p.resolveTenantUUID(ctx)
		if err != nil {
			return err
		}
		spec["tenantId"] = uuid
	}

	switch kind {
	case manifest.KindTenant:
		return p.createTenant(ctx, key, spec)
	case manifest.KindProvider:
		return p.createProvider(ctx, key, spec)
	case manifest.KindAgent:
		return p.createAgent(ctx, key, spec)
	case manifest.KindChannel:
		return p.createChannelInstance(ctx, key, spec)
	case manifest.KindMCPServer:
		return p.createMCPServer(ctx, key, spec)
	case manifest.KindTool:
		return p.createCustomTool(ctx, key, spec)
	case manifest.KindCronJob:
		return p.createCronJob(ctx, key, spec)
	case manifest.KindAgentTeam:
		return p.createTeam(ctx, key, spec)
	case manifest.KindTTSConfig:
		return p.createTTSConfig(ctx, key, spec)
	case manifest.KindBuiltinToolConfig:
		return p.createBuiltinToolConfig(ctx, key, spec)
	case manifest.KindSkillConfig:
		return p.createSkillConfig(ctx, key, spec)
	case manifest.KindMCPCredentials:
		return p.createMCPCredentials(ctx, key, spec)
	default:
		return fmt.Errorf("create not implemented for kind %s", kind)
	}
}

// Update patches an existing resource in GoClaw.
func (p *Provider) Update(ctx context.Context, kind manifest.ResourceKind, key string, spec map[string]any) error {
	switch kind {
	case manifest.KindTenant:
		return p.updateTenant(ctx, key, spec)
	case manifest.KindProvider:
		return p.updateProvider(ctx, key, spec)
	case manifest.KindAgent:
		return p.updateAgent(ctx, key, spec)
	case manifest.KindChannel:
		return p.updateChannelInstance(ctx, key, spec)
	case manifest.KindMCPServer:
		return p.updateMCPServer(ctx, key, spec)
	case manifest.KindSkill:
		return p.updateSkill(ctx, key, spec)
	case manifest.KindTool:
		return p.updateCustomTool(ctx, key, spec)
	case manifest.KindCronJob:
		return p.updateCronJob(ctx, key, spec)
	case manifest.KindAgentTeam:
		return p.updateTeam(ctx, key, spec)
	case manifest.KindTTSConfig:
		return p.updateTTSConfig(ctx, key, spec)
	case manifest.KindBuiltinToolConfig:
		return p.updateBuiltinToolConfig(ctx, key, spec)
	case manifest.KindSkillConfig:
		return p.updateSkillConfig(ctx, key, spec)
	case manifest.KindMCPCredentials:
		return p.updateMCPCredentials(ctx, key, spec)
	default:
		return fmt.Errorf("update not implemented for kind %s", kind)
	}
}

// Delete removes a resource from GoClaw. Idempotent: no-op if already absent.
func (p *Provider) Delete(ctx context.Context, kind manifest.ResourceKind, key string) error {
	switch kind {
	case manifest.KindTenant:
		return p.deleteTenant(ctx, key)
	case manifest.KindProvider:
		return p.deleteProvider(ctx, key)
	case manifest.KindAgent:
		return p.deleteAgent(ctx, key)
	case manifest.KindChannel:
		return p.deleteChannelInstance(ctx, key)
	case manifest.KindMCPServer:
		return p.deleteMCPServer(ctx, key)
	case manifest.KindTool:
		return p.deleteCustomTool(ctx, key)
	case manifest.KindCronJob:
		return p.deleteCronJob(ctx, key)
	case manifest.KindAgentTeam:
		return p.deleteTeam(ctx, key)
	case manifest.KindBuiltinToolConfig:
		return p.deleteBuiltinToolConfig(ctx, key)
	case manifest.KindSkillConfig:
		return p.deleteSkillConfig(ctx, key)
	case manifest.KindMCPCredentials:
		return p.deleteMCPCredentials(ctx, key)
	case manifest.KindSkill, manifest.KindTTSConfig:
		return nil // not deletable
	default:
		return fmt.Errorf("delete not implemented for kind %s", kind)
	}
}

// ListAll returns lightweight resource references for every remote resource of a given kind.
func (p *Provider) ListAll(ctx context.Context, kind manifest.ResourceKind) ([]reconciler.ResourceInfo, error) {
	switch kind {
	case manifest.KindTenant:
		return p.listAllTenants(ctx)
	case manifest.KindProvider:
		return p.listAllProviders(ctx)
	case manifest.KindAgent:
		return p.listAllAgents(ctx)
	case manifest.KindChannel:
		return p.listAllChannels(ctx)
	case manifest.KindMCPServer:
		return p.listAllMCPServers(ctx)
	case manifest.KindTool:
		return p.listAllCustomTools(ctx)
	case manifest.KindSkill:
		return p.listAllSkills(ctx)
	case manifest.KindCronJob:
		return p.listAllCronJobs(ctx)
	case manifest.KindAgentTeam:
		return p.listAllTeams(ctx)
	case manifest.KindTTSConfig, manifest.KindBuiltinToolConfig, manifest.KindSkillConfig, manifest.KindMCPCredentials:
		return nil, nil // per-tenant configs not enumerable for prune
	default:
		return nil, fmt.Errorf("list not implemented for kind %s", kind)
	}
}
