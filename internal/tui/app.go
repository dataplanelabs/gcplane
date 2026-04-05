package tui

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/dataplanelabs/gcplane/internal/manifest"
	"github.com/dataplanelabs/gcplane/internal/reconciler"
	"github.com/dataplanelabs/gcplane/internal/tui/trace"
	"github.com/dataplanelabs/gcplane/internal/tui/views"
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

// ProviderAPI is the subset of provider operations the TUI needs.
type ProviderAPI interface {
	Observe(ctx context.Context, kind manifest.ResourceKind, key string) (map[string]any, error)
	Create(ctx context.Context, kind manifest.ResourceKind, key string, spec map[string]any) error
	Update(ctx context.Context, kind manifest.ResourceKind, key string, spec map[string]any) error
	Delete(ctx context.Context, kind manifest.ResourceKind, key string) error
	Close() error
}

// App is the top-level TUI application, wiring layout, views, and keybindings.
type App struct {
	tapp     *tview.Application
	model    *Model
	layout   *tview.Flex
	registry *ViewRegistry
	bus      *EventBus
	header   *tview.TextView
	cmdBar   *tview.InputField
	keys     *KeyHandler
	table    *views.ResourceTable
	detail   *views.ResourceDetail
	drift    *views.DriftView
	confirm      *views.ConfirmModal
	traceView    *views.TraceView
	traceHandler *trace.RingHandler

	// Refresh infrastructure
	refreshMu sync.Mutex
	refreshCh chan struct{} // manual refresh trigger
	cancel    context.CancelFunc

	// Integration points
	Provider ProviderAPI
	Engine   *reconciler.Engine
	Manifest *manifest.Manifest

	// Attach mode — poll a running gcplane serve instance
	attachClient *AttachClient
	tenant       string // current tenant in multi-tenant attach mode
}

// Config holds the parameters for creating a new TUI App.
type Config struct {
	Manifest *manifest.Manifest
	Endpoint string
	Provider ProviderAPI
	Engine   *reconciler.Engine
	Interval string // e.g. "10s"
	Attach       string               // optional: URL of running gcplane serve instance
	TraceHandler *trace.RingHandler   // optional: ring handler for trace capture
}

// NewApp creates and wires the TUI application.
func NewApp(cfg Config) (*App, error) {
	interval, err := time.ParseDuration(cfg.Interval)
	if err != nil {
		return nil, fmt.Errorf("invalid interval %q: %w", cfg.Interval, err)
	}

	app := &App{
		tapp:      tview.NewApplication(),
		Provider:  cfg.Provider,
		Engine:    cfg.Engine,
		Manifest:  cfg.Manifest,
		refreshCh: make(chan struct{}, 1),
	}

	// Attach mode — connect to running serve instance
	if cfg.Attach != "" {
		client := NewAttachClient(cfg.Attach)
		if err := client.Healthcheck(); err != nil {
			return nil, err
		}
		app.attachClient = client
		app.Provider = &stubProvider{baseURL: cfg.Attach}
		app.model = NewModel(nil, cfg.Attach, interval)
		app.model.manifestName = "attached: " + cfg.Attach
	} else {
		app.model = NewModel(cfg.Manifest, cfg.Endpoint, interval)
	}

	app.bus = NewEventBus(app.tapp)
	app.traceHandler = cfg.TraceHandler
	if app.traceHandler != nil {
		app.traceHandler.SetOnEntry(func(e trace.Entry) {
			app.bus.Publish(Event{Type: EventTraceEntry, Payload: e})
		})
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
	a.header.SetBackgroundColor(views.ColorMantle)
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

	// View registry — manages all pages and navigation stack
	pages := tview.NewPages()
	a.registry = NewViewRegistry(pages, a.tapp)
	a.registry.Register(a.table)
	a.registry.Register(a.detail)
	a.registry.Register(a.drift)
	a.registry.Register(a.confirm)
	a.registry.Register(views.NewHelpView(helpText()))

	// Trace view — register if handler is available
	if a.traceHandler != nil {
		a.traceView = views.NewTraceView(a.traceHandler)
		a.registry.Register(a.traceView)

		// Auto-refresh trace view when new entries arrive
		a.bus.Subscribe(EventTraceEntry, func(_ Event) {
			if a.registry.Current() == "trace" {
				a.traceView.Refresh()
			}
		})
	}

	// Show main view by default
	pages.SwitchToPage("main")

	// Command bar — 1 row input
	a.cmdBar = tview.NewInputField().
		SetLabel("").
		SetFieldWidth(0).
		SetDoneFunc(a.onCommandDone)
	a.cmdBar.SetFieldBackgroundColor(views.ColorMantle)
	a.cmdBar.SetLabelColor(views.ColorMauve)

	// Root layout: header(1) + pages(flex) + cmdbar(1)
	a.layout = tview.NewFlex().SetDirection(tview.FlexRow).
		AddItem(a.header, 1, 0, false).
		AddItem(a.registry.Pages(), 0, 1, true).
		AddItem(a.cmdBar, 1, 0, false)
}

// Run starts the TUI event loop. Blocks until the app exits.
func (a *App) Run() error {
	a.registry.Push("main")

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

	if a.attachClient != nil {
		a.refreshFromServe()
	} else {
		a.refreshDirect()
	}
}

// refreshDirect does a direct dry-run reconciliation against the GoClaw API.
func (a *App) refreshDirect() {
	plan, _ := a.Engine.Reconcile(context.Background(), a.Manifest, reconciler.ReconcileOpts{DryRun: true})
	a.model.UpdatePlan(plan)

	a.tapp.QueueUpdateDraw(func() {
		a.table.Refresh(a.model.GetChanges())
		a.updateHeader()
	})
	a.bus.Publish(Event{Type: EventPlanUpdated})
}

// refreshFromServe polls the gcplane serve HTTP API for status.
func (a *App) refreshFromServe() {
	var changes []reconciler.Change

	if a.tenant != "" {
		// Fetch specific tenant status
		status, err := a.attachClient.FetchTenantStatus(a.tenant)
		if err != nil {
			a.model.SetError(err)
			return
		}
		changes = StatusToChanges(status)
	} else {
		// Fetch single-tenant or aggregated status
		status, err := a.attachClient.FetchStatus()
		if err != nil {
			a.model.SetError(err)
			return
		}
		changes = StatusToChanges(status)
	}

	// Build a synthetic plan from the status
	plan := &reconciler.Plan{Changes: changes}
	for _, c := range changes {
		switch c.Action {
		case reconciler.ActionNoop:
			plan.Noops++
		case reconciler.ActionCreate:
			plan.Creates++
		case reconciler.ActionUpdate:
			plan.Updates++
		case reconciler.ActionDelete:
			plan.Deletes++
		}
		if c.Error != "" {
			plan.Errors = append(plan.Errors, c.Error)
		}
	}
	a.model.UpdatePlan(plan)

	a.tapp.QueueUpdateDraw(func() {
		a.table.Refresh(a.model.GetChanges())
		a.updateHeader()
	})
	a.bus.Publish(Event{Type: EventPlanUpdated})
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

	sep := views.HeaderSep()
	kindLabel := views.Tag(views.HexGreen, "all")
	if kind != "" {
		kindLabel = views.Tag(views.HexGreen, string(kind))
	}

	// Status summary from current changes
	summary := ""
	if changes := a.model.GetChanges(); len(changes) > 0 {
		summary = sep + views.StatusSummaryColored(changes)
	}

	lastRefresh := a.model.GetLastRefresh()
	age := ""
	if !lastRefresh.IsZero() {
		age = sep + views.Tag(views.HexOverlay0, formatDuration(time.Since(lastRefresh))+" ago")
	}

	mode := ""
	if a.attachClient != nil {
		mode = sep + views.Tag(views.HexBlue, "attach")
		if a.tenant != "" {
			mode = sep + views.Tag(views.HexBlue, "tenant:"+a.tenant)
		}
	}

	text := fmt.Sprintf(" %s%s%s%s%s%s%s%s",
		views.BoldTag(views.HexMauve, "gcplane"), sep, name, sep, ep, sep, kindLabel, mode) + summary + age
	a.header.SetText(text)
}

// switchKind changes the kind filter and refreshes the table.
func (a *App) switchKind(kind manifest.ResourceKind) {
	a.model.SetKind(kind)
	a.refreshTable()
}

// pushView navigates to a named page, preserving the stack for Esc.
func (a *App) pushView(name string) {
	a.registry.Push(name)
}

// popView returns to the previous page in the view stack.
func (a *App) popView() {
	page := a.registry.Pop()
	if page == "main" {
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
	if a.registry.Current() == "help" {
		a.popView()
	} else {
		a.pushView("help")
	}
}

// activateCommandBar focuses the command bar input.
func (a *App) activateCommandBar() {
	a.cmdBar.SetLabel(":")
	a.cmdBar.SetText("")
	a.tapp.SetFocus(a.cmdBar)
}

// deactivateCommandBar returns focus to the main content.
func (a *App) deactivateCommandBar() {
	a.cmdBar.SetLabel("")
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
	"cron":         manifest.KindCronJob,
	"cronjob":      manifest.KindCronJob,
	"team":         manifest.KindAgentTeam,
	"agentteam":    manifest.KindAgentTeam,
	"sysconfig":    manifest.KindSystemConfig,
	"systemconfig": manifest.KindSystemConfig,
	"cli":          manifest.KindSecureCLI,
	"securecli":    manifest.KindSecureCLI,
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
	case "sync":
		a.triggerRemoteSync()
		return
	}

	// Tenant switching: ":tenant <name>" or ":tenant" to clear
	if strings.HasPrefix(cmd, "tenant") {
		a.handleTenantCommand(cmd)
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

// handleTenantCommand processes ":tenant <name>" or ":tenant" to clear.
func (a *App) handleTenantCommand(cmd string) {
	if a.attachClient == nil {
		a.showStatus(views.Tag(views.HexYellow, "Tenant switching only available in attach mode (--attach)"))
		return
	}
	parts := strings.Fields(cmd)
	if len(parts) == 1 {
		// Clear tenant filter
		a.tenant = ""
		a.model.manifestName = "attached: " + a.attachClient.baseURL
		a.showStatus(views.Tag(views.HexGreen, "Showing all tenants"))
	} else {
		a.tenant = parts[1]
		a.model.manifestName = "tenant: " + a.tenant
		a.showStatus(views.Tag(views.HexGreen, fmt.Sprintf("Switched to tenant: %s", a.tenant)))
	}
	a.triggerRefresh()
}

// triggerRemoteSync triggers a sync on the remote serve instance.
func (a *App) triggerRemoteSync() {
	if a.attachClient == nil {
		// In direct mode, just apply
		a.applyAll()
		return
	}
	go func() {
		var err error
		if a.tenant != "" {
			err = a.attachClient.TriggerTenantSync(a.tenant)
		} else {
			err = a.attachClient.TriggerSync()
		}
		a.tapp.QueueUpdateDraw(func() {
			if err != nil {
				a.showStatus(views.Tag(views.HexRed, fmt.Sprintf("Sync trigger failed: %s", err)))
			} else {
				a.showStatus(views.Tag(views.HexGreen, "Sync triggered"))
			}
		})
		// Refresh after a brief delay to let the sync complete
		time.Sleep(2 * time.Second)
		a.refresh()
	}()
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
		a.cmdBar.SetLabel("")
		a.cmdBar.SetDoneFunc(a.onCommandDone)
		a.deactivateCommandBar()
		a.refreshTable()
		return
	}
	if key == tcell.KeyEnter {
		filter := a.cmdBar.GetText()
		a.model.SetFilter(filter)
		a.keys.mode = ModeNormal
		a.cmdBar.SetLabel("")
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
	h := func(title string) string { return views.BoldTag(views.HexYellow, title) }
	k := func(key string) string { return views.Tag(views.HexBlue, key) }

	return fmt.Sprintf(`
 %s
   %s         Move down/up
   %s         Jump to top/bottom
   %s       View resource detail
   %s           Show drift diff
   %s         Back / Close overlay
   %s           Quit

 %s
   %s Provider   %s Agent      %s Channel
   %s MCPServer  %s Skill      %s CronJob
   %s AgentTeam  %s SysConfig  %s SecureCLI
   %s All

 %s
   %s   %s    %s   %s
   %s      %s     %s      %s
   %s        Show all resources
   %s       Show this help
   %s          Quit

 %s
   %s           Filter by name (case-insensitive)
   %s       Apply filter
   %s         Cancel / clear filter

 %s
   %s      Apply (reconcile all pending changes)
   %s      Delete selected resource
   %s           Edit selected resource ($EDITOR)
   %s      Apply all changes
   %s     Delete selected resource
   %s       Trigger sync (attach mode)
   %s   Switch to tenant X (attach mode)
   %s     Clear tenant filter

 %s
   %s           Toggle trace log
   %s       Pause/resume auto-scroll
   %s         Filter level (DEBUG/INFO/WARN/ERROR)
   %s           Reset filters

 %s
   %s           Toggle this help
   %s           Refresh now
`,
		h("Navigation"),
		k("j/k"), k("g/G"), k("Enter"), k("d"), k("Esc"), k("q"),
		h("Kind Filter"),
		k("1"), k("2"), k("3"),
		k("4"), k("5"), k("6"),
		k("7"), k("8"), k("9"),
		k("0"),
		h("Commands"),
		k(":provider"), k(":agent"), k(":channel"), k(":mcp"),
		k(":skill"), k(":cron"), k(":team"), k(":cli"),
		k(":all"), k(":help"), k(":q"),
		h("Search"),
		k("/"), k("Enter"), k("Esc"),
		h("Actions"),
		k("Ctrl+R"), k("Ctrl+D"), k("e"),
		k(":apply"), k(":delete"), k(":sync"),
		k(":tenant X"), k(":tenant"),
		h("Trace View"),
		k("t"), k("Space"), k("1-4"), k("c"),
		h("Other"),
		k("?"), k("r"),
	)
}
