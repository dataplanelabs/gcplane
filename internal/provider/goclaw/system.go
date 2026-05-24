package goclaw

import (
	"context"
	"encoding/json"
	"fmt"
)

// GetCLIVersions returns {binary_name: version} for the caller's tenant from
// the goclaw `/v1/system/cli-versions` endpoint. Used by the apply-time
// requires.cli cross-check on Skill resources.
func (p *Provider) GetCLIVersions(ctx context.Context) (map[string]string, error) {
	data, err := p.http.Get(ctx, "/v1/system/cli-versions")
	if err != nil {
		return nil, fmt.Errorf("get cli-versions: %w", err)
	}
	var resp struct {
		Versions map[string]string `json:"versions"`
	}
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, fmt.Errorf("parse cli-versions response: %w", err)
	}
	if resp.Versions == nil {
		return map[string]string{}, nil
	}
	return resp.Versions, nil
}
