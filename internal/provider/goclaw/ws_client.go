package goclaw

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gorilla/websocket"
)

// WSClient communicates with GoClaw via WebSocket RPC v3 protocol.
type WSClient struct {
	endpoint string
	token    string
	tenantID string // optional — passed in connect handshake for tenant scoping
	userID   string // optional — X-GoClaw-User-Id equivalent for WS (default: "gcplane")
	conn     *websocket.Conn
	mu       sync.Mutex
	nextID   int64
}

// requestFrame is the v3 RPC request format.
type requestFrame struct {
	Type   string `json:"type"`
	ID     string `json:"id"`
	Method string `json:"method"`
	Params any    `json:"params,omitempty"`
}

// responseFrame is the v3 RPC response format.
type responseFrame struct {
	Type    string          `json:"type"`
	ID      string          `json:"id"`
	OK      bool            `json:"ok"`
	Payload json.RawMessage `json:"payload,omitempty"`
	Error   *rpcError       `json:"error,omitempty"`
}

type rpcError struct {
	Code    string `json:"code,omitempty"`
	Message string `json:"message"`
}

// NewWSClient creates a new WebSocket RPC client (not yet connected).
// tenantID is optional — when set, included in connect handshake for tenant scoping.
func NewWSClient(endpoint, token, tenantID, userID string) *WSClient {
	if userID == "" {
		userID = "gcplane"
	}
	return &WSClient{
		endpoint: endpoint,
		token:    token,
		tenantID: tenantID,
		userID:   userID,
	}
}

// Connect dials the WebSocket endpoint and performs the v3 connect handshake.
func (c *WSClient) Connect(ctx context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	wsURL := "ws://" + c.endpoint + "/ws"
	if parsed, err := url.Parse(c.endpoint); err == nil && parsed.Scheme != "" {
		switch parsed.Scheme {
		case "https", "wss":
			parsed.Scheme = "wss"
		default:
			parsed.Scheme = "ws"
		}
		parsed.Path = "/ws"
		wsURL = parsed.String()
	}

	dialer := websocket.Dialer{
		HandshakeTimeout: 10 * time.Second,
	}

	conn, _, err := dialer.DialContext(ctx, wsURL, nil)
	if err != nil {
		return fmt.Errorf("ws dial %s: %w", wsURL, err)
	}
	c.conn = conn

	// Send connect handshake
	params := map[string]string{"token": c.token, "user_id": c.userID}
	if c.tenantID != "" {
		params["tenant_id"] = c.tenantID
	}
	frame := requestFrame{
		Type:   "req",
		ID:     "1",
		Method: "connect",
		Params: params,
	}
	if err := conn.WriteJSON(frame); err != nil {
		conn.Close()
		return fmt.Errorf("ws connect handshake write: %w", err)
	}

	// Read connect response
	var resp responseFrame
	if err := conn.ReadJSON(&resp); err != nil {
		conn.Close()
		return fmt.Errorf("ws connect handshake read: %w", err)
	}
	if !resp.OK {
		conn.Close()
		msg := "unknown error"
		if resp.Error != nil {
			msg = resp.Error.Message
		}
		return fmt.Errorf("ws connect rejected: %s", msg)
	}

	atomic.StoreInt64(&c.nextID, 1)
	return nil
}

// Call sends an RPC request and waits for the matching response.
func (c *WSClient) Call(ctx context.Context, method string, params any) (json.RawMessage, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.conn == nil {
		return nil, fmt.Errorf("ws not connected")
	}

	id := fmt.Sprintf("%d", atomic.AddInt64(&c.nextID, 1))
	frame := requestFrame{
		Type:   "req",
		ID:     id,
		Method: method,
		Params: params,
	}

	if err := c.conn.WriteJSON(frame); err != nil {
		return nil, fmt.Errorf("ws write %s: %w", method, err)
	}

	// Read frames until we get matching response ID.
	// Set read deadline from context so ReadJSON unblocks on cancellation.
	for {
		if deadline, ok := ctx.Deadline(); ok {
			_ = c.conn.SetReadDeadline(deadline)
		} else {
			// Default 60s read deadline to prevent indefinite hangs.
			_ = c.conn.SetReadDeadline(time.Now().Add(60 * time.Second))
		}

		var resp responseFrame
		if err := c.conn.ReadJSON(&resp); err != nil {
			// Check if context was cancelled while blocked on read.
			if ctx.Err() != nil {
				return nil, ctx.Err()
			}
			return nil, fmt.Errorf("ws read %s: %w", method, err)
		}

		// Skip event frames or mismatched IDs
		if resp.Type != "res" || resp.ID != id {
			continue
		}

		if !resp.OK {
			msg := "rpc error"
			if resp.Error != nil {
				msg = resp.Error.Message
			}
			return nil, fmt.Errorf("ws rpc %s: %s", method, msg)
		}

		return resp.Payload, nil
	}
}

// Close cleanly shuts down the WebSocket connection.
func (c *WSClient) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.conn == nil {
		return nil
	}

	err := c.conn.WriteMessage(
		websocket.CloseMessage,
		websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""),
	)
	c.conn.Close()
	c.conn = nil
	return err
}
