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

// LogsPanel displays live GoClaw server logs with selectable rows.
type LogsPanel struct {
	flex      *tview.Flex
	Table     *tview.Table
	statusBar *tview.TextView
	OnCopy    func(text string)
	paused    bool
	levelMin  string     // minimum level to show
	msgFilter string     // search filter on message text
	entries   []LogEntry // current visible entries backing the table
}

// NewLogsPanel creates a logs panel view with a selectable table.
func NewLogsPanel() *LogsPanel {
	table := tview.NewTable().
		SetSelectable(true, false).
		SetFixed(1, 0).
		SetSeparator(' ')
	table.SetBackgroundColor(ColorBase)
	table.SetSelectedStyle(tcell.StyleDefault.
		Background(ColorSurface0).
		Foreground(ColorText))

	statusBar := tview.NewTextView().
		SetDynamicColors(true).
		SetTextAlign(tview.AlignLeft)
	statusBar.SetBackgroundColor(ColorBase)

	flex := tview.NewFlex().SetDirection(tview.FlexRow).
		AddItem(table, 0, 1, true).
		AddItem(statusBar, 1, 0, false)

	p := &LogsPanel{
		flex:      flex,
		Table:     table,
		statusBar: statusBar,
		levelMin:  "info",
	}
	table.SetInputCapture(p.handleInput)
	return p
}

// Name implements View.
func (p *LogsPanel) Name() string { return "logs" }

// Primitive implements View.
func (p *LogsPanel) Primitive() tview.Primitive { return p.flex }

// Activate implements View.
func (p *LogsPanel) Activate() {}

// Refresh renders log entries from LiveStore snapshot.
func (p *LogsPanel) Refresh(entries []LogEntry) {
	p.Table.Clear()
	p.renderHeader()

	minRank := logLevelRank[p.levelMin]
	p.entries = p.entries[:0]

	for _, e := range entries {
		if logLevelRank[e.Level] < minRank {
			continue
		}
		if p.msgFilter != "" && !strings.Contains(
			strings.ToLower(e.Message+" "+e.Source), p.msgFilter) {
			continue
		}
		p.entries = append(p.entries, e)
	}

	if len(p.entries) == 0 {
		p.Table.SetCell(1, 0, tview.NewTableCell(
			Tag(HexOverlay0, "  No log entries")).
			SetExpansion(1).SetSelectable(false))
	} else {
		for i, e := range p.entries {
			p.renderRow(i+1, e)
		}
	}

	// Auto-scroll to bottom if not paused
	if !p.paused && len(p.entries) > 0 {
		p.Table.Select(len(p.entries), 0)
	}

	p.updateStatusBar(len(p.entries), len(entries))
}

// RefreshUnavailable shows a fallback message when logs.tail is not available.
func (p *LogsPanel) RefreshUnavailable(reason string) {
	p.Table.Clear()
	p.renderHeader()
	p.Table.SetCell(1, 0, tview.NewTableCell(
		Tag(HexOverlay0, "  "+reason)).
		SetExpansion(1).SetSelectable(false))
}

// renderHeader draws the fixed header row.
func (p *LogsPanel) renderHeader() {
	headers := []struct {
		text      string
		expansion int
		width     int
	}{
		{"Time", 0, 14},
		{"Level", 0, 6},
		{"Source", 0, 16},
		{"Message", 1, 0},
	}
	for col, h := range headers {
		cell := tview.NewTableCell(" " + h.text).
			SetTextColor(ColorSubtext0).
			SetSelectable(false).
			SetBackgroundColor(ColorMantle)
		if h.expansion > 0 {
			cell.SetExpansion(h.expansion)
		} else {
			cell.SetMaxWidth(h.width)
		}
		p.Table.SetCell(0, col, cell)
	}
}

// renderRow draws a single log entry row.
func (p *LogsPanel) renderRow(row int, e LogEntry) {
	// Time
	p.Table.SetCell(row, 0, tview.NewTableCell(" "+e.Time.Format("15:04:05.000")).
		SetTextColor(ColorOverlay0).SetMaxWidth(14))

	// Level
	hex := logLevelHex[e.Level]
	if hex == "" {
		hex = HexText
	}
	label := logLevelLabel[e.Level]
	if label == "" {
		label = e.Level
	}
	color, _ := colorFromHex(hex)
	p.Table.SetCell(row, 1, tview.NewTableCell(label).
		SetTextColor(color).SetMaxWidth(6))

	// Source
	src := e.Source
	if len(src) > 14 {
		src = src[:11] + "..."
	}
	p.Table.SetCell(row, 2, tview.NewTableCell(src).
		SetTextColor(ColorSky).SetMaxWidth(16))

	// Message + attrs
	msg := e.Message
	attrs := formatLogAttrs(e.Attrs)
	if attrs != "" {
		msg += " " + attrs
	}
	p.Table.SetCell(row, 3, tview.NewTableCell(msg).
		SetTextColor(ColorText).SetExpansion(1))
}

// SelectedEntry returns the text of the currently selected log row.
func (p *LogsPanel) SelectedEntry() string {
	row, _ := p.Table.GetSelection()
	idx := row - 1 // header offset
	if idx < 0 || idx >= len(p.entries) {
		return ""
	}
	e := p.entries[idx]
	attrs := formatLogAttrs(e.Attrs)
	s := fmt.Sprintf("%s %s %s %s", e.Time.Format("15:04:05.000"), e.Level, e.Source, e.Message)
	if attrs != "" {
		s += " " + attrs
	}
	return s
}

// SetLevelMin sets the minimum log level filter.
func (p *LogsPanel) SetLevelMin(level string) { p.levelMin = level }

// SetFilter sets a message search filter (case-insensitive).
func (p *LogsPanel) SetFilter(f string) { p.msgFilter = strings.ToLower(f) }

// TogglePause toggles the pause state.
func (p *LogsPanel) TogglePause() {
	p.paused = !p.paused
}

// GoToTop selects the first log row.
func (p *LogsPanel) GoToTop() { p.Table.Select(1, 0) }

// GoToBottom selects the last log row.
func (p *LogsPanel) GoToBottom() {
	if rc := p.Table.GetRowCount(); rc > 1 {
		p.Table.Select(rc-1, 0)
	}
}

// handleInput processes Ctrl+D/Ctrl+U for half-page scroll.
// j/k/gg/G/yy are handled by the global KeyHandler.
func (p *LogsPanel) handleInput(event *tcell.EventKey) *tcell.EventKey {
	if event.Key() == tcell.KeyCtrlD {
		row, col := p.Table.GetSelection()
		_, _, _, height := p.Table.GetInnerRect()
		newRow := row + height/2
		if max := p.Table.GetRowCount() - 1; newRow > max {
			newRow = max
		}
		p.Table.Select(newRow, col)
		return nil
	}
	if event.Key() == tcell.KeyCtrlU {
		row, col := p.Table.GetSelection()
		_, _, _, height := p.Table.GetInnerRect()
		newRow := row - height/2
		if newRow < 1 {
			newRow = 1
		}
		p.Table.Select(newRow, col)
		return nil
	}
	return event
}

// updateStatusBar renders the status bar with current state.
func (p *LogsPanel) updateStatusBar(shown, total int) {
	pauseTag := ""
	if p.paused {
		pauseTag = Tag(HexYellow, "[PAUSED] ")
	}
	status := fmt.Sprintf(" %s%s %d/%d entries | Level: %s+",
		pauseTag, Tag(HexOverlay0, "---"), shown, total, logLevelLabel[p.levelMin])
	p.statusBar.SetText(status)
}

// formatLogAttrs renders key=value pairs sorted by key.
func formatLogAttrs(attrs map[string]string) string {
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
		parts = append(parts, k+"="+attrs[k])
	}
	return strings.Join(parts, " ")
}

// colorFromHex converts a hex color string to tcell.Color.
func colorFromHex(hex string) (tcell.Color, bool) {
	switch hex {
	case HexOverlay0:
		return ColorOverlay0, true
	case HexGreen:
		return ColorGreen, true
	case HexYellow:
		return ColorYellow, true
	case HexRed:
		return ColorRed, true
	case HexBlue:
		return ColorBlue, true
	case HexText:
		return ColorText, true
	default:
		return ColorText, false
	}
}
