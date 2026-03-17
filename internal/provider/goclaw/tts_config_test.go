package goclaw

import (
	"testing"

	"github.com/dataplanelabs/gcplane/internal/manifest"
)

func TestTTSConfig_Observe_Found(t *testing.T) {
	p, cleanup := newWSTestServer(t, []wsResponse{
		{method: "tts.get", ok: true, payload: map[string]any{
			"provider": "elevenlabs",
			"voice_id": "voice-abc",
		}},
	}, nil)
	defer cleanup()

	result, err := p.Observe(manifest.KindTTSConfig, "")
	if err != nil {
		t.Fatalf("observe: %v", err)
	}
	if result == nil {
		t.Fatal("expected non-nil result")
	}
	if result["provider"] != "elevenlabs" {
		t.Errorf("expected provider=elevenlabs, got %v", result["provider"])
	}
	// snake_case → camelCase translation
	if result["voiceId"] != "voice-abc" {
		t.Errorf("expected voiceId=voice-abc, got %v", result["voiceId"])
	}
}

func TestTTSConfig_Observe_Empty(t *testing.T) {
	p, cleanup := newWSTestServer(t, []wsResponse{
		{method: "tts.get", ok: true, payload: map[string]any{}},
	}, nil)
	defer cleanup()

	result, err := p.Observe(manifest.KindTTSConfig, "")
	if err != nil {
		t.Fatalf("observe: %v", err)
	}
	if result != nil {
		t.Fatalf("expected nil for empty config, got %v", result)
	}
}

func TestTTSConfig_Create(t *testing.T) {
	p, cleanup := newWSTestServer(t, []wsResponse{
		{method: "tts.set", ok: true, payload: map[string]any{"ok": true}},
	}, nil)
	defer cleanup()

	err := p.Create(manifest.KindTTSConfig, "", map[string]any{
		"provider": "elevenlabs",
		"voiceId":  "voice-abc",
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
}

func TestTTSConfig_Update(t *testing.T) {
	p, cleanup := newWSTestServer(t, []wsResponse{
		{method: "tts.set", ok: true, payload: map[string]any{"ok": true}},
	}, nil)
	defer cleanup()

	err := p.Update(manifest.KindTTSConfig, "", map[string]any{
		"provider": "google",
		"voiceId":  "voice-xyz",
	})
	if err != nil {
		t.Fatalf("update: %v", err)
	}
}

// TTSConfig is not deletable — Delete should return nil without any calls.
func TestTTSConfig_Delete_Noop(t *testing.T) {
	p, cleanup := newWSTestServer(t, nil, nil)
	defer cleanup()

	if err := p.Delete(manifest.KindTTSConfig, ""); err != nil {
		t.Fatalf("expected no-op delete to succeed, got: %v", err)
	}
}

// ListAll for TTSConfig returns nil — it's a global singleton.
func TestTTSConfig_ListAll_Nil(t *testing.T) {
	p, cleanup := newWSTestServer(t, nil, nil)
	defer cleanup()

	infos, err := p.ListAll(manifest.KindTTSConfig)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if infos != nil {
		t.Errorf("expected nil for TTSConfig listAll, got %v", infos)
	}
}
