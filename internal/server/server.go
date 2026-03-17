// Package server provides the HTTP server for health, metrics, and status endpoints.
package server

import (
	"context"
	"log/slog"
	"net/http"

	"github.com/dataplanelabs/gcplane/internal/controller"
)

// Server exposes health, metrics, status, and sync trigger endpoints.
type Server struct {
	tracker       *controller.StatusTracker
	controller    *controller.Controller
	tenantManager *controller.TenantManager
	httpServer    *http.Server
	logger        *slog.Logger
	webhookSecret string
}

// Config holds server dependencies.
type Config struct {
	Addr          string
	Tracker       *controller.StatusTracker
	Controller    *controller.Controller
	TenantManager *controller.TenantManager
	Logger        *slog.Logger
	WebhookSecret string
}

// New creates an HTTP server with all routes registered.
func New(cfg Config) *Server {
	s := &Server{
		tracker:       cfg.Tracker,
		controller:    cfg.Controller,
		tenantManager: cfg.TenantManager,
		logger:        cfg.Logger,
		webhookSecret: cfg.WebhookSecret,
	}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", s.handleHealthz)
	mux.HandleFunc("GET /readyz", s.handleReadyz)
	mux.HandleFunc("GET /metrics", s.handleMetrics)
	mux.HandleFunc("GET /api/v1/status", s.handleStatus)
	mux.HandleFunc("GET /api/v1/status/{tenant}", s.handleTenantStatus)
	mux.HandleFunc("POST /api/v1/sync", s.handleSync)
	mux.HandleFunc("POST /api/v1/sync/{tenant}", s.handleTenantSync)
	mux.HandleFunc("POST /api/v1/webhook/git", s.handleWebhook)

	s.httpServer = &http.Server{Addr: cfg.Addr, Handler: mux}
	return s
}

// ListenAndServe starts the HTTP server. Blocks until server stops.
func (s *Server) ListenAndServe() error {
	s.logger.Info("http server listening", "addr", s.httpServer.Addr)
	return s.httpServer.ListenAndServe()
}

// Shutdown gracefully stops the HTTP server.
func (s *Server) Shutdown(ctx context.Context) error {
	return s.httpServer.Shutdown(ctx)
}
