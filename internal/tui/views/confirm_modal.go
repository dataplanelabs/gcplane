package views

import (
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

// ConfirmModal is a centered confirmation dialog with Yes/No buttons.
type ConfirmModal struct {
	layout *tview.Flex // outer centering wrapper
	form   *tview.Form
	text   *tview.TextView
}

// NewConfirmModal creates a modal dialog for destructive action confirmation.
func NewConfirmModal() *ConfirmModal {
	cm := &ConfirmModal{}

	// Message text
	cm.text = tview.NewTextView().
		SetTextAlign(tview.AlignCenter).
		SetTextColor(ColorText).
		SetDynamicColors(true)
	cm.text.SetBackgroundColor(ColorMantle)

	// Button form
	cm.form = tview.NewForm().
		SetButtonsAlign(tview.AlignCenter).
		SetButtonBackgroundColor(ColorSurface1).
		SetButtonTextColor(ColorText).
		SetButtonActivatedStyle(tcell.StyleDefault.
			Foreground(tcell.GetColor("#1e1e2e")).
			Background(tcell.GetColor("#cba6f7")))
	cm.form.SetBackgroundColor(ColorMantle)
	cm.form.AddButton("Yes", nil).AddButton("No", nil)

	// Inner card: text + buttons, single border
	card := tview.NewFlex().SetDirection(tview.FlexRow).
		AddItem(cm.text, 0, 1, false).
		AddItem(cm.form, 1, 0, true)
	card.SetBackgroundColor(ColorMantle).
		SetBorder(true).
		SetBorderColor(ColorSurface1).
		SetTitle(" Confirm ").
		SetTitleColor(ColorMauve).
		SetBorderPadding(1, 1, 2, 2)

	// Center the card on screen
	cm.layout = tview.NewFlex().
		AddItem(nil, 0, 1, false).
		AddItem(tview.NewFlex().SetDirection(tview.FlexRow).
			AddItem(nil, 0, 1, false).
			AddItem(card, 9, 0, true).
			AddItem(nil, 0, 1, false), 50, 0, true).
		AddItem(nil, 0, 1, false)
	cm.layout.SetBackgroundColor(tcell.ColorDefault)

	return cm
}

// Name implements View.
func (cm *ConfirmModal) Name() string { return "confirm" }

// Primitive implements View.
func (cm *ConfirmModal) Primitive() tview.Primitive { return cm.layout }

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

