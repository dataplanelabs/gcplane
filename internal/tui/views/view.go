package views

import "github.com/rivo/tview"

// View is the interface all TUI views must implement.
type View interface {
	// Name returns the unique page name for this view.
	Name() string
	// Primitive returns the tview widget for this view.
	Primitive() tview.Primitive
	// Activate is called when the view gains focus.
	Activate()
}
