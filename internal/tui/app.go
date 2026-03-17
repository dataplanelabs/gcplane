package tui

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/dataplanelabs/gcplane/internal/manifest"
	"github.com/dataplanelabs/gcplane/internal/reconciler"
	"github.com/dataplanelabs/gcplane/internal/tui/views"
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

// ProviderAPI is the subset of provider operations the TUI needs.
type ProviderAPI interface {
	Observe(kind manifest.ResourceKind, key string) (map[string]any, error)
	Create(kind manifest.ResourceKind, key string, spec map[string]any) error
	Update(kind manifest.ResourceKind, key string, spec map[string]any) error
	Delete(kind manifest.ResourceKind, key string) error
	Close() error
}

// App is the top-level TUI application, wiring layout, views, and keybindings.
type App struct {
	tapp      *tview.Application
	model     *Model
	layout    *tview.Flex
	pages     *tview.Pages
	header    *tview.TextView
	cmdBar    *tview.InputField
	keys      *KeyHandler
	table     *views.ResourceTable
	detail    *views.ResourceDetail
	drift     *views.DriftView
	confirm   *views.ConfirmModal
	viewStack []string // page name stack for Esc navigation

	// Refresh infrastructure
	refreshMu sync.Mutex
	refreshCh chan struct{} // manual refresh trigger
	cancel    context.CancelFunc

	// Integration points
	Provider ProviderAPI
	Engine   *reconciler.Engine
	Manifest *manifest.Manifest
}

// Config holds the parameters for creating a new TUI App.
type Config struct {
	Manifest *manifest.Manifest
	Endpoint string
	Provider ProviderAPI
	Engine   *reconciler.Engine
	Interval string // e.g. "10s"
}

// NewApp creates and wires the TUI application.
func NewApp(cfg Config) (*App, error) {
	interval, err := time.ParseDuration(cfg.Interval)
	if err != nil {
		return nil, fmt.Errorf("invalid interval %q: %w", cfg.Interval, err)
	}

	app := &App{
		tapp:      tview.NewApplication(),
		model:     NewModel(cfg.Manifest, cfg.Endpoint, interval),
		Provider:  cfg.Provider,
		Engine:    cfg.Engine,
		Manifest:  cfg.Manifest,
		refreshCh: make(chan struct{}, 1),
	}
	app.keys = NewKeyHandler(app)

	app.buildLayout()
	app.tapp.SetInputCapture(app.keys.Handle)

	return app, nil
}

// buildLayout creates the 3-row layout: header, pages, command bar.
func (a *App) buildLayout() {
	// Header bar — 1 row
	a.header = tview.NewTextView().
		SetDynamicColors(true).
		SetTextAlign(tview.AlignLeft)
	a.header.SetBackgroundColor(tview.Styles.PrimitiveBackgroundColor)
	a.updateHeader()

	// Resource table — main view
	a.table = views.NewResourceTable()
	a.table.OnSelect = func(c reconciler.Change) {
		a.showDetail(c)
	}
	a.table.OnDrift = func(c reconciler.Change) {
		a.showDrift(c)
	}

	// Detail view — shows full YAML of observed resource
	a.detail = views.NewResourceDetail()

	// Drift view — shows field-level diff
	a.drift = views.NewDriftView()

	// Confirmation modal
	a.confirm = views.NewConfirmModal()

	// Switchable main content area
	a.pages = tview.NewPages()
	a.pages.AddPage("main", a.table.Table, true, true)
	a.pages.AddPage("detail", a.detail.TextView, true, false)
	a.pages.AddPage("drift", a.drift.TextView, true, false)
	a.pages.AddPage("confirm", a.confirm.Modal, true, false)

	// Help overlay
	helpView := tview.NewTextView().
		SetDynamicColors(true).
		SetText(helpText())
	helpView.SetBorder(true).SetTitle(" Help (? to close) ")
	a.pages.AddPage("help", helpView, true, false)

	// Command bar — 1 row input
	a.cmdBar = tview.NewInputField().
		SetLabel(":").
		SetFieldWidth(0).
		SetDoneFunc(a.onCommandDone)
	a.cmdBar.SetFieldBackgroundColor(tview.Styles.PrimitiveBackgroundColor)

	// Root layout: header(1) + pages(flex) + cmdbar(1)
	a.layout = tview.NewFlex().SetDirection(tview.FlexRow).
		AddItem(a.header, 1, 0, false).
		AddItem(a.pages, 0, 1, true).
		AddItem(a.cmdBar, 1, 0, false)
}

// Run starts the TUI event loop. Blocks until the app exits.
func (a *App) Run() error {
	a.viewStack = []string{"main"}

	ctx, cancel := context.WithCancel(context.Background())
	a.cancel = cancel

	// Initial data load + start refresh loop
	go a.refresh()
	go a.refreshLoop(ctx)

	err := a.tapp.SetRoot(a.layout, true).EnableMouse(false).Run()
	cancel() // stop refresh loop on exit
	return err
}

// Stop gracefully shuts down the TUI.
func (a *App) Stop() {
	if a.cancel != nil {
		a.cancel()
	}
	a.tapp.Stop()
}

// triggerRefresh sends a manual refresh signal (non-blocking).
func (a *App) triggerRefresh() {
	select {
	case a.refreshCh <- struct{}{}:
	default: // already pending
	}
}

// refresh runs a dry-run reconciliation and updates the table.
func (a *App) refresh() {
	if !a.refreshMu.TryLock() {
		return
	}
	defer a.refreshMu.Unlock()

	plan, _ := a.Engine.Reconcile(a.Manifest, reconciler.ReconcileOpts{DryRun: true})
	a.model.UpdatePlan(plan)

	a.tapp.QueueUpdateDraw(func() {
		a.table.Refresh(a.model.GetChanges())
		a.updateHeader()
	})
}

// refreshLoop periodically triggers refresh; also handles manual refresh signals.
func (a *App) refreshLoop(ctx context.Context) {
	interval := a.model.GetInterval()
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			a.refresh()
		case <-a.refreshCh:
			a.refresh()
			ticker.Reset(interval) // reset timer after manual refresh
		}
	}
}

// refreshTable redraws the table with current model data (no API call).
func (a *App) refreshTable() {
	a.table.Refresh(a.model.GetChanges())
	a.updateHeader()
}

// updateHeader refreshes the header bar text.
func (a *App) updateHeader() {
	name := a.model.GetManifestName()
	ep := a.model.GetEndpoint()
	kind := a.model.GetKind()

	kindLabel := "[green]all"
	if kind != "" {
		kindLabel = "[green]" + string(kind)
	}

	// Status summary from current changes
	summary := ""
	if changes := a.model.GetChanges(); len(changes) > 0 {
		summary = " | " + views.StatusSummary(changes)
	}

	lastRefresh := a.model.GetLastRefresh()
	age := ""
	if !lastRefresh.IsZero() {
		age = fmt.Sprintf(" | %s ago", formatDuration(time.Since(lastRefresh)))
	}

	text := fmt.Sprintf(" [bold]gcplane[white] | %s | %s | %s%s%s",
		name, ep, kindLabel, summary, age)
	a.header.SetText(text)
}

// switchKind changes the kind filter and refreshes the table.
func (a *App) switchKind(kind manifest.ResourceKind) {
	a.model.SetKind(kind)
	a.refreshTable()
}

// pushView navigates to a named page, preserving the stack for Esc.
func (a *App) pushView(name string) {
	a.viewStack = append(a.viewStack, name)
	a.pages.SwitchToPage(name)
}

// popView returns to the previous page in the view stack.
func (a *App) popView() {
	if len(a.viewStack) > 0 {
		a.viewStack = a.viewStack[:len(a.viewStack)-1]
	}
	if len(a.viewStack) > 0 {
		page := a.viewStack[len(a.viewStack)-1]
		a.pages.SwitchToPage(page)
		if page == "main" {
			a.tapp.SetFocus(a.table.Table)
		}
	} else {
		a.pages.SwitchToPage("main")
		a.tapp.SetFocus(a.table.Table)
	}
}

// showDetail navigates to the resource detail YAML view.
func (a *App) showDetail(c reconciler.Change) {
	a.detail.Show(c.Kind, c.Name, a.Provider, a.tapp)
	a.pushView("detail")
	a.tapp.SetFocus(a.detail.TextView)
}

// showDrift navigates to the drift diff view for a resource.
func (a *App) showDrift(c reconciler.Change) {
	a.drift.Show(c)
	a.pushView("drift")
	a.tapp.SetFocus(a.drift.TextView)
}

// toggleHelp shows or hides the help overlay.
func (a *App) toggleHelp() {
	if name, _ := a.pages.GetFrontPage(); name == "help" {
		a.pages.SwitchToPage("main")
		a.tapp.SetFocus(a.table.Table)
	} else {
		a.pages.SwitchToPage("help")
	}
}

// activateCommandBar focuses the command bar input.
func (a *App) activateCommandBar() {
	a.cmdBar.SetText("")
	a.tapp.SetFocus(a.cmdBar)
}

// deactivateCommandBar returns focus to the main content.
func (a *App) deactivateCommandBar() {
	a.cmdBar.SetText("")
	a.tapp.SetFocus(a.table.Table)
}

// onCommandDone handles command bar submission or cancellation.
func (a *App) onCommandDone(key tcell.Key) {
	if key == tcell.KeyEscape {
		a.keys.mode = ModeNormal
		a.deactivateCommandBar()
		return
	}
	if key == tcell.KeyEnter {
		cmd := a.cmdBar.GetText()
		a.keys.mode = ModeNormal
		a.deactivateCommandBar()
		a.executeCommand(cmd)
	}
}

// kindAliases maps short command names to resource kinds.
var kindAliases = map[string]manifest.ResourceKind{
	"provider":  manifest.KindProvider,
	"agent":     manifest.KindAgent,
	"channel":   manifest.KindChannel,
	"mcp":       manifest.KindMCPServer,
	"mcpserver": manifest.KindMCPServer,
	"skill":     manifest.KindSkill,
	"tool":      manifest.KindTool,
	"cron":      manifest.KindCronJob,
	"cronjob":   manifest.KindCronJob,
	"team":      manifest.KindAgentTeam,
	"agentteam": manifest.KindAgentTeam,
	"tts":       manifest.KindTTSConfig,
	"ttsconfig": manifest.KindTTSConfig,
}

// executeCommand processes : commands (kind switching, quit, etc.)
func (a *App) executeCommand(cmd string) {
	cmd = strings.TrimSpace(strings.ToLower(cmd))

	switch cmd {
	case "q", "quit":
		a.Stop()
		return
	case "all":
		a.switchKind("")
		return
	case "help":
		a.toggleHelp()
		return
	case "apply":
		a.applyAll()
		return
	case "delete", "del":
		a.deleteResource()
		return
	}

	// Kind alias lookup
	if kind, ok := kindAliases[cmd]; ok {
		a.switchKind(kind)
		return
	}

	// Try full kind name match
	for _, kind := range manifest.ApplyOrder() {
		if strings.EqualFold(cmd, string(kind)) {
			a.switchKind(kind)
			return
		}
	}
}

// activateSearch switches to search mode with / prefix.
func (a *App) activateSearch() {
	a.cmdBar.SetLabel("/")
	a.cmdBar.SetText("")
	a.cmdBar.SetDoneFunc(a.onSearchDone)
	a.tapp.SetFocus(a.cmdBar)
}

// onSearchDone handles search input completion.
func (a *App) onSearchDone(key tcell.Key) {
	if key == tcell.KeyEscape {
		a.model.SetFilter("")
		a.keys.mode = ModeNormal
		a.cmdBar.SetLabel(":")
		a.cmdBar.SetDoneFunc(a.onCommandDone)
		a.deactivateCommandBar()
		a.refreshTable()
		return
	}
	if key == tcell.KeyEnter {
		filter := a.cmdBar.GetText()
		a.model.SetFilter(filter)
		a.keys.mode = ModeNormal
		a.cmdBar.SetLabel(":")
		a.cmdBar.SetDoneFunc(a.onCommandDone)
		a.deactivateCommandBar()
		a.refreshTable()
	}
}

// formatDuration returns a human-friendly short duration string.
func formatDuration(d time.Duration) string {
	if d < time.Minute {
		return fmt.Sprintf("%ds", int(d.Seconds()))
	}
	if d < time.Hour {
		return fmt.Sprintf("%dm", int(d.Minutes()))
	}
	return fmt.Sprintf("%dh", int(d.Hours()))
}

// helpText returns the help overlay content.
func helpText() string {
	return `
 [yellow]Navigation[white]
   j/k         Move down/up
   g/G         Jump to top/bottom
   Enter       View resource detail
   d           Show drift diff
   Esc         Back / Close overlay
   q           Quit

 [yellow]Kind Filter[white]
   1 Provider   2 Agent      3 Channel
   4 MCPServer  5 Skill      6 Tool
   7 CronJob    8 AgentTeam  9 TTSConfig
   0 All

 [yellow]Commands[white]
   :provider   :agent    :channel   :mcp
   :skill      :tool     :cron      :team    :tts
   :all        Show all resources
   :help       Show this help
   :q          Quit

 [yellow]Search[white]
   /           Filter by name (case-insensitive)
   Enter       Apply filter
   Esc         Cancel / clear filter

 [yellow]Actions[white]
   Ctrl+R      Apply (reconcile all pending changes)
   Ctrl+D      Delete selected resource
   e           Edit selected resource ($EDITOR)
   :apply      Apply all changes
   :delete     Delete selected resource

 [yellow]Other[white]
   ?           Toggle this help
   r           Refresh now
`
}
