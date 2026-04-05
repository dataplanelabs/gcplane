package views

import (
	"fmt"

	"github.com/rivo/tview"
)

// TracesPanel is the Traces tab with stack-based drill-down navigation.
// Level 0: TraceList → Level 1: SpanList → Level 2: SpanDetail
type TracesPanel struct {
	flex      *tview.Flex
	pages     *tview.Pages
	list      *TraceList
	spanList  *SpanList
	detail    *SpanDetail
	statusBar *tview.TextView

	// Navigation state
	Level     int    // 0=traces, 1=spans, 2=detail
	traceName string // breadcrumb: selected trace name
	spanName  string // breadcrumb: selected span name

	// Data state
	paused        bool
	agentFilter   string
	channelFilter string
	traceCount    int
	traceTotal    int
	filter        string // current search filter
}

// NewTracesPanel creates a traces panel with drill-down navigation.
func NewTracesPanel() *TracesPanel {
	p := &TracesPanel{
		list:     NewTraceList(),
		spanList: NewSpanList(),
		detail:   NewSpanDetail(),
	}

	p.statusBar = tview.NewTextView().
		SetDynamicColors(true).
		SetTextAlign(tview.AlignLeft)
	p.statusBar.SetBackgroundColor(ColorBase)

	p.pages = tview.NewPages()
	p.pages.AddPage("traces", p.list.Table, true, true)
	p.pages.AddPage("spans", p.spanList.Table, true, false)
	p.pages.AddPage("detail", p.detail.TextView, true, false)

	p.flex = tview.NewFlex().SetDirection(tview.FlexRow).
		AddItem(p.pages, 0, 1, true).
		AddItem(p.statusBar, 1, 0, false)

	p.updateStatusBar()
	return p
}

// Name implements View.
func (p *TracesPanel) Name() string { return "traces" }

// Primitive implements View.
func (p *TracesPanel) Primitive() tview.Primitive { return p.flex }

// Activate implements View.
func (p *TracesPanel) Activate() {}

// List returns the TraceList for wiring callbacks.
func (p *TracesPanel) List() *TraceList { return p.list }

// SpanListView returns the SpanList for wiring callbacks.
func (p *TracesPanel) SpanListView() *SpanList { return p.spanList }

// Detail returns the SpanDetail for wiring.
func (p *TracesPanel) Detail() *SpanDetail { return p.detail }

// FocusPrimitive returns the widget that should receive focus at the current level.
func (p *TracesPanel) FocusPrimitive() tview.Primitive {
	switch p.Level {
	case 1:
		return p.spanList.Table
	case 2:
		return p.detail.TextView
	default:
		return p.list.Table
	}
}

// DrillIn navigates one level deeper. Returns true if drill happened.
func (p *TracesPanel) DrillIn(tapp *tview.Application) bool {
	switch p.Level {
	case 1:
		// Span list → span detail
		span := p.spanList.SelectedSpan()
		if span == nil {
			return false
		}
		p.spanName = span.Name
		if span.SpanType == "llm_call" && span.Model != "" {
			p.spanName = span.Model
		} else if span.SpanType == "tool_call" && span.ToolName != "" {
			p.spanName = span.ToolName
		}
		p.detail.Show(*span)
		p.Level = 2
		p.pages.SwitchToPage("detail")
		tapp.SetFocus(p.detail.TextView)
		p.updateStatusBar()
		return true
	}
	// Level 0 drill-in is handled by app (needs data fetch)
	return false
}

// ShowSpans transitions to the span list view (level 1).
// Called by app after fetching trace detail.
func (p *TracesPanel) ShowSpans(trace *TraceData, spans []SpanData, tapp *tview.Application) {
	p.traceName = trace.Name
	if p.traceName == "" {
		p.traceName = trace.AgentID
	}
	p.spanList.Refresh(spans)
	p.Level = 1
	p.pages.SwitchToPage("spans")
	tapp.SetFocus(p.spanList.Table)
	p.updateStatusBar()
}

// DrillOut navigates one level back.
func (p *TracesPanel) DrillOut(tapp *tview.Application) {
	switch p.Level {
	case 2:
		p.spanName = ""
		p.Level = 1
		p.pages.SwitchToPage("spans")
		tapp.SetFocus(p.spanList.Table)
	case 1:
		p.traceName = ""
		p.Level = 0
		p.pages.SwitchToPage("traces")
		tapp.SetFocus(p.list.Table)
	}
	p.updateStatusBar()
}

// DrillRoot navigates all the way back to trace list (level 0).
func (p *TracesPanel) DrillRoot(tapp *tview.Application) {
	p.traceName = ""
	p.spanName = ""
	p.Level = 0
	p.pages.SwitchToPage("traces")
	tapp.SetFocus(p.list.Table)
	p.updateStatusBar()
}

// Refresh updates the trace list display (level 0 only).
func (p *TracesPanel) Refresh(traces []TraceData, total int, selectedID string) {
	p.traceCount = len(traces)
	p.traceTotal = total
	p.list.Refresh(traces, selectedID)
	p.updateStatusBar()
}

// RefreshUnavailable shows fallback when trace API is not available.
func (p *TracesPanel) RefreshUnavailable(reason string) {
	p.list.Table.Clear()
	p.list.Table.SetCell(0, 0, tview.NewTableCell(
		Tag(HexOverlay0, "  "+reason)).
		SetExpansion(1).SetSelectable(false))
}

// SetAgentFilter records the active agent filter.
func (p *TracesPanel) SetAgentFilter(agent string) {
	p.agentFilter = agent
	p.updateStatusBar()
}

// SetChannelFilter records the active channel filter.
func (p *TracesPanel) SetChannelFilter(channel string) {
	p.channelFilter = channel
	p.updateStatusBar()
}

// TogglePause toggles auto-refresh pause.
func (p *TracesPanel) TogglePause() {
	p.paused = !p.paused
	p.updateStatusBar()
}

// IsPaused returns the pause state.
func (p *TracesPanel) IsPaused() bool { return p.paused }

// GoToTop selects the first row in the current level's table.
func (p *TracesPanel) GoToTop() {
	switch p.Level {
	case 0:
		p.list.Table.Select(1, 0)
	case 1:
		p.spanList.Table.Select(1, 0)
	case 2:
		p.detail.TextView.ScrollToBeginning()
	}
}

// GoToBottom selects the last row in the current level's table.
func (p *TracesPanel) GoToBottom() {
	switch p.Level {
	case 0:
		if rc := p.list.Table.GetRowCount(); rc > 1 {
			p.list.Table.Select(rc-1, 0)
		}
	case 1:
		if rc := p.spanList.Table.GetRowCount(); rc > 1 {
			p.spanList.Table.Select(rc-1, 0)
		}
	case 2:
		p.detail.TextView.ScrollToEnd()
	}
}

// CopySelected returns the text to copy at the current level.
func (p *TracesPanel) CopySelected() string {
	switch p.Level {
	case 0:
		return p.list.SelectedTraceID()
	case 1:
		return p.spanList.SelectedText()
	case 2:
		if p.detail.lastSpan != nil {
			return formatSpanCopyText(*p.detail.lastSpan)
		}
	}
	return ""
}

// SetFilter applies search filter to the current level.
func (p *TracesPanel) SetFilter(f string) {
	p.filter = f
	switch p.Level {
	case 0:
		p.list.SetFilter(f)
	case 1:
		p.spanList.SetFilter(f)
	}
}

// updateStatusBar renders the breadcrumb status bar.
func (p *TracesPanel) updateStatusBar() {
	pauseTag := ""
	if p.paused {
		pauseTag = Tag(HexYellow, "[PAUSED] ")
	}

	// Breadcrumb
	crumb := Tag(HexMauve, "Traces")
	if p.traceName != "" {
		crumb += Tag(HexOverlay0, " > ") + Tag(HexBlue, p.traceName)
	}
	if p.spanName != "" {
		crumb += Tag(HexOverlay0, " > ") + Tag(HexTeal, p.spanName)
	}

	// Context info
	info := ""
	switch p.Level {
	case 0:
		agentInfo := "all"
		if p.agentFilter != "" {
			agentInfo = p.agentFilter
		}
		info = fmt.Sprintf(" %d/%d | Agent: %s", p.traceCount, p.traceTotal, agentInfo)
	case 1:
		info = fmt.Sprintf(" %d spans", p.spanList.Count())
	case 2:
		info = " detail"
	}

	nav := Tag(HexOverlay0, " [l=in h=out Esc=root]")
	status := fmt.Sprintf(" %s%s%s%s", pauseTag, crumb, info, nav)
	p.statusBar.SetText(status)
}
