package views

import (
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

// VimScrollInput returns an input capture handler for vim-style scrolling on a TextView.
func VimScrollInput(tv *tview.TextView) func(*tcell.EventKey) *tcell.EventKey {
	return func(event *tcell.EventKey) *tcell.EventKey {
		switch event.Rune() {
		case 'j':
			row, col := tv.GetScrollOffset()
			tv.ScrollTo(row+1, col)
			return nil
		case 'k':
			row, col := tv.GetScrollOffset()
			if row > 0 {
				tv.ScrollTo(row-1, col)
			}
			return nil
		case 'g':
			tv.ScrollToBeginning()
			return nil
		case 'G':
			tv.ScrollToEnd()
			return nil
		}
		return event
	}
}
