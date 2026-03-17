package tui

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/dataplanelabs/gcplane/internal/controller"
	"github.com/dataplanelabs/gcplane/internal/manifest"
	"github.com/dataplanelabs/gcplane/internal/reconciler"
)

// AttachClient polls a running gcplane serve instance via its HTTP API.
type AttachClient struct {
	baseURL    string
	httpClient *http.Client
}

// NewAttachClient creates a client connected to a gcplane serve instance.
func NewAttachClient(baseURL string) *AttachClient {
	return &AttachClient{
		baseURL: baseURL,
		httpClient: &http.Client{
			Timeout: 5 * time.Second,
		},
	}
}

// FetchStatus fetches the sync status from /api/v1/status.
func (c *AttachClient) FetchStatus() (*controller.SyncStatus, error) {
	resp, err := c.httpClient.Get(c.baseURL + "/api/v1/status")
	if err != nil {
		return nil, fmt.Errorf("connect to serve: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("serve returned %d: %s", resp.StatusCode, string(body))
	}

	var status controller.SyncStatus
	if err := json.Unmarshal(body, &status); err != nil {
		return nil, fmt.Errorf("parse status: %w", err)
	}
	return &status, nil
}

// FetchTenantStatus fetches status for a specific tenant.
func (c *AttachClient) FetchTenantStatus(tenant string) (*controller.SyncStatus, error) {
	resp, err := c.httpClient.Get(c.baseURL + "/api/v1/status/" + tenant)
	if err != nil {
		return nil, fmt.Errorf("connect to serve: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("serve returned %d: %s", resp.StatusCode, string(body))
	}

	var status controller.SyncStatus
	if err := json.Unmarshal(body, &status); err != nil {
		return nil, fmt.Errorf("parse status: %w", err)
	}
	return &status, nil
}

// FetchTenantsStatus fetches the aggregated status map for multi-tenant mode.
func (c *AttachClient) FetchTenantsStatus() (map[string]controller.SyncStatus, error) {
	resp, err := c.httpClient.Get(c.baseURL + "/api/v1/status")
	if err != nil {
		return nil, fmt.Errorf("connect to serve: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("serve returned %d: %s", resp.StatusCode, string(body))
	}

	// Try multi-tenant format first (map[string]SyncStatus)
	var tenants map[string]controller.SyncStatus
	if err := json.Unmarshal(body, &tenants); err == nil && len(tenants) > 0 {
		// Verify it's actually a map of tenants, not a single SyncStatus
		if _, hasTenantKey := tenants[""]; !hasTenantKey {
			return tenants, nil
		}
	}

	// Fall back to single-tenant mode
	return nil, nil
}

// TriggerSync triggers an immediate sync via POST /api/v1/sync.
func (c *AttachClient) TriggerSync() error {
	resp, err := c.httpClient.Post(c.baseURL+"/api/v1/sync", "application/json", nil)
	if err != nil {
		return fmt.Errorf("trigger sync: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("sync returned %d", resp.StatusCode)
	}
	return nil
}

// TriggerTenantSync triggers sync for a specific tenant.
func (c *AttachClient) TriggerTenantSync(tenant string) error {
	resp, err := c.httpClient.Post(c.baseURL+"/api/v1/sync/"+tenant, "application/json", nil)
	if err != nil {
		return fmt.Errorf("trigger sync: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("sync returned %d", resp.StatusCode)
	}
	return nil
}

// StatusToChanges converts a SyncStatus to reconciler.Change slice for table display.
func StatusToChanges(status *controller.SyncStatus) []reconciler.Change {
	if status == nil {
		return nil
	}
	changes := make([]reconciler.Change, 0, len(status.Resources))
	for _, r := range status.Resources {
		c := reconciler.Change{
			Kind: r.Kind,
			Name: r.Name,
		}
		switch r.Status {
		case "InSync":
			c.Action = reconciler.ActionNoop
		case "Created":
			c.Action = reconciler.ActionCreate
		case "Updated":
			c.Action = reconciler.ActionUpdate
		case "Error":
			c.Action = reconciler.ActionNoop
			c.Error = r.Message
		default:
			c.Action = reconciler.ActionNoop
		}
		changes = append(changes, c)
	}
	return changes
}

// Healthcheck checks if the serve instance is reachable.
func (c *AttachClient) Healthcheck() error {
	resp, err := c.httpClient.Get(c.baseURL + "/healthz")
	if err != nil {
		return fmt.Errorf("serve not reachable at %s: %w", c.baseURL, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("serve unhealthy: status %d", resp.StatusCode)
	}
	return nil
}

// stubProvider implements ProviderAPI but returns errors for all mutation ops.
// Used in attach mode where we can't directly access the GoClaw provider.
type stubProvider struct {
	baseURL string
}

func (s *stubProvider) Observe(kind manifest.ResourceKind, key string) (map[string]any, error) {
	return nil, fmt.Errorf("observe not available in attach mode (connected to %s)", s.baseURL)
}
func (s *stubProvider) Create(kind manifest.ResourceKind, key string, spec map[string]any) error {
	return fmt.Errorf("create not available in attach mode — use gcplane apply directly")
}
func (s *stubProvider) Update(kind manifest.ResourceKind, key string, spec map[string]any) error {
	return fmt.Errorf("update not available in attach mode — use gcplane apply directly")
}
func (s *stubProvider) Delete(kind manifest.ResourceKind, key string) error {
	return fmt.Errorf("delete not available in attach mode — use gcplane apply directly")
}
func (s *stubProvider) Close() error { return nil }
