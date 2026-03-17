package cmd

import (
	"github.com/dataplanelabs/gcplane/internal/provider/goclaw"
	"github.com/dataplanelabs/gcplane/internal/reconciler"
	"github.com/dataplanelabs/gcplane/internal/tui"
	"github.com/spf13/cobra"
)

var topInterval string

var topCmd = &cobra.Command{
	Use:   "top",
	Short: "Interactive TUI for monitoring GoClaw resources",
	Long: `Launch a k9s-style terminal UI for real-time monitoring of GoClaw resources.

Shows resource status, drift detection, and YAML details in an interactive
terminal dashboard with vim-style keybindings.

Examples:
  gcplane top -f gcplane.yaml
  gcplane top --interval 5s
  gcplane top -f manifest.yaml --endpoint http://localhost:8080`,
	RunE: runTop,
}

func init() {
	topCmd.Flags().StringVar(&topInterval, "interval", "10s", "refresh interval")
}

func runTop(_ *cobra.Command, _ []string) error {
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

	app, err := tui.NewApp(tui.Config{
		Manifest: m,
		Endpoint: ep,
		Provider: provider,
		Engine:   engine,
		Interval: topInterval,
	})
	if err != nil {
		return err
	}

	return app.Run()
}
