package cmd

import (
	"github.com/dataplanelabs/gcplane/internal/display"
	"github.com/dataplanelabs/gcplane/internal/provider/goclaw"
	"github.com/dataplanelabs/gcplane/internal/reconciler"
	"github.com/spf13/cobra"
)

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

		ep, tok, err := resolveConnection(m)
		if err != nil {
			return err
		}

		provider := goclaw.New(ep, tok)
		defer provider.Close()

		engine := reconciler.NewEngine(provider)
		plan, _ := engine.Reconcile(m, true)

		display.PrintPlan(plan, verbose)
		return nil
	},
}
