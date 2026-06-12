package agent

import (
	"context"
	"fmt"

	"github.com/iamhectordev/hector/pkg/llm"
	"github.com/iamhectordev/hector/pkg/llm/schema"
	"github.com/iamhectordev/hector/pkg/llm/structured"
)

const perceptionPrompt = `You decide whether Hector should engage with a newly received message.

You will receive two separate inputs:
- history: prior conversation context from the session
- incoming: the newly received message or update being judged now

Return:
- action="queue" when Hector should proceed into the main agent loop
- action="ignore" when Hector should stay quiet

Use the distinction carefully:
- history is background only
- incoming is the thing to judge

Always return a short reason.`

type structuredPerceiver struct {
	extractor *structured.Extractor[PerceptionResult]
}

func NewStructuredPerceiver(completer llm.Completer) (Perceiver, error) {
	extractor, err := structured.NewExtractor[PerceptionResult](completer, perceptionPrompt)
	if err != nil {
		return nil, fmt.Errorf("agent: build perceiver: %w", err)
	}
	return &structuredPerceiver{extractor: extractor}, nil
}

func (p *structuredPerceiver) Assess(ctx context.Context, history []*schema.Message, incoming []*schema.Message) (PerceptionResult, error) {
	msgs := []*schema.Message{
		schema.UserMessage(renderPerceptionBlock("history", history)),
		schema.UserMessage(renderPerceptionBlock("incoming", incoming)),
	}
	result, err := p.extractor.Extract(ctx, msgs)
	if err != nil {
		return PerceptionResult{}, err
	}
	if result.Action != PerceptionActionIgnore && result.Action != PerceptionActionQueue {
		return PerceptionResult{}, fmt.Errorf("agent: invalid perception action %q", result.Action)
	}
	if result.Reason == "" {
		return PerceptionResult{}, fmt.Errorf("agent: perception reason is required")
	}
	return result, nil
}

func renderPerceptionBlock(name string, messages []*schema.Message) string {
	if len(messages) == 0 {
		return "<" + name + " />"
	}

	out := "<" + name + ">\n"
	for _, msg := range messages {
		out += `<message role="` + string(msg.Role) + `">` + msg.Content + "</message>\n"
	}
	out += "</" + name + ">"
	return out
}
