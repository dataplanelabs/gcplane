package tui

import (
	"log/slog"

	"github.com/dataplanelabs/gcplane/internal/manifest"
	"github.com/gdamore/tcell/v2"
)

// InputMode represents the current input mode of the TUI.
type InputMode int

const (
	ModeNormal  InputMode = iota // vim normal mode
	ModeCommand                  // : command input mode
	ModeSearch                   // / search filter mode (P1)
)

// kindByNumber maps number keys to resource kinds following ApplyOrder.
// 0=All, 1=Provider, 2=Agent, 3=Channel, 4=MCPServer, 5=Skill,
// 6=CronJob, 7=AgentTeam, 8=SystemConfig, 9=SecureCLI
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
	app  *App
	mode InputMode
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

	// Escape is the universal "get out" key
	if event.Key() == tcell.KeyEscape {
		if h.mode != ModeNormal {
			h.mode = ModeNormal
			h.app.model.SetFilter("")
			h.app.deactivateCommandBar()
			h.app.refreshTable()
			return nil
		}
		if h.app.model.GetFilter() != "" {
			h.app.model.SetFilter("")
			h.app.refreshTable()
			return nil
		}
		h.app.popView()
		return nil
	}

	// In command/search mode, let the InputField handle all other keys
	if h.mode == ModeCommand || h.mode == ModeSearch {
		return event
	}

	// Ctrl+E: edit selected resource — only on State tab, no overlay
	if event.Key() == tcell.KeyCtrlE {
		if h.app.registry.ActiveTab() == TabState && !h.app.registry.HasOverlay() {
			h.app.editResource()
			return nil
		}
	}

	// Ctrl+R: apply (reconcile)
	if event.Key() == tcell.KeyCtrlR {
		h.app.applyAll()
		return nil
	}

	// Ctrl+D: delete selected resource — only on State tab, no overlay
	if event.Key() == tcell.KeyCtrlD {
		if h.app.registry.ActiveTab() == TabState && !h.app.registry.HasOverlay() {
			h.app.deleteResource()
			return nil
		}
	}

	return h.handleNormal(event)
}

// handleNormal processes key events in normal (vim) mode.
func (h *KeyHandler) handleNormal(event *tcell.EventKey) *tcell.EventKey {
	// Tab switching — only when no overlay is active
	if !h.app.registry.HasOverlay() {
		switch event.Rune() {
		case 's':
			h.app.switchTab(TabState)
			return nil
		case 'l':
			h.app.switchTab(TabLogs)
			return nil
		case 'e':
			h.app.switchTab(TabEvents)
			return nil
		case 't':
			h.app.switchTab(TabTrace)
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
		h.app.triggerRefresh()
		return nil
	case ' ':
		h.handlePauseResume()
		return nil
	case 'c':
		h.handleClearFilters()
		return nil
	}

	// Context-sensitive number keys
	if event.Rune() >= '0' && event.Rune() <= '9' {
		h.handleNumberKey(event.Rune())
		return nil
	}

	// j/k vim navigation — translate to arrow keys for the table
	switch event.Rune() {
	case 'j':
		return tcell.NewEventKey(tcell.KeyDown, 0, tcell.ModNone)
	case 'k':
		return tcell.NewEventKey(tcell.KeyUp, 0, tcell.ModNone)
	}

	return event
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
	case TabEvents:
		h.handleEventFilterKey(r)
	case TabTrace:
		h.handleTraceLevelKey(r)
	}
}

func (h *KeyHandler) handleLogLevelKey(r rune) {
	levels := map[rune]string{'1': "debug", '2': "info", '3': "warn", '4': "error"}
	if lvl, ok := levels[r]; ok {
		h.app.logsPanel.SetLevelMin(lvl)
	}
}

func (h *KeyHandler) handleEventFilterKey(r rune) {
	filters := map[rune]string{
		'1': "",        // all
		'2': "agent",
		'3': "chat",
		'4': "health",
		'5': "cron",
		'6': "team.",
	}
	if f, ok := filters[r]; ok {
		h.app.eventsPanel.SetFilter(f)
	}
}

func (h *KeyHandler) handleTraceLevelKey(r rune) {
	if h.app.traceView == nil {
		return
	}
	levels := map[rune]slog.Level{
		'1': slog.LevelDebug,
		'2': slog.LevelInfo,
		'3': slog.LevelWarn,
		'4': slog.LevelError,
	}
	if lvl, ok := levels[r]; ok {
		h.app.traceView.SetLevelMin(lvl)
		h.app.traceView.Refresh()
	}
}

func (h *KeyHandler) handlePauseResume() {
	switch h.app.registry.ActiveTab() {
	case TabLogs:
		h.app.logsPanel.TogglePause()
	case TabEvents:
		h.app.eventsPanel.TogglePause()
	case TabTrace:
		if h.app.traceView != nil {
			h.app.traceView.TogglePause()
		}
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
	case TabEvents:
		h.app.eventsPanel.SetFilter("")
	case TabTrace:
		if h.app.traceView != nil {
			h.app.traceView.SetLevelMin(slog.LevelDebug)
			h.app.traceView.Refresh()
		}
	}
}
