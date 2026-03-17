package views

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/dataplanelabs/gcplane/internal/manifest"
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
	"gopkg.in/yaml.v3"
)

// yamlKeyRegex matches a YAML key at the start of a line (with optional indentation).
var yamlKeyRegex = regexp.MustCompile(`^(\s*)([\w._-]+)(:)(.*)$`)

// ResourceDetail displays a full YAML view of an observed resource.
type ResourceDetail struct {
	TextView *tview.TextView
}

// NewResourceDetail creates a scrollable YAML detail view.
func NewResourceDetail() *ResourceDetail {
	tv := tview.NewTextView().
		SetDynamicColors(true).
		SetScrollable(true).
		SetWordWrap(true)
	tv.SetBorder(true)

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

	return &ResourceDetail{TextView: tv}
}

// ProviderObserver is the minimal interface needed to observe a resource.
type ProviderObserver interface {
	Observe(kind manifest.ResourceKind, key string) (map[string]any, error)
}

// Show loads and displays the YAML for a given resource.
// Fetches data in a goroutine; caller must provide tview.Application for QueueUpdateDraw.
func (rd *ResourceDetail) Show(kind manifest.ResourceKind, name string, provider ProviderObserver, tapp *tview.Application) {
	rd.TextView.SetTitle(fmt.Sprintf(" %s/%s ", kind, name))
	rd.TextView.SetText("[yellow]Loading...[-]")
	rd.TextView.ScrollToBeginning()

	go func() {
		observed, err := provider.Observe(kind, name)

		tapp.QueueUpdateDraw(func() {
			if err != nil {
				rd.TextView.SetText(fmt.Sprintf("[red]Error observing %s/%s:\n%s[-]", kind, name, err))
				return
			}
			if observed == nil {
				rd.TextView.SetText(fmt.Sprintf("[yellow]%s/%s not yet created in GoClaw.[-]\n\nRun [green]gcplane apply[-] to create it.", kind, name))
				return
			}

			yamlBytes, err := yaml.Marshal(observed)
			if err != nil {
				rd.TextView.SetText(fmt.Sprintf("[red]Failed to marshal YAML: %s[-]", err))
				return
			}

			rd.TextView.SetText(highlightYAML(string(yamlBytes)))
		})
	}()
}

// highlightYAML applies tview color tags to YAML text for readability.
func highlightYAML(yamlStr string) string {
	lines := strings.Split(yamlStr, "\n")
	var result []string

	for _, line := range lines {
		// Escape existing tview tags in values
		escaped := strings.ReplaceAll(line, "[", "[[]")

		if m := yamlKeyRegex.FindStringSubmatch(line); m != nil {
			indent, key, colon, rest := m[1], m[2], m[3], m[4]
			// Escape the rest portion for tview tags
			rest = strings.ReplaceAll(rest, "[", "[[]")
			highlighted := fmt.Sprintf("%s[blue]%s%s[-]%s", indent, key, colon, colorizeValue(rest))
			result = append(result, highlighted)
		} else if strings.HasPrefix(strings.TrimSpace(escaped), "#") {
			result = append(result, "[gray]"+escaped+"[-]")
		} else if strings.HasPrefix(strings.TrimSpace(escaped), "- ") {
			result = append(result, "[white]"+escaped+"[-]")
		} else {
			result = append(result, escaped)
		}
	}
	return strings.Join(result, "\n")
}

// colorizeValue adds color to a YAML value portion (after the colon).
func colorizeValue(val string) string {
	trimmed := strings.TrimSpace(val)
	if trimmed == "" {
		return val
	}

	switch {
	case trimmed == "true" || trimmed == "false":
		return " [yellow]" + trimmed + "[-]"
	case trimmed == "null" || trimmed == "~":
		return " [gray]" + trimmed + "[-]"
	case isNumeric(trimmed):
		return " [yellow]" + trimmed + "[-]"
	default:
		return " [green]" + trimmed + "[-]"
	}
}

// isNumeric checks if a string looks like a number.
func isNumeric(s string) bool {
	if len(s) == 0 {
		return false
	}
	for i, c := range s {
		if c == '-' && i == 0 {
			continue
		}
		if c == '.' {
			continue
		}
		if c < '0' || c > '9' {
			return false
		}
	}
	return true
}
