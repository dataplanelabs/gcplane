package tui

import (
	"github.com/dataplanelabs/gcplane/internal/manifest"
	"github.com/dataplanelabs/gcplane/internal/tui/views"
	"github.com/gdamore/tcell/v2"
)

// InputMode represents the current input mode of the TUI.
type InputMode int

const (
	ModeNormal  InputMode = iota // vim normal mode
	ModeCommand                  // : command input mode
	ModeSearch                   // / search filter mode
)

// kindByNumber maps number keys to resource kinds following ApplyOrder.
var kindByNumber = map[rune]manifest.ResourceKind{
	'1': manifest.KindProvider,
	'2': manifest.KindAgent,
	'3': manifest.KindChannel,
	'4': manifest.KindMCPServer,
	'5': manifest.KindSkill,
	'6': manifest.KindCronJob,
	'7': manifest.KindAgentTeam,
	'8': manifest.KindSystemConfig,
	'9': manifest.KindSecureCLI,
}

// KeyHandler dispatches key events based on the current input mode.
type KeyHandler struct {
	app     *App
	mode    InputMode
	pending rune // for multi-key sequences: gg, yy
}

// NewKeyHandler creates a key handler bound to the given app.
func NewKeyHandler(app *App) *KeyHandler {
	return &KeyHandler{app: app, mode: ModeNormal}
}

// Handle is the global input capture function for tview.Application.
func (h *KeyHandler) Handle(event *tcell.EventKey) *tcell.EventKey {
	if event.Key() == tcell.KeyCtrlC || event.Key() == tcell.KeyCtrlQ {
		h.app.Stop()
		return nil
	}

	// Escape — universal "go back"
	if event.Key() == tcell.KeyEscape {
		h.pending = 0
		if h.mode != ModeNormal {
			h.mode = ModeNormal
			h.app.clearSearch()
			h.app.deactivateCommandBar()
			return nil
		}
		// On Traces tab: drill back to root
		if h.app.registry.ActiveTab() == TabTraces && h.app.tracesPanel.Level > 0 {
			h.app.tracesPanel.DrillRoot(h.app.tapp)
			return nil
		}
		// Pop overlay or clear filter
		if h.app.registry.HasOverlay() {
			h.app.popView()
			return nil
		}
		if h.app.model.GetFilter() != "" {
			h.app.model.SetFilter("")
			h.app.refreshTable()
			return nil
		}
		return nil
	}

	// In command/search mode, let the InputField handle keys
	if h.mode == ModeCommand || h.mode == ModeSearch {
		return event
	}

	// Ctrl+E: edit selected resource — State tab only
	if event.Key() == tcell.KeyCtrlE {
		if h.app.registry.ActiveTab() == TabState && !h.app.registry.HasOverlay() {
			h.app.editResource()
			return nil
		}
	}

	// Ctrl+R: refresh active tab
	if event.Key() == tcell.KeyCtrlR {
		h.app.refreshActiveTab()
		return nil
	}

	// Ctrl+D: delete on State tab, half-page down elsewhere
	if event.Key() == tcell.KeyCtrlD {
		if h.app.registry.ActiveTab() == TabState && !h.app.registry.HasOverlay() {
			h.app.deleteResource()
			return nil
		}
		return event // pass to component for half-page scroll
	}

	// Ctrl+U: half-page up — pass to component
	if event.Key() == tcell.KeyCtrlU {
		return event
	}

	// Enter — drill in on Traces tab
	if event.Key() == tcell.KeyEnter {
		if h.app.registry.ActiveTab() == TabTraces && !h.app.registry.HasOverlay() {
			h.handleDrillIn()
			return nil
		}
		return event // let table/other handle Enter normally
	}

	return h.handleNormal(event)
}

// handleNormal processes key events in normal (vim) mode.
func (h *KeyHandler) handleNormal(event *tcell.EventKey) *tcell.EventKey {
	// Multi-key sequences (gg, yy)
	if h.pending != 0 {
		prev := h.pending
		h.pending = 0
		if prev == 'g' && event.Rune() == 'g' {
			h.goToTop()
			return nil
		}
		if prev == 'y' && event.Rune() == 'y' {
			h.copySelected()
			return nil
		}
		// Not a valid sequence — process current key as new input
	}

	// Tab switching — uppercase S/T/L, only when no overlay
	if !h.app.registry.HasOverlay() {
		switch event.Rune() {
		case 'S':
			h.app.switchTab(TabState)
			return nil
		case 'T':
			h.app.switchTab(TabTraces)
			return nil
		case 'L':
			h.app.switchTab(TabLogs)
			return nil
		}
	}

	switch event.Rune() {
	case 'q':
		h.app.Stop()
		return nil
	case '?':
		h.app.toggleHelp()
		return nil
	case ':':
		h.mode = ModeCommand
		h.app.activateCommandBar()
		return nil
	case '/':
		h.mode = ModeSearch
		h.app.activateSearch()
		return nil
	case 'r':
		h.app.refreshActiveTab()
		return nil

	// Vim motions — handled globally
	case 'G':
		h.goToBottom()
		return nil
	case 'g':
		h.pending = 'g'
		return nil
	case 'y':
		h.pending = 'y'
		return nil
	case 'j':
		return tcell.NewEventKey(tcell.KeyDown, 0, tcell.ModNone)
	case 'k':
		return tcell.NewEventKey(tcell.KeyUp, 0, tcell.ModNone)

	// Drill navigation — Traces tab
	case 'l':
		if h.app.registry.ActiveTab() == TabTraces {
			h.handleDrillIn()
			return nil
		}
		return event
	case 'h':
		if h.app.registry.ActiveTab() == TabTraces && h.app.tracesPanel.Level > 0 {
			h.app.tracesPanel.DrillOut(h.app.tapp)
			return nil
		}
		return event

	// Pause/resume
	case ' ':
		h.handlePauseResume()
		return nil
	case 'p':
		if h.app.registry.ActiveTab() == TabTraces {
			h.app.tracesPanel.TogglePause()
			return nil
		}
		return event

	// Clear filters
	case 'c':
		h.handleClearFilters()
		return nil
	}

	// Number keys
	if event.Rune() >= '0' && event.Rune() <= '9' {
		h.handleNumberKey(event.Rune())
		return nil
	}

	return event
}

// goToTop navigates to the first item in the active view.
func (h *KeyHandler) goToTop() {
	switch h.app.registry.ActiveTab() {
	case TabState:
		h.app.table.Table.Select(1, 0)
	case TabTraces:
		h.app.tracesPanel.GoToTop()
	case TabLogs:
		h.app.logsPanel.GoToTop()
	}
}

// goToBottom navigates to the last item in the active view.
func (h *KeyHandler) goToBottom() {
	switch h.app.registry.ActiveTab() {
	case TabState:
		if rc := h.app.table.Table.GetRowCount(); rc > 1 {
			h.app.table.Table.Select(rc-1, 0)
		}
	case TabTraces:
		h.app.tracesPanel.GoToBottom()
	case TabLogs:
		h.app.logsPanel.GoToBottom()
	}
}

// copySelected copies the selected item to clipboard.
func (h *KeyHandler) copySelected() {
	var text string
	switch h.app.registry.ActiveTab() {
	case TabTraces:
		text = h.app.tracesPanel.CopySelected()
	case TabLogs:
		text = h.app.logsPanel.SelectedEntry()
	}
	if text != "" {
		_ = views.CopyToClipboard(text)
		h.app.showStatus(views.Tag(views.HexGreen, "Copied"))
	}
}

// handleDrillIn navigates one level deeper in the Traces tab.
func (h *KeyHandler) handleDrillIn() {
	if h.app.registry.ActiveTab() != TabTraces {
		return
	}
	panel := h.app.tracesPanel
	switch panel.Level {
	case 0:
		// Trace list → fetch spans and show span list
		h.app.drillIntoTrace()
	case 1:
		// Span list → span detail
		panel.DrillIn(h.app.tapp)
	}
}

// handleNumberKey dispatches number keys based on active tab.
func (h *KeyHandler) handleNumberKey(r rune) {
	switch h.app.registry.ActiveTab() {
	case TabState:
		if r == '0' {
			h.app.switchKind("")
		} else if kind, ok := kindByNumber[r]; ok {
			h.app.switchKind(kind)
		}
	case TabLogs:
		h.handleLogLevelKey(r)
	}
}

func (h *KeyHandler) handleLogLevelKey(r rune) {
	levels := map[rune]string{'1': "debug", '2': "info", '3': "warn", '4': "error"}
	if lvl, ok := levels[r]; ok {
		h.app.logsPanel.SetLevelMin(lvl)
	}
}

func (h *KeyHandler) handlePauseResume() {
	switch h.app.registry.ActiveTab() {
	case TabLogs:
		h.app.logsPanel.TogglePause()
	case TabTraces:
		h.app.tracesPanel.TogglePause()
	}
}

func (h *KeyHandler) handleClearFilters() {
	switch h.app.registry.ActiveTab() {
	case TabState:
		h.app.switchKind("")
		h.app.model.SetFilter("")
		h.app.refreshTable()
	case TabLogs:
		h.app.logsPanel.SetLevelMin("debug")
		h.app.logsPanel.SetFilter("")
		h.app.logsPanel.Refresh(h.app.liveStore.Logs())
	case TabTraces:
		h.app.traceStore.SetFilters(views.TraceFilters{Limit: 50})
		h.app.tracesPanel.SetAgentFilter("")
		h.app.tracesPanel.SetChannelFilter("")
		h.app.tracesPanel.SetFilter("")
		h.app.traceStore.NotifyTraceUpdated()
	}
}
