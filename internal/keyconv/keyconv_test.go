package keyconv

import (
	"reflect"
	"testing"
)

func TestCamelToSnake(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{"simple", "displayName", "display_name"},
		{"api key", "apiKey", "api_key"},
		{"api base", "apiBase", "api_base"},
		{"provider type", "providerType", "provider_type"},
		{"is default", "isDefault", "is_default"},
		{"agent type", "agentType", "agent_type"},
		{"tools config", "toolsConfig", "tools_config"},
		{"agent key", "agentKey", "agent_key"},
		{"channel type", "channelType", "channel_type"},
		{"bot token", "botToken", "bot_token"},
		{"context files", "contextFiles", "context_files"},
		{"acronym userID", "userID", "user_id"},
		{"acronym getHTTPResponse", "getHTTPResponse", "get_http_response"},
		{"acronym ownerID", "ownerID", "owner_id"},
		{"no change lowercase", "name", "name"},
		{"single char", "x", "x"},
		{"empty", "", ""},
		{"already snake", "display_name", "display_name"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := camelToSnake(tt.in)
			if got != tt.want {
				t.Errorf("camelToSnake(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

func TestSnakeToCamel(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{"simple", "display_name", "displayName"},
		{"api key", "api_key", "apiKey"},
		{"api base", "api_base", "apiBase"},
		{"provider type", "provider_type", "providerType"},
		{"is default", "is_default", "isDefault"},
		{"agent type", "agent_type", "agentType"},
		{"tools config", "tools_config", "toolsConfig"},
		{"agent key", "agent_key", "agentKey"},
		{"no change camel", "name", "name"},
		{"empty", "", ""},
		{"already camel", "displayName", "displayName"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := snakeToCamel(tt.in)
			if got != tt.want {
				t.Errorf("snakeToCamel(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

func TestCamelToSnakeMap(t *testing.T) {
	in := map[string]any{
		"displayName":  "Bot",
		"providerType": "zai_coding",
		"apiBase":      "https://example.com",
		"apiKey":       "secret",
		"enabled":      true,
	}

	want := map[string]any{
		"display_name":  "Bot",
		"provider_type": "zai_coding",
		"api_base":      "https://example.com",
		"api_key":       "secret",
		"enabled":       true,
	}

	got := CamelToSnake(in)
	if !reflect.DeepEqual(got, want) {
		t.Errorf("CamelToSnake() = %v, want %v", got, want)
	}
}

func TestSnakeToCamelMap(t *testing.T) {
	in := map[string]any{
		"display_name":  "Bot",
		"provider_type": "zai_coding",
		"api_base":      "https://example.com",
		"api_key":       "***",
		"enabled":       true,
	}

	want := map[string]any{
		"displayName":  "Bot",
		"providerType": "zai_coding",
		"apiBase":      "https://example.com",
		"apiKey":       "***",
		"enabled":      true,
	}

	got := SnakeToCamel(in)
	if !reflect.DeepEqual(got, want) {
		t.Errorf("SnakeToCamel() = %v, want %v", got, want)
	}
}

func TestNestedMapTranslation(t *testing.T) {
	in := map[string]any{
		"displayName": "Agent",
		"toolsConfig": map[string]any{
			"maxRetries": 3,
			"allowList":  []any{"exec", "webFetch"},
		},
		"contextFiles": []any{
			map[string]any{
				"fileName":    "IDENTITY.md",
				"fileContent": "You are a bot.",
			},
		},
	}

	snake := CamelToSnake(in)

	// Verify nested map keys
	tc, ok := snake["tools_config"].(map[string]any)
	if !ok {
		t.Fatal("tools_config should be map")
	}
	if tc["max_retries"] != 3 {
		t.Errorf("nested key max_retries = %v, want 3", tc["max_retries"])
	}

	// Verify slice of maps
	cf, ok := snake["context_files"].([]any)
	if !ok || len(cf) != 1 {
		t.Fatal("context_files should be []any with 1 element")
	}
	cfMap, ok := cf[0].(map[string]any)
	if !ok {
		t.Fatal("context_files[0] should be map")
	}
	if cfMap["file_name"] != "IDENTITY.md" {
		t.Errorf("file_name = %v, want IDENTITY.md", cfMap["file_name"])
	}
}

func TestRoundTrip(t *testing.T) {
	original := map[string]any{
		"displayName":  "Bot",
		"providerType": "anthropic",
		"toolsConfig": map[string]any{
			"profileName": "coding",
		},
	}

	roundTripped := SnakeToCamel(CamelToSnake(original))
	if !reflect.DeepEqual(roundTripped, original) {
		t.Errorf("round-trip failed: got %v, want %v", roundTripped, original)
	}
}
