package goclaw

import "encoding/json"

// WSEventFrame represents a push event from GoClaw WS v3.
type WSEventFrame struct {
	Type    string          `json:"type"`    // always "event"
	Event   string          `json:"event"`   // e.g. "agent", "log", "health"
	Payload json.RawMessage `json:"payload,omitempty"`
	Seq     int64           `json:"seq,omitempty"`
}

// WSEventHandler is called for each event frame received from GoClaw.
// Called from the read goroutine — handler must not block.
type WSEventHandler func(WSEventFrame)
