// Package testing provides test doubles for the llm package.
package testing

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/iamhectordev/hector/pkg/llm/schema"
)

type turn struct {
	reply *schema.Message
	err   error
}

// Completer is a scripted llm.Completer for use in tests.
// Each Complete call consumes the next scripted turn in order.
type Completer struct {
	t     *testing.T
	turns []turn
	i     int
}

// NewCompleter returns a Completer that plays back the given turns.
// Fails the test if Complete is called more times than turns provided.
func NewCompleter(t *testing.T, turns ...turn) *Completer {
	t.Helper()
	return &Completer{t: t, turns: turns}
}

func (c *Completer) Complete(_ context.Context, _ schema.CompletionRequest) (*schema.Message, error) {
	c.t.Helper()
	if c.i >= len(c.turns) {
		c.t.Fatalf("llmtest: unexpected Complete call (scripted %d turns, got call #%d)", len(c.turns), c.i+1)
		return nil, nil
	}
	turn := c.turns[c.i]
	c.i++
	return turn.reply, turn.err
}

// Stop returns a turn where the model replies with content and FinishReasonStop.
func Stop(content string) turn {
	m := schema.AssistantMessage(content)
	m.FinishReason = schema.FinishReasonStop
	return turn{reply: m}
}

// ToolCalls returns a turn where the model requests the given tool calls.
func ToolCalls(calls ...schema.ToolCall) turn {
	return turn{reply: &schema.Message{
		Role:         schema.RoleAssistant,
		FinishReason: schema.FinishReasonToolCalls,
		ToolCalls:    calls,
	}}
}

// Call constructs a schema.ToolCall from plain values.
// argsJSON must be valid JSON (e.g. `{"key":"value"}` or `{}`).
func Call(id, name, argsJSON string) schema.ToolCall {
	return schema.ToolCall{
		ID:        id,
		Name:      name,
		Arguments: json.RawMessage(argsJSON),
	}
}

// Error returns a turn where the completer returns an error.
func Error(err error) turn {
	return turn{err: err}
}

// Nil returns a turn where the completer returns a nil reply with no error.
// Useful for testing nil-reply handling in the agent loop.
func Nil() turn {
	return turn{}
}

// Remaining returns the number of scripted turns not yet consumed.
// Useful for asserting the loop stopped at the right point.
func (c *Completer) Remaining() int {
	n := len(c.turns) - c.i
	if n < 0 {
		return 0
	}
	return n
}

// Sentinel errors for common failure scenarios.
var ErrLLMDown = errors.New("llm: service unavailable")
