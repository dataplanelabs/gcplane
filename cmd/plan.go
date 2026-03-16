package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

var planCmd = &cobra.Command{
	Use:   "plan",
	Short: "Show changes required to reach desired state (dry-run)",
	Long: `Compare the declared manifest against the actual GoClaw state
and display a diff of what would change, without applying anything.

Similar to 'terraform plan'.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		// TODO: implement plan
		fmt.Println("gcplane plan: not yet implemented")
		return nil
	},
}
