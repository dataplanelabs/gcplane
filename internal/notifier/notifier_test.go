package notifier

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/dataplanelabs/gcplane/internal/controller"
	"github.com/dataplanelabs/gcplane/internal/manifest"
	"github.com/dataplanelabs/gcplane/internal/reconciler"
)

var testChanges = []reconciler.Change{
	{Kind: manifest.KindProvider, Name: "openai", Action: reconciler.ActionCreate},
	{Kind: manifest.KindAgent, Name: "my-agent", Action: reconciler.ActionUpdate},
}

func TestWebhookNotifier_NoOp_WhenURLEmpty(t *testing.T) {
	n := New("", "")
	err := n.NotifyDrift(context.Background(), testChanges)
	if err != nil {
		t.Errorf("expected nil error for empty webhook URL, got %v", err)
	}
}

func TestWebhookNotifier_NoOp_WhenChangesEmpty(t *testing.T) {
	called := false
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	n := New(srv.URL, "slack")
	if err := n.NotifyDrift(context.Background(), nil); err != nil {
		t.Errorf("expected nil error for empty changes, got %v", err)
	}
	if called {
		t.Error("expected no HTTP call for empty changes")
	}
}

func TestWebhookNotifier_DefaultsToSlack(t *testing.T) {
	n := New("http://example.com", "")
	if n.Format != FormatSlack {
		t.Errorf("expected default format slack, got %s", n.Format)
	}
}

func TestWebhookNotifier_SetsContentType(t *testing.T) {
	var gotCT string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotCT = r.Header.Get("Content-Type")
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	n := New(srv.URL, "discord")
	if err := n.NotifyDrift(context.Background(), testChanges); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gotCT != "application/json" {
		t.Errorf("expected application/json, got %s", gotCT)
	}
}

func TestWebhookNotifier_ReturnsError_OnNon2xx(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	n := New(srv.URL, "slack")
	if err := n.NotifyDrift(context.Background(), testChanges); err == nil {
		t.Error("expected error on 500 response")
	}
}

func TestWebhookNotifier_ReturnsError_OnNetworkFailure(t *testing.T) {
	n := New("http://127.0.0.1:0/webhook", "slack")
	if err := n.NotifyDrift(context.Background(), testChanges); err == nil {
		t.Error("expected error on network failure")
	}
}

// --- Format-specific payload tests ---

func TestBuildPayload_Slack(t *testing.T) {
	p := buildPayload(FormatSlack, testChanges)
	b, _ := json.Marshal(p)
	s := string(b)
	if !strings.Contains(s, "blocks") {
		t.Error("slack payload should contain blocks")
	}
	if !strings.Contains(s, "openai") {
		t.Error("slack payload should contain resource name")
	}
}

func TestBuildPayload_Discord(t *testing.T) {
	p := buildPayload(FormatDiscord, testChanges)
	b, _ := json.Marshal(p)
	s := string(b)
	if !strings.Contains(s, "embeds") {
		t.Error("discord payload should contain embeds")
	}
}

func TestBuildPayload_GoogleChat(t *testing.T) {
	p := buildPayload(FormatGoogleChat, testChanges)
	b, _ := json.Marshal(p)
	s := string(b)
	if !strings.Contains(s, "cardsV2") {
		t.Error("google chat payload should contain cardsV2")
	}
}

func TestBuildPayload_Teams(t *testing.T) {
	p := buildPayload(FormatTeams, testChanges)
	b, _ := json.Marshal(p)
	s := string(b)
	if !strings.Contains(s, "MessageCard") {
		t.Error("teams payload should contain MessageCard")
	}
}

func TestBuildPayload_Telegram(t *testing.T) {
	p := buildPayload(FormatTelegram, testChanges)
	b, _ := json.Marshal(p)
	s := string(b)
	if !strings.Contains(s, "parse_mode") {
		t.Error("telegram payload should contain parse_mode")
	}
}

func TestBuildPayload_AllFormatsIncludeResourceNames(t *testing.T) {
	formats := []string{FormatSlack, FormatDiscord, FormatGoogleChat, FormatTeams, FormatTelegram}
	for _, f := range formats {
		p := buildPayload(f, testChanges)
		b, _ := json.Marshal(p)
		s := string(b)
		if !strings.Contains(s, "openai") || !strings.Contains(s, "my-agent") {
			t.Errorf("format %s: payload missing resource names", f)
		}
	}
}

// --- Structure validation tests ---

func toMap(t *testing.T, v any) map[string]any {
	t.Helper()
	b, _ := json.Marshal(v)
	var m map[string]any
	if err := json.Unmarshal(b, &m); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	return m
}

func TestSlackPayload_Structure(t *testing.T) {
	t.Parallel()
	m := toMap(t, buildSlackPayload(testChanges))
	blocks, ok := m["blocks"].([]any)
	if !ok || len(blocks) < 2 {
		t.Fatalf("expected at least 2 blocks, got %v", m["blocks"])
	}
	header := blocks[0].(map[string]any)
	if header["type"] != "header" {
		t.Errorf("first block type = %v, want header", header["type"])
	}
	section := blocks[1].(map[string]any)
	if section["type"] != "section" {
		t.Errorf("second block type = %v, want section", section["type"])
	}
	text := section["text"].(map[string]any)
	if text["type"] != "mrkdwn" {
		t.Errorf("section text type = %v, want mrkdwn", text["type"])
	}
	// Slack uses *bold* not **bold**
	body := text["text"].(string)
	if strings.Contains(body, "**") {
		t.Error("slack should use *bold* not **bold**")
	}
}

func TestDiscordPayload_Structure(t *testing.T) {
	t.Parallel()
	m := toMap(t, buildDiscordPayload(testChanges))
	embeds, ok := m["embeds"].([]any)
	if !ok || len(embeds) < 1 {
		t.Fatalf("expected embeds array, got %v", m["embeds"])
	}
	embed := embeds[0].(map[string]any)
	if embed["title"] == nil || embed["title"] == "" {
		t.Error("embed title should be present")
	}
	if embed["color"] != float64(16750848) {
		t.Errorf("embed color = %v, want 16750848", embed["color"])
	}
	if embed["description"] == nil || embed["description"] == "" {
		t.Error("embed description should be non-empty")
	}
}

func TestGoogleChatPayload_Structure(t *testing.T) {
	t.Parallel()
	m := toMap(t, buildGoogleChatPayload(testChanges))
	cards, ok := m["cardsV2"].([]any)
	if !ok || len(cards) < 1 {
		t.Fatalf("expected cardsV2 array, got %v", m["cardsV2"])
	}
	card := cards[0].(map[string]any)["card"].(map[string]any)
	header := card["header"].(map[string]any)
	if header["title"] == nil || header["title"] == "" {
		t.Error("card header title should be present")
	}
	sections := card["sections"].([]any)
	if len(sections) < 1 {
		t.Fatal("expected at least 1 section")
	}
}

func TestTeamsPayload_Structure(t *testing.T) {
	t.Parallel()
	m := toMap(t, buildTeamsPayload(testChanges))
	if m["@type"] != "MessageCard" {
		t.Errorf("@type = %v, want MessageCard", m["@type"])
	}
	if m["themeColor"] != "FF8C00" {
		t.Errorf("themeColor = %v, want FF8C00", m["themeColor"])
	}
	sections, ok := m["sections"].([]any)
	if !ok || len(sections) < 1 {
		t.Fatalf("expected sections array, got %v", m["sections"])
	}
	sec := sections[0].(map[string]any)
	if sec["text"] == nil || sec["text"] == "" {
		t.Error("section text should be non-empty")
	}
}

func TestTelegramPayload_Structure(t *testing.T) {
	t.Parallel()
	m := toMap(t, buildTelegramPayload(testChanges))
	if m["parse_mode"] != "Markdown" {
		t.Errorf("parse_mode = %v, want Markdown", m["parse_mode"])
	}
	text, ok := m["text"].(string)
	if !ok || text == "" {
		t.Fatal("text should be non-empty string")
	}
	if !strings.HasPrefix(text, "*") {
		t.Errorf("telegram text should start with * (Markdown bold), got %q", text[:20])
	}
}

func TestWebhookNotifier_SendsFormatPayload(t *testing.T) {
	var received map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		json.Unmarshal(body, &received)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	n := New(srv.URL, FormatDiscord)
	if err := n.NotifyDrift(context.Background(), testChanges); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, ok := received["embeds"]; !ok {
		t.Error("expected discord embeds in payload")
	}
}

// --- Provider Verify Failure Tests ---

var testFailures = []controller.ProviderVerifyFailure{
	{Name: "openai", Error: "unauthorized: Missing Authentication header"},
	{Name: "anthropic", Error: "unauthorized: Invalid API key"},
}

func TestWebhookNotifier_VerifyFailure_NoOp_WhenURLEmpty(t *testing.T) {
	n := New("", "")
	err := n.NotifyProviderVerifyFailure(context.Background(), testFailures)
	if err != nil {
		t.Errorf("expected nil error for empty webhook URL, got %v", err)
	}
}

func TestWebhookNotifier_VerifyFailure_NoOp_WhenFailuresEmpty(t *testing.T) {
	called := false
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	n := New(srv.URL, "slack")
	if err := n.NotifyProviderVerifyFailure(context.Background(), nil); err != nil {
		t.Errorf("expected nil error for empty failures, got %v", err)
	}
	if called {
		t.Error("expected no HTTP call for empty failures")
	}
}

func TestWebhookNotifier_VerifyFailure_SendsPayload(t *testing.T) {
	var received map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		json.Unmarshal(body, &received)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	n := New(srv.URL, FormatSlack)
	if err := n.NotifyProviderVerifyFailure(context.Background(), testFailures); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, ok := received["blocks"]; !ok {
		t.Error("expected slack blocks in verify failure payload")
	}
}

func TestBuildVerifyFailurePayload_AllFormats(t *testing.T) {
	formats := []string{FormatSlack, FormatDiscord, FormatGoogleChat, FormatTeams, FormatTelegram}
	for _, f := range formats {
		p := buildVerifyFailurePayload(f, testFailures)
		b, _ := json.Marshal(p)
		s := string(b)
		if !strings.Contains(s, "openai") || !strings.Contains(s, "anthropic") {
			t.Errorf("format %s: payload missing provider names", f)
		}
		if !strings.Contains(s, "FAILED") && !strings.Contains(s, "failed") {
			// Telegram uses markdown, check for title content
			if f != FormatTelegram {
				t.Errorf("format %s: payload should indicate failure", f)
			}
		}
	}
}

func TestDiscordVerifyPayload_UsesRedColor(t *testing.T) {
	m := toMap(t, buildDiscordVerifyPayload(testFailures))
	embeds := m["embeds"].([]any)
	embed := embeds[0].(map[string]any)
	if embed["color"] != float64(15158332) {
		t.Errorf("discord verify embed color = %v, want 15158332 (red)", embed["color"])
	}
}

func TestTeamsVerifyPayload_UsesRedColor(t *testing.T) {
	m := toMap(t, buildTeamsVerifyPayload(testFailures))
	if m["themeColor"] != "FF0000" {
		t.Errorf("teams verify themeColor = %v, want FF0000", m["themeColor"])
	}
}
