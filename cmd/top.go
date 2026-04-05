package cmd

import (
	"log/slog"

	"github.com/dataplanelabs/gcplane/internal/provider/goclaw"
	"github.com/dataplanelabs/gcplane/internal/reconciler"
	"github.com/dataplanelabs/gcplane/internal/tui"
	"github.com/dataplanelabs/gcplane/internal/tui/trace"
	"github.com/spf13/cobra"
)

var (
	topInterval string
	topAttach   string
)

var topCmd = &cobra.Command{
	Use:   "top",
	Short: "Interactive TUI for monitoring GoClaw resources",
	Long: `Launch a k9s-style terminal UI for real-time monitoring of GoClaw resources.

Shows resource status, drift detection, and YAML details in an interactive
terminal dashboard with vim-style keybindings.

Modes:
  Direct:  gcplane top -f gcplane.yaml          (reads manifest, talks to GoClaw API)
  Attach:  gcplane top --attach http://host:8480 (polls a running gcplane serve instance)

Examples:
  gcplane top -f gcplane.yaml
  gcplane top --interval 5s
  gcplane top --attach http://localhost:8480`,
	RunE: runTop,
}

func init() {
	topCmd.Flags().StringVar(&topInterval, "interval", "10s", "refresh interval")
	topCmd.Flags().StringVar(&topAttach, "attach", "", "URL of running gcplane serve instance (attach mode)")
}

func runTop(_ *cobra.Command, _ []string) error {
	// Attach mode — no manifest needed
	if topAttach != "" {
		app, err := tui.NewApp(tui.Config{
			Attach:   topAttach,
			Interval: topInterval,
		})
		if err != nil {
			return err
		}
		return app.Run()
	}

	// Direct mode — requires manifest
	m, err := loadAndValidateManifest()
	if err != nil {
		return err
	}

	ep, tok, err := resolveConnection(m)
	if err != nil {
		return err
	}
	provOpts, err := resolveProviderOpts(m)
	if err != nil {
		return err
	}

	// Trace handler captures engine + provider logs for TUI display
	traceHandler := trace.NewRingHandler(1000, slog.LevelDebug, nil)
	logger := slog.New(traceHandler)

	provOpts = append(provOpts, goclaw.WithLogger(logger))
	provider := goclaw.New(ep, tok, provOpts...)
	defer provider.Close()

	engine := reconciler.NewEngine(provider, logger)

	app, err := tui.NewApp(tui.Config{
		Manifest:     m,
		Endpoint:     ep,
		Provider:     provider,
		Engine:       engine,
		Interval:     topInterval,
		TraceHandler: traceHandler,
	})
	if err != nil {
		return err
	}

	return app.Run()
}
