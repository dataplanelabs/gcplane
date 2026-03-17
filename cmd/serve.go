package cmd

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/dataplanelabs/gcplane/internal/controller"
	"github.com/dataplanelabs/gcplane/internal/notifier"
	"github.com/dataplanelabs/gcplane/internal/provider/goclaw"
	"github.com/dataplanelabs/gcplane/internal/server"
	"github.com/dataplanelabs/gcplane/internal/source"
	"github.com/spf13/cobra"
)

var (
	serveAddr          string
	serveInterval      string
	serveRepo          string
	serveBranch        string
	servePath          string
	servePrune         bool
	serveWebhookSecret string
	serveTenantsDir    string
)

var serveCmd = &cobra.Command{
	Use:   "serve",
	Short: "Run as a continuous reconciliation service",
	Long: `Start GCPlane as a long-running GitOps controller.

Watches a manifest file or git repo for changes and periodically
reconciles against GoClaw. Exposes HTTP endpoints for health,
metrics, status, and sync triggers.

Examples:
  # Watch local file
  gcplane serve -f manifest.yaml --interval 30s

  # Watch git repo
  gcplane serve --repo git@github.com:org/config.git --path manifests/prod.yaml

Endpoints:
  GET  /healthz           — liveness probe
  GET  /readyz            — readiness probe (200 after first sync)
  GET  /metrics           — Prometheus metrics
  GET  /api/v1/status     — sync status + per-resource state
  POST /api/v1/sync       — trigger immediate reconcile
  POST /api/v1/webhook/git — git push webhook trigger`,
	RunE: runServe,
}

func init() {
	serveCmd.Flags().StringVar(&serveAddr, "addr", ":8480", "listen address")
	serveCmd.Flags().StringVar(&serveInterval, "interval", "30s", "reconciliation interval")
	serveCmd.Flags().StringVar(&serveRepo, "repo", "", "git repository URL")
	serveCmd.Flags().StringVar(&serveBranch, "branch", "main", "git branch")
	serveCmd.Flags().StringVar(&servePath, "path", "manifest.yaml", "manifest path in repo")
	serveCmd.Flags().BoolVar(&servePrune, "prune", false, "delete resources not in manifest")
	serveCmd.Flags().StringVar(&serveWebhookSecret, "webhook-secret", "", "webhook signature verification secret")
	serveCmd.Flags().StringVar(&serveTenantsDir, "tenants-dir", "", "directory of tenant subdirs (mutually exclusive with -f/--repo)")
}

func newLogger() *slog.Logger {
	opts := &slog.HandlerOptions{Level: slog.LevelInfo}
	if os.Getenv("GCPLANE_LOG_FORMAT") == "json" {
		return slog.New(slog.NewJSONHandler(os.Stdout, opts))
	}
	return slog.New(slog.NewTextHandler(os.Stdout, opts))
}

func runServe(_ *cobra.Command, _ []string) error {
	logger := newLogger()

	// Parse interval
	interval, err := time.ParseDuration(serveInterval)
	if err != nil {
		return fmt.Errorf("invalid interval %q: %w", serveInterval, err)
	}

	// Multi-tenant mode
	if serveTenantsDir != "" {
		if configFile != "" || serveRepo != "" {
			return fmt.Errorf("--tenants-dir is mutually exclusive with -f/--file and --repo")
		}
		return runMultiTenantServe(logger, interval)
	}

	// Determine manifest source
	var src source.ManifestSource
	var gitSrc *source.GitSource

	switch {
	case serveRepo != "":
		var err error
		gitSrc, err = source.NewGitSource(serveRepo, serveBranch, servePath, logger)
		if err != nil {
			return err
		}
		src = gitSrc
		logger.Info("using git source", "repo", serveRepo, "branch", serveBranch, "path", servePath)
	case configFile != "":
		src = source.NewFileSource(configFile)
		logger.Info("using file source", "path", configFile)
	default:
		return fmt.Errorf("either --file (-f), --repo, or --tenants-dir is required")
	}

	// Initial fetch to validate config and resolve connection
	m, _, err := src.Fetch()
	if err != nil {
		return fmt.Errorf("initial manifest fetch: %w", err)
	}

	ep, tok, err := resolveConnection(m)
	if err != nil {
		return err
	}

	// Create long-lived provider
	provider := goclaw.New(ep, tok)
	defer provider.Close()

	// Create shared components
	tracker := controller.NewStatusTracker()
	n := notifier.New(os.Getenv("GCPLANE_WEBHOOK_URL"), os.Getenv("GCPLANE_WEBHOOK_FORMAT"))

	ctrl := controller.New(controller.Config{
		Source:    src,
		Provider:  provider,
		Tracker:   tracker,
		Notifier:  n,
		Interval:  interval,
		Prune:     servePrune,
		Logger:    logger,
	})

	srv := server.New(server.Config{
		Addr:          serveAddr,
		Tracker:       tracker,
		Controller:    ctrl,
		Logger:        logger,
		WebhookSecret: serveWebhookSecret,
	})

	// Signal handling
	done := make(chan struct{})
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	// Start controller loop
	go ctrl.Run(done)

	// Start HTTP server
	srvErrCh := make(chan error, 1)
	go func() {
		if err := srv.ListenAndServe(); err != nil && err.Error() != "http: Server closed" {
			srvErrCh <- err
		}
	}()

	logger.Info("gcplane serve started", "addr", serveAddr, "interval", interval)

	// Wait for shutdown signal or server error
	select {
	case sig := <-sigCh:
		logger.Info("received signal, shutting down", "signal", sig)
	case err := <-srvErrCh:
		logger.Error("http server failed", "error", err)
		close(done)
		return fmt.Errorf("http server: %w", err)
	}
	close(done)

	// Graceful HTTP shutdown
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	srv.Shutdown(shutdownCtx)

	// Cleanup git source
	if gitSrc != nil {
		gitSrc.Cleanup()
	}

	logger.Info("gcplane serve stopped")
	return nil
}

// runMultiTenantServe handles the --tenants-dir mode.
func runMultiTenantServe(logger *slog.Logger, interval time.Duration) error {
	logger.Info("multi-tenant mode", "tenants-dir", serveTenantsDir)

	tm, err := controller.NewTenantManager(controller.TenantManagerConfig{
		TenantsDir: serveTenantsDir,
		Interval:   interval,
		Prune:      servePrune,
		Logger:     logger,
	})
	if err != nil {
		return fmt.Errorf("init tenant manager: %w", err)
	}
	defer tm.CloseAll()

	srv := server.New(server.Config{
		Addr:          serveAddr,
		TenantManager: tm,
		Logger:        logger,
		WebhookSecret: serveWebhookSecret,
	})

	done := make(chan struct{})
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	go tm.RunAll(done)

	srvErrCh := make(chan error, 1)
	go func() {
		if err := srv.ListenAndServe(); err != nil && err.Error() != "http: Server closed" {
			srvErrCh <- err
		}
	}()

	logger.Info("gcplane serve started (multi-tenant)", "addr", serveAddr, "interval", interval)

	select {
	case sig := <-sigCh:
		logger.Info("received signal, shutting down", "signal", sig)
	case err := <-srvErrCh:
		logger.Error("http server failed", "error", err)
		close(done)
		return fmt.Errorf("http server: %w", err)
	}
	close(done)

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	srv.Shutdown(shutdownCtx)

	logger.Info("gcplane serve stopped")
	return nil
}
