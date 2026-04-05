package tui

import (
	"github.com/dataplanelabs/gcplane/internal/tui/views"
	"github.com/rivo/tview"
)

// ViewRegistry manages named views backed by tview.Pages.
type ViewRegistry struct {
	pages     *tview.Pages
	views     map[string]views.View
	viewStack []string
	tapp      *tview.Application
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

// Push navigates to a named view, preserving the stack for Esc.
func (r *ViewRegistry) Push(name string) {
	r.viewStack = append(r.viewStack, name)
	r.pages.SwitchToPage(name)
	if v, ok := r.views[name]; ok {
		v.Activate()
	}
}

// Pop returns to the previous view in the stack.
// Returns the name of the view now showing.
func (r *ViewRegistry) Pop() string {
	if len(r.viewStack) > 0 {
		r.viewStack = r.viewStack[:len(r.viewStack)-1]
	}
	if len(r.viewStack) > 0 {
		name := r.viewStack[len(r.viewStack)-1]
		r.pages.SwitchToPage(name)
		return name
	}
	r.pages.SwitchToPage("main")
	return "main"
}

// Current returns the name of the current front page.
func (r *ViewRegistry) Current() string {
	if len(r.viewStack) > 0 {
		return r.viewStack[len(r.viewStack)-1]
	}
	return "main"
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
