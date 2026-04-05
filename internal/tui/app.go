package tui

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/dataplanelabs/gcplane/internal/manifest"
	"github.com/dataplanelabs/gcplane/internal/provider/goclaw"
	"github.com/dataplanelabs/gcplane/internal/reconciler"
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

// LogTailer is optionally implemented by providers that support log streaming.
type LogTailer interface {
	StartLogTail(ctx context.Context, level string) error
	StopLogTail(ctx context.Context) error
}

// TraceFetcher is optionally implemented by providers that expose LLM trace data.
type TraceFetcher interface {
	ListTraces(ctx context.Context, f views.TraceFilters) ([]views.TraceData, int, error)
	GetTrace(ctx context.Context, traceID string) (*views.TraceData, []views.SpanData, error)
}

// EventListenerSetup is optionally implemented by providers that support WS event streaming.
type EventListenerSetup interface {
	SetEventHandler(h goclaw.WSEventHandler)
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
	confirm     *views.ConfirmModal
	liveStore   *LiveStore
	logsPanel   *views.LogsPanel
	tracesPanel *views.TracesPanel
	traceStore  *TraceStore

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

	statusTimer *time.Timer // guards against goroutine leak in showStatus
}

// Config holds the parameters for creating a new TUI App.
type Config struct {
	Manifest *manifest.Manifest
	Endpoint string
	Provider ProviderAPI
	Engine   *reconciler.Engine
	Interval string // e.g. "10s"
	Attach string // optional: URL of running gcplane serve instance
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
		app.model.SetManifestName("attached: " + cfg.Attach)
	} else {
		app.model = NewModel(cfg.Manifest, cfg.Endpoint, interval)
	}

	app.bus = NewEventBus(app.tapp)

	// LiveStore — ring-buffered state from WS events
	app.liveStore = NewLiveStore(500, 500)

	// TraceStore — LLM agent traces from /v1/traces API
	app.traceStore = NewTraceStore()

	// Wire WS event handler → LiveStore + TraceStore (if provider supports it)
	if setup, ok := app.Provider.(EventListenerSetup); ok {
		setup.SetEventHandler(func(frame goclaw.WSEventFrame) {
			app.liveStore.HandleEvent(frame)
			if frame.Event == "trace.updated" {
				app.traceStore.NotifyTraceUpdated()
				// Also refresh spans if viewing a trace at level 1
				if app.tracesPanel != nil && app.tracesPanel.Level >= 1 {
					app.traceStore.NotifyDetailDirty()
				}
			}
		})
	}

	app.keys = NewKeyHandler(app)
	app.buildLayout()
	app.tapp.SetInputCapture(app.keys.Handle)

	return app, nil
}

// buildLayout creates the static 3-row layout: header(2), pages(flex), command bar(1).
func (a *App) buildLayout() {
	// Header bar — 2 rows (metadata + tab bar)
	a.header = tview.NewTextView().
		SetDynamicColors(true).
		SetTextAlign(tview.AlignLeft)
	a.header.SetBackgroundColor(views.ColorMantle)
	a.updateHeader()

	// Resource table — State tab primary view
	a.table = views.NewResourceTable()
	a.table.OnSelect = func(c reconciler.Change) {
		a.showDetail(c)
	}
	a.table.OnDrift = func(c reconciler.Change) {
		a.showDrift(c)
	}

	// Overlay views
	a.detail = views.NewResourceDetail()
	a.drift = views.NewDriftView()
	a.confirm = views.NewConfirmModal()

	// View registry
	pages := tview.NewPages()
	a.registry = NewViewRegistry(pages, a.tapp)

	// Register primary tab views
	a.registry.Register(a.table)

	a.logsPanel = views.NewLogsPanel()
	a.logsPanel.OnCopy = func(text string) {
		_ = views.CopyToClipboard(text)
		a.showStatus(views.Tag(views.HexGreen, "Copied log entry"))
	}
	a.registry.Register(a.logsPanel)

	a.tracesPanel = views.NewTracesPanel()
	a.registry.Register(a.tracesPanel)

	// Register overlay views
	a.registry.Register(a.detail)
	a.registry.Register(a.drift)
	a.registry.Register(a.confirm)
	a.registry.Register(views.NewHelpView(helpText()))

	// Command bar
	a.cmdBar = tview.NewInputField().
		SetLabel("").
		SetFieldWidth(0).
		SetDoneFunc(a.onCommandDone)
	a.cmdBar.SetFieldBackgroundColor(views.ColorMantle)
	a.cmdBar.SetLabelColor(views.ColorMauve)

	// Static layout: header(2) + pages(flex) + cmdbar(1)
	a.layout = tview.NewFlex().SetDirection(tview.FlexRow).
		AddItem(a.header, 2, 0, false).
		AddItem(a.registry.Pages(), 0, 1, true).
		AddItem(a.cmdBar, 1, 0, false)
}

// Run starts the TUI event loop. Blocks until the app exits.
func (a *App) Run() error {
	a.registry.SwitchTab(TabState)

	ctx, cancel := context.WithCancel(context.Background())
	a.cancel = cancel

	// Initial data load + start refresh loop
	go a.refresh()
	go a.refreshLoop(ctx)
	go a.tabRefreshLoop(ctx)

	err := a.tapp.SetRoot(a.layout, true).EnableMouse(false).Run()
	cancel() // stop refresh loop on exit
	return err
}

// tabRefreshLoop redraws the active tab content on a 150ms tick.
func (a *App) tabRefreshLoop(ctx context.Context) {
	ticker := time.NewTicker(150 * time.Millisecond)
	defer ticker.Stop()
	var tickCount int

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			tickCount++
			// Traces: poll every ~3s (20 ticks) for live updates
			if tickCount%20 == 0 {
				a.traceStore.NotifyTraceUpdated()
			}
			a.refreshTraces()

			if !a.liveStore.IsDirty() {
				continue
			}
			a.tapp.QueueUpdateDraw(func() {
				if a.registry.ActiveTab() == TabLogs {
					a.logsPanel.Refresh(a.liveStore.Logs())
				}
				a.liveStore.MarkClean()
			})
		}
	}
}

// Stop gracefully shuts down the TUI.
func (a *App) Stop() {
	if a.cancel != nil {
		a.cancel()
	}
	a.tapp.Stop()
}

// refreshActiveTab refreshes only the currently active tab's data.
func (a *App) refreshActiveTab() {
	switch a.registry.ActiveTab() {
	case TabState:
		a.triggerRefresh()
	case TabTraces:
		a.traceStore.NotifyTraceUpdated()
		if a.tracesPanel.Level >= 1 {
			a.traceStore.NotifyDetailDirty()
		}
	case TabLogs:
		// Logs are live-streamed; force a redraw
		a.tapp.QueueUpdateDraw(func() {
			a.logsPanel.Refresh(a.liveStore.Logs())
		})
	}
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
		if len(plan.Errors) > 0 {
			a.showStatus(views.Tag(views.HexRed, fmt.Sprintf("%d observe error(s): %s", len(plan.Errors), plan.Errors[0])))
		}
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

	metadataLine := fmt.Sprintf(" %s%s%s%s%s%s%s%s",
		views.BoldTag(views.HexMauve, "gcplane"), sep, name, sep, ep, sep, kindLabel, mode) + summary + age

	tabBar := ""
	if a.registry != nil {
		tabBar = a.registry.TabBar()
	}
	a.header.SetText(metadataLine + "\n" + tabBar)
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

// switchTab switches to a primary tab, managing log tail lifecycle.
func (a *App) switchTab(tab PrimaryTab) {
	prev := a.registry.ActiveTab()
	a.registry.SwitchTab(tab)
	a.updateHeader()

	// Log tail lifecycle
	if tab == TabLogs && prev != TabLogs {
		go a.startLogTail()
	} else if tab != TabLogs && prev == TabLogs {
		go a.stopLogTail()
	}

	// Initial trace list fetch when entering Traces tab
	if tab == TabTraces && prev != TabTraces {
		a.traceStore.NotifyTraceUpdated()
	}

	a.focusActiveTab()
}

// focusActiveTab sets focus to the current tab's primary widget.
func (a *App) focusActiveTab() {
	switch a.registry.ActiveTab() {
	case TabState:
		a.tapp.SetFocus(a.table.Table)
	case TabTraces:
		a.tapp.SetFocus(a.tracesPanel.FocusPrimitive())
	case TabLogs:
		a.tapp.SetFocus(a.logsPanel.Table)
	}
}

// popView returns to the previous page in the view stack.
func (a *App) popView() {
	a.registry.Pop()
	a.focusActiveTab()
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

// startLogTail subscribes to GoClaw logs if provider supports it.
func (a *App) startLogTail() {
	tailer, ok := a.Provider.(LogTailer)
	if !ok {
		a.tapp.QueueUpdateDraw(func() {
			a.logsPanel.RefreshUnavailable("logs.tail not available (provider does not support log streaming)")
		})
		return
	}
	if a.liveStore.IsTailing() {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := tailer.StartLogTail(ctx, "info"); err != nil {
		a.tapp.QueueUpdateDraw(func() {
			a.logsPanel.RefreshUnavailable("logs.tail: " + err.Error())
		})
		return
	}
	a.liveStore.SetTailing(true)
}

// stopLogTail unsubscribes from GoClaw log stream.
func (a *App) stopLogTail() {
	if !a.liveStore.IsTailing() {
		return
	}
	a.liveStore.SetTailing(false)
	if tailer, ok := a.Provider.(LogTailer); ok {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = tailer.StopLogTail(ctx)
	}
}

// refreshTraces handles async trace data refresh from TraceStore.
func (a *App) refreshTraces() {
	if a.tracesPanel.IsPaused() {
		return
	}

	fetcher, ok := a.Provider.(TraceFetcher)
	if !ok {
		if a.registry.ActiveTab() == TabTraces {
			a.tapp.QueueUpdateDraw(func() {
				a.tracesPanel.RefreshUnavailable("Traces not available (provider does not support trace API)")
			})
		}
		return
	}

	if a.traceStore.NeedsListRefresh() {
		go func() {
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			if err := a.traceStore.RefreshList(ctx, fetcher); err != nil {
				return // silently skip; will retry next tick
			}
			a.tapp.QueueUpdateDraw(func() {
				if a.registry.ActiveTab() == TabTraces && a.tracesPanel.Level == 0 {
					a.tracesPanel.Refresh(
						a.traceStore.Traces(),
						a.traceStore.Total(),
						a.traceStore.SelectedID(),
					)
				}
			})
		}()
	}

	// Live span refresh: when viewing a running trace at level 1,
	// periodically re-fetch its spans so new steps appear in real time.
	if a.tracesPanel.Level == 1 && a.registry.ActiveTab() == TabTraces {
		trace := a.traceStore.SelectedTrace()
		if trace != nil && (trace.Status == "running" || trace.Status == "pending") {
			a.traceStore.NotifyDetailDirty()
		}
	}

	if a.traceStore.NeedsDetailRefresh() {
		go func() {
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			if err := a.traceStore.RefreshDetail(ctx, fetcher); err != nil {
				return
			}
			a.tapp.QueueUpdateDraw(func() {
				if a.registry.ActiveTab() == TabTraces && a.tracesPanel.Level == 1 {
					spans := a.traceStore.SelectedSpans()
					if spans != nil {
						a.tracesPanel.SpanListView().Refresh(spans)
					}
				}
			})
		}()
	}
}

// drillIntoTrace fetches trace detail and navigates to the span list.
func (a *App) drillIntoTrace() {
	id := a.tracesPanel.List().SelectedTraceID()
	if id == "" {
		return
	}

	fetcher, ok := a.Provider.(TraceFetcher)
	if !ok {
		a.showStatus(views.Tag(views.HexRed, "Trace API not available"))
		return
	}

	// Select and fetch
	a.traceStore.SelectTrace(id)
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := a.traceStore.RefreshDetail(ctx, fetcher); err != nil {
			a.tapp.QueueUpdateDraw(func() {
				a.showStatus(views.Tag(views.HexRed, "Failed to fetch trace: "+err.Error()))
			})
			return
		}
		a.tapp.QueueUpdateDraw(func() {
			trace := a.traceStore.SelectedTrace()
			spans := a.traceStore.SelectedSpans()
			if trace != nil {
				a.tracesPanel.ShowSpans(trace, spans, a.tapp)
			}
		})
	}()
}

// applySearch applies a search filter to the active tab.
func (a *App) applySearch(filter string) {
	switch a.registry.ActiveTab() {
	case TabState:
		a.model.SetFilter(filter)
		a.refreshTable()
	case TabTraces:
		a.tracesPanel.SetFilter(filter)
		// Re-render the current level with filter
		if a.tracesPanel.Level == 0 {
			a.tracesPanel.Refresh(
				a.traceStore.Traces(),
				a.traceStore.Total(),
				a.traceStore.SelectedID(),
			)
		}
	case TabLogs:
		a.logsPanel.SetFilter(filter)
	}
}

// clearSearch clears the active search filter for the current tab.
func (a *App) clearSearch() {
	switch a.registry.ActiveTab() {
	case TabState:
		a.model.SetFilter("")
		a.refreshTable()
	case TabTraces:
		a.tracesPanel.SetFilter("")
	case TabLogs:
		a.logsPanel.SetFilter("")
	}
}

// activateCommandBar focuses the command bar input.
func (a *App) activateCommandBar() {
	a.cmdBar.SetLabel(":")
	a.cmdBar.SetText("")
	a.tapp.SetFocus(a.cmdBar)
}

// deactivateCommandBar returns focus to the active tab's content.
func (a *App) deactivateCommandBar() {
	a.cmdBar.SetLabel("")
	a.cmdBar.SetText("")
	a.focusActiveTab()
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
	case "state":
		a.switchTab(TabState)
		return
	case "logs":
		a.switchTab(TabLogs)
		return
	case "traces":
		a.switchTab(TabTraces)
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
		a.model.SetManifestName("attached: " + a.attachClient.baseURL)
		a.showStatus(views.Tag(views.HexGreen, "Showing all tenants"))
	} else {
		a.tenant = parts[1]
		a.model.SetManifestName("tenant: " + a.tenant)
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

// onSearchDone handles search input completion, dispatching to the active tab.
func (a *App) onSearchDone(key tcell.Key) {
	if key == tcell.KeyEscape {
		a.clearSearch()
		a.keys.mode = ModeNormal
		a.cmdBar.SetLabel("")
		a.cmdBar.SetDoneFunc(a.onCommandDone)
		a.deactivateCommandBar()
		return
	}
	if key == tcell.KeyEnter {
		filter := a.cmdBar.GetText()
		a.applySearch(filter)
		a.keys.mode = ModeNormal
		a.cmdBar.SetLabel("")
		a.cmdBar.SetDoneFunc(a.onCommandDone)
		a.deactivateCommandBar()
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
   %s State(S)   %s Traces(T)   %s Logs(L)

 %s
   %s           Move down / up
   %s          Go to top / bottom
   %s          Copy selected to clipboard
   %s %s   Half-page down / up
   %s %s     Refresh active tab / quit

 %s
   %s           Search / filter current view
   %s         Apply filter
   %s         Clear filter / cancel

 %s (drill-down: Traces > Spans > Detail)
   %s %s   Drill in / drill out
   %s         Back to trace list (root)
   %s %s     Pause / resume auto-refresh
   %s           Clear filters
   Live: auto-polls every 3s + WS events

 %s (resource filter)
   %s Provider   %s Agent      %s Channel
   %s MCPServer  %s Skill      %s CronJob
   %s AgentTeam  %s SysConfig  %s SecureCLI
   %s All kinds

 %s (level filter)
   %s DEBUG+   %s INFO+   %s WARN+   %s ERROR

 %s (State tab only)
   %s     Edit resource ($EDITOR)
   %s     Delete selected resource
   %s     Show drift diff
   %s      Apply all   %s Sync (attach)
   %s       Tenant switch

 %s
   %s           Command mode
   %s           This help
`,
		h("Tabs"),
		k("●"), k("○"), k("○"),
		h("Navigation (all tabs)"),
		k("j/k"), k("gg/G"), k("yy"),
		k("Ctrl+D"), k("Ctrl+U"),
		k("Ctrl+R/r"), k("q"),
		h("Search"),
		k("/"), k("Enter"), k("Esc"),
		h("Traces"),
		k("l/Enter"), k("h"),
		k("Esc"),
		k("Space"), k("p"),
		k("c"),
		h("State"),
		k("1"), k("2"), k("3"),
		k("4"), k("5"), k("6"),
		k("7"), k("8"), k("9"),
		k("0"),
		h("Logs"),
		k("1"), k("2"), k("3"), k("4"),
		h("Actions"),
		k("Ctrl+E"), k("Ctrl+D"), k("d"),
		k(":apply"), k(":sync"),
		k(":tenant X"),
		h("Other"),
		k(":"), k("?"),
	)
}
