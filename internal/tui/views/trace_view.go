package views

import (
	"fmt"
	"log/slog"
	"strings"

	"github.com/dataplanelabs/gcplane/internal/tui/trace"
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

// levelHex maps slog levels to Catppuccin hex colors.
var levelHex = map[slog.Level]string{
	slog.LevelDebug: HexOverlay0,
	slog.LevelInfo:  HexGreen,
	slog.LevelWarn:  HexYellow,
	slog.LevelError: HexRed,
}

// levelLabel maps slog levels to short display labels.
var levelLabel = map[slog.Level]string{
	slog.LevelDebug: "DBG",
	slog.LevelInfo:  "INF",
	slog.LevelWarn:  "WRN",
	slog.LevelError: "ERR",
}

// TraceView displays a scrollable, filterable log of trace events.
type TraceView struct {
	TextView *tview.TextView
	handler  *trace.RingHandler
	paused   bool
	levelMin slog.Level // show entries >= this level
	search   string     // substring filter on message
}

// NewTraceView creates a trace view backed by the given ring handler.
func NewTraceView(handler *trace.RingHandler) *TraceView {
	tv := tview.NewTextView().
		SetDynamicColors(true).
		SetScrollable(true).
		SetWordWrap(false)
	tv.SetBorder(true).SetTitle(" Trace (t to close) ")
	tv.SetBorderColor(ColorSurface1)
	tv.SetTitleColor(ColorMauve)

	traceView := &TraceView{
		TextView: tv,
		handler:  handler,
		levelMin: slog.LevelDebug,
	}

	tv.SetInputCapture(traceView.handleInput)
	return traceView
}

// Name implements View.
func (tv *TraceView) Name() string { return "trace" }

// Primitive implements View.
func (tv *TraceView) Primitive() tview.Primitive { return tv.TextView }

// Activate implements View.
func (tv *TraceView) Activate() { tv.Refresh() }

// Refresh re-renders the trace from the ring buffer snapshot.
func (tv *TraceView) Refresh() {
	entries := tv.handler.Snapshot()

	var b strings.Builder
	shown := 0
	for _, e := range entries {
		if e.Level < tv.levelMin {
			continue
		}
		if tv.search != "" && !strings.Contains(strings.ToLower(e.Message), strings.ToLower(tv.search)) {
			continue
		}
		b.WriteString(tv.formatEntry(e))
		b.WriteByte('\n')
		shown++
	}

	// Status line
	pauseTag := ""
	if tv.paused {
		pauseTag = Tag(HexYellow, "[PAUSED] ")
	}
	filterInfo := ""
	if tv.search != "" {
		filterInfo = fmt.Sprintf(" | Filter: %q", tv.search)
	}
	status := fmt.Sprintf("\n%s%s %d/%d entries | Level: %s+%s",
		pauseTag,
		Tag(HexOverlay0, "---"),
		shown, tv.handler.Count(),
		levelLabel[tv.levelMin],
		filterInfo)
	b.WriteString(status)

	tv.TextView.SetText(b.String())
	if !tv.paused {
		tv.TextView.ScrollToEnd()
	}
}

// formatEntry renders a single trace entry with colors.
func (tv *TraceView) formatEntry(e trace.Entry) string {
	ts := Tag(HexOverlay0, e.Time.Format("15:04:05.000"))

	lvlHex := levelHex[e.Level]
	if lvlHex == "" {
		lvlHex = HexText
	}
	lvlStr := levelLabel[e.Level]
	if lvlStr == "" {
		lvlStr = e.Level.String()
	}
	lvl := Tag(lvlHex, lvlStr)

	// Message prefix tag based on content
	prefix := tv.messagePrefix(e.Message)

	// Format attrs
	attrs := tv.formatAttrs(e.Attrs)

	return fmt.Sprintf(" %s %s %s%s %s", ts, lvl, prefix, Tag(HexText, e.Message), attrs)
}

// messagePrefix returns a colored category tag for known message patterns.
func (tv *TraceView) messagePrefix(msg string) string {
	lower := strings.ToLower(msg)
	switch {
	case strings.HasPrefix(lower, "observing") || strings.HasPrefix(lower, "observe"):
		return Tag(HexBlue, "OBS") + " "
	case strings.HasPrefix(lower, "creating") || strings.HasPrefix(lower, "updating"):
		return Tag(HexGreen, "ACT") + " "
	case strings.HasPrefix(lower, "pruning"):
		return Tag(HexYellow, "PRN") + " "
	case strings.Contains(lower, "failed") || strings.Contains(lower, "error"):
		return Tag(HexRed, "ERR") + " "
	case strings.HasPrefix(lower, "api.") || strings.HasPrefix(lower, "ws."):
		return Tag(HexSky, "API") + " "
	default:
		return ""
	}
}

// formatAttrs renders key=value pairs in muted color.
func (tv *TraceView) formatAttrs(attrs map[string]any) string {
	if len(attrs) == 0 {
		return ""
	}
	var parts []string
	for k, v := range attrs {
		parts = append(parts, fmt.Sprintf("%s=%v", k, v))
	}
	return Tag(HexOverlay0, strings.Join(parts, " "))
}

// handleInput processes trace-specific keybindings.
func (tv *TraceView) handleInput(event *tcell.EventKey) *tcell.EventKey {
	switch event.Rune() {
	case 'j':
		row, col := tv.TextView.GetScrollOffset()
		tv.TextView.ScrollTo(row+1, col)
		return nil
	case 'k':
		row, col := tv.TextView.GetScrollOffset()
		if row > 0 {
			tv.TextView.ScrollTo(row-1, col)
		}
		return nil
	case 'g':
		tv.TextView.ScrollToBeginning()
		return nil
	case 'G':
		tv.TextView.ScrollToEnd()
		return nil
	case ' ':
		tv.paused = !tv.paused
		tv.Refresh()
		return nil
	case '1':
		tv.levelMin = slog.LevelDebug
		tv.Refresh()
		return nil
	case '2':
		tv.levelMin = slog.LevelInfo
		tv.Refresh()
		return nil
	case '3':
		tv.levelMin = slog.LevelWarn
		tv.Refresh()
		return nil
	case '4':
		tv.levelMin = slog.LevelError
		tv.Refresh()
		return nil
	case 'c':
		// Clear is visual only — buffer still holds entries, but we reset the view
		tv.search = ""
		tv.levelMin = slog.LevelDebug
		tv.Refresh()
		return nil
	}
	return event
}
