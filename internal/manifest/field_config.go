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
	KindSkill: {
		// sourceDir is the local filesystem path used by gcplane to build the
		// upload ZIP; goclaw never returns it.
		"sourceDir",
		// version is goclaw-assigned (auto-bumped on upload); excluded from diff
		// so re-applying without code changes doesn't surface drift.
		"version",
	},
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

// kindsSupportingWriteOnlyHash lists resource kinds whose goclaw side accepts
// and echoes back a writeOnlyHash field on list/get. Only these kinds get the
// "empty observedHash treated as drift" safety-net in engine.stepCompare —
// for kinds NOT in this set, goclaw silently drops the field, so an empty
// observed value would otherwise create an infinite update loop.
//
// Add a kind here only after confirming goclaw migration + handler support.
// Current support:
//   - KindCronJob:  migration 000059 (goclaw)
//   - KindProvider: migration 000060 (goclaw)
var kindsSupportingWriteOnlyHash = map[ResourceKind]bool{
	KindCronJob:  true,
	KindProvider: true,
}

// SupportsWriteOnlyHash returns true if goclaw is known to persist + echo
// the writeOnlyHash field for this kind. The reconciler uses this to decide
// whether an empty observed hash should be treated as drift (auto-heal) or
// as the legacy blind-noop case (kind whose goclaw side never echoes hash).
func SupportsWriteOnlyHash(kind ResourceKind) bool {
	return kindsSupportingWriteOnlyHash[kind]
}
