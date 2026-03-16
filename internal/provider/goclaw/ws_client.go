package goclaw

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gorilla/websocket"
)

// WSClient communicates with GoClaw via WebSocket RPC v3 protocol.
type WSClient struct {
	endpoint string
	token    string
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
func NewWSClient(endpoint, token string) *WSClient {
	return &WSClient{
		endpoint: endpoint,
		token:    token,
	}
}

// Connect dials the WebSocket endpoint and performs the v3 connect handshake.
func (c *WSClient) Connect(ctx context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	url := "ws://" + c.endpoint + "/ws"
	if len(c.endpoint) > 5 && c.endpoint[:5] == "https" {
		url = "wss://" + c.endpoint[8:] + "/ws"
	} else if len(c.endpoint) > 7 && c.endpoint[:7] == "http://" {
		url = "ws://" + c.endpoint[7:] + "/ws"
	} else if len(c.endpoint) > 8 && c.endpoint[:8] == "https://" {
		url = "wss://" + c.endpoint[8:] + "/ws"
	}

	dialer := websocket.Dialer{
		HandshakeTimeout: 10 * time.Second,
	}

	conn, _, err := dialer.DialContext(ctx, url, nil)
	if err != nil {
		return fmt.Errorf("ws dial %s: %w", url, err)
	}
	c.conn = conn

	// Send connect handshake
	frame := requestFrame{
		Type:   "req",
		ID:     "1",
		Method: "connect",
		Params: map[string]string{"token": c.token},
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

	// Read frames until we get matching response ID
	for {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}

		var resp responseFrame
		if err := c.conn.ReadJSON(&resp); err != nil {
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
