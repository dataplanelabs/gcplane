package goclaw

import (
	"context"
	"encoding/json"
	"fmt"
)

// observeTTSConfig fetches the global TTS config via WS RPC.
func (p *Provider) observeTTSConfig(_ string) (map[string]any, error) {
	if err := p.ensureWS(); err != nil {
		return nil, fmt.Errorf("ws connect for tts: %w", err)
	}

	payload, err := p.ws.Call(context.Background(), "tts.get", nil)
	if err != nil {
		return nil, fmt.Errorf("tts.get: %w", err)
	}

	var config map[string]any
	if err := json.Unmarshal(payload, &config); err != nil {
		return nil, fmt.Errorf("parse tts.get response: %w", err)
	}

	if len(config) == 0 {
		return nil, nil
	}
	return config, nil
}

// createTTSConfig sets the global TTS config via WS RPC (same as update).
func (p *Provider) createTTSConfig(_ string, spec map[string]any) error {
	return p.updateTTSConfig("", spec)
}

// updateTTSConfig updates the global TTS config via WS RPC.
func (p *Provider) updateTTSConfig(_ string, spec map[string]any) error {
	if err := p.ensureWS(); err != nil {
		return fmt.Errorf("ws connect for tts: %w", err)
	}

	_, err := p.ws.Call(context.Background(), "tts.set", spec)
	if err != nil {
		return fmt.Errorf("tts.set: %w", err)
	}
	return nil
}
