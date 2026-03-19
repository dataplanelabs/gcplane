package cmd

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/dataplanelabs/gcplane/internal/display"
	"github.com/dataplanelabs/gcplane/internal/manifest"
	"github.com/dataplanelabs/gcplane/internal/provider/goclaw"
	"github.com/dataplanelabs/gcplane/internal/reconciler"
	"github.com/spf13/cobra"
)

var autoApprove bool
var applyPrune bool
var applyForce bool
var applyLabelSelector string
var applyLogFile string

var applyCmd = &cobra.Command{
	Use:   "apply",
	Short: "Apply manifest to reach desired state",
	Long: `Read the manifest, compute a plan, then execute create/update
operations against GoClaw to reconcile the actual state.

Only manages declared resources — UI-created objects are untouched.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		m, err := loadAndValidateManifest()
		if err != nil {
			return err
		}

		if applyLabelSelector != "" {
			selector := manifest.ParseLabelSelector(applyLabelSelector)
			m.Resources = manifest.FilterByLabels(m.Resources, selector)
		}

		ep, tok, err := resolveConnection(m)
		if err != nil {
			return err
		}

		provider := goclaw.New(ep, tok)
		defer provider.Close()

		engine := reconciler.NewEngine(provider)
		opts := reconciler.ReconcileOpts{Prune: applyPrune, Force: applyForce}

		// Show plan first (dry-run)
		plan, _ := engine.Reconcile(m, reconciler.ReconcileOpts{DryRun: true, Prune: applyPrune, Force: applyForce})
		display.PrintPlan(plan, verbose)

		if plan.Creates == 0 && plan.Updates == 0 && plan.Deletes == 0 {
			fmt.Println("\nNo changes to apply.")
			return nil
		}

		// Warn before confirmation when deletions are planned
		if plan.Deletes > 0 {
			display.PrintPruneWarning(plan.Deletes)
		}

		// Confirm unless auto-approve
		if !autoApprove {
			fmt.Print("\nApply these changes? [y/N] ")
			reader := bufio.NewReader(os.Stdin)
			answer, _ := reader.ReadString('\n')
			answer = strings.TrimSpace(strings.ToLower(answer))
			if answer != "y" && answer != "yes" {
				fmt.Println("Apply cancelled.")
				return nil
			}
		}

		// Apply
		opts.DryRun = false
		_, result := engine.Reconcile(m, opts)
		display.PrintApplyResult(result)

		writeApplyAuditLog(applyLogFile, configFile, plan, result)

		if result.Failed > 0 {
			return fmt.Errorf("%d resource(s) failed to apply", result.Failed)
		}
		return nil
	},
}

func init() {
	applyCmd.Flags().BoolVar(&autoApprove, "auto-approve", false, "skip confirmation prompt")
	applyCmd.Flags().BoolVar(&applyPrune, "prune", false, "delete gcplane-owned resources not present in manifest")
	applyCmd.Flags().BoolVar(&applyForce, "force", false, "re-apply all resources even when no diff detected")
	applyCmd.Flags().StringVarP(&applyLabelSelector, "label", "l", "", "filter resources by label (key=value,key2=value2)")
	applyCmd.Flags().StringVar(&applyLogFile, "log-file", "", "write audit log to file (JSON format)")
}

// writeApplyAuditLog appends a JSON audit entry to logFile (no-op if empty).
func writeApplyAuditLog(logFile, manifestFile string, plan *reconciler.Plan, result *reconciler.ApplyResult) {
	if logFile == "" {
		return
	}
	entry := map[string]any{
		"timestamp": time.Now().UTC().Format(time.RFC3339),
		"action":    "apply",
		"manifest":  manifestFile,
		"creates":   plan.Creates,
		"updates":   plan.Updates,
		"deletes":   plan.Deletes,
		"applied":   result.Applied,
		"failed":    result.Failed,
		"errors":    result.Errors,
	}
	data, _ := json.Marshal(entry)
	f, err := os.OpenFile(logFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return
	}
	defer f.Close()
	f.Write(append(data, '\n'))
}
