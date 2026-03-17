package server

import (
	"encoding/json"
	"fmt"
	"net/http"
)

func (s *Server) handleHealthz(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	fmt.Fprint(w, `{"status":"ok"}`)
}

func (s *Server) handleReadyz(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if s.tracker.IsSynced() {
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, `{"status":"ready"}`)
	} else {
		w.WriteHeader(http.StatusServiceUnavailable)
		fmt.Fprint(w, `{"status":"not ready"}`)
	}
}

func (s *Server) handleStatus(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(s.tracker.Get())
}

func (s *Server) handleSync(w http.ResponseWriter, _ *http.Request) {
	s.controller.Trigger()
	s.logger.Info("sync triggered via API")
	w.Header().Set("Content-Type", "application/json")
	fmt.Fprint(w, `{"message":"sync triggered"}`)
}

func (s *Server) handleWebhook(w http.ResponseWriter, _ *http.Request) {
	s.controller.Trigger()
	s.logger.Info("sync triggered via webhook")
	w.WriteHeader(http.StatusOK)
}

func (s *Server) handleMetrics(w http.ResponseWriter, _ *http.Request) {
	snap := s.controller.GetMetrics().Snapshot()
	m := &snap
	w.Header().Set("Content-Type", "text/plain; version=0.0.4; charset=utf-8")

	fmt.Fprintf(w, "# HELP gcplane_sync_total Total number of sync operations\n")
	fmt.Fprintf(w, "# TYPE gcplane_sync_total counter\n")
	fmt.Fprintf(w, "gcplane_sync_total{status=\"success\"} %d\n", m.SyncSuccess)
	fmt.Fprintf(w, "gcplane_sync_total{status=\"error\"} %d\n", m.SyncErrors)
	fmt.Fprintf(w, "# HELP gcplane_sync_duration_seconds Duration of last sync\n")
	fmt.Fprintf(w, "# TYPE gcplane_sync_duration_seconds gauge\n")
	fmt.Fprintf(w, "gcplane_sync_duration_seconds %.3f\n", m.SyncDuration.Seconds())
	fmt.Fprintf(w, "# HELP gcplane_last_sync_timestamp Unix timestamp of last sync\n")
	fmt.Fprintf(w, "# TYPE gcplane_last_sync_timestamp gauge\n")
	fmt.Fprintf(w, "gcplane_last_sync_timestamp %d\n", m.LastSyncTime.Unix())
}
