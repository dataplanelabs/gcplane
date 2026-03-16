package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

var diffCmd = &cobra.Command{
	Use:   "diff",
	Short: "Quick drift detection between manifest and actual state",
	RunE: func(cmd *cobra.Command, args []string) error {
		// TODO: implement diff
		fmt.Println("gcplane diff: not yet implemented")
		return nil
	},
}
