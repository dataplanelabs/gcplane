package views

import (
	"github.com/rivo/tview"
)

// HelpView displays the help overlay with keybinding reference.
type HelpView struct {
	TextView *tview.TextView
}

// NewHelpView creates a help view with the given content text.
func NewHelpView(helpText string) *HelpView {
	tv := tview.NewTextView().
		SetDynamicColors(true).
		SetText(helpText)
	tv.SetBorder(true).SetTitle(" Help (? to close) ")
	tv.SetBorderColor(ColorSurface1)
	tv.SetTitleColor(ColorMauve)

	return &HelpView{TextView: tv}
}

// Name implements View.
func (hv *HelpView) Name() string { return "help" }

// Primitive implements View.
func (hv *HelpView) Primitive() tview.Primitive { return hv.TextView }

// Activate implements View.
func (hv *HelpView) Activate() {}
