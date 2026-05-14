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
		// v3 promoted JSONB configs (complex structures, write-only)
		"reasoningConfig", "workspaceSharing", "chatgptOauthRouting",
		"shellDenyGroups", "kgDedupConfig",
	},
	KindChannel:           {"agentKey", "credentials", "config"},
	KindMCPServer:         {"grants"},
	KindCronJob:           {"agentKey", "message"},
	KindAgentTeam:         {"lead", "members", "displayName"},
	KindSkill:             {},
	KindBuiltinToolConfig: {}, // settings are observable via tenant_settings in list response
	KindSkillConfig:       {},
	KindSystemConfig:      {},
	KindMCPCredentials:    {"credentials"},      // credentials are write-only (encrypted)
	KindSecureCLI:         {"env"},              // encrypted env vars are write-only
	KindSecureCLIGrant:    {"agentKey", "binaryName"}, // manifest references, not in API response
	KindAgentLink: {
		"sourceAgent", "targetAgent", // manifest references resolved to UUIDs at create time
		"settings", // per-user grants and rate limits — JSONB write-only
	},
}

// WriteOnlyFields returns the write-only fields for a resource kind.
// These fields should be excluded from spec comparison during reconciliation.
func WriteOnlyFields(kind ResourceKind) []string {
	return writeOnlyFields[kind]
}
