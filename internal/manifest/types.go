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
}

// ResourceKind enumerates the managed resource types.
type ResourceKind string

const (
	KindProvider        ResourceKind = "Provider"
	KindAgent           ResourceKind = "Agent"
	KindChannelInstance ResourceKind = "ChannelInstance"
	KindCronJob         ResourceKind = "CronJob"
	KindMCPServer       ResourceKind = "MCPServer"
	KindSkill           ResourceKind = "Skill"
	KindCustomTool      ResourceKind = "CustomTool"
	KindTeam            ResourceKind = "Team"
	KindTTSConfig       ResourceKind = "TTSConfig"
)

// Resource is a generic managed resource with kind + key + arbitrary spec.
type Resource struct {
	Kind ResourceKind           `yaml:"kind"`
	Key  string                 `yaml:"key"`
	Spec map[string]any         `yaml:"spec"`
}

// ApplyOrder returns the dependency-ordered resource kinds.
// Resources must be applied in this order to satisfy dependencies.
func ApplyOrder() []ResourceKind {
	return []ResourceKind{
		KindProvider,        // no deps
		KindAgent,           // depends on Provider
		KindSkill,           // depends on Agent for grants
		KindMCPServer,       // depends on Agent for grants
		KindCustomTool,      // depends on Agent
		KindChannelInstance, // depends on Agent
		KindCronJob,         // depends on Agent
		KindTeam,            // no strict deps
		KindTTSConfig,       // global, no deps
	}
}
