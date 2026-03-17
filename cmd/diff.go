package cmd

import (
	"github.com/dataplanelabs/gcplane/internal/display"
	"github.com/dataplanelabs/gcplane/internal/provider/goclaw"
	"github.com/dataplanelabs/gcplane/internal/reconciler"
	"github.com/spf13/cobra"
)

var diffCmd = &cobra.Command{
	Use:   "diff",
	Short: "Show drift between manifest and live state",
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
		plan, _ := engine.Reconcile(m, reconciler.ReconcileOpts{DryRun: true})

		display.PrintDiff(plan)
		return nil
	},
}
