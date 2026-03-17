package views

import (
	"github.com/gdamore/tcell/v2"
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
		SetBackgroundColor(tview.Styles.PrimitiveBackgroundColor)

	return &ConfirmModal{Modal: modal}
}

// Show displays the modal with a message and calls onConfirm(true/false).
func (cm *ConfirmModal) Show(message string, onConfirm func(confirmed bool)) {
	cm.Modal.SetText(message)
	cm.Modal.SetDoneFunc(func(buttonIndex int, buttonLabel string) {
		onConfirm(buttonLabel == "Yes")
	})
	// Focus the "No" button by default for safety
	cm.Modal.SetFocus(1)
}

// HandleInput provides extra keybindings for the modal (y/n shortcuts).
func (cm *ConfirmModal) HandleInput(event *tcell.EventKey) *tcell.EventKey {
	switch event.Rune() {
	case 'y', 'Y':
		cm.Modal.SetFocus(0) // select Yes
		// Simulate Enter to trigger DoneFunc
		return tcell.NewEventKey(tcell.KeyEnter, 0, tcell.ModNone)
	case 'n', 'N':
		cm.Modal.SetFocus(1) // select No
		return tcell.NewEventKey(tcell.KeyEnter, 0, tcell.ModNone)
	}
	return event
}
