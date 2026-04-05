package views

import (
	"github.com/rivo/tview"
)

// ConfirmModal is a centered confirmation dialog with Yes/No buttons.
type ConfirmModal struct {
	Modal *tview.Modal
}

// NewConfirmModal creates a modal dialog for destructive action confirmation.
func NewConfirmModal() *ConfirmModal {
	modal := tview.NewModal().
		AddButtons([]string{"Yes", "No"}).
		SetBackgroundColor(ColorMantle).
		SetTextColor(ColorText).
		SetButtonBackgroundColor(ColorMauve).
		SetButtonTextColor(ColorBase)

	return &ConfirmModal{Modal: modal}
}

// Name implements View.
func (cm *ConfirmModal) Name() string { return "confirm" }

// Primitive implements View.
func (cm *ConfirmModal) Primitive() tview.Primitive { return cm.Modal }

// Activate implements View.
func (cm *ConfirmModal) Activate() {}

// Show displays the modal with a message and calls onConfirm(true/false).
func (cm *ConfirmModal) Show(message string, onConfirm func(confirmed bool)) {
	cm.Modal.SetText(message)
	cm.Modal.SetDoneFunc(func(buttonIndex int, buttonLabel string) {
		onConfirm(buttonLabel == "Yes")
	})
	// Focus the "No" button by default for safety
	cm.Modal.SetFocus(1)
}

