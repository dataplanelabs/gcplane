package views

import (
	"fmt"
	"strings"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

// EventsPanel displays live GoClaw push events in the bottom panel.
type EventsPanel struct {
	TextView *tview.TextView
	paused   bool
	filter   string // filter by event type (empty = all)
}

// NewEventsPanel creates an events panel view.
func NewEventsPanel() *EventsPanel {
	tv := tview.NewTextView().
		SetDynamicColors(true).
		SetScrollable(true).
		SetWordWrap(false)
	tv.SetBackgroundColor(ColorBase)

	p := &EventsPanel{TextView: tv}
	tv.SetInputCapture(p.handleInput)
	return p
}

// Name implements View.
func (p *EventsPanel) Name() string { return "events" }

// Primitive implements View.
func (p *EventsPanel) Primitive() tview.Primitive { return p.TextView }

// Activate implements View.
func (p *EventsPanel) Activate() {}

// Refresh renders live events from LiveStore snapshot (called from tick-based redraw).
func (p *EventsPanel) Refresh(events []LiveEvent) {
	var b strings.Builder
	shown := 0
	for _, e := range events {
		if p.filter != "" && !strings.HasPrefix(e.Type, p.filter) {
			continue
		}
		b.WriteString(p.formatEvent(e))
		b.WriteByte('\n')
		shown++
	}

	// Status line
	pauseTag := ""
	if p.paused {
		pauseTag = Tag(HexYellow, "[PAUSED] ")
	}
	filterInfo := ""
	if p.filter != "" {
		filterInfo = fmt.Sprintf(" | Filter: %s", p.filter)
	}
	status := fmt.Sprintf("\n%s%s %d/%d events%s",
		pauseTag, Tag(HexOverlay0, "---"), shown, len(events), filterInfo)
	b.WriteString(status)

	p.TextView.SetText(b.String())
	if !p.paused {
		p.TextView.ScrollToEnd()
	}
}

// formatEvent renders a single event with colored type tag.
func (p *EventsPanel) formatEvent(e LiveEvent) string {
	ts := Tag(HexOverlay0, e.Time.Format("15:04:05"))
	tag, hex := eventTypeTag(e.Type, e.Subtype)
	return fmt.Sprintf(" %s %s %s", ts, Tag(hex, tag), Tag(HexText, e.Summary))
}

// eventTypeTag returns a 3-char tag and hex color for an event type.
func eventTypeTag(typ, subtype string) (string, string) {
	switch typ {
	case "agent":
		if strings.HasPrefix(subtype, "tool.") {
			return "AGT", HexSky
		}
		if subtype == "run.failed" || subtype == "run.cancelled" {
			return "AGT", HexRed
		}
		return "AGT", HexMauve
	case "chat":
		return "CHT", HexTeal
	case "health":
		return "HLT", HexGreen
	case "cron":
		return "CRN", HexYellow
	case "trace.updated":
		return "TRC", HexBlue
	case "session.updated":
		return "SES", HexLavender
	case "presence":
		return "USR", HexPeach
	case "heartbeat":
		return "HRT", HexOverlay0
	case "shutdown":
		return "SHT", HexRed
	default:
		if strings.HasPrefix(typ, "team.") {
			return "TEM", HexPink
		}
		return "EVT", HexOverlay0
	}
}

// SetFilter sets the event type filter (empty = all).
func (p *EventsPanel) SetFilter(f string) { p.filter = f }

// TogglePause toggles the pause state.
func (p *EventsPanel) TogglePause() { p.paused = !p.paused }

// handleInput processes scroll-only keybindings (j/k/g/G).
func (p *EventsPanel) handleInput(event *tcell.EventKey) *tcell.EventKey {
	return VimScrollInput(p.TextView)(event)
}
