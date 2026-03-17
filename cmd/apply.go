package cmd

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/dataplanelabs/gcplane/internal/display"
	"github.com/dataplanelabs/gcplane/internal/provider/goclaw"
	"github.com/dataplanelabs/gcplane/internal/reconciler"
	"github.com/spf13/cobra"
)

var autoApprove bool
var applyPrune bool

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

		ep, tok, err := resolveConnection(m)
		if err != nil {
			return err
		}

		provider := goclaw.New(ep, tok)
		defer provider.Close()

		engine := reconciler.NewEngine(provider)
		opts := reconciler.ReconcileOpts{Prune: applyPrune}

		// Show plan first (dry-run)
		plan, _ := engine.Reconcile(m, reconciler.ReconcileOpts{DryRun: true, Prune: applyPrune})
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

		if result.Failed > 0 {
			return fmt.Errorf("%d resource(s) failed to apply", result.Failed)
		}
		return nil
	},
}

func init() {
	applyCmd.Flags().BoolVar(&autoApprove, "auto-approve", false, "skip confirmation prompt")
	applyCmd.Flags().BoolVar(&applyPrune, "prune", false, "delete gcplane-owned resources not present in manifest")
}
