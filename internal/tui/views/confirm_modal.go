package views

import (
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

// ConfirmModal is a centered confirmation dialog with Yes/No buttons.
type ConfirmModal struct {
	grid *tview.Grid // outer centering wrapper
	form *tview.Form
	text *tview.TextView
}

// NewConfirmModal creates a modal dialog for destructive action confirmation.
func NewConfirmModal() *ConfirmModal {
	cm := &ConfirmModal{}
	cardBg := ColorSurface0

	// Message text
	cm.text = tview.NewTextView().
		SetTextAlign(tview.AlignCenter).
		SetTextColor(ColorText).
		SetDynamicColors(true)
	cm.text.SetBackgroundColor(cardBg)

	// Button form
	cm.form = tview.NewForm().
		SetButtonsAlign(tview.AlignCenter).
		SetButtonBackgroundColor(ColorSurface1).
		SetButtonTextColor(ColorText).
		SetButtonActivatedStyle(tcell.StyleDefault.
			Foreground(tcell.GetColor("#1e1e2e")).
			Background(tcell.GetColor("#cba6f7")))
	cm.form.SetBackgroundColor(cardBg)
	cm.form.AddButton("Yes", nil).AddButton("No", nil)

	// Arrow keys navigate between buttons
	cm.form.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		switch event.Key() {
		case tcell.KeyLeft, tcell.KeyUp:
			return tcell.NewEventKey(tcell.KeyBacktab, 0, tcell.ModNone)
		case tcell.KeyRight, tcell.KeyDown:
			return tcell.NewEventKey(tcell.KeyTab, 0, tcell.ModNone)
		}
		return event
	})

	// Card: single border, uniform background
	card := tview.NewFlex().SetDirection(tview.FlexRow).
		AddItem(cm.text, 0, 1, false).
		AddItem(cm.form, 3, 0, true)
	card.SetBackgroundColor(cardBg).
		SetBorder(true).
		SetBorderColor(ColorOverlay0).
		SetTitle(" Confirm ").
		SetTitleColor(ColorMauve).
		SetBorderPadding(1, 0, 2, 2)

	// Center with grid
	cm.grid = tview.NewGrid().
		SetColumns(0, 42, 0).
		SetRows(0, 10, 0)
	cm.grid.SetBackgroundColor(ColorBase)
	cm.grid.AddItem(card, 1, 1, 1, 1, 0, 0, true)

	return cm
}

// Name implements View.
func (cm *ConfirmModal) Name() string { return "confirm" }

// Primitive implements View.
func (cm *ConfirmModal) Primitive() tview.Primitive { return cm.grid }

// Activate implements View.
func (cm *ConfirmModal) Activate() {}

// Focusable returns the form for tapp.SetFocus.
func (cm *ConfirmModal) Focusable() tview.Primitive { return cm.form }

// Show displays the modal with a message and calls onConfirm(true/false).
func (cm *ConfirmModal) Show(message string, onConfirm func(confirmed bool)) {
	cm.text.SetText("\n" + message)
	cm.form.GetButton(0).SetSelectedFunc(func() { onConfirm(true) })
	cm.form.GetButton(1).SetSelectedFunc(func() { onConfirm(false) })
	// Focus "No" button by default for safety
	cm.form.SetFocus(1)
}

