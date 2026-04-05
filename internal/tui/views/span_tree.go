package views

import (
	"fmt"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

// SpanTree renders a hierarchical span tree using tview.TreeView.
type SpanTree struct {
	Tree     *tview.TreeView
	OnDetail func(span SpanData) // called on Enter for span detail
	OnCopy   func(text string)   // called on y to copy span info
}

// NewSpanTree creates a span tree component.
func NewSpanTree() *SpanTree {
	tree := tview.NewTreeView()
	tree.SetBackgroundColor(ColorBase)
	tree.SetGraphicsColor(ColorOverlay0)

	st := &SpanTree{Tree: tree}
	tree.SetSelectedFunc(st.onSelected)
	tree.SetInputCapture(st.handleInput)
	return st
}

// Refresh rebuilds the tree from SpanNode roots.
func (st *SpanTree) Refresh(roots []*SpanNode) {
	if len(roots) == 0 {
		placeholder := tview.NewTreeNode(Tag(HexOverlay0, "  Select a trace"))
		placeholder.SetSelectable(false)
		st.Tree.SetRoot(placeholder).SetCurrentNode(placeholder)
		return
	}

	// Invisible root to hold multiple top-level spans
	root := tview.NewTreeNode("")
	root.SetSelectable(false)

	for _, node := range roots {
		st.addNode(root, node, 0)
	}

	st.Tree.SetRoot(root).SetCurrentNode(root)
	// Expand root children (top-level agent spans)
	for _, child := range root.GetChildren() {
		child.SetExpanded(true)
	}
}

// addNode recursively builds tree nodes from a SpanNode.
func (st *SpanTree) addNode(parent *tview.TreeNode, sn *SpanNode, depth int) {
	label := formatSpanLabel(sn.Span)
	color := spanTypeColor(sn.Span.SpanType)

	node := tview.NewTreeNode(label)
	node.SetReference(sn.Span)
	node.SetColor(color)
	node.SetSelectable(true)
	node.SetExpanded(depth == 0) // top-level expanded, children collapsed

	parent.AddChild(node)

	for _, child := range sn.Children {
		st.addNode(node, child, depth+1)
	}
}

// onSelected handles Enter key on a tree node.
func (st *SpanTree) onSelected(node *tview.TreeNode) {
	ref := node.GetReference()
	if ref == nil {
		return
	}

	span, ok := ref.(SpanData)
	if !ok {
		return
	}

	// If node has children, toggle expand/collapse
	if len(node.GetChildren()) > 0 {
		node.SetExpanded(!node.IsExpanded())
		return
	}

	// Leaf node — show detail
	if st.OnDetail != nil {
		st.OnDetail(span)
	}
}

// handleInput processes vim keybindings and copy on the span tree.
func (st *SpanTree) handleInput(event *tcell.EventKey) *tcell.EventKey {
	switch event.Rune() {
	case 'j':
		return tcell.NewEventKey(tcell.KeyDown, 0, tcell.ModNone)
	case 'k':
		return tcell.NewEventKey(tcell.KeyUp, 0, tcell.ModNone)
	case 'o': // toggle expand/collapse current node
		node := st.Tree.GetCurrentNode()
		if node != nil && len(node.GetChildren()) > 0 {
			node.SetExpanded(!node.IsExpanded())
		}
		return nil
	case 'O': // expand all children recursively
		node := st.Tree.GetCurrentNode()
		if node != nil {
			expandAll(node, true)
		}
		return nil
	case 'y': // copy span info to clipboard
		node := st.Tree.GetCurrentNode()
		if node != nil {
			if ref := node.GetReference(); ref != nil {
				if span, ok := ref.(SpanData); ok && st.OnCopy != nil {
					st.OnCopy(formatSpanCopyText(span))
				}
			}
		}
		return nil
	case ' ': // space toggles expand/collapse
		node := st.Tree.GetCurrentNode()
		if node != nil && len(node.GetChildren()) > 0 {
			node.SetExpanded(!node.IsExpanded())
		}
		return nil
	}
	return event
}

// expandAll recursively expands or collapses a tree node.
func expandAll(node *tview.TreeNode, expand bool) {
	node.SetExpanded(expand)
	for _, child := range node.GetChildren() {
		expandAll(child, expand)
	}
}

// formatSpanLabel builds the one-line label for a span node.
func formatSpanLabel(s SpanData) string {
	icon := spanStatusIcon(s.Status)
	name := s.Name
	if name == "" {
		name = s.SpanType
	}

	switch s.SpanType {
	case "llm_call":
		if s.Model != "" {
			name = s.Model
		}
	case "tool_call":
		if s.ToolName != "" {
			name = s.ToolName
		}
	}

	dur := formatTraceDuration(s.DurationMs)
	label := fmt.Sprintf("%s %s (%s)", icon, name, dur)

	if s.SpanType == "llm_call" && (s.InputTokens > 0 || s.OutputTokens > 0) {
		in := compactNumber(s.InputTokens)
		out := compactNumber(s.OutputTokens)
		label += fmt.Sprintf(" [%s/%s]", in, out)
	}

	return label
}

// formatSpanCopyText returns a clipboard-friendly text for a span.
func formatSpanCopyText(s SpanData) string {
	name := s.Name
	if s.SpanType == "llm_call" && s.Model != "" {
		name = s.Model
	} else if s.SpanType == "tool_call" && s.ToolName != "" {
		name = s.ToolName
	}
	return fmt.Sprintf("%s: %s (%s) %s", s.SpanType, name, formatTraceDuration(s.DurationMs), s.ID)
}

// spanStatusIcon returns a Unicode icon for span status.
func spanStatusIcon(status string) string {
	switch status {
	case "ok", "success", "completed":
		return "\u25cf" // ●
	case "error", "failed":
		return "\u2717" // ✗
	case "running", "pending":
		return "\u25d0" // ◐
	default:
		return "\u25cb" // ○
	}
}

// spanTypeColor returns the Catppuccin color for a span type.
func spanTypeColor(spanType string) tcell.Color {
	switch spanType {
	case "agent":
		return ColorMauve
	case "llm_call":
		return ColorBlue
	case "tool_call":
		return ColorTeal
	default:
		return ColorOverlay0
	}
}
