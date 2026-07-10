package mock

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/gorilla/websocket"
	"github.com/slack-go/slack/slackevents"
)

var upgrader = websocket.Upgrader{CheckOrigin: func(r *http.Request) bool { return true }}

type Server struct {
	baseURL      string
	conn         *websocket.Conn
	connected    chan struct{}
	seq          atomic.Uint64
	mu           sync.Mutex
	expectations map[string][]ExpectationEntry
}

type ExpectationEntry struct {
	ch       chan url.Values
	response any
}

func New(t *testing.T) *Server {
	t.Helper()

	s := &Server{
		connected:    make(chan struct{}),
		expectations: make(map[string][]ExpectationEntry),
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/api/auth.test", s.handleAuthTest)
	mux.HandleFunc("/api/apps.connections.open", s.handleConnectionsOpen)
	mux.HandleFunc("/api/", s.handleAPI)
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

func (s *Server) BaseURL() string { return s.baseURL }

func (s *Server) Expect(method string) *Expectation {
	return s.ExpectWithResponse(method, map[string]any{"ok": true})
}

func (s *Server) ExpectWithResponse(method string, response any) *Expectation {
	ch := make(chan url.Values, 1)
	s.mu.Lock()
	s.expectations[method] = append(s.expectations[method], ExpectationEntry{
		ch:       ch,
		response: response,
	})
	s.mu.Unlock()
	return &Expectation{ch: ch}
}

func (s *Server) Push(ctx context.Context, event any) error {
	select {
	case <-s.connected:
	case <-ctx.Done():
		return fmt.Errorf("slackmock: no client connected: %w", ctx.Err())
	}

	payload, err := buildPayload(event)
	if err != nil {
		return fmt.Errorf("slackmock: build payload: %w", err)
	}

	conn := s.conn

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

func (s *Server) handleAPI(w http.ResponseWriter, r *http.Request) {
	method := strings.TrimPrefix(r.URL.Path, "/api/")
	if err := r.ParseForm(); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}

	s.mu.Lock()
	entries := s.expectations[method]
	var response any = map[string]any{"ok": true}
	if len(entries) > 0 {
		entry := entries[0]
		s.expectations[method] = entries[1:]
		s.mu.Unlock()
		entry.ch <- r.Form
		if entry.response != nil {
			response = entry.response
		}
	} else {
		s.mu.Unlock()
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response) //nolint:errcheck
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

	conn.WriteJSON(map[string]any{ //nolint:errcheck
		"type":            "hello",
		"num_connections": 1,
		"debug_info":      map[string]any{},
		"connection_info": map[string]any{"app_id": "A111"},
	})
	close(s.connected)
}

func buildPayload(event any) (json.RawMessage, error) {
	var innerType string
	switch event.(type) {
	case *slackevents.MessageEvent:
		innerType = "message"
	default:
		return nil, fmt.Errorf("unsupported event type %T", event)
	}

	inner, err := json.Marshal(event)
	if err != nil {
		return nil, err
	}

	innerMap := make(map[string]any)
	if err := json.Unmarshal(inner, &innerMap); err != nil {
		return nil, err
	}
	innerMap["type"] = innerType

	callbackType := "event_callback"

	return json.Marshal(map[string]any{
		"token":      "verification-token",
		"team_id":    "T111",
		"api_app_id": "A111",
		"event":      innerMap,
		"type":       callbackType,
		"event_id":   "Ev111",
		"event_time": 1610241741,
	})
}
