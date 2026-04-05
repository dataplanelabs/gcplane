package views

import (
	"fmt"
	"strings"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

// SpanList displays a flat table of spans within a trace for drill-down navigation.
type SpanList struct {
	Table  *tview.Table
	OnCopy func(text string)
	spans  []SpanData
	filter string
}

// NewSpanList creates a span list table component.
func NewSpanList() *SpanList {
	table := tview.NewTable().
		SetSelectable(true, false).
		SetFixed(1, 0).
		SetSeparator(' ')
	table.SetBackgroundColor(ColorBase)
	table.SetSelectedStyle(tcell.StyleDefault.
		Background(ColorSurface0).
		Foreground(ColorText))

	return &SpanList{Table: table}
}

// Refresh rebuilds the table from span data.
func (sl *SpanList) Refresh(spans []SpanData) {
	sl.spans = sl.spans[:0]
	sl.Table.Clear()
	sl.renderHeader()

	for _, s := range spans {
		if sl.filter != "" && !sl.matchesFilter(s) {
			continue
		}
		sl.spans = append(sl.spans, s)
	}

	if len(sl.spans) == 0 {
		sl.Table.SetCell(1, 0, tview.NewTableCell(
			Tag(HexOverlay0, "  No spans")).
			SetExpansion(1).SetSelectable(false))
		return
	}

	for i, s := range sl.spans {
		sl.renderRow(i+1, s)
	}
	sl.Table.Select(1, 0)
}

// SelectedSpan returns the currently selected span data.
func (sl *SpanList) SelectedSpan() *SpanData {
	row, _ := sl.Table.GetSelection()
	idx := row - 1
	if idx < 0 || idx >= len(sl.spans) {
		return nil
	}
	return &sl.spans[idx]
}

// SelectedText returns a copy-friendly text for the selected span.
func (sl *SpanList) SelectedText() string {
	s := sl.SelectedSpan()
	if s == nil {
		return ""
	}
	name := s.Name
	if s.SpanType == "llm_call" && s.Model != "" {
		name = s.Model
	} else if s.SpanType == "tool_call" && s.ToolName != "" {
		name = s.ToolName
	}
	return fmt.Sprintf("%s %s (%s) %s", s.SpanType, name, formatTraceDuration(s.DurationMs), s.ID)
}

// SetFilter sets a search filter string (case-insensitive).
func (sl *SpanList) SetFilter(f string) {
	sl.filter = strings.ToLower(f)
}

// Count returns the number of visible spans.
func (sl *SpanList) Count() int { return len(sl.spans) }

func (sl *SpanList) matchesFilter(s SpanData) bool {
	text := strings.ToLower(s.SpanType + " " + s.Name + " " + s.ToolName + " " + s.Model + " " + s.Status)
	return strings.Contains(text, sl.filter)
}

func (sl *SpanList) renderHeader() {
	headers := []struct {
		text      string
		expansion int
		width     int
	}{
		{"Type", 0, 12},
		{"Name", 1, 0},
		{"Status", 0, 12},
		{"Duration", 0, 10},
		{"Tokens", 0, 14},
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
		sl.Table.SetCell(0, col, cell)
	}
}

func (sl *SpanList) renderRow(row int, s SpanData) {
	// Type with color
	typeColor := spanTypeColor(s.SpanType)
	sl.Table.SetCell(row, 0, tview.NewTableCell(" "+s.SpanType).
		SetTextColor(typeColor).SetMaxWidth(12))

	// Name — model for llm_call, tool name for tool_call
	name := s.Name
	switch s.SpanType {
	case "llm_call":
		if s.Model != "" {
			name = s.Model
		}
	case "tool_call":
		if s.ToolName != "" {
			name = s.ToolName
		}
	}
	if len(name) > 40 {
		name = name[:37] + "..."
	}
	sl.Table.SetCell(row, 1, tview.NewTableCell(" "+name).
		SetTextColor(ColorText).SetExpansion(1))

	// Status
	sym, color := traceStatusCell(s.Status)
	sl.Table.SetCell(row, 2, tview.NewTableCell(sym+" "+s.Status).
		SetTextColor(color).SetMaxWidth(12))

	// Duration
	dur := formatTraceDuration(s.DurationMs)
	sl.Table.SetCell(row, 3, tview.NewTableCell(dur).
		SetTextColor(ColorSubtext0).SetMaxWidth(10))

	// Tokens
	tok := fmt.Sprintf("%s/%s", compactNumber(s.InputTokens), compactNumber(s.OutputTokens))
	sl.Table.SetCell(row, 4, tview.NewTableCell(tok).
		SetTextColor(ColorText).SetMaxWidth(14))
}
