// Package views contains TUI view components for gcplane top.
package views

import (
	"fmt"
	"sort"
	"strings"

	"github.com/dataplanelabs/gcplane/internal/manifest"
	"github.com/dataplanelabs/gcplane/internal/reconciler"
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

// statusColors maps resource status to terminal colors.
var statusColors = map[string]tcell.Color{
	"InSync":  tcell.ColorGreen,
	"Drifted": tcell.ColorYellow,
	"Missing": tcell.ColorRed,
	"Error":   tcell.ColorRed,
	"Extra":   tcell.ColorBlue,
}

// tableRow holds pre-computed data for one table row.
type tableRow struct {
	kind      manifest.ResourceKind
	name      string
	status    string
	driftInfo string
	change    reconciler.Change
}

// ResourceTable renders a k9s-style resource list with status coloring.
type ResourceTable struct {
	Table *tview.Table
	rows  []tableRow

	// Callbacks set by the app to handle navigation
	OnSelect func(change reconciler.Change) // Enter pressed
	OnDrift  func(change reconciler.Change) // d pressed
}

// NewResourceTable creates a table configured for resource browsing.
func NewResourceTable() *ResourceTable {
	rt := &ResourceTable{
		Table: tview.NewTable(),
	}

	rt.Table.SetSelectable(true, false)  // row selection only
	rt.Table.SetFixed(1, 0)              // fixed header row
	rt.Table.SetBorders(false)           // cleaner k9s look
	rt.Table.SetSeparator(' ')

	// Handle table-specific keybindings
	rt.Table.SetInputCapture(rt.handleInput)

	return rt
}

// Refresh rebuilds the table from the given changes.
func (rt *ResourceTable) Refresh(changes []reconciler.Change) {
	rt.Table.Clear()
	rt.rows = nil

	// Header row
	headers := []string{"KIND", "NAME", "STATUS", "DRIFT"}
	for col, h := range headers {
		cell := tview.NewTableCell(h).
			SetTextColor(tcell.ColorWhite).
			SetAttributes(tcell.AttrBold).
			SetSelectable(false)
		if col == len(headers)-1 {
			cell.SetExpansion(1) // DRIFT column expands
		}
		rt.Table.SetCell(0, col, cell)
	}

	// Sort changes: by kind order, then name alphabetically
	sorted := make([]reconciler.Change, len(changes))
	copy(sorted, changes)
	kindOrder := kindOrderMap()
	sort.Slice(sorted, func(i, j int) bool {
		oi, oj := kindOrder[sorted[i].Kind], kindOrder[sorted[j].Kind]
		if oi != oj {
			return oi < oj
		}
		return sorted[i].Name < sorted[j].Name
	})

	// Data rows
	for i, c := range sorted {
		row := toTableRow(c)
		rt.rows = append(rt.rows, row)
		rowIdx := i + 1 // offset for header

		// Kind cell
		rt.Table.SetCell(rowIdx, 0, tview.NewTableCell(string(row.kind)).
			SetTextColor(tcell.ColorWhite).SetMaxWidth(12))

		// Name cell
		rt.Table.SetCell(rowIdx, 1, tview.NewTableCell(row.name).
			SetTextColor(tcell.ColorWhite).SetMaxWidth(30))

		// Status cell with color
		color := statusColors[row.status]
		rt.Table.SetCell(rowIdx, 2, tview.NewTableCell(row.status).
			SetTextColor(color).SetMaxWidth(10))

		// Drift info cell
		rt.Table.SetCell(rowIdx, 3, tview.NewTableCell(row.driftInfo).
			SetTextColor(tcell.ColorGray).SetExpansion(1))
	}

	// Select first data row if available
	if len(rt.rows) > 0 {
		rt.Table.Select(1, 0)
	}
}

// GetSelectedChange returns the change for the currently selected row.
func (rt *ResourceTable) GetSelectedChange() *reconciler.Change {
	row, _ := rt.Table.GetSelection()
	idx := row - 1 // offset for header
	if idx < 0 || idx >= len(rt.rows) {
		return nil
	}
	return &rt.rows[idx].change
}

// handleInput processes table-specific key events.
func (rt *ResourceTable) handleInput(event *tcell.EventKey) *tcell.EventKey {
	switch event.Rune() {
	case 'd':
		if c := rt.GetSelectedChange(); c != nil && rt.OnDrift != nil {
			rt.OnDrift(*c)
			return nil
		}
	case 'g':
		if len(rt.rows) > 0 {
			rt.Table.Select(1, 0) // first data row
			return nil
		}
	case 'G':
		if len(rt.rows) > 0 {
			rt.Table.Select(len(rt.rows), 0) // last data row
			return nil
		}
	}

	// Enter key — select handler
	if event.Key() == tcell.KeyEnter {
		if c := rt.GetSelectedChange(); c != nil && rt.OnSelect != nil {
			rt.OnSelect(*c)
			return nil
		}
	}

	return event
}

// StatusSummary returns a formatted summary string like "12 InSync  2 Drifted  1 Missing".
func StatusSummary(changes []reconciler.Change) string {
	counts := map[string]int{}
	for _, c := range changes {
		counts[actionToStatus(c)]++
	}
	var parts []string
	for _, s := range []string{"InSync", "Drifted", "Missing", "Error", "Extra"} {
		if n := counts[s]; n > 0 {
			parts = append(parts, fmt.Sprintf("%d %s", n, s))
		}
	}
	if len(parts) == 0 {
		return "no resources"
	}
	return strings.Join(parts, "  ")
}

// toTableRow converts a reconciler.Change to a display-friendly tableRow.
func toTableRow(c reconciler.Change) tableRow {
	status := actionToStatus(c)
	drift := "-"
	if len(c.Diff) > 0 {
		fields := make([]string, 0, len(c.Diff))
		for k := range c.Diff {
			fields = append(fields, k)
		}
		sort.Strings(fields)
		drift = strings.Join(fields, ", ")
	}
	return tableRow{
		kind:      c.Kind,
		name:      c.Name,
		status:    status,
		driftInfo: drift,
		change:    c,
	}
}

// actionToStatus maps a reconciler Change to a display status string.
func actionToStatus(c reconciler.Change) string {
	if c.Error != "" {
		return "Error"
	}
	switch c.Action {
	case reconciler.ActionNoop:
		return "InSync"
	case reconciler.ActionUpdate:
		return "Drifted"
	case reconciler.ActionCreate:
		return "Missing"
	case reconciler.ActionDelete:
		return "Extra"
	default:
		return "Unknown"
	}
}

// kindOrderMap returns a map of ResourceKind to its position in ApplyOrder.
func kindOrderMap() map[manifest.ResourceKind]int {
	m := make(map[manifest.ResourceKind]int)
	for i, k := range manifest.ApplyOrder() {
		m[k] = i
	}
	return m
}
