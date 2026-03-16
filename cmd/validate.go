package cmd

import (
	"fmt"
	"os"

	"github.com/dataplanelabs/gcplane/internal/manifest"
	"github.com/spf13/cobra"
)

var validateCmd = &cobra.Command{
	Use:   "validate",
	Short: "Validate manifest schema without connecting to GoClaw",
	RunE: func(cmd *cobra.Command, args []string) error {
		if configFile == "" {
			return fmt.Errorf("manifest file required: use --file or -f")
		}

		m, err := manifest.Load(configFile)
		if err != nil {
			return err
		}

		errs := manifest.Validate(m)
		if len(errs) > 0 {
			fmt.Fprintln(os.Stderr, "Validation errors:")
			for _, e := range errs {
				fmt.Fprintf(os.Stderr, "  - %s\n", e)
			}
			return fmt.Errorf("%d validation error(s)", len(errs))
		}

		fmt.Printf("Valid: %s (%d resources)\n", m.Metadata.Name, len(m.Resources))
		return nil
	},
}
