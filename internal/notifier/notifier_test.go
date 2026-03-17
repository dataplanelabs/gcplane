package notifier

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

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
