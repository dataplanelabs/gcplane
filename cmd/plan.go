package cmd

import (
	"github.com/dataplanelabs/gcplane/internal/display"
	"github.com/dataplanelabs/gcplane/internal/manifest"
	"github.com/dataplanelabs/gcplane/internal/provider/goclaw"
	"github.com/dataplanelabs/gcplane/internal/reconciler"
	"github.com/spf13/cobra"
)

var planPrune bool
var planForce bool
var planLabelSelector string

var planCmd = &cobra.Command{
	Use:   "plan",
	Short: "Show changes required to reach desired state (dry-run)",
	Long: `Compare the declared manifest against the actual GoClaw state
and display a diff of what would change, without applying anything.

Similar to 'terraform plan'.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		m, err := loadAndValidateManifest()
		if err != nil {
			return err
		}

		if planLabelSelector != "" {
			selector := manifest.ParseLabelSelector(planLabelSelector)
			m.Resources = manifest.FilterByLabels(m.Resources, selector)
		}

		ep, tok, err := resolveConnection(m)
		if err != nil {
			return err
		}

		provider := goclaw.New(ep, tok)
		defer provider.Close()

		engine := reconciler.NewEngine(provider)
		plan, _ := engine.Reconcile(m, reconciler.ReconcileOpts{DryRun: true, Prune: planPrune, Force: planForce})

		display.PrintPlan(plan, verbose)
		return nil
	},
}

func init() {
	planCmd.Flags().BoolVar(&planPrune, "prune", false, "include deletion of gcplane-owned resources not in manifest")
	planCmd.Flags().BoolVar(&planForce, "force", false, "show all resources as updates even when no diff detected")
	planCmd.Flags().StringVarP(&planLabelSelector, "label", "l", "", "filter resources by label (key=value,key2=value2)")
}
