package manifest

// writeOnlyFields lists fields that exist in manifest but are not returned
// by the GoClaw API (secrets, UUIDs, grants managed separately).
// These fields are excluded from comparison during reconciliation.
var writeOnlyFields = map[ResourceKind][]string{
	KindTenant:            {},
	KindProvider:          {"apiKey"},
	KindAgent: {
		"contextFiles", "systemPrompt",
		// Complex JSONB configs — sent on create/update but not compared
		// (managed via GoClaw UI at runtime, stripped from observe by internalFields)
		"toolsConfig", "sandboxConfig", "subagentsConfig",
		"memoryConfig", "compactionConfig", "contextPruning", "otherConfig",
	},
	KindChannel:           {"agentKey", "credentials", "config"},
	KindMCPServer:         {"grants"},
	KindCronJob:           {"agentKey", "message"},
	KindAgentTeam:         {"lead", "members", "displayName"},
	KindSkill:             {},
	KindBuiltinToolConfig: {"settings"}, // provider chain settings not returned by list API
	KindSkillConfig:       {},
	KindSystemConfig:      {},
	KindMCPCredentials:    {"credentials"},      // credentials are write-only (encrypted)
	KindSecureCLI:         {"env"},              // encrypted env vars are write-only
	KindSecureCLIGrant:    {"agentKey", "binaryName"}, // manifest references, not in API response
}

// WriteOnlyFields returns the write-only fields for a resource kind.
// These fields should be excluded from spec comparison during reconciliation.
func WriteOnlyFields(kind ResourceKind) []string {
	return writeOnlyFields[kind]
}
