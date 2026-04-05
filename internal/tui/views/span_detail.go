package views

import (
	"fmt"
	"strings"

	"github.com/rivo/tview"
)

// SpanDetail displays detailed information about a single span as an overlay.
type SpanDetail struct {
	TextView *tview.TextView
	lastSpan *SpanData // for copy support
}

// NewSpanDetail creates a span detail overlay view.
func NewSpanDetail() *SpanDetail {
	tv := tview.NewTextView().
		SetDynamicColors(true).
		SetScrollable(true).
		SetWordWrap(true)
	tv.SetBackgroundColor(ColorBase)
	tv.SetBorder(true).
		SetBorderColor(ColorOverlay0).
		SetTitle(" Span Detail ").
		SetTitleColor(ColorMauve)

	sd := &SpanDetail{TextView: tv}
	tv.SetInputCapture(VimScrollInput(tv))
	return sd
}

// Name implements View.
func (sd *SpanDetail) Name() string { return "span-detail" }

// Primitive implements View.
func (sd *SpanDetail) Primitive() tview.Primitive { return sd.TextView }

// Activate implements View.
func (sd *SpanDetail) Activate() {}

// Show renders span detail content.
func (sd *SpanDetail) Show(s SpanData) {
	sd.lastSpan = &s
	var b strings.Builder

	// Header
	b.WriteString(fmt.Sprintf("\n %s %s",
		BoldTag(HexMauve, "Span:"),
		Tag(spanTypeHex(s.SpanType), s.SpanType)))
	if s.Name != "" {
		b.WriteString(" — " + Tag(HexText, s.Name))
	}
	b.WriteString("\n")

	// Status line
	icon := spanStatusIcon(s.Status)
	b.WriteString(fmt.Sprintf(" %s %s %s  %s %s",
		Tag(HexOverlay0, "Status:"), icon+" "+s.Status,
		Tag(HexOverlay0, "|"),
		Tag(HexOverlay0, "Duration:"), formatTraceDuration(s.DurationMs)))
	if s.Provider != "" {
		b.WriteString(fmt.Sprintf("  %s %s", Tag(HexOverlay0, "| Provider:"), s.Provider))
	}
	b.WriteString("\n")

	// Model
	if s.Model != "" {
		b.WriteString(fmt.Sprintf(" %s %s\n", Tag(HexOverlay0, "Model:"), Tag(HexBlue, s.Model)))
	}

	// Tokens
	if s.InputTokens > 0 || s.OutputTokens > 0 {
		b.WriteString(fmt.Sprintf(" %s %s in / %s out",
			Tag(HexOverlay0, "Tokens:"),
			Tag(HexText, formatNumber(s.InputTokens)),
			Tag(HexText, formatNumber(s.OutputTokens))))
		if s.Metadata != nil {
			if s.Metadata.CacheReadTokens > 0 {
				b.WriteString(fmt.Sprintf("  %s %s",
					Tag(HexGreen, "cache read:"), formatNumber(s.Metadata.CacheReadTokens)))
			}
			if s.Metadata.CacheCreationTokens > 0 {
				b.WriteString(fmt.Sprintf("  %s %s",
					Tag(HexYellow, "cache write:"), formatNumber(s.Metadata.CacheCreationTokens)))
			}
			if s.Metadata.ThinkingTokens > 0 {
				b.WriteString(fmt.Sprintf("  %s %s",
					Tag(HexPeach, "thinking:"), formatNumber(s.Metadata.ThinkingTokens)))
			}
		}
		b.WriteString("\n")
	}

	// Tool info
	if s.ToolName != "" {
		b.WriteString(fmt.Sprintf(" %s %s\n", Tag(HexOverlay0, "Tool:"), Tag(HexTeal, s.ToolName)))
	}

	b.WriteString("\n")

	// Input preview
	if s.InputPreview != "" {
		b.WriteString(fmt.Sprintf(" %s\n", BoldTag(HexOverlay0, "Input Preview:")))
		b.WriteString(formatPreviewBlock(s.InputPreview))
		b.WriteString("\n")
	}

	// Output preview
	if s.OutputPreview != "" {
		b.WriteString(fmt.Sprintf(" %s\n", BoldTag(HexOverlay0, "Output Preview:")))
		b.WriteString(formatPreviewBlock(s.OutputPreview))
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

// spanTypeHex returns hex color for span type (for tview tags).
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

// formatPreviewBlock wraps preview text with indentation.
func formatPreviewBlock(text string) string {
	lines := strings.Split(text, "\n")
	var b strings.Builder
	for _, line := range lines {
		b.WriteString("   " + Tag(HexSubtext0, line) + "\n")
	}
	return b.String()
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
