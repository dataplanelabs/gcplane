package tui

import (
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
// 6=Tool, 7=CronJob, 8=AgentTeam, 9=TTSConfig
var kindByNumber = map[rune]manifest.ResourceKind{
	'1': manifest.KindProvider,
	'2': manifest.KindAgent,
	'3': manifest.KindChannel,
	'4': manifest.KindMCPServer,
	'5': manifest.KindSkill,
	'6': manifest.KindTool,
	'7': manifest.KindCronJob,
	'8': manifest.KindAgentTeam,
	'9': manifest.KindTTSConfig,
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
	if event.Key() == tcell.KeyCtrlC {
		h.app.Stop()
		return nil
	}

	switch h.mode {
	case ModeNormal:
		return h.handleNormal(event)
	case ModeCommand:
		return h.handleCommand(event)
	case ModeSearch:
		return h.handleSearch(event)
	}
	return event
}

// handleNormal processes key events in normal (vim) mode.
func (h *KeyHandler) handleNormal(event *tcell.EventKey) *tcell.EventKey {
	if event.Key() == tcell.KeyEscape {
		h.app.popView()
		return nil
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
	case '0':
		h.app.switchKind("")
		return nil
	}

	// Number keys 1-9 for kind switching
	if kind, ok := kindByNumber[event.Rune()]; ok {
		h.app.switchKind(kind)
		return nil
	}

	// j/k vim navigation — translate to arrow keys for the table
	switch event.Rune() {
	case 'j':
		return tcell.NewEventKey(tcell.KeyDown, 0, tcell.ModNone)
	case 'k':
		return tcell.NewEventKey(tcell.KeyUp, 0, tcell.ModNone)
	}

	// Pass through for table's own input capture (Enter, d, g, G)
	return event
}

// handleCommand processes key events when the command bar is active.
func (h *KeyHandler) handleCommand(event *tcell.EventKey) *tcell.EventKey {
	if event.Key() == tcell.KeyEscape {
		h.mode = ModeNormal
		h.app.deactivateCommandBar()
		return nil
	}
	return event
}

// handleSearch processes key events in search/filter mode (P1 placeholder).
func (h *KeyHandler) handleSearch(event *tcell.EventKey) *tcell.EventKey {
	if event.Key() == tcell.KeyEscape {
		h.mode = ModeNormal
		return nil
	}
	return event
}
