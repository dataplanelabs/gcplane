package controller

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"github.com/dataplanelabs/gcplane/internal/manifest"
	goclaw "github.com/dataplanelabs/gcplane/internal/provider/goclaw"
)

// ProviderVerifier can verify a provider's API key validity.
// Satisfied by *goclaw.Provider.
type ProviderVerifier interface {
	VerifyProvider(ctx context.Context, name string) error
}

// ProviderVerifyNotifier can send alerts for provider verification failures.
type ProviderVerifyNotifier interface {
	NotifyProviderVerifyFailure(ctx context.Context, failures []ProviderVerifyFailure) error
}

// ProviderVerifyFailure describes a single provider that failed key verification.
type ProviderVerifyFailure struct {
	Name  string
	Error string
}

// verifyProviders checks API key validity for all Provider resources in the manifest.
// On auth failure: logs error, increments metric, collects failures for notification.
// Skips gracefully if the provider doesn't support verification or the endpoint is unavailable.
func (c *Controller) verifyProviders(ctx context.Context, m *manifest.Manifest) {
	verifier, ok := c.provider.(ProviderVerifier)
	if !ok {
		return
	}

	var providers []manifest.Resource
	for _, r := range m.Resources {
		if r.Kind == manifest.KindProvider {
			providers = append(providers, r)
		}
	}
	if len(providers) == 0 {
		return
	}

	var failures []ProviderVerifyFailure

	for _, p := range providers {
		err := verifier.VerifyProvider(ctx, p.Name)
		if err == nil {
			continue
		}

		// Verify endpoint not available — skip silently (forward-compatible).
		if errors.Is(err, goclaw.ErrNotFound) {
			c.logger.Debug("provider verify endpoint not available, skipping",
				slog.String("provider", p.Name))
			return // endpoint doesn't exist — stop checking all providers
		}

		if errors.Is(err, goclaw.ErrUnauthorized) {
			c.logger.Error("provider API key verification failed",
				slog.String("provider", p.Name),
				slog.String("error", err.Error()))
			failures = append(failures, ProviderVerifyFailure{
				Name:  p.Name,
				Error: err.Error(),
			})
			continue
		}

		// Network/server error — log but don't count as key failure.
		c.logger.Warn("provider verify request failed",
			slog.String("provider", p.Name),
			slog.Any("error", err))
	}

	if len(failures) == 0 {
		return
	}

	// Update metric
	c.metrics.mu.Lock()
	c.metrics.ProviderVerifyErrors += int64(len(failures))
	c.metrics.mu.Unlock()

	// Send webhook alert
	if notifier, ok := c.notifier.(ProviderVerifyNotifier); ok {
		alertCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
		defer cancel()
		if err := notifier.NotifyProviderVerifyFailure(alertCtx, failures); err != nil {
			c.logger.Warn("provider verify failure notification failed",
				slog.Any("error", err))
		}
	}
}
