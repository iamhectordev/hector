package slack

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/slack-go/slack/socketmode"
)

func expandPath(path string) string {
	if strings.HasPrefix(path, "~/") {
		home, err := os.UserHomeDir()
		if err == nil {
			return filepath.Join(home, path[2:])
		}
	}
	return path
}

// EventLogger logs raw Slack socket mode events for debugging.
type EventLogger interface {
	Log(ctx context.Context, evt socketmode.Event) error
	Close() error
}

// EventLogConfig configures the file-based event logger.
type EventLogConfig struct {
	Enabled bool   `yaml:"enabled" env:"SLACK_EVENT_LOG_ENABLED"`
	Path    string `yaml:"path" env:"SLACK_EVENT_LOG_PATH"`
}

type logEntry struct {
	Timestamp time.Time       `json:"ts"`
	Type      socketmode.EventType `json:"type"`
	Payload   json.RawMessage `json:"payload,omitempty"`
}

type fileEventLogger struct {
	enc *json.Encoder
	w   *os.File
	mu  sync.Mutex
}

func NewFileEventLogger(cfg EventLogConfig) (EventLogger, error) {
	path := expandPath(cfg.Path)
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return nil, fmt.Errorf("slack: create event log dir: %w", err)
	}
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return nil, fmt.Errorf("slack: open event log: %w", err)
	}
	return &fileEventLogger{
		enc: json.NewEncoder(f),
		w:   f,
	}, nil
}

func (l *fileEventLogger) Log(_ context.Context, evt socketmode.Event) error {
	entry := logEntry{
		Timestamp: time.Now().UTC(),
		Type:      evt.Type,
	}
	if evt.Request != nil && len(evt.Request.Payload) > 0 {
		entry.Payload = evt.Request.Payload
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	if err := l.enc.Encode(entry); err != nil {
		return fmt.Errorf("slack: event log encode: %w", err)
	}
	return nil
}

func (l *fileEventLogger) Close() error {
	return l.w.Close()
}

type discardLogger struct{}

func (discardLogger) Log(_ context.Context, _ socketmode.Event) error { return nil }
func (discardLogger) Close() error                                    { return nil }
