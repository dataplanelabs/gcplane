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
// After Connect(), a background readLoop goroutine dispatches response frames
// to pending Call() waiters and event frames to the registered event handler.
type WSClient struct {
	endpoint string
	token    string
	tenantID string // optional — passed in connect handshake for tenant scoping
	userID   string // optional — X-GoClaw-User-Id equivalent for WS (default: "gcplane")
	conn     *websocket.Conn
	writeMu  sync.Mutex // protects conn writes only
	nextID   int64

	// Async read loop infrastructure
	pending   map[string]chan responseFrame // per-call response channels
	pendingMu sync.Mutex
	onEvent   WSEventHandler // called from readLoop for event frames
	done      chan struct{}   // closed when readLoop exits
	connMu    sync.Mutex     // protects conn field and connection state
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
		pending:  make(map[string]chan responseFrame),
	}
}

// SetEventHandler registers a callback for WS push events.
// Must be called before Connect(). The handler is called from the readLoop
// goroutine and must not block.
func (c *WSClient) SetEventHandler(h WSEventHandler) {
	c.onEvent = h
}

// IsConnected reports whether the underlying socket is alive. Returns false
// when never connected, when readLoop has exited (e.g., server-side close),
// or when a write failure cleared the conn in Call().
func (c *WSClient) IsConnected() bool {
	c.connMu.Lock()
	defer c.connMu.Unlock()
	return c.conn != nil
}

// Connect dials the WebSocket endpoint and performs the v3 connect handshake.
// Idempotent: returns nil immediately if already connected. After a successful
// handshake, starts the background readLoop goroutine.
func (c *WSClient) Connect(ctx context.Context) error {
	c.connMu.Lock()
	defer c.connMu.Unlock()

	if c.conn != nil {
		return nil
	}

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
	c.done = make(chan struct{})
	go c.readLoop(conn)

	return nil
}

// readLoop continuously reads frames from the WS connection and dispatches them.
// Response frames go to pending Call() waiters; event frames go to onEvent handler.
// Takes conn directly to avoid racing with Close() which nils c.conn.
func (c *WSClient) readLoop(conn *websocket.Conn) {
	defer close(c.done)
	defer c.clearConnIfSame(conn) // mark WS dead so ensureWS reconnects on next call
	for {
		_, msg, err := conn.ReadMessage()
		if err != nil {
			c.cancelAllPending(fmt.Errorf("ws read: %w", err))
			return
		}

		// Peek at the type field to determine dispatch target.
		var peek struct {
			Type string `json:"type"`
		}
		if err := json.Unmarshal(msg, &peek); err != nil {
			continue
		}

		switch peek.Type {
		case "res":
			var resp responseFrame
			if err := json.Unmarshal(msg, &resp); err != nil {
				continue
			}
			c.deliverResponse(resp)
		case "event":
			var evt WSEventFrame
			if err := json.Unmarshal(msg, &evt); err != nil {
				continue
			}
			if c.onEvent != nil {
				c.onEvent(evt)
			}
		}
	}
}

// deliverResponse sends a response frame to the matching pending Call() waiter.
func (c *WSClient) deliverResponse(resp responseFrame) {
	c.pendingMu.Lock()
	ch, ok := c.pending[resp.ID]
	if ok {
		delete(c.pending, resp.ID)
	}
	c.pendingMu.Unlock()

	if ok {
		ch <- resp
	}
}

// clearConnIfSame nils c.conn iff it still points at the supplied conn.
// Guards against racing with a concurrent reconnect that may have already
// replaced c.conn with a new socket.
func (c *WSClient) clearConnIfSame(conn *websocket.Conn) {
	c.connMu.Lock()
	if c.conn == conn {
		c.conn = nil
	}
	c.connMu.Unlock()
}

// cancelAllPending fails all pending Call() waiters with the given error.
func (c *WSClient) cancelAllPending(err error) {
	c.pendingMu.Lock()
	waiters := c.pending
	c.pending = make(map[string]chan responseFrame)
	c.pendingMu.Unlock()

	errResp := responseFrame{
		Type: "res",
		OK:   false,
		Error: &rpcError{
			Message: err.Error(),
		},
	}
	for id, ch := range waiters {
		errResp.ID = id
		ch <- errResp
	}
}

// Call sends an RPC request and waits for the matching response.
// Multiple Call() operations can run concurrently.
func (c *WSClient) Call(ctx context.Context, method string, params any) (json.RawMessage, error) {
	c.connMu.Lock()
	conn := c.conn
	c.connMu.Unlock()

	if conn == nil {
		return nil, fmt.Errorf("ws not connected")
	}

	id := fmt.Sprintf("%d", atomic.AddInt64(&c.nextID, 1))
	frame := requestFrame{
		Type:   "req",
		ID:     id,
		Method: method,
		Params: params,
	}

	// Register pending channel before sending to avoid race with readLoop.
	ch := make(chan responseFrame, 1)
	c.pendingMu.Lock()
	c.pending[id] = ch
	c.pendingMu.Unlock()

	// Write with mutex (multiple Call() may write concurrently).
	c.writeMu.Lock()
	err := conn.WriteJSON(frame)
	c.writeMu.Unlock()
	if err != nil {
		c.pendingMu.Lock()
		delete(c.pending, id)
		c.pendingMu.Unlock()
		// Write failure on a WS conn (broken pipe, connection reset) means the
		// socket is dead — clear it so the next ensureWS reconnects. Without
		// this, every subsequent Call hits the same dead conn until pod restart.
		c.clearConnIfSame(conn)
		return nil, fmt.Errorf("ws write %s: %w", method, err)
	}

	// Wait for response or context cancellation.
	select {
	case resp := <-ch:
		if !resp.OK {
			msg := "rpc error"
			if resp.Error != nil {
				msg = resp.Error.Message
			}
			return nil, fmt.Errorf("ws rpc %s: %s", method, msg)
		}
		return resp.Payload, nil
	case <-ctx.Done():
		c.pendingMu.Lock()
		delete(c.pending, id)
		c.pendingMu.Unlock()
		return nil, ctx.Err()
	}
}

// Close cleanly shuts down the WebSocket connection and readLoop.
func (c *WSClient) Close() error {
	c.connMu.Lock()
	if c.conn == nil {
		c.connMu.Unlock()
		return nil
	}
	conn := c.conn
	done := c.done
	c.connMu.Unlock()

	// Send close message (best effort), then close conn.
	// Closing the conn causes readLoop's ReadMessage to return an error and exit.
	c.writeMu.Lock()
	_ = conn.WriteMessage(
		websocket.CloseMessage,
		websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""),
	)
	c.writeMu.Unlock()
	err := conn.Close()

	// Wait for readLoop to exit.
	if done != nil {
		<-done
	}

	c.connMu.Lock()
	c.conn = nil
	c.connMu.Unlock()

	return err
}
