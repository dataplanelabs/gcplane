package views

import (
	"fmt"
	"sort"
	"strings"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

// logLevelHex maps GoClaw log levels to Catppuccin hex colors.
var logLevelHex = map[string]string{
	"debug": HexOverlay0,
	"info":  HexGreen,
	"warn":  HexYellow,
	"error": HexRed,
}

// logLevelLabel maps GoClaw log levels to short display labels.
var logLevelLabel = map[string]string{
	"debug": "DBG",
	"info":  "INF",
	"warn":  "WRN",
	"error": "ERR",
}

// logLevelRank maps levels to numeric rank for filtering.
var logLevelRank = map[string]int{
	"debug": 0, "info": 1, "warn": 2, "error": 3,
}

// LogsPanel displays live GoClaw server logs in the bottom panel.
type LogsPanel struct {
	TextView *tview.TextView
	paused   bool
	levelMin string // minimum level to show: "debug", "info", "warn", "error"
}

// NewLogsPanel creates a logs panel view.
func NewLogsPanel() *LogsPanel {
	tv := tview.NewTextView().
		SetDynamicColors(true).
		SetScrollable(true).
		SetWordWrap(false)
	tv.SetBackgroundColor(ColorBase)

	p := &LogsPanel{
		TextView: tv,
		levelMin: "info",
	}
	tv.SetInputCapture(p.handleInput)
	return p
}

// Name implements View.
func (p *LogsPanel) Name() string { return "logs" }

// Primitive implements View.
func (p *LogsPanel) Primitive() tview.Primitive { return p.TextView }

// Activate implements View.
func (p *LogsPanel) Activate() {}

// Refresh renders log entries from LiveStore snapshot (called from tick-based redraw).
func (p *LogsPanel) Refresh(entries []LogEntry) {
	var b strings.Builder
	shown := 0
	minRank := logLevelRank[p.levelMin]

	for _, e := range entries {
		if logLevelRank[e.Level] < minRank {
			continue
		}
		b.WriteString(p.formatEntry(e))
		b.WriteByte('\n')
		shown++
	}

	// Status line
	pauseTag := ""
	if p.paused {
		pauseTag = Tag(HexYellow, "[PAUSED] ")
	}
	status := fmt.Sprintf("\n%s%s %d/%d entries | Level: %s+",
		pauseTag, Tag(HexOverlay0, "---"), shown, len(entries), logLevelLabel[p.levelMin])
	b.WriteString(status)

	p.TextView.SetText(b.String())
	if !p.paused {
		p.TextView.ScrollToEnd()
	}
}

// RefreshUnavailable shows a fallback message when logs.tail is not available.
func (p *LogsPanel) RefreshUnavailable(reason string) {
	p.TextView.SetText(Tag(HexOverlay0, "\n  "+reason))
}

// formatEntry renders a single log entry with Catppuccin colors.
func (p *LogsPanel) formatEntry(e LogEntry) string {
	ts := Tag(HexOverlay0, e.Time.Format("15:04:05.000"))

	hex := logLevelHex[e.Level]
	if hex == "" {
		hex = HexText
	}
	label := logLevelLabel[e.Level]
	if label == "" {
		label = e.Level
	}
	lvl := Tag(hex, label)

	src := ""
	if e.Source != "" {
		src = Tag(HexSky, e.Source) + " "
	}

	attrs := p.formatAttrs(e.Attrs)

	return fmt.Sprintf(" %s %s %s%s %s", ts, lvl, src, Tag(HexText, e.Message), attrs)
}

// formatAttrs renders key=value pairs in muted color, sorted by key.
func (p *LogsPanel) formatAttrs(attrs map[string]string) string {
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
		parts = append(parts, fmt.Sprintf("%s=%s", k, attrs[k]))
	}
	return Tag(HexOverlay0, strings.Join(parts, " "))
}

// SetLevelMin sets the minimum log level filter.
func (p *LogsPanel) SetLevelMin(level string) { p.levelMin = level }

// TogglePause toggles the pause state.
func (p *LogsPanel) TogglePause() { p.paused = !p.paused }

// handleInput processes scroll-only keybindings (j/k/g/G).
func (p *LogsPanel) handleInput(event *tcell.EventKey) *tcell.EventKey {
	return VimScrollInput(p.TextView)(event)
}
