package controller

import (
	"log/slog"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestNewTenantManager_noDir(t *testing.T) {
	_, err := NewTenantManager(TenantManagerConfig{
		TenantsDir: "/nonexistent/path",
		Interval:   time.Second,
		Logger:     slog.Default(),
	})
	if err == nil {
		t.Fatal("expected error for missing directory")
	}
}

func TestNewTenantManager_emptyDir(t *testing.T) {
	dir := t.TempDir()
	_, err := NewTenantManager(TenantManagerConfig{
		TenantsDir: dir,
		Interval:   time.Second,
		Logger:     slog.Default(),
	})
	if err == nil {
		t.Fatal("expected error when no valid tenants found")
	}
}

func TestNewTenantManager_skipInvalidTenant(t *testing.T) {
	dir := t.TempDir()
	// Create a file (not a dir) — should be skipped
	if err := os.WriteFile(filepath.Join(dir, "not-a-dir.yaml"), []byte(""), 0o644); err != nil {
		t.Fatal(err)
	}
	// Create an empty subdir with no YAML — should be skipped (missing connection)
	if err := os.Mkdir(filepath.Join(dir, "bad-tenant"), 0o755); err != nil {
		t.Fatal(err)
	}

	_, err := NewTenantManager(TenantManagerConfig{
		TenantsDir: dir,
		Interval:   time.Second,
		Logger:     slog.Default(),
	})
	if err == nil {
		t.Fatal("expected error: no valid tenants")
	}
}

func writeTenantManifest(t *testing.T, tenantsDir, name string) {
	t.Helper()
	tenantDir := filepath.Join(tenantsDir, name)
	if err := os.Mkdir(tenantDir, 0o755); err != nil {
		t.Fatal(err)
	}
	manifest := `apiVersion: gcplane.io/v1
kind: Manifest
metadata:
  name: ` + name + `
connection:
  endpoint: http://localhost:9999
  token: test-token
resources: []
`
	if err := os.WriteFile(filepath.Join(tenantDir, "manifest.yaml"), []byte(manifest), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestTenantManager_GetAllTrigger(t *testing.T) {
	dir := t.TempDir()
	writeTenantManifest(t, dir, "alpha")
	writeTenantManifest(t, dir, "beta")

	tm, err := NewTenantManager(TenantManagerConfig{
		TenantsDir: dir,
		Interval:   time.Minute,
		Logger:     slog.Default(),
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer tm.CloseAll()

	// All() should return both tenants
	all := tm.All()
	if len(all) != 2 {
		t.Fatalf("expected 2 tenants, got %d", len(all))
	}

	// Get() should find existing tenant
	inst, ok := tm.Get("alpha")
	if !ok || inst == nil {
		t.Fatal("expected to find tenant 'alpha'")
	}

	// Get() returns false for unknown tenant
	_, ok = tm.Get("unknown")
	if ok {
		t.Fatal("expected false for unknown tenant")
	}

	// Trigger returns true for known tenant
	if !tm.Trigger("alpha") {
		t.Fatal("Trigger should return true for existing tenant")
	}
	// Trigger returns false for unknown tenant
	if tm.Trigger("unknown") {
		t.Fatal("Trigger should return false for unknown tenant")
	}

	// TriggerAll should not panic
	tm.TriggerAll()

	// AggregatedStatus returns map with both tenant keys
	status := tm.AggregatedStatus()
	if _, ok := status["alpha"]; !ok {
		t.Error("AggregatedStatus missing 'alpha'")
	}
	if _, ok := status["beta"]; !ok {
		t.Error("AggregatedStatus missing 'beta'")
	}

	// AggregatedMetrics should not panic
	_ = tm.AggregatedMetrics()
}
