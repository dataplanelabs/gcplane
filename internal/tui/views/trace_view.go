package views

import (
	"fmt"
	"log/slog"
	"sort"
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
	tv.SetBackgroundColor(ColorBase)

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

// formatAttrs renders key=value pairs in muted color, sorted by key.
func (tv *TraceView) formatAttrs(attrs map[string]any) string {
	if len(attrs) == 0 {
		return ""
	}
	keys := make([]string, 0, len(attrs))
	for k := range attrs {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	var parts []string
	for _, k := range keys {
		parts = append(parts, fmt.Sprintf("%s=%v", k, attrs[k]))
	}
	return Tag(HexOverlay0, strings.Join(parts, " "))
}

// SetLevelMin sets the minimum trace level filter.
func (tv *TraceView) SetLevelMin(level slog.Level) { tv.levelMin = level }

// SetPaused sets the pause state.
func (tv *TraceView) SetPaused(paused bool) { tv.paused = paused }

// TogglePause toggles pause and refreshes.
func (tv *TraceView) TogglePause() {
	tv.paused = !tv.paused
	tv.Refresh()
}

// handleInput processes scroll-only keybindings (j/k/g/G).
func (tv *TraceView) handleInput(event *tcell.EventKey) *tcell.EventKey {
	return VimScrollInput(tv.TextView)(event)
}
