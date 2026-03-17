package server

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/dataplanelabs/gcplane/internal/controller"
)

func (s *Server) handleHealthz(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	fmt.Fprint(w, `{"status":"ok"}`)
}

func (s *Server) handleReadyz(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	ready := s.isReady()
	if ready {
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, `{"status":"ready"}`)
	} else {
		w.WriteHeader(http.StatusServiceUnavailable)
		fmt.Fprint(w, `{"status":"not ready"}`)
	}
}

// isReady returns true when all tenants (or single tenant) have synced at least once.
func (s *Server) isReady() bool {
	if s.tenantManager != nil {
		for _, inst := range s.tenantManager.All() {
			if !inst.Tracker.IsSynced() {
				return false
			}
		}
		return true
	}
	return s.tracker.IsSynced()
}

func (s *Server) handleStatus(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if s.tenantManager != nil {
		json.NewEncoder(w).Encode(s.tenantManager.AggregatedStatus())
		return
	}
	json.NewEncoder(w).Encode(s.tracker.Get())
}

func (s *Server) handleTenantStatus(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if s.tenantManager == nil {
		w.WriteHeader(http.StatusNotFound)
		fmt.Fprint(w, `{"error":"multi-tenant not enabled"}`)
		return
	}
	tenant := r.PathValue("tenant")
	inst, ok := s.tenantManager.Get(tenant)
	if !ok {
		w.WriteHeader(http.StatusNotFound)
		fmt.Fprintf(w, `{"error":"tenant %q not found"}`, tenant)
		return
	}
	json.NewEncoder(w).Encode(inst.Tracker.Get())
}

func (s *Server) handleSync(w http.ResponseWriter, _ *http.Request) {
	if s.tenantManager != nil {
		s.tenantManager.TriggerAll()
		s.logger.Info("sync triggered for all tenants via API")
	} else {
		s.controller.Trigger()
		s.logger.Info("sync triggered via API")
	}
	w.Header().Set("Content-Type", "application/json")
	fmt.Fprint(w, `{"message":"sync triggered"}`)
}

func (s *Server) handleTenantSync(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if s.tenantManager == nil {
		w.WriteHeader(http.StatusNotFound)
		fmt.Fprint(w, `{"error":"multi-tenant not enabled"}`)
		return
	}
	tenant := r.PathValue("tenant")
	if !s.tenantManager.Trigger(tenant) {
		w.WriteHeader(http.StatusNotFound)
		fmt.Fprintf(w, `{"error":"tenant %q not found"}`, tenant)
		return
	}
	s.logger.Info("sync triggered for tenant via API", "tenant", tenant)
	fmt.Fprintf(w, `{"message":"sync triggered","tenant":%q}`, tenant)
}

func (s *Server) handleWebhook(w http.ResponseWriter, r *http.Request) {
	if s.webhookSecret != "" {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		r.Body = io.NopCloser(bytes.NewReader(body))

		if !s.verifyWebhookSignature(r, body) {
			s.logger.Warn("webhook signature verification failed")
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
	}

	if s.tenantManager != nil {
		s.tenantManager.TriggerAll()
	} else {
		s.controller.Trigger()
	}
	s.logger.Info("sync triggered via webhook")
	w.WriteHeader(http.StatusOK)
}

// verifyWebhookSignature checks GitHub (X-Hub-Signature-256) or GitLab (X-Gitlab-Token).
func (s *Server) verifyWebhookSignature(r *http.Request, body []byte) bool {
	// GitHub: HMAC-SHA256
	if sig := r.Header.Get("X-Hub-Signature-256"); sig != "" {
		mac := hmac.New(sha256.New, []byte(s.webhookSecret))
		mac.Write(body)
		expected := "sha256=" + hex.EncodeToString(mac.Sum(nil))
		return hmac.Equal([]byte(expected), []byte(sig))
	}

	// GitLab: simple token comparison
	if token := r.Header.Get("X-Gitlab-Token"); token != "" {
		return token == s.webhookSecret
	}

	// No recognized header — reject if secret is configured
	return false
}

func (s *Server) handleMetrics(w http.ResponseWriter, _ *http.Request) {
	var m *controller.Metrics
	if s.tenantManager != nil {
		m = s.tenantManager.AggregatedMetrics()
	} else {
		snap := s.controller.GetMetrics().Snapshot()
		m = &snap
	}
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
	fmt.Fprintf(w, "# HELP gcplane_drift_detected_total Total number of sync cycles where drift was detected\n")
	fmt.Fprintf(w, "# TYPE gcplane_drift_detected_total counter\n")
	fmt.Fprintf(w, "gcplane_drift_detected_total %d\n", m.DriftDetected)
	fmt.Fprintf(w, "# HELP gcplane_drift_resources Number of resources that drifted in the last sync cycle\n")
	fmt.Fprintf(w, "# TYPE gcplane_drift_resources gauge\n")
	fmt.Fprintf(w, "gcplane_drift_resources %d\n", m.DriftResources)
}
