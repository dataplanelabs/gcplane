package notifier

import (
	"fmt"

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
