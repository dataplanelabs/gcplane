package goclaw

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gorilla/websocket"
)

// wsResponse describes one canned RPC response for the mock WS server.
type wsResponse struct {
	method  string
	payload any
	ok      bool
}

// newWSTestServer creates an httptest server with both HTTP routes and a WebSocket
// endpoint at /ws. The ws endpoint handles the connect handshake then serves
// canned responses keyed by RPC method. Additional HTTP routes can be provided
// via httpRoutes (called first; fall through to 404 if not matched).
func newWSTestServer(t *testing.T, responses []wsResponse, httpRoutes http.HandlerFunc) (*Provider, func()) {
	t.Helper()

	upgrader := websocket.Upgrader{CheckOrigin: func(r *http.Request) bool { return true }}

	mux := http.NewServeMux()

	// HTTP routes for non-WS calls
	if httpRoutes != nil {
		mux.HandleFunc("/", httpRoutes)
	}

	// WebSocket endpoint
	mux.HandleFunc("/ws", func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		defer conn.Close()

		// Build response map keyed by method
		respMap := make(map[string]wsResponse, len(responses))
		for _, rr := range responses {
			respMap[rr.method] = rr
		}

		for {
			var req requestFrame
			if err := conn.ReadJSON(&req); err != nil {
				return
			}

			// For connect handshake always accept
			if req.Method == "connect" {
				conn.WriteJSON(responseFrame{Type: "res", ID: req.ID, OK: true})
				continue
			}

			rr, found := respMap[req.Method]
			if !found {
				conn.WriteJSON(responseFrame{
					Type:  "res",
					ID:    req.ID,
					OK:    false,
					Error: &rpcError{Message: "method not found: " + req.Method},
				})
				continue
			}

			payload, _ := json.Marshal(rr.payload)
			conn.WriteJSON(responseFrame{
				Type:    "res",
				ID:      req.ID,
				OK:      rr.ok,
				Payload: json.RawMessage(payload),
			})
		}
	})

	srv := httptest.NewServer(mux)

	// Build the provider — we need to override the WS endpoint to use ws:// not wss://
	// The WSClient.Connect already handles http:// → ws:// translation.
	p := New(srv.URL, "test-token")
	return p, srv.Close
}

