// Package display renders reconciler plans and results as colored terminal output.
package display

import (
	"fmt"
	"sort"
	"strings"

	"github.com/dataplanelabs/gcplane/internal/reconciler"
)

// ANSI color codes
const (
	colorReset  = "\033[0m"
	colorGreen  = "\033[32m"
	colorYellow = "\033[33m"
	colorRed    = "\033[31m"
	colorDim    = "\033[2m"
	colorBold   = "\033[1m"
)

// PrintPlan renders a plan as terraform-style colored output.
func PrintPlan(plan *reconciler.Plan, verbose bool) {
	fmt.Printf("\n%sGCPlane Plan:%s %d to create, %d to update, %d unchanged\n\n",
		colorBold, colorReset, plan.Creates, plan.Updates, plan.Noops)

	for _, c := range plan.Changes {
		switch c.Action {
		case reconciler.ActionCreate:
			printCreate(c)
		case reconciler.ActionUpdate:
			printUpdate(c)
		case reconciler.ActionNoop:
			if verbose {
				printNoop(c)
			}
		}
	}

	if len(plan.Errors) > 0 {
		fmt.Printf("\n%sErrors:%s\n", colorRed, colorReset)
		for _, e := range plan.Errors {
			fmt.Printf("  %s%s%s\n", colorRed, e, colorReset)
		}
	}

	fmt.Printf("\n%sPlan:%s %d to create, %d to update, %d unchanged.\n",
		colorBold, colorReset, plan.Creates, plan.Updates, plan.Noops)
}

func printCreate(c reconciler.Change) {
	fmt.Printf("%s+ %s/%s%s\n", colorGreen, c.Kind, c.Key, colorReset)
}

func printUpdate(c reconciler.Change) {
	fmt.Printf("%s~ %s/%s%s\n", colorYellow, c.Kind, c.Key, colorReset)

	keys := sortedKeys(c.Diff)
	for _, k := range keys {
		d := c.Diff[k]
		fmt.Printf("    %s%s:%s %s%v%s → %s%v%s\n",
			colorDim, k, colorReset,
			colorRed, formatVal(d.Old), colorReset,
			colorGreen, formatVal(d.New), colorReset)
	}
}

func printNoop(c reconciler.Change) {
	fmt.Printf("%s= %s/%s (no changes)%s\n", colorDim, c.Kind, c.Key, colorReset)
}

// PrintApplyResult renders the result of applying a plan.
func PrintApplyResult(result *reconciler.ApplyResult) {
	fmt.Printf("\n%sApply complete!%s %d applied, %d failed.\n",
		colorBold, colorReset, result.Applied, result.Failed)

	if len(result.Errors) > 0 {
		fmt.Printf("\n%sErrors:%s\n", colorRed, colorReset)
		for _, e := range result.Errors {
			fmt.Printf("  %s%s%s\n", colorRed, e, colorReset)
		}
	}
}

func sortedKeys(m map[string]reconciler.FieldDiff) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

func formatVal(v any) string {
	if v == nil {
		return "(none)"
	}
	s := fmt.Sprintf("%v", v)
	if len(s) > 80 {
		return s[:77] + "..."
	}
	return strings.TrimSpace(s)
}
