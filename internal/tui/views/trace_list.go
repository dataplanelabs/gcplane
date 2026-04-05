package views

import (
	"fmt"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

// TraceList displays a table of LLM agent traces.
type TraceList struct {
	Table    *tview.Table
	OnSelect func(traceID string) // called when user selects/navigates to a row
	OnCopy   func(text string)    // called when user presses y to copy trace ID
	traces   []TraceData          // current data backing the table
}

// NewTraceList creates a trace list table component.
func NewTraceList() *TraceList {
	table := tview.NewTable().
		SetSelectable(true, false).
		SetFixed(1, 0). // fixed header row
		SetSeparator(' ')
	table.SetBackgroundColor(ColorBase)
	table.SetSelectedStyle(tcell.StyleDefault.
		Background(ColorSurface0).
		Foreground(ColorText))

	tl := &TraceList{Table: table}
	table.SetSelectionChangedFunc(tl.onSelectionChanged)
	table.SetInputCapture(tl.handleInput)
	return tl
}

// Refresh rebuilds the table from trace data, preserving selection.
func (tl *TraceList) Refresh(traces []TraceData, selectedID string) {
	tl.traces = traces
	tl.Table.Clear()
	tl.renderHeader()

	if len(traces) == 0 {
		tl.Table.SetCell(1, 0, tview.NewTableCell(
			Tag(HexOverlay0, "  No traces")).
			SetExpansion(1).SetSelectable(false))
		return
	}

	selectedRow := 1 // default to first data row
	for i, tr := range traces {
		row := i + 1
		tl.renderRow(row, tr)
		if tr.ID == selectedID {
			selectedRow = row
		}
	}

	tl.Table.Select(selectedRow, 0)
}

// SelectedTraceID returns the trace ID of the currently selected row.
func (tl *TraceList) SelectedTraceID() string {
	row, _ := tl.Table.GetSelection()
	idx := row - 1 // header offset
	if idx < 0 || idx >= len(tl.traces) {
		return ""
	}
	return tl.traces[idx].ID
}

// renderHeader draws the fixed header row.
func (tl *TraceList) renderHeader() {
	headers := []struct {
		text      string
		expansion int
		width     int
	}{
		{"Name", 1, 0},
		{"Status", 0, 10},
		{"Duration", 0, 10},
		{"Tokens", 0, 14},
		{"Spans", 0, 7},
		{"Time", 0, 10},
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
		tl.Table.SetCell(0, col, cell)
	}
}

// renderRow draws a single trace row.
func (tl *TraceList) renderRow(row int, tr TraceData) {
	// Name — agent name or trace name
	name := tr.Name
	if name == "" {
		name = tr.AgentID
	}
	if len(name) > 30 {
		name = name[:27] + "..."
	}
	tl.Table.SetCell(row, 0, tview.NewTableCell(" "+name).
		SetTextColor(ColorText).SetExpansion(1))

	// Status
	sym, color := traceStatusCell(tr.Status)
	tl.Table.SetCell(row, 1, tview.NewTableCell(sym+" "+tr.Status).
		SetTextColor(color).SetMaxWidth(10))

	// Duration
	dur := formatTraceDuration(tr.DurationMs)
	tl.Table.SetCell(row, 2, tview.NewTableCell(dur).
		SetTextColor(ColorSubtext0).SetMaxWidth(10))

	// Tokens
	tok := formatTokens(tr.TotalInputTokens, tr.TotalOutputTokens, tr.Metadata)
	tl.Table.SetCell(row, 3, tview.NewTableCell(tok).
		SetTextColor(ColorText).SetMaxWidth(14))

	// Spans
	spans := fmt.Sprintf("%d", tr.SpanCount)
	tl.Table.SetCell(row, 4, tview.NewTableCell(spans).
		SetTextColor(ColorSubtext0).SetMaxWidth(7))

	// Time
	ts := tr.StartTime.Format("15:04:05")
	tl.Table.SetCell(row, 5, tview.NewTableCell(ts).
		SetTextColor(ColorOverlay0).SetMaxWidth(10))
}

// handleInput processes vim keybindings on the trace list.
func (tl *TraceList) handleInput(event *tcell.EventKey) *tcell.EventKey {
	switch event.Rune() {
	case 'j':
		return tcell.NewEventKey(tcell.KeyDown, 0, tcell.ModNone)
	case 'k':
		return tcell.NewEventKey(tcell.KeyUp, 0, tcell.ModNone)
	case 'g':
		tl.Table.Select(1, 0)
		return nil
	case 'G':
		if rowCount := tl.Table.GetRowCount(); rowCount > 1 {
			tl.Table.Select(rowCount-1, 0)
		}
		return nil
	case 'y':
		if id := tl.SelectedTraceID(); id != "" && tl.OnCopy != nil {
			tl.OnCopy(id)
		}
		return nil
	}

	// Ctrl+D: half-page down
	if event.Key() == tcell.KeyCtrlD {
		row, col := tl.Table.GetSelection()
		_, _, _, height := tl.Table.GetInnerRect()
		newRow := row + height/2
		if max := tl.Table.GetRowCount() - 1; newRow > max {
			newRow = max
		}
		tl.Table.Select(newRow, col)
		return nil
	}
	// Ctrl+U: half-page up
	if event.Key() == tcell.KeyCtrlU {
		row, col := tl.Table.GetSelection()
		_, _, _, height := tl.Table.GetInnerRect()
		newRow := row - height/2
		if newRow < 1 {
			newRow = 1 // skip header
		}
		tl.Table.Select(newRow, col)
		return nil
	}

	return event
}

// onSelectionChanged fires OnSelect callback with the selected trace ID.
func (tl *TraceList) onSelectionChanged(row, _ int) {
	idx := row - 1
	if idx < 0 || idx >= len(tl.traces) || tl.OnSelect == nil {
		return
	}
	tl.OnSelect(tl.traces[idx].ID)
}

// traceStatusCell returns a Unicode symbol and color for a trace status.
func traceStatusCell(status string) (string, tcell.Color) {
	switch status {
	case "ok", "success", "completed":
		return "\u25cf", ColorGreen // ●
	case "error", "failed":
		return "\u2717", ColorRed // ✗
	case "running", "pending":
		return "\u25d0", ColorYellow // ◐
	default:
		return "\u25cb", ColorOverlay0 // ○
	}
}

// formatTraceDuration formats milliseconds into a human-friendly string.
func formatTraceDuration(ms int) string {
	if ms <= 0 {
		return "—"
	}
	if ms < 1000 {
		return fmt.Sprintf("%dms", ms)
	}
	secs := float64(ms) / 1000.0
	if secs < 60 {
		return fmt.Sprintf("%.1fs", secs)
	}
	mins := int(secs) / 60
	remSecs := int(secs) % 60
	return fmt.Sprintf("%dm%ds", mins, remSecs)
}

// formatTokens formats input/output tokens compactly.
func formatTokens(input, output int, meta *TraceMetadata) string {
	in := compactNumber(input)
	out := compactNumber(output)
	s := fmt.Sprintf("%s/%s", in, out)
	if meta != nil && meta.TotalCacheReadTokens > 0 {
		s += fmt.Sprintf(" [%s]+%s[-]", HexGreen, compactNumber(meta.TotalCacheReadTokens))
	}
	return s
}

// compactNumber formats a number compactly: <1000 as-is, else Xk.
func compactNumber(n int) string {
	if n < 1000 {
		return fmt.Sprintf("%d", n)
	}
	return fmt.Sprintf("%dk", n/1000)
}
