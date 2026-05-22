package openai

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/iamhectordev/hector/pkg/session"
	"github.com/openai/openai-go/v3/option"
)

type BodyLogConfig struct {
	Enabled bool
	Dir     string
}

func WithBodyLog(cfg BodyLogConfig) Option {
	return func(c *Completer) {
		c.bodyLog = cfg
	}
}

func (c *Completer) bodyLogMiddleware() option.Middleware {
	return func(req *http.Request, next option.MiddlewareNext) (*http.Response, error) {
		if !c.bodyLog.Enabled {
			return next(req)
		}
		s, ok := session.From(req.Context())
		if !ok || s.ID == "" {
			return next(req)
		}

		if reqBody, err := drainAndRestoreReadCloser(&req.Body); err == nil {
			if err := c.writeBodyLog(s.ID, "request", reqBody); err != nil {
				return nil, err
			}
		}

		resp, err := next(req)
		if err != nil {
			return resp, err
		}
		if resp == nil || resp.Body == nil {
			return resp, nil
		}

		respBody, err := drainAndRestoreReadCloser(&resp.Body)
		if err != nil {
			return resp, nil
		}
		if err := c.writeBodyLog(s.ID, "response", respBody); err != nil {
			_ = resp.Body.Close()
			return nil, err
		}
		return resp, nil
	}
}

func drainAndRestoreReadCloser(rc *io.ReadCloser) ([]byte, error) {
	if rc == nil || *rc == nil {
		return nil, nil
	}
	body, err := io.ReadAll(*rc)
	if err != nil {
		return nil, err
	}
	if err := (*rc).Close(); err != nil {
		return nil, err
	}
	*rc = io.NopCloser(bytes.NewReader(body))
	return body, nil
}

func (c *Completer) writeBodyLog(sessionID, direction string, body []byte) error {
	dir := c.bodyLog.Dir
	if strings.TrimSpace(dir) == "" {
		dir = "sessions/llm"
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("openai: create body log dir: %w", err)
	}

	var raw json.RawMessage
	if len(body) > 0 {
		raw = json.RawMessage(body)
	}
	record := struct {
		Direction string          `json:"direction"`
		Body      json.RawMessage `json:"body"`
	}{
		Direction: direction,
		Body:      raw,
	}
	line, err := json.Marshal(record)
	if err != nil {
		return fmt.Errorf("openai: marshal body log: %w", err)
	}
	line = append(line, '\n')

	path := filepath.Join(dir, sessionID+".jsonl")
	file, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return fmt.Errorf("openai: open body log: %w", err)
	}
	defer func() { _ = file.Close() }()
	if _, err := file.Write(line); err != nil {
		return fmt.Errorf("openai: write body log: %w", err)
	}
	return nil
}
