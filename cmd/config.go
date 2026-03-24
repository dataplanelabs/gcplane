package cmd

import (
	"fmt"
	"os"
	"regexp"

	"github.com/dataplanelabs/gcplane/internal/manifest"
	"github.com/dataplanelabs/gcplane/internal/provider/goclaw"
	"github.com/dataplanelabs/gcplane/internal/secrets"
)

// tenantIDRe validates tenantId values (slug or UUID format).
var tenantIDRe = regexp.MustCompile(`^[a-z0-9]([a-z0-9-]*[a-z0-9])?$`)

// resolveConnection resolves the GoClaw connection config.
// Priority: CLI flags > env vars > manifest connection block.
func resolveConnection(m *manifest.Manifest) (ep, tok string, err error) {
	// Endpoint: flag > env > manifest
	ep = endpoint
	if ep == "" {
		ep = os.Getenv("GCPLANE_ENDPOINT")
	}
	if ep == "" {
		ep = m.Connection.Endpoint
	}
	if ep == "" {
		return "", "", fmt.Errorf("endpoint required: use --endpoint, GCPLANE_ENDPOINT, or manifest connection.endpoint")
	}

	// Token: flag > env > manifest
	tok = token
	if tok == "" {
		tok = os.Getenv("GCPLANE_TOKEN")
	}
	if tok == "" {
		tok = m.Connection.Token
	}
	if tok == "" {
		return "", "", fmt.Errorf("token required: use --token, GCPLANE_TOKEN, or manifest connection.token")
	}

	// Resolve secrets in connection values
	ep, err = secrets.Resolve(ep)
	if err != nil {
		return "", "", fmt.Errorf("resolve endpoint: %w", err)
	}
	tok, err = secrets.Resolve(tok)
	if err != nil {
		return "", "", fmt.Errorf("resolve token: %w", err)
	}

	return ep, tok, nil
}

// resolveProviderOpts builds goclaw.Option slice from manifest connection.
// Priority for tenantId: env var > manifest.
func resolveProviderOpts(m *manifest.Manifest) ([]goclaw.Option, error) {
	var opts []goclaw.Option
	tid := os.Getenv("GCPLANE_TENANT_ID")
	if tid == "" {
		tid = m.Connection.TenantID
	}
	if tid != "" {
		resolved, err := secrets.Resolve(tid)
		if err != nil {
			return nil, fmt.Errorf("resolve tenantId: %w", err)
		}
		if !tenantIDRe.MatchString(resolved) {
			return nil, fmt.Errorf("invalid tenantId %q: must be kebab-case (a-z0-9, hyphens)", resolved)
		}
		opts = append(opts, goclaw.WithTenantID(resolved))
	}

	// UserId: env > manifest > default ("gcplane")
	uid := os.Getenv("GCPLANE_USER_ID")
	if uid == "" {
		uid = m.Connection.UserID
	}
	if uid != "" {
		resolved, err := secrets.Resolve(uid)
		if err != nil {
			return nil, fmt.Errorf("resolve userId: %w", err)
		}
		opts = append(opts, goclaw.WithUserID(resolved))
	}

	return opts, nil
}

// loadAndValidateManifest loads and validates the manifest from configFile.
// Discovery order: --file flag > GCPLANE_CONFIG env > gcplane.yaml > gcplane.yml > .gcplane.yaml
func loadAndValidateManifest() (*manifest.Manifest, error) {
	path := configFile
	if path == "" {
		path = os.Getenv("GCPLANE_CONFIG")
	}
	if path == "" {
		for _, name := range []string{"gcplane.yaml", "gcplane.yml", ".gcplane.yaml"} {
			if _, err := os.Stat(name); err == nil {
				path = name
				break
			}
		}
	}
	if path == "" {
		return nil, fmt.Errorf("manifest file required: use --file, GCPLANE_CONFIG env, or create gcplane.yaml")
	}

	m, err := manifest.Load(path)
	if err != nil {
		return nil, err
	}

	errs := manifest.Validate(m)
	if len(errs) > 0 {
		for _, e := range errs {
			fmt.Fprintf(os.Stderr, "  %s\n", e)
		}
		return nil, fmt.Errorf("manifest validation failed with %d error(s)", len(errs))
	}

	return m, nil
}
