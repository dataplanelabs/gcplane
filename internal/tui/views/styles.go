package views

import (
	"fmt"
	"strings"

	"github.com/dataplanelabs/gcplane/internal/manifest"
	"github.com/dataplanelabs/gcplane/internal/reconciler"
	"github.com/gdamore/tcell/v2"
)

// Catppuccin Mocha — base/surface colors.
var (
	ColorBase     = tcell.NewRGBColor(30, 30, 46)     // #1e1e2e
	ColorMantle   = tcell.NewRGBColor(24, 24, 37)     // #181825
	ColorSurface0 = tcell.NewRGBColor(49, 50, 68)     // #313244
	ColorSurface1 = tcell.NewRGBColor(69, 71, 90)     // #45475a
	ColorOverlay0 = tcell.NewRGBColor(108, 112, 134)  // #6c7086
	ColorText     = tcell.NewRGBColor(205, 214, 244)  // #cdd6f4
	ColorSubtext0 = tcell.NewRGBColor(166, 173, 200)  // #a6adc8
)

// Catppuccin Mocha — accent colors.
var (
	ColorRed       = tcell.NewRGBColor(243, 139, 168) // #f38ba8
	ColorGreen     = tcell.NewRGBColor(166, 227, 161) // #a6e3a1
	ColorYellow    = tcell.NewRGBColor(249, 226, 175) // #f9e2af
	ColorBlue      = tcell.NewRGBColor(137, 180, 250) // #89b4fa
	ColorMauve     = tcell.NewRGBColor(203, 166, 247) // #cba6f7
	ColorPeach     = tcell.NewRGBColor(250, 179, 135) // #fab387
	ColorTeal      = tcell.NewRGBColor(148, 226, 213) // #94e2d5
	ColorLavender  = tcell.NewRGBColor(180, 190, 254) // #b4befe
	ColorFlamingo  = tcell.NewRGBColor(242, 205, 205) // #f2cdcd
	ColorSky       = tcell.NewRGBColor(137, 220, 235) // #89dceb
	ColorSapphire  = tcell.NewRGBColor(116, 199, 236) // #74c7ec
	ColorPink      = tcell.NewRGBColor(245, 194, 231) // #f5c2e7
	ColorRosewater = tcell.NewRGBColor(245, 224, 220) // #f5e0dc
)

// Hex string constants for tview [#hex] color tags.
const (
	HexOverlay0 = "#6c7086"
	HexText     = "#cdd6f4"
	HexSubtext0 = "#a6adc8"
	HexRed      = "#f38ba8"
	HexGreen    = "#a6e3a1"
	HexYellow   = "#f9e2af"
	HexBlue     = "#89b4fa"
	HexMauve    = "#cba6f7"
	HexPeach    = "#fab387"
	HexTeal     = "#94e2d5"
	HexLavender = "#b4befe"
	HexSky      = "#89dceb"
	HexPink     = "#f5c2e7"
)

// StatusIndicator maps status strings to Unicode symbols.
var StatusIndicator = map[string]string{
	"InSync":  "\u25cf", // ●
	"Drifted": "\u25c6", // ◆
	"Missing": "\u2717", // ✗
	"Error":   "\u2717", // ✗
	"Extra":   "?",
}

// StatusColor maps status strings to tcell colors for table cells.
var StatusColor = map[string]tcell.Color{
	"InSync":  ColorGreen,
	"Drifted": ColorYellow,
	"Missing": ColorPeach,
	"Error":   ColorRed,
	"Extra":   ColorLavender,
}

// StatusHex maps status strings to hex color codes for tview tags.
var StatusHex = map[string]string{
	"InSync":  HexGreen,
	"Drifted": HexYellow,
	"Missing": HexPeach,
	"Error":   HexRed,
	"Extra":   HexLavender,
}

// KindColor maps resource kinds to distinct palette colors.
var KindColor = map[manifest.ResourceKind]tcell.Color{
	manifest.KindTenant:            ColorPink,
	manifest.KindProvider:          ColorBlue,
	manifest.KindAgent:             ColorMauve,
	manifest.KindChannel:           ColorTeal,
	manifest.KindMCPServer:         ColorSky,
	manifest.KindSkill:             ColorGreen,
	manifest.KindCronJob:           ColorYellow,
	manifest.KindAgentTeam:         ColorPeach,
	manifest.KindBuiltinToolConfig: ColorFlamingo,
	manifest.KindSkillConfig:       ColorLavender,
	manifest.KindMCPCredentials:    ColorSapphire,
	manifest.KindSystemConfig:      ColorRosewater,
	manifest.KindSecureCLI:         ColorPink,
	manifest.KindSecureCLIGrant:    ColorFlamingo,
	manifest.KindAgentLink:         ColorSky,
}

// StatusCell returns a formatted status string with Unicode indicator and its color.
func StatusCell(status string) (string, tcell.Color) {
	sym := StatusIndicator[status]
	if sym == "" {
		sym = "?"
	}
	color := StatusColor[status]
	if color == 0 {
		color = ColorText
	}
	return sym + " " + status, color
}

// KindCellColor returns the palette color for a resource kind.
func KindCellColor(kind manifest.ResourceKind) tcell.Color {
	if c, ok := KindColor[kind]; ok {
		return c
	}
	return ColorText
}

// StatusSummaryColored returns a colored version of the status summary for the header bar.
func StatusSummaryColored(changes []reconciler.Change) string {
	counts := map[string]int{}
	for _, c := range changes {
		counts[actionToStatus(c)]++
	}
	var parts []string
	for _, s := range []string{"InSync", "Drifted", "Missing", "Error", "Extra"} {
		if n := counts[s]; n > 0 {
			sym := StatusIndicator[s]
			hex := StatusHex[s]
			parts = append(parts, Tag(hex, fmt.Sprintf("%s %d %s", sym, n, s)))
		}
	}
	if len(parts) == 0 {
		return Tag(HexOverlay0, "no resources")
	}
	return strings.Join(parts, "  ")
}

// Tag wraps text in a tview hex color tag: [#hex]text[-].
func Tag(hex, text string) string {
	return "[" + hex + "]" + text + "[-]"
}

// BoldTag wraps text in bold + hex color: [#hex::b]text[-::-].
func BoldTag(hex, text string) string {
	return "[" + hex + "::b]" + text + "[-::-]"
}

// HeaderSep returns a styled separator for the header bar.
func HeaderSep() string {
	return Tag(HexOverlay0, " | ")
}
