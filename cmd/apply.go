package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

var applyCmd = &cobra.Command{
	Use:   "apply",
	Short: "Apply manifest to reach desired state",
	Long: `Read the manifest, compute a plan, then execute create/update
operations against GoClaw to reconcile the actual state.

Only manages declared resources — UI-created objects are untouched.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		// TODO: implement apply
		fmt.Println("gcplane apply: not yet implemented")
		return nil
	},
}
