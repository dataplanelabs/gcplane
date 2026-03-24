// Package manifest handles YAML manifest parsing and validation.
package manifest

// Manifest is the top-level declarative config for a GoClaw deployment.
type Manifest struct {
	APIVersion string     `yaml:"apiVersion"`
	Kind       string     `yaml:"kind"`
	Metadata   Metadata   `yaml:"metadata"`
	Connection Connection `yaml:"connection"`
	Resources  []Resource `yaml:"resources"`
}

// Metadata contains manifest-level metadata.
type Metadata struct {
	Name        string `yaml:"name"`
	Environment string `yaml:"environment,omitempty"`
}

// Connection configures how to reach the GoClaw instance.
type Connection struct {
	Endpoint string `yaml:"endpoint"`
	Token    string `yaml:"token"`
	TenantID string `yaml:"tenantId,omitempty"` // optional — scope all resources to this tenant
	UserID   string `yaml:"userId,omitempty"`   // optional — X-GoClaw-User-Id header (default: "gcplane")
}

// ResourceKind enumerates the managed resource types.
type ResourceKind string

const (
	KindTenant            ResourceKind = "Tenant"
	KindProvider          ResourceKind = "Provider"
	KindAgent             ResourceKind = "Agent"
	KindChannel           ResourceKind = "Channel"
	KindCronJob           ResourceKind = "CronJob"
	KindMCPServer         ResourceKind = "MCPServer"
	KindSkill             ResourceKind = "Skill"
	KindTool              ResourceKind = "Tool"
	KindAgentTeam         ResourceKind = "AgentTeam"
	KindTTSConfig         ResourceKind = "TTSConfig"
	KindBuiltinToolConfig ResourceKind = "BuiltinToolConfig"
	KindSkillConfig       ResourceKind = "SkillConfig"
	KindMCPCredentials    ResourceKind = "MCPCredentials"
)

// Resource is a generic managed resource with kind + name + arbitrary spec.
// Labels are gcplane-local metadata and are not sent to GoClaw.
type Resource struct {
	Kind   ResourceKind      `yaml:"kind"`
	Name   string            `yaml:"name"`
	Labels map[string]string `yaml:"labels,omitempty"`
	Spec   map[string]any    `yaml:"spec"`
}

// ApplyOrder returns the dependency-ordered resource kinds.
// Resources must be applied in this order to satisfy dependencies.
func ApplyOrder() []ResourceKind {
	return []ResourceKind{
		KindTenant,            // first — creates tenant before anything
		KindProvider,          // no deps within tenant
		KindAgent,             // depends on Provider
		KindSkill,             // depends on Agent for grants
		KindBuiltinToolConfig, // configures builtin tools per tenant
		KindSkillConfig,       // configures skills per tenant
		KindMCPServer,         // depends on Agent for grants
		KindMCPCredentials,    // per-user MCP creds, after MCPServer
		KindTool,              // depends on Agent
		KindChannel,           // depends on Agent
		KindCronJob,           // depends on Agent
		KindAgentTeam,         // no strict deps
		KindTTSConfig,         // global, no deps
	}
}
