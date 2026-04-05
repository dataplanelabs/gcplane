package tui

import (
	"strings"

	"github.com/dataplanelabs/gcplane/internal/tui/views"
	"github.com/rivo/tview"
)

// PrimaryTab identifies a primary full-screen tab.
type PrimaryTab int

const (
	TabState  PrimaryTab = iota
	TabTraces
	TabLogs
)

// tabMeta maps tabs to page names and display labels.
var tabMeta = []struct {
	PageName string
	Label    string
	Key      string
}{
	{"main", "State", "S"},
	{"traces", "Traces", "T"},
	{"logs", "Logs", "L"},
}

// ViewRegistry manages named views backed by tview.Pages.
type ViewRegistry struct {
	pages        *tview.Pages
	views        map[string]views.View
	activeTab    PrimaryTab
	overlayStack []string
	tapp         *tview.Application
}

// NewViewRegistry creates a registry backed by the given Pages widget.
func NewViewRegistry(pages *tview.Pages, tapp *tview.Application) *ViewRegistry {
	return &ViewRegistry{
		pages: pages,
		views: make(map[string]views.View),
		tapp:  tapp,
	}
}

// Register adds a view to the registry and pages widget.
func (r *ViewRegistry) Register(v views.View) {
	r.views[v.Name()] = v
	r.pages.AddPage(v.Name(), v.Primitive(), true, false)
}

// SwitchTab switches to a primary tab, clearing any overlay stack.
func (r *ViewRegistry) SwitchTab(tab PrimaryTab) {
	r.activeTab = tab
	r.overlayStack = r.overlayStack[:0]
	pageName := tabMeta[tab].PageName
	r.pages.SwitchToPage(pageName)
	if v, ok := r.views[pageName]; ok {
		v.Activate()
	}
}

// Push navigates to an overlay view on top of the current tab.
func (r *ViewRegistry) Push(name string) {
	r.overlayStack = append(r.overlayStack, name)
	r.pages.SwitchToPage(name)
	if v, ok := r.views[name]; ok {
		v.Activate()
	}
}

// Pop returns to the previous overlay, or the active tab if stack is empty.
// Returns the name of the view now showing.
func (r *ViewRegistry) Pop() string {
	if len(r.overlayStack) > 0 {
		r.overlayStack = r.overlayStack[:len(r.overlayStack)-1]
	}
	if len(r.overlayStack) > 0 {
		name := r.overlayStack[len(r.overlayStack)-1]
		r.pages.SwitchToPage(name)
		return name
	}
	pageName := tabMeta[r.activeTab].PageName
	r.pages.SwitchToPage(pageName)
	return pageName
}

// Current returns the name of the current front page.
func (r *ViewRegistry) Current() string {
	if len(r.overlayStack) > 0 {
		return r.overlayStack[len(r.overlayStack)-1]
	}
	return tabMeta[r.activeTab].PageName
}

// ActiveTab returns the current primary tab.
func (r *ViewRegistry) ActiveTab() PrimaryTab { return r.activeTab }

// HasOverlay returns true when an overlay view is on the stack.
func (r *ViewRegistry) HasOverlay() bool { return len(r.overlayStack) > 0 }

// TabBar returns a colored tab bar string for the header.
func (r *ViewRegistry) TabBar() string {
	var parts []string
	for i, t := range tabMeta {
		if PrimaryTab(i) == r.activeTab {
			parts = append(parts, views.BoldTag(views.HexGreen, "\u25cf "+t.Label))
		} else {
			parts = append(parts, views.Tag(views.HexOverlay0, "\u25cb "+t.Label))
		}
	}
	return " " + strings.Join(parts, "  ")
}

// Get returns a registered view by name.
func (r *ViewRegistry) Get(name string) (views.View, bool) {
	v, ok := r.views[name]
	return v, ok
}

// Pages returns the underlying tview.Pages widget.
func (r *ViewRegistry) Pages() *tview.Pages {
	return r.pages
}

// Count returns the number of registered views.
func (r *ViewRegistry) Count() int {
	return len(r.views)
}
