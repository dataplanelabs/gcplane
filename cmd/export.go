package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

var exportCmd = &cobra.Command{
	Use:   "export",
	Short: "Export current GoClaw state as a YAML manifest",
	Long: `Connect to GoClaw, read all managed resource types,
and output a YAML manifest representing the current state.

Useful for bootstrapping a manifest from an existing deployment.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		// TODO: implement export
		fmt.Println("gcplane export: not yet implemented")
		return nil
	},
}
