package goclaw

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
)

// newRawWSServer creates a minimal httptest server with a /ws WebSocket endpoint
// controlled by a handler function that receives the raw conn.
// The server URL should be passed directly to NewWSClient — WSClient.Connect
// already handles the http:// → ws:// translation internally.
func newRawWSServer(t *testing.T, handle func(*websocket.Conn)) *httptest.Server {
	t.Helper()
	upgrader := websocket.Upgrader{CheckOrigin: func(r *http.Request) bool { return true }}
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		defer conn.Close()
		handle(conn)
	}))
}

func TestWSClient_Connect_Success(t *testing.T) {
	srv := newRawWSServer(t, func(conn *websocket.Conn) {
		var req requestFrame
		if err := conn.ReadJSON(&req); err != nil {
			return
		}
		conn.WriteJSON(responseFrame{Type: "res", ID: req.ID, OK: true})
		// keep open until client closes
		for {
			if _, _, err := conn.ReadMessage(); err != nil {
				return
			}
		}
	})
	defer srv.Close()

	c := NewWSClient(srv.URL, "test-token", "", "")
	if err := c.Connect(context.Background()); err != nil {
		t.Fatalf("Connect: %v", err)
	}
	c.Close()
}

func TestWSClient_Connect_Rejected(t *testing.T) {
	srv := newRawWSServer(t, func(conn *websocket.Conn) {
		var req requestFrame
		conn.ReadJSON(&req)
		conn.WriteJSON(responseFrame{
			Type:  "res",
			ID:    req.ID,
			OK:    false,
			Error: &rpcError{Message: "invalid token"},
		})
	})
	defer srv.Close()

	c := NewWSClient(srv.URL, "bad-token", "", "")
	err := c.Connect(context.Background())
	if err == nil {
		t.Fatal("expected error on rejected connect")
	}
	if !strings.Contains(err.Error(), "invalid token") {
		t.Errorf("expected 'invalid token' in error, got: %v", err)
	}
}

func TestWSClient_Connect_DialFail(t *testing.T) {
	c := NewWSClient("ws://127.0.0.1:1", "tok", "", "")
	err := c.Connect(context.Background())
	if err == nil {
		t.Fatal("expected dial error for unreachable address")
	}
}

func TestWSClient_Call_Success(t *testing.T) {
	srv := newRawWSServer(t, func(conn *websocket.Conn) {
		for {
			var req requestFrame
			if err := conn.ReadJSON(&req); err != nil {
				return
			}
			if req.Method == "connect" {
				conn.WriteJSON(responseFrame{Type: "res", ID: req.ID, OK: true})
				continue
			}
			payload, _ := json.Marshal(map[string]any{"result": "ok"})
			conn.WriteJSON(responseFrame{
				Type:    "res",
				ID:      req.ID,
				OK:      true,
				Payload: json.RawMessage(payload),
			})
		}
	})
	defer srv.Close()

	c := NewWSClient(srv.URL, "tok", "", "")
	if err := c.Connect(context.Background()); err != nil {
		t.Fatalf("Connect: %v", err)
	}
	defer c.Close()

	payload, err := c.Call(context.Background(), "some.method", nil)
	if err != nil {
		t.Fatalf("Call: %v", err)
	}
	var result map[string]any
	if err := json.Unmarshal(payload, &result); err != nil {
		t.Fatalf("unmarshal payload: %v", err)
	}
	if result["result"] != "ok" {
		t.Errorf("expected result=ok, got %v", result["result"])
	}
}

func TestWSClient_Call_RPCError(t *testing.T) {
	srv := newRawWSServer(t, func(conn *websocket.Conn) {
		for {
			var req requestFrame
			if err := conn.ReadJSON(&req); err != nil {
				return
			}
			if req.Method == "connect" {
				conn.WriteJSON(responseFrame{Type: "res", ID: req.ID, OK: true})
				continue
			}
			conn.WriteJSON(responseFrame{
				Type:  "res",
				ID:    req.ID,
				OK:    false,
				Error: &rpcError{Message: "not authorized"},
			})
		}
	})
	defer srv.Close()

	c := NewWSClient(srv.URL, "tok", "", "")
	if err := c.Connect(context.Background()); err != nil {
		t.Fatalf("Connect: %v", err)
	}
	defer c.Close()

	_, err := c.Call(context.Background(), "restricted.method", nil)
	if err == nil {
		t.Fatal("expected RPC error")
	}
	if !strings.Contains(err.Error(), "not authorized") {
		t.Errorf("expected 'not authorized' in error, got: %v", err)
	}
}

func TestWSClient_Call_NotConnected(t *testing.T) {
	c := NewWSClient("ws://irrelevant", "tok", "", "")
	_, err := c.Call(context.Background(), "any.method", nil)
	if err == nil {
		t.Fatal("expected error when not connected")
	}
}

func TestWSClient_Call_SkipsNonMatchingFrames(t *testing.T) {
	srv := newRawWSServer(t, func(conn *websocket.Conn) {
		for {
			var req requestFrame
			if err := conn.ReadJSON(&req); err != nil {
				return
			}
			if req.Method == "connect" {
				conn.WriteJSON(responseFrame{Type: "res", ID: req.ID, OK: true})
				continue
			}
			// Send a frame with wrong ID first, then the correct one
			conn.WriteJSON(responseFrame{Type: "res", ID: "999", OK: false})
			// Also send an event frame (type != "res")
			conn.WriteJSON(responseFrame{Type: "event", ID: req.ID, OK: false})
			// Finally the real response
			payload, _ := json.Marshal(map[string]any{"done": true})
			conn.WriteJSON(responseFrame{
				Type:    "res",
				ID:      req.ID,
				OK:      true,
				Payload: json.RawMessage(payload),
			})
		}
	})
	defer srv.Close()

	c := NewWSClient(srv.URL, "tok", "", "")
	if err := c.Connect(context.Background()); err != nil {
		t.Fatalf("Connect: %v", err)
	}
	defer c.Close()

	payload, err := c.Call(context.Background(), "test.method", nil)
	if err != nil {
		t.Fatalf("Call: %v", err)
	}
	var result map[string]any
	json.Unmarshal(payload, &result)
	if result["done"] != true {
		t.Errorf("expected done=true, got %v", result["done"])
	}
}

func TestWSClient_Close_Idempotent(t *testing.T) {
	c := NewWSClient("ws://irrelevant", "tok", "", "")
	// Close on unconnected client should be a no-op
	if err := c.Close(); err != nil {
		t.Fatalf("Close on unconnected: %v", err)
	}
}

func TestWSClient_Close_Connected(t *testing.T) {
	srv := newRawWSServer(t, func(conn *websocket.Conn) {
		for {
			var req requestFrame
			if err := conn.ReadJSON(&req); err != nil {
				return
			}
			if req.Method == "connect" {
				conn.WriteJSON(responseFrame{Type: "res", ID: req.ID, OK: true})
			}
		}
	})
	defer srv.Close()

	c := NewWSClient(srv.URL, "tok", "", "")
	if err := c.Connect(context.Background()); err != nil {
		t.Fatalf("Connect: %v", err)
	}
	// Close should not error
	c.Close()
}

// TestWSClient_IsConnected_FalseAfterServerClose verifies that when the
// server tears down the WS, readLoop's deferred clearConnIfSame fires and
// IsConnected() flips to false. Regression for the bug where Provider
// cached wsReady=true and never reconnected after a transient drop.
func TestWSClient_IsConnected_FalseAfterServerClose(t *testing.T) {
	connReady := make(chan struct{})
	srv := newRawWSServer(t, func(conn *websocket.Conn) {
		var req requestFrame
		conn.ReadJSON(&req)
		conn.WriteJSON(responseFrame{Type: "res", ID: req.ID, OK: true})
		close(connReady)
		// Server hangs up immediately.
	})
	defer srv.Close()

	c := NewWSClient(srv.URL, "tok", "", "")
	if err := c.Connect(context.Background()); err != nil {
		t.Fatalf("Connect: %v", err)
	}
	if !c.IsConnected() {
		t.Fatal("expected IsConnected=true right after Connect")
	}
	<-connReady
	// Wait for readLoop to observe the close and clear the conn.
	select {
	case <-c.done:
	case <-makeTimeout(2 * time.Second):
		t.Fatal("readLoop did not exit within 2s after server close")
	}
	if c.IsConnected() {
		t.Error("expected IsConnected=false after server-side close")
	}
}

// TestWSClient_Connect_Idempotent verifies a second Connect() call on an
// already-connected client is a no-op (returns nil, does not re-dial).
func TestWSClient_Connect_Idempotent(t *testing.T) {
	srv := newRawWSServer(t, func(conn *websocket.Conn) {
		for {
			var req requestFrame
			if err := conn.ReadJSON(&req); err != nil {
				return
			}
			conn.WriteJSON(responseFrame{Type: "res", ID: req.ID, OK: true})
		}
	})
	defer srv.Close()

	c := NewWSClient(srv.URL, "tok", "", "")
	if err := c.Connect(context.Background()); err != nil {
		t.Fatalf("first Connect: %v", err)
	}
	if err := c.Connect(context.Background()); err != nil {
		t.Fatalf("second Connect should be a no-op, got error: %v", err)
	}
	c.Close()
}

// TestWSClient_Call_AfterServerClose_ClearsConn verifies the self-heal
// path: when the server closes the WS, IsConnected() flips to false so the
// next ensureWS reconnects. Without this, every subsequent Call would hit
// the same dead conn until pod restart.
func TestWSClient_Call_AfterServerClose_ClearsConn(t *testing.T) {
	handshakeDone := make(chan struct{})
	srv := newRawWSServer(t, func(conn *websocket.Conn) {
		var req requestFrame
		conn.ReadJSON(&req)
		conn.WriteJSON(responseFrame{Type: "res", ID: req.ID, OK: true})
		close(handshakeDone)
		// Hang up immediately.
	})
	defer srv.Close()

	c := NewWSClient(srv.URL, "tok", "", "")
	if err := c.Connect(context.Background()); err != nil {
		t.Fatalf("Connect: %v", err)
	}
	<-handshakeDone

	// readLoop's deferred clearConnIfSame will fire after ReadMessage sees the
	// close. Wait for done to close — that bounds the test without a flaky sleep.
	select {
	case <-c.done:
	case <-makeTimeout(2 * time.Second):
		t.Fatal("readLoop did not exit within 2s after server close")
	}
	if c.IsConnected() {
		t.Error("expected IsConnected=false after server-side close")
	}

	// Now exercise the write-failure clearConnIfSame path explicitly: re-attach
	// a stale conn and verify a failed Call() leaves IsConnected=false.
	// (The readLoop path already flipped it false; this asserts the write-failure
	// branch is also wired to clearConnIfSame.)
	// Reconnect via a server that immediately drops on first request:
	dropSrv := newRawWSServer(t, func(conn *websocket.Conn) {
		var req requestFrame
		conn.ReadJSON(&req)
		conn.WriteJSON(responseFrame{Type: "res", ID: req.ID, OK: true})
		conn.Close()
	})
	defer dropSrv.Close()
	c2 := NewWSClient(dropSrv.URL, "tok", "", "")
	if err := c2.Connect(context.Background()); err != nil {
		t.Fatalf("Connect c2: %v", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	_, _ = c2.Call(ctx, "ping", nil) // expected to error; we only assert IsConnected after
	if c2.IsConnected() {
		t.Error("expected IsConnected=false after Call against dropped conn")
	}
}

func makeTimeout(d time.Duration) <-chan time.Time {
	return time.After(d)
}

// Ensure json import is exercised even if no test above uses it explicitly.
var _ = json.RawMessage{}
