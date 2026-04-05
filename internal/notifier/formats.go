package notifier

import (
	"fmt"
	"strings"

	"github.com/dataplanelabs/gcplane/internal/controller"
	"github.com/dataplanelabs/gcplane/internal/reconciler"
)

// --- Slack (Block Kit) ---

func buildSlackPayload(changes []reconciler.Change) any {
	return map[string]any{
		"blocks": []map[string]any{
			{"type": "header", "text": map[string]any{"type": "plain_text", "text": driftTitle(changes)}},
			{"type": "section", "text": map[string]any{"type": "mrkdwn", "text": slackSummary(changes)}},
		},
	}
}

func slackSummary(changes []reconciler.Change) string {
	// Slack uses *bold* not **bold**
	var s string
	for _, ch := range changes {
		s += fmt.Sprintf("• *%s/%s* — `%s`\n", ch.Kind, ch.Name, ch.Action)
	}
	return s
}

// --- Discord (Embed) ---

func buildDiscordPayload(changes []reconciler.Change) any {
	return map[string]any{
		"embeds": []map[string]any{
			{
				"title":       driftTitle(changes),
				"description": driftSummary(changes),
				"color":       16750848, // orange
			},
		},
	}
}

// --- Google Chat (Card v2) ---

func buildGoogleChatPayload(changes []reconciler.Change) any {
	return map[string]any{
		"cardsV2": []map[string]any{
			{
				"cardId": "drift-alert",
				"card": map[string]any{
					"header": map[string]any{"title": driftTitle(changes)},
					"sections": []map[string]any{
						{
							"widgets": []map[string]any{
								{"textParagraph": map[string]any{"text": driftSummary(changes)}},
							},
						},
					},
				},
			},
		},
	}
}

// --- Microsoft Teams (MessageCard) ---

func buildTeamsPayload(changes []reconciler.Change) any {
	return map[string]any{
		"@type":      "MessageCard",
		"@context":   "http://schema.org/extensions",
		"summary":    driftTitle(changes),
		"themeColor": "FF8C00", // orange
		"title":      driftTitle(changes),
		"sections": []map[string]any{
			{"text": driftSummary(changes)},
		},
	}
}

// --- Telegram (sendMessage) ---

func buildTelegramPayload(changes []reconciler.Change) any {
	text := fmt.Sprintf("*%s*\n\n%s", driftTitle(changes), driftSummary(changes))
	return map[string]any{
		"text":       text,
		"parse_mode": "Markdown",
	}
}

// --- Provider Verify Failure Formats ---

func buildVerifyFailurePayload(format string, failures []controller.ProviderVerifyFailure) any {
	switch format {
	case FormatDiscord:
		return buildDiscordVerifyPayload(failures)
	case FormatGoogleChat:
		return buildGoogleChatVerifyPayload(failures)
	case FormatTeams:
		return buildTeamsVerifyPayload(failures)
	case FormatTelegram:
		return buildTelegramVerifyPayload(failures)
	default:
		return buildSlackVerifyPayload(failures)
	}
}

func verifyFailureTitle(failures []controller.ProviderVerifyFailure) string {
	return fmt.Sprintf("GCPlane provider key verification FAILED (%d provider(s))", len(failures))
}

func verifyFailureSummary(failures []controller.ProviderVerifyFailure) string {
	var sb strings.Builder
	for _, f := range failures {
		fmt.Fprintf(&sb, "• **%s** — %s\n", f.Name, f.Error)
	}
	return sb.String()
}

func buildSlackVerifyPayload(failures []controller.ProviderVerifyFailure) any {
	var summary string
	for _, f := range failures {
		summary += fmt.Sprintf("• *%s* — %s\n", f.Name, f.Error)
	}
	return map[string]any{
		"blocks": []map[string]any{
			{"type": "header", "text": map[string]any{"type": "plain_text", "text": verifyFailureTitle(failures)}},
			{"type": "section", "text": map[string]any{"type": "mrkdwn", "text": summary}},
		},
	}
}

func buildDiscordVerifyPayload(failures []controller.ProviderVerifyFailure) any {
	return map[string]any{
		"embeds": []map[string]any{
			{
				"title":       verifyFailureTitle(failures),
				"description": verifyFailureSummary(failures),
				"color":       15158332, // red
			},
		},
	}
}

func buildGoogleChatVerifyPayload(failures []controller.ProviderVerifyFailure) any {
	return map[string]any{
		"cardsV2": []map[string]any{
			{
				"cardId": "verify-alert",
				"card": map[string]any{
					"header": map[string]any{"title": verifyFailureTitle(failures)},
					"sections": []map[string]any{
						{
							"widgets": []map[string]any{
								{"textParagraph": map[string]any{"text": verifyFailureSummary(failures)}},
							},
						},
					},
				},
			},
		},
	}
}

func buildTeamsVerifyPayload(failures []controller.ProviderVerifyFailure) any {
	return map[string]any{
		"@type":      "MessageCard",
		"@context":   "http://schema.org/extensions",
		"summary":    verifyFailureTitle(failures),
		"themeColor": "FF0000",
		"title":      verifyFailureTitle(failures),
		"sections": []map[string]any{
			{"text": verifyFailureSummary(failures)},
		},
	}
}

func buildTelegramVerifyPayload(failures []controller.ProviderVerifyFailure) any {
	text := fmt.Sprintf("*%s*\n\n%s", verifyFailureTitle(failures), verifyFailureSummary(failures))
	return map[string]any{
		"text":       text,
		"parse_mode": "Markdown",
	}
}

