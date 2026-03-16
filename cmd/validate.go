package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

var validateCmd = &cobra.Command{
	Use:   "validate",
	Short: "Validate manifest schema without connecting to GoClaw",
	RunE: func(cmd *cobra.Command, args []string) error {
		// TODO: implement validate
		fmt.Println("gcplane validate: not yet implemented")
		return nil
	},
}
