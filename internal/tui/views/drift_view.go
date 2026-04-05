package views

import (
	"fmt"
	"sort"
	"strings"

	"github.com/dataplanelabs/gcplane/internal/reconciler"
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

// DriftView displays field-level drift between desired and actual state.
type DriftView struct {
	TextView *tview.TextView
}

// NewDriftView creates a scrollable drift diff view.
func NewDriftView() *DriftView {
	tv := tview.NewTextView().
		SetDynamicColors(true).
		SetScrollable(true).
		SetWordWrap(true)
	tv.SetBorder(true)
	tv.SetBorderColor(ColorSurface1)
	tv.SetTitleColor(ColorYellow)

	// Vim-style scrolling
	tv.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
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
	})

	return &DriftView{TextView: tv}
}

// Show renders the drift diff for a given change.
func (dv *DriftView) Show(c reconciler.Change) {
	dv.TextView.SetTitle(fmt.Sprintf(" Drift: %s/%s ", c.Kind, c.Name))
	dv.TextView.ScrollToBeginning()

	switch {
	case c.Error != "":
		dv.TextView.SetText(Tag(HexRed, fmt.Sprintf("Error: %s", c.Error)))

	case c.Action == reconciler.ActionNoop:
		dv.TextView.SetText(Tag(HexGreen, "No drift detected. Resource is in sync."))

	case c.Action == reconciler.ActionCreate:
		dv.TextView.SetText(Tag(HexYellow, "Resource missing in GoClaw. Will be created on apply."))

	case c.Action == reconciler.ActionDelete:
		dv.TextView.SetText(Tag(HexLavender, "Resource exists in GoClaw but not in manifest (orphan).\nWill be deleted on apply with --prune."))

	case c.Action == reconciler.ActionUpdate:
		dv.TextView.SetText(renderDiff(c.Diff))

	default:
		dv.TextView.SetText(Tag(HexOverlay0, "Unknown state."))
	}
}

// renderDiff formats field-level diffs with red/green coloring.
func renderDiff(diffs map[string]reconciler.FieldDiff) string {
	if len(diffs) == 0 {
		return Tag(HexGreen, "No differences found.")
	}

	keys := make([]string, 0, len(diffs))
	for k := range diffs {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	var b strings.Builder
	_, _ = fmt.Fprintf(&b, "%s\n\n", BoldTag(HexMauve, fmt.Sprintf("%d field(s) drifted:", len(diffs))))

	for i, k := range keys {
		d := diffs[k]
		_, _ = fmt.Fprintf(&b, "  %s\n", BoldTag(HexText, k+":"))
		_, _ = fmt.Fprintf(&b, "    %s\n", Tag(HexRed, "- "+formatDiffVal(d.Old)))
		_, _ = fmt.Fprintf(&b, "    %s\n", Tag(HexGreen, "+ "+formatDiffVal(d.New)))
		if i < len(keys)-1 {
			_, _ = fmt.Fprintf(&b, "  %s\n", Tag(HexOverlay0, "────────"))
		}
	}

	return b.String()
}

// formatDiffVal formats a diff value for display, truncating long values.
func formatDiffVal(v any) string {
	if v == nil {
		return "(none)"
	}
	s := fmt.Sprintf("%v", v)
	// Escape tview color tags
	s = strings.ReplaceAll(s, "[", "[[]")
	if len(s) > 80 {
		return s[:77] + "..."
	}
	return strings.TrimSpace(s)
}
