package views

import (
	"fmt"

	"github.com/rivo/tview"
)

// TracesPanel is the main Traces tab view with split list + tree layout.
type TracesPanel struct {
	flex      *tview.Flex // outer: vertical (content + status)
	split     *tview.Flex // inner: horizontal (list | tree)
	list      *TraceList
	tree      *SpanTree
	statusBar *tview.TextView
	paused    bool
	focusLeft bool // true = list focused, false = tree focused
	agentFilter   string
	channelFilter string
	traceCount    int
	traceTotal    int
}

// NewTracesPanel creates a traces panel with split list/tree layout.
func NewTracesPanel() *TracesPanel {
	p := &TracesPanel{
		list:      NewTraceList(),
		tree:      NewSpanTree(),
		focusLeft: true,
	}

	// Status bar
	p.statusBar = tview.NewTextView().
		SetDynamicColors(true).
		SetTextAlign(tview.AlignLeft)
	p.statusBar.SetBackgroundColor(ColorBase)

	// Horizontal split: list (2) | tree (3)
	p.split = tview.NewFlex().SetDirection(tview.FlexColumn).
		AddItem(p.list.Table, 0, 2, true).
		AddItem(p.tree.Tree, 0, 3, false)

	// Vertical layout: split (flex) + status (1 row)
	p.flex = tview.NewFlex().SetDirection(tview.FlexRow).
		AddItem(p.split, 0, 1, true).
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

// List returns the inner TraceList for wiring callbacks.
func (p *TracesPanel) List() *TraceList { return p.list }

// Tree returns the inner SpanTree for wiring callbacks.
func (p *TracesPanel) Tree() *SpanTree { return p.tree }

// FocusPrimitive returns the widget that should receive focus.
func (p *TracesPanel) FocusPrimitive() tview.Primitive {
	if p.focusLeft {
		return p.list.Table
	}
	return p.tree.Tree
}

// ToggleFocus switches focus between list and tree.
func (p *TracesPanel) ToggleFocus(tapp *tview.Application) {
	p.focusLeft = !p.focusLeft
	tapp.SetFocus(p.FocusPrimitive())
}

// Refresh updates the trace list display.
func (p *TracesPanel) Refresh(traces []TraceData, total int, selectedID string) {
	p.traceCount = len(traces)
	p.traceTotal = total
	p.list.Refresh(traces, selectedID)
	p.updateStatusBar()
}

// RefreshDetail updates the span tree display.
func (p *TracesPanel) RefreshDetail(roots []*SpanNode) {
	p.tree.Refresh(roots)
}

// RefreshUnavailable shows fallback when trace API is not available.
func (p *TracesPanel) RefreshUnavailable(reason string) {
	p.list.Table.Clear()
	p.list.Table.SetCell(0, 0, tview.NewTableCell(
		Tag(HexOverlay0, "  "+reason)).
		SetExpansion(1).SetSelectable(false))
	p.tree.Refresh(nil)
}

// SetAgentFilter records the active agent filter for status bar display.
func (p *TracesPanel) SetAgentFilter(agent string) {
	p.agentFilter = agent
	p.updateStatusBar()
}

// SetChannelFilter records the active channel filter for status bar display.
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

// updateStatusBar renders the status bar with current state.
func (p *TracesPanel) updateStatusBar() {
	pauseTag := ""
	if p.paused {
		pauseTag = Tag(HexYellow, "[PAUSED] ")
	}

	agentInfo := "all"
	if p.agentFilter != "" {
		agentInfo = p.agentFilter
	}

	channelInfo := "all"
	if p.channelFilter != "" {
		channelInfo = p.channelFilter
	}

	status := fmt.Sprintf(" %s%s %d/%d traces | Agent: %s | Channel: %s",
		pauseTag,
		Tag(HexOverlay0, "---"),
		p.traceCount, p.traceTotal,
		agentInfo, channelInfo)

	p.statusBar.SetText(status)
}
