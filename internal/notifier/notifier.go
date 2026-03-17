// Package notifier sends drift alert notifications to external webhook services.
// Supports Slack, Discord, Google Chat, Microsoft Teams, and Telegram.
package notifier

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/dataplanelabs/gcplane/internal/reconciler"
)

// Supported webhook formats.
const (
	FormatSlack      = "slack"
	FormatDiscord    = "discord"
	FormatGoogleChat = "googlechat"
	FormatTeams      = "teams"
	FormatTelegram   = "telegram"
)

// WebhookNotifier posts drift alerts to any webhook-compatible service.
// If WebhookURL is empty, NotifyDrift is a no-op.
type WebhookNotifier struct {
	WebhookURL string
	Format     string // slack, discord, googlechat, teams, telegram
	Client     *http.Client
}

// New returns a WebhookNotifier. Format defaults to "slack" if empty.
func New(webhookURL, format string) *WebhookNotifier {
	if format == "" {
		format = FormatSlack
	}
	return &WebhookNotifier{
		WebhookURL: webhookURL,
		Format:     format,
		Client:     &http.Client{Timeout: 15 * time.Second},
	}
}

// NotifyDrift posts a formatted message listing all drifted resources.
// Returns nil immediately when WebhookURL is empty or changes is empty.
func (n *WebhookNotifier) NotifyDrift(ctx context.Context, changes []reconciler.Change) error {
	if n.WebhookURL == "" || len(changes) == 0 {
		return nil
	}

	payload := buildPayload(n.Format, changes)
	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("notifier: marshal payload: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, n.WebhookURL, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("notifier: build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := n.Client.Do(req)
	if err != nil {
		return fmt.Errorf("notifier: send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("notifier: unexpected status %d", resp.StatusCode)
	}
	return nil
}

// buildPayload dispatches to the format-specific payload builder.
func buildPayload(format string, changes []reconciler.Change) any {
	switch format {
	case FormatDiscord:
		return buildDiscordPayload(changes)
	case FormatGoogleChat:
		return buildGoogleChatPayload(changes)
	case FormatTeams:
		return buildTeamsPayload(changes)
	case FormatTelegram:
		return buildTelegramPayload(changes)
	default:
		return buildSlackPayload(changes)
	}
}

// driftSummary builds a markdown-formatted resource list shared across formats.
func driftSummary(changes []reconciler.Change) string {
	var sb strings.Builder
	for _, ch := range changes {
		fmt.Fprintf(&sb, "• **%s/%s** — `%s`\n", ch.Kind, ch.Name, ch.Action)
	}
	return sb.String()
}

func driftTitle(changes []reconciler.Change) string {
	return fmt.Sprintf("GCPlane drift detected (%d resource(s))", len(changes))
}
