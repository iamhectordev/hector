package slackmock

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"sync/atomic"
	"testing"

	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{CheckOrigin: func(r *http.Request) bool { return true }}

// Server is a fake Slack Socket Mode server for use in tests.
type Server struct {
	baseURL   string
	conn      *websocket.Conn
	connected chan struct{}
	seq       atomic.Uint64
}

// New starts a fake Slack server on a random port and registers cleanup with t.
func New(t *testing.T) *Server {
	t.Helper()

	s := &Server{connected: make(chan struct{})}
	mux := http.NewServeMux()
	mux.HandleFunc("/api/auth.test", s.handleAuthTest)
	mux.HandleFunc("/api/apps.connections.open", s.handleConnectionsOpen)
	mux.HandleFunc("/ws", s.handleWS)

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("slackmock: listen: %v", err)
	}

	srv := &http.Server{Handler: mux}
	go srv.Serve(ln) //nolint:errcheck

	s.baseURL = "http://" + ln.Addr().String()

	t.Cleanup(func() {
		if err := srv.Close(); err != nil {
			t.Errorf("slackmock: close server: %v", err)
		}
	})
	return s
}

// BaseURL returns the HTTP base URL of the fake server.
func (s *Server) BaseURL() string { return s.baseURL }

// Push sends a Socket Mode event envelope to the connected client and waits for the ACK.
// It waits for the client to connect if not already connected.
func (s *Server) Push(ctx context.Context, payload json.RawMessage) error {
	select {
	case <-s.connected:
	case <-ctx.Done():
		return fmt.Errorf("slackmock: no client connected: %w", ctx.Err())
	}

	conn := s.conn // safe: conn is written before connected is closed

	envelopeID := fmt.Sprintf("env-%d", s.seq.Add(1))
	env := map[string]any{
		"envelope_id":              envelopeID,
		"type":                     "events_api",
		"accepts_response_payload": false,
		"retry_attempt":            0,
		"retry_reason":             "",
		"payload":                  json.RawMessage(payload),
	}
	if err := conn.WriteJSON(env); err != nil {
		return fmt.Errorf("slackmock: write envelope: %w", err)
	}

	// wait for ACK
	ackCh := make(chan error, 1)
	go func() {
		_, msg, err := conn.ReadMessage()
		if err != nil {
			ackCh <- fmt.Errorf("slackmock: read ack: %w", err)
			return
		}
		var ack struct {
			EnvelopeID string `json:"envelope_id"`
		}
		if err := json.Unmarshal(msg, &ack); err != nil || ack.EnvelopeID != envelopeID {
			ackCh <- fmt.Errorf("slackmock: unexpected ack: %s", msg)
			return
		}
		ackCh <- nil
	}()

	select {
	case err := <-ackCh:
		return err
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (s *Server) handleAuthTest(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{ //nolint:errcheck
		"ok":      true,
		"url":     s.baseURL + "/",
		"team":    "Test Workspace",
		"user":    "bot",
		"team_id": "T111",
		"user_id": "U111",
		"bot_id":  "B111",
	})
}

func (s *Server) handleConnectionsOpen(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{ //nolint:errcheck
		"ok":  true,
		"url": "ws://" + r.Host + "/ws",
	})
}

func (s *Server) handleWS(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}

	s.conn = conn

	// send hello before signaling readiness so Push never races with this write
	conn.WriteJSON(map[string]any{ //nolint:errcheck
		"type":            "hello",
		"num_connections": 1,
		"debug_info":      map[string]any{},
		"connection_info": map[string]any{"app_id": "A111"},
	})
	close(s.connected)
}
