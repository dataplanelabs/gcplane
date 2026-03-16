package cmd

import (
	"fmt"
	"os"

	"github.com/dataplanelabs/gcplane/internal/manifest"
	"github.com/dataplanelabs/gcplane/internal/secrets"
)

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

// loadAndValidateManifest loads and validates the manifest from configFile.
func loadAndValidateManifest() (*manifest.Manifest, error) {
	if configFile == "" {
		return nil, fmt.Errorf("manifest file required: use --file or -f")
	}

	m, err := manifest.Load(configFile)
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
