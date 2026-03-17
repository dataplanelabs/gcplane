package cmd

import (
	"fmt"

	"github.com/dataplanelabs/gcplane/internal/manifest"
	"github.com/dataplanelabs/gcplane/internal/provider/goclaw"
	"github.com/dataplanelabs/gcplane/internal/reconciler"
	"github.com/spf13/cobra"
)

var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show resource status summary",
	Long:  "Quick overview of managed resources and their sync state.",
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

		total := plan.Creates + plan.Updates + plan.Noops + plan.Deletes
		inSync := plan.Noops
		drifted := plan.Updates
		missing := plan.Creates

		fmt.Printf("\n\033[1mGCPlane Status\033[0m — %s\n\n", m.Metadata.Name)
		fmt.Printf("  Resources:  %d total\n", total)
		fmt.Printf("  In Sync:    \033[32m%d\033[0m\n", inSync)
		if drifted > 0 {
			fmt.Printf("  Drifted:    \033[33m%d\033[0m\n", drifted)
		}
		if missing > 0 {
			fmt.Printf("  Missing:    \033[31m%d\033[0m\n", missing)
		}

		// Per-kind breakdown (only kinds with pending changes)
		fmt.Println()
		kindCounts := make(map[manifest.ResourceKind]int)
		for _, c := range plan.Changes {
			kindCounts[c.Kind]++
		}
		for _, kind := range manifest.ApplyOrder() {
			if count, ok := kindCounts[kind]; ok {
				fmt.Printf("  %-12s %d\n", kind, count)
			}
		}

		if drifted > 0 || missing > 0 {
			fmt.Printf("\n  Run \033[1mgcplane plan\033[0m for details.\n")
		} else {
			fmt.Printf("\n  \033[32m✓ All resources in sync\033[0m\n")
		}
		fmt.Println()

		return nil
	},
}
