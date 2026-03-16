package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

var (
	serveAddr     string
	serveInterval string
)

var serveCmd = &cobra.Command{
	Use:   "serve",
	Short: "Run as a continuous reconciliation service",
	Long: `Start GCPlane as an HTTP service with continuous reconciliation.

Watches the manifest directory for changes and periodically reconciles
against GoClaw. Exposes an HTTP API for status, plan, and apply operations.

Endpoints:
  POST /api/v1/plan    — dry-run reconcile
  POST /api/v1/apply   — apply manifest
  GET  /api/v1/status  — sync status per resource
  GET  /api/v1/drift   — drift report
  POST /api/v1/webhook/git — trigger on git push`,
	RunE: func(cmd *cobra.Command, args []string) error {
		// TODO: implement serve
		fmt.Println("gcplane serve: not yet implemented")
		return nil
	},
}

func init() {
	serveCmd.Flags().StringVar(&serveAddr, "addr", ":8480", "listen address")
	serveCmd.Flags().StringVar(&serveInterval, "interval", "5m", "reconciliation interval")
}
