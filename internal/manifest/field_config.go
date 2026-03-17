package manifest

// writeOnlyFields lists fields that exist in manifest but are not returned
// by the GoClaw API (secrets, UUIDs, grants managed separately).
// These fields are excluded from comparison during reconciliation.
var writeOnlyFields = map[ResourceKind][]string{
	KindProvider:  {},
	KindAgent:     {"contextFiles"},
	KindChannel:   {"agentKey", "botToken", "config"},
	KindMCPServer: {"grants"},
	KindCronJob:   {"agentKey", "message"},
	KindAgentTeam:      {"lead", "members", "displayName"},
	KindTool:      {},
	KindSkill:     {},
	KindTTSConfig: {},
}

// WriteOnlyFields returns the write-only fields for a resource kind.
// These fields should be excluded from spec comparison during reconciliation.
func WriteOnlyFields(kind ResourceKind) []string {
	return writeOnlyFields[kind]
}
