package cmd

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/dataplanelabs/gcplane/internal/update"
	"github.com/spf13/cobra"
)

var (
	// Version is set at build time.
	Version = "dev"

	// Global flags
	configFile string
	endpoint   string
	token      string
	verbose    bool
)

// updateResult receives the background update check result.
var updateResult chan *update.ReleaseInfo

var rootCmd = &cobra.Command{
	Use:   "gcplane",
	Short: "Declarative config management for GoClaw",
	Long: `GCPlane is a GitOps-style control plane for managing GoClaw deployments.

It reads YAML manifests describing your desired GoClaw configuration
(agents, providers, channels, cron jobs, MCP servers, etc.) and
reconciles them against the actual state via GoClaw's API.

Modes:
  CLI:     gcplane plan/apply/diff for manual operations
  Service: gcplane serve for continuous reconciliation`,
	PersistentPreRun: func(cmd *cobra.Command, args []string) {
		// Skip update check for serve (long-running) and version commands
		if cmd.Name() == "serve" || cmd.Name() == "version" {
			return
		}
		if !update.ShouldCheck() {
			return
		}
		updateResult = make(chan *update.ReleaseInfo, 1)
		go func() {
			updateResult <- update.Check(context.Background(), Version)
		}()
	},
	PersistentPostRun: func(cmd *cobra.Command, args []string) {
		if updateResult == nil {
			return
		}
		// Wait up to 2 seconds for the background check
		select {
		case rel := <-updateResult:
			if rel != nil {
				fmt.Fprintf(os.Stderr, "\nA new version of gcplane is available: %s → %s\n", Version, rel.Version)
				fmt.Fprintf(os.Stderr, "Upgrade: curl -fsSL https://raw.githubusercontent.com/dataplanelabs/gcplane/main/install.sh | sh\n")
			}
		case <-time.After(2 * time.Second):
			// don't block forever
		}
	},
}

// Execute runs the root command.
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func init() {
	rootCmd.PersistentFlags().StringVarP(&configFile, "file", "f", "", "manifest file or directory")
	rootCmd.PersistentFlags().StringVar(&endpoint, "endpoint", "", "GoClaw endpoint URL (or GCPLANE_ENDPOINT env)")
	rootCmd.PersistentFlags().StringVar(&token, "token", "", "GoClaw auth token (or GCPLANE_TOKEN env)")
	rootCmd.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false, "verbose output")

	rootCmd.AddCommand(initCmd)
	rootCmd.AddCommand(planCmd)
	rootCmd.AddCommand(applyCmd)
	rootCmd.AddCommand(diffCmd)
	rootCmd.AddCommand(destroyCmd)
	rootCmd.AddCommand(validateCmd)
	rootCmd.AddCommand(exportCmd)
	rootCmd.AddCommand(serveCmd)
	rootCmd.AddCommand(statusCmd)
	rootCmd.AddCommand(versionCmd)
}

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print version",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Printf("gcplane %s\n", Version)
	},
}
