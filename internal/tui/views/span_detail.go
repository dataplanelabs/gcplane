package views

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/rivo/tview"
)

// SpanDetail displays detailed information about a single span.
type SpanDetail struct {
	TextView *tview.TextView
	lastSpan *SpanData // for copy support
}

// NewSpanDetail creates a span detail view.
func NewSpanDetail() *SpanDetail {
	tv := tview.NewTextView().
		SetDynamicColors(true).
		SetScrollable(true).
		SetWordWrap(true)
	tv.SetBackgroundColor(ColorBase)
	tv.SetInputCapture(VimScrollInput(tv))

	return &SpanDetail{TextView: tv}
}

// Name implements View.
func (sd *SpanDetail) Name() string { return "span-detail" }

// Primitive implements View.
func (sd *SpanDetail) Primitive() tview.Primitive { return sd.TextView }

// Activate implements View.
func (sd *SpanDetail) Activate() {}

// Show renders span detail content with JSON-formatted previews.
func (sd *SpanDetail) Show(s SpanData) {
	sd.lastSpan = &s
	var b strings.Builder

	// Header
	typeHex := spanTypeHex(s.SpanType)
	b.WriteString(fmt.Sprintf("\n %s %s",
		BoldTag(HexMauve, "Span:"),
		Tag(typeHex, s.SpanType)))
	if s.Name != "" {
		b.WriteString(" — " + Tag(HexText, s.Name))
	}
	b.WriteString("\n")

	// Status + Duration
	icon := spanStatusIcon(s.Status)
	b.WriteString(fmt.Sprintf(" %s %s  %s %s",
		Tag(HexOverlay0, "Status:"), icon+" "+s.Status,
		Tag(HexOverlay0, "Duration:"), formatTraceDuration(s.DurationMs)))
	if s.Provider != "" {
		b.WriteString(fmt.Sprintf("  %s %s", Tag(HexOverlay0, "Provider:"), s.Provider))
	}
	b.WriteString("\n")

	// Model
	if s.Model != "" {
		b.WriteString(fmt.Sprintf(" %s %s\n", Tag(HexOverlay0, "Model:"), Tag(HexBlue, s.Model)))
	}

	// Tool
	if s.ToolName != "" {
		b.WriteString(fmt.Sprintf(" %s %s\n", Tag(HexOverlay0, "Tool:"), Tag(HexTeal, s.ToolName)))
	}

	// Tokens
	if s.InputTokens > 0 || s.OutputTokens > 0 {
		b.WriteString(fmt.Sprintf(" %s %s in / %s out",
			Tag(HexOverlay0, "Tokens:"),
			formatNumber(s.InputTokens),
			formatNumber(s.OutputTokens)))
		if s.Metadata != nil {
			if s.Metadata.CacheReadTokens > 0 {
				b.WriteString(fmt.Sprintf("  %s %s",
					Tag(HexGreen, "cache-r:"), formatNumber(s.Metadata.CacheReadTokens)))
			}
			if s.Metadata.CacheCreationTokens > 0 {
				b.WriteString(fmt.Sprintf("  %s %s",
					Tag(HexYellow, "cache-w:"), formatNumber(s.Metadata.CacheCreationTokens)))
			}
			if s.Metadata.ThinkingTokens > 0 {
				b.WriteString(fmt.Sprintf("  %s %s",
					Tag(HexPeach, "think:"), formatNumber(s.Metadata.ThinkingTokens)))
			}
		}
		b.WriteString("\n")
	}

	// ID
	b.WriteString(fmt.Sprintf(" %s %s\n", Tag(HexOverlay0, "ID:"), Tag(HexOverlay0, s.ID)))

	b.WriteString("\n")

	// Input preview
	if s.InputPreview != "" {
		b.WriteString(fmt.Sprintf(" %s\n", BoldTag(HexOverlay0, "Input:")))
		b.WriteString(formatPreview(s.InputPreview))
		b.WriteString("\n")
	}

	// Output preview
	if s.OutputPreview != "" {
		b.WriteString(fmt.Sprintf(" %s\n", BoldTag(HexOverlay0, "Output:")))
		b.WriteString(formatPreview(s.OutputPreview))
		b.WriteString("\n")
	}

	// Error
	if s.Error != "" {
		b.WriteString(fmt.Sprintf(" %s\n", BoldTag(HexRed, "Error:")))
		b.WriteString("   " + Tag(HexRed, s.Error) + "\n")
	}

	sd.TextView.SetText(b.String())
	sd.TextView.ScrollToBeginning()
}

// formatPreview tries to pretty-print JSON, falls back to indented text.
func formatPreview(text string) string {
	// Try JSON pretty-print
	text = strings.TrimSpace(text)
	if (strings.HasPrefix(text, "{") || strings.HasPrefix(text, "[")) {
		var parsed any
		if err := json.Unmarshal([]byte(text), &parsed); err == nil {
			pretty, err := json.MarshalIndent(parsed, "   ", "  ")
			if err == nil {
				return indentBlock(string(pretty), HexSubtext0)
			}
		}
	}
	// Plain text fallback
	return indentBlock(text, HexSubtext0)
}

// indentBlock wraps each line with indentation and color.
func indentBlock(text string, hex string) string {
	lines := strings.Split(text, "\n")
	var b strings.Builder
	for _, line := range lines {
		b.WriteString("   " + Tag(hex, line) + "\n")
	}
	return b.String()
}

// spanTypeHex returns hex color for span type.
func spanTypeHex(spanType string) string {
	switch spanType {
	case "agent":
		return HexMauve
	case "llm_call":
		return HexBlue
	case "tool_call":
		return HexTeal
	default:
		return HexOverlay0
	}
}

// formatNumber formats an integer with comma separators.
func formatNumber(n int) string {
	if n < 1000 {
		return fmt.Sprintf("%d", n)
	}
	s := fmt.Sprintf("%d", n)
	var result []byte
	for i, c := range s {
		if i > 0 && (len(s)-i)%3 == 0 {
			result = append(result, ',')
		}
		result = append(result, byte(c))
	}
	return string(result)
}

// formatSpanCopyText returns a clipboard-friendly text for a span.
func formatSpanCopyText(s SpanData) string {
	name := s.Name
	if s.SpanType == "llm_call" && s.Model != "" {
		name = s.Model
	} else if s.SpanType == "tool_call" && s.ToolName != "" {
		name = s.ToolName
	}
	return fmt.Sprintf("%s: %s (%s) %s", s.SpanType, name, formatTraceDuration(s.DurationMs), s.ID)
}
